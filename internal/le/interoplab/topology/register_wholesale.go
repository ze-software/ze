// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_wholesale.go -- the checkers this file registers.
// Related: register_access_chain.go -- the chain these scenarios extend.
// Related: check_realm_relay.go -- the realm-relay checkers this file registers.
package topology

import (
	"net/netip"
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
			chain.nodes[i] = realmLAC()
		}
	}
	return chain
}

// realmLAC is the access chain's LAC reading the scenario directory's
// ze-lac.conf, which relays a realm instead of the empty Service-Name.
func realmLAC() node {
	lac := accessLAC()
	lac.shared = nil
	lac.config = "ze-lac.conf"
	lac.mountedAt = zeMountedConfig
	return lac
}

// pppdSubscriber dials PPPoE on eth1 (link access) with pppd, CHAP only, and
// keeps retrying until the chain is up. `debug` puts every LCP packet in the
// log the checker reads, so a second negotiation is visible there. eth0 is
// flushed so the session is the only path to the LNS, and `defaultroute`
// makes it the path: IPCP gives ppp0 the subscriber address and the pool
// gateway as its peer, so without a default route over it pppd's ping of the
// LNS's core address 192.0.2.3 meets "Network unreachable".
func pppdSubscriber() node {
	return pppdSubscriberAs(nodePPPD, pppdUser, 5)
}

// pppdSubscriberAs is pppdSubscriber dialing as user, in the container name
// on host number host.
func pppdSubscriberAs(name, user string, host uint8) node {
	arguments := []string{
		"pppd", "plugin", "pppoe.so", "nic-eth1",
		"user", user, "password", pppdPassword,
		"noauth", "refuse-pap", "refuse-eap", "refuse-mschap", "refuse-mschap-v2",
		"noipdefault", "defaultroute", "noaccomp", "nopcomp",
		"mtu", "1492", "mru", "1492",
		"persist", "holdoff", "2", "maxfail", "0",
		"nodetach", "debug", "logfile", pppdLog,
	}
	shell := "mkdir -p /etc/ppp /var/log/ppp; exec " + strings.Join(arguments, " ")
	return node{name: name, image: pppdClientImage, host: host,
		capabilities: []string{capabilityNetAdmin},
		arguments:    append(interoplab.PPPDeviceArguments(), "--entrypoint", "sh"),
		command:      []string{"-c", shell},
		prestart:     "ip -4 address flush dev eth0; ip -6 address flush dev eth0 scope global",
		logFiles:     []string{pppdLog}}
}

// The realm-relay scenarios of spec-home-cpe-14: a pppd subscriber whose
// realm the Ze LAC relays reaches a third-party LNS, and a second one whose
// realm no binding names is refused at the LAC.
const (
	realmRelayAccelScenario  = "l2tp-lac-realm-relay-accel-ppp"
	realmRelayXL2TPDScenario = "l2tp-lac-realm-relay-xl2tpd"
)

// The realm-relay scenarios' second subscriber. Its realm, other.example.net,
// is not the one ze-lac.conf relays (lab.example.net), so the LAC
// authenticates it locally, and with no auth handler configured it refuses it.
const (
	nodeStrayPPPD = "pppd-stray"
	strayUser     = "pppd@other.example.net"
)

// The xl2tpd LNS of l2tp-lac-realm-relay-xl2tpd.
const (
	nodeXL2TPDLNS = "xl2tpd-lns"
	// xl2tpdImage is the xl2tpd 1.3.20 and pppd 2.5.5 image the RADIUS
	// subscriber suite registers for its LAC
	// (radiussubscriber/register_images.go, lacImage). The same two programs
	// serve the LNS role from configuration alone.
	xl2tpdImage  = "ze-radius-subscriber-lac"
	xl2tpdConfig = "/etc/xl2tpd/xl2tpd.conf"
	// xl2tpdLNSLog holds what xl2tpd writes to stderr in the foreground
	// (misc.c l2tp_log: syslog is off when -D keeps it from daemonizing).
	xl2tpdLNSLog = "/var/log/xl2tpd.log"
	// xl2tpdPPPDLog is the debug log of the pppd xl2tpd starts for the call
	// (the scenario's ppp-options, `logfile`).
	xl2tpdPPPDLog = "/var/log/ppp/pppd-lns.log"
	// xl2tpdPPPOptions is where the LNS's pppd options are mounted, the path
	// xl2tpd.conf's `pppoptfile` names.
	xl2tpdPPPOptions = "/etc/ppp/options.xl2tpd-lns"
)

// realmRelayChain declares a realm-relay scenario: pppd (pppd@lab.example.net)
// and pppd-stray (pppd@other.example.net) dial PPPoE to the scenario's Ze LAC
// on link access, and lns answers the LAC's call on link core at
// lnsCoreAddress. No FreeRADIUS runs: the LNS checks CHAP from its own
// secrets. Each subscriber flushes its eth0, so the session is its only path.
func realmRelayChain(lns node, checker interoplab.Checker) scenario {
	return scenario{
		nodes: []node{lns, realmLAC(), pppdSubscriber(), pppdSubscriberAs(nodeStrayPPPD, strayUser, 6)},
		links: []interoplab.Link{
			{Name: accessLink, VLAN: 191, Members: []string{nodeLAC, nodePPPD, nodeStrayPPPD}},
			{Name: coreLink, VLAN: 192, IPv4: netip.MustParsePrefix("192.0.2.0/24"), Members: []string{nodeLAC, lns.name}},
		},
		checker: checker,
	}
}

// xl2tpdLNS is xl2tpd answering L2TP on eth1 (link core) at lnsCoreAddress,
// starting pppd for each call it accepts (control.c control_finish, ICCN).
// The shell keeps xl2tpd in the foreground and sends its log to xl2tpdLNSLog
// for the checker; the pppd it starts inherits that stderr and also writes
// its own debug log. 06A5 in the UDP socket table is port 1701.
func xl2tpdLNS() node {
	shell := "mkdir -p /var/log/ppp /var/run/xl2tpd; exec xl2tpd -D -c " + xl2tpdConfig + " >>" + xl2tpdLNSLog + " 2>&1"
	directory := scenarioDirectory + "/" + realmRelayXL2TPDScenario
	return node{name: nodeXL2TPDLNS, image: xl2tpdImage, host: 3,
		config: "xl2tpd.conf", mountedAt: xl2tpdConfig,
		capabilities: []string{capabilityNetAdmin},
		arguments:    append(interoplab.PPPDeviceArguments(), "--entrypoint", "sh"),
		command:      []string{"-c", shell},
		shared: []sharedFile{
			{source: directory + "/l2tp-secrets", target: "/etc/xl2tpd/l2tp-secrets"},
			{source: directory + "/ppp-options", target: xl2tpdPPPOptions},
			{source: directory + "/chap-secrets", target: "/etc/ppp/chap-secrets"},
		},
		ready:    []string{"sh", "-c", "grep -q ':06A5 ' /proc/net/udp"},
		logFiles: []string{xl2tpdLNSLog, xl2tpdPPPDLog}}
}

func init() {
	scenarios[accessWholesaleScenario] = wholesaleChain(accessChain(checkAccessWholesale))
	scenarios[lcpRenegotiatePPPD] = wholesaleChain(dialChain(checkLCPRenegotiatePPPD, pppdSubscriber()))
	scenarios[realmRelayAccelScenario] = realmRelayChain(accelLNSFor(pppdChapLine), checkRealmRelayAccel)
	scenarios[realmRelayXL2TPDScenario] = realmRelayChain(xl2tpdLNS(), checkRealmRelayXL2TPD)
}
