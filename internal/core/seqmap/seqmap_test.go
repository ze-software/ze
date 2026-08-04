package seqmap

import (
	"sort"
	"strconv"
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

// TestDeleteTracksDead verifies Delete increments the dead-entry counter.
//
// VALIDATES: dead counter increases on Delete (drives compaction timing).
// PREVENTS: Broken compaction scheduling if dead counter goes wrong.
func TestDeleteTracksDead(t *testing.T) {
	m := New[string, int]()
	m.Put("a", 1, 100)
	m.Put("b", 2, 200)

	m.Delete("a")
	assert.Equal(t, 1, m.dead, "Delete should increment dead counter")
}

// TestCompactionNotTriggeredBelowMinLog verifies compaction requires log > compactMinLog.
//
// VALIDATES: Compaction does not trigger when log length equals compactMinLog exactly.
// PREVENTS: Off-by-one in the compactMinLog threshold.
func TestCompactionNotTriggeredBelowMinLog(t *testing.T) {
	m := New[string, int]()
	for i := range compactMinLog {
		m.Put("k", uint64(i+1), i)
	}
	assert.Equal(t, compactMinLog-1, m.dead, "no compaction at exactly compactMinLog entries")
	assert.Equal(t, compactMinLog, len(m.log), "log should not be compacted")
}

// TestCompactionTriggeredAboveMinLog verifies compaction triggers above compactMinLog.
//
// VALIDATES: One entry above threshold triggers compaction, resetting dead and shrinking log.
// PREVENTS: Compaction never triggering due to wrong threshold arithmetic.
func TestCompactionTriggeredAboveMinLog(t *testing.T) {
	m := New[string, int]()
	for i := range compactMinLog + 1 {
		m.Put("k", uint64(i+1), i)
	}
	assert.Equal(t, 0, m.dead, "compaction should reset dead counter")
	assert.Equal(t, 1, len(m.log), "compaction should shrink log to live entries only")
}

// TestCompactionRequiresStrictDeadMajority verifies dead must strictly exceed half the log.
//
// VALIDATES: dead == len/2 does not trigger compaction (strict >).
// PREVENTS: Off-by-one where dead-equals-half incorrectly triggers compaction.
func TestCompactionRequiresStrictDeadMajority(t *testing.T) {
	m := New[string, int]()
	n := compactMinLog + 2 // 258
	for i := range n {
		m.Put(strconv.Itoa(i), uint64(i+1), i)
	}
	for i := range n / 2 {
		m.Delete(strconv.Itoa(i))
	}
	assert.Equal(t, n/2, m.dead, "dead == len/2: compaction should not trigger")
}

// TestCompactionTriggersOnDeadMajority verifies compaction fires when dead just exceeds half.
//
// VALIDATES: dead == len/2 + 1 triggers compaction (complement to StrictDeadMajority).
// PREVENTS: Wrong arithmetic in dead-ratio threshold (e.g. len-2 instead of len/2).
func TestCompactionTriggersOnDeadMajority(t *testing.T) {
	m := New[string, int]()
	n := compactMinLog + 2 // 258
	for i := range n {
		m.Put(strconv.Itoa(i), uint64(i+1), i)
	}
	for i := range n/2 + 1 {
		m.Delete(strconv.Itoa(i))
	}
	assert.Equal(t, 0, m.dead, "dead just above half: compaction should trigger")
}

// TestCompactionIgnoredWithFewDeadEntries verifies compaction needs a dead majority.
//
// VALIDATES: A single dead entry in a large log does not trigger compaction.
// PREVENTS: Compaction on every Put regardless of dead ratio.
func TestCompactionIgnoredWithFewDeadEntries(t *testing.T) {
	m := New[string, int]()
	for i := range compactMinLog + 1 {
		m.Put(strconv.Itoa(i), uint64(i+1), i)
	}
	m.Put("0", uint64(compactMinLog+2), 999)
	assert.Equal(t, 1, m.dead, "single overwrite should leave dead=1")
	assert.Equal(t, compactMinLog+2, len(m.log), "log should not be compacted with few dead")
}

// TestCompactSortsMultipleLiveEntries verifies compact rebuilds log in seq order.
//
// VALIDATES: After compaction with many live entries, Since returns ascending seq order.
// PREVENTS: Broken sort comparator in compact (reversed or no-op sort).
func TestCompactSortsMultipleLiveEntries(t *testing.T) {
	m := New[string, int]()
	nKeys := 130
	for i := range nKeys {
		m.Put(strconv.Itoa(i), uint64(i+1), i)
	}
	seq := uint64(nKeys + 1)
	for range 2 {
		for i := range 100 {
			m.Put(strconv.Itoa(i), seq, i)
			seq++
		}
	}
	require.Equal(t, nKeys, m.Len())

	var seqs []uint64
	m.Since(0, func(_ string, s uint64, _ int) bool {
		seqs = append(seqs, s)
		return true
	})
	require.Equal(t, nKeys, len(seqs))
	for i := 1; i < len(seqs); i++ {
		assert.Less(t, seqs[i-1], seqs[i], "seq order violated at index %d", i)
	}
}

// TestSinceHonorsBoundOnUnsortedLog pins Since's stated bound for a caller that
// breaks the non-decreasing contract. sort.Search assumes a sorted log, so an
// entry below fromSeq that sits AFTER the search index used to be handed to fn.
//
// VALIDATES: no entry with seq < fromSeq reaches fn, whatever the insert order.
// PREVENTS: route loss in RecentUpdateCache (bgp/reactor). Its ids come from
// nextMsgID() before the ingress filter chain and its Add() calls run on
// per-peer read goroutines, so its log is not sorted. Ack acks every entry the
// walk delivers, so an out-of-bound entry is acked by a plugin that never
// handled it, evicted under a consumer that still owes it, and its UPDATE is
// forwarded to nobody.
func TestSinceHonorsBoundOnUnsortedLog(t *testing.T) {
	m := New[uint64, int]()
	// Insert order is the log order. 5 lands after the search index for 21.
	for _, s := range []uint64{20, 30, 5} {
		m.Put(s, s, int(s))
	}

	var got []uint64
	m.Since(21, func(k uint64, _ uint64, _ int) bool {
		got = append(got, k)
		return true
	})

	require.Equal(t, []uint64{30}, got, "Since(21) must deliver only entries at or above 21")
}

// TestSinceVisitsEveryEntryOnUnsortedLog is the other half of the bound: a
// binary search over an unsorted log starts PAST entries at or above fromSeq
// and never visits them.
//
// VALIDATES: every live entry >= fromSeq reaches fn, whatever the insert order.
// PREVENTS: a stranded consumer obligation in RecentUpdateCache (bgp/reactor).
// Its cumulative ack and its unregister walk both release entries through a
// Since walk, so one that is never delivered keeps a plugin's pending count
// above zero and pins a pooled read buffer until the 5-minute safety valve.
func TestSinceVisitsEveryEntryOnUnsortedLog(t *testing.T) {
	m := New[uint64, int]()
	// sort.Search for >= 41 over [40,50,10,60] returns index 3, so a plain
	// binary search would miss 50.
	for _, s := range []uint64{40, 50, 10, 60} {
		m.Put(s, s, int(s))
	}

	var got []uint64
	m.Since(41, func(k uint64, _ uint64, _ int) bool {
		got = append(got, k)
		return true
	})

	require.Equal(t, []uint64{50, 60}, got, "Since(41) must deliver every entry at or above 41")
}

// TestPutKeepsTheLogSorted pins the invariant Since's binary search rests on.
// It replaces an earlier test of a `sorted` flag and a full-scan fallback: the
// log is now kept sorted at insert, so the flag and the fallback are gone and
// there is nothing left to flag. Coverage moved, not dropped -- the property
// asserted here (Since delivers every entry at or above the bound, whatever
// the insert order) is the one the flag existed to preserve, and
// TestSinceVisitsEveryEntryOnUnsortedLog still asserts it end to end.
//
// VALIDATES: the log is non-decreasing in seq after any insert order, and a
// live entry is never lost to the slide.
// PREVENTS: Since silently degrading. A log that stops being sorted makes
// sort.Search start past entries it must visit, and RecentUpdateCache
// (bgp/reactor) releases consumer obligations through exactly that walk.
func TestPutKeepsTheLogSorted(t *testing.T) {
	m := New[uint64, int]()
	for _, s := range []uint64{40, 10, 60, 5, 55, 41} {
		m.Put(s, s, int(s))
	}

	for i := 1; i < len(m.log); i++ {
		require.LessOrEqual(t, m.log[i-1].seq, m.log[i].seq,
			"log out of order at index %d", i)
	}

	var got []uint64
	m.Since(0, func(k uint64, _ uint64, _ int) bool {
		got = append(got, k)
		return true
	})
	assert.Equal(t, []uint64{5, 10, 40, 41, 55, 60}, got, "every entry, in sequence order")

	// A replaced key keeps one live entry at its new position.
	m.Put(10, 70, 700)
	require.Equal(t, 6, m.Len(), "replacing a key must not change the live count")
	got = got[:0]
	m.Since(0, func(k uint64, _ uint64, _ int) bool {
		got = append(got, k)
		return true
	})
	assert.Equal(t, []uint64{5, 40, 41, 55, 60, 10}, got, "the replaced key moved to its new sequence")
}
