package seqmap

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPutAndGet verifies basic put/get round-trip.
//
// VALIDATES: Put stores value retrievable by Get.
// PREVENTS: Broken map operations.
func TestPutAndGet(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)

	v, ok := m.Get("a")
	require.True(t, ok)
	assert.Equal(t, 100, v)
}

// TestGetMissing verifies Get returns false for missing key.
//
// VALIDATES: Get returns zero value and false for absent keys.
// PREVENTS: Panic on missing key lookup.
func TestGetMissing(t *testing.T) {
	m := New[string, int]()

	v, ok := m.Get("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestPutOverwrite verifies Put with existing key updates value and seq.
//
// VALIDATES: Overwriting a key updates value, seq, and keeps Len stable.
// PREVENTS: Stale values after update, double-counted entries.
func TestPutOverwrite(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("a", 5, 200)

	v, ok := m.Get("a")
	require.True(t, ok)
	assert.Equal(t, 200, v)
	assert.Equal(t, 1, m.Len(), "overwrite should not increase count")
}

// TestDelete verifies Delete removes key and returns true.
//
// VALIDATES: Delete removes key from map, returns true for existing key.
// PREVENTS: Stale entries after delete.
func TestDelete(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)

	ok := m.Delete("a")
	assert.True(t, ok)
	assert.Equal(t, 0, m.Len())

	_, found := m.Get("a")
	assert.False(t, found)
}

// TestDeleteNonExistent verifies Delete returns false for missing key.
//
// VALIDATES: Delete of absent key returns false without panic.
// PREVENTS: Panic on deleting missing key.
func TestDeleteNonExistent(t *testing.T) {
	m := New[string, int]()
	ok := m.Delete("missing")
	assert.False(t, ok)
}

// TestLen verifies Len tracks live entries through put/delete/overwrite.
//
// VALIDATES: Len reflects only live entries.
// PREVENTS: Wrong count after mixed operations.
func TestLen(t *testing.T) {
	m := New[string, int]()
	assert.Equal(t, 0, m.Len())

	m.Put("a", 1, 100)
	assert.Equal(t, 1, m.Len())

	m.Put("b", 2, 200)
	assert.Equal(t, 2, m.Len())

	m.Put("a", 3, 150) // overwrite
	assert.Equal(t, 2, m.Len())

	m.Delete("b")
	assert.Equal(t, 1, m.Len())
}

// TestClear verifies Clear resets all state.
//
// VALIDATES: Clear empties map, log, and counters.
// PREVENTS: Stale state after clear.
func TestClear(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)
	m.Clear()

	assert.Equal(t, 0, m.Len())
	_, ok := m.Get("a")
	assert.False(t, ok)

	// Since should return nothing
	called := false
	m.Since(0, func(_ string, _ uint64, _ int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

// TestSinceAll verifies Since(0) returns all live entries in seq order.
//
// VALIDATES: Since(0) visits every entry in ascending seq order.
// PREVENTS: Missing entries or wrong order.
func TestSinceAll(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 3, 200)
	m.Put("c", 5, 300)

	var seqs []uint64
	var vals []int
	m.Since(0, func(_ string, seq uint64, v int) bool {
		seqs = append(seqs, seq)
		vals = append(vals, v)
		return true
	})

	assert.Equal(t, []uint64{1, 3, 5}, seqs)
	assert.Equal(t, []int{100, 200, 300}, vals)
}

// TestSincePartial verifies Since(N) returns only entries with seq >= N.
//
// VALIDATES: Binary search correctly skips entries below fromSeq.
// PREVENTS: O(N) scan, returning stale entries.
func TestSincePartial(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 5, 200)
	m.Put("c", 10, 300)

	var seqs []uint64
	m.Since(5, func(_ string, seq uint64, _ int) bool {
		seqs = append(seqs, seq)
		return true
	})

	assert.Equal(t, []uint64{5, 10}, seqs)
}

// TestSinceSkipsDead verifies Since skips overwritten and deleted entries.
//
// VALIDATES: Dead log entries (from overwrite or delete) are not visited.
// PREVENTS: Ghost entries in range results.
func TestSinceSkipsDead(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)
	m.Put("c", 3, 300)

	m.Put("a", 4, 150) // overwrite a: old entry (seq=1) becomes dead
	m.Delete("b")      // delete b: entry (seq=2) becomes dead

	var keys []string
	m.Since(0, func(key string, _ uint64, _ int) bool {
		keys = append(keys, key)
		return true
	})

	sort.Strings(keys) // Range order in since may vary only for same-seq, but ours are unique
	assert.Equal(t, []string{"a", "c"}, keys)
}

// TestSinceEarlyStop verifies Since stops when fn returns false.
//
// VALIDATES: Iteration stops immediately when callback returns false.
// PREVENTS: Ignoring early-stop signal.
func TestSinceEarlyStop(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)
	m.Put("c", 3, 300)

	count := 0
	m.Since(0, func(_ string, _ uint64, _ int) bool {
		count++
		return false // stop after first
	})

	assert.Equal(t, 1, count)
}

// TestSinceOrder verifies Since iterates in ascending sequence order.
//
// VALIDATES: Entries are visited in seq order (monotonic log).
// PREVENTS: Unordered results.
func TestSinceOrder(t *testing.T) {
	m := New[string, int]()
	// Insert in seq order (as required by monotonic contract)
	m.Put("c", 1, 300)
	m.Put("a", 2, 100)
	m.Put("b", 3, 200)

	var seqs []uint64
	m.Since(0, func(_ string, seq uint64, _ int) bool {
		seqs = append(seqs, seq)
		return true
	})

	assert.Equal(t, []uint64{1, 2, 3}, seqs)
}

// TestSinceEmpty verifies Since on empty map calls fn zero times.
//
// VALIDATES: No panic or incorrect behavior on empty map.
// PREVENTS: Nil dereference or out-of-bounds.
func TestSinceEmpty(t *testing.T) {
	m := New[string, int]()

	called := false
	m.Since(0, func(_ string, _ uint64, _ int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

// TestSinceBeyondMax verifies Since with seq > all entries returns nothing.
//
// VALIDATES: Binary search beyond end of log is handled correctly.
// PREVENTS: Out-of-bounds access.
func TestSinceBeyondMax(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 5, 200)

	called := false
	m.Since(999, func(_ string, _ uint64, _ int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

// TestRange verifies Range visits all live entries.
//
// VALIDATES: Range iterates all live entries with key, seq, and value.
// PREVENTS: Missing entries from Range.
func TestRange(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)

	result := make(map[string]int)
	m.Range(func(key string, _ uint64, v int) bool {
		result[key] = v
		return true
	})

	assert.Equal(t, map[string]int{"a": 100, "b": 200}, result)
}

// TestRangeEarlyStop verifies Range stops when fn returns false.
//
// VALIDATES: Range iteration stops on first false return.
// PREVENTS: Ignoring early-stop signal.
func TestRangeEarlyStop(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)
	m.Put("c", 3, 300)

	count := 0
	m.Range(func(_ string, _ uint64, _ int) bool {
		count++
		return false
	})

	assert.Equal(t, 1, count)
}

// TestRangeIncludesSeq verifies Range callback receives correct seq values.
//
// VALIDATES: Range passes the current seq (not stale) for overwritten keys.
// PREVENTS: Stale seq in Range callback after overwrite.
func TestRangeIncludesSeq(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("a", 5, 200) // overwrite with new seq

	var seq uint64
	m.Range(func(_ string, s uint64, _ int) bool {
		seq = s
		return true
	})

	assert.Equal(t, uint64(5), seq, "Range should return updated seq")
}

// TestCompaction verifies auto-compaction cleans dead entries.
//
// VALIDATES: After many overwrites, log is compacted and Since still works.
// PREVENTS: Unbounded memory growth from dead log entries.
func TestCompaction(t *testing.T) {
	m := New[string, int]()

	// Create enough dead entries to trigger compaction.
	// Put 300 entries, then overwrite all of them → 300 dead + 300 live in log.
	// Threshold: dead > len(log)/2 && len(log) > 256 → 300 > 300 → true.
	for i := range 300 {
		m.Put("key", uint64(i+1), i)
	}
	// At this point: 1 live entry, 299 dead entries in log (300 total).
	// dead (299) > len(log)/2 (150) && len(log) (300) > 256 → compaction triggered.

	assert.Equal(t, 1, m.Len())

	// Since should still return the latest value
	var vals []int
	m.Since(0, func(_ string, _ uint64, v int) bool {
		vals = append(vals, v)
		return true
	})
	assert.Equal(t, []int{299}, vals)
}

// TestCompactionPreservesOrder verifies Since returns correct order after compaction.
//
// VALIDATES: Compaction rebuilds log sorted by seq, Since still correct.
// PREVENTS: Broken order after compact.
func TestCompactionPreservesOrder(t *testing.T) {
	m := New[string, int]()

	// Insert 300 distinct keys to build up log, then overwrite first 257 to trigger compaction.
	for i := range 300 {
		m.Put(string(rune('A'+i)), uint64(i+1), i)
	}
	// Now overwrite 257 of them to create enough dead entries.
	for i := range 257 {
		m.Put(string(rune('A'+i)), uint64(300+i+1), i+1000)
	}
	// dead > len(log)/2 should trigger compaction in Put.

	// Verify Since still returns in seq order
	var seqs []uint64
	m.Since(0, func(_ string, seq uint64, _ int) bool {
		seqs = append(seqs, seq)
		return true
	})

	// All seqs should be in ascending order
	for i := 1; i < len(seqs); i++ {
		assert.Greater(t, seqs[i], seqs[i-1], "seq[%d]=%d should be > seq[%d]=%d", i, seqs[i], i-1, seqs[i-1])
	}
	assert.Equal(t, 300, len(seqs), "should have 300 live entries")
}

// TestSinceAfterDelete verifies Since correctly skips deleted entries.
//
// VALIDATES: Deleted entries are invisible to Since even without compaction.
// PREVENTS: Deleted routes appearing in replay results.
func TestSinceAfterDelete(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)
	m.Put("c", 3, 300)

	m.Delete("b") // seq=2 entry becomes dead

	var seqs []uint64
	m.Since(0, func(_ string, seq uint64, _ int) bool {
		seqs = append(seqs, seq)
		return true
	})

	assert.Equal(t, []uint64{1, 3}, seqs, "deleted entry seq=2 should be skipped")
}
