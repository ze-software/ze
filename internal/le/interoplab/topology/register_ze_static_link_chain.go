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

// linkABPrefix is link a-b's IPv4 prefix, which outside routes onto eth0.
var linkABPrefix = netip.MustParsePrefix("10.181.1.0/24")

// outsidePrestart makes outside's probe a test of the link, not of the
// Docker host's routing: it removes eth0's default route, so no gateway can
// carry the ping to some other 10.181.1.0/24 the host reaches, and routes
// link a-b's prefix straight onto eth0, so the ping ARPs on the lab network.
// Only a frame of link a-b that leaked onto eth0's segment answers it.
var outsidePrestart = "ip route del default; ip route add " + linkABPrefix.String() + " dev eth0"

// A -- B -- C: three Ze nodes, each with its own config file, on two IPv4
// links. A and C share no link, so A reaches C only through B, which
// forwards; A and C each route the far link through B in their own static
// config. outside sits on eth0 alone, runs no daemon, and has no default
// route (outsidePrestart).
func init() {
	scenarios[staticLinkChainScenario] = scenario{
		nodes: []node{
			zeNode(nodeA, 2, "ze-a.conf"),
			zeNode(nodeB, 3, "ze-b.conf", "--sysctl", "net.ipv4.ip_forward=1"),
			zeNode(nodeC, 4, "ze-c.conf"),
			{name: "outside", image: zeImage, host: 5,
				capabilities: []string{capabilityNetAdmin}, prestart: outsidePrestart,
				arguments: []string{"--entrypoint", "sleep"}, command: []string{"infinity"}},
		},
		links: []interoplab.Link{
			{Name: "a-b", VLAN: 181, IPv4: linkABPrefix, Members: []string{nodeA, nodeB}},
			{Name: "b-c", VLAN: 182, IPv4: netip.MustParsePrefix("10.181.2.0/24"), Members: []string{nodeB, nodeC}},
		},
		checker: checkStaticLinkChain,
	}
}
