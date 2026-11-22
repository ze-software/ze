// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/anomaly/shape.

package shape

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorAnomalyShapeArmedNoFirewall = "doctor-anomaly-shape-armed-no-firewall"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorAnomalyShapeArmedNoFirewall,
		Title:       "Anomaly responder armed without a firewall",
		Description: "anomaly-shape is in armed mode but no firewall is configured. The responder installs live per-source rate-limit rules via the firewall component; with no firewall the armed actions cannot be applied to the kernel, so autonomous mitigation silently does nothing. Configure a firewall, or keep the responder in shadow mode.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-anomaly-shape-armed-no-firewall"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: shape: doctor code registration refused: " + err.Error())
		}
	}
}
