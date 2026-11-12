// Design: docs/architecture/doctor-and-health-checks.md -- reload and stop records a check reads
// Related: register_reload_dataplane.go -- registers both checks at the runtime phase
// Related: ../server/doctor_reload_records.go -- PluginReloadRecords, the state they read

package doctor

import (
	"time"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// reloadRetryCommand is the fix of both rows: a reload retries an unrestored
// removal first (Server.retryRemovalRecovery), and a committed reload
// installs the running configuration's capability declarations again.
const reloadRetryCommand = "ze request reload"

// checkRemovalUnrecovered answers one error per plugin a refused reload
// removed and could not restore.
func checkRemovalUnrecovered(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	records, err := server.PluginReloadRecords()
	if err != nil {
		return []diagnostic.Diagnostic{reloadRecordsUnread(diagnostic.CodeDoctorPluginRemovalUnrecovered, err)}
	}
	return removalUnrecoveredRows(&records)
}

// checkCapabilitiesNotRestored answers one error when a refused reload left
// capability declarations it could not restore, until a reload commits.
func checkCapabilitiesNotRestored(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	records, err := server.PluginReloadRecords()
	if err != nil {
		return []diagnostic.Diagnostic{reloadRecordsUnread(diagnostic.CodeDoctorPluginCapabilitiesNotRestored, err)}
	}
	return capabilitiesNotRestoredRows(&records)
}

// reloadRecordsUnread says the records could not be read, rather than
// answering no row, which would read as every reload restored.
func reloadRecordsUnread(code string, err error) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str("cannot tell whether a refused reload left plugin state unrestored: ").Err(err)
	return diagnostic.Diagnostic{Code: code, Severity: diagnostic.SeverityWarning, Message: message.String(), Fix: fixShowHealth}
}

// removalUnrecoveredRows builds one row per unrestored removal.
func removalUnrecoveredRows(records *server.ReloadDoctorRecords) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(records.RemovalsUnrecovered))
	for _, removal := range records.RemovalsUnrecovered {
		var message textbuf.Buffer
		message.Str("plugin ").Str(removal.Plugin).
			Str(" was removed by a reload that was then refused, and restoring it failed, so it stays removed: ").
			Str(removal.Err)
		rows = append(rows, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorPluginRemovalUnrecovered,
			Severity: diagnostic.SeverityError,
			Message:  message.String(),
			Fix:      reloadRetryCommand,
		})
	}
	return rows
}

// capabilitiesNotRestoredRows builds the row for a failed capability restore.
func capabilitiesNotRestoredRows(records *server.ReloadDoctorRecords) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(records.CapabilitiesNotRestored))
	for _, failure := range records.CapabilitiesNotRestored {
		var message textbuf.Buffer
		message.Str("a reload refused at ").Str(failure.Time.UTC().Format(time.RFC3339)).
			Str(" could not restore the plugins' capability declarations, so a BGP OPEN built since may carry ").
			Str("what the refused configuration declared: ").Str(failure.Err)
		rows = append(rows, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorPluginCapabilitiesNotRestored,
			Severity: diagnostic.SeverityError,
			Message:  message.String(),
			Fix:      reloadRetryCommand,
		})
	}
	return rows
}
