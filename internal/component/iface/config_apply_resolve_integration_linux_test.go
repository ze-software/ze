//go:build integration && linux

// Design: docs/architecture/iface/logical-name-resolution.md -- the apply path translates.
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply on a real kernel. An
// ethernet entry whose logical name differs from its kernel device gets its
// address, MTU, admin state and VLAN on the device its selector names, and the
// device that merely carries the logical name is left untouched. The host unit
// tests prove the same through a fake backend; this proves it against netlink,
// which is where the composed VLAN name and the not-found errors are real.
// PREVENTS: the apply path keying by the logical name again -- which either
// failed the whole commit (no such device) or configured a stranger's port (a
// device with that name existed).

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"
)

// selectorTestMAC is the address the mac/match entry binds to. It is locally
// administered (the 0x02 bit) so it can never collide with real hardware.
const selectorTestMAC = "02:00:00:00:be:01"

// setMACForTest gives a device an explicit MAC so a mac/match selector has
// something deterministic to bind to. A dummy reports no permanent address, so
// deviceMatchMAC matches its current one, which is what this sets.
func setMACForTest(t *testing.T, name, mac string) {
	t.Helper()
	if err := SetMACAddress(name, mac); err != nil {
		t.Fatalf("set mac %s on %s: %v", mac, name, err)
	}
}

// TestApplyKeysByMACSelectedDeviceOnKernel proves an apply driven by a mac/match
// selector configures the selected kernel device and leaves the device carrying
// the entry's logical name alone.
func TestApplyKeysByMACSelectedDeviceOnKernel(t *testing.T) {
	withNetNS(t, func() {
		const selected, decoy = "zesel0", "zeselwan"
		createDummyForTest(t, selected)
		createDummyForTest(t, decoy)
		setMACForTest(t, selected, selectorTestMAC)

		cfg := &ifaceConfig{Ethernet: []ifaceEntry{{
			Name:     decoy, // the LOGICAL name, which a real device also carries
			MatchMAC: selectorTestMAC,
			MTU:      1400,
			Units: []unitEntry{
				{Label: "0", Addresses: []string{"198.51.100.1/28"}},
				{Label: "100", VLANID: 100, Addresses: []string{"198.51.100.17/28"}},
			},
		}}}

		if errs := applyConfig(cfg, nil, GetBackend()); len(errs) > 0 {
			t.Fatalf("applyConfig: %v", errs)
		}
		t.Cleanup(func() { _ = DeleteInterface(selected + ".100") })

		requireAddress(t, selected, "198.51.100.1/28")
		requireNoAddress(t, decoy, "198.51.100.1/28")

		info, err := GetInterface(selected)
		if err != nil {
			t.Fatalf("GetInterface(%s): %v", selected, err)
		}
		if info.MTU != 1400 {
			t.Errorf("MTU on %s = %d, want 1400", selected, info.MTU)
		}
		if info.State != "up" && info.State != "UP" {
			t.Errorf("%s state = %q, want up", selected, info.State)
		}
		if decoyInfo, decoyErr := GetInterface(decoy); decoyErr != nil {
			t.Fatalf("GetInterface(%s): %v", decoy, decoyErr)
		} else if decoyInfo.MTU == 1400 {
			t.Errorf("MTU 1400 reached %s, the device that merely shares the logical name", decoy)
		}

		// The VLAN is named after the KERNEL parent, because both backends
		// compose "<parent>.<vid>" from the parent they are handed.
		if _, err := netlink.LinkByName(selected + ".100"); err != nil {
			t.Fatalf("VLAN %s.100 was not created on the selected parent: %v", selected, err)
		}
		if _, err := netlink.LinkByName(decoy + ".100"); err == nil {
			t.Errorf("VLAN %s.100 exists: the VLAN was created on the logical name", decoy)
		}
		requireAddress(t, selected+".100", "198.51.100.17/28")
	})
}

// TestApplyKeysByOSNameAliasOnKernel proves the same for an os-name alias, and
// that the offload block reaches the aliased device: applyOffloads issues
// ethtool ioctls by name, so an unresolved name is the one naming site no
// backend fake can observe.
func TestApplyKeysByOSNameAliasOnKernel(t *testing.T) {
	withNetNS(t, func() {
		const selected, logical = "zesel1", "zealias"
		createDummyForTest(t, selected)
		gro := false

		cfg := &ifaceConfig{Ethernet: []ifaceEntry{{
			Name:    logical, // no kernel device carries this name
			OSName:  selected,
			Offload: &offloadConfig{GRO: &gro},
			Units:   []unitEntry{{Label: "0", Addresses: []string{"198.51.100.33/28"}}},
		}}}

		if errs := applyConfig(cfg, nil, GetBackend()); len(errs) > 0 {
			t.Fatalf("applyConfig: %v", errs)
		}

		requireAddress(t, selected, "198.51.100.33/28")
		if _, err := netlink.LinkByName(logical); err == nil {
			t.Errorf("a device named %q exists: the apply created one under the logical name", logical)
		}
	})
}

// TestApplyDefersAbsentSelectorOnKernel proves AC-4 against netlink: a selector
// no device answers to leaves the commit succeeding and touches nothing, rather
// than failing at the address add as it did before.
func TestApplyDefersAbsentSelectorOnKernel(t *testing.T) {
	withNetNS(t, func() {
		const decoy = "zeselgone"
		createDummyForTest(t, decoy)

		cfg := &ifaceConfig{Ethernet: []ifaceEntry{{
			Name:     decoy,
			MatchMAC: "02:00:00:00:be:ff", // no device carries it
			MTU:      1400,
			Units:    []unitEntry{{Label: "0", Addresses: []string{"198.51.100.49/28"}}},
		}}}

		if errs := applyConfig(cfg, nil, GetBackend()); len(errs) > 0 {
			t.Fatalf("a deferred binding must not fail the commit: %v", errs)
		}
		requireNoAddress(t, decoy, "198.51.100.49/28")
		info, err := GetInterface(decoy)
		if err != nil {
			t.Fatalf("GetInterface(%s): %v", decoy, err)
		}
		if info.MTU == 1400 {
			t.Errorf("MTU 1400 reached %s while the selector matched nothing", decoy)
		}
	})
}
