// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_access.go -- the checkers this file registers.
// Related: register_images.go -- the FreeRADIUS image the core link needs.
package topology

import (
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/test/labconfig/homecpe"
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
	// cpeStaticNameServer is the converted ze-cpe.conf's `system name-server`
	// (VyOS `set system name-server`). It is also the LNS's primary, so the
	// learned secondary alone tells a resolv.conf that kept static precedence
	// from one that merged the IPCP servers in.
	cpeStaticNameServer = "192.0.2.53"
	subscriberUser      = "cpe@lab.example.net"
	lnsNASIdentifier    = "ze-lns"
	sessionMRU          = 1492
	// cpeDevice is the logical name the converted ze-cpe.conf gives its
	// PPPoE client.
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

// accessChain answers the access chain's declaration with the home CPE as
// the subscriber: ze-cpe ==PPPoE (link access)== ze-lac ==L2TP (link core)==
// ze-lns -- RADIUS -- freeradius. The CPE runs the converted configuration
// the prepare step rendered (convertedCPE, spec-home-cpe-11 AC-1), whose
// bridge br0 holds eth2, so the CPE also joins link lan, with lan-client as
// the link's second member: a link needs two. Links are declared access,
// core, lan, so the CPE reaches access as eth1 (pppoe0's source) and lan as
// eth2, and the LAC reaches access as eth1 and core as eth2. Each node of
// lan joins link lan after lan-client.
func accessChain(checker interoplab.Checker, lan ...node) scenario {
	chain := dialChain(checker, convertedCPE())
	members := make([]string, 0, 2+len(lan))
	members = append(members, nodeCPE, nodeLANClient)
	chain.nodes = append(chain.nodes, dhcpClient(nodeLANClient, 21, ""))
	chain.nodes = append(chain.nodes, lan...)
	for index := range lan {
		members = append(members, lan[index].name)
	}
	chain.links = append(chain.links, interoplab.Link{Name: homeLANLink, VLAN: 197, Members: members})
	chain.prepare = prepareHomeCPE(homecpe.KeyP256)
	return chain
}

// The lab's pass-through subnet, which access-up adds to the CPE
// (spec-home-cpe-23b story 1b): 10.77.0.0/24 on br0, with no dns-server and
// no dns-source, so option 6 there comes from the D-5 default (configured,
// then received, first usable). lan-res-c is reserved in it, so it leases
// there whatever subnet a new client is allocated from.
const (
	nodeLANResC         = "lan-res-c"
	resCMAC             = "02:00:5e:50:00:50"
	resCAddress         = "10.77.0.50"
	passThroughGateway  = "10.77.0.1"
	passThroughSubnetIP = "10.77.0.0/24"
)

// passThroughAdditions add the pass-through subnet to the converted CPE. The
// VyOS source has no subnet whose conversion keeps the default: a VyOS subnet
// with no `option name-server` sends no option 6, so its faithful conversion
// writes `dns-source source configured` (docs/guide/migrating-from-vyos.md).
// The subnet is added beside 10.99.0.0/24, so 192.168.50.0/24 stays first in
// both the address list and the subnet list: br0's primary address, and so
// the subnet a new client is allocated from (resolveSegment), are unchanged.
var passThroughAdditions = []cpeAddition{
	{anchor: "10.99.0.1/24 ]", text: passThroughGateway + "/24 "},
	{anchor: "            subnet 10.99.0.0/24 {\n", text: "" +
		"            subnet " + passThroughSubnetIP + " {\n" +
		"                range 0 {\n" +
		"                    start 10.77.0.100;\n" +
		"                    stop 10.77.0.199;\n" +
		"                }\n" +
		"                lease-time 86400;\n" +
		"                default-router " + passThroughGateway + ";\n" +
		"                static-mapping res-c {\n" +
		"                    mac-address " + resCMAC + ";\n" +
		"                    ip-address " + resCAddress + ";\n" +
		"                }\n" +
		"            }\n"},
}

// accessUpChain is access-up's chain: accessChain with lan-res-c on link lan
// and the pass-through subnet added to the CPE, so the scenario also proves
// option 6 under the default (checkOption6PassThrough).
func accessUpChain() scenario {
	chain := accessChain(checkAccessUp, dhcpClient(nodeLANResC, 25, resCMAC))
	chain.prepare = prepareHomeCPE(homecpe.KeyP256, passThroughAdditions...)
	return chain
}

// dialChain answers the access chain with subscriber dialing PPPoE on link
// access. The LAC relays the empty Service-Name and terminates nothing, so
// LCP, CHAP and IPCP run between the subscriber and the LNS across the LAC's
// kernel bridge. The nodes start server first, so each one's peer is up when
// it starts; the subscriber flushes its eth0 in its prestart, so the session
// is its only path to the LNS.
func dialChain(checker interoplab.Checker, subscriber node) scenario {
	lns := zeNode(nodeLNS, 3, "ze-lns.conf", interoplab.PPPDeviceArguments()...)
	lns.prestart = captureStart("eth1", coreCapture, "udp port 1701")
	lns.ready = []string{"sh", "-c", "ss -lun | grep -q ':1701 '"}
	return scenario{
		nodes: []node{accessRADIUS(""), lns, accessLAC(), subscriber},
		links: []interoplab.Link{
			{Name: accessLink, VLAN: 191, Members: []string{nodeLAC, subscriber.name}},
			{Name: coreLink, VLAN: 192, IPv4: netip.MustParsePrefix("192.0.2.0/24"), Members: []string{nodeLAC, nodeLNS, nodeRADIUS}},
		},
		checker: checker,
	}
}

// reconnectUsersTarget is where access-reconnect mounts its users file, so
// its FreeRADIUS answers from a writable copy the checker edits.
const reconnectUsersTarget = "/etc/ze-lab/users"

// accessRADIUS is the access chain's FreeRADIUS on link core, answering from
// the scenario's users file. usersCopy empty mounts that file read-only as the
// authorize file; otherwise it is mounted at usersCopy and the prestart copies
// it into place once, so a checker can change the answer and restart the
// server (the copy survives a restart; a second copy would undo the change).
func accessRADIUS(usersCopy string) node {
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
	if usersCopy != "" {
		freeRADIUS.mountedAt = usersCopy
		freeRADIUS.prestart = "[ -e /etc/raddb/.ze-lab-users ] || { cp " + usersCopy +
			" /etc/raddb/mods-config/files/authorize && touch /etc/raddb/.ze-lab-users; }"
	}
	return freeRADIUS
}

// accessLAC is the access chain's Ze LAC, relaying the empty Service-Name to
// ze-lns, with its access capture started before the daemon.
func accessLAC() node {
	// The LAC's configuration is the same in every access scenario, so it is
	// a shared file rather than a scenario-directory config.
	return node{name: nodeLAC, image: zeImage, host: 2,
		command: []string{"start", zeMountedConfig}, capabilities: []string{capabilityNetAdmin},
		arguments: interoplab.PPPDeviceArguments(),
		shared:    []sharedFile{{source: accessChainDirectory + "/ze-lac.conf", target: zeMountedConfig}},
		prestart:  captureStart("eth1", accessCapture, "ether proto 0x8863"),
		ready:     []string{"sh", "-c", "ss -lun | grep -q ':1701 '"}}
}

// captureStart is the prestart that records filter on device into file from
// before the daemon starts, so the first frame of the session is in it. The
// second's wait lets tcpdump open the device before the daemon sends.
func captureStart(device, file, filter string) string {
	return "tcpdump -p -n -U -i " + device + " -w " + file + " '" + filter + "' >/dev/null 2>&1 &\nsleep 1"
}

func init() {
	scenarios[accessUpScenario] = accessUpChain()
	scenarios[accessPoolScenario] = accessChain(checkAccessPool)
	scenarios[accessAccountingScenario] = accessChain(checkAccessAccounting)
}
