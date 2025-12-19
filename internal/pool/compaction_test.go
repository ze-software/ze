package pool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompactionReclaimsDeadSpace verifies compaction frees dead entry space.
//
// VALIDATES: Memory reclamation through compaction.
//
// PREVENTS: Unbounded memory growth as routes are withdrawn and
// re-announced, leaving dead entries consuming space forever.
func TestCompactionReclaimsDeadSpace(t *testing.T) {
	p := New(1024)

	// Create entries
	h1 := p.Intern([]byte("AAAA"))
	h2 := p.Intern([]byte("BBBB"))
	h3 := p.Intern([]byte("CCCC"))

	_ = h1
	_ = h3

	// Release middle entry (creates dead space)
	p.Release(h2)

	// Record metrics before compaction
	before := p.Metrics()
	require.Greater(t, before.DeadSlots, int32(0), "should have dead slots before compaction")

	// Force compaction
	p.Compact()

	// Record metrics after
	after := p.Metrics()

	require.Less(t, after.DeadSlots, before.DeadSlots,
		"compaction must reduce dead slots")
}

// TestCompactionPreservesLiveData verifies handles remain valid after compaction.
//
// VALIDATES: Handle stability guarantee across compaction.
//
// PREVENTS: Data corruption where compaction moves data but handles
// still point to old offsets, causing Get() to return garbage.
func TestCompactionPreservesLiveData(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("AAAA"))
	h2 := p.Intern([]byte("BBBB"))
	h3 := p.Intern([]byte("CCCC"))

	// Release middle entry
	p.Release(h2)

	// Compact
	p.Compact()

	// Remaining handles must still work
	require.Equal(t, []byte("AAAA"), p.Get(h1), "h1 must survive compaction")
	require.Equal(t, []byte("CCCC"), p.Get(h3), "h3 must survive compaction")
}

// TestCompactionMultipleRounds verifies repeated compaction works correctly.
//
// VALIDATES: Repeated compaction doesn't corrupt state.
//
// PREVENTS: State corruption from multiple compaction cycles.
func TestCompactionMultipleRounds(t *testing.T) {
	p := New(1024)

	// Round 1: create and release
	h1 := p.Intern([]byte("round1"))
	p.Release(h1)
	p.Compact()

	// Round 2: create and release
	h2 := p.Intern([]byte("round2"))
	require.Equal(t, []byte("round2"), p.Get(h2))
	p.Release(h2)
	p.Compact()

	// Round 3: create and keep
	h3 := p.Intern([]byte("round3"))
	p.Compact()
	require.Equal(t, []byte("round3"), p.Get(h3))
}

// TestCompactionWithNoDeadEntries verifies compaction on clean pool is safe.
//
// VALIDATES: No-op compaction doesn't corrupt state.
//
// PREVENTS: Panic or corruption when compacting pool with no dead entries.
func TestCompactionWithNoDeadEntries(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("live1"))
	h2 := p.Intern([]byte("live2"))

	// Compact with no dead entries
	p.Compact()

	require.Equal(t, []byte("live1"), p.Get(h1))
	require.Equal(t, []byte("live2"), p.Get(h2))
}

// TestCompactionEmptyPool verifies compaction on empty pool is safe.
//
// VALIDATES: Compaction handles edge case of empty pool.
//
// PREVENTS: Panic on compacting empty pool.
func TestCompactionEmptyPool(t *testing.T) {
	p := New(1024)

	// Compact empty pool
	require.NotPanics(t, func() {
		p.Compact()
	})

	// Pool should still be usable
	h := p.Intern([]byte("after-compact"))
	require.Equal(t, []byte("after-compact"), p.Get(h))
}

// TestCompactionAllDead verifies compaction when all entries are dead.
//
// VALIDATES: Full cleanup works correctly.
//
// PREVENTS: Corruption when removing all entries.
func TestCompactionAllDead(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("dead1"))
	h2 := p.Intern([]byte("dead2"))
	h3 := p.Intern([]byte("dead3"))

	p.Release(h1)
	p.Release(h2)
	p.Release(h3)

	p.Compact()

	m := p.Metrics()
	require.Equal(t, int32(0), m.LiveSlots, "no live slots after full release")
	require.Equal(t, int32(0), m.DeadSlots, "no dead slots after compaction")

	// Pool should still be usable
	h := p.Intern([]byte("new-after-all-dead"))
	require.Equal(t, []byte("new-after-all-dead"), p.Get(h))
}

// TestConcurrentAccessDuringCompaction verifies operations work during compaction.
//
// VALIDATES: Availability during maintenance operations.
//
// PREVENTS: BGP session stalls during compaction, causing holdtime
// expiry and session drops.
func TestConcurrentAccessDuringCompaction(t *testing.T) {
	p := New(1024 * 1024)

	// Pre-populate
	handles := make([]Handle, 1000)
	for i := range handles {
		handles[i] = p.Intern([]byte(fmt.Sprintf("data-%04d", i)))
	}

	// Release half to create dead space
	for i := 0; i < len(handles); i += 2 {
		p.Release(handles[i])
	}

	var wg sync.WaitGroup

	// Start compaction in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Compact()
	}()

	// Concurrent reads during compaction
	for i := 1; i < len(handles); i += 2 {
		wg.Add(1)
		go func(h Handle, expected string) {
			defer wg.Done()
			got := p.Get(h)
			require.Equal(t, []byte(expected), got)
		}(handles[i], fmt.Sprintf("data-%04d", i))
	}

	wg.Wait()
}

// TestConcurrentInternDuringCompaction verifies Intern works during compaction.
//
// VALIDATES: New entries can be added during compaction.
//
// PREVENTS: Blocking new route announcements during maintenance.
func TestConcurrentInternDuringCompaction(t *testing.T) {
	p := New(1024 * 1024)

	// Pre-populate and create dead space
	for i := 0; i < 100; i++ {
		h := p.Intern([]byte(fmt.Sprintf("pre-%d", i)))
		if i%2 == 0 {
			p.Release(h)
		}
	}

	var wg sync.WaitGroup

	// Compaction in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Compact()
	}()

	// Concurrent Intern during compaction
	newHandles := make([]Handle, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newHandles[idx] = p.Intern([]byte(fmt.Sprintf("new-%d", idx)))
		}(i)
	}

	wg.Wait()

	// Verify new entries are accessible
	for i, h := range newHandles {
		require.Equal(t, []byte(fmt.Sprintf("new-%d", i)), p.Get(h))
	}
}
