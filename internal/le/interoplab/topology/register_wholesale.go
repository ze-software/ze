// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_wholesale.go -- the checkers this file registers.
// Related: register_access_chain.go -- the chain these scenarios extend.
package topology

import (
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The wholesale scenarios: a LAC that relays by realm, sending the proxy LCP
// AVPs of RFC 2661 Section 4.4.5, to an LNS configured `proxy-lcp
// renegotiate` (spec-home-cpe-15).
const (
	accessWholesaleScenario = "access-wholesale"
	lcpRenegotiatePPPD      = "l2tp-lns-lcp-renegotiate-pppd"
)

// The pppd subscriber of l2tp-lns-lcp-renegotiate-pppd.
const (
	nodePPPD = "pppd"
	// pppdClientImage is the pppd + rp-pppoe image the PPPoE suite registers
	// (pppoe/register_images.go, clientImageTag).
	pppdClientImage = "ze-pppoe-client"
	pppdUser        = "pppd@lab.example.net"
	pppdPassword    = "pppd-fixture-password"
	pppdLog         = "/var/log/ppp/pppd.log"
)

// wholesaleChain is chain with the scenario's own LAC, which relays the
// realm lab.example.net instead of the empty Service-Name, so the LAC runs
// LCP and starts CHAP itself and hands ze-lns proxy LCP. chain is the access
// chain with its subscriber: the converted Ze CPE (accessChain) or pppd
// (dialChain).
func wholesaleChain(chain scenario) scenario {
	for i := range chain.nodes {
		if chain.nodes[i].name == nodeLAC {
			chain.nodes[i].shared = nil
			chain.nodes[i].config = "ze-lac.conf"
			chain.nodes[i].mountedAt = zeMountedConfig
		}
	}
	return chain
}

// pppdSubscriber dials PPPoE on eth1 (link access) with pppd, CHAP only, and
// keeps retrying until the chain is up. `debug` puts every LCP packet in the
// log the checker reads, so a second negotiation is visible there. eth0 is
// flushed so the session is the only path to the LNS, and `defaultroute`
// makes it the path: IPCP gives ppp0 the subscriber address and the pool
// gateway as its peer, so without a default route over it pppd's ping of the
// LNS's core address 192.0.2.3 meets "Network unreachable".
func pppdSubscriber() node {
	arguments := []string{
		"pppd", "plugin", "pppoe.so", "nic-eth1",
		"user", pppdUser, "password", pppdPassword,
		"noauth", "refuse-pap", "refuse-eap", "refuse-mschap", "refuse-mschap-v2",
		"noipdefault", "defaultroute", "noaccomp", "nopcomp",
		"mtu", "1492", "mru", "1492",
		"persist", "holdoff", "2", "maxfail", "0",
		"nodetach", "debug", "logfile", pppdLog,
	}
	shell := "mkdir -p /etc/ppp /var/log/ppp; exec " + strings.Join(arguments, " ")
	return node{name: nodePPPD, image: pppdClientImage, host: 5,
		capabilities: []string{capabilityNetAdmin},
		arguments:    append(interoplab.PPPDeviceArguments(), "--entrypoint", "sh"),
		command:      []string{"-c", shell},
		prestart:     "ip -4 address flush dev eth0; ip -6 address flush dev eth0 scope global"}
}

func init() {
	scenarios[accessWholesaleScenario] = wholesaleChain(accessChain(checkAccessWholesale))
	scenarios[lcpRenegotiatePPPD] = wholesaleChain(dialChain(checkLCPRenegotiatePPPD, pppdSubscriber()))
}
