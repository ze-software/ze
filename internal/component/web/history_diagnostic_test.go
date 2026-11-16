// Design: docs/architecture/doctor-and-health-checks.md -- transient recall failures.
// Related: editor_history.go -- authenticated callers of the shared History owner.
package web

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	_ "github.com/ze-software/ze/internal/component/doctor" // Registers the actual recent-events producer.
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/zefs"
)

type diagnosticHistoryStore struct {
	storage.Storage
	readFailure  atomic.Bool
	writeFailure atomic.Bool
}

func (s *diagnosticHistoryStore) ReadFile(key string) ([]byte, error) {
	if s.readFailure.Load() {
		if strings.HasPrefix(key, zefs.KeyHistory.Prefix()) {
			return nil, errors.New("history read unavailable: " + strings.Repeat("失", report.MessageMax))
		}
	}
	return s.Storage.ReadFile(key)
}

func (s *diagnosticHistoryStore) WriteFile(key string, data []byte, mode fs.FileMode) error {
	if s.writeFailure.Load() {
		if strings.HasPrefix(key, zefs.KeyHistory.Prefix()) {
			return errors.New("history write unavailable: " + strings.Repeat("失", report.MessageMax))
		}
	}
	return s.Storage.WriteFile(key, data, mode)
}

// TestBrowserHistoryReadFailureReportsOneBoundedDoctorEvent drives missing,
// invalid and failed reads through the manager, then reopens the retained data.
// Many owners and both modes share one class event, rather than user report keys.
func TestBrowserHistoryReadFailureReportsOneBoundedDoctorEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	mgr := newTestEditorManager(t)
	store := &diagnosticHistoryStore{Storage: mgr.store}
	mgr.store = store
	entries, err := mgr.HistorySnapshot("missing", cli.ModeOperational)
	require.NoError(t, err)
	require.Empty(t, entries)
	_, err = mgr.HistorySnapshot("invalid", cli.EditorMode(255))
	require.Error(t, err)
	require.Empty(t, report.Errors(0), "missing history and invalid mode are not failed storage reads")
	key := zefs.KeyHistory.Prefix() + "alice/config"
	require.NoError(t, store.WriteFile(key, []byte("show saved"), 0o600))
	store.readFailure.Store(true)
	for owner := range 8 {
		for _, mode := range []cli.EditorMode{cli.ModeConfig, cli.ModeOperational} {
			_, err := mgr.HistorySnapshot("reader-"+strconv.Itoa(owner), mode)
			require.ErrorContains(t, err, "history read unavailable")
			require.Error(t, mgr.RecordHistory("submitter-"+strconv.Itoa(owner), mode, nil, "show private-input-never-report", ""))
		}
	}
	_, err = mgr.HistorySnapshot("alice", cli.ModeConfig)
	require.Error(t, err)
	requireHistoryDoctorEvent(t, "doctor-cli-history-read", "history read unavailable")
	store.readFailure.Store(false)
	entries, err = mgr.HistorySnapshot("alice", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"show saved"}, entries)
	data, err := store.ReadFile(key)
	require.NoError(t, err)
	require.Equal(t, "show saved", string(data))
	requireHistoryDoctorEvent(t, "doctor-cli-history-read", "history read unavailable")
}

// TestBrowserHistoryRecoveredWriteRetainsOneBoundedDoctorEvent waits for actual
// asynchronous publications, then succeeds before any status read or Close.
// The producer must record the transient failure even after its state recovers.
func TestBrowserHistoryRecoveredWriteRetainsOneBoundedDoctorEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	synctest.Test(t, func(t *testing.T) {
		mgr := newTestEditorManager(t)
		store := &diagnosticHistoryStore{Storage: mgr.store}
		mgr.store = store
		store.writeFailure.Store(true)
		for owner := range 8 {
			for _, mode := range []cli.EditorMode{cli.ModeConfig, cli.ModeOperational} {
				// The second mode can see the first mode's completed failure.
				err := mgr.RecordHistory("writer-"+strconv.Itoa(owner), mode, nil, "show private-input-never-report", "")
				if err != nil {
					require.ErrorContains(t, err, "history write unavailable")
				}
			}
		}
		synctest.Wait()
		store.writeFailure.Store(false)
		for owner := range 8 {
			for _, mode := range []cli.EditorMode{cli.ModeConfig, cli.ModeOperational} {
				// Recording is still accepted while exposing the previous failure.
				err := mgr.RecordHistory("writer-"+strconv.Itoa(owner), mode, nil, "show recovery", "")
				if err != nil {
					require.ErrorContains(t, err, "history write unavailable")
				}
			}
		}
		synctest.Wait()
		require.NoError(t, mgr.Close())
		requireHistoryDoctorEvent(t, "doctor-cli-history-write", "history write unavailable")
		for owner := range 8 {
			for _, mode := range []string{"config", "operational"} {
				key := zefs.KeyHistory.Prefix() + "writer-" + strconv.Itoa(owner) + "/" + mode
				data, err := store.ReadFile(key)
				require.NoError(t, err)
				require.Contains(t, string(data), "show recovery")
			}
		}
	})
}

// requireHistoryDoctorEvent reads the real bounded bus and registered doctor
// recent-events check; no test-only event producer supplies the expected row.
func requireHistoryDoctorEvent(t *testing.T, code, cause string) {
	t.Helper()
	events := report.Errors(0)
	require.Len(t, events, 1, "the producer must publish one globally deduplicated class event")
	event := events[0]
	require.Equal(t, "cli-history", event.Source)
	require.Equal(t, "command-recall", event.Subject)
	require.Equal(t, code, event.Code)
	require.Equal(t, report.SeverityError, event.Severity)
	require.Equal(t, "ze show doctor", event.Fix)
	require.Contains(t, event.Message, cause)
	require.NotContains(t, event.Message, "private-input-never-report")
	require.LessOrEqual(t, len(event.Message), report.MessageMax)
	require.Empty(t, event.Detail)
	diagnostic.RegisterBuiltinCodes()
	require.NotNil(t, diagnostic.Lookup(code))
	require.Contains(t, diagnostic.EventCodes(), code)
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "recent-events" {
			continue
		}
		require.Contains(t, check.Codes, code)
		rows := check.Check(diagnostic.DoctorCheckContext{})
		require.Len(t, rows, 1)
		require.Equal(t, code, rows[0].Code)
		require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
		require.Equal(t, event.Fix, rows[0].Fix)
		require.Contains(t, rows[0].Message, cause)
		return
	}
	t.Fatal("the daemon recent-events doctor producer was not registered")
}
