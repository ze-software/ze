// VALIDATES: spec-ospf-ext-5 AC-12/AC-13, R-4 -- an Adj-SID is allocated from the
// SRLB when a neighbor reaches Full (>= 2-Way), installs a pop/forward entry toward
// that neighbor, and is withdrawn + freed when the neighbor leaves Full.
// PREVENTS: a stale pop entry to a dead neighbor; an SRLB label leak.
package ospf

import (
	"net/netip"
	"sync"
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

type blockingSRAdjBus struct {
	srCaptureBus
	entered chan mplsfibevents.Entry
	release <-chan struct{}
}

func (b *blockingSRAdjBus) Emit(namespace, eventType string, payload any) (int, error) {
	if batch, ok := payload.(*mplsfibevents.EntryBatch); ok && len(batch.Entries) == 1 {
		b.entered <- batch.Entries[0]
		<-b.release
	}
	return b.srCaptureBus.Emit(namespace, eventType, payload)
}

// VALIDATES: IPv4 origination sees no SID during install emission or withdrawal,
// both adjacency readers resolve the completed install, and the withdrawn label
// can then be reused toward a different next hop.
// PREVENTS: advertising an incomplete install or retaining a withdrawn adjacency.
// Concurrent reader/lifecycle access is exercised separately under -race below.
func TestSRAdjSIDPublicationAndReuse(t *testing.T) {
	release := make(chan struct{})
	var workers sync.WaitGroup
	defer func() {
		close(release)
		workers.Wait()
	}()
	bus := &blockingSRAdjBus{entered: make(chan mplsfibevents.Entry, 2), release: release}
	self := types.RouterID{10, 0, 0, 1}
	peer := types.RouterID{10, 0, 0, 2}
	nextPeer := types.RouterID{10, 0, 0, 3}
	linkData := [4]byte{10, 0, 12, 1}
	nextLinkData := [4]byte{10, 0, 13, 1}
	nextHop := netip.MustParseAddr("10.0.12.2")
	nextNextHop := netip.MustParseAddr("10.0.13.2")
	m := &srAdjManager{
		alloc: sr.NewLabelAllocator([]sr.LabelRange{{Base: 40000, Size: 1}}),
		fib:   newSRFIB(bus, mplsSourceOSPFSR), store: newSRWireStore(),
		self: self, labels: map[srAdjKey]srAdjRecord{},
	}
	installed := make(chan bool, 1)
	workers.Go(func() { installed <- m.neighborFull("eth0", peer, linkData, nextHop, false, [4]byte{}) })
	add := <-bus.entered
	if add.Action != mplsfibevents.ActionAdd || add.InLabel != 40000 || add.NextHop != nextHop {
		t.Fatalf("install emission = %+v, want label 40000 toward %v", add, nextHop)
	}
	if _, ok := m.store.adjFor(self, linkData); ok {
		t.Fatal("IPv4 origination can advertise the SID before install emission completes")
	}
	release <- struct{}{}
	if !<-installed {
		t.Fatal("Full did not allocate the sole SRLB label")
	}
	if adj, ok := m.adjFor("eth0", peer); !ok || adj.Label != 40000 {
		t.Fatalf("IPv6 Adj-SID = %+v, present %v, want installed label 40000", adj, ok)
	}
	if label, ok := m.adjLabelForRouter(peer); !ok || label != 40000 {
		t.Fatalf("TI-LFA Adj-SID = %d, present %v, want installed label 40000", label, ok)
	}

	removed := make(chan struct{})
	workers.Go(func() {
		m.neighborLost("eth0", peer)
		close(removed)
	})
	remove := <-bus.entered
	if remove.Action != mplsfibevents.ActionRemove || remove.InLabel != 40000 {
		t.Fatalf("withdrawal emission = %+v, want removal of label 40000", remove)
	}
	if _, ok := m.store.adjFor(self, linkData); ok {
		t.Fatal("IPv4 origination still advertises the SID during withdrawal")
	}
	release <- struct{}{}
	<-removed
	if _, ok := m.adjFor("eth0", peer); ok {
		t.Fatal("the withdrawn adjacency remains available to IPv6 origination")
	}
	if _, ok := m.adjLabelForRouter(peer); ok {
		t.Fatal("the withdrawn adjacency remains available to TI-LFA")
	}
	workers.Go(func() {
		installed <- m.neighborFull("eth1", nextPeer, nextLinkData, nextNextHop, false, [4]byte{})
	})
	reused := <-bus.entered
	if reused.Action != mplsfibevents.ActionAdd || reused.InLabel != 40000 || reused.NextHop != nextNextHop {
		t.Fatalf("reused label emission = %+v, want label 40000 toward %v", reused, nextNextHop)
	}
	if _, ok := m.store.adjFor(self, nextLinkData); ok {
		t.Fatal("reused SID is advertised before its new install emission completes")
	}
	release <- struct{}{}
	if !<-installed {
		t.Fatal("the withdrawn SRLB label was not available to the next adjacency")
	}
	if adj, ok := m.adjFor("eth1", nextPeer); !ok || adj.Label != 40000 {
		t.Fatalf("new adjacency SID = %+v, present %v, want label 40000", adj, ok)
	}
}

// VALIDATES: concurrent neighbor changes, origination, TI-LFA and metric reads
// preserve each live adjacency and remove it at withdrawal.
// PREVENTS: unsynchronized access to the manager's label map and allocator.
// Run with -race to detect unsafe overlap; no particular scheduling order is assumed.
func TestSRAdjSIDConcurrentReadersAndLifecycle(t *testing.T) {
	m := &srAdjManager{
		alloc: sr.NewLabelAllocator([]sr.LabelRange{{Base: 40000, Size: 4}}),
		fib:   newSRFIB(nil, mplsSourceOSPFSR), store: newSRWireStore(),
		self: types.RouterID{10, 0, 0, 1}, labels: map[srAdjKey]srAdjRecord{},
	}
	peers := []struct {
		name     string
		router   types.RouterID
		linkData [4]byte
		nextHop  netip.Addr
	}{
		{"eth0", types.RouterID{10, 0, 0, 2}, [4]byte{10, 0, 12, 1}, netip.MustParseAddr("10.0.12.2")},
		{"eth1", types.RouterID{10, 0, 0, 3}, [4]byte{10, 0, 13, 1}, netip.MustParseAddr("10.0.13.2")},
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, peer := range peers {
		workers.Go(func() {
			<-start
			for range 100 {
				if !m.neighborFull(peer.name, peer.router, peer.linkData, peer.nextHop, false, [4]byte{}) {
					t.Errorf("%s did not allocate an Adj-SID with capacity for both peers", peer.name)
					return
				}
				adj, ok := m.adjFor(peer.name, peer.router)
				if !ok || adj.Label < 40000 || adj.Label > 40003 {
					t.Errorf("%s live Adj-SID = %+v, present %v", peer.name, adj, ok)
				}
				if label, found := m.adjLabelForRouter(peer.router); !found || label != adj.Label {
					t.Errorf("%s TI-LFA label = %d, present %v, want live label %d", peer.name, label, found, adj.Label)
				}
				m.neighborLost(peer.name, peer.router)
				if _, found := m.adjFor(peer.name, peer.router); found {
					t.Errorf("%s remains advertised after withdrawal", peer.name)
				}
			}
		})
	}
	for range 2 {
		workers.Go(func() {
			<-start
			for range 1000 {
				for _, peer := range peers {
					if adj, ok := m.adjFor(peer.name, peer.router); ok && (adj.Label < 40000 || adj.Label > 40003) {
						t.Errorf("%s concurrent origination read = %+v, outside the SRLB", peer.name, adj)
					}
					if label, ok := m.adjLabelForRouter(peer.router); ok && (label < 40000 || label > 40003) {
						t.Errorf("%s concurrent TI-LFA label = %d, outside the SRLB", peer.name, label)
					}
				}
				if count := m.inUse(); count < 0 || count > len(peers) {
					t.Errorf("allocated adjacency count = %d, want 0..%d", count, len(peers))
				}
			}
		})
	}
	close(start)
	workers.Wait()
	if count := m.inUse(); count != 0 {
		t.Fatalf("allocated adjacency count after all withdrawals = %d, want 0", count)
	}
}
