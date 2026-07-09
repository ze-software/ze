// Property tests for the forward pool ordering + concurrency invariants
// (spec followup-test-infra L94).
//
// Engine: stdlib testing/quick with a fixed RNG seed (deterministic CI; R-1)
// for the pure ordering/hash invariants, plus a hand-rolled concurrency stress
// (run under -race) for the "routes never dropped under concurrent dispatch"
// invariant that a value-in/value-out property function cannot express.
package reactor

import (
	"math/rand"
	"net/netip"
	"sort"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/require"
)

// TestForwardPoolOrderingProperty bundles the L94 invariants.
//
// VALIDATES: AC-1 / L94 -- withdrawals-first stable reordering, supersede-key
// determinism, malformed-body classification robustness, and exactly-once
// delivery under concurrent dispatch (channel + overflow paths).
// PREVENTS: a batch reorder that reshuffles/duplicates/drops items, a
// nondeterministic supersede key, a panic on malformed wire bytes, or a route
// lost when TryDispatch overflows under concurrency.
func TestForwardPoolOrderingProperty(t *testing.T) {
	t.Parallel()

	// Property 1 -- fwdReorderWithdrawalsFirst: the output partitions
	// withdrawals before announcements, preserves relative order within each
	// group (stability), and is a permutation of the input (no add/drop/dup).
	t.Run("reorder_partition_stable_permutation", func(t *testing.T) {
		t.Parallel()
		f := func(flags []bool) bool {
			batch := make([]fwdItem, len(flags))
			for i, w := range flags {
				batch[i] = fwdItem{withdrawal: w, meta: map[string]any{"seq": i}}
			}
			out := fwdReorderWithdrawalsFirst(batch)
			if len(out) != len(flags) {
				return false
			}
			// Partition: no withdrawal may follow an announcement.
			seenAnnounce := false
			var wdSeqs, annSeqs, allSeqs []int
			for i := range out {
				seq, ok := out[i].meta["seq"].(int)
				if !ok {
					return false
				}
				allSeqs = append(allSeqs, seq)
				if out[i].withdrawal {
					if seenAnnounce {
						return false // withdrawal after announcement: partition broken
					}
					wdSeqs = append(wdSeqs, seq)
				} else {
					seenAnnounce = true
					annSeqs = append(annSeqs, seq)
				}
			}
			// Stability: original index == seq, so a stable partition keeps each
			// group's seqs strictly increasing.
			if !isStrictlyIncreasing(wdSeqs) || !isStrictlyIncreasing(annSeqs) {
				return false
			}
			// Permutation: the multiset of seqs is exactly {0..n-1}.
			sort.Ints(allSeqs)
			for i, s := range allSeqs {
				if s != i {
					return false
				}
			}
			return true
		}
		if err := quick.Check(f, propertyQuickConfig(94)); err != nil {
			t.Fatalf("reorder invariants violated: %v", err)
		}
	})

	// Property 2 -- fwdSupersedeKey: deterministic; empty input hashes to 0.
	t.Run("supersede_key_deterministic", func(t *testing.T) {
		t.Parallel()
		f := func(bodies [][]byte) bool {
			k1 := fwdSupersedeKey(bodies)
			k2 := fwdSupersedeKey(bodies)
			if k1 != k2 {
				return false
			}
			if len(bodies) == 0 {
				return k1 == 0
			}
			return true
		}
		if err := quick.Check(f, propertyQuickConfig(95)); err != nil {
			t.Fatalf("supersede key not deterministic: %v", err)
		}
	})

	// Property 3 -- fwdIsWithdrawal robustness: never panics on arbitrary
	// (possibly truncated/malformed) wire bytes. Reaching the assertion means no
	// panic occurred during quick.Check.
	t.Run("is_withdrawal_robust_to_garbage", func(t *testing.T) {
		t.Parallel()
		f := func(bodies [][]byte) bool {
			item := fwdItem{rawBodies: bodies}
			_ = fwdIsWithdrawal(&item) // must not panic
			return true
		}
		if err := quick.Check(f, propertyQuickConfig(96)); err != nil {
			t.Fatalf("fwdIsWithdrawal panicked on generated input: %v", err)
		}
	})

	// Property 4 -- concurrent dispatch never loses or duplicates a route.
	// G goroutines dispatch to one worker; the small channel forces the
	// TryDispatch->DispatchOverflow fallback. Every dispatched id must be
	// delivered to the handler exactly once. Run under -race.
	t.Run("concurrent_dispatch_exactly_once", func(t *testing.T) {
		t.Parallel()

		const goroutines = 16
		const perGoroutine = 64
		total := goroutines * perGoroutine

		var mu sync.Mutex
		delivered := make(map[int]int, total) // id -> delivery count

		pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
			mu.Lock()
			for i := range items {
				if id, ok := items[i].meta["id"].(int); ok {
					delivered[id]++
				}
			}
			mu.Unlock()
		}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})

		key := fwdKey{peerAddr: netip.MustParseAddrPort("10.9.8.7:179")}

		var wg sync.WaitGroup
		for g := range goroutines {
			wg.Add(1)
			go func(base int) {
				defer wg.Done()
				for k := range perGoroutine {
					item := fwdItem{meta: map[string]any{"id": base + k}}
					// Real caller pattern: non-blocking try, overflow fallback.
					if !pool.TryDispatch(key, item) {
						pool.DispatchOverflow(key, item)
					}
				}
			}(g * perGoroutine)
		}
		wg.Wait()

		// Wait until every route is delivered (routes are never dropped, only
		// deferred through the unbounded overflow buffer).
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(delivered) == total
		}, 5*time.Second, time.Millisecond, "all dispatched routes should be delivered")

		pool.Stop()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, delivered, total, "every id delivered")
		for id, count := range delivered {
			require.Equalf(t, 1, count, "id %d delivered %d times (want exactly once)", id, count)
		}
	})
}

func isStrictlyIncreasing(xs []int) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i] <= xs[i-1] {
			return false
		}
	}
	return true
}

// propertyQuickConfig returns a deterministic quick.Config (fixed seed, bounded
// count) so property runs are reproducible and cannot time out in CI.
func propertyQuickConfig(seed int64) *quick.Config {
	return &quick.Config{
		MaxCount: 2000,
		Rand:     rand.New(rand.NewSource(seed)), //nolint:gosec // deterministic test seed, not crypto
	}
}
