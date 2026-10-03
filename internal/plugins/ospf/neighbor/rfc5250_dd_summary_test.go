// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- type-9 opaque LSAs in
// the Database summary list.
// RFC: rfc/short/rfc5250.md -- Section 3.2 (Database Exchange with opaque LSAs).
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// opaqueLinkHeader builds a type-9 (link-local opaque) LSA header whose Link State ID
// carries the opaque type and id.
func opaqueLinkHeader(t *testing.T, opaqueID byte) packet.LSAHeader {
	t.Helper()
	h := testHeader(t, types.InitialSequenceNumber)
	h.Type = types.LSTypeOpaqueLink
	h.LinkStateID = types.LinkStateID{1, 0, 0, opaqueID}
	return h
}

// summaryKeys returns the LSA keys of a neighbor's Database summary list.
func summaryKeys(list []packet.LSAHeader) map[types.LSAKey]bool {
	out := make(map[types.LSAKey]bool, len(list))
	for _, h := range list {
		out[h.Key()] = true
	}
	return out
}

// RFC requirement: RFC5250-3.2-4 positive -- a type-9 opaque LSA associated with the
// neighbor's own interface is listed in that neighbor's Database summary list at ExStart
// (databaseSummaryLocked, table.go, appends LinkLSAs(cfg.Name)).
func TestRFC5250SummaryListsOwnInterfaceType9(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	areaLSA := testHeader(t, types.InitialSequenceNumber)
	own := opaqueLinkHeader(t, 0x11)
	db := &fakeScopedLSDB{
		area:  fakeLSDB{areaLSA.Key(): {Header: areaLSA}},
		links: map[string]fakeLSDB{cfg.Name: {own.Key(): {Header: own}}},
	}
	tbl.SetLSDB(db)

	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	n, ok := tbl.lookupLocked(cfg.Name, peer)
	if !ok {
		t.Fatalf("neighbor %s missing", peer)
	}
	keys := summaryKeys(n.SummaryList)
	if !keys[own.Key()] {
		t.Fatalf("DD summary %+v lacks the interface's type-9 opaque LSA %v", n.SummaryList, own.Key())
	}
	if !keys[areaLSA.Key()] {
		t.Fatalf("DD summary %+v lacks the area's router LSA %v", n.SummaryList, areaLSA.Key())
	}
}

// RFC requirement: RFC5250-3.2-4 negative -- a type-9 opaque LSA associated with another
// interface is omitted from this neighbor's Database summary list, even though a type-9
// LSA of the neighbor's own interface is listed (databaseSummaryLocked, table.go, reads
// only LinkLSAs(cfg.Name)).
func TestRFC5250SummaryOmitsOtherInterfaceType9(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	own := opaqueLinkHeader(t, 0x11)
	other := opaqueLinkHeader(t, 0x22)
	db := &fakeScopedLSDB{
		area: fakeLSDB{},
		links: map[string]fakeLSDB{
			cfg.Name: {own.Key(): {Header: own}},
			"eth9":   {other.Key(): {Header: other}},
		},
	}
	tbl.SetLSDB(db)

	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	n, ok := tbl.lookupLocked(cfg.Name, peer)
	if !ok {
		t.Fatalf("neighbor %s missing", peer)
	}
	keys := summaryKeys(n.SummaryList)
	if keys[other.Key()] {
		t.Fatalf("DD summary %+v lists eth9's type-9 opaque LSA %v", n.SummaryList, other.Key())
	}
	if !keys[own.Key()] {
		t.Fatalf("DD summary %+v lacks %s's own type-9 opaque LSA %v", n.SummaryList, cfg.Name, own.Key())
	}
}
