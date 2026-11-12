package doctor

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestBoundedRecordsNameWhatTheyDropped hands each row builder a snapshot
// whose bounded record dropped entries past its bound.
//
// VALIDATES: the enricher, subscription and engine handler panic rows each
// add a row naming how many more failed than the doctor lists, and the
// delivery row says its counts are a lower bound when the ring overwrote a
// problem inside the window.
// PREVENTS: a row that lists the first entries and reads as the whole story
// while the producer counted, or overwrote, the rest.
//
// The rows past the bound name the plugin server's recent log, at the level
// each entry was logged: those entries are only logged, never raised on the
// report bus, so `ze show warnings` and `ze show errors` show none of them.
func TestBoundedRecordsNameWhatTheyDropped(t *testing.T) {
	const (
		loggedWarnings = "ze show log recent level warn component plugin.server"
		loggedErrors   = "ze show log recent level err component plugin.server"
	)
	t.Run("enrichers", func(t *testing.T) {
		state := server.PluginDoctorState{Name: "rich", Configured: true, EnrichersDropped: 3}
		row := oneRow(t, enricherRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}),
			diagnostic.CodeDoctorPluginEnricherUnavailable, diagnostic.SeverityWarning, "rich", "3 more", "logged")
		if row.Fix != loggedWarnings {
			t.Fatalf("the dropped-enricher row's fix %q does not list the logged warnings", row.Fix)
		}
	})
	t.Run("subscriptions", func(t *testing.T) {
		state := runningState("subscriber", process.DoctorRecord{SubscriptionDropsDropped: 4})
		row := oneRow(t, subscriptionDropRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}),
			diagnostic.CodeDoctorPluginSubscriptionDropped, diagnostic.SeverityError, "subscriber", "4 more", "logged")
		if row.Fix != loggedWarnings {
			t.Fatalf("the dropped-subscription row's fix %q does not list the logged warnings", row.Fix)
		}
	})
	t.Run("handler panics", func(t *testing.T) {
		row := oneRow(t, panicRows(&server.DoctorSnapshot{HandlerPanicsTruncated: true}),
			diagnostic.CodeDoctorPluginPanic, diagnostic.SeverityError, "more", server.HandlerPanicWindow.String(), "logged")
		if row.Fix != loggedErrors {
			t.Fatalf("the truncated handler panic row's fix %q does not list the logged errors", row.Fix)
		}
	})
	t.Run("command refusals", func(t *testing.T) {
		state := runningState("commander", process.DoctorRecord{CommandRefusalsDropped: 2})
		row := oneRow(t, commandRefusalRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}),
			diagnostic.CodeDoctorPluginCommandNotRegistered, diagnostic.SeverityWarning, "commander", "2 more", "logged")
		if row.Fix != loggedWarnings {
			t.Fatalf("the dropped command refusal row's fix %q does not list the logged warnings", row.Fix)
		}
	})
	t.Run("delivery window", func(t *testing.T) {
		records := process.DoctorRecord{Delivery: process.DeliveryWindowSummary{
			Failures: 64, Window: process.DeliveryWindow, Truncated: true,
		}}
		state := runningState("busy", records)
		oneRow(t, deliveryFailedRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}),
			diagnostic.CodeDoctorPluginDeliveryFailed, diagnostic.SeverityWarning, "busy", "at least")
		records.Delivery.Truncated = false
		state = runningState("busy", records)
		row := oneRow(t, deliveryFailedRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{state}}),
			diagnostic.CodeDoctorPluginDeliveryFailed, diagnostic.SeverityWarning, "busy")
		if strings.Contains(row.Message, "at least") {
			t.Fatalf("a window that kept every problem reads as a lower bound: %s", row.Message)
		}
	})
}

// TestConfigUnappliedFixRestartsThePlugin builds the AC-22 row.
//
// VALIDATES: the fix names `ze request plugin restart <name>`, the command
// that replaces the process (its record starts clean) and configures the
// replacement from the daemon's committed config.
// PREVENTS: the row naming `ze signal reload`, which sends no transaction for
// an unchanged config and so never clears the record (reloadConfig answers
// "no changes" before any participant is told anything).
func TestConfigUnappliedFixRestartsThePlugin(t *testing.T) {
	state := runningState("rib", process.DoctorRecord{ConfigCommit: process.ConfigCommit{
		Transaction: "tx-1", Error: "context deadline exceeded",
	}})
	row := oneRow(t, configUnappliedRows(&server.DoctorSnapshot{
		Plugins: []server.PluginDoctorState{state}, LastCommittedTransaction: "tx-2",
	}), diagnostic.CodeDoctorPluginConfigUnapplied, diagnostic.SeverityWarning, "rib", "tx-1", "tx-2")
	if row.Fix != process.RestartCommand("rib") {
		t.Fatalf("fix %q, want %q", row.Fix, process.RestartCommand("rib"))
	}
}
