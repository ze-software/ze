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

// A -- B -- C: three Ze nodes, each with its own config file, on two IPv4
// links. A and C share no link, so A reaches C only through B, which
// forwards; A and C each route the far link through B in their own static
// config. outside sits on eth0 alone and runs no daemon.
func init() {
	scenarios[staticLinkChainScenario] = scenario{
		nodes: []node{
			zeNode(nodeA, 2, "ze-a.conf"),
			zeNode(nodeB, 3, "ze-b.conf", "--sysctl", "net.ipv4.ip_forward=1"),
			zeNode(nodeC, 4, "ze-c.conf"),
			{name: "outside", image: zeImage, host: 5,
				arguments: []string{"--entrypoint", "sleep"}, command: []string{"infinity"}},
		},
		links: []interoplab.Link{
			{Name: "a-b", VLAN: 181, IPv4: netip.MustParsePrefix("10.181.1.0/24"), Members: []string{nodeA, nodeB}},
			{Name: "b-c", VLAN: 182, IPv4: netip.MustParsePrefix("10.181.2.0/24"), Members: []string{nodeB, nodeC}},
		},
		checker: checkStaticLinkChain,
	}
}
