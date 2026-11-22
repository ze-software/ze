// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ldp.

package ldp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorLDPPortUnavailable = "doctor-ldp-port-unavailable"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorLDPPortUnavailable,
		Title:       "LDP port 646 cannot be bound",
		Description: "LDP is configured but the daemon cannot bind UDP/TCP port 646 (RFC 5036). The port is privileged (<1024) and may need CAP_NET_BIND_SERVICE or root, or it is already in use by another process.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-ldp-port-unavailable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ldp: doctor code registration refused: " + err.Error())
		}
	}
}
