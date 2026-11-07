// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: check_pppoe_dhcpv6_pd.go -- the checker this file registers.
// Related: register_pppoe_mss_pmtu.go -- the accel-ppp image and CPE node it shares.
package topology

import "github.com/ze-software/ze/internal/le/interoplab"

// pppoeDHCPv6PDScenario is spec-home-cpe-8's scenario: the Ze PPPoE client
// on IPv6 against accel-ppp, for Phases 1 to 3 only (IPv6CP, the WAN RA and
// SLAAC, DHCPv6 prefix delegation on pppoe0). The delegated prefix's use on a
// LAN (Phases 4 and 5) waits on the owner, so no LAN node runs yet.
const pppoeDHCPv6PDScenario = "pppoe-dhcpv6-pd-ze-client"

// Values accel-ppp.conf and the checker share. 2001:db8::/32 is the
// documentation range (RFC 3849).
const (
	// pdAccelLinkLocal is accel-ppp's link-local address on the session,
	// from its ipv6-intf-id 0:0:0:1: the RA's source and so the CPE's
	// default router.
	pdAccelLinkLocal = "fe80::1"
	// pdSLAACPool is accel-ppp's ipv6-pool, whose /64 its RA carries.
	pdSLAACPool = "2001:db8:198::/48"
	// pdDelegatePool and pdDelegatedLength are its delegate pool and the
	// length it delegates, which differs from the hint pdHintLength.
	pdDelegatePool    = "2001:db8:2000::/40"
	pdDelegatedLength = 60
	pdHintLength      = 56
	// pdAccelLog is the file accel-ppp logs to ([log] log-file).
	pdAccelLog = "/var/log/accel-ppp.log"
	// pdAccelChapLines is the chap-secrets line PAP reads: ze-cpe.conf's
	// credentials.
	pdAccelChapLines = "alice * s3cr3t *"
)

// The node arguments both peers take: IPv6 on, which a lab container starts
// with off.
var ipv6On = []string{"--sysctl", "net.ipv6.conf.all.disable_ipv6=0"}

// ze-cpe ==PPPoE (link access, VLAN 198, no prefix)== accel. The CPE starts
// after accel-ppp, so the concentrator answers its first PADI.
func init() {
	scenarios[pppoeDHCPv6PDScenario] = scenario{
		nodes: []node{
			{name: nodeAccel, image: accelImage, host: 2, config: "accel-ppp.conf", mountedAt: "/etc/accel-ppp.conf",
				capabilities: []string{capabilityNetAdmin},
				arguments:    append(interoplab.PPPDeviceArguments(), ipv6On...),
				prestart:     "mkdir -p /etc/accel-ppp /var/log; printf '%s\\n' '" + pdAccelChapLines + "' > /etc/accel-ppp/chap-secrets"},
			zeNode(nodeCPE, 3, "ze-cpe.conf", append(interoplab.PPPDeviceArguments(), ipv6On...)...),
		},
		links: []interoplab.Link{
			{Name: accessLink, VLAN: 198, Members: []string{nodeAccel, nodeCPE}},
		},
		checker: checkPPPoEDHCPv6PD,
	}
}
