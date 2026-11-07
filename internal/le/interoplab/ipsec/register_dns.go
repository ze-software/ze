// Design: docs/architecture/testing/interop.md -- the IPsec lab name server.
// Related: dns.go -- the container, dns.conf and the late start.
// Related: fqdn.go -- the two checkers registered here.
package ipsec

import "github.com/ze-software/ze/internal/le/interoplab"

// The name server's image is built only when a planned scenario carries a dns.conf
// (runAt), as the NAT box's is for nat.conf. The two scenarios dial strongSwan by
// name: one boots Ze while DNS is down and with no local address, the other crosses
// the NAT box (spec-home-cpe-4-ike-dynamic-wan).
func init() {
	interoplab.RegisterImage(ipsecImage(dnsImage, "Dockerfile.dns", true))
	scenarioCheckers["fqdn-remote-dynamic-local"] = scenarioCheck{run: checkFQDNRemoteDynamicLocal}
	scenarioCheckers["fqdn-remote-real-nat"] = scenarioCheck{run: checkFQDNRemoteRealNAT}
}
