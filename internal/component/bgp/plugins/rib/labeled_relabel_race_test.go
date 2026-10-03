package rib

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// relabelRounds is how many relabels the writer performs while the reader
// watches. A replacement that unbinds before it rebinds leaves the window open
// for one instruction sequence per round, so the count buys the reader enough
// chances to land in it.
const relabelRounds = 20000

// TestADDPathRelabelNeverUnbindsTheLabel proves that a labeled unicast path
// replaced under ADD-PATH keeps a label bound at every instant a concurrent
// election can read it.
//
// VALIDATES: while one goroutine relabels path 7 of 10.0.0.0/24 by the ingest
// sequence (Insert, then SetLabelsIfRouteExists, as insertLabeled runs it),
// another reading the path's labels the way the election does
// (PeerRIB.LookupLabels) never sees InvalidHandle once the path was bound.
// PREVENTS: the replacement releasing the old label inside Insert's lock and
// binding the new one under a second lock, between which an election installs
// the route with no label stack.
func TestADDPathRelabelNeverUnbindsTheLabel(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	peer := netip.MustParseAddr("192.0.2.41")
	peerRIB := storage.NewPeerRIB(peer.String())
	peerRIB.SetAddPath(fam, true)
	nlri := []byte{0, 0, 0, 7, 24, 10, 0, 0}
	// Each round changes the next hop as well as the label, so the UPDATE
	// replaces the route rather than refreshing it: a refresh keeps the binding.
	attrs := [2][]byte{makeAttrBytes([4]byte{192, 0, 2, 41}), makeAttrBytes([4]byte{192, 0, 2, 42})}

	bind := func(round int) {
		peerRIB.Insert(fam, attrs[round%2], nlri)
		label := uint32(100 + round%2)
		h := pool.InternLabels([]uint32{label})
		if !peerRIB.SetLabelsIfRouteExists(fam, nlri, h) {
			_ = pool.Labels.Release(h)
			t.Error("the path was not stored when its labels were bound")
		}
	}
	bind(0)
	t.Cleanup(peerRIB.Release)

	stop := make(chan struct{})
	var unbound int
	var wg sync.WaitGroup
	wg.Add(1)
	go watchPathLabels(&wg, stop, peerRIB, fam, nlri, &unbound)
	for round := 1; round <= relabelRounds; round++ {
		bind(round)
	}
	close(stop)
	wg.Wait()
	if unbound > 0 {
		t.Fatalf("a concurrent reader saw path 7 with no label %d times during %d relabels", unbound, relabelRounds)
	}
}

// watchPathLabels reads path 7's label handle until stop closes, counting the
// reads that found none. MUST be started with wg.Add(1); it calls wg.Done.
func watchPathLabels(wg *sync.WaitGroup, stop <-chan struct{}, peerRIB *storage.PeerRIB, fam family.Family, nlri []byte, unbound *int) {
	defer wg.Done()
	for {
		select {
		case <-stop:
			return
		default:
		}
		if !peerRIB.LookupLabels(fam, nlri).IsValid() {
			*unbound++
		}
	}
}
