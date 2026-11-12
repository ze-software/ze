package server

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
)

// TestLastStopNewRunReplacesRecord proves a run's first stop reason replaces
// the record an earlier run wrote, while a run's later reasons append.
//
// VALIDATES: recordLastStopIn starts a new record when the stored one names
// another run, with its own entry bound and drop count.
// PREVENTS: ze under a supervisor, stopped lastStopEntriesMax times without a
// clean start, keeping the first run's cause forever, so `ze doctor` names a
// stale reason and counts the current one as a dropped later reason.
// METHOD: one run fills the record past its bound; a second run records one
// reason and then a second. Red: the record kept across runs (a dropped count
// above zero, or the first run's entries still listed).
func TestLastStopNewRunReplacesRecord(t *testing.T) {
	useLastStopStore(t)
	earlier := time.Date(2026, 11, 12, 8, 0, 0, 1, time.UTC)
	later := earlier.Add(time.Minute)

	for range lastStopEntriesMax + 2 {
		require.NoError(t, recordLastStopIn(earlier, LastStopEntry{Reason: LastStopBootRefused, Detail: "old refusal"}))
	}
	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, record.Entries, lastStopEntriesMax)
	require.Equal(t, 2, record.Dropped, "one run's reasons past the bound are not counted")

	require.NoError(t, recordLastStopIn(later, LastStopEntry{Reason: LastStopBootRefused, Detail: "new refusal"}))
	require.NoError(t, recordLastStopIn(later, LastStopEntry{Reason: LastStopCleanupLate, Plugin: "bgp", Detail: "late"}))

	record, found, err = ReadLastStop()
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, record.Run.Equal(later), "record run = %v, want the later run %v", record.Run, later)
	require.Zero(t, record.Dropped, "the later run inherited the earlier run's drop count")
	require.Len(t, record.Entries, 2, "the later run's record still holds the earlier run's entries: %+v", record.Entries)
	require.Equal(t, "new refusal", record.Entries[0].Detail, "the later run's first reason is not first")
	require.Equal(t, LastStopCleanupLate, record.Entries[1].Reason, "a reason within the run did not append")
}

// TestFatalBootFailureRecordsOneEntry proves a fatal plugin that fails during
// boot, which also ends startup with an error, leaves one entry.
//
// VALIDATES: settleLastStop leaves the record alone when this server already
// recorded a stop of its own, before it judges the startup error.
// PREVENTS: `ze doctor` showing a fatal-policy row and a boot-refused row for
// one fault.
// METHOD: stopDaemonForPlugin records the fatal stop, then the startup error
// the failed phase set is settled. Red: the startupErr branch run first.
func TestFatalBootFailureRecordsOneEntry(t *testing.T) {
	useLastStopStore(t)
	s, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	s.shutdownFunc = func() {}

	s.stopDaemonForPlugin("fatal-boot", "the plugin declares that its failure stops ze, and its startup ended: stage 1 refused")
	s.startupErr = errors.New("fatal-boot: stage 1 refused")
	s.settleLastStop()

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found, "the fatal boot failure wrote no record")
	require.Len(t, record.Entries, 1, "one fault gave several entries: %+v", record.Entries)
	require.Equal(t, LastStopFatalPolicy, record.Entries[0].Reason)
	require.Equal(t, "fatal-boot", record.Entries[0].Plugin)
}

// TestLastStopDetailCutAtRuneBoundary proves a detail longer than the bound,
// whose bound falls inside a multi-byte character, is cut before that
// character.
//
// VALIDATES: recordLastStopIn bounds the detail with report.FitMessage.
// PREVENTS: a byte cut that splits a UTF-8 rune, which the JSON encoder then
// stores as a replacement character the plugin never wrote.
// METHOD: report.MessageMax-1 ASCII bytes then a two-byte rune straddling the
// bound. Red: Detail[:bound], which stores the ASCII run plus U+FFFD.
func TestLastStopDetailCutAtRuneBoundary(t *testing.T) {
	useLastStopStore(t)
	ascii := strings.Repeat("a", report.MessageMax-1)

	require.NoError(t, RecordLastStop(LastStopEntry{Reason: LastStopBootRefused, Detail: ascii + "é and more"}))

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, record.Entries, 1)
	detail := record.Entries[0].Detail
	require.True(t, utf8.ValidString(detail), "the stored detail is not valid UTF-8")
	require.Equal(t, ascii, detail, "the detail was not cut at the rune boundary before the bound")
}
