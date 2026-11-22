// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/flowexport.

package flowexport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorFlowexportConntrackUnavailable = "doctor-flowexport-conntrack-unavailable"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorFlowexportConntrackUnavailable,
		Title:       "flow-export conntrack table unavailable",
		Description: "flow-export conntrack export is enabled but nf_conntrack is not loaded (nothing -- no firewall/NAT rule, no modprobe -- pulled it in), so the ctnetlink dump reads an empty table. The recent-flow ring stays empty and DDoS characterization degrades to generic-flood with no discriminating vector. Load nf_conntrack (a firewall or NAT rule does this) so per-flow records are available.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-flowexport-conntrack-unavailable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: flowexport: doctor code registration refused: " + err.Error())
		}
	}
}
