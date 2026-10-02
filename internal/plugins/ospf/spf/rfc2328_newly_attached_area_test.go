// Design: docs/architecture/ospf/ospf-9-inter-area-abr.md -- Type 3 Summary-LSA origination.
// Related: computer.go -- Computer.Run, which feeds OriginateSummaries the active areas.
//
// VALIDATES: RFC 2328 section 12.4 event (7): when the router becomes newly attached to an
// area, the next SPF run originates summary-LSAs into that area for the pertinent intra-area
// routes of its other areas and for its inter-area routes, and for no route of the area
// itself.
// PREVENTS: an area that becomes attached with no summary-LSAs until some route changes, and
// inter-area routes left out of a newly attached area.
package spf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc2328AttachResolver resolves every next-hop address to eth0.
type rfc2328AttachResolver struct{}

func (rfc2328AttachResolver) ResolveInterface(netip.Addr) (string, bool) { return "eth0", true }

// rfc2328AttachRouterLSA builds a Router-LSA for router with flags and links.
func rfc2328AttachRouterLSA(router types.RouterID, flags uint8, links ...packet.RouterLink) packet.LSA {
	return packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router),
		AdvertisingRouter: router, Sequence: types.InitialSequenceNumber},
		Router: &packet.RouterLSA{Flags: flags, Links: links}}
}

// rfc2328AttachStub builds a 255.255.255.0 stub link for the /24 at a.
func rfc2328AttachStub(a [4]byte, metric types.Metric) packet.RouterLink {
	return packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID(a),
		LinkData: [4]byte{255, 255, 255, 0}, Metric: metric}
}

// rfc2328NewlyAttached builds router 1.1.1.1 in the backbone, point-to-point to the area
// border router 2.2.2.2, with the backbone stub 10.10.0.0/24 (cost 7); 2.2.2.2 originates a
// summary-LSA for 10.30.0.0/24 (metric 20) into the backbone, so 10.30.0.0/24 is an
// inter-area route of cost 30. Area 0.0.0.1 is configured, and SPF runs once with no
// Router-LSA of 1.1.1.1 in it. Then 1.1.1.1 becomes attached to area 0.0.0.1, whose
// Router-LSA carries the stub 10.20.0.0/24 (cost 3), and SPF runs again.
func rfc2328NewlyAttached(t *testing.T) (db *ospflsdb.LSDB, root types.RouterID, area1 types.AreaID) {
	t.Helper()
	root = types.RouterID{1, 1, 1, 1}
	border := types.RouterID{2, 2, 2, 2}
	area1 = types.AreaID{0, 0, 0, 1}
	db = ospflsdb.New(nil)
	toBorder := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(border),
		LinkData: [4]byte{10, 0, 0, 1}, Metric: 10}
	toRoot := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID(root),
		LinkData: [4]byte{10, 0, 0, 2}, Metric: 10}
	summary := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeSummaryNetwork,
		LinkStateID: types.LinkStateID{10, 30, 0, 0}, AdvertisingRouter: border, Sequence: types.InitialSequenceNumber},
		Summary: &packet.SummaryLSA{NetworkMask: [4]byte{255, 255, 255, 0}, Metric: 20}}
	for _, lsa := range []packet.LSA{
		rfc2328AttachRouterLSA(root, 0, toBorder, rfc2328AttachStub([4]byte{10, 10, 0, 0}, 7)),
		rfc2328AttachRouterLSA(border, packet.RouterFlagB, toRoot),
		summary,
	} {
		if !db.Install(types.BackboneArea, lsa) {
			t.Fatalf("installing %v from %s in the backbone failed", lsa.Header.Type, lsa.Header.AdvertisingRouter)
		}
	}
	c := NewComputer(Config{Source: db, Resolver: rfc2328AttachResolver{}, Root: root,
		Areas: []types.AreaID{types.BackboneArea, area1}, SummarySink: db})
	c.SetTimers(time.Hour, time.Hour, time.Hour)
	c.Run()
	if n := len(db.Summary(area1)); n != 0 {
		t.Fatalf("area 0.0.0.1 holds %d LSAs before 1.1.1.1 is attached to it, want 0", n)
	}
	if !db.Install(area1, rfc2328AttachRouterLSA(root, 0, rfc2328AttachStub([4]byte{10, 20, 0, 0}, 3))) {
		t.Fatal("installing the Router-LSA of 1.1.1.1 in area 0.0.0.1 failed")
	}
	c.Run()
	return db, root, area1
}

// RFC requirement: RFC2328-12.4.3-1 positive -- once 1.1.1.1 becomes newly attached to area
// 0.0.0.1, the next SPF run originates into that area a summary-LSA for the backbone's
// intra-area route 10.10.0.0/24 (mask 255.255.255.0, metric 7) and for the inter-area route
// 10.30.0.0/24 (mask 255.255.255.0, metric 30); before the attachment the area held none.
func TestRFC2328NewlyAttachedAreaGetsIntraAndInterAreaSummaries(t *testing.T) {
	// Goal: attachment alone triggers the origination, for both route classes. Method: a real
	// LSDB and SPF computer, two runs around the attachment, the area's summary-LSAs read.
	db, root, area1 := rfc2328NewlyAttached(t)
	want := map[string]uint32{"10.10.0.0": 7, "10.30.0.0": 30}
	for lsid, metric := range want {
		lsa, ok := db.LookupLSA(area1, type3Key(root, testLSID(t, lsid)))
		if !ok {
			t.Fatalf("no summary-LSA for %s/24 in the newly attached area 0.0.0.1", lsid)
		}
		body, err := lsa.DecodeSummary()
		if err != nil {
			t.Fatalf("DecodeSummary %s: %v", lsid, err)
		}
		if body.NetworkMask != ([4]byte{255, 255, 255, 0}) || body.Metric != metric {
			t.Fatalf("summary-LSA for %s = %+v, want mask 255.255.255.0 metric %d", lsid, body, metric)
		}
	}
}

// RFC requirement: RFC2328-12.4.3-1 negative -- only pertinent routes are summarized: the
// newly attached area's own intra-area route 10.20.0.0/24 gets no summary-LSA in area 0.0.0.1
// (it is summarized into the backbone, metric 3), and the inter-area route 10.30.0.0/24 gets
// no summary-LSA from 1.1.1.1 in the backbone.
func TestRFC2328NewlyAttachedAreaGetsNoSummaryOfItsOwnRoutes(t *testing.T) {
	// Goal: the newly attached area is not handed back its own routes. Method: the positive's
	// two runs, the summary-LSAs of 1.1.1.1 read in both areas.
	db, root, area1 := rfc2328NewlyAttached(t)
	if lsa, ok := db.LookupLSA(area1, type3Key(root, testLSID(t, "10.20.0.0"))); ok {
		t.Fatalf("area 0.0.0.1's own route 10.20.0.0/24 was summarized into it: %+v", lsa.Header)
	}
	back, ok := db.LookupLSA(types.BackboneArea, type3Key(root, testLSID(t, "10.20.0.0")))
	if !ok {
		t.Fatal("10.20.0.0/24 was not summarized into the backbone, so the route is missing")
	}
	if body, err := back.DecodeSummary(); err != nil || body.Metric != 3 {
		t.Fatalf("backbone summary-LSA for 10.20.0.0/24 = %+v (%v), want metric 3", body, err)
	}
	if lsa, ok := db.LookupLSA(types.BackboneArea, type3Key(root, testLSID(t, "10.30.0.0"))); ok {
		t.Fatalf("the inter-area route 10.30.0.0/24 was summarized into the backbone: %+v", lsa.Header)
	}
}
