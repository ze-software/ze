// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPFv3 GR preservation.
// Related: gr_preserve.go -- captureInterfaceIDs, capturePrefixLSIDs, restorePrefixLSIDs, restoreInterfaceIDs.
// Related: gr_restarter.go -- prepareRestart persists the maps, resumeFromNVS restores them.
//
// VALIDATES: RFC 5187 Section 3.1, "the restarting router MUST preserve the LSA ID to prefix
// correspondence across graceful restarts", and Section 3.2, "the OSPFv3 Interface ID ...
// MUST be preserved by the restarting router across restarts", through the real restart
// path: one engine prepares the restart into a restart-fact store, and a second engine,
// started over that same store, resumes from it.
// PREVENTS: a capture or a restore that does nothing, so the restarted router re-originates
// a prefix under another LSA ID or announces an interface under another Interface ID.
package ospf

import (
	"net/netip"
	"testing"

	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

const rfc5187Iface = "gr-test"

// rfc5187RestartPair runs rfc5187RestartPairOn over rfc5187Iface, an interface the kernel
// does not hold.
func rfc5187RestartPair(t *testing.T) (*engine, *fakeV6Backend, netip.Prefix, uint32) {
	t.Helper()
	return rfc5187RestartPairOn(t, rfc5187Iface)
}

// rfc5187RestartPairOn runs the restart: the first engine redistributes one IPv6 prefix, holds
// Interface ID 1041 for iface, and prepares a graceful restart; the second engine, configured
// the same way over the same store, opens its interfaces and so resumes. It returns the second
// engine, its backend, the prefix, and the LSA ID the first engine used.
func rfc5187RestartPairOn(t *testing.T, iface string) (*engine, *fakeV6Backend, netip.Prefix, uint32) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"`+iface+`":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	store := newFakeGRStore()
	prefix := netip.MustParsePrefix("2001:db8:5187::/48")

	before := newEngineWithCodecAF(ospfv3transport.New(&fakeV6Backend{nextIdx: 10}), v6Codec{}, afIPv6Unicast)
	t.Cleanup(before.shutdown)
	before.state = store
	before.setConfig(cfg)
	before.gr.configure(grTestConfig())
	before.gr.mu.Lock()
	before.gr.preservedIfaceIDs = map[string]uint32{iface: 1041}
	before.gr.mu.Unlock()
	if err := before.openInterfaces(); err != nil {
		t.Fatalf("first engine openInterfaces: %v", err)
	}
	if err := before.v6InjectExternal(prefix, "static", 0); err != nil {
		t.Fatalf("v6InjectExternal: %v", err)
	}
	before.mu.Lock()
	lsid := lsidToUint32(before.redistV6[prefix])
	before.mu.Unlock()
	if err := before.gr.prepareRestart(grReasonReload); err != nil {
		t.Fatalf("prepareRestart: %v", err)
	}

	backend := &fakeV6Backend{nextIdx: 40}
	after := newEngineWithCodecAF(ospfv3transport.New(backend), v6Codec{}, afIPv6Unicast)
	t.Cleanup(after.shutdown)
	after.state = store
	after.setConfig(cfg)
	after.gr.configure(grTestConfig())
	if err := after.openInterfaces(); err != nil {
		t.Fatalf("second engine openInterfaces: %v", err)
	}
	if !after.gr.inRestart() {
		t.Fatal("the second engine did not resume the graceful restart the first one prepared")
	}
	return after, backend, prefix, lsid
}

// TestRFC5187LSAIDToPrefixPreservedAcrossRestart: after the restart the redistributed prefix
// maps to the LSA ID it had before, and a prefix redistributed afterwards cannot take it.
func TestRFC5187LSAIDToPrefixPreservedAcrossRestart(t *testing.T) {
	after, _, prefix, lsid := rfc5187RestartPair(t)

	// RFC requirement: RFC5187-3.1-1 positive -- the LSA ID to prefix correspondence
	// survives a graceful restart: the prefix the first engine redistributed under LSA ID
	// lsid maps to that same LSA ID in the engine that resumed the restart.
	after.mu.Lock()
	got, ok := after.redistV6[prefix]
	after.mu.Unlock()
	if !ok {
		t.Fatalf("the restarted engine holds no LSA ID for %s", prefix)
	}
	if lsidToUint32(got) != lsid {
		t.Fatalf("%s re-maps to LSA ID %d after the restart, want the preserved %d", prefix, lsidToUint32(got), lsid)
	}

	// RFC requirement: RFC5187-3.1-1 negative -- the preserved correspondence is not handed
	// to another prefix: a prefix first redistributed after the restart is refused the
	// preserved LSA ID and gets a different one, and the preserved prefix keeps its own.
	other := netip.MustParsePrefix("2001:db8:5188::/48")
	if err := after.v6InjectExternal(other, "static", 0); err != nil {
		t.Fatalf("v6InjectExternal after restart: %v", err)
	}
	after.mu.Lock()
	otherID := lsidToUint32(after.redistV6[other])
	keptID := lsidToUint32(after.redistV6[prefix])
	after.mu.Unlock()
	if otherID == lsid {
		t.Fatalf("a new prefix %s took the preserved LSA ID %d of %s", other, lsid, prefix)
	}
	if keptID != lsid {
		t.Fatalf("%s lost its preserved LSA ID %d (now %d)", prefix, lsid, keptID)
	}
}

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
