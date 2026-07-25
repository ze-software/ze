// VALIDATES: spec-ospf-14 AC-8 (RFC 2328 §13 step 8: a MaxAge + MaxSequenceNumber database
// copy makes an older received instance a silent discard) and AC-11 (RFC 2328 §13.4: a
// self-originated LSA received with no local copy is flushed by premature aging).
// PREVENTS: a wrapping MaxSeq LSA being disturbed by a stale older instance (sending the DB
// copy back or acking it), and a stale self-originated LSA lingering in the domain after a
// restart instead of being purged.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-4 positive -- when the database copy is MaxAge with LS sequence number MaxSequenceNumber, a received older instance is discarded silently: the database copy is not sent back and no acknowledgment is generated (ReceiveUpdate Older branch, flooding.go:231-238).
func TestOSPFMaxSeqMaxAgeSilentDiscard(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a0 := area("0.0.0.0")
	// DB copy: a 2.2.2.2 router-LSA with MaxSequenceNumber, aged to MaxAge (the wrapping case).
	if !db.Install(a0, routerLSA(t, rid("2.2.2.2"), types.MaxSequenceNumber, 10)) {
		t.Fatalf("install MaxSeq copy rejected")
	}
	clock.Add(time.Duration(types.MaxAge) * time.Second) // age the DB copy to MaxAge
	tx.sends = nil

	// Receiving an older instance must be discarded silently: no DB copy sent back, no ack.
	older := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{older}}})
	db.FlushDelayedAcks("eth0")
	if len(tx.sends) != 0 {
		t.Fatalf("RFC 2328 §13: MaxAge+MaxSeq older instance must be discarded silently, got: %+v", tx.sends)
	}
}

// RFC requirement: RFC2328-13.4-1 negative -- a self-originated LSA (Advertising Router == own Router ID) that this router holds no record of originating is not accepted into the database as-is: it is flushed from the routing domain by premature aging (LS age MaxAge) and reflooded (handleSelfReceived/flushReceivedSelfLSA, origination.go:788-806).
func TestOSPFSelfOriginatedNoLocalCopyFlush(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock) // self router = 1.1.1.1
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a0 := area("0.0.0.0")

	// A self-originated (adv = 1.1.1.1) LSA we hold no local record of -- a stale instance a
	// neighbor still floods after our restart.
	stale := routerLSA(t, rid("1.1.1.1"), types.InitialSequenceNumber.Next(), 10)
	if _, exists := db.Lookup(a0, stale.Header.Key()); exists {
		t.Fatalf("precondition: no local copy expected")
	}
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{stale}}})

	got, ok := db.LookupLSA(a0, stale.Header.Key())
	if !ok {
		t.Fatalf("RFC 2328 §13.4: flush must install a self LSA, none found")
	}
	if !got.Header.Age.IsMaxAge() {
		t.Fatalf("flush must set MaxAge, got age %d", got.Header.Age)
	}
	if got.Header.Sequence != stale.Header.Sequence.Next() {
		t.Fatalf("flush sequence = %v, want received.Next()", got.Header.Sequence)
	}
	if len(tx.sends) == 0 {
		t.Fatalf("flush was not flooded to neighbors")
	}
}

// TestOSPFDRRefloodsBackOutReceivingInterface proves AC-10 (RFC 2328 §13.3 + Table 19): the
// Designated Router re-floods an LSA received from a DROther back out the receiving interface
// to AllSPFRouters (so the other DROthers on the segment receive it), queues every neighbor
// except the sender for retransmit, and sends no acknowledgment (the re-flood is the implicit
// ack). Without this, broadcast segments with 3+ routers do not converge.
// RFC requirement: RFC2328-13.5-1 negative -- the acknowledgment is suppressed exactly where Table 19 says it must be: an LSA flooded back out the receiving interface is NOT acknowledged (ackForReceive floodedBack, flooding.go:616-618).
func TestOSPFDRRefloodsBackOutReceivingInterface(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock) // self/DR router = 1.1.1.1
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkBroadcast, State: InterfaceStateDR,
			Address: ip4("10.0.0.1"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), BDR: rid("2.2.2.2"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateFull},
			},
		}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("3.3.3.3"), Src: netip.MustParseAddr("10.0.0.3"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})

	if len(tx.sends) != 1 || tx.sends[0].iface != "eth0" || tx.sends[0].dst != transport.AllSPFRouters || tx.sends[0].pkt.LSUpdate == nil {
		t.Fatalf("DR did not re-flood out the receiving interface to AllSPFRouters: %+v", tx.sends)
	}
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}][lsa.Header.Key()] == nil {
		t.Fatalf("the other neighbor (BDR 2.2.2.2) was not queued for retransmit")
	}
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("3.3.3.3")}][lsa.Header.Key()] != nil {
		t.Fatalf("the sender 3.3.3.3 must not be queued for retransmit")
	}
	if n := db.FlushDelayedAcks("eth0"); n != 0 {
		t.Fatalf("RFC 2328 Table 19: a flooded-back LSA must not be acknowledged, got %d acks", n)
	}
}
