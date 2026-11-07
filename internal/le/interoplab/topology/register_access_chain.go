// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_access.go -- the checkers this file registers.
// Related: register_images.go -- the FreeRADIUS image the core link needs.
package topology

import (
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The access chain's scenarios, each a directory under scenarioDirectory.
const (
	accessUpScenario         = "access-up"
	accessPoolScenario       = "access-pool"
	accessAccountingScenario = "access-accounting"
)

// The access chain's nodes, named by the declaration and the checkers.
const (
	nodeRADIUS = "freeradius"
	nodeLNS    = "ze-lns"
	nodeLAC    = "ze-lac"
	// nodeCPE ("ze-cpe") is shared with pppoe-mss-pmtu, where it is declared.
)

// The access chain's links: access carries PPPoE only, core carries L2TP and
// RADIUS. Each member's host number is its last octet on core.
const (
	accessLink = "access"
	coreLink   = "core"
)

// Values the configuration files, the declaration and the checkers share.
// Every address is a documentation or shared-address range: core
// 192.0.2.0/24 (RFC 5737), the subscriber 203.0.113.1/32 (RFC 5737) and the
// pool 100.64.0.0/24 (RFC 6598).
const (
	lnsCoreAddress    = "192.0.2.3"
	subscriberAddress = "203.0.113.1"
	poolGateway       = "100.64.0.1"
	poolNetwork       = "100.64.0.0/24"
	dnsPrimary        = "192.0.2.53"
	dnsSecondary      = "192.0.2.54"
	subscriberUser    = "cpe@lab.example.net"
	poolUser          = "pool@lab.example.net"
	lnsNASIdentifier  = "ze-lns"
	sessionMRU        = 1492
	// cpeDevice is the logical name ze-cpe.conf gives its PPPoE client.
	cpeDevice = "pppoe0"
	// accessCapture and coreCapture are the captures ze-lac and ze-lns start
	// in their prestart, before their daemon: PPPoE discovery on ze-lac's
	// access device, and L2TP on ze-lns's core device.
	accessCapture = "/run/access.pcap"
	coreCapture   = "/run/core.pcap"
	// authLogPath and accountingDetailPath are the files FreeRADIUS writes
	// (access-chain/mods-ze-auth-log, interop-radius-subscriber/mods-ze-detail).
	authLogPath          = "/var/log/freeradius/ze-auth.log"
	accountingDetailPath = "/var/log/freeradius/ze-detail"
)

// accessChain answers the access chain's declaration: ze-cpe ==PPPoE (link
// access)== ze-lac ==L2TP (link core)== ze-lns -- RADIUS -- freeradius. The
// LAC relays the empty Service-Name and terminates nothing, so LCP, CHAP and
// IPCP run between the CPE and the LNS across the LAC's kernel bridge. The
// nodes start server first, so each one's peer is up when it starts; the
// CPE's eth0 is flushed, so the session is its only path to the LNS.
func accessChain(checker interoplab.Checker) scenario {
	freeRADIUS := node{name: nodeRADIUS, image: freeRADIUSLinkImage, host: 4,
		config: "users", mountedAt: "/etc/raddb/mods-config/files/authorize",
		capabilities: []string{capabilityNetAdmin},
		// -X keeps every decoded request on the container log, so a failed
		// scenario names what the server saw.
		command: []string{"radiusd", "-f", "-X"},
		shared: []sharedFile{
			{source: accessChainDirectory + "/clients.conf", target: "/etc/raddb/clients.conf"},
			{source: accessChainDirectory + "/site-default", target: "/etc/raddb/sites-enabled/default"},
			{source: accessChainDirectory + "/mods-ze-auth-log", target: "/etc/raddb/mods-enabled/ze_auth_log"},
			{source: "test/interop-radius-subscriber/mods-ze-detail", target: "/etc/raddb/mods-enabled/ze_detail"},
		},
		// 0714 is 1812, the authentication listener, and 0715 is 1813, the
		// accounting listener (access-chain/site-default).
		ready: []string{"sh", "-c", "grep -q ':0714 ' /proc/net/udp && grep -q ':0715 ' /proc/net/udp"}}
	lns := zeNode(nodeLNS, 3, "ze-lns.conf", interoplab.PPPDeviceArguments()...)
	lns.prestart = captureStart("eth1", coreCapture, "udp port 1701")
	lns.ready = []string{"sh", "-c", "ss -lun | grep -q ':1701 '"}
	// The LAC's configuration is the same in every access scenario, so it is
	// a shared file rather than a scenario-directory config.
	lac := node{name: nodeLAC, image: zeImage, host: 2,
		command: []string{"start", zeMountedConfig}, capabilities: []string{capabilityNetAdmin},
		arguments: interoplab.PPPDeviceArguments(),
		shared:    []sharedFile{{source: accessChainDirectory + "/ze-lac.conf", target: zeMountedConfig}},
		prestart:  captureStart("eth1", accessCapture, "ether proto 0x8863"),
		ready:     []string{"sh", "-c", "ss -lun | grep -q ':1701 '"}}
	cpe := zeNode(nodeCPE, 5, "ze-cpe.conf", interoplab.PPPDeviceArguments()...)
	cpe.prestart = "ip -4 address flush dev eth0; ip -6 address flush dev eth0 scope global"
	return scenario{
		nodes: []node{freeRADIUS, lns, lac, cpe},
		links: []interoplab.Link{
			{Name: accessLink, VLAN: 191, Members: []string{nodeLAC, nodeCPE}},
			{Name: coreLink, VLAN: 192, IPv4: netip.MustParsePrefix("192.0.2.0/24"), Members: []string{nodeLAC, nodeLNS, nodeRADIUS}},
		},
		checker: checker,
	}
}

// captureStart is the prestart that records filter on device into file from
// before the daemon starts, so the first frame of the session is in it. The
// second's wait lets tcpdump open the device before the daemon sends.
func captureStart(device, file, filter string) string {
	return "tcpdump -p -n -U -i " + device + " -w " + file + " '" + filter + "' >/dev/null 2>&1 &\nsleep 1"
}

func init() {
	scenarios[accessUpScenario] = accessChain(checkAccessUp)
	scenarios[accessPoolScenario] = accessChain(checkAccessPool)
	scenarios[accessAccountingScenario] = accessChain(checkAccessAccounting)
}
