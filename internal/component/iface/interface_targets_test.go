// Design: docs/architecture/iface/logical-name-resolution.md -- a configured-interface name binds through its entry's selector
// Related: interface_targets.go -- interfaceTargets, tunnelSpecOnDevices
// Related: config_bridge_members.go -- enslaveBridgeMembers

package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBridgeMemberVLANUnitFollowsItsParentsSelector verifies AC-10: a bridge
// member naming a VLAN unit of an entry bound by mac/match is enslaved as the
// VLAN of the device the selector chose.
//
// VALIDATES: the member enslaved is <bound device>.<vlan-id>, never the device
// carrying the unit's logical name.
// PREVENTS: the measured defect. Phase 2a handed BridgeAddPort the unit name
// unchanged, because only entries are in the selector map. With a decoy device
// carrying that name the decoy was enslaved; without one the apply failed.
func TestBridgeMemberVLANUnitFollowsItsParentsSelector(t *testing.T) {
	b := selectorBackend()
	b.ifaces["wan.100"] = fakeIface{name: "wan.100", linkType: "device", index: 9, mtu: 1500, state: "down"}
	cfg := &ifaceConfig{
		Backend:  "kernel",
		Ethernet: []ifaceEntry{{Name: "wan", MatchMAC: selectedPermMAC, Units: []unitEntry{{Label: "100", VLANID: 100}}}},
		Bridge:   []bridgeEntry{{Name: "br0", Members: []string{"wan.100"}}},
	}

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	require.Len(t, b.bridgePorts, 1)
	assert.Equal(t, bridgePortCall{op: bridgePortOpAdd, bridge: "br0", port: "enp1s0.100"}, b.bridgePorts[0],
		"the VLAN unit member must be the VLAN of the device its parent's selector names")
}

// TestBridgeMemberVLANUnitWaitsForTheVLAN verifies the ordering half of AC-10:
// on a fresh apply, a member naming a VLAN unit this same config declares is
// enslaved after the VLAN exists.
//
// VALIDATES: the apply succeeds and enslaves the VLAN device.
// PREVENTS: the measured defect. Phase 2a ran before the Phase 2 loop that
// creates VLANs, so BridgeAddPort named a device that did not exist yet, and
// the whole apply rolled back. The fake refuses a port it holds no device for,
// as the kernel does.
func TestBridgeMemberVLANUnitWaitsForTheVLAN(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	cfg := &ifaceConfig{
		Backend: "kernel",
		Dummy:   []ifaceEntry{{Name: "d0", Units: []unitEntry{{Label: "100", VLANID: 100}}}},
		Bridge:  []bridgeEntry{{Name: "br0", Members: []string{"d0.100"}}},
	}

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	require.Len(t, b.bridgePorts, 1)
	assert.Equal(t, bridgePortCall{op: bridgePortOpAdd, bridge: "br0", port: "d0.100"}, b.bridgePorts[0])
}

// selectedTunnelConfig is a gre tunnel whose underlay is the ethernet entry
// "wan", bound to hardware by MAC.
func selectedTunnelConfig() *ifaceConfig {
	return &ifaceConfig{
		Backend:  "kernel",
		Ethernet: []ifaceEntry{{Name: "wan", MatchMAC: selectedPermMAC}},
		Tunnel: []tunnelEntry{{
			Name: "gre0",
			Spec: TunnelSpec{
				Kind:           TunnelKindGRE,
				Name:           "gre0",
				LocalInterface: "wan",
				RemoteAddress:  "198.51.100.1",
			},
		}},
	}
}

// TestTunnelLocalInterfaceFollowsItsSelector verifies AC-11: a tunnel whose
// local interface names an entry bound by mac/match is created on the device
// the selector chose.
//
// VALIDATES: the spec reaching the backend names the bound device.
// PREVENTS: the measured defect. Phase 1 handed the backend the logical name,
// which looks the device up by name, so the tunnel took its parent and source
// address from the decoy device that merely carries the entry's name.
func TestTunnelLocalInterfaceFollowsItsSelector(t *testing.T) {
	b := selectorBackend()

	errs := applyConfig(selectedTunnelConfig(), nil, b)

	require.Empty(t, errs)
	require.Contains(t, b.tunnels, "gre0")
	assert.Equal(t, "enp1s0", b.tunnels["gre0"].LocalInterface,
		"the tunnel's underlay must be the device the selector names, not the decoy carrying the logical name")
}

// TestTunnelOnAnUnboundLocalInterfaceFailsNamingIt verifies the unbound half of
// AC-11: with the selected NIC absent, no tunnel is created and the apply fails
// with a message naming the tunnel and the entry.
//
// VALIDATES: an unbound underlay is reported, never replaced by the logical name.
// PREVENTS: creating the tunnel over the decoy device that carries the entry's
// name, which is what passing the logical name through did.
func TestTunnelOnAnUnboundLocalInterfaceFailsNamingIt(t *testing.T) {
	b := selectorBackend()
	delete(b.ifaces, "enp1s0") // the selected NIC has not appeared

	errs := applyConfig(selectedTunnelConfig(), nil, b)

	require.NotEmpty(t, errs)
	joined := errs[0].Error()
	assert.Contains(t, joined, `tunnel "gre0" local interface "wan"`)
	assert.Contains(t, joined, "no present device answers")
	assert.NotContains(t, b.tunnels, "gre0", "no tunnel may be created over the decoy")
}
