// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/dhcpserver.

package dhcpserver

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorDHCPReplyWriteFailed = "doctor-dhcp-reply-write-failed"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDHCPIface,
		Title:       "DHCP listen interface missing",
		Description: "The DHCP server is configured to listen on an interface that does not exist on the system.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeDHCPIface},
	},
	{
		Code:  codeDHCPSegmentAddress,
		Title: "DHCP listen interface holds no address in a configured subnet",
		Description: "The DHCP server answers a client that no relay forwarded only from the subnets of the shared networks " +
			"the receiving interface holds an address in. This listen interface holds no IPv4 address in any configured " +
			"subnet, so it answers relayed clients only.",
		Examples: []string{exampleDoctorJSON, exampleShowDiagPrefix + codeDHCPSegmentAddress},
	},
	{
		Code:  codeDoctorDHCPReplyWriteFailed,
		Title: "DHCP server could not send a reply packet",
		Description: "Writing a DHCP reply UDP datagram on the listen interface failed, so the requesting client did not receive " +
			"its offer, acknowledgment, or refusal and must retransmit. Check link state, addressing, firewall policy, and " +
			"socket permissions on the listen interface with `ze show interface`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorDHCPReplyWriteFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorDHCPReplyWriteFailed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: dhcpserver: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: dhcpserver: doctor event code registration refused: " + err.Error())
		}
	}
}
