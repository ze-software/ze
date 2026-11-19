// Design: docs/labs/pppoe-interop.md -- native shared-access fixture topology
//go:build ze_l2tp

package pppoe

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	sharedAccessScenario = "pppoe-shared-access-two-clients"
	sharedAccessDevice   = "eth0.100"
	sharedAccessVLAN     = 100
)

// This registry entry adds a topology without changing any existing scenario.
func init() {
	wireCheckers[sharedAccessScenario] = checkSharedAccess
	scenarioPlans[sharedAccessScenario] = sharedAccessPlan
}

// sharedAccessPeer is a fixed fixture declaration, not a subscriber capability.
// Its labels select one independent container; native state is read separately.
type sharedAccessPeer struct{ name, username, device string }

func sharedAccessPeers() [2]sharedAccessPeer {
	return [2]sharedAccessPeer{{"client-a", "alice", "ppp-a"}, {"client-b", "bob", "ppp-b"}}
}

func (peer sharedAccessPeer) logPath() string { return "/var/log/ppp/" + peer.name + ".log" }
func (peer sharedAccessPeer) pidPath() string { return "/var/run/pppd/ppp-" + peer.name + ".pid" }

func sharedAccessPlan(source interoplab.ScenarioSource, suffix string) (interoplab.ScenarioPlan, error) {
	role, err := readRole(source.Directory)
	if err != nil {
		return interoplab.ScenarioPlan{}, err
	}
	if role != roleZeAC {
		return interoplab.ScenarioPlan{}, fmt.Errorf("%s requires role %s", source.Name, roleZeAC)
	}
	base, err := prepareZeAccessConcentrator(source, containerNames(suffix))
	if err != nil {
		return interoplab.ScenarioPlan{}, err
	}
	peers := []interoplab.PeerConfig{base[0]}
	for i, peer := range sharedAccessPeers() {
		peers = append(peers, interoplab.PeerConfig{
			Name: peer.name, Container: "ze-pppoe-" + peer.name + "-" + suffix,
			Image: clientImageTag, Host: uint8(4 + i),
			Capabilities: []string{netAdminCapability}, Arguments: interoplab.PPPDeviceArguments(),
		})
	}
	plan := interoplab.ScenarioPlan{Source: source,
		Network: interoplab.NetworkSpec{Name: "ze-pppoe-" + suffix,
			Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.30.0.0/24")}}},
		Peers: peers}
	for i := range peers {
		plan.Containers = append(plan.Containers, peers[i].Container)
	}
	return plan, nil
}
