package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
)

// sameBestAllocsMax is what a CIDR election whose best did not change costs:
// one Candidate for the one stored path, and the slice that holds it. It is
// the count before the per-prefix election landed (e059baccb8^), which nothing
// in the election's own key or winner reads may add to.
const sameBestAllocsMax = 2

// TestCIDRSameBestAllocations pins the allocations of the hot path most UPDATEs
// take: a CIDR prefix re-elected to the best it already had.
//
// VALIDATES: a re-run of checkBestPathChange over one stored path allocates no
// more than sameBestAllocsMax per call, for a session without ADD-PATH and for
// one with it, whose path is read back by prefix and path identifier.
// PREVENTS: a stack buffer of the election (the route-key scratch, the winner's
// or a sibling's key) reaching an indirect call, which moves it to the heap on
// every UPDATE. `go test -gcflags=-m` names the buffer that moved.
func TestCIDRSameBestAllocations(t *testing.T) {
	t.Run("without-add-path", func(t *testing.T) {
		r := newTestRIBManagerWithBus(newTestEventBus())
		peer := netip.MustParseAddr("192.0.2.1")
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65000, LocalASN: 65000}
		rib := storage.NewPeerRIB(peer.String())
		r.bgpPeers[peer] = rib
		key := []byte{8, 10}
		rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), key)
		if _, changed := r.checkBestPathChange(famV4, key, false, nil); !changed {
			t.Fatal("the first election published no best")
		}
		allocations := testing.AllocsPerRun(1000, func() {
			r.checkBestPathChange(famV4, key, false, nil)
		})
		if allocations > sameBestAllocsMax {
			t.Fatalf("same-best election allocated %v times per call, want at most %d", allocations, sameBestAllocsMax)
		}
	})
	t.Run("add-path", func(t *testing.T) {
		r := newTestRIBManagerWithBus(newTestEventBus())
		peer := netip.MustParseAddr("192.0.2.31")
		rib := newAddPathPeer(r, peer)
		key := addPathKey(7)
		rib.Insert(famV4, unicastAttrs([4]byte{10, 0, 0, 1}, 10, 100), key)
		if _, changed := r.checkBestPathChange(famV4, key, true, nil); !changed {
			t.Fatal("the first election published no best")
		}
		allocations := testing.AllocsPerRun(1000, func() {
			r.checkBestPathChange(famV4, key, true, nil)
		})
		if allocations > sameBestAllocsMax {
			t.Fatalf("same-best election allocated %v times per call, want at most %d", allocations, sameBestAllocsMax)
		}
	})
}
