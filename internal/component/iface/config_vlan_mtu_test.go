package iface

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// mtuVLANConfig is one ethernet eth0 with the given MTU and one VLAN unit
// v100 (eth0.100), as resolved config. The unit carries no MTU of its own:
// the schema has no unit mtu leaf, so a VLAN's MTU is its parent's.
func mtuVLANConfig(mtu int) *ifaceConfig {
	return &ifaceConfig{Ethernet: []ifaceEntry{{
		Name:  "eth0",
		MTU:   mtu,
		Units: []unitEntry{{Label: "v100", VLANID: 100}},
	}}}
}

// TestApplyPropagatesParentMTUToExistingVLAN proves that a reload changing
// only the parent's MTU reaches the VLAN sub-interface that already exists.
// Method: boot with mtu 1500, reload with mtu 9000, and read the VLAN's MTU
// back from the fake. The fake models what Linux does (verified in a network
// namespace on 2026-10-18): a VLAN takes its parent's MTU when it is created
// and keeps it when the parent's MTU later rises. Before the fix the VLAN
// stayed at 1500 under a 9000 parent, so the same config produced a 9000 VLAN
// on a fresh boot and a 1500 one after a reload. The rollback apply (iface
// re-applies the previous config over a failed one) must put 1500 back, and
// an unchanged reload must make no VLAN MTU call.
func TestApplyPropagatesParentMTUToExistingVLAN(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 1500},
	}}
	boot := mtuVLANConfig(1500)
	require.Empty(t, applyConfig(boot, nil, b), "boot apply")
	require.Equal(t, 1500, b.ifaces["eth0.100"].mtu, "the VLAN is created with its parent's MTU")

	raised := mtuVLANConfig(9000)
	require.Empty(t, applyConfig(raised, boot, b), "reload apply")
	require.Equal(t, 9000, b.ifaces["eth0"].mtu, "parent MTU after reload")
	require.Equal(t, 9000, b.ifaces["eth0.100"].mtu, "the existing VLAN follows its parent's raised MTU")

	delete(b.mtuSet, "eth0.100")
	require.Empty(t, applyConfig(raised, raised, b), "unchanged reload")
	_, touched := b.mtuSet["eth0.100"]
	require.False(t, touched, "a VLAN already at its parent's MTU gets no SetMTU call")

	require.Empty(t, applyConfig(boot, raised, b), "rollback apply")
	require.Equal(t, 1500, b.ifaces["eth0"].mtu, "parent MTU after rollback")
	require.Equal(t, 1500, b.ifaces["eth0.100"].mtu, "VLAN MTU after rollback")
}

// TestApplyLeavesVLANMTUAloneWithoutParentMTU proves the propagation acts only
// on an MTU the operator configured: a parent with no mtu leaf gives the VLAN
// nothing to follow, so the apply makes no VLAN MTU call.
func TestApplyLeavesVLANMTUAloneWithoutParentMTU(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 1500},
	}}
	require.Empty(t, applyConfig(mtuVLANConfig(0), nil, b), "boot apply")
	_, touched := b.mtuSet["eth0.100"]
	require.False(t, touched, "no configured parent MTU, no VLAN MTU call")
}
