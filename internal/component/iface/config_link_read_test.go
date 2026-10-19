package iface

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// errAddressDump is the address dump failure addressDumpFailingBackend answers.
var errAddressDump = errors.New("address dump failed")

// addressDumpFailingBackend fails every address read, as the netlink backend
// does when its RTM_GETADDR dump fails: GetInterface and ListInterfaces
// return the error, while GetLink, which reads no address, still answers.
type addressDumpFailingBackend struct {
	*linuxMTUBackend
}

func (b *addressDumpFailingBackend) GetInterface(string) (*InterfaceInfo, error) {
	return nil, errAddressDump
}

// TestApplyKeepsExistingDummyWhenAddressDumpFails proves the apply reads a
// device's existence from its link alone. Method: a dummy d0 already exists,
// its create answers a raw "file exists" (what the kernel says for a device
// an earlier process made), and every address read fails. Reading existence
// through GetInterface made d0 read as absent, so the raw create error failed
// the apply.
//
// VALIDATES: a present device is found present while the address dump fails.
// PREVENTS: an address-dump failure failing the apply of an existing device.
func TestApplyKeepsExistingDummyWhenAddressDumpFails(t *testing.T) {
	resetAddressOwners(t)
	fake := &fakeBackend{
		ifaces:         map[string]fakeIface{"d0": {name: "d0", linkType: "dummy", mtu: 1500}},
		createDummyErr: map[string]error{"d0": errors.New("create dummy d0: file exists")},
	}
	b := &addressDumpFailingBackend{&linuxMTUBackend{fake}}

	errs := applyConfig(&ifaceConfig{Dummy: []ifaceEntry{{Name: "d0"}}}, nil, b)
	require.Empty(t, errs, "the existing dummy is kept")
	require.False(t, fake.deleted["d0"], "nothing deletes the existing dummy")
}

// TestRollbackRestoresVLANMTUWhenAddressDumpFails proves the VLAN MTU undo
// reads the VLAN's old MTU from its link. Method: boot eth0 and VLAN eth0.100
// at 9000, make every address read fail, then apply mtu 1500 with a failing
// VLAN address so the apply rolls back. Reading the old MTU through
// GetInterface found no VLAN, so the parent's undo held no VLAN MTU and the
// rollback left eth0.100 capped at 1500 under a 9000 parent.
//
// VALIDATES: a rolled-back lowering restores the VLAN while addresses fail.
// PREVENTS: an address-dump failure silently skipping a VLAN's MTU undo.
func TestRollbackRestoresVLANMTUWhenAddressDumpFails(t *testing.T) {
	resetAddressOwners(t)
	fake := &fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 9000},
	}}
	b := &addressDumpFailingBackend{&linuxMTUBackend{fake}}
	boot := mtuVLANConfig(9000)
	require.Empty(t, applyConfig(boot, nil, b), "boot apply")
	require.Equal(t, 9000, fake.ifaces["eth0.100"].mtu, "the VLAN is created with its parent's MTU")

	fake.addAddressErr = map[string]error{addressErrKey("eth0.100", "10.0.0.1/24"): errors.New("address refused")}
	errs := applyConfig(failingVLANAddress(mtuVLANConfig(1500)), boot, b)
	require.NotEmpty(t, errs, "the address failure fails the apply")
	for _, err := range errs {
		require.NotContains(t, err.Error(), "rollback partial apply", "the undo itself failed")
	}
	require.Equal(t, 9000, fake.ifaces["eth0"].mtu, "parent MTU after rollback")
	require.Equal(t, 9000, fake.ifaces["eth0.100"].mtu, "VLAN MTU after rollback")
}
