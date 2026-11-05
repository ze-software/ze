// Design: server_forward.go -- the rail that serves what the reactor fast path did not.

package rs

import (
	"slices"
	"sync"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// fastPathServedAnnounce is an ipv4/unicast announce of 10.0.0.0/24 the
// reactor fast path claims it forwarded, handing no peer back.
func fastPathServedAnnounce(t *testing.T) *bgptypes.RawMessage {
	t.Helper()
	// ORIGIN IGP, AS_PATH empty, NEXT_HOP 1.1.1.1; NLRI 10.0.0.0/24.
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 1, 1, 1, 1}
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	body = append(body, 24, 10, 0, 0)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	return &bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu, ReactorForwarded: true}
}

// TestFastPathServedUpdateReachesAPeerEstablishedAfterIt proves that an
// UPDATE the reactor fast path claims, numbered below a target's
// initial-replay receipt and above its peer-up cut, is forwarded to that
// target by this plugin.
//
// VALIDATES: batchForwardUpdateSkipped selects, besides the handed-back
// peers, every up peer whose receipt (PeerState.InitialReplay) is above the
// UPDATE's MessageID, under the same cut as every other forward; a peer whose
// receipt is at or below the MessageID is left to the fast path.
//
// PREVENTS: the lost route under stress (bgp-rs-opaque-transparency.ci). The
// fast path ran before the target's session was up, so it neither served the
// target nor handed it back. This plugin then saw the peer up before it took
// delivery of the UPDATE, so the cut put the UPDATE on the live rail, and the
// live rail trusted the fast path's claim: the target received its End-of-RIB
// and never the route (RFC 4724 Section 2).
//
// Method: the peers are installed directly and the work item is processed
// on the test goroutine, so the interleaving is fixed with no timing.
func TestFastPathServedUpdateReachesAPeerEstablishedAfterIt(t *testing.T) {
	rs := newTestRouteServer(t)
	var mu sync.Mutex
	var forwarded [][]string
	rs.forwardCachedHook = func(_ []uint64, targets []string) {
		mu.Lock()
		forwarded = append(forwarded, slices.Clone(targets))
		mu.Unlock()
	}

	rs.mu.Lock()
	rs.peers["10.0.0.1"] = &PeerState{Address: "10.0.0.1", Up: true, StateSeen: true}
	// Up after the UPDATE was numbered (receipt 50 > 40), cut below it.
	rs.peers["10.0.0.2"] = &PeerState{Address: "10.0.0.2", Up: true, StateSeen: true, ForwardFrom: 10, InitialReplay: 50}
	// Up before the UPDATE was numbered: the fast path served it.
	rs.peers["10.0.0.3"] = &PeerState{Address: "10.0.0.3", Up: true, StateSeen: true, ForwardFrom: 10, InitialReplay: 20}
	// Up after the UPDATE, but its cut covers it: the replay carries it.
	rs.peers["10.0.0.4"] = &PeerState{Address: "10.0.0.4", Up: true, StateSeen: true, ForwardFrom: 45, InitialReplay: 60}
	rs.mu.Unlock()

	key := workerKey{sourcePeer: "10.0.0.1"}
	rs.processForward(key, workItem{msgID: 40, sourcePeer: "10.0.0.1", msg: fastPathServedAnnounce(t), reactorForwarded: true})
	rs.flushWorkerBatch(key)

	mu.Lock()
	defer mu.Unlock()
	if len(forwarded) != 1 {
		t.Fatalf("forward-cached calls = %v, want one call to 10.0.0.2", forwarded)
	}
	if !slices.Equal(forwarded[0], []string{"10.0.0.2"}) {
		t.Fatalf("forward-cached targets = %v, want [10.0.0.2]", forwarded[0])
	}
}
