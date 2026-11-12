// Design: docs/architecture/doctor-and-health-checks.md -- plugin state a runtime check reads
// Related: register_lifecycle.go -- registers this check at the runtime phase
// Related: ../server/doctor_snapshot.go -- PluginDoctorSnapshot, the state it reads
// Related: ../process/stopped.go -- the stop record, its message and its fix

package doctor

import (
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fixShowHealth is the fix of a row that cannot read the state it judges:
// the command that shows what the daemon reports instead.
const fixShowHealth = "ze show health"

// pluginStoppedTimeLayout is how the row writes when the plugin went down.
const pluginStoppedTimeLayout = time.RFC3339

// checkPluginStopped answers one error per configured plugin that is down:
// its failure policy left it stopped, the respawn limit disabled it, its
// restart failed or its startup handshake failed. Each names the plugin and
// why, and its fix is the command that starts it again. A plugin a reload
// removed while it ran is not reported: the removal took its process out of
// its slot (ProcessManager.RemoveProcess), which cleared its record. One that
// was already down when a reload removed its config keeps its record, so it
// is still reported.
//
// When the snapshot cannot be read it says so, as a warning, rather than
// answering no row, which would read as every plugin running.
func checkPluginStopped(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	snapshot, err := server.PluginDoctorSnapshot()
	if err != nil {
		var message textbuf.Buffer
		message.Str("cannot tell whether every configured plugin is running: ").Err(err)
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorPluginStopped,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      fixShowHealth,
		}}
	}
	return pluginStoppedRows(&snapshot)
}

// pluginStoppedRows builds the rows from a snapshot.
func pluginStoppedRows(snapshot *server.DoctorSnapshot) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for i := range snapshot.Plugins {
		state := &snapshot.Plugins[i]
		if !state.Configured {
			continue
		}
		if !state.Down {
			continue
		}
		// A generation that reached running clears the record; one that
		// is running while the record stands is between the two writes.
		if state.Running {
			continue
		}
		rows = append(rows, pluginStoppedRow(state.Name, &state.Stop, state.CleanupOwed))
	}
	return rows
}

// daemonRestartLastResort names the one way out when a stopped process's
// cleanup never finishes: restarting the ze daemon ends every call a plugin
// made into it. `ze signal restart` alone does not bring ze back: it writes
// the graceful-restart marker and shuts ze down (internal/plugins/signal).
const daemonRestartLastResort = "if it never finishes, the last resort is to restart the ze daemon, " +
	"which ends every call a plugin made into it: systemctl restart ze under systemd " +
	"(ze signal restart writes the graceful-restart marker and shuts ze down, " +
	"so ze comes back only under a supervisor)"

// pluginStoppedRow is the row for one down plugin. While a stopped process
// of it still owes its cleanup, the restart command refuses the plugin, so
// the row says so, says the restart works once that cleanup has finished,
// and names the daemon restart as the last resort.
func pluginStoppedRow(name string, stop *process.PluginStop, cleanupOwed bool) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str(process.StoppedMessage(name, stop)).
		Str("; down since ").Str(stop.Time.UTC().Format(pluginStoppedTimeLayout))
	if cleanupOwed {
		message.Str("; the cleanup of its stopped process had not finished within the restart's bound " +
			"(it may be waiting for a call that process made into ze, or still be running), " +
			"and ze refuses to start the plugin until it has, because that cleanup " +
			"would remove what a new process registered under the name; " +
			"run the restart again once this row no longer says so; ").Str(daemonRestartLastResort)
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginStopped,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      process.RestartCommand(name),
	}
}
