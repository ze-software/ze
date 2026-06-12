//go:build integration && linux

package iface

import (
	"testing"
)

func TestIntegrationApplyConfigCreateVLANUnit(t *testing.T) {
	// VALIDATES: config commit with a VLAN unit creates the sub-interface.
	// PREVENTS: regression where applyConfig Phase 2 fails to create VLAN
	// sub-interfaces from the config unit list.
	withNetNS(t, func() {
		b := GetBackend()
		createDummyForTest(t, "parent0")

		cfg := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{{
				Name: "parent0",
				Units: []unitEntry{
					{Label: "default", Addresses: []string{"10.50.0.1/24"}},
					{Label: "100", VLANID: 100, Addresses: []string{"10.50.100.1/24"}},
				},
			}},
		}
		t.Cleanup(func() { _ = DeleteInterface("parent0.100") })

		if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
			t.Fatalf("applyConfig: %v", errs)
		}

		if !linkExists("parent0") {
			t.Fatal("parent0 should exist after apply")
		}
		if !linkExists("parent0.100") {
			t.Fatal("parent0.100 VLAN sub-interface should exist after apply")
		}
		requireAddress(t, "parent0", "10.50.0.1/24")
		requireAddress(t, "parent0.100", "10.50.100.1/24")
	})
}

func TestIntegrationApplyConfigDeleteVLANUnit(t *testing.T) {
	// VALIDATES: config reload that removes a VLAN unit deletes the sub-interface.
	// PREVENTS: regression where applyConfig Phase 4 fails to reconcile removed
	// VLAN sub-interfaces, or where the parent interface is deleted instead.
	withNetNS(t, func() {
		b := GetBackend()
		createDummyForTest(t, "parent0")

		previous := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{{
				Name: "parent0",
				Units: []unitEntry{
					{Label: "default", Addresses: []string{"10.50.0.1/24"}},
					{Label: "100", VLANID: 100, Addresses: []string{"10.50.100.1/24"}},
				},
			}},
		}
		t.Cleanup(func() { _ = DeleteInterface("parent0.100") })

		if errs := applyConfig(previous, nil, b); len(errs) > 0 {
			t.Fatalf("applyConfig (initial): %v", errs)
		}
		if !linkExists("parent0.100") {
			t.Fatal("parent0.100 should exist after initial apply")
		}

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{{
				Name: "parent0",
				Units: []unitEntry{
					{Label: "default", Addresses: []string{"10.50.0.1/24"}},
				},
			}},
		}

		if errs := applyConfig(current, previous, b); len(errs) > 0 {
			t.Fatalf("applyConfig (reload): %v", errs)
		}

		if !linkExists("parent0") {
			t.Fatal("parent interface parent0 must survive unit deletion")
		}
		requireAddress(t, "parent0", "10.50.0.1/24")
		requireNoLink(t, "parent0.100")
	})
}

func TestIntegrationApplyConfigVLANUnitAddressReconcile(t *testing.T) {
	// VALIDATES: config reload that changes addresses on a VLAN unit reconciles
	// correctly (adds new, removes old).
	withNetNS(t, func() {
		b := GetBackend()
		createDummyForTest(t, "parent0")

		previous := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{{
				Name: "parent0",
				Units: []unitEntry{
					{Label: "200", VLANID: 200, Addresses: []string{"10.60.200.1/24"}},
				},
			}},
		}
		t.Cleanup(func() { _ = DeleteInterface("parent0.200") })

		if errs := applyConfig(previous, nil, b); len(errs) > 0 {
			t.Fatalf("applyConfig (initial): %v", errs)
		}
		requireAddress(t, "parent0.200", "10.60.200.1/24")

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{{
				Name: "parent0",
				Units: []unitEntry{
					{Label: "200", VLANID: 200, Addresses: []string{"10.60.200.2/24"}},
				},
			}},
		}

		if errs := applyConfig(current, previous, b); len(errs) > 0 {
			t.Fatalf("applyConfig (reload): %v", errs)
		}

		if !linkExists("parent0.200") {
			t.Fatal("parent0.200 should still exist after address change")
		}
		requireAddress(t, "parent0.200", "10.60.200.2/24")
		if hasAddress("parent0.200", "10.60.200.1/24") {
			t.Error("old address 10.60.200.1/24 should have been removed")
		}
	})
}
