// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/as112.

package as112

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorAS112PortUnavailable = "doctor-as112-port-unavailable"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorAS112PortUnavailable,
		Title:       "AS112 port 53 cannot be bound",
		Description: "AS112 is enabled but the daemon cannot bind its fixed UDP/TCP port 53 (RFC 7534 Section 3.5). The port needs CAP_NET_BIND_SERVICE or root, or it is already in use by another service (e.g. GeoDNS on the same address).",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-as112-port-unavailable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: as112: doctor code registration refused: " + err.Error())
		}
	}
}
