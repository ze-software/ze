// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ospf/transport.

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorOSPFRawSocket = "doctor-ospf-raw-socket"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorOSPFRawSocket,
		Title:       "OSPF raw IP socket unavailable",
		Description: "OSPF is configured but a raw IP socket for protocol 89 (RFC 2328) cannot be opened. This requires CAP_NET_RAW or root; without it OSPF cannot send or receive Hello, DD, LS Request, LS Update, or LS Ack packets. The raw transport is Linux-only, so on any other host the check warns with a message naming that host's OS.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-ospf-raw-socket"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: transport: doctor code registration refused: " + err.Error())
		}
	}
}
