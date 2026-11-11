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

// pluginStoppedTimeLayout is how the row writes when the plugin went down.
const pluginStoppedTimeLayout = time.RFC3339

// checkPluginStopped answers one error per configured plugin that is down:
// its failure policy left it stopped, the respawn limit disabled it, its
// restart failed or its startup handshake failed. Each names the plugin and
// why, and its fix is the command that starts it again. A plugin removed by
// config is not reported: the removal cleared its record.
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
			Fix:      "ze show health",
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
		rows = append(rows, pluginStoppedRow(state.Name, &state.Stop))
	}
	return rows
}

// pluginStoppedRow is the row for one down plugin.
func pluginStoppedRow(name string, stop *process.PluginStop) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str(process.StoppedMessage(name, stop)).
		Str("; down since ").Str(stop.Time.UTC().Format(pluginStoppedTimeLayout))
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorPluginStopped,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      process.RestartCommand(name),
	}
}
