//go:build integration && linux

// Design: docs/architecture/iface/logical-name-resolution.md -- resolver os-name remapping.
//
// VALIDATES: spec-iface-resolve-2 AC-1/AC-2/AC-3 end-to-end against the real
// netlink backend: the os-name selector resolves a LOGICAL interface name to a
// DIFFERENT real kernel device (iface.Resolve), the Binding carries the real
// ifindex/MAC/MTU the IS-IS/PPPoE ioctl wrappers produced, and Addresses()
// classifies v4 / v6-link-local / v6-global on a real interface. This is the
// core proof of the logical-name decoupling the unit test (TestResolveByOsName)
// can only show against a stub backend.
// PREVENTS: a regression where Resolve maps a logical name to the kernel by the
// name itself (forcing name == os-name) instead of honoring the selector.

package iface

import "testing"

// TestResolveRemapsLogicalNameToOSDevice proves iface.Resolve("uplink") binds to
// the real kernel device named by the os-name selector, not to a device literally
// named "uplink".
func TestResolveRemapsLogicalNameToOSDevice(t *testing.T) {
	withNetNS(t, func() {
		const osDev = "zeosdev0"
		createDummyForTest(t, osDev)
		if err := SetAdminUp(osDev); err != nil {
			t.Fatalf("set %s up: %v", osDev, err)
		}

		// Map logical "uplink" -> kernel "zeosdev0" (as an iface config os-name
		// selector would via setResolverConfig).
		globalResolver.setMapping(map[string]string{"uplink": osDev}, nil)
		t.Cleanup(func() { globalResolver.setMapping(nil, nil) })

		b, err := Resolve("uplink")
		if err != nil {
			t.Fatalf("Resolve(uplink): %v", err)
		}
		if b.OsName != osDev {
			t.Errorf("OsName = %q, want %q", b.OsName, osDev)
		}

		// Cross-check ifindex/MTU against the real device.
		info, err := GetInterface(osDev)
		if err != nil {
			t.Fatalf("GetInterface(%s): %v", osDev, err)
		}
		if b.Ifindex == 0 || b.Ifindex != info.Index {
			t.Errorf("Ifindex = %d, want real %d", b.Ifindex, info.Index)
		}
		if b.MTU != info.MTU {
			t.Errorf("MTU = %d, want %d", b.MTU, info.MTU)
		}

		// A name with no override resolves to itself (backward compatibility).
		if _, err := Resolve(osDev); err != nil {
			t.Errorf("Resolve(%s) default mapping: %v", osDev, err)
		}

		// A logical name with no device behind it is not-found, not a stale hit.
		if _, err := Resolve("ghost"); err == nil {
			t.Error("Resolve(ghost) must fail for an absent device")
		}
	})
}

// TestAddressesRemapAndClassify proves Addresses() resolves the logical name to
// its OS device and tags address scope correctly against a real interface.
func TestAddressesRemapAndClassify(t *testing.T) {
	withNetNS(t, func() {
		const osDev = "zeosdev1"
		createDummyForTest(t, osDev)
		if err := SetAdminUp(osDev); err != nil {
			t.Fatalf("up: %v", err)
		}
		if err := AddAddress(osDev, "192.0.2.7/24"); err != nil {
			t.Fatalf("add v4: %v", err)
		}
		if err := AddAddress(osDev, "2001:db8::7/64"); err != nil {
			t.Fatalf("add v6: %v", err)
		}

		globalResolver.setMapping(map[string]string{"core": osDev}, nil)
		t.Cleanup(func() { globalResolver.setMapping(nil, nil) })

		addrs, err := Addresses("core")
		if err != nil {
			t.Fatalf("Addresses(core): %v", err)
		}
		var sawV4, sawV6Global bool
		for _, a := range addrs {
			switch {
			case a.Family == "ipv4" && a.Address == "192.0.2.7":
				sawV4 = true
				if a.LinkLocal {
					t.Error("IPv4 must not be classified link-local")
				}
			case a.Family == "ipv6" && a.Address == "2001:db8::7":
				sawV6Global = true
				if a.LinkLocal {
					t.Error("global IPv6 must not be classified link-local")
				}
			}
		}
		if !sawV4 || !sawV6Global {
			t.Errorf("missing classified addresses: v4=%v v6global=%v", sawV4, sawV6Global)
		}
	})
}

// TestResolveByMACBindsToDevice proves iface.Resolve binds a logical name to a
// real kernel device selected by its hardware MAC, not by name, against the
// real netlink backend. Virtual devices report no permanent MAC (confirmed via
// `ethtool -P` on dummy/veth/eth0 in CI), so this exercises the current-MAC
// fallback end-to-end; the permanent-MAC preference itself is unit-tested with
// a stub backend (TestResolveByPermMACPrefersPermanentOverCurrent) because CI
// hosts expose no permanent-address NICs.
func TestResolveByMACBindsToDevice(t *testing.T) {
	withNetNS(t, func() {
		const osDev = "zemacdev0"
		const wantMAC = "02:00:00:00:be:ef"
		createDummyForTest(t, osDev)
		// Set the MAC before bringing the device up (a down device accepts the
		// change unconditionally). Dummy has no permanent MAC, so this becomes
		// the device's only hardware identity.
		if err := SetMACAddress(osDev, wantMAC); err != nil {
			t.Fatalf("set mac on %s: %v", osDev, err)
		}
		if err := SetAdminUp(osDev); err != nil {
			t.Fatalf("up %s: %v", osDev, err)
		}

		// Match logical "uplink" by the device MAC (uppercase to prove
		// normalization), with NO os-name mapping for it.
		globalResolver.setMapping(nil, map[string]string{"uplink": "02:00:00:00:BE:EF"})
		t.Cleanup(func() { globalResolver.setMapping(nil, nil) })

		b, err := Resolve("uplink")
		if err != nil {
			t.Fatalf("Resolve(uplink) by MAC: %v", err)
		}
		if b.OsName != osDev {
			t.Errorf("OsName = %q, want %q (bound by MAC, not name)", b.OsName, osDev)
		}
		info, err := GetInterface(osDev)
		if err != nil {
			t.Fatalf("GetInterface(%s): %v", osDev, err)
		}
		if b.Ifindex == 0 || b.Ifindex != info.Index {
			t.Errorf("Ifindex = %d, want real %d", b.Ifindex, info.Index)
		}

		// A MAC no device carries is a deferred binding, not a stale hit.
		globalResolver.setMapping(nil, map[string]string{"ghost": "de:ad:be:ef:00:99"})
		if _, err := Resolve("ghost"); err == nil {
			t.Error("Resolve(ghost) must fail when no device carries the MAC")
		}
	})
}
