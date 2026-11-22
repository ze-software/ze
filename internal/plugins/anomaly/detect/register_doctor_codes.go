// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/anomaly/detect.

package detect

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorAnomalyDetectNoFeatureSource = "doctor-anomaly-detect-no-feature-source"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorAnomalyDetectNoFeatureSource,
		Title:       "Anomaly detector has no feature source",
		Description: "anomaly-detect is enabled, but neither traffic-usage nor flow-export is configured. The trafficfeature layer is fed by the observation feed from these sources; with neither configured it produces no per-source features, so the behavioral detector observes nothing and emits no incidents. Enable traffic-usage and/or flow-export.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-anomaly-detect-no-feature-source"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: detect: doctor code registration refused: " + err.Error())
		}
	}
}
