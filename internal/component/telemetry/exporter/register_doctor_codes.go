// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/telemetry/exporter.

package exporter

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeTelemetryProcfs names a /proc/stat the exporter cannot read.
// internal/core/diagnostic/codes.go declares it, so `ze show diagnostic
// doctor-telemetry-procfs` answers.
const codeTelemetryProcfs = "doctor-telemetry-procfs"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeTelemetryProcfs,
		Title:       "Telemetry procfs unavailable",
		Description: "Telemetry OS collectors are configured but required procfs files are not readable.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-telemetry-procfs"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: exporter: doctor code registration refused: " + err.Error())
		}
	}
}
