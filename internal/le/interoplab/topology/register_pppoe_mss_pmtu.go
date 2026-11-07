// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: check_mss_pmtu.go -- the checker this file registers.
package topology

import (
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const pppoeMSSPMTUScenario = "pppoe-mss-pmtu"

// The nodes of pppoe-mss-pmtu, named by the declaration and the checker.
const (
	nodeAccel   = "accel"
	nodeCPE     = "ze-cpe"
	nodeLANHost = "lan-host"
	nodeServer  = "server"
)

// accelImage is the accel-ppp image the PPPoE suite registers
// (pppoe/register_images.go, accelImageTag).
const accelImage = "ze-pppoe-accel"

// Addresses the declaration, the prestart shell and the checker share. The
// server is host 5 on link core and the CPE host 3 on link lan.
const (
	serverAddress  = "10.186.1.5"
	cpeLANAddress  = "10.185.1.3"
	accelCoreHop   = "10.186.1.2"
	sessionPool    = "10.184.0.0/24"
	serverNetwork  = "10.186.1.0/24"
	serverCapture  = "/run/syn.pcap"
	accelChapLines = "alice * s3cr3t *"
)

// lan-host -- ze-cpe ==PPPoE== accel -- server. Ze dials accel-ppp on link
// access with MTU 1492 and clamps every SYN leaving on the PPPoE device to
// the route MSS (tcp-mss-clamp-to-pmtu). lan-host opens TCP to server, whose
// capture is what the checker reads, so the MSS asserted is the one the far
// end of the path receives. accel-ppp is third-party software on both the
// PPPoE session and the forwarding path.
func init() {
	scenarios[pppoeMSSPMTUScenario] = scenario{
		nodes: []node{
			{name: nodeAccel, image: accelImage, host: 2, config: "accel-ppp.conf", mountedAt: "/etc/accel-ppp.conf",
				capabilities: []string{capabilityNetAdmin},
				arguments:    append(interoplab.PPPDeviceArguments(), "--sysctl", "net.ipv4.ip_forward=1"),
				prestart:     "mkdir -p /etc/accel-ppp; printf '%s\\n' '" + accelChapLines + "' > /etc/accel-ppp/chap-secrets"},
			{name: nodeServer, image: zeImage, host: 5,
				capabilities: []string{capabilityNetAdmin},
				arguments:    []string{"--entrypoint", "tcpdump"},
				command:      []string{"-p", "-n", "-U", "-i", "eth1", "-w", serverCapture, "tcp[tcpflags] & tcp-syn != 0"},
				prestart:     "ip route add " + sessionPool + " via " + accelCoreHop + " dev eth1"},
			zeNode(nodeCPE, 3, "ze-cpe.conf", append(interoplab.PPPDeviceArguments(), "--sysctl", "net.ipv4.ip_forward=1")...),
			{name: nodeLANHost, image: zeImage, host: 4,
				capabilities: []string{capabilityNetAdmin},
				arguments:    []string{"--entrypoint", "sleep"}, command: []string{"infinity"}},
		},
		links: []interoplab.Link{
			{Name: "access", VLAN: 184, Members: []string{nodeAccel, nodeCPE}},
			{Name: "lan", VLAN: 185, IPv4: netip.MustParsePrefix("10.185.1.0/24"), Members: []string{nodeCPE, nodeLANHost}},
			{Name: "core", VLAN: 186, IPv4: netip.MustParsePrefix("10.186.1.0/24"), Members: []string{nodeAccel, nodeServer}},
		},
		checker: checkMSSPMTU,
	}
}
