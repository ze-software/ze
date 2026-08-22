// Design: docs/architecture/plugin/rib-storage-design.md -- every show walk runs
// beside the writer that removes the routes it reads, and finishes.
//
// The goal of this file is one property, held for all three show pipelines: a
// walk of a peer's RIB does not wedge against a concurrent PeerRIB write. The
// method is a writer goroutine that removes and re-adds the routes under the
// walk, and a deadline around the walk, so the failure this exists to catch
// reports itself as a wedge rather than as a suite timeout.
//
// WHY A DEADLINE AND NOT AN ASSERTION ON CONTENT. The failure is a deadlock, not
// a wrong answer. A walk that asks PeerRIB.IsAddPath from inside a
// PeerRIB.IterateSorted callback takes that type's read lock twice, and a writer
// arriving between the two stops all three goroutines for good: nothing logs,
// nothing times out, and the process has to be killed
// (plan/journal/read-lock-taken-twice-on-one-path.md). What a row carries for a
// route being withdrawn under it is a question the walk answers either way.
//
// Related: rib_pipeline_best_stream_test.go -- the same property for the
// best-path walk, beside the payload and lock-boundary tests that walk owns.
package rib

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// showWalkRows is the seeded table each walk below reads. It is large enough
// that one iteration is worth racing a writer against, and small enough that a
// green run costs milliseconds.
const showWalkRows = 512

// showWalkRepeats is how many times each walk runs beside the writer. One walk
// wedges only if a write lands inside its iteration; repeating the walk turns
// that into a near certainty without making a green run slow.
const showWalkRepeats = 20

// seedShowWalkRIB fills peerRIB with showWalkRows routes and returns the NLRIs,
// so the writer below removes and re-adds exactly what the walk reads.
func seedShowWalkRIB(t testing.TB, peerRIB *storage.PeerRIB) [][]byte {
	t.Helper()

	fam := family.IPv4Unicast
	attrs := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	nlris := make([][]byte, showWalkRows)
	for i := range showWalkRows {
		nlri := []byte{32, 10, byte(i >> 16), byte(i >> 8), byte(i)}
		peerRIB.Insert(fam, attrs, nlri, true)
		nlris[i] = nlri
	}
	return nlris
}

// walkBesideRIBWriter runs walk repeatedly while a goroutine removes and re-adds
// the routes it reads, and fails the test when a walk does not return in time.
//
// The writer is what the UPDATE path does: PeerRIB.Remove and PeerRIB.Insert
// take the storage write lock with no other lock held, because
// handleReceivedStructured gives peerMu back before its phase 2 removes or
// inserts anything (rib_structured.go). So this is the interleaving a withdrawal
// on a peer whose RIB a show command is walking produces in production.
func walkBesideRIBWriter(t *testing.T, peerRIB *storage.PeerRIB, nlris [][]byte, walk func()) {
	t.Helper()

	fam := family.IPv4Unicast
	attrs := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, nlri := range nlris {
				peerRIB.Remove(fam, nlri)
				peerRIB.Insert(fam, attrs, nlri, true)
			}
		}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range showWalkRepeats {
			walk()
		}
	}()

	select {
	case <-done:
	case <-time.After(walkDeadline):
		// The walk goroutine is wedged and stays wedged, so the writer cannot be
		// stopped either. Say what was detected: a package timeout minutes later
		// names nothing, and this is the whole failure mode under test.
		t.Fatalf("a show walk did not finish within %s beside a writer removing "+
			"the routes it reads: the walk is wedged, which is the storage read "+
			"lock taken twice on one path (PeerRIB.IsAddPath)", walkDeadline)
	}

	close(stop)
	writer.Wait()
}

// TestShowPipelineWalkSurvivesConcurrentUpdates runs `show bgp rib` beside the
// writer that removes the routes it reads.
//
// VALIDATES: AC-5 of spec-record-answers-3-zero-alloc -- a RIB dump runs while
// UPDATEs are being processed, and the race detector is clean.
// PREVENTS: inboundSource asking PeerRIB.IsAddPath from inside a
// PeerRIB.IterateSorted callback again. That is one read lock taken twice on one
// path, and a PeerRIB.Remove arriving between the two wedges the walk, the
// writer, and every later reader of that peer's RIB, with nothing logged.
func TestShowPipelineWalkSurvivesConcurrentUpdates(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	nlris := seedShowWalkRIB(t, peerRIB)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	walkBesideRIBWriter(t, peerRIB, nlris, func() {
		status, _, err := r.handleCommand("show bgp rib", "*", nil)
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
	})
}

// TestShowProtocolPipelineWalkSurvivesConcurrentUpdates runs
// `show bgp rib protocol` beside the same writer.
//
// VALIDATES: AC-5, for the protocol-scoped source. A BMP monitored peer's RIB is
// written by the BMP feed and read by this command, on two goroutines.
// PREVENTS: protocolInboundSource asking PeerRIB.IsAddPath from inside a
// PeerRIB.IterateSorted callback again. It is a separate source from
// inboundSource with the same defect and the same fix, so it needs its own
// proof: a guard over one half of a pair is what left this reachable.
func TestShowProtocolPipelineWalkSurvivesConcurrentUpdates(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := storage.NewPeerRIB("router1:peer1")
	nlris := seedShowWalkRIB(t, peerRIB)
	r.ribInPool[bmpProtocolID]["router1:peer1"] = peerRIB

	walkBesideRIBWriter(t, peerRIB, nlris, func() {
		result := r.showProtocolPipeline("bmp", "", nil)
		require.NotNil(t, result)
	})
}
