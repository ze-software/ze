// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ospf/v3/transport.

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorOSPFv3RawSocket = "doctor-ospfv3-raw-socket"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorOSPFv3RawSocket,
		Title:       "OSPFv3 raw IPv6 socket unavailable",
		Description: "OSPFv3 is configured but a raw IPv6 socket for protocol 89 (RFC 5340) cannot be opened. This requires CAP_NET_RAW or root; without it OSPFv3 cannot send or receive Hello, DD, LS Request, LS Update, or LS Ack packets. The raw transport is Linux-only, so on any other host the check warns with a message naming that host's OS.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-ospfv3-raw-socket"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: transport: doctor code registration refused: " + err.Error())
		}
	}
}
