// VALIDATES: spec-ospf-11 RFC 2328 sec 3.6 / RFC 3101 -- a stub or NSSA area rejects
// Type 5 AS-External and Type 4 ASBR-Summary LSAs on both flood-in and flood-out; a
// Type 7 NSSA-LSA is accepted/flooded ONLY inside an NSSA; intra-area types (Router,
// Network, Type 3) cross every area type.
// PREVENTS: regressions where a stub/NSSA area leaks Type 4/5, or a Type 7 escapes
// its NSSA into the backbone or a normal/stub area.
package lsdb

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-2 positive -- the send-side filter refuses every stub/NSSA interface for a Type-5 AS-external-LSA while allowing a normal-area interface, so an AS-external-LSA is never flooded into a stub area (eligibleInterface, flooding.go:401-417).
func TestOSPFStubFloodFilter(t *testing.T) {
	area := types.AreaID{0, 0, 0, 1}
	cases := []struct {
		name     string
		areaType string
		typ      types.LSType
		drop     bool // expected shouldDropByArea
	}{
		{"normal accepts Type5", AreaTypeNormal, types.LSTypeASExternal, false},
		{"stub drops Type5", AreaTypeStub, types.LSTypeASExternal, true},
		{"nssa drops Type5", AreaTypeNSSA, types.LSTypeASExternal, true},
		{"normal accepts Type4", AreaTypeNormal, types.LSTypeSummaryASBR, false},
		{"stub drops Type4", AreaTypeStub, types.LSTypeSummaryASBR, true},
		{"nssa drops Type4", AreaTypeNSSA, types.LSTypeSummaryASBR, true},
		{"normal drops Type7", AreaTypeNormal, types.LSTypeNSSA, true},
		{"stub drops Type7", AreaTypeStub, types.LSTypeNSSA, true},
		{"nssa accepts Type7", AreaTypeNSSA, types.LSTypeNSSA, false},
		{"stub accepts Type3", AreaTypeStub, types.LSTypeSummaryNetwork, false},
		{"stub accepts Router", AreaTypeStub, types.LSTypeRouter, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldDropByArea(tc.areaType, tc.typ); got != tc.drop {
				t.Fatalf("shouldDropByArea(%s, %v) = %v, want %v", tc.areaType, tc.typ, got, tc.drop)
			}
			// eligibleInterface is the send-side mirror: a same-area interface should be
			// eligible exactly when the receive side would NOT drop. Type 5 is AS-wide so
			// area match is not required; the others are area-scoped.
			iface := InterfaceInfo{AreaID: area, AreaType: tc.areaType}
			wantEligible := !tc.drop
			if got := eligibleInterface(iface, area, tc.typ); got != wantEligible {
				t.Fatalf("eligibleInterface(%s, %v) = %v, want %v", tc.areaType, tc.typ, got, wantEligible)
			}
		})
	}

	// A Type 7 must not flood out of its NSSA into a different area on the same router.
	nssaIface := InterfaceInfo{AreaID: types.AreaID{0, 0, 0, 2}, AreaType: AreaTypeNSSA}
	if eligibleInterface(nssaIface, area, types.LSTypeNSSA) {
		t.Fatal("Type 7 flooded out an interface in a different area")
	}
}
