// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing Adj-SID lifecycle.
// Related: bfd_client.go -- neighborEventSinkValue, the neighbor FSM sink this test drives.
// Related: sr_adjsid.go -- srAdjNeighborLost, the hook the sink calls on leaving Full.
//
// VALIDATES: RFC 8665 section 7.4.1 through the neighbor state machine's own sink: a neighbor
// that leaves Full has its Adj-SID withdrawn and a self-LSA re-origination queued.
// PREVENTS: an Adj-SID withdrawal that only a direct srAdjManager call performs, with the
// production neighbor-down path leaving it advertised.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC8665-7.4.1-1 positive -- a neighbor that holds an Adj-SID and leaves
// Full through the engine's neighbor sink (neighborEventSinkValue().NeighborDown) has its
// Adj-SID removed from the store the Extended Link LSA reads, its pop entry removed and its
// SRLB label freed, and a self-LSA re-origination is queued to carry the withdrawal.
func TestRFC8665AdjSIDWithdrawnThroughNeighborSink(t *testing.T) {
	// Goal: the production neighbor-down path reaches the withdrawal. Method: install the
	// Adj-SID through the manager, then report the neighbor down through the sink the FSM
	// calls. The neighbor table is detached so the queued re-origination stays observable.
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	nbr := types.RouterID{10, 0, 0, 2}
	linkData := [4]byte{10, 0, 12, 1}
	m.neighborFull("eth0", nbr, linkData, netip.MustParseAddr("10.0.12.2"), false, [4]byte{})
	adj, ok := store.adjFor(m.self, linkData)
	if !ok {
		t.Fatal("Adj-SID not advertised while the adjacency is up")
	}
	eng := newEngine(nil)
	eng.srAdj = m
	eng.neighbors = nil
	before := len(bus.entries)

	eng.neighborEventSinkValue().NeighborDown(ospfneighbor.Snapshot{Interface: "eth0", RouterID: "10.0.0.2"})

	if _, still := store.adjFor(m.self, linkData); still {
		t.Fatal("the Adj-SID advertisement was not withdrawn after the neighbor left Full")
	}
	removed := false
	for _, entry := range bus.entries[before:] {
		if entry.Action == mplsfibevents.ActionRemove && entry.Op == mplsfibevents.OpPop && entry.InLabel == adj.Label {
			removed = true
		}
	}
	if !removed {
		t.Fatalf("the Adj-SID pop entry was not removed: %+v", bus.entries[before:])
	}
	if m.alloc.InUse() != 0 {
		t.Fatalf("the SRLB label was not freed, InUse=%d", m.alloc.InUse())
	}
	if len(eng.originationNotify) != 1 {
		t.Fatal("no self-LSA re-origination queued after the Adj-SID withdrawal")
	}
}
