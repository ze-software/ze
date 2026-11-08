// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: check_ze_static_link_chain.go -- the checker this file registers.
package topology

import (
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const staticLinkChainScenario = "ze-static-link-chain"

// The chain's three Ze nodes, named by the declaration and the checker.
const (
	nodeA = "ze-a"
	nodeB = "ze-b"
	nodeC = "ze-c"
)

// linkABPrefix is link a-b's IPv4 prefix, whose frames outside records.
var linkABPrefix = netip.MustParsePrefix("10.181.1.0/24")

// outsideCapture is where outside's capture of link a-b's addresses lands.
// The filter takes IPv4 and ARP of the link's prefix, tagged or not: veth
// hands the VLAN tag to the packet socket out of band, so the kernel filter
// matches the link's own tagged frames too, and `tcpdump -e` prints the tag
// back (untaggedFrames).
const outsideCapture = "/run/link-ab.pcap"

// A -- B -- C: three Ze nodes, each with its own config file, on two IPv4
// links. A and C share no link, so A reaches C only through B, which
// forwards; A and C each route the far link through B in their own static
// config. outside sits on eth0 alone, runs no daemon, and records every
// frame of link a-b's prefix its port receives (outsideCapture).
func init() {
	scenarios[staticLinkChainScenario] = scenario{
		nodes: []node{
			zeNode(nodeA, 2, "ze-a.conf"),
			zeNode(nodeB, 3, "ze-b.conf", "--sysctl", "net.ipv4.ip_forward=1"),
			zeNode(nodeC, 4, "ze-c.conf"),
			{name: "outside", image: zeImage, host: 5,
				arguments: []string{"--entrypoint", "tcpdump"},
				command: []string{"-p", "-n", "-U", "-i", "eth0", "-w", outsideCapture,
					"ip net " + linkABPrefix.String() + " or arp net " + linkABPrefix.String()}},
		},
		links: []interoplab.Link{
			{Name: "a-b", VLAN: 181, IPv4: linkABPrefix, Members: []string{nodeA, nodeB}},
			{Name: "b-c", VLAN: 182, IPv4: netip.MustParsePrefix("10.181.2.0/24"), Members: []string{nodeB, nodeC}},
		},
		checker: checkStaticLinkChain,
	}
}
