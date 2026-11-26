package doctor

import (
	"context"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runningState is a configured plugin whose process runs, carrying records.
func runningState(name string, records process.DoctorRecord) server.PluginDoctorState {
	return server.PluginDoctorState{Name: name, Configured: true, Running: true, HasProcess: true, Records: records}
}

// oneRow asserts rows hold exactly one finding of code and severity, whose
// message holds every want and whose fix is not empty, and answers it.
func oneRow(t *testing.T, rows []diagnostic.Diagnostic, code string, severity diagnostic.Severity, want ...string) diagnostic.Diagnostic {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("want one %s row, got %d: %+v", code, len(rows), rows)
	}
	row := rows[0]
	if row.Code != code || row.Severity != severity {
		t.Fatalf("row is %s/%v, want %s/%v", row.Code, row.Severity, code, severity)
	}
	for _, w := range want {
		if !strings.Contains(row.Message, w) {
			t.Fatalf("row message %q lacks %q", row.Message, w)
		}
	}
	if row.Fix == "" {
		t.Fatalf("row has no fix: %+v", row)
	}
	return row
}

// TestDoctorClassChecks builds each registration and delivery class row from
// a snapshot holding one finding, and from one holding none (AC-13 to AC-22).
//
// VALIDATES: each class answers one row with its code and severity, naming
// what its AC names, with a fix; an empty snapshot answers nothing; a plugin
// still starting is not reported undelivered.
// PREVENTS: a record the server keeps that no doctor row ever shows.
func TestDoctorClassChecks(t *testing.T) {
	t.Run("AC-13 command not registered", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{runningState("late", process.DoctorRecord{
			CommandRefusals: []process.CommandRefusal{{Command: "show alpha", Owner: "first", Reason: "already registered by process: first"}},
		})}}
		row := oneRow(t, commandRefusalRows(snapshot), server.CodeDoctorPluginCommandNotRegistered,
			diagnostic.SeverityWarning, "late", "show alpha", "first")
		if !strings.Contains(row.Fix, "first") {
			t.Fatalf("fix %q does not name the conflicting plugin", row.Fix)
		}
	})
	t.Run("AC-14 subscription dropped", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{runningState("sub", process.DoctorRecord{
			SubscriptionDrops: []process.SubscriptionDrop{{Cause: process.SubscriptionDropUnknownNamespace, Namespace: "nope"}},
		})}}
		oneRow(t, subscriptionDropRows(snapshot), server.CodeDoctorPluginSubscriptionDropped,
			diagnostic.SeverityError, "sub", "unknown event namespace", "nope")
	})
	t.Run("AC-15 delivery disagreement", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{DeliveryDisagreements: []server.DeliveryDisagreement{{
			Process: "watcher", Reason: "event delivery: the plugin declared events and no peer attaches it",
		}}}
		row := oneRow(t, deliveryDisagreementRows(snapshot), server.CodeDoctorPluginDeliveryDisagreement,
			diagnostic.SeverityWarning, "watcher")
		if !strings.Contains(row.Fix, "bus") {
			t.Fatalf("fix %q does not name the bus configuration change", row.Fix)
		}
	})
	t.Run("AC-16 claim unbacked", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{UnbackedClaims: []server.UnbackedClaim{{Role: "peer-up-replay", Claimants: []string{"bgp-rs"}}}}
		oneRow(t, unbackedClaimRows(snapshot), server.CodeDoctorPluginClaimUnbacked,
			diagnostic.SeverityError, "peer-up-replay", "bgp-rs")
	})
	t.Run("AC-17 post-startup undelivered", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
			runningState("missed", process.DoctorRecord{PostStartup: process.PostStartupUndelivered, PostStartupError: "connection closed"}),
			runningState("starting", process.DoctorRecord{PostStartup: process.PostStartupPending}),
			runningState("fresh", process.DoctorRecord{}),
		}}
		oneRow(t, postStartupRows(snapshot), server.CodeDoctorPluginPostStartupUndelivered,
			diagnostic.SeverityError, "missed", "connection closed")
	})
	t.Run("AC-18 panic", func(t *testing.T) {
		plugin := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{runningState("broken", process.DoctorRecord{ProjectionError: "delivery panic: boom"})}}
		oneRow(t, panicRows(plugin), diagnostic.CodeDoctorPluginPanic, diagnostic.SeverityError, "broken", "delivery panic: boom")
		engine := &server.DoctorSnapshot{HandlerPanics: []server.EngineHandlerPanic{{Namespace: "config", EventType: "committed", Panic: "nil map"}}}
		oneRow(t, panicRows(engine), diagnostic.CodeDoctorPluginPanic, diagnostic.SeverityError, "config", "committed", "nil map")
	})
	t.Run("AC-19 delivery failed", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
			runningState("slow", process.DoctorRecord{Delivery: process.DeliveryWindowSummary{
				Failures: 2, Slow: 1, Slowest: 501 * time.Millisecond, LastError: "broken pipe", Window: process.DeliveryWindow,
			}}),
			runningState("fine", process.DoctorRecord{Delivery: process.DeliveryWindowSummary{Window: process.DeliveryWindow}}),
		}}
		oneRow(t, deliveryFailedRows(snapshot), process.CodeDoctorPluginDeliveryFailed,
			diagnostic.SeverityWarning, "slow", "2", "501ms", "broken pipe")
	})
	t.Run("AC-20 stderr relay stopped is an event code", func(t *testing.T) {
		if !slices.Contains(diagnostic.EventCodes(), process.CodeDoctorPluginStderrRelayStopped) {
			t.Fatalf("%s is not a registered event code, so recent-events does not declare it", process.CodeDoctorPluginStderrRelayStopped)
		}
	})
	t.Run("AC-21 enricher unavailable", func(t *testing.T) {
		state := runningState("enrich", process.DoctorRecord{})
		state.Enrichers = []server.EnricherFailure{{Command: "show peer", Key: "extra", Registration: true, Error: "duplicate key"}}
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}
		oneRow(t, enricherRows(snapshot), server.CodeDoctorPluginEnricherUnavailable,
			diagnostic.SeverityWarning, "enrich", "show peer", "extra")
	})
	t.Run("AC-22 config unapplied", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{
			LastCommittedTransaction: "tx-9",
			Plugins: []server.PluginDoctorState{runningState("rib", process.DoctorRecord{
				ConfigCommit: process.ConfigCommit{Transaction: "tx-9", Error: "deadline exceeded"},
			})},
		}
		row := oneRow(t, configUnappliedRows(snapshot), server.CodeDoctorPluginConfigUnapplied,
			diagnostic.SeverityWarning, "rib", "tx-9", "deadline exceeded")
		if want := process.RestartCommand("rib"); row.Fix != want {
			t.Fatalf("fix %q, want %q", row.Fix, want)
		}
	})
	t.Run("an empty snapshot answers nothing", func(t *testing.T) {
		empty := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{runningState("quiet", process.DoctorRecord{})}}
		for _, rows := range [][]diagnostic.Diagnostic{
			commandRefusalRows(empty), subscriptionDropRows(empty), deliveryDisagreementRows(empty),
			unbackedClaimRows(empty), postStartupRows(empty), panicRows(empty), deliveryFailedRows(empty),
			enricherRows(empty), configUnappliedRows(empty),
		} {
			if len(rows) != 0 {
				t.Fatalf("a snapshot with no finding answered rows: %+v", rows)
			}
		}
	})
}

// TestDeliveryChecksAnswerNotPublished runs a registered check with no
// server published (the local route, a test).
//
// VALIDATES: the check says it could not read the plugin state, as a
// warning under its own code.
// PREVENTS: no row read as "no plugin has the problem".
func TestDeliveryChecksAnswerNotPublished(t *testing.T) {
	rows := snapshotCheck(process.CodeDoctorPluginDeliveryFailed, deliveryFailedRows)(diagnostic.DoctorCheckContext{})
	oneRow(t, rows, process.CodeDoctorPluginDeliveryFailed, diagnostic.SeverityWarning, "cannot")
}

// TestDeliveryBackpressureWaitReportedByDoctor drives a real Process to its
// eventQueue ceiling while its consumer is paused, then feeds its DoctorRecord
// into deliveryFailedRows (AC-6).
//
// VALIDATES: spec-bus-observer-delivery-decoupling AC-6 -- a producer wait at
// the queue ceiling emits a CodeDoctorPluginDeliveryFailed warning naming the
// plugin and backpressure wait count with a non-empty fix; bursts below the
// ceiling emit zero waits and zero rows; waits older than DeliveryWindow expire.
// PREVENTS: silent producer stalls at the delivery queue ceiling with no
// doctor visibility, or spurious warnings on ordinary below-ceiling bursts.
func TestDeliveryBackpressureWaitReportedByDoctor(t *testing.T) {
	const name = "slow-observer"

	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))

	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce sync.Once
	var released atomic.Bool
	releaseConsumer := func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
	}

	exited := make(chan struct{})
	go func() {
		defer close(exited)
		for req := range pluginMux.Requests() {
			enterOnce.Do(func() { close(entered) })
			select {
			case <-release:
			case <-t.Context().Done():
				return
			}
			if err := pluginMux.SendOK(t.Context(), req.ID); err != nil {
				return
			}
		}
	}()

	proc := process.NewProcess(plugin.PluginConfig{
		Name:             name,
		Encoder:          "json",
		DeliveryQueueMax: plugin.MinDeliveryQueueMax,
	})
	proc.SetConn(ipc.NewMuxPluginConn(engineMux))
	proc.StartDelivery(t.Context())
	t.Cleanup(func() {
		releaseConsumer()
		proc.Stop()
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(stopCtx); err != nil {
			t.Errorf("wait for delivery worker: %v", err)
		}
		_ = pluginMux.Close() //nolint:errcheck // test transport teardown
		<-exited
	})

	// Park deliveryLoop on the first SendDeliverBatch RPC.
	if !proc.Deliver(process.EventDelivery{Output: "ev-0"}) {
		t.Fatal("initial Deliver failed")
	}
	<-entered

	// Fill up to the ceiling (64 items) without blocking: must emit 0 waits and 0 rows.
	for range plugin.MinDeliveryQueueMax {
		if !proc.Deliver(process.EventDelivery{Output: "ev-burst"}) {
			t.Fatal("below-ceiling Deliver failed")
		}
	}
	belowRecord := proc.DoctorRecord(time.Now())
	if belowRecord.Delivery.BackpressureWaits != 0 {
		t.Fatalf("below-ceiling BackpressureWaits = %d, want 0", belowRecord.Delivery.BackpressureWaits)
	}
	belowSnap := &server.DoctorSnapshot{
		Plugins: []server.PluginDoctorState{runningState(name, belowRecord)},
	}
	if rows := deliveryFailedRows(belowSnap); len(rows) != 0 {
		t.Fatalf("below-ceiling burst emitted unexpected doctor rows: %+v", rows)
	}

	// One more DeliverContext hits the ceiling, records a backpressure wait, and
	// exits promptly on caller cancellation.
	waitCtx, cancelWait := context.WithCancel(t.Context())
	done := make(chan bool, 1)
	go func() {
		done <- proc.DeliverContext(waitCtx, process.EventDelivery{Output: "ev-ceiling"})
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for proc.DoctorRecord(time.Now()).Delivery.BackpressureWaits == 0 {
		select {
		case <-deadline.C:
			cancelWait()
			t.Fatal("timed out waiting for backpressure wait to be recorded")
		case <-ticker.C:
		}
	}
	cancelWait()
	if <-done {
		t.Fatal("canceled ceiling DeliverContext returned true, want false")
	}

	now := time.Now()
	afterRecord := proc.DoctorRecord(now)
	if afterRecord.Delivery.BackpressureWaits != 1 {
		t.Fatalf("BackpressureWaits = %d, want 1", afterRecord.Delivery.BackpressureWaits)
	}
	afterSnap := &server.DoctorSnapshot{
		Plugins: []server.PluginDoctorState{runningState(name, afterRecord)},
	}
	oneRow(t, deliveryFailedRows(afterSnap), process.CodeDoctorPluginDeliveryFailed,
		diagnostic.SeverityWarning, name, "1 producer backpressure waits at queue ceiling")

	// After DeliveryWindow elapses, the wait expires and produces 0 rows.
	expiredRecord := proc.DoctorRecord(now.Add(process.DeliveryWindow + time.Second))
	if expiredRecord.Delivery.BackpressureWaits != 0 {
		t.Fatalf("expired BackpressureWaits = %d, want 0", expiredRecord.Delivery.BackpressureWaits)
	}
	expiredSnap := &server.DoctorSnapshot{
		Plugins: []server.PluginDoctorState{runningState(name, expiredRecord)},
	}
	if rows := deliveryFailedRows(expiredSnap); len(rows) != 0 {
		t.Fatalf("expired backpressure wait emitted unexpected doctor rows: %+v", rows)
	}

	// Unblock consumer so cleanup drains cleanly.
	releaseConsumer()
	drainCtx, drainCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer drainCancel()
	if err := proc.DrainEvents(drainCtx); err != nil {
		t.Fatalf("DrainEvents after release: %v", err)
	}
}
