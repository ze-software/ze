// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/isis/transport.

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorISISRawSocket = "doctor-isis-raw-socket"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorISISRawSocket,
		Title:       "IS-IS raw L2 socket unavailable",
		Description: "IS-IS is configured but a raw AF_PACKET/SOCK_RAW socket cannot be opened. IS-IS runs directly over IEEE 802.3 frames (ISO/IEC 10589), so it needs CAP_NET_RAW or root; without it IS-IS cannot send or receive IIH/LSP/CSNP/PSNP PDUs and forms no adjacencies. The raw transport is Linux-only, so on any other host the check warns with a message naming that host's OS.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-isis-raw-socket"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: transport: doctor code registration refused: " + err.Error())
		}
	}
}
