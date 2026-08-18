// Design: docs/architecture/iface/logical-name-resolution.md -- the apply path translates
// Related: config_apply.go -- bindDevices, desiredState, applyAndPublish

package iface

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
)

// The MAC the tests bind to, and the one the decoy carries. They differ in the
// last octet so a test that matched on a prefix would still fail.
const (
	selectedPermMAC = "aa:bb:cc:00:00:01"
	decoyPermMAC    = "aa:bb:cc:00:00:99"
)

// selectorBackend returns a fake holding two NICs: enp1s0, the hardware the
// tests select, and a second device named "wan", which is the LOGICAL name the
// config uses. The decoy is the point of the fixture: an apply that keys by the
// logical name finds a device to configure, so the defect shows up as settings
// on the wrong port rather than as a not-found error, which is what made it
// survivable in production.
func selectorBackend() *fakeBackend {
	return &fakeBackend{ifaces: map[string]fakeIface{
		"enp1s0": {name: "enp1s0", linkType: "device", mac: selectedPermMAC, permMAC: selectedPermMAC, index: 2, mtu: 1500, state: "down"},
		"wan":    {name: "wan", linkType: "device", mac: decoyPermMAC, permMAC: decoyPermMAC, index: 3, mtu: 1500, state: "down"},
	}}
}

// macMatchConfig is the config every AC in this file starts from: one ethernet
// entry whose logical name is "wan" and whose hardware is selected by MAC.
func macMatchConfig(units ...unitEntry) *ifaceConfig {
	return &ifaceConfig{
		Ethernet: []ifaceEntry{{Name: "wan", MatchMAC: selectedPermMAC, Units: units}},
	}
}

// TestDesiredStateKeysBySelectedDevice verifies AC-1 at the map that drives the
// address reconcile: the key is the kernel device the selector names, never the
// logical entry name.
//
// VALIDATES: AC-1 -- desiredState keys by the selected device.
// PREVENTS: the address reconcile diffing a name the kernel does not carry.
func TestDesiredStateKeysBySelectedDevice(t *testing.T) {
	cfg := macMatchConfig(unitEntry{Label: "0", Addresses: []string{"10.0.0.1/24"}})
	infos, err := selectorBackend().ListInterfaces()
	require.NoError(t, err)

	addrs, _, _ := cfg.desiredState(cfg.bindDevices(infos))

	require.Contains(t, addrs, "enp1s0", "the desired map must be keyed by the selected device")
	assert.Equal(t, map[string]bool{"10.0.0.1/24": true}, addrs["enp1s0"])
	assert.NotContains(t, addrs, "wan", "the logical name must not key anything")
}

// TestApplyAddressLandsOnSelectedDevice verifies AC-1 and AC-5: the address of a
// mac/match entry reaches the selected NIC, and the device that merely carries
// the entry's logical name is left alone.
//
// VALIDATES: AC-1, AC-5 -- the address lands on the selected hardware only.
// PREVENTS: a management address landing on the wrong physical port, and the
// commit failure that the same defect produced when no such device existed.
func TestApplyAddressLandsOnSelectedDevice(t *testing.T) {
	cfg := macMatchConfig(unitEntry{Label: "0", Addresses: []string{"10.0.0.1/24"}})
	b := selectorBackend()

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	assert.Equal(t, []string{"10.0.0.1/24"}, b.addrs["enp1s0"])
	assert.Empty(t, b.addrs["wan"], "the coincidentally named device must get nothing")
}

// TestApplyAddressFollowsOsNameAlias verifies AC-3: an os-name alias reaches the
// same outcome as a mac/match selector.
//
// VALIDATES: AC-3 -- os-name steers the apply path.
// PREVENTS: the alias working for every consumer except the one that assigns
// the addresses.
func TestApplyAddressFollowsOsNameAlias(t *testing.T) {
	cfg := &ifaceConfig{Ethernet: []ifaceEntry{{
		Name:   "wan",
		OSName: "enp1s0",
		Units:  []unitEntry{{Label: "0", Addresses: []string{"10.0.0.1/24"}}},
	}}}
	b := selectorBackend()

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	assert.Equal(t, []string{"10.0.0.1/24"}, b.addrs["enp1s0"])
	assert.Empty(t, b.addrs["wan"])
}

// TestApplyNonAddressSettingsFollowSelector verifies AC-2 over every naming site
// the apply path has: MTU, the MAC override, admin state, the per-interface
// sysctl keys and the mirror source. Offloads take the same resolved name at the
// one call site applyOffloads has (config_apply.go), and are proven on a live
// kernel by the Linux integration test, because applyOffloads issues ethtool
// ioctls rather than backend calls.
//
// VALIDATES: AC-2 -- every non-address setting lands on the selected device.
// PREVENTS: an entry whose addresses are right and whose MTU, sysctl or mirror
// is applied to another port.
func TestApplyNonAddressSettingsFollowSelector(t *testing.T) {
	bus := newRecordingEventBus()
	var sysctlKeys []string
	bus.Subscribe(sysctlevents.Namespace, sysctlevents.EventDefault, func(p any) {
		var payload struct {
			Key string `json:"key"`
		}
		if s, ok := p.(string); ok && json.Unmarshal([]byte(s), &payload) == nil {
			sysctlKeys = append(sysctlKeys, payload.Key)
		}
	})
	SetEventBus(bus)
	t.Cleanup(func() { SetEventBus(nil) })

	forwarding := true
	cfg := macMatchConfig(unitEntry{
		Label:         "0",
		IPv4:          &ipv4Settings{Forwarding: &forwarding},
		MirrorIngress: "mon0",
	})
	cfg.Ethernet[0].MTU = 9000
	cfg.Ethernet[0].MACAddress = "02:00:00:00:00:01"
	b := selectorBackend()
	b.ifaces["mon0"] = fakeIface{name: "mon0", linkType: "device", index: 4}

	errs := applyConfig(cfg, nil, b)
	require.Empty(t, errs)

	assert.Equal(t, 9000, b.mtuSet["enp1s0"])
	assert.NotContains(t, b.mtuSet, "wan", "MTU must not reach the coincidentally named device")
	assert.Equal(t, "02:00:00:00:00:01", b.macSet["enp1s0"])
	assert.NotContains(t, b.macSet, "wan")
	assert.Equal(t, "up", b.adminSet["enp1s0"])
	assert.NotContains(t, b.adminSet, "wan")
	assert.Contains(t, sysctlKeys, "net.ipv4.conf.enp1s0.forwarding")
	assert.NotContains(t, sysctlKeys, "net.ipv4.conf.wan.forwarding")
	require.Len(t, b.mirrorCalls, 1)
	assert.Equal(t, "enp1s0", b.mirrorCalls[0].iface)
}

// TestVLANOnSelectedParent verifies AC-7 and pins the A-2 answer: a VLAN unit on
// a selected parent is created on the kernel device and carries the kernel
// device's name. Both backends compose the netdev name from the parent they are
// handed, so no other answer names a device that exists.
//
// VALIDATES: AC-7 -- the VLAN is made on the selected parent, named after it.
// PREVENTS: a VLAN created on the wrong parent, and addresses keyed by a VLAN
// name the kernel never assigned.
func TestVLANOnSelectedParent(t *testing.T) {
	cfg := macMatchConfig(unitEntry{Label: "100", VLANID: 100, Addresses: []string{"10.0.100.1/24"}})
	b := selectorBackend()

	errs := applyConfig(cfg, nil, b)

	require.Empty(t, errs)
	require.Contains(t, b.vlans, "enp1s0.100")
	assert.Equal(t, "enp1s0", b.vlans["enp1s0.100"].Parent)
	assert.NotContains(t, b.vlans, "wan.100")
	assert.Equal(t, []string{"10.0.100.1/24"}, b.addrs["enp1s0.100"])
}

// TestVLANOnSelectedParentBoundaryIDs walks the ends of the VLAN id range on a
// selected parent, because the composed name is what this spec changed and a
// four-digit id is the case where a composed name can outgrow IFNAMSIZ.
//
// VALIDATES: the boundary row for "VLAN id on a selected parent" (1 and 4094).
// PREVENTS: a name composed from the logical parent at either end of the range.
func TestVLANOnSelectedParentBoundaryIDs(t *testing.T) {
	for _, vid := range []int{1, 4094} {
		cfg := macMatchConfig(unitEntry{Label: "v", VLANID: vid})
		b := selectorBackend()

		errs := applyConfig(cfg, nil, b)

		require.Emptyf(t, errs, "vlan %d", vid)
		want := unitOSName("enp1s0", &cfg.Ethernet[0].Units[0])
		assert.Containsf(t, b.vlans, want, "vlan %d must be created on the selected parent", vid)
	}
}

// TestDeferredBindingStillDefers verifies AC-4 and validates A-4: a selector no
// device answers to leaves the binding deferred. The commit succeeds, nothing is
// applied, and in particular nothing is applied to the device that shares the
// entry's logical name.
//
// VALIDATES: AC-4 -- an absent binding defers rather than failing the commit.
// PREVENTS: a config the YANG calls valid being refused, and the fallback to
// the logical name that a refusal would otherwise tempt.
func TestDeferredBindingStillDefers(t *testing.T) {
	// Two shapes of "the selected NIC is not here", because the defect had a
	// different symptom in each: with a same-named device present the settings
	// landed on it silently, and with none present the address add failed and
	// took the whole commit down (A-1).
	for _, tc := range []struct {
		name  string
		decoy bool
	}{
		{name: "a device shares the logical name", decoy: true},
		{name: "no device answers to either name", decoy: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := macMatchConfig(unitEntry{Label: "0", Addresses: []string{"10.0.0.1/24"}})
			cfg.Ethernet[0].MTU = 9000
			b := selectorBackend()
			delete(b.ifaces, "enp1s0") // the selected NIC has not appeared yet
			if !tc.decoy {
				delete(b.ifaces, "wan")
			}

			errs := applyConfig(cfg, nil, b)

			require.Empty(t, errs, "a deferred binding must not fail the commit")
			assert.Empty(t, b.addrs["wan"], "no address may reach the device that shares the logical name")
			assert.Empty(t, b.addrs["enp1s0"])
			assert.NotContains(t, b.mtuSet, "wan")
			assert.NotContains(t, b.adminSet, "wan")
		})
	}
}

// TestApplyRefusesWrongDeviceForSelector verifies the fail-closed half of AC-5:
// a mac/match selector two present devices answer to refuses the apply instead
// of binding to one of them. Nothing distinguishes the candidates, so a bind is
// a guess about which physical port the entry's addresses reach.
//
// VALIDATES: AC-5 -- an ambiguous selector is refused, not resolved.
// PREVENTS: a cloned or duplicated MAC silently steering an operator's
// addresses onto a device they did not configure.
func TestApplyRefusesWrongDeviceForSelector(t *testing.T) {
	cfg := macMatchConfig(unitEntry{Label: "0", Addresses: []string{"10.0.0.1/24"}})
	b := selectorBackend()
	b.ifaces["enp2s0"] = fakeIface{name: "enp2s0", linkType: "device", permMAC: selectedPermMAC, index: 5}

	errs := applyConfig(cfg, nil, b)

	require.Len(t, errs, 1)
	assert.ErrorContains(t, errs[0], `ethernet "wan"`)
	assert.ErrorContains(t, errs[0], "enp1s0, enp2s0")
	assert.Empty(t, b.addrs["enp1s0"], "a refused apply changes nothing")
	assert.Empty(t, b.addrs["enp2s0"])
	assert.Empty(t, b.addrs["wan"])
}

// observingBackend records what the shared resolver answered for a logical name
// at the moment the apply reached the backend. That moment is the only place the
// publication ORDER is observable: publishing the mapping after the apply leaves
// this answer on the previous commit's mapping.
type observingBackend struct {
	*fakeBackend
	logical  string
	observed string
}

func (o *observingBackend) SetAdminUp(name string) error {
	if binding, err := Resolve(o.logical); err == nil {
		o.observed = binding.OsName
	}
	return o.fakeBackend.SetAdminUp(name)
}

// TestResolverMappingPublishedBeforeApply verifies AC-6 and validates A-3: the
// apply reads the mapping of its OWN commit. The previous commit bound "wan" to
// a device that no longer exists, so an apply running on the stale mapping
// resolves to nothing and the observation stays empty.
//
// VALIDATES: AC-6 -- a commit that changes a selector applies under the new one.
// PREVENTS: setResolverConfig running after applyConfig, which made every apply
// read the mapping of the commit before it, and a reload that never republished
// at all.
func TestResolverMappingPublishedBeforeApply(t *testing.T) {
	previous := &ifaceConfig{Ethernet: []ifaceEntry{{Name: "wan", OSName: "eth9"}}}
	setResolverConfig(previous)
	t.Cleanup(func() { setResolverConfig(&ifaceConfig{}) })

	ob := &observingBackend{fakeBackend: selectorBackend(), logical: "wan"}
	const backendName = "selector-observer"
	require.NoError(t, RegisterBackend(backendName, func() (Backend, error) { return ob, nil }))
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, backendName)
		backendsMu.Unlock()
	})
	require.NoError(t, LoadBackend(backendName))

	cfg := &ifaceConfig{Ethernet: []ifaceEntry{{
		Name:   "wan",
		OSName: "enp1s0",
		Units:  []unitEntry{{Label: "0", Addresses: []string{"10.0.0.1/24"}}},
	}}}

	errs := applyAndPublish(cfg, previous, ob)

	require.Empty(t, errs)
	assert.Equal(t, "enp1s0", ob.observed,
		"the apply must resolve through its own commit's mapping, not the previous one")
	assert.Equal(t, []string{"10.0.0.1/24"}, ob.addrs["enp1s0"])
}

// decomposeSelectorAddressChange asks the operation decomposer for the change
// that adds an address to an ethernet entry bound by mac/match.
func decomposeSelectorAddressChange(t *testing.T) []tx.ConfigOperation {
	t.Helper()
	const active = `{"interface":{"backend":"test","ethernet":{"wan":{"mac":{"match":"aa:bb:cc:00:00:01"}}}}}`
	const candidate = `{"interface":{"backend":"test","ethernet":{"wan":{"mac":{"match":"aa:bb:cc:00:00:01"},` +
		`"unit":{"0":{"ipv4":{"address":["10.0.0.1/24"]}}}}}}}`

	ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-iface-selector",
		Root:          configRootInterface,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff: tx.DiffSection{
			Root:  configRootInterface,
			Added: `{"interface/ethernet/wan/unit/0/ipv4/address/0":"10.0.0.1/24"}`,
		},
	})
	require.NoError(t, err)
	return ops
}

// TestDecomposedOperationNamesTheSelectedDevice verifies the operation
// decomposer resolves an entry's hardware selector, so the operation the
// executor hands to the backend names the kernel device. The settlement rule
// waits on a monitor event carrying the KERNEL name, so a logical name here
// configures the wrong device and then never settles.
//
// VALIDATES: AC-1 on the operation path, the sibling of the apply path.
// PREVENTS: an address operation decomposed under the logical entry name.
func TestDecomposedOperationNamesTheSelectedDevice(t *testing.T) {
	b := selectorBackend()
	const backendName = "selector-decompose"
	require.NoError(t, RegisterBackend(backendName, func() (Backend, error) { return b, nil }))
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, backendName)
		backendsMu.Unlock()
	})
	require.NoError(t, LoadBackend(backendName))

	ops := decomposeSelectorAddressChange(t)

	require.Len(t, ops, 1)
	assert.Equal(t, tx.OperationAddAddress, ops[0].Type)
	assert.Equal(t, "enp1s0", ops[0].Target.Interface)
	assert.Equal(t, "enp1s0", ops[0].Params.Interface)
}
