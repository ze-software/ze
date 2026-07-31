// RFC 7296 Section 2.3 obligations. One covers a peer request that arrives while one of
// our own is unanswered. The other covers a request whose Message ID is outside the
// window.
//
// Helpers here start with `osr`, so they cannot collide with the sibling RFC files in
// this package. This file reuses the `rtx` loopback helpers and the `win` DPD helper.
//
// rfc-test-change-approved: 2026-07-31 the owner gave standing approval to strengthen a
// tagged test whose tag asserted more than its body drove, for the whole of
// plan/spec-rfcgate-1b-rfc7296-pilot.md. It covers edits that make a tagged test assert MORE
// than before, and nothing that weakens one. Here it covers
// TestOsrOwnerLoopKeepsAForeignWindowHeld, which passed dpd == nil and so never reached
// retireRequest at all.

package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// osrRequest builds the peer's INFORMATIONAL request at msgID. An empty inner chain is
// the liveness probe of RFC 7296 Section 1.4, which every endpoint must answer.
func osrRequest(t *testing.T, peer *SA, msgID uint32) []byte {
	t.Helper()
	raw, err := buildEncryptedMessageEx(peer, nil, msgID,
		wire.ExchangeInformational, initiatorFlag(peer))
	if err != nil {
		t.Fatalf("build the peer request at id %d: %v", msgID, err)
	}
	return raw
}

// osrSession returns an established pair on a loopback link, with the peer address
// resolvable so sendRaw reaches peerTr.
func osrSession(t *testing.T) (ini, peer *SA, ps *PeerSession, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	peer, ini, ps = establishPSK(t)
	peerTr, myTr = rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	if ini.remoteUDPAddr() == nil {
		t.Fatal("the SA has no resolvable peer address")
	}
	return ini, peer, ps, peerTr, myTr
}

// VALIDATES: a peer request that arrives while our own request is unanswered is
// accepted, processed and answered, and accepting it neither frees nor strands our
// own request window.
// PREVENTS: a deadlock where both endpoints wait for an answer neither will send, and
// the opposite defect where accepting the peer request lets a second request of ours
// go out beside the first.
//
// RFC requirement: RFC7296-2.3-8 positive -- RFC 7296 Section 2.3 requires an endpoint to accept
// and process a request while it has a request outstanding. That rule avoids a deadlock.
// The checklist row carries the sentence verbatim (rfc/full/rfc7296.txt:1472-1475).
// classifyInbound (msgid.go) judges a request against ExpectedMsgID alone and never reads
// requestOutstanding. handleOwnedInbound (inbound.go) then decrypts and dispatches it, and
// handleInformationalOwned answers it at the request's own Message ID.
//
// RFC requirement: RFC7296-2.3-8 negative -- accepting the peer request does not free our own
// request window. A Delete raised right after it still finds the window held. Ze never
// puts two requests in flight, so Section 2.3's own wait rule stays intact.
//
// RFC requirement: RFC7296-2.3-8 negative -- accepting the peer request does not strand our own
// window either. Retiring the probe frees the window it holds, so the deferred request
// goes out at once rather than after the 30-second requestWindowTimeout.
func TestOsrRequestAcceptedWhileOursIsOutstanding(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	// Our own request goes out and stays unanswered.
	dpd := winDueDPD()
	sendDPD(ini, myTr, dpd, log)
	probe := rtxRecv(t, peerTr)
	if probe == nil {
		t.Fatal("the DPD probe never reached the peer")
	}
	if !ini.requestOutstanding {
		t.Fatal("the probe did not take the request window")
	}

	// The peer's own request crosses ours. It is accepted and answered.
	req := osrRequest(t, peer, ini.ExpectedMsgID)
	reqID := parseMsg(t, req).Header.MessageID
	out := ps.handleOwnedInbound(ini, transport.Packet{Data: req}, myTr, nil, log)
	if !out.peerAlive {
		t.Fatal("the peer request was not processed while our own request was outstanding")
	}
	answer := rtxRecv(t, peerTr)
	if answer == nil {
		t.Fatal("the peer request drew no answer; RFC 7296 Section 2.3 requires it to be " +
			"accepted and processed, and Section 1.4 requires an answer")
	}
	hdr := parseMsg(t, answer).Header
	if hdr.Flags&wire.FlagResponse == 0 {
		t.Error("the answer does not carry the Response flag, so it is not a response")
	}
	if hdr.MessageID != reqID {
		t.Errorf("the answer carries Message ID %d, want %d (the id of the request)",
			hdr.MessageID, reqID)
	}
	if ini.ExpectedMsgID != reqID+1 {
		t.Errorf("ExpectedMsgID = %d after answering request %d, want %d",
			ini.ExpectedMsgID, reqID, reqID+1)
	}

	// Answering the peer does not free OUR window, so we never have two requests in
	// flight. The DPD probe is still the holder.
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	rtxExpectSilence(t, peerTr, myTr, remote,
		"an ESP Delete after answering a peer request while our probe is unanswered")

	// Nor does it strand the window. maintainSA retires the probe together with the
	// window it holds, so the next request goes out on the following tick rather than
	// 30 seconds later.
	if dpd.awaitingReply() {
		ini.retireRequest(dpd.probeMsgID)
	}
	handleDPDResponse(dpd, log, ps.peerName)
	if ini.requestOutstanding {
		t.Fatal("the window is still held after the probe was retired; the SA would raise " +
			"no request for the whole requestWindowTimeout")
	}
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the deferred Delete never went out after the probe and its window retired")
	}
}

// VALIDATES: the peerAlive path of the owner loop retires the probe and the request
// window in one step, through maintainSA itself.
// PREVENTS: the retire call living only in a test. The defect it fixes is reached
// through the loop, so the loop is what must be driven.
//
// RFC requirement: RFC7296-2.3-8 positive -- the deadlock Section 2.3 names is avoided through
// maintainSA (established.go), not only through a helper. The loop answers the crossing
// peer request. It also retires the probe with the window it holds, so the SA can raise
// its next request at once.
func TestOsrOwnerLoopRetiresTheStrandedWindow(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)

	dpd := winDueDPD()
	sendDPD(ini, myTr, dpd, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the DPD probe never reached the peer")
	}
	probeID := dpd.probeMsgID

	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, 4)
	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(ini, dpd, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(done)
	}()

	// The peer's request crosses our probe. The loop answers it.
	ps.inbound <- transport.Packet{Data: osrRequest(t, peer, ini.ExpectedMsgID)}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the owner loop did not answer the peer request")
	}
	close(ps.stopCh)
	<-done

	if ini.requestOutstanding {
		t.Errorf("the request window is still held by probe %d after an authenticated "+
			"inbound retired the probe", probeID)
	}
	if dpd.awaitingReply() {
		t.Error("the probe is still awaiting a reply after an authenticated inbound")
	}
}

// VALIDATES: a request whose Message ID is outside the window draws no datagram at all,
// while a request at the expected id and a retransmit of the previous one are both
// answered.
// PREVENTS: acknowledging an invalid request, and a classifier that reaches the same
// silence by answering nothing.
//
// RFC requirement: RFC7296-2.3-5 positive -- RFC 7296 Section 2.3 forbids INVALID_MESSAGE_ID in a
// response. It also forbids any acknowledgement of the invalid request. The checklist row
// carries the sentence verbatim (rfc/full/rfc7296.txt:1503-1506). classifyInbound returns
// inboundInvalid (msgid.go). handleOwnedInbound's inboundInvalid arm (inbound.go) logs at
// debug and writes no datagram, so the request draws neither a response nor a notify.
//
// RFC requirement: RFC7296-2.3-5 negative -- the silence is scoped to an out-of-window id. A
// request at the expected id IS answered, and a retransmit of the previous id replays the
// cached response. The drop is therefore a decision about the Message ID and not a sender
// that writes nothing.
func TestOsrOutOfWindowRequestIsNotAcknowledged(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	// Control arm. A request at the expected id IS answered, so silence below is a
	// decision about the id and not a mute sender.
	good := osrRequest(t, peer, ini.ExpectedMsgID)
	goodID := parseMsg(t, good).Header.MessageID
	ps.handleOwnedInbound(ini, transport.Packet{Data: good}, myTr, nil, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("a request at the expected id drew no answer")
	}

	// The invalid request. Its id is far outside the one-request window.
	for _, offset := range []uint32{5, 100, 0xFFFF} {
		bad := osrRequest(t, peer, ini.ExpectedMsgID+offset)
		ps.handleOwnedInbound(ini, transport.Packet{Data: bad}, myTr, nil, log)
		rtxExpectSilence(t, peerTr, myTr, remote, "a request outside the window")
	}

	// A retransmit of the previous request replays the cached response. The drop is
	// therefore scoped to an out-of-window id, and it has not swallowed the retransmit
	// path.
	again := osrRequest(t, peer, goodID)
	ps.handleOwnedInbound(ini, transport.Packet{Data: again}, myTr, nil, log)
	replay := rtxRecv(t, peerTr)
	if replay == nil {
		t.Fatal("a retransmitted request drew no cached response")
	}
	if parseMsg(t, replay).Header.MessageID != goodID {
		t.Errorf("the replayed response carries Message ID %d, want %d",
			parseMsg(t, replay).Header.MessageID, goodID)
	}
}

// TestOsrClassifierRejectsOutOfWindow pins the producer of the silence above.
// classifyInbound is the only place an established-SA message is judged against the
// window, and it runs before the message is decrypted.
func TestOsrClassifierRejectsOutOfWindow(t *testing.T) {
	sa := testSA()
	sa.ExpectedMsgID = 10
	cacheResponse(sa, 9, []byte("cached"))
	sa.ExpectedMsgID = 10

	for _, tc := range []struct {
		name  string
		msgID uint32
		want  inboundClass
	}{
		{"the expected id", 10, inboundNewRequest},
		{"the previous id", 9, inboundRetransmit},
		{"one below the previous id", 8, inboundInvalid},
		{"one above the expected id", 11, inboundInvalid},
		{"far above the expected id", 1000, inboundInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyInbound(sa, tc.msgID, false, nil); got != tc.want {
				t.Errorf("classifyInbound(id %d) = %v, want %v", tc.msgID, got, tc.want)
			}
		})
	}

	// The classifier never yields a class that carries a notification, so no
	// INVALID_MESSAGE_ID can leave through it. Its only answer to an invalid request
	// is the silent drop that handleOwnedInbound performs.
	if inboundInvalid != 0 {
		t.Errorf("inboundInvalid = %d, want the zero value; a new class inserted before "+
			"it would silently change what an unclassified message means", inboundInvalid)
	}
}

// VALIDATES: the owner loop retires the request window only while a DPD probe is actually
// awaiting its reply. A window held by a Delete or a rekey survives an authenticated
// inbound message.
// PREVENTS: freeing the window on every authenticated inbound, which lets a second
// request go out beside a request that is still unanswered.
//
// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the whole of
// plan/spec-rfcgate-1b-rfc7296-pilot.md, to strengthen a tagged test whose tag asserted more
// than its body drove. This body passed dpd == nil. So established.go's awaitingReply gate
// short-circuited, and retireRequest was never called. No mutation of either guard turned
// the test red. The approval covers strengthening only, never weakening.
//
// RFC requirement: RFC7296-2.3-8 negative -- accepting and processing a peer request is bounded.
// maintainSA (established.go) reaches retireRequest only through its awaitingReply gate. A
// window held by a Delete or a rekey therefore survives an authenticated inbound. Section 2.3
// still requires Ze to wait for the answer to that request, and the second request is
// refused. The Message ID half of the same guard is pinned by
// TestOsrRetireOnlyFreesItsOwnWindow, which drives retireRequest directly.
func TestOsrOwnerLoopKeepsAForeignWindowHeld(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	// A best-effort Delete takes the window. No DPD probe is outstanding, so the probe
	// is not the holder.
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the ESP Delete never reached the peer")
	}
	if !ini.requestOutstanding {
		t.Fatal("the Delete did not take the request window")
	}
	heldBy := ini.requestMsgID

	// DPD state that is NOT awaiting a reply. That is the only state reachable while
	// another request holds the window, because sendDPD takes the window itself and
	// refuses when it is already held. handleDPDResponse leaves probeMsgID behind when it
	// clears awaitReply, so a stale id here is what the loop really sees.
	//
	// probeMsgID carries the Delete's own id ON PURPOSE. That makes awaitingReply the single
	// guard under test. Remove that gate and the window is freed here, rather than being
	// caught by the Message ID comparison behind it. The long interval keeps the ticker from
	// raising a probe of its own.
	dpd := &dpdState{
		interval:   time.Hour,
		timeout:    time.Hour,
		lastSent:   time.Now(),
		probeMsgID: heldBy,
	}
	if dpd.awaitingReply() {
		t.Fatal("the test's DPD state is awaiting a reply; it must not be, or the retire " +
			"runs and this test stops isolating the awaitingReply gate")
	}

	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, 4)
	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(ini, dpd, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(done)
	}()

	// An authenticated peer request arrives and is answered.
	ps.inbound <- transport.Packet{Data: osrRequest(t, peer, ini.ExpectedMsgID)}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the owner loop did not answer the peer request")
	}
	close(ps.stopCh)
	<-done

	if !ini.requestOutstanding || ini.requestMsgID != heldBy {
		t.Fatalf("the window held by the Delete at id %d was freed by an authenticated "+
			"inbound (outstanding=%v id=%d); only the DPD probe's own window may be retired",
			heldBy, ini.requestOutstanding, ini.requestMsgID)
	}

	// The proof that it is still held: a second request is refused.
	ps.sendDeleteIKE(ini, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote,
		"an IKE Delete while the ESP Delete's window is still held")
}

// VALIDATES: retireRequest frees the window only for the request that actually holds it.
// A probe abandoned at any other Message ID leaves the window where it is.
// PREVENTS: an abandoned DPD probe freeing a Delete's or a rekey's window, which would put
// two of our own requests in flight on one SA.
//
// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the whole of
// plan/spec-rfcgate-1b-rfc7296-pilot.md, to strengthen tagged coverage. This test is new. It
// adds proof, and it removes none.
//
// It drives the producer directly rather than through maintainSA. The owner loop reaches
// retireRequest only while a probe awaits its reply. And sendDPD refuses the window when
// another request already holds it. The loop therefore cannot present retireRequest with a
// foreign id itself. The comparison is a fail-closed guard on the function's own contract
// (ai/rules/fail-closed-guards.md), and this is where that contract is proven.
//
// RFC requirement: RFC7296-2.3-8 negative -- accepting and processing a peer request is bounded.
// Section 2.3 lets the next request go out only once the outstanding one is settled. Nothing
// can therefore free a window that belongs to a different Message ID. The loop half of the
// same guard is pinned by TestOsrOwnerLoopKeepsAForeignWindowHeld.
func TestOsrRetireOnlyFreesItsOwnWindow(t *testing.T) {
	sa := testSA()
	sa.NextMsgID = 7
	if !sa.reserveRequestWindow() {
		t.Fatal("the window was not free on a fresh SA, so nothing below is a retire test")
	}
	held := sa.requestMsgID
	if held != 7 {
		t.Fatalf("the window was taken at id %d, want 7 (the SA's NextMsgID)", held)
	}

	for _, foreign := range []uint32{held - 1, held + 1, 0, ^uint32(0)} {
		sa.retireRequest(foreign)
		if !sa.requestOutstanding {
			t.Fatalf("retireRequest(%d) freed a window held at id %d; only the holder's own "+
				"id may retire it, or an abandoned probe frees a Delete's window and two "+
				"requests go out at once", foreign, held)
		}
	}

	// The control. The holder's own id DOES free the window. The four refusals above are
	// therefore the Message ID comparison speaking, and not a function that frees nothing.
	sa.retireRequest(held)
	if sa.requestOutstanding {
		t.Fatalf("retireRequest(%d) did not free the window it holds", held)
	}
}
