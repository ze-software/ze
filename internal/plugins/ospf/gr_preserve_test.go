// VALIDATES: the RFC 5187 sec 3.1 / sec 3.2 OSPFv3 preservation across a restart -- the
// LSA-ID -> prefix correspondence for redistributed External LSAs (AC-10) and the OSPFv3
// Interface ID per interface (AC-9) are captured, persisted, and restored so re-originated
// LSAs carry the same identifiers as before the restart.
// PREVENTS: network churn (a prefix re-originated under a different LSA-ID) or a silently
// terminated restart (a renumbered Interface ID mismatching neighbor adjacency state).
package ospf

import (
	"net/netip"
	"testing"

	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestLSAIDPrefixCorrespondencePreserved (AC-10, A-12, R-7): a redistributed IPv6 prefix's
// arbitrary 32-bit LSA ID survives capture -> persist form -> restore into a fresh engine.
func TestLSAIDPrefixCorrespondencePreserved(t *testing.T) {
	src := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	pfx := netip.MustParsePrefix("2001:db8:1::/48")
	src.redistV6[pfx] = v6SummaryLSID(77)

	captured := src.capturePrefixLSIDs()
	if captured[pfx.String()] != 77 {
		t.Fatalf("capturePrefixLSIDs: got %v want lsid 77 for %s", captured, pfx)
	}

	// Simulate the restart: a fresh engine restores the map.
	dst := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	dst.restorePrefixLSIDs(captured)
	if got := dst.redistV6[pfx]; got != v6SummaryLSID(77) {
		t.Fatalf("restorePrefixLSIDs: prefix %s got LSID %v want %v", pfx, got, v6SummaryLSID(77))
	}
}

// TestInterfaceIDPreservedAcrossRestart (AC-9, A-12, R-6): the OSPFv3 Interface ID map is
// captured and restored so grInterfaceID returns the preserved value on resume.
func TestInterfaceIDPreservedAcrossRestart(t *testing.T) {
	dst := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	dst.restoreInterfaceIDs(map[string]uint32{"eth0": 12, "eth1": 34})
	if got := dst.grInterfaceID("eth0"); got != 12 {
		t.Fatalf("grInterfaceID(eth0) = %d, want preserved 12", got)
	}
	if got := dst.grInterfaceID("eth1"); got != 34 {
		t.Fatalf("grInterfaceID(eth1) = %d, want preserved 34", got)
	}
	// An interface with no preserved ID falls back to the live kernel ifindex (0 in tests).
	if got := dst.grInterfaceID("eth9"); got != interfaceIndex("eth9") {
		t.Fatalf("grInterfaceID(eth9) = %d, want live ifindex %d", got, interfaceIndex("eth9"))
	}
}

// TestLSIDRoundTrip guards the uint32 <-> LinkStateID conversion the preservation maps use.
func TestLSIDRoundTrip(t *testing.T) {
	for _, v := range []uint32{0, 1, 77, 0xFFFFFFFF} {
		if got := lsidToUint32(v6SummaryLSID(v)); got != v {
			t.Fatalf("lsidToUint32(v6SummaryLSID(%d)) = %d", v, got)
		}
	}
	_ = ospftypes.LinkStateID{}
}
