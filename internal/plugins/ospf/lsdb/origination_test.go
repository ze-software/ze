package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func originTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name:               "eth0",
			AreaID:             area("0.0.0.0"),
			AreaType:           AreaTypeNormal,
			NetworkType:        NetworkBroadcast,
			State:              InterfaceStateDR,
			Address:            ip4("10.0.0.1"),
			NetworkMask:        ip4("255.255.255.0"),
			Cost:               10,
			RouterID:           rid("1.1.1.1"),
			Options:            types.OptionE,
			DR:                 rid("1.1.1.1"),
			BDR:                rid("2.2.2.2"),
			RetransmitInterval: 5,
			TransmitDelay:      1,
			Neighbors:          []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
		},
		{
			Name:               "ptp0",
			AreaID:             area("0.0.0.0"),
			AreaType:           AreaTypeNormal,
			NetworkType:        NetworkPointToPoint,
			State:              "point-to-point",
			Address:            ip4("192.0.2.1"),
			NetworkMask:        ip4("255.255.255.252"),
			Cost:               20,
			RouterID:           rid("1.1.1.1"),
			Options:            types.OptionE,
			RetransmitInterval: 5,
			TransmitDelay:      1,
			Neighbors:          []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("192.0.2.2"), State: NeighborStateFull}},
		},
	}
}

// virtualLinkTopology is an ABR (areas 0.0.0.0 + 0.0.0.1) with a Full virtual link through
// the transit area presented as a synthetic NetworkVirtual interface in the BACKBONE. Its
// local (transit) address is 172.16.0.1 and the virtual neighbor is 9.9.9.9 at cost 15.
func virtualLinkTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkBroadcast, State: InterfaceStateDR,
			Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"),
			Cost: 10, RouterID: rid("1.1.1.1"), Options: types.OptionE,
			DR: rid("1.1.1.1"), BDR: rid("2.2.2.2"), RetransmitInterval: 5, TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
		},
		{
			Name: "eth2", AreaID: area("0.0.0.1"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: "point-to-point",
			Address: ip4("198.51.100.1"), NetworkMask: ip4("255.255.255.252"),
			Cost: 5, RouterID: rid("1.1.1.1"), Options: types.OptionE, RetransmitInterval: 5, TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("4.4.4.4"), Address: naddr4("198.51.100.2"), State: NeighborStateFull}},
		},
		{
			Name: "*vlink-0.0.0.1-9.9.9.9", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkVirtual, State: "point-to-point", VirtualTransitArea: area("0.0.0.1"),
			Address: ip4("172.16.0.1"), Cost: 15, RouterID: rid("1.1.1.1"), Options: types.OptionE,
			RetransmitInterval: 5, TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("9.9.9.9"), Address: naddr4("172.16.0.2"), State: NeighborStateFull}},
		},
	}
}

func virtualLink(t *testing.T, links []packet.RouterLink) (packet.RouterLink, bool) {
	t.Helper()
	for _, l := range links {
		if l.Type == packet.RouterLinkTypeVirtual {
			return l, true
		}
	}
	return packet.RouterLink{}, false
}

// VALIDATES: spec-ospf-ext-7 AC-8 -- routerLinks emits a Type-4 virtual-link record for a
// Full virtual link with Link ID = neighbor Router ID, Link Data = the local transit
// address, and Metric = the transit cost (RFC 2328 App A.4.2).
func TestRouterLinksEmitsVirtualType4(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	vif := virtualLinkTopology()[2]
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, ABR: true, VirtualLinkEndpoint: true, Interfaces: []InterfaceInfo{vif}}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("OriginateRouter false")
	}
	lsa, _ := db.LookupLSA(in.AreaID, h.Key())
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	link, ok := virtualLink(t, body.Links)
	if !ok {
		t.Fatalf("no Type-4 virtual link in %+v", body.Links)
	}
	if link.LinkID != types.LinkStateID(rid("9.9.9.9")) {
		t.Fatalf("virtual LinkID = %s, want 9.9.9.9", link.LinkID)
	}
	if link.LinkData != ip4("172.16.0.1") {
		t.Fatalf("virtual LinkData = %v, want 172.16.0.1", link.LinkData)
	}
	if link.Metric != types.Metric(15) {
		t.Fatalf("virtual Metric = %d, want 15", link.Metric)
	}
	// No stub link (the virtual interface carries no NetworkMask).
	for _, l := range body.Links {
		if l.Type == packet.RouterLinkTypeStub {
			t.Fatalf("virtual link produced a stub link: %+v", l)
		}
	}
}

func decodeAreaRouter(t *testing.T, db *LSDB, a types.AreaID) packet.RouterLSA {
	t.Helper()
	lsa, ok := db.LookupLSA(a, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("1.1.1.1")), AdvertisingRouter: rid("1.1.1.1")})
	if !ok {
		t.Fatalf("router LSA missing in area %s", a)
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter area %s: %v", a, err)
	}
	return body
}

// VALIDATES: spec-ospf-ext-7 AC-8 / A-4 -- RFC 2328 App A.4.2 / section 16.3 splits the two
// signals of a Full virtual link: the Type-4 record lives in the BACKBONE Router-LSA (the
// virtual link belongs to Area 0), while the V-bit is set in the TRANSIT area's Router-LSA
// (marking it a transit area, which drives the far ABR's TransitCapability).
func TestBackboneRouterLSAHasVBitWhenVLFull(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(virtualLinkTopology)
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count == 0 {
		t.Fatalf("originated count = 0")
	}
	backbone := decodeAreaRouter(t, db, area("0.0.0.0"))
	if backbone.Flags&packet.RouterFlagV != 0 {
		t.Fatalf("V-bit must NOT be set on the backbone Router-LSA: flags = %#x", backbone.Flags)
	}
	link, ok := virtualLink(t, backbone.Links)
	if !ok {
		t.Fatalf("backbone Router-LSA missing Type-4 link: %+v", backbone.Links)
	}
	if link.LinkID != types.LinkStateID(rid("9.9.9.9")) {
		t.Fatalf("backbone virtual LinkID = %s, want 9.9.9.9", link.LinkID)
	}
	transit := decodeAreaRouter(t, db, area("0.0.0.1"))
	if transit.Flags&packet.RouterFlagV == 0 {
		t.Fatalf("V-bit not set on the transit-area Router-LSA: flags = %#x", transit.Flags)
	}
}

// VALIDATES: spec-ospf-ext-7 R-5 -- the Type-4 virtual link RECORD appears only in the
// backbone Router-LSA, never in a non-backbone (transit) Router-LSA (RFC 2328 App A.4.2).
func TestVirtualLinkBackboneOnly(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(virtualLinkTopology)
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count == 0 {
		t.Fatalf("originated count = 0")
	}
	transit := decodeAreaRouter(t, db, area("0.0.0.1"))
	if _, ok := virtualLink(t, transit.Links); ok {
		t.Fatalf("Type-4 record leaked into non-backbone Router-LSA: %+v", transit.Links)
	}
	if transit.Flags&packet.RouterFlagV == 0 {
		t.Fatalf("transit-area Router-LSA should carry the V-bit: flags = %#x", transit.Flags)
	}
	backbone := decodeAreaRouter(t, db, area("0.0.0.0"))
	if _, ok := virtualLink(t, backbone.Links); !ok {
		t.Fatalf("backbone Router-LSA missing the Type-4 record: %+v", backbone.Links)
	}
}

func TestOSPFOriginateRouterLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, ABR: true, ASBR: true, Interfaces: originTopology()}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("OriginateRouter returned false")
	}
	if h.Sequence != types.InitialSequenceNumber || h.Checksum == 0 {
		t.Fatalf("bad header: %+v", h)
	}
	lsa, ok := db.LookupLSA(in.AreaID, h.Key())
	if !ok {
		t.Fatalf("originated LSA not installed")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags&(packet.RouterFlagB|packet.RouterFlagE) != packet.RouterFlagB|packet.RouterFlagE {
		t.Fatalf("flags = %#x", body.Flags)
	}
	if len(body.Links) != 4 {
		t.Fatalf("links = %+v", body.Links)
	}
	if body.Flags&packet.RouterFlagNt != 0 {
		t.Fatalf("Nt-bit must be clear when NSSATranslator is false: flags = %#x", body.Flags)
	}
	if !lsa.VerifyChecksum() {
		t.Fatalf("originated Router-LSA checksum invalid")
	}
}

func TestOSPFOriginateRouterLSANtBit(t *testing.T) {
	// RFC 3101 §3.5: an NSSA translator-candidate ABR sets the Router-LSA Nt-bit.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.5"), RouterID: rid("1.1.1.1"), Options: types.OptionNP, ABR: true, NSSATranslator: true, Interfaces: originTopology()}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("OriginateRouter returned false")
	}
	lsa, ok := db.LookupLSA(in.AreaID, h.Key())
	if !ok {
		t.Fatalf("originated LSA not installed")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags&packet.RouterFlagNt == 0 {
		t.Fatalf("Nt-bit must be set for a translator-candidate ABR: flags = %#x", body.Flags)
	}
	if body.Flags&packet.RouterFlagB == 0 {
		t.Fatalf("B-bit must remain set: flags = %#x", body.Flags)
	}
}

func TestOSPFOriginateNetworkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	iface := originTopology()[0]
	h, ok := db.OriginateNetwork(area("0.0.0.0"), rid("1.1.1.1"), types.OptionE, iface)
	if !ok {
		t.Fatalf("OriginateNetwork returned false")
	}
	if h.LinkStateID != types.LinkStateID(ip4("10.0.0.1")) {
		t.Fatalf("LSID = %s", h.LinkStateID)
	}
	lsa, _ := db.LookupLSA(area("0.0.0.0"), h.Key())
	body, err := lsa.DecodeNetwork()
	if err != nil {
		t.Fatalf("DecodeNetwork: %v", err)
	}
	if body.NetworkMask != ip4("255.255.255.0") || len(body.AttachedRouters) != 2 {
		t.Fatalf("network body = %+v", body)
	}
}

func TestOSPFOriginateOnAdjacencyFull(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(originTopology)
	count := db.OriginateFromTopology(rid("1.1.1.1"), false)
	if count != 2 {
		t.Fatalf("originated count = %d", count)
	}
	if len(tx.sends) == 0 {
		t.Fatalf("no flood sent")
	}
	if _, ok := db.Lookup(area("0.0.0.0"), types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("1.1.1.1")), AdvertisingRouter: rid("1.1.1.1")}); !ok {
		t.Fatalf("router LSA not installed")
	}
	if len(db.retransmit) == 0 {
		t.Fatalf("full neighbors did not receive retransmit entries")
	}
}

func TestOSPFOriginateReorigOnChange(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()}
	h1, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("first originate false")
	}
	if h2, ok := db.OriginateRouter(in); ok || h2.Sequence != h1.Sequence {
		t.Fatalf("MinLSInterval not enforced: h2=%+v ok=%v", h2, ok)
	}
	clock.Add(5 * time.Second)
	in.Interfaces[0].Cost = 30
	h3, ok := db.OriginateRouter(in)
	if !ok || h3.Sequence != h1.Sequence.Next() || h3.Checksum == h1.Checksum {
		t.Fatalf("reorig wrong: h1=%+v h3=%+v ok=%v", h1, h3, ok)
	}
}

func TestOSPFOriginateFromTopologySetsABRFlag(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(func() []InterfaceInfo {
		ifs := originTopology()
		ifs = append(ifs, InterfaceInfo{
			Name:        "eth2",
			AreaID:      area("0.0.0.1"),
			AreaType:    AreaTypeNormal,
			NetworkType: NetworkPointToPoint,
			State:       "point-to-point",
			Address:     ip4("198.51.100.1"),
			NetworkMask: ip4("255.255.255.252"),
			Cost:        5,
			RouterID:    rid("1.1.1.1"),
			Options:     types.OptionE,
		})
		return ifs
	})
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count == 0 {
		t.Fatalf("originated count = 0")
	}
	lsa, ok := db.LookupLSA(area("0.0.0.0"), types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("1.1.1.1")), AdvertisingRouter: rid("1.1.1.1")})
	if !ok {
		t.Fatalf("router LSA missing")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags&packet.RouterFlagB == 0 {
		t.Fatalf("ABR flag missing: %#x", body.Flags)
	}
}

func TestOSPFOriginateFlushesLostDRNetworkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	current := originTopology()
	db.SetTopology(func() []InterfaceInfo { return current })
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count != 2 {
		t.Fatalf("originated count = %d", count)
	}
	key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(ip4("10.0.0.1")), AdvertisingRouter: rid("1.1.1.1")}
	clock.Add(5 * time.Second)
	current[0].DR = rid("2.2.2.2")
	current[0].State = "dr-other"
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count != 2 {
		t.Fatalf("reoriginate+flush count = %d", count)
	}
	h, ok := db.Lookup(area("0.0.0.0"), key)
	if !ok {
		t.Fatalf("network LSA missing")
	}
	if !h.Age.IsMaxAge() {
		t.Fatalf("network LSA not flushed: %+v", h)
	}
}

func TestOSPFOriginateDownActiveInterfaceKeepsEmptyArea(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	current := originTopology()[:1]
	db.SetTopology(func() []InterfaceInfo { return current })
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count != 2 {
		t.Fatalf("originated count = %d", count)
	}

	routerKey := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("1.1.1.1")), AdvertisingRouter: rid("1.1.1.1")}
	networkKey := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(ip4("10.0.0.1")), AdvertisingRouter: rid("1.1.1.1")}

	clock.Add(5 * time.Second)
	current[0].State = InterfaceStateDown
	current[0].Passive = false
	if count := db.OriginateFromTopology(rid("1.1.1.1"), false); count != 2 {
		t.Fatalf("reoriginate+flush count = %d", count)
	}

	router, ok := db.LookupLSA(area("0.0.0.0"), routerKey)
	if !ok {
		t.Fatalf("router LSA missing")
	}
	body, err := router.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if len(body.Links) != 0 {
		t.Fatalf("down active interface still advertised links: %+v", body.Links)
	}
	network, ok := db.Lookup(area("0.0.0.0"), networkKey)
	if !ok {
		t.Fatalf("network LSA missing")
	}
	if !network.Age.IsMaxAge() {
		t.Fatalf("network LSA not flushed: %+v", network)
	}
}

func TestOSPFOriginateMaxMetric(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, MaxMetric: true, Interfaces: originTopology()}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("originate false")
	}
	lsa, _ := db.LookupLSA(in.AreaID, h.Key())
	body, _ := lsa.DecodeRouter()
	for _, link := range body.Links {
		if link.Type == packet.RouterLinkTypeStub && link.Metric == LSInfinity {
			t.Fatalf("stub metric raised: %+v", link)
		}
		if link.Type != packet.RouterLinkTypeStub && link.Metric != LSInfinity {
			t.Fatalf("non-stub metric not maxed: %+v", link)
		}
	}
}

// RFC requirement: RFC2328-13.4-1 positive -- a received self-originated LSA newer than the instance this router last originated is detected by Advertising Router == own Router ID and answered by re-originating with the LS sequence number advanced one past the received one (handleSelfReceived, origination.go:787-833).
func TestOSPFOriginateSelfReceivedHigherSeq(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("originate false")
	}
	newer := routerLSA(t, rid("1.1.1.1"), h.Sequence.Next().Next(), 10)
	if !db.handleSelfReceived(in.AreaID, newer) {
		t.Fatalf("self-received not handled")
	}
	got, _ := db.Lookup(in.AreaID, h.Key())
	if got.Sequence != newer.Header.Sequence.Next() {
		t.Fatalf("self reorig sequence = %v want %v", got.Sequence, newer.Header.Sequence.Next())
	}
}
