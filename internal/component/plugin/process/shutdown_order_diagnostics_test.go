// Design: docs/architecture/doctor-and-health-checks.md -- shutdown dependency ordering
// Related: manager_shutdown.go -- the first ordering failure and report event
package process

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestShutdownWithoutResolverReportsOrderFailure proves the defensive terminal
// stop still joins a real engine and fences starts, while retaining and raising
// the missing resolver as an ordering error, never a cleanup timeout.
// MUTATION: omit the nil-resolver RecordShutdownOrderFailure after terminal admission.
func TestShutdownWithoutResolverReportsOrderFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	const name = "test-shutdown-order-missing"
	finished := make(chan struct{})
	registerRunEngine(t, name, func(conn net.Conn) int {
		drainUntilClosed(conn)
		close(finished)
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	if err := pm.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	pm.StopInOrder(nil)
	select {
	case <-finished:
	default:
		t.Fatal("forced stop returned before its real engine cleanup")
	}
	failure := pm.ShutdownOrderFailure()
	if failure == nil {
		t.Fatal("missing resolver left no ordering failure")
	}
	if !strings.Contains(failure.Error(), "no plugin dependency resolver was supplied") {
		t.Fatal(failure)
	}
	if failure.Fix() != ShutdownOrderFix {
		t.Fatalf("fix = %q", failure.Fix())
	}
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Fatalf("joined engine reported late: %v", late)
	}
	if err := pm.Start(); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("start after forced terminal stop = %v", err)
	}
	pm.StopInOrder(nil)
	pm.RecordShutdownOrderFailure(errors.New("a later ordering refusal"))
	if current := pm.ShutdownOrderFailure(); current != failure {
		t.Fatalf("later refusal replaced first failure: %v", current)
	}
	issues := report.Errors(0)
	if len(issues) != 1 {
		t.Fatalf("ordering events = %v, want one retained first failure", issues)
	}
	if issues[0].Code != diagnostic.CodeDoctorPluginShutdownOrder {
		t.Fatalf("ordering event code = %s", issues[0].Code)
	}
	if issues[0].Subject != "plugin-shutdown" {
		t.Fatalf("ordering event subject = %s", issues[0].Subject)
	}
	if issues[0].Fix != ShutdownOrderFix {
		t.Fatalf("ordering event fix = %s", issues[0].Fix)
	}
}

// TestResolvedShutdownHasNoOrderFailure proves an empty but genuinely resolved
// manager does not invent an ordering error merely because its tiers are empty.
// MUTATION: unconditionally record an order failure for every terminal stop.
func TestResolvedShutdownHasNoOrderFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	pm := NewProcessManager(nil)
	pm.StopInOrder(func([]plugin.PluginConfig) [][]string { return nil })
	if failure := pm.ShutdownOrderFailure(); failure != nil {
		t.Fatal(failure)
	}
	if issues := report.Errors(0); len(issues) != 0 {
		t.Fatalf("healthy empty stop raised errors: %v", issues)
	}
}

// shutdownOrderAdmissionHandler observes the existing synchronous warning at
// its actual publication point. It reads only the atomic terminal fence and
// never reenters the manager lock held by a stop owner.
type shutdownOrderAdmissionHandler struct {
	pm       *ProcessManager
	seen     atomic.Bool
	unsealed atomic.Bool
}

func (*shutdownOrderAdmissionHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *shutdownOrderAdmissionHandler) WithAttrs([]slog.Attr) slog.Handler     { return h }
func (h *shutdownOrderAdmissionHandler) WithGroup(string) slog.Handler          { return h }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature; a pointer receiver for the record is not allowed.
func (h *shutdownOrderAdmissionHandler) Handle(_ context.Context, record slog.Record) error {
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "code" {
			return true
		}
		if attr.Value.String() != diagnostic.CodeDoctorPluginShutdownOrder {
			return true
		}
		h.seen.Store(true)
		if h.pm.shutdown.Load() == nil {
			h.unsealed.Store(true)
		}
		return false
	})
	return nil
}

// TestTerminalShutdownWithoutResolverRecordsFailureDuringReusableStop holds
// the actual first Stop inside a real engine finalizer, then admits terminal
// StopInOrder(nil). Its diagnostic MUST follow the terminal fence and precede
// the entered-owner return, while the original owner remains responsible for
// joining the held finalizer. Channel gates establish every ordering boundary.
// MUTATION: move nil-resolver reporting before terminal admission or after the
// entered-owner return; the actual log observation or retained result is wrong.
func TestTerminalShutdownWithoutResolverRecordsFailureDuringReusableStop(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	const name = "test-terminal-order-during-reusable-stop"
	entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseEngine := func() { releaseOnce.Do(func() { close(release) }) }
	registerRunEngine(t, name, func(conn net.Conn) int {
		drainUntilClosed(conn)
		close(entered)
		<-release
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	handler := &shutdownOrderAdmissionHandler{pm: pm}
	originalLogger := logger
	logger = func() *slog.Logger { return slog.New(handler) }
	t.Cleanup(func() { logger = originalLogger })
	if err := pm.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	// This cleanup joins all logging workers before the logger is restored,
	// including after any assertion below fails while the finalizer is held.
	t.Cleanup(func() {
		releaseEngine()
		select {
		case <-stopped:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("reusable stop owner did not join the released engine")
		}
	})
	go func() { pm.Stop(); close(stopped) }()
	select {
	case <-entered:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("reusable stop did not reach the actual engine finalizer")
	}
	pm.StopInOrder(nil)
	if !handler.seen.Load() {
		t.Fatal("terminal entry returned without its missing-resolver diagnostic")
	}
	if handler.unsealed.Load() {
		t.Fatal("ordering diagnostic preceded terminal admission")
	}
	if pm.shutdown.Load() == nil {
		t.Fatal("terminal entry published no admission fence")
	}
	failure := pm.ShutdownOrderFailure()
	if failure == nil {
		t.Fatal("terminal entry lost its ordering failure while reusable Stop owned the join")
	}
	if !strings.Contains(failure.Error(), "no plugin dependency resolver was supplied") {
		t.Fatal(failure)
	}
	issues := report.Errors(0)
	if len(issues) != 1 {
		t.Fatalf("terminal ordering events = %+v", issues)
	}
	if issues[0].Code != diagnostic.CodeDoctorPluginShutdownOrder {
		t.Fatalf("terminal ordering code = %s", issues[0].Code)
	}
	select {
	case <-stopped:
		t.Fatal("reusable owner returned while its engine finalizer was still held")
	default:
	}
	releaseEngine()
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("reusable owner did not finish after finalizer release")
	}
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Fatalf("joined original owner reported late: %v", late)
	}
	if err := pm.Start(); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("terminal overlap reopened admission: %v", err)
	}
}
