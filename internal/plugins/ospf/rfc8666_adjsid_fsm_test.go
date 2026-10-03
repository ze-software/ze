// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing Adj-SID lifecycle.
// Related: bfd_client.go -- neighborEventSinkValue, the sink the neighbor table calls.
// Related: sr_adjsid.go -- srAdjNeighborLost, the Adj-SID withdrawal hook.
// Related: rfc5882_bfd_client_test.go -- driveNeighborFull, the DD exchange this file mirrors.
//
// VALIDATES: RFC 8666 Section 8.4.1, "If the adjacency transitions to a state lower than
// 2-Way, then the Adj-SID Advertisement MUST be withdrawn from the area.", through the
// OSPFv3 engine's own neighbor state machine: a Full neighbor whose Hello stops listing this
// router (1-WayReceived, the neighbor falls to Init) has its Adj-SID withdrawn.
// PREVENTS: a withdrawal that only a direct srAdjManager call performs, with the neighbor
// table's down transition never reaching the Adj-SID hook.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// v6AdjHello feeds the engine's neighbor table one Hello from nbr on eth0; twoWay reports
// whether the Hello lists this router.
func v6AdjHello(t *testing.T, eng *engine, self, nbr types.RouterID, twoWay bool) {
	t.Helper()
	if reason := eng.neighbors.Hello(ospfneighbor.HelloInput{
		InterfaceName: "eth0", AreaID: types.BackboneArea, LocalRouterID: self,
		NeighborID: nbr, Address: netip.MustParseAddr("fe80::2"), Priority: 1, TwoWay: twoWay,
		NetworkType: types.NetworkPointToPoint, DeadInterval: 40, InterfaceMTU: 1500, Now: time.Now(),
	}); reason != "" {
		t.Fatalf("Hello (two-way %v): %s", twoWay, reason)
	}
}

// v6AdjDriveFull drives nbr through the DD exchange to Full in the engine's table.
func v6AdjDriveFull(t *testing.T, eng *engine, self, nbr types.RouterID) {
	t.Helper()
	v6AdjHello(t, eng, self, nbr, true)
	if r := eng.neighbors.HandleDBDesc("eth0", nbr, ospfpacket.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags:      ospfpacket.DDFlagInit | ospfpacket.DDFlagMore | ospfpacket.DDFlagMaster,
		DDSequence: 7,
	}); r != "" {
		t.Fatalf("ExStart DD: %s", r)
	}
	snap, ok := eng.neighbors.Lookup("eth0", nbr)
	if !ok {
		t.Fatalf("neighbor %s missing after ExStart", nbr)
	}
	seq := snap.DDSequence
	flags := uint8(0)
	if !snap.Master {
		seq++
		flags = ospfpacket.DDFlagMaster
	}
	if r := eng.neighbors.HandleDBDesc("eth0", nbr, ospfpacket.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE, Flags: flags, DDSequence: seq,
	}); r != "" {
		t.Fatalf("Exchange DD: %s", r)
	}
	waitFor(t, func() bool {
		s, ok := eng.neighbors.Lookup("eth0", nbr)
		return ok && s.State == neighborStateFull
	})
}

// TestRFC8666AdjSIDWithdrawnWhenFSMFallsBelowTwoWay checks the FSM-driven withdrawal.
// Goal: the neighbor state machine's own transition below 2-Way withdraws the Adj-SID.
// Method: an OSPFv3 engine with an Adj-SID manager; the neighbor reaches Full through
// Hello and DD exchange, its Adj-SID is advertised, then a Hello that no longer lists
// this router moves it to Init. The E-Router-LSA body, the origination store and the
// mpls-fib events are read after the transition.
// RFC requirement: RFC8666-8.4.1-1 positive -- when the neighbor table moves a Full
// adjacency to Init (below 2-Way), the Adj-SID leaves the origination store, the
// E-Router-LSA no longer carries it, and its mpls-fib pop entry is removed.
func TestRFC8666AdjSIDWithdrawnWhenFSMFallsBelowTwoWay(t *testing.T) {
	eng := newV6RIEngine(t)
	t.Cleanup(eng.shutdown)
	bus := &srCaptureBus{}
	self := types.RouterID{1, 1, 1, 1}
	nbr := types.RouterID{2, 2, 2, 2}
	m := &srAdjManager{
		alloc:  sr.NewLabelAllocator([]sr.LabelRange{{Base: 40000, Size: 4}}),
		fib:    newSRFIB(bus, mplsSourceOSPFv3SR),
		store:  newSRWireStore(),
		self:   self,
		labels: map[srAdjKey]srAdjRecord{},
	}
	eng.srAdj = m
	eng.neighbors.ConfigureInterface(ospfneighbor.InterfaceConfig{
		Name: "eth0", AreaID: types.BackboneArea, RouterID: self,
		NetworkType: types.NetworkPointToPoint, Options: types.OptionE, DeadInterval: 40, InterfaceMTU: 1500,
	})
	v6AdjDriveFull(t, eng, self, nbr)
	if _, ok := m.adjFor("eth0", nbr); !ok {
		if !m.neighborFull("eth0", nbr, [4]byte(nbr), netip.MustParseAddr("fe80::2"), false, [4]byte(nbr)) {
			t.Fatal("Adj-SID not allocated at Full")
		}
	}
	ifaces := []ospflsdb.InterfaceInfo{{
		Name: "eth0", NetworkType: types.NetworkPointToPoint, InterfaceID: 5, Cost: 10,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: nbr, State: ospflsdb.NeighborStateFull, InterfaceID: 6}},
	}}
	if _, ok := eng.v6BuildERouterBody(ifaces); !ok {
		t.Fatal("the E-Router-LSA must carry the Adj-SID while the adjacency is Full")
	}
	label := m.labels[srAdjKey{iface: "eth0", router: nbr}].label

	v6AdjHello(t, eng, self, nbr, false)

	snap, ok := eng.neighbors.Lookup("eth0", nbr)
	if !ok || snap.State != "init" {
		t.Fatalf("neighbor after a one-way Hello = %+v (found %v), want init", snap, ok)
	}
	if _, ok := m.adjFor("eth0", nbr); ok {
		t.Fatal("the Adj-SID must be withdrawn when the adjacency falls below 2-Way")
	}
	if _, ok := m.store.adjFor(self, [4]byte(nbr)); ok {
		t.Fatal("the Adj-SID must leave the origination store")
	}
	if _, ok := eng.v6BuildERouterBody(ifaces); ok {
		t.Fatal("the E-Router-LSA must no longer advertise the withdrawn Adj-SID")
	}
	removed := false
	for _, entry := range bus.entries {
		if entry.Action == mplsfibevents.ActionRemove && entry.Op == mplsfibevents.OpPop && entry.InLabel == label {
			removed = true
		}
	}
	if !removed {
		t.Fatalf("the Adj-SID pop entry must be withdrawn: %+v", bus.entries)
	}
}
