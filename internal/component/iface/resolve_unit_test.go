// Design: docs/architecture/iface/logical-name-resolution.md -- a VLAN unit binds through its parent's selector
// Related: resolve.go -- unitDeviceFor, selectedUnitMap, setMapping

package iface

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unitStub holds a selected parent, its VLAN, and a decoy device carrying the
// unit's literal logical name, so a resolver that passes the name through is
// caught binding the decoy. Each VLAN names its parent in ParentIndex and wears
// the parent's MAC, as the netlink backend reports a VLAN (isStackedDevice).
func unitStub() *resolveStubBackend {
	return macStub(
		&InterfaceInfo{Name: "eth5", OsName: "eth5", Index: 5, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:05"},
		&InterfaceInfo{Name: "eth5.100", OsName: "eth5.100", Index: 6, ParentIndex: 5, MTU: 1500, State: "up", MAC: "aa:bb:cc:dd:ee:05"},
		&InterfaceInfo{Name: "eth3", OsName: "eth3", Index: 3, MTU: 1500, State: "up", PermanentMAC: "aa:bb:cc:dd:ee:03"},
		&InterfaceInfo{Name: "eth3.200", OsName: "eth3.200", Index: 7, ParentIndex: 3, MTU: 1500, State: "up", MAC: "aa:bb:cc:dd:ee:03"},
		&InterfaceInfo{Name: "uplink.100", OsName: "uplink.100", Index: 8, MTU: 1500, State: "up", PermanentMAC: "02:00:00:00:00:08"},
		&InterfaceInfo{Name: "core.200", OsName: "core.200", Index: 9, MTU: 1500, State: "up", PermanentMAC: "02:00:00:00:00:09"},
	)
}

// unitResolver maps "uplink" by mac/match onto eth5 and "core" by os-name onto
// eth3, each with one VLAN unit.
func unitResolver() *resolver {
	r := freshResolver()
	r.setMapping(
		map[string]string{"core": "eth3"},
		map[string]string{"uplink": "aa:bb:cc:dd:ee:05"},
		map[string]resolverUnit{
			"uplink.100": {parent: "uplink", vlanID: 100},
			"core.200":   {parent: "core", vlanID: 200},
		},
	)
	return r
}

// TestResolveVLANUnitFollowsItsParentsSelector verifies AC-12: a VLAN unit of
// an entry bound by mac/match or os-name resolves to <parent device>.<vlan-id>.
//
// VALIDATES: the unit's binding is the VLAN of the device the parent's selector
// chose, for both selectors.
// PREVENTS: the measured defect. The resolver looked the unit's logical name up
// literally, so a PPPoE client, a PPPoE server or a protocol naming the unit
// reached the decoy device carrying that name, or none.
func TestResolveVLANUnitFollowsItsParentsSelector(t *testing.T) {
	withResolveBackend(t, unitStub())
	r := unitResolver()

	for name, want := range map[string]string{"uplink.100": "eth5.100", "core.200": "eth3.200"} {
		b, err := r.resolve(name)
		require.NoError(t, err, name)
		assert.Equal(t, want, b.OsName, "%s must be the VLAN of its parent's selected device, not the decoy", name)
	}
}

// TestResolveVLANUnitRefusesWhenItsParentIsUnbound verifies that a unit whose
// parent's selector names no present device resolves to nothing, and counts as
// selector-bound, so ResolveDevice refuses it rather than passing the name on.
//
// VALIDATES: an unbound parent leaves the unit unbound.
// PREVENTS: falling back to the literal name, which is the decoy here.
func TestResolveVLANUnitRefusesWhenItsParentIsUnbound(t *testing.T) {
	stub := unitStub()
	delete(stub.ifaces, "eth5") // the selected NIC has not appeared
	withResolveBackend(t, stub)
	r := unitResolver()

	_, err := r.resolve("uplink.100")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `VLAN "uplink.100": its parent "uplink" resolves to no device`)
	assert.True(t, r.hasSelector("uplink.100"), "a unit of a selector-bound entry must not fall back to its name")
}

// TestSubscribeVLANUnitReceivesItsDevicesEvents verifies that a link event for
// the unit's kernel device reaches a subscriber on the unit's logical name:
// statically for an os-name parent, and through the appearing superset for a
// mac/match parent whose device is not yet known.
//
// VALIDATES: the event path for both selectors.
// PREVENTS: a consumer of the unit (PPPoE, a protocol) never hearing that its
// device appeared or went down, because the event carries the kernel name.
func TestSubscribeVLANUnitReceivesItsDevicesEvents(t *testing.T) {
	withResolveBackend(t, unitStub())
	r := unitResolver()

	for _, tc := range []struct{ logical, kernel string }{
		{"core.200", "eth3.200"},
		{"uplink.100", "eth5.100"},
	} {
		ch, cancel := r.subscribe(tc.logical)
		r.onLinkEvent(tc.kernel, LinkAppeared, 1)
		select {
		case ev := <-ch:
			assert.Equal(t, tc.logical, ev.Name)
			assert.Equal(t, LinkAppeared, ev.Kind)
		case <-time.After(time.Second):
			t.Errorf("no appeared event for %s reached %s", tc.kernel, tc.logical)
		}
		cancel()
	}
}

// TestSelectedUnitMapScope verifies which units the config hands the resolver:
// the VLAN units of ethernet entries carrying mac/match or an aliasing os-name,
// and no other.
//
// VALIDATES: a unit of a plain entry, a unit with no VLAN id, and a unit whose
// name is an ethernet entry's are omitted.
// PREVENTS: re-routing a name that already IS its device, and shadowing an
// entry that is declared directly.
func TestSelectedUnitMapScope(t *testing.T) {
	cfg := &ifaceConfig{Ethernet: []ifaceEntry{
		{Name: "wan", MatchMAC: "aa:bb:cc:dd:ee:01", Units: []unitEntry{{Label: "0"}, {Label: "100", VLANID: 100}}},
		{Name: "lan", OSName: "eth9", Units: []unitEntry{{Label: "200", VLANID: 200}, {Label: "300", VLANID: 300}}},
		{Name: "same", OSName: "same", Units: []unitEntry{{Label: "5", VLANID: 5}}},
		{Name: "plain", Units: []unitEntry{{Label: "7", VLANID: 7}}},
		{Name: "lan.300"},
	}}

	assert.Equal(t, map[string]resolverUnit{
		"wan.100": {parent: "wan", vlanID: 100},
		"lan.200": {parent: "lan", vlanID: 200},
	}, cfg.selectedUnitMap())
}
