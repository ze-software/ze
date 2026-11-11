// Design: docs/architecture/doctor-and-health-checks.md -- plugin registration and delivery checks
// Related: register_delivery.go -- registers these checks at the runtime phase
// Related: ../server/doctor_snapshot.go -- PluginDoctorSnapshot, the state they read
// Related: ../process/doctor_records.go -- the per-process records
//
// The runtime checks of the registration and delivery classes. Each reads the
// running server's plugin snapshot and answers one row per finding, with the
// plugin, what failed and the fix. A row builder is pure over the snapshot,
// so a test hands it a snapshot holding the finding.

package doctor

import (
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fixReload is the fix of a plugin that missed a committed transaction.
const fixReload = "ze signal reload"

// snapshotCheck adapts a row builder into a runtime check. When the snapshot
// cannot be read it says so under the check's code, as a warning, rather
// than answering no row, which would read as no plugin having the problem.
func snapshotCheck(code string, rows func(*server.DoctorSnapshot) []diagnostic.Diagnostic) func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
		snapshot, err := server.PluginDoctorSnapshot()
		if err != nil {
			var message textbuf.Buffer
			message.Str("cannot read the plugin state this check reports on: ").Err(err)
			return []diagnostic.Diagnostic{{
				Code:     code,
				Severity: diagnostic.SeverityWarning,
				Message:  message.String(),
				Fix:      "ze show health",
			}}
		}
		return rows(&snapshot)
	}
}

// withProcess calls each for every plugin whose process the manager holds:
// the per-process records exist only there.
func withProcess(snapshot *server.DoctorSnapshot, each func(state *server.PluginDoctorState)) {
	for i := range snapshot.Plugins {
		state := &snapshot.Plugins[i]
		if !state.HasProcess {
			continue
		}
		each(state)
	}
}

// commandRefusalRows answers one warning per command or alias a plugin
// declared and the registry refused (AC-13).
func commandRefusalRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	withProcess(snapshot, func(state *server.PluginDoctorState) {
		for i := range state.Records.CommandRefusals {
			rows = append(rows, commandRefusalRow(state.Name, &state.Records.CommandRefusals[i]))
		}
		if dropped := state.Records.CommandRefusalsDropped; dropped != 0 {
			var message textbuf.Buffer
			message.Str("plugin ").Str(state.Name).Str(" had ").Int(int64(dropped)).
				Str(" more refused commands than the doctor keeps; the log lists them")
			rows = append(rows, diagnostic.Diagnostic{
				Code: diagnostic.CodeDoctorPluginCommandNotRegistered, Severity: diagnostic.SeverityWarning,
				Message: message.String(), Fix: "ze show warnings",
			})
		}
	})
	return rows
}

// commandRefusalRow is the row for one refused command.
func commandRefusalRow(name string, refusal *process.CommandRefusal) diagnostic.Diagnostic {
	kind := "command"
	if refusal.Alias {
		kind = "deprecated alias"
	}
	var message, fix textbuf.Buffer
	message.Str("plugin ").Str(name).Str(" declared the ").Str(kind).Str(" `").Str(refusal.Command).
		Str("` and it was not registered: ").Str(refusal.Reason)
	switch refusal.Owner {
	case "":
		fix.Str("run a version of plugin ").Str(name).Str(" whose ").Str(kind).Str(" names are valid")
	case server.CommandOwnerBuiltin:
		fix.Str("`").Str(refusal.Command).Str("` is a ze builtin: remove plugin ").Str(name).
			Str(" from the config, or run a version of it that does not declare the name")
	default:
		message.Str(" (owned by plugin ").Str(refusal.Owner).Str(")")
		fix.Str("remove or rename plugin ").Str(refusal.Owner).Str(" or plugin ").Str(name).
			Str(" in the config, so one plugin owns `").Str(refusal.Command).Str("`")
	}
	return diagnostic.Diagnostic{
		Code: diagnostic.CodeDoctorPluginCommandNotRegistered, Severity: diagnostic.SeverityWarning,
		Message: message.String(), Fix: fix.String(),
	}
}

// subscriptionDropRows answers one error per dropped subscription block
// (AC-14).
func subscriptionDropRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	withProcess(snapshot, func(state *server.PluginDoctorState) {
		for i := range state.Records.SubscriptionDrops {
			drop := &state.Records.SubscriptionDrops[i]
			var message textbuf.Buffer
			message.Str("plugin ").Str(state.Name).Str(" receives none of the events of a subscription the daemon dropped: ").
				Str(drop.Cause.String())
			if drop.Namespace != "" {
				message.Str(" `").Str(drop.Namespace).Str("`")
			}
			if drop.Detail != "" {
				message.Str(" (").Str(drop.Detail).Str(")")
			}
			rows = append(rows, diagnostic.Diagnostic{
				Code: diagnostic.CodeDoctorPluginSubscriptionDropped, Severity: diagnostic.SeverityError,
				Message: message.String(), Fix: subscriptionDropFix(state.Name, drop.Cause),
			})
		}
	})
	return rows
}

// subscriptionDropFix names what makes the subscription register.
func subscriptionDropFix(name string, cause process.SubscriptionDropCause) string {
	var fix textbuf.Buffer
	switch cause {
	case process.SubscriptionDropNoDefaultNamespace:
		fix.Str("run a ze build that carries the protocol component whose events plugin ").Str(name).Str(" subscribes to")
	case process.SubscriptionDropUnknownNamespace, process.SubscriptionDropReadyParamsUnparsable, process.SubscriptionDropUnspecified:
		fix.Str("run a version of plugin ").Str(name).Str(" that subscribes in a namespace this ze build registers, then ").
			Str(process.RestartCommand(name))
	}
	return fix.String()
}

// deliveryDisagreementRows answers one warning per disagreement between an
// `attach process` grant and the events a plugin declared (AC-15).
func deliveryDisagreementRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(snapshot.DeliveryDisagreements))
	for i := range snapshot.DeliveryDisagreements {
		d := &snapshot.DeliveryDisagreements[i]
		var message textbuf.Buffer
		message.Str(d.Reason).Str(": process ").Str(d.Process)
		if d.Peer != "" {
			message.Str(", peer ").Str(d.Peer)
		}
		if d.Granted != "" {
			message.Str(", granted ").Str(d.Granted)
		}
		if d.Declared != "" {
			message.Str(", declared ").Str(d.Declared)
		}
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginDeliveryDisagreement, Severity: diagnostic.SeverityWarning,
			Message: message.String(), Fix: d.Fix(),
		})
	}
	return rows
}

// unbackedClaimRows answers one error per exclusive role claimed with no
// claimant running (AC-16).
func unbackedClaimRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(snapshot.UnbackedClaims))
	for i := range snapshot.UnbackedClaims {
		claim := &snapshot.UnbackedClaims[i]
		var message textbuf.Buffer
		message.Str("exclusive role ").Str(claim.Role).Str(" is claimed by ").Str(strings.Join(claim.Claimants, ", ")).
			Str(" and no claimant is running, so the plugin that stood down for it does not perform the role")
		fix := "ze show plugin list"
		if len(claim.Claimants) != 0 {
			fix = process.RestartCommand(claim.Claimants[0])
		}
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginClaimUnbacked, Severity: diagnostic.SeverityError,
			Message: message.String(), Fix: fix,
		})
	}
	return rows
}

// postStartupRows answers one error per running plugin whose post-startup
// callback failed (AC-17). A plugin still starting, or whose callback is in
// flight, has nothing recorded undelivered and is not reported.
func postStartupRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	withProcess(snapshot, func(state *server.PluginDoctorState) {
		if !state.Running {
			return
		}
		if state.Records.PostStartup != process.PostStartupUndelivered {
			return
		}
		var message textbuf.Buffer
		message.Str("plugin ").Str(state.Name).
			Str(" never received its post-startup callback, so the work it starts after startup is not running: ").
			Str(state.Records.PostStartupError)
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginPostStartupUndelivered, Severity: diagnostic.SeverityError,
			Message: message.String(), Fix: process.RestartCommand(state.Name),
		})
	})
	return rows
}

// panicRows answers one error per plugin whose event application failed and
// one per recovered engine handler panic (AC-18).
func panicRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	withProcess(snapshot, func(state *server.PluginDoctorState) {
		if state.Records.ProjectionError == "" {
			return
		}
		var message textbuf.Buffer
		message.Str("plugin ").Str(state.Name).Str(" failed to apply an event, so its state may lack it: ").
			Str(state.Records.ProjectionError)
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginPanic, Severity: diagnostic.SeverityError,
			Message: message.String(), Fix: process.RestartCommand(state.Name),
		})
	})
	for i := range snapshot.HandlerPanics {
		p := &snapshot.HandlerPanics[i]
		var message textbuf.Buffer
		message.Str("the engine handler for ").Str(p.Namespace).Str(" event ").Str(p.EventType).
			Str(" panicked and was recovered, so it missed that event: ").Str(p.Panic)
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginPanic, Severity: diagnostic.SeverityError,
			Message: message.String(), Fix: "ze show errors",
		})
	}
	return rows
}

// deliveryFailedRows answers one warning per plugin with failed or slow
// event batches inside the recent delivery window (AC-19).
func deliveryFailedRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	withProcess(snapshot, func(state *server.PluginDoctorState) {
		window := &state.Records.Delivery
		if window.Failures == 0 && window.Slow == 0 {
			return
		}
		var message textbuf.Buffer
		message.Str("plugin ").Str(state.Name).Str(" in the last ").Str(window.Window.String()).Str(": ").
			Int(int64(window.Failures)).Str(" event batches failed with no caller waiting, ").
			Int(int64(window.Slow)).Str(" took over ").Str(process.DeliverySlowThreshold.String())
		if window.Slow != 0 {
			message.Str(" (slowest ").Str(window.Slowest.String()).Str(")")
		}
		if window.LastError != "" {
			message.Str("; last error: ").Str(window.LastError)
		}
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginDeliveryFailed, Severity: diagnostic.SeverityWarning,
			Message: message.String(), Fix: process.RestartCommand(state.Name),
		})
	})
	return rows
}

// enricherRows answers one warning per `show` enricher of a plugin that
// failed to register or whose last call failed (AC-21).
func enricherRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for i := range snapshot.Plugins {
		state := &snapshot.Plugins[i]
		for j := range state.Enrichers {
			failure := &state.Enrichers[j]
			var message textbuf.Buffer
			message.Str("plugin ").Str(state.Name).Str("'s field ").Str(failure.Key).Str(" of `").Str(failure.Command).Str("` ")
			fix := process.RestartCommand(state.Name)
			if failure.Registration {
				message.Str("failed to register, so the command answers without it: ")
				fix = "remove from the config the other plugin that adds this field to the command"
			} else {
				message.Str("failed on its last call, so the command answered without it: ")
			}
			message.Str(failure.Error)
			rows = append(rows, diagnostic.Diagnostic{
				Code: diagnostic.CodeDoctorPluginEnricherUnavailable, Severity: diagnostic.SeverityWarning,
				Message: message.String(), Fix: fix,
			})
		}
	}
	return rows
}

// configUnappliedRows answers one warning per running plugin that was not
// told its last transaction committed (AC-22).
func configUnappliedRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	withProcess(snapshot, func(state *server.PluginDoctorState) {
		if !state.Running {
			return
		}
		commit := &state.Records.ConfigCommit
		if commit.Error == "" {
			return
		}
		var message textbuf.Buffer
		message.Str("plugin ").Str(state.Name).Str(" was not told transaction ").Str(commit.Transaction).
			Str(" committed (").Str(commit.Error).Str("); the daemon's last committed transaction is ").
			Str(snapshot.LastCommittedTransaction).
			Str(", and the plugin keeps the undo set of the one it missed")
		rows = append(rows, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPluginConfigUnapplied, Severity: diagnostic.SeverityWarning,
			Message: message.String(), Fix: fixReload,
		})
	})
	return rows
}
