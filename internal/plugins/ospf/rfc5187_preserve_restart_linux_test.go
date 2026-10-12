// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPFv3 GR preservation.
// Related: rfc5187_preserve_restart_test.go -- rfc5187RestartPairOn, the restart this unit drives.
// Related: gr_preserve.go -- grInterfaceID, restoreInterfaceIDs.
//
// VALIDATES: RFC 5187 Section 3.2, "the OSPFv3 Interface ID ... MUST be preserved by the
// restarting router across restarts", against the Interface ID a fresh start takes: the
// kernel ifindex interfaceIndex reads through the iface component.
// PREVENTS: a restore that does nothing, so the restarted router announces the interface
// under its fresh-start Interface ID.
//
// Linux only: interfaceIndex asks the iface backend, and the only interface backend is
// netlink. On another host it answers 0 for every name, so the fresh-start ID this unit
// compares against does not exist there.

//go:build linux

package ospf

import "testing"

// TestRFC5187InterfaceIDPreservedAcrossRestart: after the restart the interface carries the
// Interface ID it had before, not the ID a fresh start would take. The unit runs on lo: its
// kernel ifindex is the default OSPFv3 Interface ID (interfaceIndex), so it is exactly the ID a
// restart that failed to restore the preserved one would announce.
func TestRFC5187InterfaceIDPreservedAcrossRestart(t *testing.T) {
	const iface = "lo"
	after, _, _, _ := rfc5187RestartPairOn(t, iface)
	// Asked after the engines opened their interfaces, which loads the interface backend
	// interfaceIndex reads.
	kernelID := interfaceIndex(iface)
	if kernelID == 0 {
		t.Fatalf("the host has no %s interface; the unit needs its kernel ifindex", iface)
	}
	if kernelID == 1041 {
		t.Fatalf("%s has kernel ifindex 1041, the preserved ID; the unit cannot tell them apart", iface)
	}
	announced := func() uint32 {
		t.Helper()
		for _, info := range after.lsdbTopology() {
			if info.Name == iface {
				return info.InterfaceID
			}
		}
		t.Fatalf("the resumed engine's topology has no %s", iface)
		return 0
	}

	// RFC requirement: RFC5187-3.2-1 negative -- the restarted router does not take a new
	// Interface ID for the interface: neither the ID the engine resolves for it nor the one its
	// topology announces is the kernel ifindex a fresh start assigns by default.
	if resolved := after.grInterfaceID(iface); resolved == kernelID {
		t.Fatalf("%s resolves to its fresh-start Interface ID %d (kernel ifindex), not the preserved one", iface, resolved)
	}
	if got := announced(); got == kernelID {
		t.Fatalf("%s announces its fresh-start Interface ID %d (kernel ifindex) after the restart", iface, got)
	}

	// RFC requirement: RFC5187-3.2-1 positive -- the OSPFv3 Interface ID is preserved across
	// the restart: the interface the first engine announced as Interface ID 1041 carries 1041
	// in the resumed engine's LSDB topology.
	if got := announced(); got != 1041 {
		t.Fatalf("%s announces Interface ID %d after the restart, want the preserved 1041", iface, got)
	}
}
