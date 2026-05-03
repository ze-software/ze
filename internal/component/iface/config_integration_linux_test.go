//go:build integration && linux

package iface

import "testing"

func TestIntegrationApplyConfigFirstApplyPreservesUnmanagedDummy(t *testing.T) {
	// VALIDATES: first apply does not adopt/delete manageable links Ze did not own.
	// PREVENTS: startup reconciliation removing pre-existing dummy/veth/bridge links.
	withNetNS(t, func() {
		createDummyForTest(t, "keep0")
		b := GetBackend()
		cfg := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{{
				Name:  "ze0",
				Units: []unitEntry{{Addresses: []string{"10.90.0.1/24"}}},
			}},
		}
		t.Cleanup(func() { _ = DeleteInterface("ze0") })

		if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
			t.Fatalf("applyConfig: %v", errs)
		}

		if !linkExists("ze0") {
			t.Fatal("ze0 should exist after apply")
		}
		requireAddress(t, "ze0", "10.90.0.1/24")
		if !linkExists("keep0") {
			t.Fatal("unmanaged keep0 was deleted on first apply")
		}
	})
}

func TestIntegrationApplyConfigReloadDeletesOnlyPreviouslyManaged(t *testing.T) {
	// VALIDATES: reload deletion is scoped to interfaces in the previous config.
	// PREVENTS: removing arbitrary manageable kernel links that Ze did not own.
	withNetNS(t, func() {
		createDummyForTest(t, "keep0")
		b := GetBackend()
		previous := &ifaceConfig{
			Backend: "netlink",
			Dummy: []ifaceEntry{
				{Name: "old0"},
				{Name: "stay0"},
			},
		}
		t.Cleanup(func() { _ = DeleteInterface("old0") })
		t.Cleanup(func() { _ = DeleteInterface("stay0") })

		if errs := applyConfig(previous, nil, b); len(errs) > 0 {
			t.Fatalf("apply previous config: %v", errs)
		}
		if !linkExists("old0") || !linkExists("stay0") || !linkExists("keep0") {
			t.Fatalf("expected old0, stay0, and keep0 before reload")
		}

		current := &ifaceConfig{
			Backend: "netlink",
			Dummy:   []ifaceEntry{{Name: "stay0"}},
		}
		if errs := applyConfig(current, previous, b); len(errs) > 0 {
			t.Fatalf("apply current config: %v", errs)
		}

		requireNoLink(t, "old0")
		if !linkExists("stay0") {
			t.Fatal("managed stay0 should remain after reload")
		}
		if !linkExists("keep0") {
			t.Fatal("unmanaged keep0 was deleted on reload")
		}
	})
}

func TestIntegrationApplyConfigFailureRollsBackCreatedLinks(t *testing.T) {
	// VALIDATES: partial apply rollback removes created links after a later failure.
	// PREVENTS: failed config commits leaking real kernel interfaces.
	withNetNS(t, func() {
		b := GetBackend()
		cfg := &ifaceConfig{
			Backend: "netlink",
			Dummy:   []ifaceEntry{{Name: "made0"}},
			Bridge: []bridgeEntry{{
				ifaceEntry: ifaceEntry{Name: "br0"},
				Members:    []string{"missing0"},
			}},
		}
		t.Cleanup(func() { _ = DeleteInterface("made0") })
		t.Cleanup(func() { _ = DeleteInterface("br0") })

		errs := applyConfig(cfg, nil, b)
		if len(errs) == 0 {
			t.Fatal("expected applyConfig to fail on missing bridge member")
		}

		requireNoLink(t, "made0")
		requireNoLink(t, "br0")
	})
}
