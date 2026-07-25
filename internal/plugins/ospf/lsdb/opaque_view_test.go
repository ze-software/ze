// VALIDATES: the opaque-LSA query builders (opaque_as.go). OpaqueLSAsByType returns
// every stored opaque LSA whose Opaque Type matches, across the per-area (Type 10),
// AS-wide (Type 11) and per-interface (Type 9) stores; OpaqueLSACounts groups the
// population by scope + Opaque Type; linkAreaOf returns the area recorded for a link
// store at install time.
// PREVENTS: a query that skips a scope's store (hiding a consumer's opaque LSAs) or
// miscounts the opaque gauge, and a link-area lookup that forgets the install-time area.
package lsdb

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// originateOpaqueTriple installs one opaque LSA at each scope: Type 10 (area, opaque
// type 1), Type 11 (AS, opaque type 1) and Type 9 (link on eth0, opaque type 2).
func originateOpaqueTriple(t *testing.T, db *LSDB) {
	t.Helper()
	a0 := area("0.0.0.0")
	if _, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x01, Scope: types.LSTypeOpaqueArea,
		Area: a0, Options: types.OptionO, Body: []byte{0xaa, 0xbb, 0xcc, 0xdd},
	}); !ok {
		t.Fatalf("originate area opaque failed")
	}
	if _, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x02, Scope: types.LSTypeOpaqueAS,
		Options: types.OptionO, Body: []byte{0x01, 0x02, 0x03, 0x04},
	}); !ok {
		t.Fatalf("originate AS opaque failed")
	}
	if _, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 2, OpaqueID: 0x03, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Area: a0, Options: types.OptionO, Body: []byte{0x09, 0x08, 0x07, 0x06},
	}); !ok {
		t.Fatalf("originate link opaque failed")
	}
}

func TestOpaqueLSAsByTypeSpansScopes(t *testing.T) {
	db, _, _ := opaqueOriginateDB(t)
	originateOpaqueTriple(t, db)

	// Opaque Type 1 was originated at both area (Type 10) and AS (Type 11) scope.
	got := db.OpaqueLSAsByType(1)
	if len(got) != 2 {
		t.Fatalf("OpaqueLSAsByType(1) len = %d, want 2: %+v", len(got), got)
	}
	scopes := map[types.LSType]OpaqueLSAView{}
	for _, v := range got {
		if v.OpaqueType != 1 {
			t.Fatalf("OpaqueLSAsByType(1) returned an Opaque Type %d view: %+v", v.OpaqueType, v)
		}
		scopes[v.Scope] = v
	}
	if _, ok := scopes[types.LSTypeOpaqueArea]; !ok {
		t.Fatalf("area-scope (Type 10) opaque missing: %+v", got)
	}
	asView, ok := scopes[types.LSTypeOpaqueAS]
	if !ok {
		t.Fatalf("AS-scope (Type 11) opaque missing: %+v", got)
	}
	if asView.OpaqueID != 0x02 || len(asView.Body) != 4 {
		t.Fatalf("AS opaque view wrong: %+v", asView)
	}

	// Opaque Type 2 was originated only at link (Type 9) scope on eth0.
	link := db.OpaqueLSAsByType(2)
	if len(link) != 1 {
		t.Fatalf("OpaqueLSAsByType(2) len = %d, want 1: %+v", len(link), link)
	}
	if link[0].Scope != types.LSTypeOpaqueLink || link[0].Interface != "eth0" {
		t.Fatalf("link opaque view wrong scope/interface: %+v", link[0])
	}

	// A never-originated Opaque Type yields nothing.
	if got := db.OpaqueLSAsByType(9); len(got) != 0 {
		t.Fatalf("OpaqueLSAsByType(9) = %+v, want empty", got)
	}
}

func TestOpaqueLSACountsGroupsByScopeAndType(t *testing.T) {
	db, _, _ := opaqueOriginateDB(t)
	originateOpaqueTriple(t, db)

	type bucket struct {
		scope types.LSType
		typ   uint8
	}
	counts := map[bucket]int{}
	for _, c := range db.OpaqueLSACounts() {
		counts[bucket{scope: c.Scope, typ: c.OpaqueType}] = c.Count
	}
	if len(counts) != 3 {
		t.Fatalf("OpaqueLSACounts buckets = %d, want 3: %+v", len(counts), counts)
	}
	for _, b := range []bucket{
		{types.LSTypeOpaqueArea, 1},
		{types.LSTypeOpaqueAS, 1},
		{types.LSTypeOpaqueLink, 2},
	} {
		if counts[b] != 1 {
			t.Fatalf("bucket %+v count = %d, want 1: %+v", b, counts[b], counts)
		}
	}
}

func TestLinkAreaOfRecordsInstallArea(t *testing.T) {
	db, _, _ := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	if _, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 2, OpaqueID: 0x03, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Area: a0, Options: types.OptionO, Body: []byte{1, 2, 3, 4},
	}); !ok {
		t.Fatalf("originate link opaque failed")
	}
	if got := db.linkAreaOf("eth0"); got != a0 {
		t.Fatalf("linkAreaOf(eth0) = %v, want %v", got, a0)
	}
	// An interface with no link store returns the zero area.
	if got := db.linkAreaOf("eth9"); got != (types.AreaID{}) {
		t.Fatalf("linkAreaOf(unknown) = %v, want zero", got)
	}
}
