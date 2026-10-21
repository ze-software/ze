// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- receiving self-originated LSAs.
// Related: origination.go -- handleSelfReceived, the RFC 2328 Section 13.4 special action.
//
// VALIDATES: RFC 2328 Section 13.4, the advance-and-reoriginate action is taken only for a
// received self-originated LSA that is newer than the instance the router holds.
// PREVENTS: a router that bumps its own sequence number for every self-originated LSA it
// hears, including an older copy still in flight, and so churns the routing domain.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-13.4-2 negative -- a received self-originated router-LSA that is OLDER than the instance in the database (sequence Initial+1, while the database holds Initial+3 after answering a newer copy) is not answered by a new instance: the database keeps sequence Initial+3 and never takes Initial+2 (handleSelfReceived, origination.go).
//
// Goal: the special action is bound to "newer than the last instance originated". Method:
// originate a router-LSA, answer a newer self copy so the database moves to Initial+3, then
// hand handleSelfReceived the older Initial+1 copy and read the database sequence back.
func TestRFC2328SelfReceivedOlderIsNotReoriginated(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()}
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatal("originate false")
	}
	if h.Sequence != types.InitialSequenceNumber {
		t.Fatalf("setup: originated sequence = %v, want %v", h.Sequence, types.InitialSequenceNumber)
	}
	newer := routerLSA(t, rid("1.1.1.1"), h.Sequence.Next().Next(), 10)
	if !db.handleSelfReceived(in.AreaID, newer) {
		t.Fatal("setup: the newer self copy was not handled")
	}
	held, _ := db.Lookup(in.AreaID, h.Key())
	if held.Sequence != newer.Header.Sequence.Next() {
		t.Fatalf("setup: database sequence = %v, want %v", held.Sequence, newer.Header.Sequence.Next())
	}

	older := routerLSA(t, rid("1.1.1.1"), h.Sequence.Next(), 10)
	db.handleSelfReceived(in.AreaID, older)
	got, ok := db.Lookup(in.AreaID, h.Key())
	if !ok {
		t.Fatal("the router-LSA left the database")
	}
	if got.Sequence != held.Sequence {
		t.Fatalf("database sequence after an older self copy = %v, want %v unchanged (no advance past %v)", got.Sequence, held.Sequence, older.Header.Sequence)
	}
}
