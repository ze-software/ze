// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/telemetry/exporter.

package exporter

import "github.com/ze-software/ze/internal/core/diagnostic"

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
