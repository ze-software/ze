package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func floodTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkBroadcast, State: InterfaceStateBackup,
			Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("2.2.2.2"), BDR: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
		},
		{
			Name: "eth1", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkBroadcast, State: InterfaceStateDR,
			Address: ip4("10.0.1.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.1.3"), State: NeighborStateFull}},
		},
	}
}

// RFC requirement: RFC2328-13-1 positive -- an LSA with a valid LS checksum and a defined LS type passes the flooding receive checks and is installed in the area database (ReceiveUpdate, flooding.go:161-219).
// RFC requirement: RFC2328-13.3-1 positive -- the LSA flooded out an eligible interface is added to that adjacency's Link state retransmission list, and the receiving adjacency is not (floodExcept/queueRetransmit, flooding.go:358-374, 448-467).
func TestOSPFFloodOutOtherInterfaces(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if _, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key()); !ok {
		t.Fatalf("newer LSA not installed")
	}
	if len(tx.sends) != 1 || tx.sends[0].iface != "eth1" || tx.sends[0].dst != transport.AllSPFRouters {
		t.Fatalf("sends = %+v", tx.sends)
	}
	if db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][lsa.Header.Key()] == nil {
		t.Fatalf("outgoing full neighbor retransmit not queued")
	}
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}][lsa.Header.Key()] != nil {
		t.Fatalf("incoming interface retransmit was queued")
	}
}

// RFC requirement: RFC2328-13.3-1 negative -- a neighbor below Exchange (2-Way) is never added to a retransmission list, so the retransmit obligation is confined to real adjacencies (isFloodEligibleNeighborState, flooding.go:573-580).
func TestOSPFFloodQueuesExchangeAndLoadingNeighbors(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "eth0", AreaID: a, AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, RouterID: rid("1.1.1.1"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateExchange},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateLoading},
				{RouterID: rid("4.4.4.4"), Address: naddr4("10.0.0.4"), State: "two-way"},
			},
		}}
	})
	lsa := routerLSA(t, rid("5.5.5.5"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)

	db.floodExcept("", types.RouterID{}, a, lsa.Header.Key())

	if len(tx.sends) != 1 || tx.sends[0].iface != "eth0" || tx.sends[0].pkt.LSUpdate == nil {
		t.Fatalf("flood sends = %+v", tx.sends)
	}
	for _, peer := range []string{"2.2.2.2", "3.3.3.3"} {
		if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid(peer)}][lsa.Header.Key()] == nil {
			t.Fatalf("neighbor %s was not queued for retransmit", peer)
		}
	}
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("4.4.4.4")}][lsa.Header.Key()] != nil {
		t.Fatalf("two-way neighbor was queued for retransmit")
	}
}

// RFC requirement: RFC2328-13.5-1 positive -- a newly received LSA is acknowledged per Table 19: a more-recent LSA not flooded back yields a delayed acknowledgment, and the Backup acknowledges an implied-ack duplicate received from the DR (ackForReceive, flooding.go:615-639).
func TestOSPFAckDecisionTable(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.queueRetransmit(area("0.0.0.0"), NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}, lsa.Header, lsa.RawBytes)
	db.Install(area("0.0.0.0"), lsa)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	// RFC 2328 Table 19: a duplicate treated as an implied ack on the Backup, received from
	// the DR, is acknowledged by a DELAYED ack (not a direct one); nothing is sent immediately.
	if len(tx.sends) != 0 {
		t.Fatalf("implied-ack duplicate should defer to a delayed ack, got: %+v", tx.sends)
	}
	if n := db.FlushDelayedAcks("eth0"); n != 1 {
		t.Fatalf("implied-ack delayed ack flush = %d", n)
	}
	if len(tx.sends) != 1 || tx.sends[0].pkt.LSAck == nil {
		t.Fatalf("implied-ack delayed ack send = %+v", tx.sends)
	}
	tx.sends = nil
	db.ackForReceive(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2")}, lsa.Header, false, false, true)
	if len(tx.sends) != 0 {
		t.Fatalf("flooded-back ack sent explicit packet: %+v", tx.sends)
	}
	db.ackForReceive(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2")}, lsa.Header, false, false, false)
	if n := db.FlushDelayedAcks("eth0"); n != 1 {
		t.Fatalf("delayed ack flush = %d", n)
	}
	if len(tx.sends) != 1 || tx.sends[0].pkt.LSAck == nil || tx.sends[0].dst != transport.AllSPFRouters {
		t.Fatalf("delayed ack send = %+v", tx.sends)
	}
}

// RFC requirement: RFC2328-13.3-1 positive -- an LSA on a retransmission list is resent every RxmtInterval (not before) and is removed from the list once the neighbor acknowledges it (RetransmitTick flooding.go:506-562, clearRetransmit flooding.go:469-485).
func TestOSPFRetransmitTimer(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.queueRetransmit(area("0.0.0.0"), NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}, lsa.Header, lsa.RawBytes)
	// RFC 2328 §13.5: the first retransmission waits a full RxmtInterval (5s here) after the
	// LSA is queued/flooded; a tick at the queue time must NOT resend.
	if n := db.RetransmitTick(clock.Now()); n != 0 {
		t.Fatalf("retransmit fired before RxmtInterval = %d", n)
	}
	clock.Add(6 * time.Second) // past the 5s RxmtInterval
	if n := db.RetransmitTick(clock.Now()); n != 1 {
		t.Fatalf("retransmit count after interval = %d", n)
	}
	if len(tx.sends) != 1 || tx.sends[0].pkt.LSUpdate == nil {
		t.Fatalf("retransmit send = %+v", tx.sends)
	}
	if tx.sends[0].pkt.LSUpdate.LSAs[0].RawBytes[1] != 1 {
		t.Fatalf("transmit delay not applied to age: % x", tx.sends[0].pkt.LSUpdate.LSAs[0].RawBytes[:2])
	}
	db.ReceiveAck(AckInput{Interface: "eth1", AreaID: area("0.0.0.0"), RouterID: rid("3.3.3.3"), Ack: packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}}})
	if len(db.retransmit) != 0 {
		t.Fatalf("ack did not clear retransmit: %+v", db.retransmit)
	}
}

func TestOSPFRetransmitTimerKeepsExchangeNeighbor(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	peer := rid("2.2.2.2")
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "eth0", AreaID: a, AreaType: AreaTypeNormal, RetransmitInterval: 5,
			Neighbors: []NeighborInfo{{RouterID: peer, Address: naddr4("10.0.0.2"), State: NeighborStateExchange}},
		}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.queueRetransmit(a, NeighborKey{Interface: "eth0", RouterID: peer}, lsa.Header, lsa.RawBytes)
	clock.Add(6 * time.Second)

	if n := db.RetransmitTick(clock.Now()); n != 1 {
		t.Fatalf("exchange retransmit count = %d, want 1", n)
	}
	if len(tx.sends) != 1 || tx.sends[0].pkt.LSUpdate == nil {
		t.Fatalf("exchange retransmit send = %+v", tx.sends)
	}
}

func TestOSPFv6ASExternalRetransmitClearsAcrossAreas(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	key := types.LSAKey{Type: types.LSType(0x4005), LinkStateID: lsid("0.0.0.9"), AdvertisingRouter: rid("1.1.1.1")}
	header := packet.LSAHeader{Type: key.Type, LinkStateID: key.LinkStateID, AdvertisingRouter: key.AdvertisingRouter, Sequence: types.InitialSequenceNumber}
	nbr := NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}
	db.queueRetransmit(area("0.0.0.1"), nbr, header, []byte{0, 1, 0x40, 0x05})

	db.removeFromAllRetransmit(area("0.0.0.0"), key)

	if len(db.retransmit) != 0 {
		t.Fatalf("OSPFv3 AS-External retransmit was not cleared AS-wide: %+v", db.retransmit)
	}
}

func TestOSPFv6LinkLSARetransmitted(t *testing.T) {
	// Regression for B1: the retransmit path must NOT decode queued LSAs through the OSPFv2
	// codec. An OSPFv3 Link-LSA (type 0x0008) is unknown to packet.DecodeLSA, so the old code
	// dropped it from the retransmit list (delete(lst, key)) instead of resending, silently
	// breaking RFC 2328 sec 13.5 reliable flooding for the v6 Link-LSA family. The fix resends
	// the stored raw verbatim with a re-stamped age, so the tick returns 1 and keeps the entry.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)

	nbr := NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}
	// A 20-octet OSPFv3 LSA header carrying type 0x0008 (Link-LSA): bytes [2:4] = 00 08.
	raw := make([]byte, types.LSAHeaderLen)
	raw[3] = 0x08
	raw[len(raw)-1] = byte(types.LSAHeaderLen) // length field
	header := packet.LSAHeader{Type: types.LSType(0x0008), LinkStateID: lsid("0.0.0.5"), AdvertisingRouter: rid("4.4.4.4"), Sequence: types.InitialSequenceNumber}
	db.queueRetransmit(area("0.0.0.0"), nbr, header, raw)

	clock.Add(6 * time.Second) // past the 5s RxmtInterval
	if n := db.RetransmitTick(clock.Now()); n != 1 {
		t.Fatalf("v6 Link-LSA retransmit count = %d, want 1 (the v2 decoder must not drop it)", n)
	}
	if len(db.retransmit[nbr]) != 1 {
		t.Fatalf("v6 Link-LSA dropped from the retransmit list (want it retained until acked): %+v", db.retransmit)
	}
}

func TestOSPFLSAckClearsRetransmit(t *testing.T) {
	TestOSPFRetransmitTimer(t)
}

func TestOSPFMinLSArrivalReject(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(floodTopology)
	old := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	newer := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 20)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{old}}})
	clock.Add(500 * time.Millisecond)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{newer}}})
	got, _ := db.Lookup(area("0.0.0.0"), old.Header.Key())
	if got.Sequence != old.Header.Sequence {
		t.Fatalf("MinLSArrival accepted too soon: got %v", got.Sequence)
	}
}

// RFC requirement: RFC2328-13-2 negative -- a Type-5 AS-external-LSA arriving on a stub-area interface is discarded and never enters the database, so it cannot be flooded onward inside the stub (shouldDropByArea, flooding.go:270-283).
func TestOSPFStubAreaDropsType5(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(func() []InterfaceInfo {
		ifs := floodTopology()
		ifs[0].AreaType = AreaTypeStub
		return ifs[:1]
	})
	lsa := externalLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber)
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if _, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key()); ok {
		t.Fatalf("type 5 installed in stub area")
	}
}

func TestOSPFStubAreaSummaryHidesType5(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	normal := area("0.0.0.0")
	stub := area("0.0.0.1")
	db.SetAreaTypes(map[types.AreaID]string{stub: AreaTypeStub})
	lsa := externalLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber)
	if !db.Install(normal, lsa) {
		t.Fatalf("external install failed")
	}
	if _, ok := db.LookupLSA(stub, lsa.Header.Key()); ok {
		t.Fatalf("stub area lookup exposed Type 5")
	}
	if got := db.Summary(stub); len(got) != 0 {
		t.Fatalf("stub summary exposed Type 5: %+v", got)
	}
	if got := db.Summary(normal); len(got) != 1 || got[0].Key() != lsa.Header.Key() {
		t.Fatalf("normal summary missing Type 5: %+v", got)
	}
}

func TestOSPFRetransmitAckScopedByArea(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	a1 := area("0.0.0.1")
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	n0 := NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}
	n1 := NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}
	db.queueRetransmit(a0, n0, lsa.Header, lsa.RawBytes)
	db.queueRetransmit(a1, n1, lsa.Header, lsa.RawBytes)
	db.ReceiveAck(AckInput{Interface: n0.Interface, AreaID: a0, RouterID: n0.RouterID, Ack: packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}}})
	if db.retransmit[n0] != nil {
		t.Fatalf("area 0 retransmit not cleared: %+v", db.retransmit[n0])
	}
	if db.retransmit[n1][lsa.Header.Key()] == nil {
		t.Fatalf("area 1 retransmit cleared by area 0 ack")
	}
}

func TestOSPFASExternalNewerClearsRetransmitsAcrossAreas(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	a1 := area("0.0.0.1")
	lsa := externalLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next())
	n0 := NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}
	n1 := NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}
	db.queueRetransmit(a0, n0, lsa.Header, lsa.RawBytes)
	db.queueRetransmit(a1, n1, lsa.Header, lsa.RawBytes)
	db.removeFromAllRetransmit(a0, lsa.Header.Key())
	if len(db.retransmit) != 0 {
		t.Fatalf("AS-external retransmits not cleared across areas: %+v", db.retransmit)
	}
}

func TestOSPFASExternalPurgeRetainedAcrossAreas(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	a1 := area("0.0.0.1")
	lsa := externalLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next())
	key := lsa.Header.Key()
	if !db.Install(a0, lsa) {
		t.Fatalf("external install failed")
	}
	db.mu.Lock()
	db.asExternal.entries[key].markPurged(clock.Now())
	db.mu.Unlock()
	n0 := NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}
	n1 := NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}
	db.queueRetransmit(a0, n0, lsa.Header, lsa.RawBytes)
	db.queueRetransmit(a1, n1, lsa.Header, lsa.RawBytes)
	db.ReceiveAck(AckInput{Interface: n0.Interface, AreaID: a0, RouterID: n0.RouterID, Ack: packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}}})
	if _, ok := db.Lookup(a0, key); !ok {
		t.Fatalf("AS-external purge deleted while another area had retransmit")
	}
	db.ReceiveAck(AckInput{Interface: n1.Interface, AreaID: a1, RouterID: n1.RouterID, Ack: packet.LSAck{Headers: []packet.LSAHeader{lsa.Header}}})
	if _, ok := db.Lookup(a0, key); ok {
		t.Fatalf("AS-external purge retained after all areas acked")
	}
}

// RFC requirement: RFC2328-13.5-1 positive -- a MaxAge LSA with no database copy and no neighbor in Exchange or Loading is acknowledged DIRECTLY and then discarded, which is the Table 19 direct-ack row (ReceiveUpdate MaxAge branch, flooding.go:204-209).
func TestOSPFUnknownMaxAgeNoCopyIsAckedAndDiscarded(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	base := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	h := base.Header
	h.Age = types.LSAge(types.MaxAge)
	lsa := encodeDecodeLSA(t, packet.LSA{Header: h, Body: base.Body})
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if _, ok := db.Lookup(area("0.0.0.0"), lsa.Header.Key()); ok {
		t.Fatalf("unknown MaxAge LSA installed")
	}
	if len(tx.sends) != 1 || tx.sends[0].pkt.LSAck == nil {
		t.Fatalf("unknown MaxAge LSA not directly acked: %+v", tx.sends)
	}
}

// RFC requirement: RFC2328-14-2 negative -- a MaxAge LSA is NOT removed while a neighbor is in Loading (or Exchange); the deletion is refused until that neighbor leaves (deletePurgedIfAcked retainForLoading, flooding.go:785, 799-802).
func TestOSPFPurgeRetainedForExchangeOrLoading(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a := area("0.0.0.0")
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: lsid("4.4.4.4"), AdvertisingRouter: rid("4.4.4.4")}
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{Name: "eth0", AreaID: a, Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), State: NeighborStateLoading}}}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)
	db.mu.Lock()
	db.areas[a].entries[key].markPurged(clock.Now())
	db.mu.Unlock()
	db.deletePurgedIfAcked(a, key)
	if _, ok := db.Lookup(a, key); !ok {
		t.Fatalf("purge deleted while Loading neighbor exists")
	}
	db.SetTopology(func() []InterfaceInfo { return []InterfaceInfo{{Name: "eth0", AreaID: a}} })
	db.deletePurgedIfAcked(a, key)
	if _, ok := db.Lookup(a, key); ok {
		t.Fatalf("purge retained after Loading neighbor left")
	}
}

func TestOSPFLSDBSync(t *testing.T) {
	TestOSPFFloodOutOtherInterfaces(t)
}

func TestOSPFFloodingFunctional(t *testing.T) {
	t.Run("flood", TestOSPFFloodOutOtherInterfaces)
	t.Run("purge", TestOSPFLSDBAgeToPurge)
}
