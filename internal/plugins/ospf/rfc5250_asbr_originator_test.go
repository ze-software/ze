// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- RFC 5250 Type-11 originator advertises itself an ASBR.
// Related: lsdb/origination.go -- selfIsASBRLocked, the originator's Router-LSA E-bit.
// Related: opaque.go -- deliverOpaque and spfASBRReachable, the receiver's routing table lookup.
// Related: rfc5250_asbr_lookup_test.go -- the receiver half alone, with a hand-built Router-LSA.
//
// VALIDATES: RFC 5250 Section 5 (1) and (2) end to end between two Ze LSDBs: router A
// (10.0.0.2) originates a Type-11 opaque LSA through its own origination path, and router B
// (10.0.0.1) installs the Router-LSA A originated, runs SPF, and judges A's Type-11 LSA.
// PREVENTS: the regression where Ze's receivers require an ASBR routing table entry for a
// Type-11 originator while Ze's own originators never set the E-bit, so two Ze routers ignore
// each other's Type-11 LSAs.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc5250OriginatorLSDB is router A: a standalone LSDB for 10.0.0.2 with one Full
// point-to-point adjacency to 10.0.0.1 over 10.0.0.0/30.
func rfc5250OriginatorLSDB(clock *time.Time) *ospflsdb.LSDB {
	a := types.RouterID{10, 0, 0, 2}
	db := ospflsdb.New(func() time.Time { return *clock })
	db.SetSelfRouterID(a)
	db.SetTimers(ospflsdb.TimerConfig{MinLSArrival: time.Second, MinLSInterval: 5 * time.Second})
	db.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name: "eth0", AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal,
			NetworkType: types.NetworkPointToPoint, State: "point-to-point",
			Address: [4]byte{10, 0, 0, 2}, NetworkMask: [4]byte{255, 255, 255, 252}, Cost: 10,
			RouterID: a, Options: types.OptionE, RetransmitInterval: 5, TransmitDelay: 1,
			Neighbors: []ospflsdb.NeighborInfo{{RouterID: types.RouterID{10, 0, 0, 1},
				Address: netip.MustParseAddr("10.0.0.1"), State: ospflsdb.NeighborStateFull}},
		}}
	})
	return db
}

// rfc5250ReceiverEngine is router B: an engine for 10.0.0.1 whose SPF computer holds only its
// own Router-LSA (a point-to-point link to 10.0.0.2) and installs into a private Loc-RIB.
func rfc5250ReceiverEngine(t *testing.T) *engine {
	t.Helper()
	e := grPrepareEngine(t, time.Unix(1_000_000, 0))
	self := types.RouterID{10, 0, 0, 1}
	e.spf = ospfspf.NewComputer(ospfspf.Config{
		Source:    e.lsdb,
		Resolver:  rfc3623NextHop{},
		Installer: ospfspf.NewInstallerFamily(locrib.NewRIB(), family.IPv4Unicast),
	})
	e.spf.SetTimers(time.Hour, time.Hour, time.Hour)
	e.spf.SetRoot(self)
	e.spf.SetAreas([]types.AreaID{types.BackboneArea})
	toA := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{10, 0, 0, 2},
		LinkData: [4]byte{10, 0, 0, 1}, Metric: 10}
	if !e.lsdb.Install(types.BackboneArea, bgplsReachabilityRouter(self, toA)) {
		t.Fatal("install router B's own Router-LSA")
	}
	return e
}

// TestRFC5250TwoRoutersType11OriginatorReachable carries router A's Router-LSA to router B
// twice: before A originates any Type-11 LSA, then after it originates one. Each time B runs
// SPF and is handed A's Type-11 LSA as its opaque delivery.
func TestRFC5250TwoRoutersType11OriginatorReachable(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	now := time.Unix(1_000_000, 0)
	a := rfc5250OriginatorLSDB(&now)
	b := rfc5250ReceiverEngine(t)
	var got []opaqueReceived
	if err := registerOpaqueConsumer(4, OpaqueScopeAS, nil, func(r opaqueReceived) { got = append(got, r) }); err != nil {
		t.Fatalf("register: %v", err)
	}
	routerA := types.RouterID{10, 0, 0, 2}
	carryRouterLSA := func() {
		t.Helper()
		a.OriginateFromTopology(routerA, false)
		key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(routerA), AdvertisingRouter: routerA}
		lsa, ok := a.LookupLSA(types.BackboneArea, key)
		if !ok {
			t.Fatal("router A originated no Router-LSA")
		}
		if !b.lsdb.Install(types.BackboneArea, lsa) {
			t.Fatal("router B refused router A's Router-LSA")
		}
		b.spf.Run()
	}
	deliver := func(h packet.LSAHeader, body []byte) bool {
		t.Helper()
		before := len(got)
		b.deliverOpaque(ospflsdb.OpaqueDelivery{Scope: h.Type, Area: types.BackboneArea,
			AdvertisingRouter: h.AdvertisingRouter, OpaqueType: 4, OpaqueID: 1, Body: body})
		if len(got) != before+1 {
			t.Fatal("router A's Type-11 LSA was not delivered on router B")
		}
		return got[before].Reachable
	}

	// Router A's Router-LSA before it originates any AS-scope opaque LSA: no E-bit, so router B
	// computes no ASBR entry for A. This is the receiver refusal the positive below must clear.
	carryRouterLSA()
	body := []byte{0, 1, 0, 4, 0, 0, 0, 0}
	if deliver(packet.LSAHeader{Type: types.LSTypeOpaqueAS, AdvertisingRouter: routerA}, body) {
		t.Fatal("router B used a Type-11 LSA from router A before A advertised itself an ASBR")
	}

	// RFC requirement: RFC5250-5-3 positive -- router A, once it originates a Type-11 opaque LSA,
	// advertises itself an ASBR in the Router-LSA it floods, so router B's SPF computes an ASBR
	// routing table entry for A and B uses A's Type-11 LSA.
	now = now.Add(6 * time.Second)
	h, ok := a.OriginateOpaque(ospflsdb.OpaqueOriginateInput{Router: routerA, OpaqueType: 4, OpaqueID: 1,
		Scope: types.LSTypeOpaqueAS, Area: types.BackboneArea, Options: types.OptionO, Body: body})
	if !ok {
		t.Fatal("router A refused to originate its Type-11 opaque LSA")
	}
	carryRouterLSA()
	if !deliver(h, body) {
		t.Fatal("router B ignored router A's Type-11 LSA: A's Router-LSA gave B no ASBR routing table entry")
	}
}
