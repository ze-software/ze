package doctor

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
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
		row := oneRow(t, commandRefusalRows(snapshot), diagnostic.CodeDoctorPluginCommandNotRegistered,
			diagnostic.SeverityWarning, "late", "show alpha", "first")
		if !strings.Contains(row.Fix, "first") {
			t.Fatalf("fix %q does not name the conflicting plugin", row.Fix)
		}
	})
	t.Run("AC-14 subscription dropped", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{runningState("sub", process.DoctorRecord{
			SubscriptionDrops: []process.SubscriptionDrop{{Cause: process.SubscriptionDropUnknownNamespace, Namespace: "nope"}},
		})}}
		oneRow(t, subscriptionDropRows(snapshot), diagnostic.CodeDoctorPluginSubscriptionDropped,
			diagnostic.SeverityError, "sub", "unknown event namespace", "nope")
	})
	t.Run("AC-15 delivery disagreement", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{DeliveryDisagreements: []server.DeliveryDisagreement{{
			Process: "watcher", Reason: "event delivery: the plugin declared events and no peer attaches it",
		}}}
		row := oneRow(t, deliveryDisagreementRows(snapshot), diagnostic.CodeDoctorPluginDeliveryDisagreement,
			diagnostic.SeverityWarning, "watcher")
		if !strings.Contains(row.Fix, "attach process") {
			t.Fatalf("fix %q does not name the attach process change", row.Fix)
		}
	})
	t.Run("AC-16 claim unbacked", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{UnbackedClaims: []server.UnbackedClaim{{Role: "peer-up-replay", Claimants: []string{"bgp-rs"}}}}
		oneRow(t, unbackedClaimRows(snapshot), diagnostic.CodeDoctorPluginClaimUnbacked,
			diagnostic.SeverityError, "peer-up-replay", "bgp-rs")
	})
	t.Run("AC-17 post-startup undelivered", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
			runningState("missed", process.DoctorRecord{PostStartup: process.PostStartupUndelivered, PostStartupError: "connection closed"}),
			runningState("starting", process.DoctorRecord{PostStartup: process.PostStartupPending}),
			runningState("fresh", process.DoctorRecord{}),
		}}
		oneRow(t, postStartupRows(snapshot), diagnostic.CodeDoctorPluginPostStartupUndelivered,
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
		oneRow(t, deliveryFailedRows(snapshot), diagnostic.CodeDoctorPluginDeliveryFailed,
			diagnostic.SeverityWarning, "slow", "2", "501ms", "broken pipe")
	})
	t.Run("AC-20 stderr relay stopped is an event code", func(t *testing.T) {
		if !slices.Contains(diagnostic.EventCodes(), diagnostic.CodeDoctorPluginStderrRelayStopped) {
			t.Fatalf("%s is not a registered event code, so recent-events does not declare it", diagnostic.CodeDoctorPluginStderrRelayStopped)
		}
	})
	t.Run("AC-21 enricher unavailable", func(t *testing.T) {
		state := runningState("enrich", process.DoctorRecord{})
		state.Enrichers = []server.EnricherFailure{{Command: "show peer", Key: "extra", Registration: true, Error: "duplicate key"}}
		snapshot := &server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}
		oneRow(t, enricherRows(snapshot), diagnostic.CodeDoctorPluginEnricherUnavailable,
			diagnostic.SeverityWarning, "enrich", "show peer", "extra")
	})
	t.Run("AC-22 config unapplied", func(t *testing.T) {
		snapshot := &server.DoctorSnapshot{
			LastCommittedTransaction: "tx-9",
			Plugins: []server.PluginDoctorState{runningState("rib", process.DoctorRecord{
				ConfigCommit: process.ConfigCommit{Transaction: "tx-9", Error: "deadline exceeded"},
			})},
		}
		row := oneRow(t, configUnappliedRows(snapshot), diagnostic.CodeDoctorPluginConfigUnapplied,
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
	rows := snapshotCheck(diagnostic.CodeDoctorPluginDeliveryFailed, deliveryFailedRows)(diagnostic.DoctorCheckContext{})
	oneRow(t, rows, diagnostic.CodeDoctorPluginDeliveryFailed, diagnostic.SeverityWarning, "cannot")
}
