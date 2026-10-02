// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- sequence number wrap
// RFC: rfc/short/rfc2328.md -- Section 12.1.6 (flush at MaxSequenceNumber, then restart).

package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// wrapNeighbor is one adjacency of floodTopology: eth0 to 2.2.2.2 and eth1 to
// 3.3.3.3, both Full in area 0.
type wrapNeighbor struct {
	iface string
	id    types.RouterID
}

var wrapNeighbors = []wrapNeighbor{{"eth0", rid("2.2.2.2")}, {"eth1", rid("3.3.3.3")}}

// wrapFlush starts from an own summary-LSA at MaxSequenceNumber and asks for the
// next instance, which must be the MaxAge flush. It returns the database, the
// transmit recorder and the flushed header.
func wrapFlush(t *testing.T) (*LSDB, *txRecorder, *fakeClock, packet.LSAHeader) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodTopology)
	a := area("0.0.0.0")
	key := summaryKey()
	db.mu.Lock()
	db.own[a] = map[types.LSAKey]ownRecord{key: {sequence: types.MaxSequenceNumber}}
	db.mu.Unlock()
	h, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 10)
	if !ok || !h.Age.IsMaxAge() || h.Sequence != types.MaxSequenceNumber {
		t.Fatalf("origination past MaxSequenceNumber = %+v ok=%v, want the MaxAge flush at MaxSequenceNumber", h, ok)
	}
	return db, tx, clock, h
}

// floodedOut reports the interfaces an LS Update carried key out of, with the
// sequence number and whether the copy was at MaxAge.
func floodedOut(tx *txRecorder, key types.LSAKey, seq types.LSSequenceNumber, maxAge bool) map[string]bool {
	out := map[string]bool{}
	for i := range tx.sends {
		s := &tx.sends[i]
		if s.pkt.LSUpdate == nil {
			continue
		}
		for _, l := range s.pkt.LSUpdate.LSAs {
			if l.Header.Key() == key && l.Header.Sequence == seq && l.Header.Age.IsMaxAge() == maxAge {
				out[s.iface] = true
			}
		}
	}
	return out
}

// VALIDATES: the three steps in order. The instance at MaxSequenceNumber is
// prematurely aged AND reflooded: an LS Update carries it at MaxAge out of both
// adjacencies' interfaces. Once both neighbors acknowledge it, it leaves the
// database, and the next origination is a new instance at InitialSequenceNumber
// that is itself flooded out of both interfaces.
//
// RFC requirement: RFC2328-12.1.6-1 positive -- incrementing past MaxSequenceNumber floods the LSA at MaxAge with sequence MaxSequenceNumber out of eth0 and eth1; after 2.2.2.2 and 3.3.3.3 both acknowledge it, it is deleted, and the next origination is a new instance at InitialSequenceNumber, age 0, flooded out of both interfaces.
func TestRFC2328SequenceWrapFloodsFlushThenRestarts(t *testing.T) {
	db, tx, clock, h := wrapFlush(t)
	key := summaryKey()
	if got := floodedOut(tx, key, types.MaxSequenceNumber, true); !got["eth0"] || !got["eth1"] {
		t.Fatalf("MaxAge flush flooded out %v, want eth0 and eth1 (sends %+v)", got, tx.sends)
	}
	a := area("0.0.0.0")
	for _, nbr := range wrapNeighbors {
		db.ReceiveAck(AckInput{Interface: nbr.iface, AreaID: a, RouterID: nbr.id, Ack: packet.LSAck{Headers: []packet.LSAHeader{h}}})
	}
	if _, still := db.Lookup(a, key); still {
		t.Fatal("the MaxAge flush acknowledged by every adjacent neighbor is still in the database")
	}
	tx.sends = nil
	clock.Add(10 * time.Second)
	next, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 20)
	if !ok || next.Sequence != types.InitialSequenceNumber || next.Age != 0 {
		t.Fatalf("origination after the acknowledged flush = %+v ok=%v, want InitialSequenceNumber at age 0", next, ok)
	}
	if got := floodedOut(tx, key, types.InitialSequenceNumber, false); !got["eth0"] || !got["eth1"] {
		t.Fatalf("new instance flooded out %v, want eth0 and eth1", got)
	}
}

// VALIDATES: one acknowledgement is not all of them. With only one of the two
// adjacent neighbors having acknowledged the flush (each one tried alone), the
// MaxAge instance stays in the database and a new origination re-issues the
// MaxAge instance at MaxSequenceNumber, never InitialSequenceNumber. A producer
// that restarted after the first acknowledgement fails here.
//
// RFC requirement: RFC2328-12.1.6-1 negative -- after only 2.2.2.2, or only 3.3.3.3, acknowledges the MaxAge flush, the instance is still in the database and the next origination is the MaxAge instance at MaxSequenceNumber, never a new instance at InitialSequenceNumber.
func TestRFC2328SequenceWrapWaitsForEveryNeighbor(t *testing.T) {
	for _, only := range wrapNeighbors {
		t.Run(only.id.String(), func(t *testing.T) {
			db, tx, clock, h := wrapFlush(t)
			a := area("0.0.0.0")
			key := summaryKey()
			db.ReceiveAck(AckInput{Interface: only.iface, AreaID: a, RouterID: only.id, Ack: packet.LSAck{Headers: []packet.LSAHeader{h}}})
			if _, still := db.Lookup(a, key); !still {
				t.Fatalf("the flush left the database after only %s acknowledged it", only.id)
			}
			tx.sends = nil
			clock.Add(10 * time.Second)
			next, ok := db.OriginateSummary(a, rid("1.1.1.1"), types.OptionE, types.LSTypeSummaryNetwork, key.LinkStateID, ip4("255.255.255.0"), 20)
			if !ok || next.Sequence != types.MaxSequenceNumber || !next.Age.IsMaxAge() {
				t.Fatalf("origination after one acknowledgement = %+v ok=%v, want the MaxAge instance at MaxSequenceNumber", next, ok)
			}
			if got := floodedOut(tx, key, types.InitialSequenceNumber, false); len(got) != 0 {
				t.Fatalf("an InitialSequenceNumber instance was flooded out %v before every neighbor acknowledged", got)
			}
		})
	}
}
