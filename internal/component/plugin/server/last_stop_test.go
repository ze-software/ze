package server

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/statestore"
)

// useLastStopStore registers a private store for the record and clears the
// process reference before closing it.
func useLastStopStore(t *testing.T) {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	statestore.SetStore(store)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
}

// TestLastStopRecordWrittenAndCleared proves the reason ze stopped because of
// a plugin survives the stop, and that a clean start removes it. Method: a
// fatal plugin's stop and a late stop cleanup each write an entry, read back
// with reason, plugin, detail and time; a boot refusal writes one; a clean
// start of a server that recorded nothing deletes the record, while one that
// recorded this run keeps it; with no store the reader says so.
//
// VALIDATES: AC-8 (spec-doctor-coverage-12-plugin-infra), R-5.
// PREVENTS: the next `ze doctor` unable to name why ze stopped, or naming a
// stale reason after a clean restart.
func TestLastStopRecordWrittenAndCleared(t *testing.T) {
	useLastStopStore(t)

	if _, found, err := ReadLastStop(); err != nil || found {
		t.Fatalf("ReadLastStop on a fresh store = found %v, err %v; want nothing recorded", found, err)
	}

	before := time.Now().Add(-time.Second)
	fatal, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	fatal.shutdownFunc = func() {}
	fatal.stopDaemonForPlugin("p1", "the plugin declares that its failure stops ze")
	RecordStopCleanupLate("bgp", "the reactor's cleanup did not finish before the stop deadline")

	record, found, err := ReadLastStop()
	require.NoError(t, err)
	require.True(t, found, "the fatal stop wrote no record")
	require.Len(t, record.Entries, 2)
	first := record.Entries[0]
	if first.Reason != LastStopFatalPolicy || first.Plugin != "p1" ||
		first.Detail != "the plugin declares that its failure stops ze" {
		t.Errorf("first entry = %+v, want the fatal stop of p1 with its reason", first)
	}
	if first.Time.Before(before) || first.Time.After(time.Now().Add(time.Second)) {
		t.Errorf("first entry time %v is not the moment of the stop", first.Time)
	}
	if first.Reason.Code() != diagnostic.CodeDoctorPluginFatalStop {
		t.Errorf("fatal stop code = %q, want %q", first.Reason.Code(), diagnostic.CodeDoctorPluginFatalStop)
	}
	second := record.Entries[1]
	if second.Reason != LastStopCleanupLate || second.Plugin != "bgp" ||
		second.Reason.Code() != diagnostic.CodeDoctorPluginStopCleanupLate {
		t.Errorf("second entry = %+v (code %q), want the late cleanup of bgp", second, second.Reason.Code())
	}

	// The server that recorded this run is not a clean start.
	fatal.settleLastStop()
	if _, found, err := ReadLastStop(); err != nil || !found {
		t.Fatalf("a run that recorded a stop cleared its own record: found %v, err %v", found, err)
	}

	clean, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	clean.settleLastStop()
	if _, found, err := ReadLastStop(); err != nil || found {
		t.Fatalf("after a clean start ReadLastStop = found %v, err %v; want the record deleted", found, err)
	}

	refused, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	refused.startupErr = errors.New("plugin x needs plugin y, which this build does not carry")
	refused.settleLastStop()
	record, found, err = ReadLastStop()
	require.NoError(t, err)
	require.True(t, found, "a boot refusal wrote no record")
	require.Len(t, record.Entries, 1)
	if got := record.Entries[0]; got.Reason != LastStopBootRefused ||
		got.Detail != "plugin x needs plugin y, which this build does not carry" {
		t.Errorf("boot refusal entry = %+v, want the refusal's text", got)
	}

	statestore.SetStore(nil)
	if _, _, err := ReadLastStop(); !errors.Is(err, statestore.ErrUnavailable) {
		t.Errorf("ReadLastStop with no store = %v, want statestore.ErrUnavailable", err)
	}
}
