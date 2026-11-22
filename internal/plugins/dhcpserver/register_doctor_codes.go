// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/dhcpserver.

package dhcpserver

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDHCPIface,
		Title:       "DHCP listen interface missing",
		Description: "The DHCP server is configured to listen on an interface that does not exist on the system.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-dhcp-iface"},
	},
	{
		Code:  codeDHCPSegmentAddress,
		Title: "DHCP listen interface holds no address in a configured subnet",
		Description: "The DHCP server answers a client that no relay forwarded only from the subnets of the shared networks " +
			"the receiving interface holds an address in. This listen interface holds no IPv4 address in any configured " +
			"subnet, so it answers relayed clients only.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-dhcp-segment-address"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: dhcpserver: doctor code registration refused: " + err.Error())
		}
	}
}
