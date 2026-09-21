// VALIDATES: RFC 2328 Sections 12.1.6, 12.2, 12.4, 12.4.1, 12.4.2, 13 and 13.3 at the
// LSDB seam -- sequence wrap flushes before InitialSequenceNumber returns, lookups are
// keyed on the LS type / Link State ID / Advertising Router triple, a withdrawn summary
// is flushed at MaxAge, one router-LSA carries every link to an area, a network-LSA is
// originated only by the DR, a replaced instance leaves every retransmission list, a
// too-soon instance is discarded without an acknowledgment, NBMA floods and delayed
// acknowledgments go out as per-adjacency unicasts, and a Type 4 summary-LSA carries a
// zero Network Mask.
// PREVENTS: a wrapped sequence restarting before the MaxAge flood is acknowledged, a
// lookup matching on partial identity, a stale summary staying advertised, a link to
// an area being split across router-LSAs, a non-DR originating a network-LSA, a
// stale instance being retransmitted, an ack naming an instance the router discarded,
// an NBMA flood reaching the multicast group, and a Type 4 body carrying a mask.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func receiveOnEth0(t *testing.T, db *LSDB, lsa packet.LSA) {
	t.Helper()
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
}

func ackPackets(tx *txRecorder) []sentPacket {
	var acks []sentPacket
	for _, s := range tx.sends {
		if s.pkt.LSAck != nil {
			acks = append(acks, s)
		}
	}
	return acks
}

// RFC requirement: RFC2328-13-7 negative -- a newer instance of an LSA arriving inside MinLSArrival of the flooded install of its database copy is discarded: the database keeps the earlier sequence, no acknowledgment packet is sent for it, and the delayed acknowledgment later flushed names the installed instance, never the discarded one (ReceiveUpdate skips a TooSoon install result, flooding.go; arrivedTooSoonLocked, lsdb.go).
func TestRFC2328MinLSArrivalDiscardsWithoutAck(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	first := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	tooSoon := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 20)
	receiveOnEth0(t, db, first)
	sendsAfterFirst := len(tx.sends)
	clock.Add(500 * time.Millisecond)
	receiveOnEth0(t, db, tooSoon)
	got, _ := db.Lookup(area("0.0.0.0"), first.Header.Key())
	if got.Sequence != first.Header.Sequence {
		t.Fatalf("database sequence = %v, want the first instance %v kept", got.Sequence, first.Header.Sequence)
	}
	if len(tx.sends) != sendsAfterFirst {
		t.Fatalf("the discarded instance produced %d packets: %+v", len(tx.sends)-sendsAfterFirst, tx.sends[sendsAfterFirst:])
	}
	if acks := ackPackets(tx); len(acks) != 0 {
		t.Fatalf("a direct acknowledgment was sent for the discarded instance: %+v", acks)
	}
	if n := db.FlushDelayedAcks("eth0"); n != 1 {
		t.Fatalf("delayed acks flushed = %d, want 1 (the installed instance)", n)
	}
	acks := ackPackets(tx)
	if len(acks) != 1 || len(acks[0].pkt.LSAck.Headers) != 1 {
		t.Fatalf("acks = %+v, want one packet with one header", acks)
	}
	if seq := acks[0].pkt.LSAck.Headers[0].Sequence; seq != first.Header.Sequence {
		t.Fatalf("acknowledged sequence = %v, want the installed %v, never the discarded %v", seq, first.Header.Sequence, tooSoon.Header.Sequence)
	}
}

// RFC requirement: RFC2328-13-7 positive -- a newer instance arriving once MinLSArrival has elapsed since the database copy was installed is accepted: the database holds the new sequence and the instance is acknowledged (installLocked accepts when arrivedTooSoonLocked is false, lsdb.go; ackForReceive, flooding.go).
func TestRFC2328MinLSArrivalElapsedAccepts(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	first := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	newer := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 20)
	receiveOnEth0(t, db, first)
	clock.Add(time.Second)
	receiveOnEth0(t, db, newer)
	got, _ := db.Lookup(area("0.0.0.0"), first.Header.Key())
	if got.Sequence != newer.Header.Sequence {
		t.Fatalf("database sequence = %v, want the newer instance %v", got.Sequence, newer.Header.Sequence)
	}
	if n := db.FlushDelayedAcks("eth0"); n != 1 {
		t.Fatalf("delayed acks flushed = %d, want 1", n)
	}
	acks := ackPackets(tx)
	if len(acks) != 1 || len(acks[0].pkt.LSAck.Headers) != 1 || acks[0].pkt.LSAck.Headers[0].Sequence != newer.Header.Sequence {
		t.Fatalf("acks = %+v, want one acknowledgment of sequence %v", acks, newer.Header.Sequence)
	}
}

func eth1Retransmit(db *LSDB, key types.LSAKey) *retransmitEntry {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][key]
}

// RFC requirement: RFC2328-13-5 positive -- when a newer instance replaces the database copy, the old instance is removed from every neighbor's Link state retransmission list before the new one is queued, so the list holds only the replacing instance and the retransmit timer resends only it (removeFromAllRetransmit, flooding.go).
func TestRFC2328ReplacedInstanceLeavesRetransmitLists(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	old := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	newer := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 20)
	receiveOnEth0(t, db, old)
	if e := eth1Retransmit(db, old.Header.Key()); e == nil || e.lsa.Sequence != old.Header.Sequence {
		t.Fatalf("old instance not queued on eth1: %+v", e)
	}
	clock.Add(2 * time.Second)
	receiveOnEth0(t, db, newer)
	e := eth1Retransmit(db, old.Header.Key())
	if e == nil || e.lsa.Sequence != newer.Header.Sequence {
		t.Fatalf("eth1 retransmit entry = %+v, want only the replacing sequence %v", e, newer.Header.Sequence)
	}
	tx.sends = nil
	clock.Add(10 * time.Second)
	db.RetransmitTick(clock.Now())
	for _, s := range tx.sends {
		if s.pkt.LSUpdate == nil {
			continue
		}
		for _, l := range s.pkt.LSUpdate.LSAs {
			if l.Header.Sequence == old.Header.Sequence {
				t.Fatalf("the replaced instance was retransmitted: %+v", s)
			}
		}
	}
}

// RFC requirement: RFC2328-13-5 negative -- a duplicate of the instance already on a neighbor's retransmission list does not remove it: the entry survives, because only a deletion or a replacement of the database copy clears a retransmission list (removeFromAllRetransmit runs only on a Newer install, flooding.go; clearRetransmit clears the receiving adjacency alone).
func TestRFC2328DuplicateKeepsOtherRetransmitLists(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	old := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	receiveOnEth0(t, db, old)
	clock.Add(2 * time.Second)
	receiveOnEth0(t, db, old)
	if e := eth1Retransmit(db, old.Header.Key()); e == nil || e.lsa.Sequence != old.Header.Sequence {
		t.Fatalf("a duplicate on eth0 cleared eth1's retransmit entry: %+v", e)
	}
}

// RFC requirement: RFC2328-12.2-1 positive -- an installed LSA is found by the triple LS type, Link State ID and Advertising Router, and the header returned is that instance (Lookup, lsdb.go).
func TestRFC2328LookupByTriple(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	if !db.Install(area("0.0.0.0"), lsa) {
		t.Fatal("Install refused")
	}
	h, ok := db.Lookup(area("0.0.0.0"), types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("4.4.4.4")), AdvertisingRouter: rid("4.4.4.4")})
	if !ok || h.Sequence != lsa.Header.Sequence || h.AdvertisingRouter != rid("4.4.4.4") {
		t.Fatalf("Lookup = %+v ok=%v, want the installed instance", h, ok)
	}
}

// RFC requirement: RFC2328-12.2-1 negative -- a lookup whose Advertising Router differs from the installed LSA's finds nothing, although the LS type and Link State ID match, so partial identity never answers for a full one (Lookup, lsdb.go).
func TestRFC2328LookupNeedsFullTriple(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	if !db.Install(area("0.0.0.0"), lsa) {
		t.Fatal("Install refused")
	}
	h, ok := db.Lookup(area("0.0.0.0"), types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("4.4.4.4")), AdvertisingRouter: rid("5.5.5.5")})
	if ok {
		t.Fatalf("Lookup with a different Advertising Router answered %+v", h)
	}
}

func summaryKey(lsidText string) types.LSAKey {
	return types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: lsid(lsidText), AdvertisingRouter: rid("1.1.1.1")}
}

// RFC requirement: RFC2328-12.1.6-1 negative -- while the MaxAge flush of an LSA whose sequence reached MaxSequenceNumber is not yet acknowledged by every adjacent neighbor, a re-origination does not produce a new instance at InitialSequenceNumber: it re-issues the MaxAge instance at MaxSequenceNumber (nextOwnSequenceForce, origination.go).
func TestRFC2328SequenceWrapWaitsForAck(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a := area("0.0.0.0")
	key := summaryKey("192.0.2.0")
	db.mu.Lock()
	db.own[a] = map[types.LSAKey]ownRecord{key: {sequence: types.MaxSequenceNumber}}
	db.mu.Unlock()
	h, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 10)
	if !ok || !h.Age.IsMaxAge() || h.Sequence != types.MaxSequenceNumber {
		t.Fatalf("origination past MaxSequenceNumber = %+v ok=%v, want a MaxAge flush at MaxSequenceNumber", h, ok)
	}
	clock.Add(10 * time.Second)
	h, ok = db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 20)
	if !ok || h.Sequence == types.InitialSequenceNumber || !h.Age.IsMaxAge() {
		t.Fatalf("re-origination before the flush was acknowledged = %+v ok=%v, want the MaxAge instance again, never InitialSequenceNumber", h, ok)
	}
}

// RFC requirement: RFC2328-12.1.6-1 positive -- once every adjacent neighbor has acknowledged the MaxAge flush of the MaxSequenceNumber instance, the next origination introduces the new instance at InitialSequenceNumber with age 0 (deletePurgedIfAcked drops the own-sequence record, flooding.go; nextOwnSequenceForce restarts, origination.go).
func TestRFC2328SequenceWrapRestartsAfterAck(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a := area("0.0.0.0")
	key := summaryKey("192.0.2.0")
	db.mu.Lock()
	db.own[a] = map[types.LSAKey]ownRecord{key: {sequence: types.MaxSequenceNumber}}
	db.mu.Unlock()
	h, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 10)
	if !ok || !h.Age.IsMaxAge() {
		t.Fatalf("flush = %+v ok=%v", h, ok)
	}
	for _, nbr := range []struct {
		iface string
		id    types.RouterID
	}{{"eth0", rid("2.2.2.2")}, {"eth1", rid("3.3.3.3")}} {
		db.ReceiveAck(AckInput{Interface: nbr.iface, AreaID: a, RouterID: nbr.id, Ack: packet.LSAck{Headers: []packet.LSAHeader{h}}})
	}
	if _, still := db.Lookup(a, key); still {
		t.Fatal("the acknowledged MaxAge instance is still in the database")
	}
	clock.Add(10 * time.Second)
	h, ok = db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 20)
	if !ok || h.Sequence != types.InitialSequenceNumber || h.Age != 0 {
		t.Fatalf("origination after the acknowledged flush = %+v ok=%v, want InitialSequenceNumber at age 0", h, ok)
	}
}

// RFC requirement: RFC2328-12.4-1 positive -- a self-originated summary-LSA whose destination is no longer advertised is flushed: its LS age becomes MaxAge and the MaxAge instance is reflooded to the area (FlushStaleSummaryLSAs, origination.go).
func TestRFC2328WithdrawnSummaryFlushedAtMaxAge(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a := area("0.0.0.0")
	key := summaryKey("192.0.2.0")
	if _, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 10); !ok {
		t.Fatal("OriginateSummary refused")
	}
	tx.sends = nil
	clock.Add(10 * time.Second)
	if n := db.FlushStaleSummaryLSAs(a, rid("1.1.1.1"), map[types.LSAKey]struct{}{}); n != 1 {
		t.Fatalf("flushed = %d, want 1", n)
	}
	h, ok := db.Lookup(a, key)
	if !ok || !h.Age.IsMaxAge() {
		t.Fatalf("withdrawn summary = %+v ok=%v, want MaxAge", h, ok)
	}
	flooded := false
	for _, s := range tx.sends {
		if s.pkt.LSUpdate == nil {
			continue
		}
		for _, l := range s.pkt.LSUpdate.LSAs {
			if l.Header.Key() == key && l.Header.Age.IsMaxAge() {
				flooded = true
			}
		}
	}
	if !flooded {
		t.Fatalf("the MaxAge instance was not reflooded: %+v", tx.sends)
	}
}

// RFC requirement: RFC2328-12.4-1 negative -- a summary-LSA whose destination is still advertised is not flushed: its age stays below MaxAge and no MaxAge instance is flooded (FlushStaleSummaryLSAs keeps every key in keep, origination.go).
func TestRFC2328AdvertisedSummaryNotFlushed(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a := area("0.0.0.0")
	key := summaryKey("192.0.2.0")
	if _, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 10); !ok {
		t.Fatal("OriginateSummary refused")
	}
	tx.sends = nil
	clock.Add(10 * time.Second)
	if n := db.FlushStaleSummaryLSAs(a, rid("1.1.1.1"), map[types.LSAKey]struct{}{key: {}}); n != 0 {
		t.Fatalf("flushed = %d, want 0", n)
	}
	h, ok := db.Lookup(a, key)
	if !ok || h.Age.IsMaxAge() {
		t.Fatalf("kept summary = %+v ok=%v, want it advertised below MaxAge", h, ok)
	}
	if len(tx.sends) != 0 {
		t.Fatalf("a kept summary produced packets: %+v", tx.sends)
	}
}

func twoInterfacesOneArea() []InterfaceInfo {
	ifs := floodTopology()
	ifs[0].Cost, ifs[1].Cost = 10, 20
	return ifs
}

// RFC requirement: RFC2328-12.4.1-1 positive -- a router with two interfaces in one area originates a single router-LSA for that area, and its body carries one link for each interface (routerLinks over every interface of the area, origination.go).
func TestRFC2328SingleRouterLSACarriesEveryLink(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(twoInterfacesOneArea)
	a := area("0.0.0.0")
	db.OriginateFromTopology(rid("1.1.1.1"), false)
	var routerLSAs []packet.LSAHeader
	for _, h := range db.Summary(a) {
		if h.Type == types.LSTypeRouter && h.AdvertisingRouter == rid("1.1.1.1") {
			routerLSAs = append(routerLSAs, h)
		}
	}
	if len(routerLSAs) != 1 {
		t.Fatalf("router-LSAs for the area = %d, want exactly one: %+v", len(routerLSAs), routerLSAs)
	}
	lsa, _ := db.LookupLSA(a, routerLSAs[0].Key())
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	transit := map[types.LinkStateID]bool{}
	for _, l := range body.Links {
		if l.Type == packet.RouterLinkTypeTransit {
			transit[l.LinkID] = true
		}
	}
	if len(transit) != 2 || !transit[lsid("10.0.0.2")] || !transit[lsid("10.0.1.1")] {
		t.Fatalf("router-LSA links = %+v, want one transit link for each interface (eth0 to DR 10.0.0.2, eth1 as DR 10.0.1.1)", body.Links)
	}
}

// RFC requirement: RFC2328-12.4.1-1 negative -- an interface attached to another area is not described in this area's router-LSA: the backbone router-LSA carries only the backbone interface's link (OriginateFromTopology groups interfaces by area before routerLinks, origination.go).
func TestRFC2328RouterLSAExcludesOtherAreaLinks(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(func() []InterfaceInfo {
		ifs := twoInterfacesOneArea()
		ifs[1].AreaID = area("0.0.0.1")
		return ifs
	})
	db.OriginateFromTopology(rid("1.1.1.1"), false)
	lsa, ok := db.LookupLSA(area("0.0.0.0"), types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid("1.1.1.1")), AdvertisingRouter: rid("1.1.1.1")})
	if !ok {
		t.Fatal("no backbone router-LSA")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	for _, l := range body.Links {
		if l.LinkID == lsid("10.0.1.1") || l.LinkID == lsid("10.0.1.0") {
			t.Fatalf("backbone router-LSA describes the area 0.0.0.1 interface eth1: %+v", body.Links)
		}
	}
	transit := 0
	for _, l := range body.Links {
		if l.Type == packet.RouterLinkTypeTransit && l.LinkID == lsid("10.0.0.2") {
			transit++
		}
	}
	if transit != 1 {
		t.Fatalf("backbone router-LSA links = %+v, want the eth0 transit link and nothing from eth1", body.Links)
	}
}

// RFC requirement: RFC2328-9.1-1 positive -- the router that is Designated Router for an attached broadcast network originates a network-LSA for it, with Link State ID the interface address and the DR plus every Full neighbor attached (OriginateFromTopology for iface.DR == router, OriginateNetwork, origination.go).
func TestRFC2328DROriginatesNetworkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(floodTopology)
	db.OriginateFromTopology(rid("1.1.1.1"), false)
	key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(ip4("10.0.1.1")), AdvertisingRouter: rid("1.1.1.1")}
	lsa, ok := db.LookupLSA(area("0.0.0.0"), key)
	if !ok {
		t.Fatal("DR interface eth1 has no network-LSA")
	}
	body, err := lsa.DecodeNetwork()
	if err != nil {
		t.Fatalf("DecodeNetwork: %v", err)
	}
	if len(body.AttachedRouters) != 2 || body.AttachedRouters[0] != rid("1.1.1.1") || body.AttachedRouters[1] != rid("3.3.3.3") {
		t.Fatalf("attached routers = %v, want [1.1.1.1 3.3.3.3]", body.AttachedRouters)
	}
}

// RFC requirement: RFC2328-9.1-1 negative -- a router that is not Designated Router for an attached network (here Backup on eth0, whose DR is 2.2.2.2) originates no network-LSA for it (OriginateFromTopology skips iface.DR != router, origination.go).
func TestRFC2328NonDROriginatesNoNetworkLSA(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(floodTopology)
	db.OriginateFromTopology(rid("1.1.1.1"), false)
	key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(ip4("10.0.0.1")), AdvertisingRouter: rid("1.1.1.1")}
	if h, ok := db.Lookup(area("0.0.0.0"), key); ok {
		t.Fatalf("Backup originated a network-LSA for eth0: %+v", h)
	}
}

func nbmaTopology(a types.AreaID) func() []InterfaceInfo {
	return func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "nb0", AreaID: a, AreaType: types.AreaTypeNormal, NetworkType: types.NetworkNBMA, State: InterfaceStateDR,
			Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateExchange},
				{RouterID: rid("4.4.4.4"), Address: naddr4("10.0.0.4"), State: "two-way"},
			},
		}}
	}
}

// RFC requirement: RFC2328-13.3-3 positive -- on an NBMA network a flooded Link State Update goes out as one unicast to each neighbor in state Exchange or greater, and a delayed Link State Acknowledgment is likewise unicast once to each such neighbor (floodExcept's nonBroadcast fan-out and FlushDelayedAcks, flooding.go).
func TestRFC2328NBMAUnicastsToEachAdjacency(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	db.SetTopology(nbmaTopology(a))
	lsa := routerLSA(t, rid("5.5.5.5"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)
	db.floodExcept("", types.RouterID{}, a, lsa.Header.Key())
	updates := map[netip.Addr]int{}
	for _, s := range tx.sends {
		if s.pkt.LSUpdate != nil {
			updates[s.dst]++
		}
	}
	if len(updates) != 2 || updates[naddr4("10.0.0.2")] != 1 || updates[naddr4("10.0.0.3")] != 1 {
		t.Fatalf("LSUpdate unicasts = %v, want one each to 10.0.0.2 (Full) and 10.0.0.3 (Exchange)", updates)
	}
	tx.sends = nil
	db.queueDelayedAck("nb0", lsa.Header)
	if n := db.FlushDelayedAcks("nb0"); n != 1 {
		t.Fatalf("delayed acks flushed = %d, want 1", n)
	}
	acks := map[netip.Addr]int{}
	for _, s := range ackPackets(tx) {
		acks[s.dst]++
	}
	if len(acks) != 2 || acks[naddr4("10.0.0.2")] != 1 || acks[naddr4("10.0.0.3")] != 1 {
		t.Fatalf("LSAck unicasts = %v, want one each to 10.0.0.2 and 10.0.0.3", acks)
	}
}

// RFC requirement: RFC2328-13.3-3 negative -- on an NBMA network neither a Link State Update nor a delayed Link State Acknowledgment is sent to the AllSPFRouters multicast group or to a neighbor below Exchange (2-Way) (floodExcept skips isFloodEligibleNeighborState false and never uses floodDestination when nonBroadcast, flooding.go).
func TestRFC2328NBMANeverMulticastsNorReachesTwoWay(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	db.SetTopology(nbmaTopology(a))
	lsa := routerLSA(t, rid("5.5.5.5"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)
	db.floodExcept("", types.RouterID{}, a, lsa.Header.Key())
	db.queueDelayedAck("nb0", lsa.Header)
	db.FlushDelayedAcks("nb0")
	if len(tx.sends) == 0 {
		t.Fatal("nothing was sent")
	}
	for _, s := range tx.sends {
		if s.dst == transport.AllSPFRouters || s.dst == transport.AllDRouters {
			t.Fatalf("NBMA packet sent to a multicast group: %+v", s)
		}
		if s.dst == naddr4("10.0.0.4") {
			t.Fatalf("NBMA packet sent to a 2-Way neighbor: %+v", s)
		}
	}
}

// RFC requirement: RFC2328-A.4.4-1 positive -- a Type 4 (ASBR) summary-LSA is originated with a zero Network Mask whatever mask the caller passes (OriginateSummary zeroes body.NetworkMask for LSTypeSummaryASBR, origination.go).
func TestRFC2328Type4SummaryMaskIsZero(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a := area("0.0.0.0")
	h, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryASBR, types.LinkStateID(rid("9.9.9.9")), ip4("255.255.255.0"), 10)
	if !ok {
		t.Fatal("OriginateSummary refused")
	}
	lsa, _ := db.LookupLSA(a, h.Key())
	body, err := lsa.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary: %v", err)
	}
	if body.NetworkMask != ([4]byte{}) {
		t.Fatalf("Type 4 Network Mask = %v, want 0.0.0.0", body.NetworkMask)
	}
}

// RFC requirement: RFC2328-A.4.4-1 negative -- the zeroing is confined to Type 4: a Type 3 summary-LSA originated with the same arguments keeps the caller's Network Mask (OriginateSummary, origination.go).
func TestRFC2328Type3SummaryKeepsMask(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a := area("0.0.0.0")
	h, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, lsid("192.0.2.0"), ip4("255.255.255.0"), 10)
	if !ok {
		t.Fatal("OriginateSummary refused")
	}
	lsa, _ := db.LookupLSA(a, h.Key())
	body, err := lsa.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary: %v", err)
	}
	if body.NetworkMask != ip4("255.255.255.0") {
		t.Fatalf("Type 3 Network Mask = %v, want 255.255.255.0", body.NetworkMask)
	}
}
