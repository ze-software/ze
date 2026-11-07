// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: check_two_ze_pppoe_link.go -- the checker this file registers.
package topology

import "github.com/ze-software/ze/internal/le/interoplab"

const pppoeLinkScenario = "two-ze-pppoe-link"

// A Ze PPPoE access concentrator and a Ze PPPoE client on a link with no
// prefix, the access segment's shape: PPPoE discovery and the session run on
// each node's eth1 and nothing else. observer sits on eth0 alone and records
// every untagged PPPoE discovery frame it receives, from before the client
// starts.
func init() {
	scenarios[pppoeLinkScenario] = scenario{
		nodes: []node{
			zeNode("ze-server", 2, "ze-server.conf", interoplab.PPPDeviceArguments()...),
			{name: "observer", image: zeImage, host: 4,
				arguments: []string{"--entrypoint", "tcpdump"},
				command:   []string{"-p", "-n", "-U", "-i", "eth0", "-w", observerCapture, "ether proto 0x8863"}},
			zeNode("ze-client", 3, "ze-client.conf", interoplab.PPPDeviceArguments()...),
		},
		links: []interoplab.Link{
			{Name: "access", VLAN: 183, Members: []string{"ze-server", "ze-client"}},
		},
		checker: checkPPPoELink,
	}
}

// observerCapture is where observer's capture lands inside its container.
const observerCapture = "/run/padi.pcap"
