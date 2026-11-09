// Design: docs/architecture/testing/interop.md -- the home-CPE lab.
// Related: home_lab_render.go -- the converted CPE configuration every node here runs.
// Related: check_conversion.go -- conversion.
// Related: check_lan.go -- lan-services.
// Related: check_vpn.go -- road-warrior, mgmt-tunnel, access-reconnect.
package topology

import (
	"net/netip"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The home-CPE lab's scenarios (spec-home-cpe-11), each a directory under
// scenarioDirectory. access-wholesale, child 14's, is declared in
// register_wholesale.go.
const (
	conversionScenario      = "conversion"
	lanServicesScenario     = "lan-services"
	roadWarriorScenario     = "road-warrior"
	mgmtTunnelScenario      = "mgmt-tunnel"
	accessReconnectScenario = "access-reconnect"
	// mgmtTunnelEd25519Scenario is mgmt-tunnel with the CPE holding an
	// Ed25519 key (AC-9); mgmt-tunnel is the P-256 variant.
	mgmtTunnelEd25519Scenario = "mgmt-tunnel-ed25519"
)

// homeLabDirectory holds the home lab's shared peer files and the web and FTP
// server's image.
const homeLabDirectory = "test/interop-topology/home-lab"

// The home lab's nodes beyond the access chain.
const (
	nodeLANClient = "lan-client"
	nodeLANResA   = "lan-res-a"
	nodeLANResB   = "lan-res-b"
	nodeLANStatic = "lan-host"
	nodeWeb       = "web"
	nodeWGClient  = "wg-client"
	nodeSwan      = "strongswan"
	nodeDNS       = "dns"
)

// The images the home lab names: two this file registers, and the Knot and
// strongSwan images the BGP and IPsec suites register.
const (
	webFTPImage    = "ze-topology-webftp"
	wireGuardImage = "ze-topology-wireguard"
	knotImage      = "knot"
	swanImage      = "ze-ipsec-strongswan"
)

// The home lab's links. Each node reaches its links as eth1, eth2, ... in
// the order below, so the CPE's access link is eth1 (pppoe0's source) and its
// LAN eth2 (br0's member), as ze-cpe.conf names them.
const (
	homeAccessLink   = "access"
	homeLANLink      = "lan"
	homeCoreLink     = "core"
	homeInternetLink = "internet"
)

// Values the declarations, the peer files and the checkers share. Every
// address is a documentation, shared-address or private range: internet
// 198.51.100.0/24 (RFC 5737), the LAN 192.168.50.0/24 and 10.99.0.0/24, the
// road warriors 10.88.0.0/24, management fd00:db8:1::/112 (ULA).
const (
	lnsInternetAddress = "198.51.100.3"
	webAddress         = "198.51.100.10"
	wgClientAddress    = "198.51.100.20"
	swanAddress        = "198.51.100.30"
	cpeLANGateway      = "192.168.50.1"
	lanStaticAddress   = "192.168.50.10"
	resAAddress        = "192.168.50.30"
	resBAddress        = "10.99.0.40"
	resAMAC            = "02:00:5e:50:00:30"
	resBMAC            = "02:00:5e:50:00:40"
	rangeFirst         = "192.168.50.100"
	rangeLast          = "192.168.50.199"
	wgClientInner      = "10.88.0.2"
	cpeManagement      = "fd00:db8:1::2"
	swanManagement     = "fd00:db8:1::1"
	reconnectAddress   = "203.0.113.2"
	webCapture         = "/run/web.pcap"
	dnsGate            = "/run/dns-go"
	// dockerSysctl is the docker run flag that sets a namespaced sysctl, and
	// sleepForever the argument that keeps a sleep-only node running.
	dockerSysctl = "--sysctl"
	sleepForever = "infinity"
	commandSleep = "sleep"
	// dockerEntrypoint is the docker run flag that replaces the image entrypoint.
	dockerEntrypoint = "--entrypoint"
)

// subscriberRoutes is the prestart line that sends the subscriber prefixes to
// gateway, for a node beyond the LNS.
func subscriberRoutes(gateway string) string {
	return "ip route add 203.0.113.0/24 via " + gateway + "; ip route add " + poolNetwork + " via " + gateway
}

// flushEth0 removes every address of the lab bridge's eth0, so the links are
// a node's only path (vacuity trap 4).
const flushEth0 = "ip -4 address flush dev eth0; ip -6 address flush dev eth0 scope global"

// homeOptions names which of the home lab's optional nodes a scenario runs,
// and the algorithm of the key the lab issues the CPE (its zero value is no
// choice, which the prepare step refuses).
type homeOptions struct {
	lan       []node
	internet  []node
	dns       *node
	usersCopy string
	cpeKey    cpeKeyAlgorithm
}

// homeLab answers a home-CPE scenario: the access chain (ze-cpe ==PPPoE==
// ze-lac ==L2TP== ze-lns, which checks CHAP with freeradius) with the CPE
// running the converted configuration, the LAN nodes on the CPE's bridge, and
// the internet nodes the LNS routes. The CPE starts after every server, so
// what it dials is listening; the LAN nodes start last.
func homeLab(checker interoplab.Checker, options homeOptions) scenario {
	lns := zeNode(nodeLNS, 3, "", append(interoplab.PPPDeviceArguments(), dockerSysctl, "net.ipv4.ip_forward=1")...)
	lns.config = ""
	lns.shared = []sharedFile{{source: "test/interop-topology/scenarios/access-up/ze-lns.conf", target: zeMountedConfig}}
	lns.ready = []string{"sh", "-c", "ss -lun | grep -q ':1701 '"}
	nodes := []node{accessRADIUS(options.usersCopy), lns, accessLAC()}
	core := []string{nodeLAC, nodeLNS, nodeRADIUS}
	if options.dns != nil {
		nodes = append(nodes, *options.dns)
		core = append(core, nodeDNS)
	}
	internet := []string{nodeLNS}
	for _, member := range options.internet {
		nodes = append(nodes, member)
		internet = append(internet, member.name)
	}
	nodes = append(nodes, convertedCPE())
	lan := []string{nodeCPE}
	for _, member := range options.lan {
		nodes = append(nodes, member)
		lan = append(lan, member.name)
	}
	links := []interoplab.Link{
		{Name: homeAccessLink, VLAN: 193, Members: []string{nodeLAC, nodeCPE}},
		{Name: homeLANLink, VLAN: 194, Members: lan},
		{Name: homeCoreLink, VLAN: 195, IPv4: netip.MustParsePrefix("192.0.2.0/24"), Members: core},
	}
	if len(internet) > 1 {
		links = append(links, interoplab.Link{Name: homeInternetLink, VLAN: 196,
			IPv4: netip.MustParsePrefix("198.51.100.0/24"), Members: internet})
	}
	return scenario{nodes: nodes, links: links, checker: checker, prepare: prepareHomeCPE(options.cpeKey)}
}

// convertedCPE is the Ze CPE of every home-CPE scenario: the converted
// configuration the prepare step rendered, never a file of its own. A
// scenario that needs a case the VyOS source cannot state inserts it at
// render time (cpeAddition: access-up's pass-through subnet). IPv6 is
// enabled for vti0's management address, and the node writes its devices'
// sysctls at run time as a deployed CPE does (netSysctlWriter).
//
// It also mounts the prepared resolv.conf at /etc/resolv.conf. On the gokrazy
// appliance /etc/resolv.conf is a link to /tmp/resolv.conf, the file Ze writes
// from `system name-server` (the YANG default of `system dns
// resolv-conf-path`), so Go's resolver, which IKE uses for the
// `vpn.example.net` remote-address, asks the configured server. A container's
// /etc/resolv.conf is Docker's own file naming 127.0.0.11, which cannot answer
// a lab name, and Ze writes /tmp/resolv.conf by rename, so no bind mount can
// follow it. The prepared file names the same server (cpeStaticNameServer),
// and access-up proves Ze's own /tmp/resolv.conf holds exactly that server.
//
// It holds BPF beside NET_ADMIN because its configuration carries `vpn ipsec`
// (capabilityBPF).
func convertedCPE() node {
	cpe := zeNode(nodeCPE, 5, "", append(interoplab.PPPDeviceArguments(),
		dockerSysctl, "net.ipv4.ip_forward=1", dockerSysctl, "net.ipv6.conf.all.disable_ipv6=0")...)
	cpe.config = ""
	// ze-cpe.conf carries `vpn ipsec` with no application-override, so the
	// default deny attaches the override guard (capabilityBPF).
	cpe.capabilities = append(cpe.capabilities, capabilityBPF)
	cpe.prepared = []sharedFile{{source: renderedCPEConfig, target: zeMountedConfig},
		{source: renderedCPEResolvConf, target: systemResolvConf}}
	cpe.prestart = flushEth0
	return netSysctlWriter(cpe)
}

// dhcpClient is a LAN host that dhcpcd numbers (the Knot image carries
// dhcpcd 10.5.2); mac, when set, is the address a reservation keys on.
func dhcpClient(name string, host uint8, mac string) node {
	prestart := flushEth0
	if mac != "" {
		prestart += "\nip link set eth1 down; ip link set eth1 address " + mac + "; ip link set eth1 up"
	}
	// dhcpcd's if_init (src/if-linux.c) reads the link's
	// net.ipv4.conf.<if>.promote_secondaries and writes 1 when it is not 1; a
	// failed write ends dhcpcd ("if_init_os: Read-only file system"), because
	// Docker mounts /proc/sys read-only. The link joins the namespace after
	// start, so it inherits the default this argument sets, and dhcpcd reads
	// 1 and writes nothing.
	return node{name: name, image: knotImage, host: host, capabilities: []string{capabilityNetAdmin},
		arguments: []string{dockerSysctl, "net.ipv4.conf.default.promote_secondaries=1"},
		command:   []string{commandSleep, sleepForever}, prestart: prestart}
}

// lanStatic is the LAN host the checkers drive TCP from: a fixed address
// outside the range, curl for HTTP and active FTP, and bash for a bare
// connect.
func lanStatic() node {
	return node{name: nodeLANStatic, image: webFTPImage, host: 24, capabilities: []string{capabilityNetAdmin},
		arguments: []string{dockerEntrypoint, commandSleep}, command: []string{sleepForever},
		prestart: flushEth0 + "\nip address add " + lanStaticAddress + "/24 dev eth1; ip route add default via " + cpeLANGateway}
}

// webServer is the internet's web and FTP server, capturing every TCP SYN
// and data connection it receives from before its daemons start.
func webServer() node {
	return node{name: nodeWeb, image: webFTPImage, host: 10, capabilities: []string{capabilityNetAdmin},
		prestart: flushEth0 + "\n" + subscriberRoutes(lnsInternetAddress) + "\n" + captureStart("eth1", webCapture, "tcp")}
}

// wgClient is the road warrior: kernel WireGuard with the PSK, dialing the
// CPE's PPP address, with the LAN and the web server routed into its tunnel.
func wgClient() node {
	return node{name: nodeWGClient, image: wireGuardImage, host: 20, capabilities: []string{capabilityNetAdmin},
		prestart: flushEth0 + "\n" + subscriberRoutes(lnsInternetAddress) + "\n" +
			"ip link add wg0 type wireguard\n" +
			"wg set wg0 private-key /etc/wireguard/laptop.key peer \"$(cat /etc/wireguard/cpe.pub)\" " +
			"preshared-key /etc/wireguard/psk endpoint " + subscriberAddress + ":51820 " +
			"allowed-ips 192.168.50.0/24,10.99.0.0/24," + webAddress + "/32\n" +
			"ip address add " + wgClientInner + "/24 dev wg0; ip link set wg0 up\n" +
			"ip route add 192.168.50.0/24 dev wg0; ip route add 10.99.0.0/24 dev wg0; ip route add " + webAddress + "/32 dev wg0",
		prepared: []sharedFile{
			{source: issuedLaptopKey, target: "/etc/wireguard/laptop.key"},
			{source: issuedCPEPublic, target: "/etc/wireguard/cpe.pub"},
			{source: issuedLaptopPSK, target: "/etc/wireguard/psk"},
		}}
}

// swanResponder is vpn.example.net: strongSwan answering the CPE with the
// run's PKI, narrowing to ::/0, holding the management address on a dummy.
func swanResponder() node {
	return node{name: nodeSwan, image: swanImage, host: 30, capabilities: []string{capabilityNetAdmin},
		arguments: []string{dockerSysctl, "net.ipv6.conf.all.disable_ipv6=0"},
		prestart: flushEth0 + "\n" + subscriberRoutes(lnsInternetAddress) + "\n" +
			"ip link add inner type dummy; ip link set inner up; ip -6 address add " + swanManagement + "/128 dev inner nodad\n" +
			"ip -6 route add fd00:db8:1::/112 dev eth1",
		shared: []sharedFile{
			{source: homeLabDirectory + "/swanctl.conf", target: "/etc/swanctl/conf.d/home-lab.conf"},
			{source: "test/interop-ipsec/strongswan-lab.conf", target: "/etc/strongswan.d/98-lab.conf"},
		},
		prepared: []sharedFile{
			{source: issuedCA, target: "/etc/swanctl/x509ca/lab-ca.pem"},
			{source: issuedResponder, target: "/etc/swanctl/x509/vpn.pem"},
			{source: issuedResponderSK, target: "/etc/swanctl/private/vpn-key.pem"},
		},
		ready: []string{"sh", "-c", "swanctl --stats | grep -q uptime && swanctl --load-all >/dev/null"}}
}

// dnsServer is Knot answering vpn.example.net on the core link. A gated
// server waits for dnsGate before knotd starts, so the CPE boots while DNS
// is down and the checker decides when it answers.
func dnsServer(gated bool) *node {
	command := []string{"knotd", "-c", "/etc/knot/knot.conf"}
	if gated {
		command = []string{"sh", "-c", "until [ -e " + dnsGate + " ]; do sleep 1; done; exec knotd -c /etc/knot/knot.conf"}
	}
	return &node{name: nodeDNS, image: knotImage, host: 53, capabilities: []string{capabilityNetAdmin},
		command:  command,
		prestart: subscriberRoutes(lnsCoreAddress),
		shared: []sharedFile{
			{source: homeLabDirectory + "/knot.conf", target: "/etc/knot/knot.conf"},
			{source: homeLabDirectory + "/example.net.zone", target: "/etc/knot/example.net.zone"},
		}}
}

func init() {
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: webFTPImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			lab := filepath.Join(context.Root, filepath.FromSlash(homeLabDirectory))
			return interoplab.ImageBuild{Name: webFTPImage, Tag: webFTPImage,
				Dockerfile: filepath.Join(lab, "Dockerfile.webftp"), Context: lab, Required: true}
		}})
	interoplab.RegisterImage(interoplab.RegisteredImage{Name: wireGuardImage, Owner: imageOwner,
		Build: func(context interoplab.ImageContext) interoplab.ImageBuild {
			testdata := filepath.Join(context.Root, "internal", "le", "test", "deployment", "testdata")
			return interoplab.ImageBuild{Name: wireGuardImage, Tag: wireGuardImage,
				Dockerfile: filepath.Join(testdata, "Dockerfile.wireguard"), Context: testdata, Required: true}
		}})
	scenarios[conversionScenario] = homeLab(checkConversion, homeOptions{
		lan:    []node{dhcpClient(nodeLANClient, 21, "")},
		cpeKey: cpeKeyP256})
	scenarios[lanServicesScenario] = homeLab(checkLANServices, homeOptions{
		lan: []node{dhcpClient(nodeLANClient, 21, ""), dhcpClient(nodeLANResA, 22, resAMAC),
			dhcpClient(nodeLANResB, 23, resBMAC), lanStatic()},
		internet: []node{webServer()},
		cpeKey:   cpeKeyP256})
	scenarios[roadWarriorScenario] = homeLab(checkRoadWarrior, homeOptions{
		lan:      []node{dhcpClient(nodeLANResA, 22, resAMAC)},
		internet: []node{webServer(), wgClient()},
		cpeKey:   cpeKeyP256})
	scenarios[mgmtTunnelScenario] = homeLab(checkMgmtTunnel, homeOptions{
		lan:      []node{dhcpClient(nodeLANClient, 21, "")},
		internet: []node{swanResponder()},
		dns:      dnsServer(true),
		cpeKey:   cpeKeyP256})
	scenarios[mgmtTunnelEd25519Scenario] = homeLab(checkMgmtTunnelEd25519, homeOptions{
		lan:      []node{dhcpClient(nodeLANClient, 21, "")},
		internet: []node{swanResponder()},
		dns:      dnsServer(true),
		cpeKey:   cpeKeyEd25519})
	scenarios[accessReconnectScenario] = homeLab(checkAccessReconnect, homeOptions{
		lan:       []node{lanStatic()},
		internet:  []node{webServer(), swanResponder()},
		dns:       dnsServer(false),
		usersCopy: reconnectUsersTarget,
		cpeKey:    cpeKeyP256})
}
