// VALIDATES: spec-ospf-ext-5 AC-12/AC-13, R-4 -- an Adj-SID is allocated from the
// SRLB when a neighbor reaches Full (>= 2-Way), installs a pop/forward entry toward
// that neighbor, and is withdrawn + freed when the neighbor leaves Full.
// PREVENTS: a stale pop entry to a dead neighbor; an SRLB label leak.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func newTestAdjManager(bus *srCaptureBus) (*srAdjManager, *srWireStore) {
	store := newSRWireStore()
	m := &srAdjManager{
		alloc:  sr.NewLabelAllocator([]sr.LabelRange{{Base: 40000, Size: 4}}),
		fib:    newSRFIB(bus, mplsSourceOSPFSR),
		store:  store,
		self:   types.RouterID{10, 0, 0, 1},
		labels: map[srAdjKey]srAdjRecord{},
	}
	return m, store
}

func TestSRAdjSIDAllocatedAtFull(t *testing.T) {
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	nbr := types.RouterID{10, 0, 0, 2}
	nh := netip.MustParseAddr("10.0.12.2")
	linkData := [4]byte{10, 0, 12, 1}
	if !m.neighborFull("eth0", nbr, linkData, nh, false, [4]byte{}) {
		t.Fatalf("neighborFull should allocate an Adj-SID")
	}
	// A pop/forward entry is installed toward the neighbor.
	e := bus.entries[len(bus.entries)-1]
	if e.Op != mplsfibevents.OpPop || e.NextHop != nh || e.InLabel < 40000 || e.InLabel > 40003 {
		t.Fatalf("adj-SID pop entry wrong: %+v", e)
	}
	// The Adj-SID is stored (so the Extended Link LSA can advertise it).
	if _, ok := store.adjFor(m.self, linkData); !ok {
		t.Fatalf("adj-SID must be stored under the link data")
	}
	// Idempotent: a repeat Full for the same neighbor allocates nothing new.
	before := m.alloc.InUse()
	m.neighborFull("eth0", nbr, linkData, nh, false, [4]byte{})
	if m.alloc.InUse() != before {
		t.Fatalf("repeat Full must not allocate a second Adj-SID")
	}
}

func TestSRAdjSIDWithdrawnBelow2Way(t *testing.T) {
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	nbr := types.RouterID{10, 0, 0, 2}
	nh := netip.MustParseAddr("10.0.12.2")
	linkData := [4]byte{10, 0, 12, 1}
	m.neighborFull("eth0", nbr, linkData, nh, false, [4]byte{})
	label, _ := store.adjFor(m.self, linkData)
	before := len(bus.entries)

	m.neighborLost("eth0", nbr)
	// The pop entry is removed and the SRLB label freed.
	var sawRemove bool
	for _, e := range bus.entries[before:] {
		if e.Action == mplsfibevents.ActionRemove && e.Op == mplsfibevents.OpPop && e.InLabel == label.Label {
			sawRemove = true
		}
	}
	if !sawRemove {
		t.Fatalf("Adj-SID pop must be removed on leaving Full")
	}
	if _, ok := store.adjFor(m.self, linkData); ok {
		t.Fatalf("Adj-SID store entry must be cleared")
	}
	if m.alloc.InUse() != 0 {
		t.Fatalf("SRLB label must be freed, InUse=%d", m.alloc.InUse())
	}
}

func TestSRLANAdjSIDCarriesNeighborID(t *testing.T) {
	bus := &srCaptureBus{}
	m, store := newTestAdjManager(bus)
	nbr := types.RouterID{10, 0, 0, 2}
	nh := netip.MustParseAddr("10.0.12.2")
	linkData := [4]byte{10, 0, 12, 1}
	m.neighborFull("eth0", nbr, linkData, nh, true, nbr)
	adj, ok := store.adjFor(m.self, linkData)
	if !ok || !adj.IsLAN || adj.NeighborID != nbr {
		t.Fatalf("LAN Adj-SID must carry the neighbor ID: %+v", adj)
	}
}

func TestSRAdjManagerNilAllocator(t *testing.T) {
	// SR disabled (nil allocator): no allocation, no panic.
	m := &srAdjManager{fib: newSRFIB(&srCaptureBus{}, mplsSourceOSPFSR), store: newSRWireStore(), labels: map[srAdjKey]srAdjRecord{}}
	if m.neighborFull("eth0", types.RouterID{10, 0, 0, 2}, [4]byte{}, netip.MustParseAddr("10.0.12.2"), false, [4]byte{}) {
		t.Fatalf("nil allocator must not allocate")
	}
	m.neighborLost("eth0", types.RouterID{10, 0, 0, 2}) // must not panic
}
