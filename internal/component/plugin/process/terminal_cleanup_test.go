// Design: docs/architecture/api/process-protocol.md -- original engine cleanup outcome
// Related: process.go -- actual internal start, stop, engine join and exit
package process

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/report"
)

const exitCleanupCode = "doctor-plugin-cleanup"
const exitCleanupCause = "original engine resource cleanup failed"

// A declared engine that fails after Stop must leave the original failure,
// even when ordinary process cleanup clears the plugin's report source.
func TestExitCleanupInternalStopRetainsOriginalFailure(t *testing.T) {
	exitCleanupReset(t)
	proc := exitCleanupStart(t, "terminal-failed-original", true, func(conn net.Conn) int {
		drainUntilClosed(conn)
		exitCleanupRecord(t, conn, exitCleanupCause)
		return 1
	})
	proc.Stop()
	exitCleanupJoin(t, proc)
	before := exitCleanupIssue(t, exitCleanupCause)
	report.ClearSource(proc.Name())
	report.ClearSource(before.Source)
	if after := exitCleanupIssue(t, exitCleanupCause); !reflect.DeepEqual(after, before) {
		t.Fatalf("ordinary cleanup changed original episode: before=%+v after=%+v", before, after)
	}
}

// A new healthy engine with the same configured name cannot clear its old
// engine's actual failure or replace the original diagnostic identity.
func TestExitCleanupReplacementKeepsOriginalFailure(t *testing.T) {
	exitCleanupReset(t)
	const name = "terminal-replacement-original"
	var generations atomic.Int32
	first := exitCleanupStart(t, name, true, func(conn net.Conn) int {
		drainUntilClosed(conn)
		if generations.Add(1) == 1 {
			exitCleanupRecord(t, conn, exitCleanupCause)
			return 1
		}
		return 0
	})
	first.Stop()
	exitCleanupJoin(t, first)
	before := exitCleanupIssue(t, exitCleanupCause)
	second := exitCleanupStartRegistered(t, name, true)
	second.Stop()
	exitCleanupJoin(t, second)
	if after := exitCleanupIssue(t, exitCleanupCause); !reflect.DeepEqual(after, before) {
		t.Fatalf("replacement rewrote original failure: before=%+v after=%+v", before, after)
	}
}

// Reservation exhaustion must refuse a declared internal engine before its
// runner begins, without publishing an invented cleanup failure.
func TestExitCleanupFullReservationsRefuseInternalStart(t *testing.T) {
	exitCleanupReset(t)
	exitCleanupFill(t, report.Losses().WarningCap)
	entered := make(chan struct{}, 1)
	name := "terminal-reservation-full"
	exitCleanupRegister(t, name, true, func(conn net.Conn) int {
		entered <- struct{}{}
		drainUntilClosed(conn)
		return 0
	})
	proc := NewProcess(plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	t.Cleanup(func() { proc.Stop(); exitCleanupJoin(t, proc) })
	err := proc.StartWithContext(t.Context())
	if err == nil {
		t.Fatal("declared engine started without capacity for its original cleanup warning")
	}
	if !strings.Contains(err.Error(), "resource warning reservation is full") {
		t.Fatalf("reservation refusal returned another failure: %v", err)
	}
	select {
	case <-entered:
		t.Fatal("failed reservation reached the engine runner")
	default:
	}
	for _, issue := range report.Warnings() {
		if issue.Code == exitCleanupCode {
			t.Fatalf("reservation refusal invented cleanup: %+v", issue)
		}
	}
}

// An ordinary engine has no exit-cleanup declaration and needs no new
// resource-warning reservation, including while every such slot is occupied.
func TestExitCleanupOrdinaryStartDoesNotReserve(t *testing.T) {
	exitCleanupReset(t)
	exitCleanupFill(t, report.Losses().WarningCap)
	proc := exitCleanupStart(t, "terminal-ordinary-control", false, func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})
	proc.Stop()
	exitCleanupJoin(t, proc)
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("ordinary exit acquired a cleanup guarantee: %+v", entries)
	}
}

// A declared successful engine releases its unused reservation only after
// the actual engine returns. Every slot is then available to another owner.
func TestExitCleanupSuccessfulInternalStopReleasesReservation(t *testing.T) {
	exitCleanupReset(t)
	proc := exitCleanupStart(t, "terminal-success-control", true, func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})
	proc.Stop()
	exitCleanupJoin(t, proc)
	exitCleanupFill(t, report.Losses().WarningCap)
	if entries := report.Warnings(); len(entries) != 0 {
		t.Fatalf("successful cleanup published a warning: %+v", entries)
	}
}

// Connection closure is not engine completion. Hold the actual runner after
// Stop and prove its original warning reservation remains occupied until join.
func TestExitCleanupInternalReservationWaitsForEngineReturn(t *testing.T) {
	exitCleanupReset(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	proc := exitCleanupStart(t, "terminal-held-engine", true, func(conn net.Conn) int {
		drainUntilClosed(conn)
		close(entered)
		<-release
		return 0
	})
	t.Cleanup(func() { once.Do(func() { close(release) }); proc.Stop(); exitCleanupJoin(t, proc) })
	proc.Stop()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-entered:
	case <-timer.C:
		t.Fatal("the stopped engine did not enter its retained cleanup")
	}
	exitCleanupFill(t, report.Losses().WarningCap-1)
	extra, err := report.NewResourceWarning("terminal-test-capacity", "must-remain-occupied")
	if err == nil {
		if closeErr := extra.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("connection closure released the original reservation before the engine returned")
	}
	once.Do(func() { close(release) })
	exitCleanupJoin(t, proc)
	last, err := report.NewResourceWarning("terminal-test-capacity", "released-after-join")
	if err != nil {
		t.Fatal(err)
	}
	if err := last.Close(); err != nil {
		t.Fatal(err)
	}
}

func exitCleanupReset(t *testing.T) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
}

func exitCleanupRegister(t *testing.T, name string, declared bool, run func(net.Conn) int) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	if err := registry.Register(registry.Registration{Name: name, Description: "original terminal cleanup fixture",
		ExitConfirmsCleanup: declared, RunEngine: run, CLIHandler: func([]string) int { return 0 }}); err != nil {
		t.Fatal(err)
	}
}

func exitCleanupStart(t *testing.T, name string, declared bool, run func(net.Conn) int) *Process {
	t.Helper()
	exitCleanupRegister(t, name, declared, run)
	return exitCleanupStartRegistered(t, name, declared)
}

func exitCleanupStartRegistered(t *testing.T, name string, declared bool) *Process {
	t.Helper()
	proc := NewProcess(plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	if err := proc.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	proc.SetRegistration(&plugin.PluginRegistration{ExitConfirmsCleanup: declared})
	t.Cleanup(func() { proc.Stop(); exitCleanupJoin(t, proc) })
	return proc
}

func exitCleanupJoin(t *testing.T, proc *Process) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := proc.Wait(ctx); err != nil {
		t.Errorf("original process did not join: %v", err)
	}
}

func exitCleanupFill(t *testing.T, count int) {
	t.Helper()
	for index := range count {
		owner, err := report.NewResourceWarning("terminal-test-capacity", fmt.Sprintf("occupied-%d", index))
		if err != nil {
			t.Fatalf("reserve unrelated slot %d/%d: %v", index, count, err)
		}
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}

func exitCleanupRecord(t *testing.T, conn net.Conn, cause string) {
	t.Helper()
	// The same test runs against the pre-implementation connection. Its
	// missing retained warning, rather than a missing Go method, is the RED.
	if recorder, ok := conn.(interface{ ReportTerminalCleanupFailure(string) error }); ok {
		if err := recorder.ReportTerminalCleanupFailure(cause); err != nil {
			t.Error(err)
		}
	}
}

func exitCleanupIssue(t *testing.T, cause string) report.Issue {
	t.Helper()
	for _, issue := range report.Warnings() {
		if issue.Code != exitCleanupCode {
			continue
		}
		if !strings.Contains(issue.Message, cause) {
			continue
		}
		if issue.Subject == "" {
			t.Fatal("original cleanup has no subject")
		}
		if issue.Fix != "ze show warnings" {
			t.Fatalf("cleanup next command=%q", issue.Fix)
		}
		return issue
	}
	t.Fatalf("no retained original cleanup warning contains %q: %+v", cause, report.Warnings())
	return report.Issue{}
}
