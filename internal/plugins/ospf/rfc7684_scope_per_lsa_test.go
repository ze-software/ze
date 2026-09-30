package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: RFC 7684 Section 2.1, every prefix in one originated Extended Prefix Opaque LSA
// is satisfied by that LSA's flooding scope.
// PREVENTS: an AS-wide prefix packed into an area-scope LSA beside area-local prefixes.

// scopeOriginations originates a router's Extended Prefix LSAs with three prefixes that need
// area scope (a connected stub, a Type-3 summary into area 1, and the stub's prefix summarized
// into area 1) and two that need AS scope (an external, and the stub's own prefix
// redistributed as an external), and returns every non-withdraw origination.
func scopeOriginations(t *testing.T) []opaqueOrigination {
	t.Helper()
	topo := []ospflsdb.InterfaceInfo{extStubIface("eth0", [4]byte{10, 0, 0, 1}, [4]byte{255, 255, 255, 0})}
	eng, router := extEngineWithTopology(t, topo)
	area1 := types.AreaID{0, 0, 0, 1}
	mask24 := [4]byte{255, 255, 255, 0}
	eng.lsdb.OriginateSummary(area1, router, 0, types.LSTypeSummaryNetwork, types.LinkStateID{10, 9, 9, 0}, mask24, 20)
	eng.lsdb.OriginateSummary(area1, router, 0, types.LSTypeSummaryNetwork, types.LinkStateID{10, 0, 0, 0}, mask24, 10)
	for _, pfx := range [][4]byte{{198, 51, 100, 0}, {10, 0, 0, 0}} {
		if _, _, err := eng.lsdb.OriginateExternal(router, pfx, mask24, 0, true, 30, [4]byte{}, 0); err != nil {
			t.Fatalf("OriginateExternal %v: %v", pfx, err)
		}
	}
	var out []opaqueOrigination
	for _, o := range eng.extPrefixOnOriginate(router) {
		if o.Withdraw {
			continue
		}
		out = append(out, o)
	}
	return out
}

// TestRFC7684EveryPrefixInAnLSAFitsItsScope proves origination never mixes prefixes that
// need different flooding scopes in one LSA. RFC 7684 Section 2.1: "However, since the
// Opaque LSA type defines the flooding scope, the LSA flooding scope MUST satisfy the
// application-specific requirements for all the prefixes included in a single OSPFv2
// Extended Prefix Opaque LSA."
//
// Goal: for EVERY Extended Prefix TLV in EVERY originated LSA, the scope its route type needs
// is the LSA's own scope, including when one prefix needs area scope in one role and AS scope
// in another (the input that would tempt a packer keyed by prefix to mix scopes).
// Method: originate from real self LSAs (stub link, two Type-3 summaries, two Type-5
// externals, 10.0.0.0/24 in all three roles), decode every body in full (not only its first
// TLV), and check each TLV's route type against the origination's scope and area.
func TestRFC7684EveryPrefixInAnLSAFitsItsScope(t *testing.T) {
	origs := scopeOriginations(t)
	byScope := map[OpaqueScope]int{}
	for _, o := range origs {
		lsa, err := packet.DecodeExtPrefixLSA(o.Body)
		if err != nil {
			t.Fatalf("decode originated body (id %d): %v", o.OpaqueID, err)
		}
		for _, tlv := range lsa.Prefixes {
			// RFC requirement: RFC7684-2.1-3 positive -- each originated Extended Prefix TLV
			// sits in an LSA whose scope is the one its route type needs: intra/inter-area in
			// Type 10 (area), AS-external in Type 11 (AS).
			if need := extPrefixScope(tlv.RouteType); need != o.Scope {
				t.Fatalf("route type %d prefix %v in an LSA of scope %v, it needs %v", tlv.RouteType, tlv.AddressPrefix, o.Scope, need)
			}
			byScope[o.Scope]++
		}
	}
	if byScope[OpaqueScopeArea] != 3 {
		t.Fatalf("area-scope TLVs originated = %d, want 3 (stub, two summaries)", byScope[OpaqueScopeArea])
	}
	if byScope[OpaqueScopeAS] != 2 {
		t.Fatalf("AS-scope TLVs originated = %d, want 2 (two externals)", byScope[OpaqueScopeAS])
	}
}

// TestRFC7684PrefixNeedingTwoScopesNeverSharesAnLSA proves the negative of the same sentence:
// a prefix Ze advertises both area-locally and AS-wide is never carried in one LSA whose
// scope fails one of its roles.
//
// Goal: 10.0.0.0/24, advertised intra-area, inter-area and AS-external, appears in no
// area-scope LSA as AS-external and in no AS-scope LSA as intra- or inter-area, and every LSA
// holding it holds only route types its scope satisfies.
// Method: the same originations as the positive test; every LSA that holds 10.0.0.0/24 is
// checked TLV by TLV, and all three roles must be present in separate LSAs.
func TestRFC7684PrefixNeedingTwoScopesNeverSharesAnLSA(t *testing.T) {
	roles := map[uint8]OpaqueScope{}
	for _, o := range scopeOriginations(t) {
		lsa, err := packet.DecodeExtPrefixLSA(o.Body)
		if err != nil {
			t.Fatalf("decode originated body (id %d): %v", o.OpaqueID, err)
		}
		for _, tlv := range lsa.Prefixes {
			if tlv.AddressPrefix != [4]byte{10, 0, 0, 0} {
				continue
			}
			// RFC requirement: RFC7684-2.1-3 negative -- the one prefix needing both area and AS
			// scope is never in an LSA whose scope fails its role: no AS-external TLV in a
			// Type 10 LSA, no intra/inter-area TLV in a Type 11 LSA.
			if extPrefixScope(tlv.RouteType) != o.Scope {
				t.Fatalf("10.0.0.0/24 route type %d carried in a scope %v LSA", tlv.RouteType, o.Scope)
			}
			roles[tlv.RouteType] = o.Scope
		}
	}
	for _, rt := range []uint8{packet.ExtRouteTypeIntraArea, packet.ExtRouteTypeInterArea, packet.ExtRouteTypeASExternal} {
		if _, ok := roles[rt]; !ok {
			t.Fatalf("10.0.0.0/24 was not originated with route type %d; roles seen %v", rt, roles)
		}
	}
}
