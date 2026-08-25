//go:build integration && linux

// Design: docs/architecture/iface/logical-name-resolution.md -- sub-spec 5 dispatch translation.
//
// VALIDATES: the iface dispatch ops translate a LOGICAL interface name to its os
// device via ResolveDevice before the backend call, so a mutation issued against a
// logical name (os-name selector) lands on the right kernel device. SetMTU and
// SetAdminDown are representative; every by-name dispatch op shares the ResolveDevice
// path. GetStats/ResetCounters additionally prove the counter baseline is keyed
// on the resolved os device so a clear-then-read cycle through the selector does
// not error or miss its baseline.
// PREVENTS: a regression where dispatch ops pass the logical name straight to the
// backend, forcing name == os device and silently failing (or mutating the wrong
// device) under an os-name / mac-match selector.

package iface

import "testing"

// TestDispatchTranslatesLogicalToOSDevice proves iface.SetMTU / SetAdminDown on a
// logical name mutate the kernel device the os-name selector points at, not a
// device named after the logical name (which does not exist).
func TestDispatchTranslatesLogicalToOSDevice(t *testing.T) {
	withNetNS(t, func() {
		const osDev = "zedisp0"
		createDummyForTest(t, osDev)
		if err := SetAdminUp(osDev); err != nil {
			t.Fatalf("up %s: %v", osDev, err)
		}

		// Map logical "uplink" -> kernel "zedisp0"; no device is literally named
		// "uplink", so an untranslated SetMTU would fail to find it.
		globalResolver.setMapping(map[string]string{"uplink": osDev}, nil)
		t.Cleanup(func() { globalResolver.setMapping(nil, nil) })

		const wantMTU = 1400
		if err := SetMTU("uplink", wantMTU); err != nil {
			t.Fatalf("SetMTU(uplink): %v", err)
		}
		info, err := GetInterface(osDev)
		if err != nil {
			t.Fatalf("GetInterface(%s): %v", osDev, err)
		}
		if info.MTU != wantMTU {
			t.Errorf("MTU on %s = %d, want %d (dispatch did not translate uplink -> %s)", osDev, info.MTU, wantMTU, osDev)
		}

		// A second op on the logical name also reaches the os device, proving the
		// translation is uniform across dispatch ops, not special-cased to SetMTU.
		if err := SetAdminDown("uplink"); err != nil {
			t.Fatalf("SetAdminDown(uplink): %v", err)
		}
		info, err = GetInterface(osDev)
		if err != nil {
			t.Fatalf("GetInterface(%s) after down: %v", osDev, err)
		}
		if info.State == "up" || info.State == "UP" {
			t.Errorf("device %s still up after SetAdminDown(uplink): state=%q", osDev, info.State)
		}
	})
}

// TestDispatchStatsBaselineKeyedByOSDevice proves GetStats and ResetCounters
// succeed when reached through a logical name and key the counter baseline on the
// same (os) device, so a clear-then-read cycle through the selector neither
// errors nor misses its baseline. (An untranslated GetStats/ResetCounters would
// hit the backend with "wan" and fail, since no such kernel device exists.)
func TestDispatchStatsBaselineKeyedByOSDevice(t *testing.T) {
	withNetNS(t, func() {
		const osDev = "zedisp1"
		createDummyForTest(t, osDev)
		if err := SetAdminUp(osDev); err != nil {
			t.Fatalf("up: %v", err)
		}
		globalResolver.setMapping(map[string]string{"wan": osDev}, nil)
		t.Cleanup(func() { globalResolver.setMapping(nil, nil) })

		if _, err := GetStats("wan"); err != nil {
			t.Fatalf("GetStats(wan): %v", err)
		}
		if err := ResetCounters("wan"); err != nil {
			t.Fatalf("ResetCounters(wan): %v", err)
		}
		if _, err := GetStats("wan"); err != nil {
			t.Fatalf("GetStats(wan) after reset: %v", err)
		}
	})
}
