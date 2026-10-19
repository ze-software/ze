package iface

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errLeftover is the create error the leftover backend answers with.
var errLeftover = errors.New("create refused after the device was made")

// leftoverBackend is the fake backend with one failure mode: a create that
// makes the device and then reports an error, leaving the device behind. It
// models a backend whose create failed half way and whose cleanup failed too,
// such as a VPP sub-interface whose QoS maps were refused and whose delete
// was refused as well. fail names the devices whose create behaves so.
type leftoverBackend struct {
	*fakeBackend
	fail map[string]bool
}

func (b *leftoverBackend) leave(name string, err error) error {
	if err != nil {
		return err
	}
	if b.fail[name] {
		return errLeftover
	}
	return nil
}

func (b *leftoverBackend) CreateVLAN(spec VLANSpec) error {
	var nb textbuf.Buffer
	name := nb.Str(spec.Parent).Byte('.').Int(int64(spec.VLANID)).String()
	return b.leave(name, b.fakeBackend.CreateVLAN(spec))
}

func (b *leftoverBackend) CreateDummy(name string) error {
	return b.leave(name, b.fakeBackend.CreateDummy(name))
}

func (b *leftoverBackend) CreateVeth(name, peerName string) error {
	return b.leave(name, b.fakeBackend.CreateVeth(name, peerName))
}

func (b *leftoverBackend) CreateBridge(name string) error {
	return b.leave(name, b.fakeBackend.CreateBridge(name))
}

func (b *leftoverBackend) CreateTunnel(spec TunnelSpec) error {
	return b.leave(spec.Name, b.fakeBackend.CreateTunnel(spec))
}

func (b *leftoverBackend) CreateWireguardDevice(name string) error {
	return b.leave(name, b.fakeBackend.CreateWireguardDevice(name))
}

func (b *leftoverBackend) CreateXFRM(spec XFRMSpec) error {
	return b.leave(spec.Name, b.fakeBackend.CreateXFRM(spec))
}

// TestApplyFailsCreateThatLeavesDeviceBehind proves a create error reaches
// the commit even when the failed create left its device behind. Method: for
// each kind of device the apply creates, a backend makes the device, then
// returns an error, on a host where the device did not exist before. Before
// the fix every create step decided "already exists" by reading the device
// AFTER the failed attempt, found the leftover, and returned nil, so the
// commit reported success over a device the backend had just said it could
// not build.
//
// VALIDATES: a create error for a device absent before the step fails the apply.
// PREVENTS: a leftover from a failed create read as a device an earlier apply made.
func TestApplyFailsCreateThatLeavesDeviceBehind(t *testing.T) {
	cases := []struct {
		name   string
		device string
		cfg    *ifaceConfig
	}{
		{"vlan", "eth0.100", &ifaceConfig{Ethernet: []ifaceEntry{{
			Name: "eth0", Units: []unitEntry{{Label: "v100", VLANID: 100}},
		}}}},
		{"dummy", "dum0", &ifaceConfig{Dummy: []ifaceEntry{{Name: "dum0"}}}},
		{"veth", "veth0", &ifaceConfig{Veth: []vethEntry{{Name: "veth0", Peer: "veth1"}}}},
		{"bridge", "br0", &ifaceConfig{Bridge: []bridgeEntry{{Name: "br0"}}}},
		{"tunnel", "gre0", &ifaceConfig{Tunnel: []tunnelEntry{{
			Name: "gre0",
			Spec: TunnelSpec{Kind: TunnelKindGRE, Name: "gre0"},
		}}}},
		{"wireguard", "wg0", &ifaceConfig{Wireguard: []wireguardEntry{{
			Name: "wg0",
			Spec: WireguardSpec{Name: "wg0"},
		}}}},
		{"xfrm", "xfrm0", &ifaceConfig{XFRM: []xfrmEntry{{
			Name: "xfrm0",
			Spec: XFRMSpec{Name: "xfrm0", IfID: 7},
		}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetAddressOwners(t)
			b := &leftoverBackend{
				fakeBackend: &fakeBackend{ifaces: map[string]fakeIface{
					"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 1500},
				}},
				fail: map[string]bool{tc.device: true},
			}
			errs := applyConfig(tc.cfg, nil, b)
			require.NotEmpty(t, errs, "a failed create of an absent device fails the apply")
			found := false
			for _, err := range errs {
				if errors.Is(err, errLeftover) {
					found = true
				}
			}
			require.True(t, found, "the backend's create error reaches the commit: %v", errs)
			_, left := b.ifaces[tc.device]
			require.True(t, left, "the fake left the device behind, so the test exercised the swallow")
		})
	}
}

// TestApplyKeepsExistingVLANAndReprogramsQoS proves the other side of the
// decision: a VLAN that existed before the step is kept when its create
// answers "exists", its QoS maps are reprogrammed, and a later failure's
// undo does not delete it. Method: boot eth0 with VLAN eth0.100 and no maps,
// then reload with an ingress map, once cleanly and once with an address on
// the VLAN that fails.
//
// VALIDATES: an existing VLAN survives create, gets UpdateVLANQoSMap, and is not undone.
// PREVENTS: the before-create read deleting or failing a VLAN an earlier apply made.
func TestApplyKeepsExistingVLANAndReprogramsQoS(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 1500},
	}}
	boot := mtuVLANConfig(0)
	require.Empty(t, applyConfig(boot, nil, b), "boot apply")

	mapped := mtuVLANConfig(0)
	mapped.Ethernet[0].Units[0].IngressQoSMap = map[uint32]uint32{1: 2}
	require.Empty(t, applyConfig(mapped, boot, b), "reload over the existing VLAN")
	require.Len(t, b.qosUpdates, 1, "the existing VLAN's maps are reprogrammed")
	require.Equal(t, "eth0.100", b.qosUpdates[0].name)

	failing := mtuVLANConfig(0)
	failing.Ethernet[0].Units[0].IngressQoSMap = map[uint32]uint32{3: 4}
	b.addAddressErr = map[string]error{addressErrKey("eth0.100", "10.0.0.1/24"): errors.New("address refused")}
	errs := applyConfig(failingVLANAddress(failing), mapped, b)
	require.NotEmpty(t, errs, "the address failure fails the apply")
	_, kept := b.ifaces["eth0.100"]
	require.True(t, kept, "the undo leaves a VLAN this apply did not create")
	require.False(t, b.deleted["eth0.100"], "no delete of the existing VLAN")
}

// TestApplyUndoDeletesVLANItCreated proves the undo still removes a VLAN the
// failed apply made. Method: one apply creates eth0.100 and then fails on an
// address on it.
//
// VALIDATES: a VLAN created by a failed apply is deleted by its rollback.
// PREVENTS: the before-create read marking a fresh VLAN as pre-existing.
func TestApplyUndoDeletesVLANItCreated(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{
		"eth0": {name: "eth0", linkType: zeTypeEthernet, mtu: 1500},
	}}
	b.addAddressErr = map[string]error{addressErrKey("eth0.100", "10.0.0.1/24"): errors.New("address refused")}
	errs := applyConfig(failingVLANAddress(mtuVLANConfig(0)), nil, b)
	require.NotEmpty(t, errs, "the address failure fails the apply")
	require.True(t, b.deleted["eth0.100"], "the rollback deletes the VLAN the apply created")
}
