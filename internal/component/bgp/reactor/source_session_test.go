// Design: docs/architecture/api/process-protocol.md -- original source-session authority.
// Related: source_session.go -- capture, callback and ordered release.
package reactor

import (
	"math"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourceSessionFixture installs an exact established session without opening TCP.
// Its context and LLGR facts differ from any successor the test may install.
func sourceSessionFixture(t *testing.T, peer *Peer) *Session {
	t.Helper()
	peer.state.Store(int32(PeerStateEstablished))
	peer.advanceForwardGeneration()
	session := peer.newSession()
	contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	session.setRecvCtxID(contextID)
	session.localLLGR.Store(true)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	return session
}

// TestSourceSessionCaptureUsesExactPeer proves a sibling at the same IP cannot
// supply source facts, and reconnect cannot replace the first session's receipt.
func TestSourceSessionCaptureUsesExactPeer(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.87")
	peer := NewPeer(&PeerSettings{Address: address, Port: 1179, LocalAS: 65000, PeerAS: 65001, Name: "original"})
	peer.remoteRouterID.Store(71)
	session := sourceSessionFixture(t, peer)
	var store sourceReceiptStore
	require.NoError(t, session.captureSourceReceipt(peer, &store))
	t.Cleanup(session.closeSourceReceipt)
	original := session.sourceReceiptID()
	require.NotZero(t, original)
	require.NoError(t, session.captureSourceReceipt(peer, &store), "one session keeps one receipt")
	require.Equal(t, original, session.sourceReceiptID())
	facts := store.entries[original].facts.source
	require.Same(t, peer, facts.info.peer)
	require.Equal(t, "original", facts.info.filterInfo.Name)
	require.Equal(t, uint32(65001), facts.info.filterInfo.PeerAS)
	require.True(t, facts.info.filterInfo.LLGRAdvertised)

	replacement := sourceSessionFixture(t, peer)
	require.NoError(t, replacement.captureSourceReceipt(peer, &store))
	t.Cleanup(replacement.closeSourceReceipt)
	require.NotEqual(t, original, replacement.sourceReceiptID())
	require.Equal(t, original, session.sourceReceiptID())
	session.closeSourceReceipt()
	require.Contains(t, store.entries, replacement.sourceReceiptID(), "old close cannot release replacement")
}

// TestSourceSessionCallbackCarriesOriginalReceipt checks the real factory
// callback, notifier and cache keep the issued identity on received UPDATEs.
func TestSourceSessionCallbackCarriesOriginalReceipt(t *testing.T) {
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	address := netip.MustParseAddr("192.0.2.87")
	settings := &PeerSettings{Address: address, Port: 1179, LocalAS: 65000, PeerAS: 65001, Name: "original"}
	acceptingPolicies(t, settings)
	require.NoError(t, reactor.AddPeer(settings))
	siblingSettings := &PeerSettings{Address: address, Port: 2179, LocalAS: 65000, PeerAS: 65002, Name: "sibling"}
	acceptingPolicies(t, siblingSettings)
	require.NoError(t, reactor.AddPeer(siblingSettings))
	peer := reactor.peers[settings.PeerKey()]
	var observed bgptypes.RawMessage
	var observedPeer plugin.PeerInfo
	reactor.setMessageReceiver(&testDeliveryReceiver{consumerCount: 1, onReceived: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
		observedPeer, observed = peer, msg
	}})
	session := sourceSessionFixture(t, peer)
	var store sourceReceiptStore
	require.NoError(t, session.captureSourceReceipt(peer, &store))
	t.Cleanup(session.closeSourceReceipt)
	body := []byte{0, 0, 0, 0} // A genuine empty UPDATE exercises event and cache without prefix validation.
	update := wireu.NewWireUpdate(body, session.recvCtxID)
	kept := session.onMessageReceived(address, msgtype.TypeUPDATE, body, update, session.recvCtxID,
		rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
	require.True(t, kept)
	require.Equal(t, "original", observedPeer.Name)
	require.Equal(t, session.sourceReceiptID(), observed.SourceReceipt)
	received, found := reactor.recentUpdates.Get(observed.MessageID)
	require.True(t, found)
	require.NotNil(t, received)
	require.Equal(t, observed.SourceReceipt, received.SourceReceipt)
	require.Equal(t, observed.SourceReceipt, received.WireUpdate.SourceReceipt())
	reactor.recentUpdates.abortPublication(observed.MessageID)
}

type sourceSessionDownObserver struct {
	owner plugin.SelectedOwner
	id    uint64
	lease chan *sourceReceiptLease
	err   chan error
}

func (*sourceSessionDownObserver) OnPeerEstablished(*Peer) {}

func (observer *sourceSessionDownObserver) OnPeerClosed(*Peer, sessionClose) {
	lease, err := sourceReceipts.acquire(observer.id, observer.owner)
	if err == nil {
		err = lease.retain()
	}
	observer.lease <- lease
	observer.err <- err
}

// TestSourceSessionDownKeepsQueuedAdmission proves the real connection's
// receipt stays open through receive delivery and the synchronous DOWN handler.
// A retained lease survives physical closure, while its delivery reference ends.
func TestSourceSessionDownKeepsQueuedAdmission(t *testing.T) {
	c := newSessionEndCase(t, acceptingLiveSettings(t, "10.0.0.1", 65000, 65002))
	id := c.session.sourceReceiptID()
	require.NotZero(t, id, "the session has authority before its published UP returns")
	owner, _ := sourceReceiptOwner(t)
	observer := &sourceSessionDownObserver{owner: owner, id: id,
		lease: make(chan *sourceReceiptLease, 1), err: make(chan error, 1)}
	c.r.addPeerObserver(observer)
	c.sendHeld(t, 1, false)
	require.NoError(t, c.v.remote.Close())
	lowEventually(t, func() bool { return c.p.State() != PeerStateEstablished }, "physical session ended")
	sourceReceipts.mu.Lock()
	open := sourceReceipts.entries[id] != nil && sourceReceipts.entries[id].deliveryOpen
	sourceReceipts.mu.Unlock()
	require.True(t, open, "queued receive still owns admission")
	c.releaseReceiver()
	select {
	case err := <-observer.err:
		require.NoError(t, err, "the ordered DOWN handler can retain the original source")
	case <-time.After(5 * time.Second):
		t.Fatal("ordered source DOWN did not finish")
	}
	lease := <-observer.lease
	t.Cleanup(func() { require.NoError(t, lease.revoke(t.Context())) })
	lowEventually(t, func() bool {
		sourceReceipts.mu.Lock()
		defer sourceReceipts.mu.Unlock()
		return !sourceReceipts.entries[id].deliveryOpen
	}, "DOWN completion releases session delivery")
	write, err := lease.begin(c.p.sourcePolicyEpoch.Load())
	require.NoError(t, err, "retained authority survives the physical DOWN")
	write.complete()
}

// TestSourceSessionMissingFactsAndExhaustionRefuse proves refusal never emits a
// zero-authority session and the live doctor check names daemon-wide exhaustion.
func TestSourceSessionMissingFactsAndExhaustionRefuse(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	session := peer.newSession()
	var store sourceReceiptStore
	require.Error(t, session.captureSourceReceipt(peer, &store), "unrecorded receive encoding grants nothing")
	require.Zero(t, session.sourceReceiptID())
	require.Empty(t, store.entries)
	store.next = math.MaxUint64
	_, err := store.issue(facts)
	require.ErrorContains(t, err, "restart Ze")
	finding := store.sourceReceiptDiagnostics()
	require.Len(t, finding, 1)
	require.Equal(t, codeSourceReceiptAdmission, finding[0].Code)
	require.Contains(t, finding[0].Fix, "restart")
	var healthy sourceReceiptStore
	require.Empty(t, healthy.sourceReceiptDiagnostics())
}
