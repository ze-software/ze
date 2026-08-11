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
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/stretchr/testify/require"
)

// TestForwardPoolOrderingProperty bundles the L94 invariants.
//
// VALIDATES: AC-1 / L94 -- batch order preservation, supersede-key
// determinism, malformed-body parsing robustness, and exactly-once
// delivery under concurrent dispatch (channel + overflow paths).
// PREVENTS: a batch handler that reshuffles/duplicates/drops items, a
// nondeterministic supersede key, a panic on malformed wire bytes, or a route
// lost when TryDispatch overflows under concurrency.
func TestForwardPoolOrderingProperty(t *testing.T) {
	t.Parallel()

	// test-relax: property 1 tested fwdReorderWithdrawalsFirst, which is deleted.
	// It hoisted every withdrawal ahead of every announcement, which inverts an
	// announce and a withdraw of ONE prefix and leaves the peer holding a route
	// that was withdrawn. The property below replaces it with the invariant the
	// batch handler now owes: whatever the mixture of kinds, the handler sees the
	// batch in the order it was queued.
	t.Run("batch_order_preserved", func(t *testing.T) {
		t.Parallel()
		f := func(flags []bool) bool {
			var seen []int
			fp := newFwdPool(func(_ fwdKey, items []fwdItem) {
				for i := range items {
					seq, ok := items[i].meta["seq"].(int)
					if !ok {
						return
					}
					seen = append(seen, seq)
				}
			}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
			defer fp.Stop()

			batch := make([]fwdItem, len(flags))
			for i, isWithdrawal := range flags {
				body := syncOrderAnnounceBody
				if isWithdrawal {
					body = syncOrderWithdrawBody
				}
				batch[i] = fwdItem{rawBodies: [][]byte{body}, meta: map[string]any{"seq": i}}
			}
			fp.safeBatchHandle(fwdKey{}, batch)

			if len(seen) != len(flags) {
				return false
			}
			return isStrictlyIncreasing(seen)
		}
		if err := quick.Check(f, propertyQuickConfig(94)); err != nil {
			t.Fatalf("batch order not preserved: %v", err)
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

	// Property 3 -- body-parsing robustness: never panics on arbitrary
	// (possibly truncated/malformed) wire bytes, and a successful parse never
	// claims more bytes than the body holds.
	//
	// test-relax: this covered fwdIsWithdrawal, deleted with the batch reorder it
	// classified for. parseBucketBody reads the same UPDATE-body shape and is the
	// reader that survives on the batch path, so the invariant moves to it.
	t.Run("body_parse_robust_to_garbage", func(t *testing.T) {
		t.Parallel()
		f := func(bodies [][]byte) bool {
			for _, body := range bodies {
				parts, ok := parseBucketBody(body)
				if !ok {
					continue
				}
				if len(parts.wd)+len(parts.attrs)+len(parts.nlri) > len(body) {
					return false
				}
			}
			return true
		}
		if err := quick.Check(f, propertyQuickConfig(96)); err != nil {
			t.Fatalf("parseBucketBody rejected an invariant on generated input: %v", err)
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
