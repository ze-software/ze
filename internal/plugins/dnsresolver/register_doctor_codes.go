// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/dnsresolver.

package dnsresolver

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorDNSResolverPortUnavailable     = "doctor-dns-resolver-port-unavailable"
	codeDoctorDNSResolverUpstreamUnreachable = "doctor-dns-resolver-upstream-unreachable"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorDNSResolverPortUnavailable,
		Title:       "DNS resolver listener cannot be bound",
		Description: "The DNS resolver (service dns-resolver) is enabled but the system refuses one of its listeners permission to bind its address and port: a port below 1024, such as the default 53, needs CAP_NET_BIND_SERVICE or root. Run ze with that capability, or move the listener to a port at or above 1024. A port in use or an address not on this host is reported as doctor-listen-unavailable instead.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-dns-resolver-port-unavailable"},
	},
	{
		Code:        codeDoctorDNSResolverUpstreamUnreachable,
		Title:       "No DNS resolver upstream answers",
		Description: "The DNS resolver (service dns-resolver) forwards to the servers under forward server, and none of them answered a probe query within the timeout, so every client query would get SERVFAIL. Check the upstream addresses, and the route and firewall towards them.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-dns-resolver-upstream-unreachable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: dnsresolver: doctor code registration refused: " + err.Error())
		}
	}
}
