// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/core/probe.

package probe

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeICMPProbeUnavailable,
		Title:       "ICMP probe socket unavailable",
		Description: "Neither ICMP socket kind can be opened, so ping and traceroute cannot run. The raw ICMP socket needs CAP_NET_RAW or root, and the unprivileged datagram ICMP socket needs the daemon's group inside net.ipv4.ping_group_range. The message carries the refusal of each kind and the group the daemon runs as, so it names which of the two fixes applies. A raw socket refused for a reason other than privilege is reported as itself, and no fallback is tried.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-icmp-probe"},
	},
	{
		Code:        codeICMPProbeUnprivileged,
		Title:       "ICMP probes run on the unprivileged datagram socket",
		Description: "The raw ICMP socket is refused for privilege (no CAP_NET_RAW), and the unprivileged datagram ICMP socket stands in because the daemon's group is inside net.ipv4.ping_group_range. Ping runs on it, including the do-not-fragment modes and the reported next-hop MTU. Traceroute does not: the kernel delivers Time Exceeded only to a raw socket, so show traceroute, monitor traceroute and show probe-round refuse to run and name CAP_NET_RAW. Grant CAP_NET_RAW to the daemon to restore them.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-icmp-probe-unprivileged"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: probe: doctor code registration refused: " + err.Error())
		}
	}
}
