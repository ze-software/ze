// VALIDATES: RFC 2328 Section 14 clause (b): a MaxAge LSA on no retransmission list is
// kept while any neighbor is in Exchange OR Loading, and deleted at once when the only
// neighbor is in neither state.
// PREVENTS: a deletion guard that checks Loading alone, which would delete the LSA
// while an Exchange neighbor's Database Description summary still lists it.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// maxAgePurgeWith installs 4.4.4.4's router-LSA, marks it purged at MaxAge with no
// retransmission list entry, runs the deletion check with one neighbor in state, and
// reports whether the LSA is still in the database.
func maxAgePurgeWith(t *testing.T, state string) bool {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a := area("0.0.0.0")
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: lsid("4.4.4.4"), AdvertisingRouter: rid("4.4.4.4")}
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{Name: "eth0", AreaID: a, Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), State: state}}}}
	})
	db.Install(a, routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10))
	db.mu.Lock()
	db.areas[a].entries[key].markPurged(clock.Now())
	db.mu.Unlock()
	db.deletePurgedIfAcked(a, key)
	_, ok := db.Lookup(a, key)
	return ok
}

// RFC requirement: RFC2328-14-2 negative -- a MaxAge LSA on no retransmission list is not removed while the router's only neighbor is in Exchange, and not while it is in Loading: clause (b) names both states (deletePurgedIfAcked through hasExchangeOrLoadingForKey, flooding.go).
func TestRFC2328MaxAgeKeptWhileNeighborInExchangeOrLoading(t *testing.T) {
	for _, state := range []string{NeighborStateExchange, NeighborStateLoading} {
		t.Run(state, func(t *testing.T) {
			if !maxAgePurgeWith(t, state) {
				t.Fatalf("the MaxAge LSA was deleted while a neighbor is in %s", state)
			}
		})
	}
}

// RFC requirement: RFC2328-14-2 positive -- with the LSA on no retransmission list and the only neighbor in neither Exchange nor Loading (Full, or ExStart before any summary exchange), the MaxAge LSA is removed from the database immediately (deletePurgedIfAcked, flooding.go).
func TestRFC2328MaxAgeRemovedOnceNoNeighborExchangingOrLoading(t *testing.T) {
	for _, state := range []string{NeighborStateFull, "exstart"} {
		t.Run(state, func(t *testing.T) {
			if maxAgePurgeWith(t, state) {
				t.Fatalf("the MaxAge LSA was kept with the only neighbor in %s", state)
			}
		})
	}
}
