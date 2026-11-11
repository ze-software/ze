// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase
// Related: register.go -- registers this check at the runtime phase
// Related: ../../../core/report/report.go -- Losses, the eviction count this check reads

package doctor

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// reportBusCheckName is the runtime check that reports a report bus that
// dropped entries.
const reportBusCheckName = "report-bus-capacity"

// checkReportBusFull answers one warning per kind of loss the daemon's report
// bus has had: active warnings evicted at the warning cap, and error events
// the ring overwrote. Each names the cap, how many entries were lost and the
// export that raises the cap. It reads the bus of the process it runs in, so
// it is registered at the runtime phase, which runs only in the daemon.
func checkReportBusFull(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	loss := report.Losses()
	var rows []diagnostic.Diagnostic
	if loss.WarningsEvicted > 0 {
		rows = append(rows, busFullRow("active warnings", "show warnings", loss.WarningsEvicted,
			loss.WarningCap, loss.WarningCapMax, "ze.report.warnings.max", "ZE_REPORT_WARNINGS_MAX"))
	}
	if loss.ErrorsOverwritten > 0 {
		rows = append(rows, busFullRow("error events", "show errors", loss.ErrorsOverwritten,
			loss.ErrorCap, loss.ErrorCapMax, "ze.report.errors.max", "ZE_REPORT_ERRORS_MAX"))
	}
	return rows
}

// busFullRow builds the row for one kind of loss. The fix doubles the cap up
// to the largest one the env key accepts; at that largest cap no export helps,
// so the fix lists what is filling the bus instead.
func busFullRow(kind, show string, lost uint64, capacity, capacityMax int, key, export string) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str("the report bus dropped ").Uint(lost).Str(" ").Str(kind).
		Str(" at its cap of ").Int(int64(capacity)).
		Str(", so ").Str(show).Str(" and the health rows that read the bus no longer see them; the cap is ").
		Str(key)

	var fix textbuf.Buffer
	if capacity >= capacityMax {
		message.Str(", already at its largest value ").Int(int64(capacityMax))
		fix.Str("ze ").Str(show)
	} else {
		fix.Str("export ").Str(export).Str("=").Int(int64(min(2*capacity, capacityMax))).
			Str(" before starting ze")
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorReportBusFull,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}
