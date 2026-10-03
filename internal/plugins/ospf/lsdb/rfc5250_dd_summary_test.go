// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- opaque LSAs in the
// Database summary list.
// RFC: rfc/short/rfc5250.md -- Section 3.2 (Database Exchange with opaque LSAs).
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// summaryHasType reports whether the Database summary list carries at least one header of
// the given LS type.
func summaryHasType(db *LSDB, area types.AreaID, typ types.LSType) bool {
	for _, h := range db.Summary(area) {
		if h.Type == typ {
			return true
		}
	}
	return false
}

// RFC requirement: RFC5250-3.2-1 positive -- the Database summary list of a normal area
// carries the area's type-10 opaque LSA together with the global type-5 AS-External and
// type-11 opaque LSAs (Summary, lsdb.go: area store, then asExternal, then asOpaque).
// RFC requirement: RFC5250-3.2-3 negative -- the omission is scoped to stub and NSSA areas:
// a normal area's summary still lists the AS-External and type-11 opaque LSAs.
func TestRFC5250SummaryListsOpaqueAndGlobalLSAs(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	normal := area("0.0.0.3")
	db.SetAreaTypes(map[types.AreaID]string{normal: types.AreaTypeNormal})

	lsa10 := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x10, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	if !db.Install(normal, lsa10) {
		t.Fatalf("type-10 opaque install rejected")
	}
	if !db.Install(types.BackboneArea, externalLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber)) {
		t.Fatalf("type-5 install rejected")
	}
	lsa11 := opaqueLSA(t, types.LSTypeOpaqueAS, 4, 0x20, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	if !db.Install(types.BackboneArea, lsa11) {
		t.Fatalf("type-11 opaque install rejected")
	}

	for _, typ := range []types.LSType{types.LSTypeOpaqueArea, types.LSTypeASExternal, types.LSTypeOpaqueAS} {
		if !summaryHasType(db, normal, typ) {
			t.Fatalf("normal area summary lacks LS type %v: %+v", typ, db.Summary(normal))
		}
	}
}

// RFC requirement: RFC5250-3.2-3 positive -- for an area configured as a stub area or as
// an NSSA, the Database summary list omits the AS-External and type-11 opaque LSAs that
// the AS-wide stores hold, while the area's own type-10 opaque LSA stays listed
// (Summary, lsdb.go: includeExternal and includeOpaqueAS from shouldDropByArea).
// RFC requirement: RFC5250-3.2-1 negative -- the "entire area link-state database" is the
// area structure plus the global structure the area is allowed to see: a stub or NSSA
// summary never lists a type-5 or type-11 LSA even though both are installed.
func TestRFC5250SummaryOmitsGlobalLSAsInStubAndNSSA(t *testing.T) {
	for _, tc := range []struct {
		name     string
		areaType string
	}{
		{name: "stub", areaType: types.AreaTypeStub},
		{name: "nssa", areaType: types.AreaTypeNSSA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := &fakeClock{now: time.Unix(0, 0)}
			db := newTestDB(clock)
			restricted := area("0.0.0.7")
			db.SetAreaTypes(map[types.AreaID]string{restricted: tc.areaType})

			lsa10 := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x10, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
			if !db.Install(restricted, lsa10) {
				t.Fatalf("type-10 opaque install rejected")
			}
			if !db.Install(types.BackboneArea, externalLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber)) {
				t.Fatalf("type-5 install rejected")
			}
			lsa11 := opaqueLSA(t, types.LSTypeOpaqueAS, 4, 0x20, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
			if !db.Install(types.BackboneArea, lsa11) {
				t.Fatalf("type-11 opaque install rejected")
			}

			if !summaryHasType(db, restricted, types.LSTypeOpaqueArea) {
				t.Fatalf("%s summary lacks its own type-10 opaque LSA", tc.name)
			}
			if summaryHasType(db, restricted, types.LSTypeASExternal) {
				t.Fatalf("%s summary lists a type-5 AS-External LSA: %+v", tc.name, db.Summary(restricted))
			}
			if summaryHasType(db, restricted, types.LSTypeOpaqueAS) {
				t.Fatalf("%s summary lists a type-11 opaque LSA: %+v", tc.name, db.Summary(restricted))
			}
		})
	}
}
