// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_wholesale_accel.go -- the checker this file registers.
// Related: register_wholesale.go -- the realm-relaying LAC this scenario reuses.
package topology

import "github.com/ze-software/ze/internal/le/interoplab"

// accessWholesaleAccelScenario is spec-home-cpe-12's closure scenario: the
// converted Ze CPE behind the realm-relaying Ze LAC, with accel-ppp as the
// LNS. accel-ppp ignores the proxy LCP AVPs (l2tp.c l2tp_recv_ICCN breaks out
// of Init_Recv_LCP, Last_Sent_LCP, Last_Recv_LCP and every Proxy_Authen_*
// AVP), so it always runs its own LCP and CHAP with a subscriber the LAC
// already took to LCP Opened: the wholesale renegotiation, by third-party
// software.
const accessWholesaleAccelScenario = "access-wholesale-accel-lns"

// nodeAccelLNS is accel-ppp as the LNS, on link core at lnsCoreAddress, the
// address the scenario's LAC dials.
const nodeAccelLNS = "accel-lns"

// accelLNSLog is the file accel-ppp logs to, so the checker reads the LCP and
// CHAP packets it printed (accel-ppp.conf, [log] and [ppp] verbose).
const accelLNSLog = "/var/log/accel-ppp.log"

// accelLNSChapLines is accel-ppp's chap-secrets entry for the CPE: the
// converted configuration's username and lab password, and the address
// accel-ppp hands it in IPCP (client, server, secret, address).
const accelLNSChapLines = subscriberUser + " * cpe-fixture-password " + subscriberAddress

// pppdChapLine is the realm-relay scenarios' chap-secrets entry for the pppd
// subscriber whose realm the LAC relays, in the same form. The stray
// subscriber has none: the LAC never relays it, and an entry would let a
// wrong relay succeed.
const pppdChapLine = pppdUser + " * " + pppdPassword + " " + subscriberAddress

// accelLNS is accel-ppp answering L2TP on eth1 (link core), checking the
// converted CPE's CHAP.
func accelLNS() node {
	return accelLNSFor(accelLNSChapLines)
}

// accelLNSFor is accel-ppp answering L2TP on eth1 (link core), with chapLines
// as its chap-secrets. The Alpine image carries no ss, so readiness reads the
// UDP socket table: 06A5 is port 1701.
func accelLNSFor(chapLines string) node {
	return node{name: nodeAccelLNS, image: accelImage, host: 3,
		config: "accel-ppp.conf", mountedAt: "/etc/accel-ppp.conf",
		capabilities: []string{capabilityNetAdmin},
		arguments:    interoplab.PPPDeviceArguments(),
		prestart:     "mkdir -p /etc/accel-ppp /var/log; printf '%s\\n' '" + chapLines + "' > /etc/accel-ppp/chap-secrets",
		ready:        []string{"sh", "-c", "grep -q ':06A5 ' /proc/net/udp"}}
}

// wholesaleAccelChain is access-wholesale with accel-ppp in place of ze-lns
// and no FreeRADIUS, since accel-ppp checks CHAP from its own chap-secrets:
// ze-cpe ==PPPoE (link access)== ze-lac ==L2TP (link core)== accel-lns.
func wholesaleAccelChain(checker interoplab.Checker) scenario {
	chain := wholesaleChain(accessChain(checker))
	nodes := make([]node, 0, len(chain.nodes))
	for _, member := range chain.nodes {
		switch member.name {
		case nodeRADIUS:
			continue
		case nodeLNS:
			nodes = append(nodes, accelLNS())
		default:
			nodes = append(nodes, member)
		}
	}
	chain.nodes = nodes
	for i := range chain.links {
		if chain.links[i].Name == coreLink {
			chain.links[i].Members = []string{nodeLAC, nodeAccelLNS}
		}
	}
	return chain
}

func init() {
	scenarios[accessWholesaleAccelScenario] = wholesaleAccelChain(checkAccessWholesaleAccel)
}
