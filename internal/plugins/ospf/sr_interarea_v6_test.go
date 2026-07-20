// VALIDATES: spec-ospf-ext-5 AC-15 (IPv6) -- an ABR re-advertises a learned intra-area
// Prefix-SID into the OTHER active areas inside an E-Inter-Area-Prefix-LSA with NP set and
// E clear (a prefix reached through the ABR is not directly attached), and never back into
// the source area.
// PREVENTS: a lost inter-area Prefix-SID, a PHP flag set on a propagated prefix, a loop
// back into the source area.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/sr"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	ospfv3types "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
)

// RFC requirement: RFC8666-6-9 positive -- a Prefix-SID an ABR propagates between areas
// gets the NP-Flag SET and the E-Flag CLEAR.
// RFC requirement: RFC8666-6-9 negative -- the rule is conditional on the prefix not being
// directly attached to that ABR: a directly-attached prefix keeps its originated flags
// (NP clear, E set here), so the ABR does not rewrite flags unconditionally.
func TestOSPFv3InterAreaPrefixSIDRule(t *testing.T) {
	src := sr.PrefixSID{Flags: sr.SIDFlags{NP: false, E: true}, Index: 7}
	// Propagated (not directly attached): NP set, E clear (RFC 8666 §8.2 / §6).
	got := v6InterAreaPrefixSIDRule(src, false)
	if !got.Flags.NP || got.Flags.E || got.Index != 7 {
		t.Fatalf("propagated Prefix-SID rule = %+v", got)
	}
	// Directly attached: the originated flags are kept.
	got = v6InterAreaPrefixSIDRule(src, true)
	if got.Flags.NP || !got.Flags.E {
		t.Fatalf("directly-attached Prefix-SID must keep its flags: %+v", got)
	}
}

// RFC requirement: RFC8666-8.2-1 positive -- Prefix-SID information is propagated between
// areas: an intra-area Prefix-SID learned in area 1 is re-advertised into the backbone in
// an E-Inter-Area-Prefix-LSA, so multi-area SR works.
// RFC requirement: RFC8666-8.2-1 negative -- propagation is directional, not a broadcast:
// the Prefix-SID is never re-originated back into the source area it was learned from.
func TestOSPFv3OriginateInterAreaPropagation(t *testing.T) {
	eng := newV6RIEngine(t)
	router := types.RouterID{1, 1, 1, 1}
	eng.setConfig(v6RIConfig(t, "area"))
	area1 := types.AreaID{0, 0, 0, 1}
	loop := netip.MustParsePrefix("2001:db8:1::9/128")
	// A neighbor in area 1 advertises an intra-area Prefix-SID for its loopback.
	installRemoteV6EPrefix(t, eng, area1, types.RouterID{9, 9, 9, 9}, 3,
		[]sr.PrefixSIDConfig{{Prefix: loop, Index: 9}})

	keep := map[ospflsdb.SelfLSARef]struct{}{}
	n := eng.v6OriginateInterAreaSR(router, []types.AreaID{types.BackboneArea, area1}, nil, keep)
	if n == 0 {
		t.Fatalf("ABR originated no inter-area Prefix-SID LSA")
	}

	// The propagated Prefix-SID appears in the backbone (destination) area with NP set, and
	// NOT in area 1 (the source area).
	var inBackbone, inSource bool
	for _, r := range eng.v6ReceivedPrefixSIDs() {
		if r.LSType != ospfv3types.LSTypeEInterAreaPrefix || r.Prefix != loop {
			continue
		}
		switch r.Area {
		case types.BackboneArea:
			inBackbone = true
			if !r.SID.Flags.NP || r.SID.Flags.E {
				t.Fatalf("propagated inter-area Prefix-SID must set NP / clear E: %+v", r.SID.Flags)
			}
		case area1:
			inSource = true
		}
	}
	if !inBackbone {
		t.Fatalf("inter-area Prefix-SID not propagated into the backbone")
	}
	if inSource {
		t.Fatalf("inter-area Prefix-SID must not be re-originated into the source area")
	}
}
