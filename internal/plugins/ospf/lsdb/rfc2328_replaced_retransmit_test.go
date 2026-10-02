// VALIDATES: RFC 2328 Section 13 step (5)(b) at the LSDB seam: when a newer instance
// replaces the database copy, the old instance leaves every neighbor's Link state
// retransmission list, including the list of a neighbor the new instance is never
// flooded to.
// PREVENTS: a replaced instance staying on the sender's retransmission list, where the
// re-flood of the newer instance cannot overwrite it, and being retransmitted later.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13-5 positive -- the old instance sits on 3.3.3.3's retransmission list (eth1) when 3.3.3.3 itself sends the newer instance; the newer instance is never flooded back to its sender, so only the removal of the database copy from all retransmission lists clears that entry: after the install 3.3.3.3's list holds nothing for the LSA, and a retransmit tick sends nothing to 3.3.3.3 (removeFromAllRetransmit, flooding.go).
func TestRFC2328ReplacedInstanceLeavesSenderRetransmitList(t *testing.T) {
	// Goal: the removal covers a list the newer instance's flood never touches.
	// Method: old instance received on eth0 queues on eth1 for 3.3.3.3; the newer
	// instance then arrives from 3.3.3.3 on eth1 and is flooded out eth0 only.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	old := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	newer := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber.Next(), 20)
	receiveOnEth0(t, db, old)
	if e := eth1Retransmit(db, old.Header.Key()); e == nil || e.lsa.Sequence != old.Header.Sequence {
		t.Fatalf("setup: old instance not queued for 3.3.3.3 on eth1: %+v", e)
	}
	clock.Add(2 * time.Second)
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth1", AreaID: area("0.0.0.0"), RouterID: rid("3.3.3.3"), Src: netip.MustParseAddr("10.0.1.3"), Update: packet.LSUpdate{LSAs: []packet.LSA{newer}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	got, _ := db.Lookup(area("0.0.0.0"), old.Header.Key())
	if got.Sequence != newer.Header.Sequence {
		t.Fatalf("setup: database sequence = %v, want the newer %v installed", got.Sequence, newer.Header.Sequence)
	}
	if e := eth1Retransmit(db, old.Header.Key()); e != nil {
		t.Fatalf("3.3.3.3's retransmission list still holds sequence %v after the database copy was replaced", e.lsa.Sequence)
	}
	tx.sends = nil
	clock.Add(10 * time.Second)
	db.RetransmitTick(clock.Now())
	for i := range tx.sends {
		if tx.sends[i].pkt.LSUpdate == nil || tx.sends[i].iface != "eth1" {
			continue
		}
		t.Fatalf("an LS Update was retransmitted to 3.3.3.3 on eth1 after the replacement: %+v", tx.sends[i])
	}
}
