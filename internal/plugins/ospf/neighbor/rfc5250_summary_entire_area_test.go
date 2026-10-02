// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- opaque LSAs in the
// Database summary list.
// RFC: rfc/short/rfc5250.md -- Section 3.2 (Database Exchange with opaque LSAs).
//
// VALIDATES: the Database summary list a neighbor receives at ExStart, built by the
// production path (Table.databaseSummaryLocked over the real lsdb.LSDB), lists every LSA
// type RFC 5250 section 3.2 names for the area: Router, Network, both Summary types,
// type-9 and type-10 Opaque, AS External and type-11 Opaque.
// PREVENTS: a summary list that drops one of those types, or one that lists LSAs of
// another area or of another interface's link-local scope.
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc5250EncodedLSA encodes and decodes lsa so the LSDB holds its wire form.
func rfc5250EncodedLSA(t *testing.T, lsa packet.LSA) packet.LSA {
	t.Helper()
	buf := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(buf, 0)
	decoded, err := packet.DecodeLSA(buf)
	if err != nil {
		t.Fatalf("DecodeLSA %v: %v", lsa.Header.Key(), err)
	}
	return decoded
}

// rfc5250Header is an LSA header advertised by adv with the initial sequence number.
func rfc5250Header(typ types.LSType, id types.LinkStateID, adv types.RouterID) packet.LSAHeader {
	return packet.LSAHeader{Options: types.OptionE, Type: typ, LinkStateID: id, AdvertisingRouter: adv, Sequence: types.InitialSequenceNumber}
}

// rfc5250Opaque installs one opaque LSA of scope (9 on iface, 10 in area, 11 AS-wide).
func rfc5250Opaque(t *testing.T, db *lsdb.LSDB, scope types.LSType, area types.AreaID, iface string, adv types.RouterID, opaqueID uint32) types.LSAKey {
	t.Helper()
	hdr, ok := db.OriginateOpaque(lsdb.OpaqueOriginateInput{
		Router: adv, OpaqueType: 1, OpaqueID: opaqueID, Scope: scope,
		Area: area, Interface: iface, Options: types.OptionO, Body: []byte{1, 2, 3, 4},
	})
	if !ok {
		t.Fatalf("installing type-%d opaque LSA %d failed", scope, opaqueID)
	}
	return hdr.Key()
}

// rfc5250Install installs lsa into area and returns its key.
func rfc5250Install(t *testing.T, db *lsdb.LSDB, area types.AreaID, lsa packet.LSA) types.LSAKey {
	t.Helper()
	enc := rfc5250EncodedLSA(t, lsa)
	if !db.Install(area, enc) {
		t.Fatalf("installing %v failed", enc.Header.Key())
	}
	return enc.Header.Key()
}

// rfc5250SummaryFixture builds a real LSDB holding one LSA of every type section 3.2 names
// in the neighbor's area 0 (type-9 on its interface eth0), plus a Router LSA and a type-10
// in area 0.0.0.4 and a type-9 on eth9, then brings a neighbor on eth0 to ExStart. It
// returns the neighbor's summary keys, the area-0 keys and the keys of the other scopes.
func rfc5250SummaryFixture(t *testing.T) (map[types.LSAKey]bool, []types.LSAKey, []types.LSAKey) {
	t.Helper()
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	db := lsdb.New(func() time.Time { return time.Unix(1, 0) })
	db.SetSelfRouterID(cfg.RouterID)
	other := area(t, "0.0.0.4")
	db.SetAreaTypes(map[types.AreaID]string{cfg.AreaID: types.AreaTypeNormal, other: types.AreaTypeNormal})
	adv := rid(t, "2.2.2.2")
	mask := [4]byte{255, 255, 255, 0}

	router := packet.RouterLSA{Links: []packet.RouterLink{{LinkID: types.LinkStateID{10, 0, 0, 0}, LinkData: mask, Type: packet.RouterLinkTypeStub, Metric: 1}}}
	network := packet.NetworkLSA{NetworkMask: mask, AttachedRouters: []types.RouterID{adv, cfg.RouterID}}
	summary := packet.SummaryLSA{NetworkMask: mask, Metric: 10}
	asbr := packet.SummaryLSA{Metric: 10}
	external := packet.ExternalLSA{NetworkMask: mask, Metric: 20}
	own := []types.LSAKey{
		rfc5250Install(t, db, cfg.AreaID, packet.LSA{Header: rfc5250Header(types.LSTypeRouter, types.LinkStateID(adv), adv), Router: &router}),
		rfc5250Install(t, db, cfg.AreaID, packet.LSA{Header: rfc5250Header(types.LSTypeNetwork, types.LinkStateID{10, 0, 0, 2}, adv), Network: &network}),
		rfc5250Install(t, db, cfg.AreaID, packet.LSA{Header: rfc5250Header(types.LSTypeSummaryNetwork, types.LinkStateID{192, 0, 2, 0}, adv), Summary: &summary}),
		rfc5250Install(t, db, cfg.AreaID, packet.LSA{Header: rfc5250Header(types.LSTypeSummaryASBR, types.LinkStateID{5, 5, 5, 5}, adv), Summary: &asbr}),
		rfc5250Opaque(t, db, types.LSTypeOpaqueLink, cfg.AreaID, cfg.Name, adv, 9),
		rfc5250Opaque(t, db, types.LSTypeOpaqueArea, cfg.AreaID, "", adv, 10),
		rfc5250Install(t, db, cfg.AreaID, packet.LSA{Header: rfc5250Header(types.LSTypeASExternal, types.LinkStateID{203, 0, 113, 0}, adv), External: &external}),
		rfc5250Opaque(t, db, types.LSTypeOpaqueAS, types.BackboneArea, "", adv, 11),
	}
	foreignRouter := rid(t, "4.4.4.4")
	foreign := []types.LSAKey{
		rfc5250Install(t, db, other, packet.LSA{Header: rfc5250Header(types.LSTypeRouter, types.LinkStateID(foreignRouter), foreignRouter), Router: &router}),
		rfc5250Opaque(t, db, types.LSTypeOpaqueArea, other, "", foreignRouter, 40),
		rfc5250Opaque(t, db, types.LSTypeOpaqueLink, cfg.AreaID, "eth9", adv, 99),
	}
	tbl.SetLSDB(db)

	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	n, ok := tbl.lookupLocked(cfg.Name, peer)
	if !ok {
		t.Fatalf("neighbor %s missing", peer)
	}
	if n.State != stateExStart {
		t.Fatalf("neighbor state %v, want ExStart", n.State)
	}
	return summaryKeys(n.SummaryList), own, foreign
}

// RFC requirement: RFC5250-3.2-1 positive -- at ExStart the neighbor's Database summary list
// (databaseSummaryLocked over the real LSDB) lists the Router, Network, Summary (type 3),
// ASBR-Summary (type 4), type-9 Opaque (its own interface), type-10 Opaque, AS External and
// type-11 Opaque LSA installed for its area: one LSA of each, all eight listed.
func TestRFC5250SummaryListsEntireAreaDatabase(t *testing.T) {
	keys, own, _ := rfc5250SummaryFixture(t)
	for _, key := range own {
		if !keys[key] {
			t.Fatalf("Database summary list lacks %v (type %d): %v", key, key.Type, keys)
		}
	}
}

// RFC requirement: RFC5250-3.2-1 negative -- the list is the neighbor's AREA database and
// nothing else: the Router LSA and type-10 Opaque LSA of area 0.0.0.4, and the type-9
// Opaque LSA of interface eth9, are installed in the same LSDB and none is listed, while
// the area's own eight LSAs are (presence asserted first).
func TestRFC5250SummaryOmitsOtherAreaAndLinkLSAs(t *testing.T) {
	keys, own, foreign := rfc5250SummaryFixture(t)
	if !keys[own[0]] {
		t.Fatalf("Database summary list lacks the area's Router LSA %v: %v", own[0], keys)
	}
	for _, key := range foreign {
		if keys[key] {
			t.Fatalf("Database summary list carries %v (type %d) from another area or interface: %v", key, key.Type, keys)
		}
	}
	if len(keys) != len(own) {
		t.Fatalf("Database summary list holds %d LSAs, want exactly the area's %d: %v", len(keys), len(own), keys)
	}
}
