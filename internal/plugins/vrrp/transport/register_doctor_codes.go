// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/vrrp/transport.

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorVRRPRawSocket = "doctor-vrrp-raw-socket"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorVRRPRawSocket,
		Title:       "VRRP raw IP socket unavailable",
		Description: "VRRP is configured but a raw IP socket for protocol 112 (RFC 9568 / RFC 3768) cannot be opened. VRRP advertisements are sent and received directly over IP protocol 112, so this requires CAP_NET_RAW or root; without it VRRP cannot send or receive advertisements and every group stays in Initialize (no failover). The transport also needs it for the gratuitous-ARP (AF_PACKET) and unsolicited-NA (raw ICMPv6) failover announcers. The raw transport is Linux-only, so on any other host the check warns with a message naming that host's OS.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-vrrp-raw-socket"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: transport: doctor code registration refused: " + err.Error())
		}
	}
}
