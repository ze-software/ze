// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/geodns.

package geodns

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorGeoDNSPortUnavailable = "doctor-geodns-port-unavailable"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorGeoDNSPortUnavailable,
		Title:       "GeoDNS privileged listen-port cannot be bound",
		Description: "GeoDNS is configured to listen on a privileged port (<1024, e.g. 53) but the daemon cannot bind it for UDP/TCP. The port needs CAP_NET_BIND_SERVICE or root, or it is already in use. The default port 5300 is unprivileged and not affected.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-geodns-port-unavailable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: geodns: doctor code registration refused: " + err.Error())
		}
	}
}
