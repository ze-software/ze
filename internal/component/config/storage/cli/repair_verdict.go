// Design: docs/architecture/storage-backends.md -- salvage verdict on every route.
package cli

import (
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// repairVerdict grades the frame salvage and repaired history once for both
// routes. A dropped key is reported loss; an unresolved error remains failure.
type repairVerdict struct {
	report     *zefs.RepairReport
	findings   []storage.HistoryFinding
	dropped    int
	unresolved int
}

func newRepairVerdict(report *zefs.RepairReport, findings []storage.HistoryFinding) repairVerdict {
	verdict := repairVerdict{report: report, findings: findings}
	for _, finding := range findings {
		if historyDropped(finding) {
			verdict.dropped++
			continue
		}
		if finding.Severity != storage.HistorySeverityWarning {
			verdict.unresolved++
		}
	}
	return verdict
}

// printHistory retains the standalone diagnostic lines while using the same
// grading that decides the daemon response. The history walk bounds the rows.
func (verdict repairVerdict) printHistory() {
	for _, finding := range verdict.findings {
		var text textbuf.Buffer
		if historyDropped(finding) {
			text.Str("dropped: ")
		} else {
			text.Str(repairSeverity(finding.Severity)).Str(": ")
		}
		text.Str(finding.Kind).Str(": ").Str(finding.Key).Str(": ").Str(finding.Detail).Byte('\n')
		if finding.Severity == storage.HistorySeverityWarning {
			text.StdOut() //nolint:errcheck // CLI diagnostic
		} else {
			text.StdErr() //nolint:errcheck // CLI diagnostic
		}
	}
}

func (verdict repairVerdict) exitCode() int {
	if verdict.report.SkippedCount > 0 {
		return 1
	}
	if verdict.dropped+verdict.unresolved > 0 {
		return 1
	}
	return 0
}

func repairSeverity(severity storage.HistorySeverity) string {
	switch severity {
	case storage.HistorySeverityUnspecified, storage.HistorySeverityError:
		return "error"
	case storage.HistorySeverityWarning:
		return "warning"
	default:
		panic("BUG: unknown internally produced history severity")
	}
}
