// Design: docs/architecture/doctor-and-health-checks.md -- shutdown dependency ordering
// Related: shutdown.go -- binds graph refusal to its actual manager
// Related: last_stop.go -- the persisted distinct shutdown-order-failed reason
package server

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/statestore"
)

func newShutdownOrderDiagnosticServer(t *testing.T, cycle bool) (*Server, *process.ProcessManager) {
	t.Helper()
	useLastStopStore(t)
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	const consumer, provider = "test-order-consumer", "test-order-provider"
	run := func(conn net.Conn) int {
		// This engine lifetime ends when the stop owner closes its connection.
		var b [1]byte
		for {
			if _, err := conn.Read(b[:]); err != nil {
				return 0
			}
		}
	}
	registerLifecyclePlugin(t, consumer, []string{provider}, run)
	var providerDependencies []string
	if cycle {
		providerDependencies = []string{consumer}
	}
	registerLifecyclePlugin(t, provider, providerDependencies, run)
	pm := process.NewProcessManager([]plugin.PluginConfig{
		{Name: consumer, Internal: true, Encoder: "json"},
		{Name: provider, Internal: true, Encoder: "json"},
	})
	if err := pm.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.procManager.Store(pm)
	return s, pm
}

// TestShutdownCyclePersistsOrderFailure drives actual Server.cleanup over two
// real engine processes whose selected optional dependencies form a cycle.
// It must retain that exact ordering refusal and persist its distinct reason,
// even though forced simultaneous cleanup itself finishes within the deadline.
// MUTATION: omit the cycle recorder, manager getter persistence or enum codec.
func TestShutdownCyclePersistsOrderFailure(t *testing.T) {
	s, pm := newShutdownOrderDiagnosticServer(t, true)
	s.cleanup()
	failure := pm.ShutdownOrderFailure()
	if failure == nil {
		t.Fatal("selected graph cycle left no manager-owned refusal")
	}
	if !strings.Contains(failure.Error(), registry.ErrCircularDependency.Error()) {
		t.Fatalf("cycle cause lost: %v", failure)
	}
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Fatalf("completed engines reported late: %v", late)
	}
	record, found, err := ReadLastStop()
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("ordering refusal was not persisted")
	}
	if len(record.Entries) != 1 {
		t.Fatalf("last-stop entries = %+v", record.Entries)
	}
	entry := record.Entries[0]
	if entry.Reason != LastStopShutdownOrder {
		t.Fatalf("persisted reason = %v", entry.Reason)
	}
	if entry.Detail != failure.Error() {
		t.Fatalf("persisted detail = %q, want %q", entry.Detail, failure.Error())
	}
	if entry.Fix != failure.Fix() {
		t.Fatalf("persisted fix = %q, want %q", entry.Fix, failure.Fix())
	}
	if entry.Reason.Code() != process.CodeDoctorPluginShutdownOrder {
		t.Fatalf("reason code = %s", entry.Reason.Code())
	}
	issues := report.Errors(0)
	if len(issues) != 1 {
		t.Fatalf("cycle report = %+v", issues)
	}
	if issues[0].Code != process.CodeDoctorPluginShutdownOrder {
		t.Fatalf("cycle report code = %s", issues[0].Code)
	}
}

// TestHealthyShutdownDoesNotPersistOrderFailure uses the same real cleanup
// with a valid dependency graph; neither an ordering refusal nor its persistent
// record may appear just because the server is stopping.
// MUTATION: unconditionally persist a shutdown-order failure from cleanup.
func TestHealthyShutdownDoesNotPersistOrderFailure(t *testing.T) {
	s, pm := newShutdownOrderDiagnosticServer(t, false)
	s.cleanup()
	if failure := pm.ShutdownOrderFailure(); failure != nil {
		t.Fatal(failure)
	}
	if _, found, err := ReadLastStop(); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("healthy cleanup invented a last-stop cause")
	}
	if issues := report.Errors(0); len(issues) != 0 {
		t.Fatalf("healthy cleanup raised errors: %+v", issues)
	}
}

// shutdownOrderWriteFailureStore preserves real last-stop reads and rejects
// only the actual durable WriteKey call, so the test reaches write failure.
type shutdownOrderWriteFailureStore struct {
	statestore.Storage
	cause error
}

func (s shutdownOrderWriteFailureStore) WriteKey(string, []byte) error { return s.cause }

// TestShutdownOrderRecordWriteFailureRaisesAnEvent proves a failed persistent
// write remains visible on the report bus while the daemon is alive, with a
// distinct subject and the actual write cause instead of a false saved record.
// MUTATION: omit the last-stop-record RaiseErrorFix in recordLastStopLogged.
func TestShutdownOrderRecordWriteFailureRaisesAnEvent(t *testing.T) {
	useLastStopStore(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	store := statestore.Store()
	cause := errors.New("synthetic last-stop durable write refusal")
	statestore.SetStore(shutdownOrderWriteFailureStore{Storage: store, cause: cause})
	t.Cleanup(func() { statestore.SetStore(store) })
	if recordLastStopLogged(LastStopEntry{Reason: LastStopShutdownOrder, Detail: "selected dependency graph contains a cycle"}) {
		t.Fatal("failed durable write was reported as recorded")
	}
	issues := report.Errors(0)
	if len(issues) != 1 {
		t.Fatalf("record failure events = %+v", issues)
	}
	issue := issues[0]
	if issue.Code != process.CodeDoctorPluginShutdownOrder {
		t.Fatalf("record event code = %s", issue.Code)
	}
	if issue.Subject != "last-stop-record" {
		t.Fatalf("record event subject = %s", issue.Subject)
	}
	if !strings.Contains(issue.Message, cause.Error()) {
		t.Fatalf("record event lost actual write cause: %+v", issue)
	}
	if issue.Fix != "ze data list" {
		t.Fatalf("record event fix = %q", issue.Fix)
	}
	if _, found, err := ReadLastStop(); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("failed write invented a persisted stop record")
	}
}
