package server

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestReloadDoctorRecords proves a runtime doctor check can read a refused
// removal that was not restored and a capability restore that failed, and
// that a committed reload clears the capability record. Method: with no
// server published the read says so; a published server holding one
// unrecovered removal and one failed restore answers both; clearing the
// restore record (what a committed reload does) leaves only the removal.
//
// VALIDATES: AC-28, AC-29 (spec-doctor-coverage-12-plugin-infra).
// PREVENTS: a plugin a refused reload removed staying down, or an OPEN
// carrying a refused reload's capabilities, with nothing in doctor.
func TestReloadDoctorRecords(t *testing.T) {
	if _, err := PluginReloadRecords(); !errors.Is(err, ErrDoctorSnapshotNotPublished) {
		t.Fatalf("PluginReloadRecords with no server = %v; want ErrDoctorSnapshotNotPublished", err)
	}

	s, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	publishDoctorSnapshot(s)
	t.Cleanup(func() { withdrawDoctorSnapshot(s) })

	proc := process.NewProcess(plugin.PluginConfig{Name: "p1"})
	clean := process.NewProcess(plugin.PluginConfig{Name: "p2"})
	s.removalMu.Lock()
	s.removals = map[*process.Process]*pluginRemoval{
		proc:  {recoveryErr: errors.New("restart refused: port in use")},
		clean: {},
	}
	s.removalMu.Unlock()
	before := time.Now().Add(-time.Second)
	s.capabilityRestore.failed(errors.New("decode stored capabilities: unexpected end"))

	records, err := PluginReloadRecords()
	require.NoError(t, err)
	require.Equal(t, []RemovalUnrecovered{{Plugin: "p1", Err: "restart refused: port in use"}}, records.RemovalsUnrecovered)
	require.Len(t, records.CapabilitiesNotRestored, 1)
	restore := records.CapabilitiesNotRestored[0]
	if restore.Err != "decode stored capabilities: unexpected end" || restore.Time.Before(before) {
		t.Fatalf("restore record = %+v; want the error and a time after %v", restore, before)
	}

	s.capabilityRestore.clear()
	records, err = PluginReloadRecords()
	require.NoError(t, err)
	require.Empty(t, records.CapabilitiesNotRestored, "a committed reload left the restore record")
	require.Len(t, records.RemovalsUnrecovered, 1)
}

// TestStopTransactionLateWritesRecord proves a config transaction that does
// not unwind at stop is written to the last-stop record, so the next local
// `ze doctor` names it. Method: an in-flight transaction that never ends and
// a 1ms grace.
//
// VALIDATES: AC-32 (the config transaction writer).
// PREVENTS: a stop that closes plugin connections under a live transaction
// with only a log line to say so.
func TestStopTransactionLateWritesRecord(t *testing.T) {
	useLastStopStore(t)
	s, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	s.txLock.mu.Lock()
	s.txLock.done = make(chan struct{})
	s.txLock.cancel = func(error) {}
	s.txLock.mu.Unlock()

	s.stopTransaction(time.Millisecond)

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found, "the late transaction wrote no record")
	require.Len(t, record.Entries, 1)
	entry := record.Entries[0]
	if entry.Reason != LastStopCleanupLate || entry.Plugin != stopOwnerConfigTransaction || entry.Detail == "" {
		t.Fatalf("entry = %+v; want a cleanup-late entry for %q with a detail", entry, stopOwnerConfigTransaction)
	}
}

// TestCleanupLateEnginesRecorded proves each plugin engine the manager saw
// miss its shutdown grace is written to the last-stop record. Method: the
// recorder over two late engine names.
//
// VALIDATES: AC-32 (the plugin engine writer).
func TestCleanupLateEnginesRecorded(t *testing.T) {
	useLastStopStore(t)
	recordLateEngines([]string{"bgp", "ike"})

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, record.Entries, 2)
	for i, want := range []string{"bgp", "ike"} {
		if record.Entries[i].Reason != LastStopCleanupLate || record.Entries[i].Plugin != want {
			t.Fatalf("entry %d = %+v; want cleanup-late for %s", i, record.Entries[i], want)
		}
	}
}
