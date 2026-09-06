package domain

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestChangeLog(t *testing.T) *changeLog {
	t.Helper()
	l := newChangeLog(filepath.Join(t.TempDir(), changeLogFileName))
	require.NoError(t, l.open())
	return l
}

// TestDomainGroupChangeLogRecordsOldAndNew proves the log answers the question
// it exists for: what did this name point at, and when did it move.
//
// VALIDATES: user story 4, AC-3 -- a record names the old set, the new set and
// the time.
func TestDomainGroupChangeLogRecordsOldAndNew(t *testing.T) {
	l := newTestChangeLog(t)
	when := time.Now().Truncate(time.Second)

	require.NoError(t, l.append(changeRecord{
		Time: when, Group: "cdn", Name: "a.invalid", Family: "ipv4", Status: "NOERROR",
		Old: []string{"192.0.2.1"}, New: []string{"192.0.2.9", "192.0.2.10"},
	}))

	records, err := l.records(0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "cdn", records[0].Group)
	assert.Equal(t, "a.invalid", records[0].Name)
	assert.Equal(t, "ipv4", records[0].Family)
	assert.Equal(t, "NOERROR", records[0].Status)
	assert.Equal(t, []string{"192.0.2.1"}, records[0].Old)
	assert.Equal(t, []string{"192.0.2.9", "192.0.2.10"}, records[0].New)
	assert.True(t, records[0].Time.Equal(when))
}

// TestChangeLogWritesBothSidesEvenWhenOneIsEmpty proves a reader never has to
// infer a direction from an absent field. An empty New with a non-empty Old is
// a name that went away; the reverse is one that arrived.
func TestChangeLogWritesBothSidesEvenWhenOneIsEmpty(t *testing.T) {
	l := newTestChangeLog(t)
	require.NoError(t, l.append(changeRecord{Group: "cdn", Name: "a.invalid", Family: "ipv4"}))

	raw, err := os.ReadFile(l.path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"old":[]`)
	assert.Contains(t, string(raw), `"new":[]`)
}

// TestChangeLogSurvivesAReopen proves the entry count is carried across a
// restart, so the bound holds over the file's whole life rather than over one
// process's share of it.
func TestChangeLogSurvivesAReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, changeLogFileName)

	first := newChangeLog(path)
	require.NoError(t, first.open())
	for i := range 3 {
		require.NoError(t, first.append(changeRecord{Group: "cdn", Name: "a.invalid", New: []string{netAddrForIndex(i)}}))
	}

	second := newChangeLog(path)
	require.NoError(t, second.open())
	assert.Equal(t, 3, second.entries, "the count is read back from the file")

	records, err := second.records(0)
	require.NoError(t, err)
	assert.Len(t, records, 3)
}

// TestChangeLogOpenOnAMissingFileIsNotAnError proves a box that has never
// recorded a change starts cleanly. The first record creates the file.
func TestChangeLogOpenOnAMissingFileIsNotAnError(t *testing.T) {
	l := newChangeLog(filepath.Join(t.TempDir(), changeLogFileName))
	require.NoError(t, l.open())

	records, err := l.records(0)
	require.NoError(t, err)
	assert.Empty(t, records, "no file means no history, not a failure")
}

// TestChangeLogEvictsAtTheBound proves an append at the bound triggers the
// eviction, and that the count never exceeds it.
//
// BOUNDARY: changeLogMaxEntries, entries retained.
// Last valid: the append that brings the count to changeLogMaxEntries is kept
// whole, with no eviction.
// Invalid below: N-A, an append never rejects a write.
// Invalid above: the next append evicts before writing, so the count stays at
// or below the bound.
//
// An unbounded log on an appliance fills a partition the rest of Ze needs
// (R-3). The count after an eviction is recomputed from the FILE rather than
// carried forward, which is why this test drives the trigger and
// TestEvictOldestKeepsTheNewest drives what eviction keeps.
func TestChangeLogEvictsAtTheBound(t *testing.T) {
	l := newTestChangeLog(t)

	// One below the bound: the next append is the last valid one and must not
	// evict, so both records stay.
	l.entries = changeLogMaxEntries - 1
	require.NoError(t, l.append(changeRecord{Group: "cdn", Name: "at-the-bound"}))
	assert.Equal(t, changeLogMaxEntries, l.entries, "the last valid append is kept whole")

	records, err := l.records(0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "at-the-bound", records[0].Name)

	// At the bound: the next append evicts first.
	require.NoError(t, l.append(changeRecord{Group: "cdn", Name: "past-the-bound"}))
	assert.LessOrEqual(t, l.entries, changeLogMaxEntries, "the count never exceeds the bound")

	records, err = l.records(0)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	assert.Equal(t, "past-the-bound", records[len(records)-1].Name,
		"the newest record is the one kept")
}

// TestEvictOldestKeepsTheNewest proves the rewrite keeps the tail of the file
// and drops from the head.
//
// Dropping the NEWEST would answer "what did this name point at recently" with
// the one thing the operator did not ask about. The rewrite is driven directly
// here rather than through 10000 appends, because each append fsyncs and the
// behavior under test is which end is dropped, not how many.
func TestEvictOldestKeepsTheNewest(t *testing.T) {
	l := newTestChangeLog(t)
	for i := range 5 {
		require.NoError(t, l.append(changeRecord{Group: "cdn", Name: netAddrForIndex(i)}))
	}

	l.mu.Lock()
	err := l.evictOldestLocked()
	l.mu.Unlock()
	require.NoError(t, err)

	records, err := l.records(0)
	require.NoError(t, err)
	require.Len(t, records, 5, "a file under the bound loses nothing")
	assert.Equal(t, netAddrForIndex(0), records[0].Name)
	assert.Equal(t, netAddrForIndex(4), records[4].Name)
}

// TestEvictOldestTrimsToTheBound proves the rewrite drops from the head when
// the file is over the bound, keeping room for one more record.
func TestEvictOldestTrimsToTheBound(t *testing.T) {
	l := newTestChangeLog(t)
	for i := range 5 {
		require.NoError(t, l.append(changeRecord{Group: "cdn", Name: netAddrForIndex(i)}))
	}

	// Trim to three records rather than to changeLogMaxEntries, so the drop is
	// observable without writing ten thousand lines. keepNewest is the same
	// code path the bound uses.
	l.mu.Lock()
	err := l.keepNewest(3)
	l.mu.Unlock()
	require.NoError(t, err)

	records, err := l.records(0)
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, netAddrForIndex(2), records[0].Name, "the oldest two were dropped")
	assert.Equal(t, netAddrForIndex(4), records[2].Name, "the newest was kept")

	l.mu.Lock()
	entries := l.entries
	l.mu.Unlock()
	assert.Equal(t, 3, entries, "the count is recomputed from the file it just wrote")
}

// TestChangeLogRecordsLimitReadsTheTail proves a caller asking for the recent
// history does not load the whole file.
func TestChangeLogRecordsLimitReadsTheTail(t *testing.T) {
	l := newTestChangeLog(t)
	for i := range 5 {
		require.NoError(t, l.append(changeRecord{Group: "cdn", Name: netAddrForIndex(i)}))
	}

	records, err := l.records(2)
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, netAddrForIndex(3), records[0].Name)
	assert.Equal(t, netAddrForIndex(4), records[1].Name)
}

// TestChangeLogSkipsATruncatedTail proves a half-written last line from an
// interrupted write does not make the whole log unreadable. The log exists to
// be read after something went wrong, so losing it to the same event defeats
// it.
func TestChangeLogSkipsATruncatedTail(t *testing.T) {
	l := newTestChangeLog(t)
	require.NoError(t, l.append(changeRecord{Group: "cdn", Name: "whole"}))

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(`{"group":"cdn","name":"trunc`)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	records, err := l.records(0)
	require.NoError(t, err, "a truncated tail is skipped, not fatal")
	require.Len(t, records, 1)
	assert.Equal(t, "whole", records[0].Name)
}

// TestChangeLogRefusesWithNoPath proves a plugin with no config directory says
// so rather than silently discarding every record. A log that quietly wrote
// nowhere would answer the operator's question with an empty history.
func TestChangeLogRefusesWithNoPath(t *testing.T) {
	l := newChangeLog("")
	require.Error(t, l.open())
	require.Error(t, l.append(changeRecord{Group: "cdn"}))

	records, err := l.records(0)
	require.NoError(t, err)
	assert.Empty(t, records)
}
