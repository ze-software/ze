// VALIDATES: RFC 2328 Section 13 step (5)(a) on a multi-LSA Link State Update: the
// instance that arrives within MinLSArrival is discarded unacknowledged, and the receiver
// goes on to the next LSA of the same packet.
// PREVENTS: a receiver that abandons the rest of the update after a too-soon discard; the
// single-LSA units cannot see it.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-7 positive -- one LS Update carries a newer instance of 4.4.4.4's router-LSA 500 ms after its copy was installed by flooding, then a new router-LSA from 5.5.5.5: the first is discarded (the database keeps the earlier sequence, no acknowledgment ever names it) and the receiver examines the next LSA, which is installed, flooded out eth1 and acknowledged (ReceiveUpdate continues past a TooSoon install result, flooding.go).
func TestRFC2328TooSoonDiscardThenNextLSAExamined(t *testing.T) {
	// Goal: both clauses of the sentence in one packet: discard without ack, and go on.
	// Method: floodTopology (eth0 Backup, 2.2.2.2 the DR); the first instance arrives
	// alone; the update under test holds the too-soon instance first, the fresh LSA second.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	first := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	tooSoon := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 20)
	next := routerLSA(t, rid("5.5.5.5"), types.InitialSequenceNumber, 30)
	receiveOnEth0(t, db, first)
	if n := db.FlushDelayedAcks("eth0"); n != 1 {
		t.Fatalf("setup: delayed acks flushed for the first instance = %d, want 1", n)
	}
	tx.sends = nil
	clock.Add(500 * time.Millisecond)
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{tooSoon, next}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	got, _ := db.Lookup(area("0.0.0.0"), first.Header.Key())
	if got.Sequence != first.Header.Sequence {
		t.Fatalf("4.4.4.4 database sequence = %v, want the earlier %v kept (the too-soon instance discarded)", got.Sequence, first.Header.Sequence)
	}
	if _, ok := db.Lookup(area("0.0.0.0"), next.Header.Key()); !ok {
		t.Fatalf("the LSA after the too-soon instance was not installed: the rest of the update was abandoned")
	}
	if !lsaFloodedOn(tx, "eth1", next.Header.Key()) {
		t.Fatalf("the next LSA was not flooded out eth1: %+v", tx.sends)
	}
	// The sends were cleared after the first instance, so any copy of 4.4.4.4 here is the
	// too-soon instance.
	if lsaFloodedOn(tx, "eth1", tooSoon.Header.Key()) {
		t.Fatalf("the too-soon instance was flooded")
	}
	db.FlushDelayedAcks("eth0")
	var acked []packet.LSAHeader
	for _, a := range ackPackets(tx) {
		acked = append(acked, a.pkt.LSAck.Headers...)
	}
	if len(acked) != 1 {
		t.Fatalf("acknowledged headers = %+v, want exactly the next LSA", acked)
	}
	if acked[0].Key() != next.Header.Key() || acked[0].Sequence != next.Header.Sequence {
		t.Fatalf("acknowledged %v seq %v, want the next LSA %v seq %v and never the discarded instance", acked[0].Key(), acked[0].Sequence, next.Header.Key(), next.Header.Sequence)
	}
}
