// Design: docs/architecture/l2tp/followup-l2tp-call.md -- exact relay-call ownership
// Related: relay_sink.go -- Cancel; teardown.go -- teardownRelaySession

package l2tp

import (
	"errors"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

// A remote CDN ends the old relay. A later Cancel from its access admission
// must leave a replacement with the same numeric tunnel/session pair alive.
func TestRelayCancelAfterRemoteEndKeepsReplacement(t *testing.T) {
	reactor, tunnel, old, call := relayLifetimeFixture(t)
	require.True(t, call.markBridged(100, old.localSID))
	// RFC 2661 Section 6.12: receive the remote's real CDN before reusing SID.
	require.Empty(t, tunnel.handleCDN(old.localSID, buildCDN(1, old.remoteSID), reactor.logger))
	replacement := &L2TPSession{localSID: old.localSID, remoteSID: 72, state: L2TPSessionEstablished}
	tunnel.addSession(replacement)
	before := tunnel.engine.Stats()
	call.Cancel()
	require.Same(t, replacement, tunnel.lookupSession(replacement.localSID), "an ended call's Cancel must not remove its replacement")
	require.Equal(t, before, tunnel.engine.Stats(), "an ended call must not enqueue a CDN for a replacement")
}

// An ended check before locking the reactor is insufficient. Pause the old
// Cancel at its actual teardown hook, remove/reuse the pair under tunnelsMu,
// then let that old teardown acquire the lock and verify the current owner.
func TestRelayCancelWaitingOnReactorKeepsReplacement(t *testing.T) {
	reactor, tunnel, old, call := relayLifetimeFixture(t)
	require.True(t, call.markBridged(100, old.localSID))
	entered := make(chan struct{})
	finished := make(chan struct{})
	call.teardown = func(expected *relayCall, tid, sid uint16) {
		close(entered)
		reactor.teardownRelaySession(expected, tid, sid)
	}
	reactor.tunnelsMu.Lock()
	locked := true
	t.Cleanup(func() {
		if locked {
			reactor.tunnelsMu.Unlock()
		}
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("old cancellation did not finish after reactor release")
		}
	})
	go func() {
		defer close(finished)
		call.Cancel()
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("old cancellation did not reach its reactor teardown")
	}
	// RFC 2661 Section 6.12: the reactor processes the peer's CDN first.
	require.Empty(t, tunnel.handleCDN(old.localSID, buildCDN(1, old.remoteSID), reactor.logger))
	replacement := &L2TPSession{localSID: old.localSID, remoteSID: 72, state: L2TPSessionEstablished}
	tunnel.addSession(replacement)
	before := tunnel.engine.Stats()
	reactor.tunnelsMu.Unlock()
	locked = false
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("old cancellation did not return")
	}
	require.Same(t, replacement, tunnel.lookupSession(replacement.localSID), "a waiting old cancellation must compare the exact relay owner")
	require.Equal(t, before, tunnel.engine.Stats(), "a waiting old cancellation must not enqueue a replacement CDN")
}

// The fixture owns the reactor maps synchronously; the only concurrent actor
// a test starts is its one explicitly joined cancellation. Its UDP peer lets
// the real teardown send a CDN if the exact-owner guard is absent.
func relayLifetimeFixture(t *testing.T) (*l2tpReactor, *L2TPTunnel, *L2TPSession, *relayCall) {
	t.Helper()
	reactor := newReactorForSnapshot(t)
	peer := newPPPRejectionPeer(t, reactor)
	address, ok := peer.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	call := &relayCall{realm: &relayRealmDetails{name: "example.net"}, handoff: &fakeRealmHandoff{}, teardown: reactor.teardownRelaySession}
	old := &L2TPSession{localSID: 41, remoteSID: 42, state: L2TPSessionEstablished, relay: call}
	other := &L2TPSession{localSID: 51, remoteSID: 52, state: L2TPSessionEstablished}
	insertEstablishedTunnel(t, reactor, 100, 200, address.AddrPort(), old, other)
	tunnel := reactor.tunnelsByLocalID[100]
	t.Cleanup(func() { tunnel.engine.Close(reactor.params.Clock()) })
	return reactor, tunnel, old, call
}

// A service handoff carries no proxy PPP methods. The actual service sink must
// still bind cancellation to the call before any ICRP can establish it.
func TestServiceRelayCancelBeforeBridgeSendsCDN(t *testing.T) {
	f := newServiceLifetimeFixture(t)
	f.call.Cancel()
	f.reactor.tunnelsMu.Lock()
	// RFC 2661 Sections 10.1 and 6.12: incoming-call reply and disconnection.
	outgoing := f.tunnel.handleICRP(f.session, icrpBody(707), f.reactor.params.Clock(), f.reactor.logger)
	survivor := f.tunnel.lookupSession(51)
	f.reactor.tunnelsMu.Unlock()
	require.Len(t, outgoing, 1, "canceled service call must send CDN rather than ICCN")
	requireCDN(t, outgoing[0].bytes, cdnResultAdministrative, f.session.localSID)
	require.NotNil(t, survivor, "an unrelated call remains admitted")
	require.Equal(t, 1, f.handoff.count())
	require.False(t, f.call.markBridged(100, f.session.localSID))
}

// Once bridged, access departure must send a real CDN and end exactly the
// service's call. A second Cancel cannot remove another admitted session.
func TestServiceRelayCancelAfterBridgeSendsCDN(t *testing.T) {
	f := newServiceLifetimeFixture(t)
	f.establish(t)
	f.call.Cancel()
	wire := readUntilMsgType(t, f.peer, MsgCDN, 4)
	requireCDN(t, wire, cdnResultLossOfCarrier, f.session.localSID)
	header, err := ParseMessageHeader(wire)
	require.NoError(t, err)
	require.Equal(t, uint16(707), header.SessionID)
	f.reactor.tunnelsMu.Lock()
	removed, sibling := f.tunnel.lookupSession(f.session.localSID), f.tunnel.lookupSession(51)
	before := f.tunnel.engine.Stats()
	f.reactor.tunnelsMu.Unlock()
	require.Nil(t, removed)
	require.NotNil(t, sibling, "another subscriber survives cancellation")
	require.Equal(t, 1, f.handoff.count())
	f.call.Cancel()
	f.reactor.tunnelsMu.Lock()
	after := f.tunnel.engine.Stats()
	f.reactor.tunnelsMu.Unlock()
	require.Equal(t, before, after, "repeated Cancel must not queue another CDN")
}

// A worker refusal has no local PPP session to emit a teardown event. Its
// actual reactor consumer must notify the service access owner directly.
func TestServiceRelayWorkerFailureEndsAccess(t *testing.T) {
	f := newServiceLifetimeFixture(t)
	f.reactor.tunnelsMu.Lock()
	f.session.state = L2TPSessionEstablished
	f.session.remoteSID = 707
	f.reactor.tunnelsMu.Unlock()
	failure := errors.New("service bridge setup failed")
	f.reactor.handleKernelError(kernelSetupFailed{localTID: 100, localSID: f.session.localSID, relay: f.call, err: failure})
	f.reactor.tunnelsMu.Lock()
	removed, sibling := f.tunnel.lookupSession(f.session.localSID), f.tunnel.lookupSession(51)
	f.reactor.tunnelsMu.Unlock()
	require.Nil(t, removed)
	require.NotNil(t, sibling)
	require.Equal(t, 1, f.handoff.count(), "worker failure must end the accepted service admission")
}

// A remote CDN must release the accepted service admission once. This drives
// the same receive handler as a peer datagram, without a timing-based oracle.
func TestServiceRelayRemoteCDNEndsAccess(t *testing.T) {
	f := newServiceLifetimeFixture(t)
	f.establish(t)
	f.reactor.tunnelsMu.Lock()
	// RFC 2661 Section 6.12: peer CDN ends this one call, never its sibling.
	outgoing := f.tunnel.handleCDN(f.session.localSID, buildCDN(1, 707), f.reactor.logger)
	removed, sibling := f.tunnel.lookupSession(f.session.localSID), f.tunnel.lookupSession(51)
	f.reactor.tunnelsMu.Unlock()
	require.Empty(t, outgoing)
	require.Nil(t, removed)
	require.NotNil(t, sibling)
	require.Equal(t, 1, f.handoff.count())
	f.call.Cancel()
	require.Equal(t, 1, f.handoff.count(), "remote end and later access departure report one end")
}

// A queued worker failure is permission to remove only the relay that owned
// its setup. Check stale refusal and a real current-owner refusal separately.
func TestRelayKernelFailureRequiresExactOwner(t *testing.T) {
	reactor, tunnel, old, stale := relayLifetimeFixture(t)
	reactor.tunnelsMu.Lock()
	// RFC 2661 Section 6.12: end the former owner before reusing its pair.
	outgoing := tunnel.handleCDN(old.localSID, buildCDN(1, old.remoteSID), reactor.logger)
	handoff := &serviceLifetimeHandoff{}
	current := &relayCall{remote: "replacement", handoff: handoff, teardown: reactor.teardownRelaySession}
	replacement := &L2TPSession{localSID: old.localSID, remoteSID: 72, state: L2TPSessionEstablished, relay: current}
	tunnel.addSession(replacement)
	before := tunnel.engine.Stats()
	reactor.tunnelsMu.Unlock()
	require.Empty(t, outgoing)
	reactor.handleKernelError(kernelSetupFailed{localTID: 100, localSID: old.localSID, relay: stale, err: errors.New("late old setup failure")})
	reactor.tunnelsMu.Lock()
	survivor, after := tunnel.lookupSession(replacement.localSID), tunnel.engine.Stats()
	reactor.tunnelsMu.Unlock()
	require.Same(t, replacement, survivor)
	require.Equal(t, before, after)
	require.Zero(t, handoff.count())
	reactor.handleKernelError(kernelSetupFailed{localTID: 100, localSID: replacement.localSID, relay: current, err: errors.New("current setup failed")})
	reactor.tunnelsMu.Lock()
	removed, sibling := tunnel.lookupSession(replacement.localSID), tunnel.lookupSession(51)
	reactor.tunnelsMu.Unlock()
	require.Nil(t, removed, "a current-owner failure still tears the call down")
	require.NotNil(t, sibling, "the other call survives either refusal")
	require.Equal(t, 1, handoff.count())
}

// serviceLifetimeHandoff deliberately implements only Ended; service calls
// must not require the realm-only Quiesce or Bridged callbacks.
type serviceLifetimeHandoff struct {
	mu    sync.Mutex
	ended []string
}

func (h *serviceLifetimeHandoff) Ended(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ended = append(h.ended, reason)
}
func (h *serviceLifetimeHandoff) count() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.ended) }

type serviceLifetimeFixture struct {
	reactor *l2tpReactor
	tunnel  *L2TPTunnel
	session *L2TPSession
	call    *relayCall
	peer    *testClient
	handoff *serviceLifetimeHandoff
}

func newServiceLifetimeFixture(t *testing.T) serviceLifetimeFixture {
	t.Helper()
	listener, reactor, _, stop := buildLogReactor(t)
	t.Cleanup(stop)
	peer := newClient(t, listener)
	t.Cleanup(func() { peer.Close() })
	remote := clientAddrPort(t, peer)
	sibling := &L2TPSession{localSID: 51, remoteSID: 52, state: L2TPSessionEstablished}
	insertEstablishedTunnel(t, reactor, 100, 200, remote, sibling)
	subsystem := &Subsystem{params: Parameters{Remotes: []Remote{{Name: "lns", Address: remote}}, Relays: []RelayBinding{{Service: "wholesale", Remote: "lns"}}}, reactors: []*l2tpReactor{reactor}, logger: slog.Default()}
	handoff := &serviceLifetimeHandoff{}
	// RFC 2661 Section 10.1: originate through the real relay sink.
	token, accepted, err := (&relaySink{s: subsystem}).Relay(callsink.Request{Service: "wholesale", Channel: relayDescriptorForTest(9), Handoff: handoff})
	require.NoError(t, err)
	require.True(t, accepted)
	call, ok := token.(*relayCall)
	require.True(t, ok)
	icrq := readUntilMsgType(t, peer, MsgICRQ, 4)
	request, err := ParseMessageHeader(icrq)
	require.NoError(t, err)
	// RFC 2661 Section 5.8: acknowledge the real ICRQ through the receive
	// path before a later call reply or teardown needs its send-window slot.
	var acknowledgement [ControlHeaderLen]byte
	WriteControlHeader(acknowledgement[:], 0, ControlHeaderLen, 100, 0, 0, request.Ns+1)
	reactor.handle(rxPacket{from: remote, bytes: acknowledgement[:], release: func() {}})
	reactor.tunnelsMu.Lock()
	tunnel := reactor.tunnelsByLocalID[100]
	var session *L2TPSession
	outstanding := -1
	if tunnel != nil {
		outstanding = tunnel.engine.Outstanding()
		for _, candidate := range tunnel.sessions {
			if candidate.relay == call {
				session = candidate
			}
		}
	}
	reactor.tunnelsMu.Unlock()
	require.NotNil(t, tunnel)
	require.NotNil(t, session, "the accepted token must own its actual L2TP session")
	require.Zero(t, outstanding, "the peer must acknowledge the actual ICRQ before the call is exercised")
	return serviceLifetimeFixture{reactor: reactor, tunnel: tunnel, session: session, call: call, peer: peer, handoff: handoff}
}

func (f serviceLifetimeFixture) establish(t *testing.T) {
	t.Helper()
	f.reactor.tunnelsMu.Lock()
	f.session.state = L2TPSessionEstablished
	f.session.remoteSID = 707
	f.reactor.tunnelsMu.Unlock()
	require.True(t, f.call.markBridged(100, f.session.localSID))
}

// A tunnel that fails before ICRQ has no session to remove. Its actual pending
// queue must nevertheless notify the accepted Service-Name admission once.
func TestServiceRelayTunnelFailureEndsAccess(t *testing.T) {
	listener, reactor, _, stop := buildLogReactor(t)
	t.Cleanup(stop)
	peer := newClient(t, listener)
	t.Cleanup(func() { peer.Close() })
	remote := clientAddrPort(t, peer)
	subsystem := &Subsystem{params: Parameters{Remotes: []Remote{{Name: "lns", Address: remote}}, Relays: []RelayBinding{{Service: "wholesale", Remote: "lns"}}}, reactors: []*l2tpReactor{reactor}, logger: slog.Default()}
	handoff := &serviceLifetimeHandoff{}
	// RFC 2661 Section 10.1: originate through the real relay sink.
	token, accepted, err := (&relaySink{s: subsystem}).Relay(callsink.Request{Service: "wholesale", Channel: relayDescriptorForTest(9), Handoff: handoff})
	require.NoError(t, err)
	require.True(t, accepted)
	packet := readUntilMsgType(t, peer, MsgSCCRQ, 4)
	id := extractAVP(t, packet, AVPAssignedTunnelID)
	require.Len(t, id, 2)
	tid := uint16(id[0])<<8 | uint16(id[1])
	func() {
		reactor.tunnelsMu.Lock()
		defer reactor.tunnelsMu.Unlock()
		tunnel := reactor.tunnelsByLocalID[tid]
		require.NotNil(t, tunnel)
		require.Len(t, tunnel.pendingCalls, 1)
		tunnel.resolvePendingCall(callOutcome{err: errCallTunnelSetupFailed})
	}()
	require.Equal(t, 1, handoff.count())
	token.Cancel()
	require.Equal(t, 1, handoff.count())
}

// A matched request without an end owner cannot be admitted into an async
// call. The opposite valid request is exercised by the lifecycle tests above.
func TestServiceRelayRequiresEndHandoff(t *testing.T) {
	listener, reactor, _, stop := buildLogReactor(t)
	t.Cleanup(stop)
	peer := newClient(t, listener)
	t.Cleanup(func() { peer.Close() })
	subsystem := &Subsystem{params: Parameters{Remotes: []Remote{{Name: "lns", Address: clientAddrPort(t, peer)}}, Relays: []RelayBinding{{Service: "wholesale", Remote: "lns"}}}, reactors: []*l2tpReactor{reactor}, logger: slog.Default()}
	// RFC 2661 Section 10.1: originate through the real relay sink.
	token, accepted, err := (&relaySink{s: subsystem}).Relay(callsink.Request{Service: "wholesale", Channel: relayDescriptorForTest(9)})
	require.ErrorIs(t, err, errRelayNoHandoff)
	require.False(t, accepted)
	require.Nil(t, token)
	reactor.tunnelsMu.Lock()
	count := len(reactor.tunnelsByLocalID)
	reactor.tunnelsMu.Unlock()
	require.Zero(t, count, "a missing end owner must refuse before dialing")
}

// Service-Name ownership must not invent the realm-only operator projection.
func TestServiceRelayHasNoRealmSnapshot(t *testing.T) {
	f := newServiceLifetimeFixture(t)
	require.Nil(t, f.call.snapshot())
}

// A live Service-Name call still completes ICRP with an ICCN and schedules its
// kernel setup. Refusing all relays cannot satisfy cancellation-only tests.
func TestServiceRelayLiveReplyEstablishesCall(t *testing.T) {
	f := newServiceLifetimeFixture(t)
	f.reactor.tunnelsMu.Lock()
	// RFC 2661 Section 10.1: a valid incoming-call reply advances the call.
	outgoing := f.tunnel.handleICRP(f.session, icrpBody(707), f.reactor.params.Clock(), f.reactor.logger)
	state, setup := f.session.state, f.session.kernelSetupNeeded
	f.reactor.tunnelsMu.Unlock()
	require.Len(t, outgoing, 1)
	require.Equal(t, MsgICCN, msgTypeOf(t, outgoing[0].bytes))
	require.Equal(t, L2TPSessionEstablished, state)
	require.True(t, setup)
}

// Completion and cancellation both forbid a new bridge. A completed owner
// reports one end, and subsequent Cancel must not resurrect its teardown.
func TestRelayCompletedOwnerCannotBridge(t *testing.T) {
	handoff := &serviceLifetimeHandoff{}
	call := &relayCall{handoff: handoff}
	call.end("remote call ended")
	require.False(t, call.markBridged(100, 41))
	require.True(t, call.stopped())
	call.Cancel()
	call.end("duplicate end")
	require.Equal(t, 1, handoff.count())
}
