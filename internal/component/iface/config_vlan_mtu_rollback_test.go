package iface

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// linuxMTUBackend is the fake backend with the two Linux VLAN MTU rules the
// rollback depends on, both observed in a user network namespace on
// 2026-10-18 (dummy d0 with VLAN d0.100): lowering a parent's MTU caps every
// VLAN above it to the new value, and a VLAN refuses an MTU above its parent's
// with ERANGE ("Numerical result out of range"). Raising the parent leaves its
// VLANs where they are.
type linuxMTUBackend struct {
	*fakeBackend
}

func (b *linuxMTUBackend) SetMTU(name string, mtu int) error {
	dev, ok := b.ifaces[name]
	if ok && dev.linkType == "vlan" {
		parent := name[:strings.LastIndexByte(name, '.')]
		if mtu > b.ifaces[parent].mtu {
			return errors.New("set mtu " + name + ": numerical result out of range")
		}
	}
	if err := b.fakeBackend.SetMTU(name, mtu); err != nil {
		return err
	}
	for vlanName, vlan := range b.ifaces {
		if vlan.linkType != "vlan" {
			continue
		}
		if !strings.HasPrefix(vlanName, name+".") {
			continue
		}
		if vlan.mtu > mtu {
			vlan.mtu = mtu
			b.ifaces[vlanName] = vlan
		}
	}
	return nil
}

// failingVLANAddress makes the address on the VLAN unit fail, which fails the
// apply after its MTU steps and rolls the partial apply back.
func failingVLANAddress(cfg *ifaceConfig) *ifaceConfig {
	cfg.Ethernet[0].Units[0].Addresses = []string{"10.0.0.1/24"}
	return cfg
}

// TestRollbackRestoresVLANMTUAfterLowering proves a failed commit that
// lowered a parent's MTU puts the VLAN's old, higher MTU back. Method: boot
// eth0 at 9000 with VLAN eth0.100 (9000), then apply mtu 1500 with an address
// on the VLAN that fails, on a backend capping VLANs as Linux does. Before the
// fix the cap left the VLAN at 1500 before its own step read it, so no undo
// was recorded and the rollback left eth0 at 9000 over a 1500 VLAN. The undo
// also has to run parent first: a VLAN restored first is refused by a parent
// still at 1500, which surfaces as a rollback error.
//
// VALIDATES: the parent's undo restores the parent, then each VLAN's
// pre-commit MTU.
// PREVENTS: a rolled-back lowering leaving a VLAN capped below its parent.
func TestRollbackRestoresVLANMTUAfterLowering(t *testing.T) {
	resetAddressOwners(t)
	b := &linuxMTUBackend{&fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 9000},
	}}}
	boot := mtuVLANConfig(9000)
	require.Empty(t, applyConfig(boot, nil, b), "boot apply")
	require.Equal(t, 9000, b.ifaces["eth0.100"].mtu, "the VLAN is created with its parent's MTU")

	b.addAddressErr = map[string]error{addressErrKey("eth0.100", "10.0.0.1/24"): errors.New("address refused")}
	errs := applyConfig(failingVLANAddress(mtuVLANConfig(1500)), boot, b)
	require.NotEmpty(t, errs, "the address failure fails the apply")
	for _, err := range errs {
		require.NotContains(t, err.Error(), "rollback partial apply", "the undo itself failed")
	}
	require.Equal(t, 9000, b.ifaces["eth0"].mtu, "parent MTU after rollback")
	require.Equal(t, 9000, b.ifaces["eth0.100"].mtu, "VLAN MTU after rollback")
}

// TestRollbackRestoresVLANMTUAfterRaising proves the reverse direction: a
// failed commit that raised the parent and its VLAN to 9000 puts both back at
// 1500 with no undo error, the parent's restore capping the VLAN on the way.
//
// VALIDATES: a rolled-back raise returns parent and VLAN to their old MTU.
// PREVENTS: a VLAN restore refused because it ran against the wrong parent MTU.
func TestRollbackRestoresVLANMTUAfterRaising(t *testing.T) {
	resetAddressOwners(t)
	b := &linuxMTUBackend{&fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 1500},
	}}}
	boot := mtuVLANConfig(1500)
	require.Empty(t, applyConfig(boot, nil, b), "boot apply")

	b.addAddressErr = map[string]error{addressErrKey("eth0.100", "10.0.0.1/24"): errors.New("address refused")}
	errs := applyConfig(failingVLANAddress(mtuVLANConfig(9000)), boot, b)
	require.NotEmpty(t, errs, "the address failure fails the apply")
	for _, err := range errs {
		require.NotContains(t, err.Error(), "rollback partial apply", "the undo itself failed")
	}
	require.Equal(t, 1500, b.ifaces["eth0"].mtu, "parent MTU after rollback")
	require.Equal(t, 1500, b.ifaces["eth0.100"].mtu, "VLAN MTU after rollback")
}
