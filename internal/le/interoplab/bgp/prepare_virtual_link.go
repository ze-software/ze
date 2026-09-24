// Design: docs/architecture/testing/interop.md -- routed OSPF virtual-link carriers.
package bgp

import (
	"errors"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	peerFRRTransit            = "frr-transit"
	virtualLinkTransitConfig  = "transit.conf"
	ospfVirtualLinkScenario   = "ospf-virtual-link-frr"
	ospfv3VirtualLinkScenario = "ospfv3-vlink-frr"
)

// VLANs partition the Docker bridge into two data links. eth0 remains management
// only: Ze and the far endpoint share no OSPF segment, so forwarding must cross
// the transit router. All interfaces exist before either routing daemon starts.
const virtualLinkZeSetup = `
ip link add link eth0 name eth1 type vlan id 100
ip address add 10.200.0.1/30 dev eth1
ip -6 address add 2001:db8:100::1/64 dev eth1 nodad
ip link set eth1 up
ip link add name backbone0 type dummy
ip address add 192.0.2.1/32 dev backbone0
ip -6 address add 2001:db8:10::1/128 dev backbone0 nodad
ip link set backbone0 up
`

const virtualLinkEndpointSetup = `
ip link add link eth0 name eth2 type vlan id 200
ip address add 10.200.0.6/30 dev eth2
ip -6 address add 2001:db8:200::2/64 dev eth2 nodad
ip link set eth2 up
ip address add 198.51.100.1/32 dev lo
ip -6 address add 2001:db8:20::1/128 dev lo nodad
`

const virtualLinkTransitSetup = `
ip link add link eth0 name eth1 type vlan id 100
ip address add 10.200.0.2/30 dev eth1
ip -6 address add 2001:db8:100::2/64 dev eth1 nodad
ip link set eth1 up
ip link add link eth0 name eth2 type vlan id 200
ip address add 10.200.0.5/30 dev eth2
ip -6 address add 2001:db8:200::1/64 dev eth2 nodad
ip link set eth2 up
`

func prepareVirtualLinkPeers(peers []interoplab.PeerConfig, scenario string) ([]interoplab.PeerConfig, error) {
	transitConfig := filepath.Join(scenario, virtualLinkTransitConfig)
	if !regularFile(transitConfig) {
		return peers, nil
	}
	endpoints := 0
	for i := range peers {
		peer := &peers[i]
		switch peer.Name {
		case "ze":
			peer.Arguments = append(peer.Arguments, dockerEntrypointFlag, "/bin/sh")
			peer.Command = append([]string{shellErrexitCommand, virtualLinkZeSetup + "exec ze \"$@\"", "--"}, peer.Command...)
		case peerFRR:
			endpoints++
			peer.Arguments = append(peer.Arguments, dockerEntrypointFlag, "/bin/sh")
			peer.Command = []string{shellErrexitCommand, virtualLinkEndpointSetup + "exec /sbin/tini -- /usr/lib/frr/docker-start"}
		case peerBIRD:
			endpoints++
			peer.Host = 3
			peer.Arguments = append(peer.Arguments, ipv6Sysctls()...)
			peer.Arguments = append(peer.Arguments, dockerEntrypointFlag, "/bin/sh")
			peer.Command = []string{shellErrexitCommand, virtualLinkEndpointSetup + "exec tini -- bird -f -c /etc/bird/bird.conf"}
		case peerFRRTransit:
			// Docker applies namespace sysctls before mounting /proc/sys read-only.
			peer.Arguments = append(peer.Arguments,
				"--sysctl", "net.ipv4.ip_forward=1",
				"--sysctl", "net.ipv6.conf.all.forwarding=1",
				dockerEntrypointFlag, "/bin/sh")
			peer.Command = []string{shellErrexitCommand, virtualLinkTransitSetup + "exec /sbin/tini -- /usr/lib/frr/docker-start"}
		}
	}
	if endpoints != 1 {
		return nil, errors.New("virtual-link transit config requires exactly one FRR or BIRD endpoint")
	}
	return peers, nil
}
