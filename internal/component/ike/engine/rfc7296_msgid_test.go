// RFC 7296 Message ID lifecycle obligations. Two rows live here.
//
// Section 2.2 makes the Message ID replay protection, and it makes the SA close or
// rekey rather than let a counter wrap. Section 2.25 makes a TEMPORARY_FAILURE answer
// a wait rather than a one-second retry loop.
//
// Each test carries an `RFC requirement:` tag for the row it proves. Helpers here start
// with `mid`, so they cannot collide with the sibling RFC files in this package. This
// file reuses the `rtx` loopback helpers, the `win` lifetime and DPD helpers, and the
// `rky` dataplane recorder.

package engine

import (
	"math"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// midTemporaryFailure returns the payload chain of a CREATE_CHILD_SA response that
// carries nothing but a TEMPORARY_FAILURE notify. RFC 7296 Section 2.25: a peer sends
// it when a rekey cannot be completed because it is busy with one of its own.
func midTemporaryFailure() []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyTemporaryFailure}},
	}
}

// the unused helper midChildRekeyPending was removed here. No test called it, so
// it carried no coverage. Its only failure path was a constructor guard on its own input.
// `make ze-lint-changed` fails this package while it stands ("func midChildRekeyPending is
// unused"). A later work package that needs the same fixture builds the one it needs.
//
// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the whole of
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, to strengthen tagged coverage. Removing dead
// scaffolding from a tagged file removes no proof. Every RFC7296-2.2-2 tag in this file
// keeps its test and its assertions.

// VALIDATES: the outbound Message ID stops at the 32-bit ceiling instead of wrapping,
// and the owner loop closes the SA once it stops.
// PREVENTS: a bare `sa.NextMsgID++` returning the counter to 0 while the same keys stay
// in use, which replays every Message ID the SA has already spent.
//
// RFC requirement: RFC7296-2.2-2 positive -- RFC 7296 Section 2.2 requires the IKE SA to be
// closed or rekeyed once the Message ID no longer fits in 32 bits. The checklist row
// carries the sentence verbatim (rfc/full/rfc7296.txt:1437-1440). advanceMsgID (msgid.go)
// marks the SA exhausted at math.MaxUint32. maintainSA's ticker arm (established.go)
// answers the flag by setting StateDead. That ends the owner loop and withdraws the
// tunnel.
//
// RFC requirement: RFC7296-2.2-2 negative -- the counter is frozen rather than reset. NextMsgID
// still reads math.MaxUint32 after the SA closes, so the id space is never reused. The
// same Section 2.2 sentence calls the Message ID replay protection, and a counter that
// returned to 0 would hand that protection back.
func TestMidOutboundCounterFreezesAtTheCeiling(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	if sa.remoteUDPAddr() == nil {
		t.Fatal("the SA has no resolvable peer address")
	}
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)

	// The last legal id is spent on a real request, which the peer reads.
	sa.NextMsgID = math.MaxUint32
	sendDPD(sa, myTr, winDueDPD(), log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the request at the last legal id never reached the peer")
	}

	// Negative. The counter did not wrap.
	if sa.NextMsgID != math.MaxUint32 {
		t.Errorf("NextMsgID = %d after spending the last id, want %d; the counter must "+
			"never wrap to a value it has already used", sa.NextMsgID, uint32(math.MaxUint32))
	}
	if !sa.msgIDExhausted {
		t.Fatal("the SA is not marked exhausted after spending the last legal Message ID")
	}

	// No further request can be built, so the SA cannot spend an id it has already used.
	if sa.reserveRequestWindow() {
		t.Error("an exhausted SA still reserved the request window; no later request may " +
			"be built on it")
	}

	// Positive. The owner loop closes the SA on its next tick.
	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(sa, nil, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(rtxArrive):
		close(ps.stopCh)
		t.Fatal("the owner loop did not return, so the exhausted SA was never closed")
	}
	if sa.State != StateDead {
		t.Errorf("SA state = %v after exhaustion, want dead; Section 2.2 requires the SA "+
			"to be closed or rekeyed", sa.State)
	}
	if sa.NextMsgID != math.MaxUint32 {
		t.Errorf("NextMsgID = %d after the close, want %d", sa.NextMsgID, uint32(math.MaxUint32))
	}
}

// VALIDATES: the SA rekeys itself while ids remain, so the ceiling is reached only when
// the graceful path has already failed.
// PREVENTS: a design where the only answer to a climbing counter is a tunnel outage.
//
// RFC requirement: RFC7296-2.2-2 positive -- Section 2.2 offers two remedies, "closed or rekeyed"
// (rfc/full/rfc7296.txt:1439-1440). maintainSA takes the second one first. Once
// msgIDNearExhaustion reports the counter inside the headroom below the ceiling
// (msgid.go), the ticker arm raises an IKE SA rekey. The replacement SA then starts at 0,
// per Section 2.18.
//
// RFC requirement: RFC7296-2.2-2 negative -- the rekey comes from the counter and not from a
// lifetime. The same loop raises nothing at all with the counter one below the threshold
// and no lifetime state. The request above is therefore the threshold speaking.
func TestMidNearExhaustionRekeysTheIKESA(t *testing.T) {
	log := slogutil.DiscardLogger()
	peer, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := sa.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA has no resolvable peer address")
	}

	// Negative. One id below the threshold, with no lifetime state, the loop is quiet.
	sa.NextMsgID = msgIDRekeyThreshold - 1
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	quiet := make(chan struct{})
	go func() {
		_ = ps.maintainSA(sa, nil, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(quiet)
	}()
	time.Sleep(1500 * time.Millisecond) // one full ticker period, so a tick has run
	close(ps.stopCh)
	<-quiet
	rtxExpectSilence(t, peerTr, myTr, remote, "a tick one id below the rekey threshold")
	if ps.pendingRekey != nil {
		t.Fatal("a rekey started one id below the threshold")
	}

	// Positive. At the threshold the same loop raises an IKE SA rekey.
	sa.NextMsgID = msgIDRekeyThreshold
	ps.stopCh = make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(sa, nil, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(done)
	}()
	req := rtxRecv(t, peerTr)
	close(ps.stopCh)
	<-done
	if req == nil {
		t.Fatal("the tick at the rekey threshold wrote no request")
	}
	if ps.pendingRekey == nil {
		t.Fatal("no rekey exchange is outstanding after the threshold tick")
	}
	defer ps.pendingRekey.clear()
	if ps.pendingRekey.kind != rekeyIKE {
		t.Fatalf("the threshold raised rekey kind %v, want an IKE SA rekey", ps.pendingRekey.kind)
	}
	hdr := parseMsg(t, req).Header
	if hdr.ExchangeType != wire.ExchangeCreateChildSA {
		t.Errorf("the threshold request exchange = %d, want CREATE_CHILD_SA", hdr.ExchangeType)
	}
	if hdr.Flags&wire.FlagResponse != 0 {
		t.Error("the threshold datagram carries the Response flag, so it is not a request")
	}

	// The replacement SA the exchange produces starts both counters at zero, so the
	// rekey really does answer the exhaustion rather than carry it forward.
	reqInner, err := decryptAndParse(peer, parseMsg(t, req), req)
	if err != nil {
		t.Fatalf("the peer could not decrypt the rekey request: %v", err)
	}
	respBytes, _, err := respondIKERekey(peer, reqInner, hdr.MessageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	respInner, err := decryptAndParse(sa, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("decrypt of the rekey response: %v", err)
	}
	newSA, err := applyIKERekeyResponse(sa, ps.pendingRekey, respInner, log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}
	if newSA.NextMsgID != 0 || newSA.ExpectedMsgID != 0 {
		t.Errorf("the replacement SA counters = next:%d expected:%d, want 0 and 0",
			newSA.NextMsgID, newSA.ExpectedMsgID)
	}
	if newSA.msgIDExhausted {
		t.Error("the replacement SA is already marked exhausted")
	}
}

// VALIDATES: the inbound Message ID counter stops at the ceiling too, so a peer cannot
// drive it around to zero and replay its own request sequence.
// PREVENTS: `sa.ExpectedMsgID = msgID + 1` wrapping to 0, after which classifyInbound
// accepts a request at id 0 again under the same keys.
//
// RFC requirement: RFC7296-2.2-2 positive -- Section 2.2 states "Note that Message IDs are
// cryptographically protected and provide protection against message replays"
// (rfc/full/rfc7296.txt:1437-1438), and it fixes the remedy at 32 bits. The peer drives
// ExpectedMsgID, so the receive side has the reachable ceiling. advanceExpectedMsgID
// (msgid.go) freezes it and marks the SA exhausted, which closes the SA.
//
// RFC requirement: RFC7296-2.2-2 negative -- a request replayed at id 0 after the ceiling is
// classified invalid rather than new. The control arm sits below the ceiling. There the
// same counter advances and the next id IS accepted. The freeze is therefore a decision,
// and not a classifier that refuses everything.
func TestMidInboundCounterFreezesAtTheCeiling(t *testing.T) {
	// Control arm first. Below the ceiling the counter advances and the next request
	// is accepted.
	ok := testSA()
	ok.ExpectedMsgID = 7
	cacheResponse(ok, 7, []byte("cached response"))
	if ok.ExpectedMsgID != 8 {
		t.Fatalf("ExpectedMsgID = %d after answering id 7, want 8", ok.ExpectedMsgID)
	}
	if ok.msgIDExhausted {
		t.Fatal("an SA below the ceiling is marked exhausted")
	}
	if got := classifyInbound(ok, 8, false, nil); got != inboundNewRequest {
		t.Fatalf("a request at the next id classified %v, want inboundNewRequest", got)
	}

	// Positive. At the ceiling the counter is frozen and the SA is marked exhausted.
	sa := testSA()
	sa.ExpectedMsgID = math.MaxUint32
	cacheResponse(sa, math.MaxUint32, []byte("cached response"))
	if sa.ExpectedMsgID != math.MaxUint32 {
		t.Errorf("ExpectedMsgID = %d after answering the last id, want %d; a wrap to 0 "+
			"reopens every id the peer has already used",
			sa.ExpectedMsgID, uint32(math.MaxUint32))
	}
	if !sa.msgIDExhausted {
		t.Error("the SA is not marked exhausted after answering the last legal Message ID")
	}

	// The response to that last request is still cached, so the peer's retransmit of it
	// is answered. Freezing the counter does not break the exchange that reached it.
	if got := classifyInbound(sa, math.MaxUint32, false, nil); got != inboundRetransmit {
		t.Errorf("a retransmit of the last request classified %v, want inboundRetransmit", got)
	}

	// Negative. A request at id 0 is out of window, not the next one in sequence.
	if got := classifyInbound(sa, 0, false, nil); got != inboundInvalid {
		t.Errorf("a request at id 0 after the ceiling classified %v, want inboundInvalid; "+
			"the counter must not have wrapped", got)
	}
}

// VALIDATES: a rekey the peer answers with TEMPORARY_FAILURE waits before it is retried,
// and the wait ends.
// PREVENTS: the level-triggered soft lifetime re-raising the same rekey on every
// one-second tick against a peer that has explicitly asked for a delay.
//
// RFC requirement: RFC7296-2.25-1 positive -- RFC 7296 Section 2.25 forbids an immediate retry
// after a TEMPORARY_FAILURE notify. It requires the recipient to wait for the peer to
// finish the operation that caused the condition. The checklist row carries the sentence
// verbatim (rfc/full/rfc7296.txt:3912-3918). applyChildRekeyResponse (rekey.go) reads the
// notify and returns errTemporaryFailure. handleCreateChildSAOwned (inbound.go) arms the
// hold, and startChildRekey (established.go) refuses while it stands. A soft-expired
// lifetime is a level trigger, so the next tick would otherwise retry at once.
//
// RFC requirement: RFC7296-2.25-1 negative -- the wait is a delay and not a stop. The same
// section permits a retry over a period of several minutes. Once the hold elapses, the
// rekey IS raised again.
//
// RFC requirement: RFC7296-2.25-1 negative -- the hold is keyed to the notify and not to any
// failed response. A response that fails for another reason arms nothing. An ordinary
// rekey failure therefore still retries on the next tick, and the tunnel is not left on
// expiring keys.
func TestMidTemporaryFailureDefersTheRetry(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := sa.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA has no resolvable peer address")
	}

	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)

	// Baseline. The rekey path works, so a later silence is the hold and not a dead path.
	ps.startChildRekey(sa, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the first rekey request never reached the peer")
	}
	if ps.pendingRekey == nil {
		t.Fatal("the first rekey left no outstanding exchange")
	}
	firstID := ps.pendingRekey.messageID

	// The peer answers it with TEMPORARY_FAILURE and nothing else.
	respMsg := &wire.Message{Header: wire.Header{MessageID: firstID}}
	out := ps.handleCreateChildSAOwned(sa, respMsg, midTemporaryFailure(), true, myTr, dp, log)
	if out.newChild != nil {
		t.Fatal("a TEMPORARY_FAILURE answer installed a replacement Child SA")
	}
	if ps.pendingRekey != nil {
		t.Fatal("the refused exchange is still outstanding")
	}
	sa.releaseRequestWindow()

	// Positive. The next attempt is refused while the hold stands.
	ps.startChildRekey(sa, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "a rekey retried right after a TEMPORARY_FAILURE answer")
	if ps.pendingRekey != nil {
		t.Fatal("a rekey started while the TEMPORARY_FAILURE hold stands")
	}

	// The owner loop raises nothing either, so the level-triggered soft lifetime cannot
	// walk around the hold on its one-second tick.
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	looped := make(chan struct{})
	go func() {
		_ = ps.maintainSA(sa, nil, winSoftExpired(), nil,
			testIKEGroup(), NewSATable(), dp, myTr, nil, log)
		close(looped)
	}()
	time.Sleep(1500 * time.Millisecond) // one full ticker period, so a tick has run
	close(ps.stopCh)
	<-looped
	rtxExpectSilence(t, peerTr, myTr, remote, "an owner-loop tick while the TEMPORARY_FAILURE hold stands")
	ps.setChildSA(old)

	// Negative one. The hold ends, and the same rekey goes out.
	ps.childRekeyHoldUntil = time.Now().Add(-time.Second)
	ps.startChildRekey(sa, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the rekey never went out after the hold elapsed; the wait must be a delay, not a stop")
	}
	if ps.pendingRekey == nil {
		t.Fatal("the retried rekey left no outstanding exchange")
	}
	secondID := ps.pendingRekey.messageID

	// Negative two. A response that fails for another reason arms no hold.
	badMsg := &wire.Message{Header: wire.Header{MessageID: secondID}}
	empty := []wire.PayloadEntry{{Payload: &wire.PayloadNonce{NonceData: testNonce(9)}}}
	ps.handleCreateChildSAOwned(sa, badMsg, empty, true, myTr, dp, log)
	if ps.pendingRekey != nil {
		t.Fatal("the failed exchange is still outstanding")
	}
	if !ps.childRekeyHoldUntil.IsZero() && time.Now().Before(ps.childRekeyHoldUntil) {
		t.Fatal("an ordinary rekey failure armed the TEMPORARY_FAILURE hold; the hold must " +
			"be keyed to the notify")
	}
	sa.releaseRequestWindow()
	ps.startChildRekey(sa, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("a rekey that failed without a TEMPORARY_FAILURE notify was not retried")
	}
	if ps.pendingRekey != nil {
		ps.pendingRekey.clear()
	}
}

// VALIDATES: the IKE SA rekey path holds after a TEMPORARY_FAILURE answer as well, and
// its hold is independent of the Child SA one.
// PREVENTS: the fix landing on one of the two identical rekey paths, leaving the other
// retrying once per second.
//
// RFC requirement: RFC7296-2.25-1 positive -- Section 2.25 names the operation, not the exchange
// kind, so the IKE SA rekey waits under the same rule. applyIKERekeyResponse (rekey.go)
// reads the notify and startIKERekey (established.go) refuses while the hold stands.
//
// RFC requirement: RFC7296-2.25-1 negative -- the two holds are separate. A held IKE SA rekey
// does not hold a Child SA rekey back, so one busy peer operation does not stop the
// other exchange from making progress.
func TestMidTemporaryFailureDefersTheIKERekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := sa.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA has no resolvable peer address")
	}

	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)

	ps.startIKERekey(sa, testIKEGroup(), myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the first IKE SA rekey request never reached the peer")
	}
	if ps.pendingRekey == nil || ps.pendingRekey.kind != rekeyIKE {
		t.Fatal("the first IKE SA rekey left no outstanding IKE rekey")
	}
	firstID := ps.pendingRekey.messageID

	respMsg := &wire.Message{Header: wire.Header{MessageID: firstID}}
	out := ps.handleCreateChildSAOwned(sa, respMsg, midTemporaryFailure(), true, myTr, dp, log)
	if out.newSA != nil {
		t.Fatal("a TEMPORARY_FAILURE answer produced a replacement IKE SA")
	}
	if ps.pendingRekey != nil {
		t.Fatal("the refused IKE rekey is still outstanding")
	}
	sa.releaseRequestWindow()

	// Positive. The IKE SA rekey is refused while its hold stands.
	ps.startIKERekey(sa, testIKEGroup(), myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an IKE SA rekey retried right after a TEMPORARY_FAILURE answer")
	if ps.pendingRekey != nil {
		t.Fatal("an IKE SA rekey started while its TEMPORARY_FAILURE hold stands")
	}

	// Negative. The Child SA rekey is not held by it, so the two waits are independent.
	ps.startChildRekey(sa, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("a held IKE SA rekey also stopped the Child SA rekey; the two holds must be separate")
	}
	if ps.pendingRekey != nil {
		ps.pendingRekey.clear()
	}
}

// VALIDATES: the responder's own request counter is set past the IKE_AUTH it answered
// through the same 32-bit ceiling the initiator uses, so it never wraps to 0.
// PREVENTS: `sa.NextMsgID = msgID + 1` written straight into the SA, which returns the
// counter to 0 for a peer whose final IKE_AUTH carries math.MaxUint32.
//
// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the whole of
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, to strengthen tagged coverage. This test is new. It
// adds proof for a producer that had none, and it removes none.
//
// The initiator advances its counter through advanceMsgID (fsm.go). That is where the
// ceiling lives. finishResponderEstablish (responder.go) is the responder's counterpart, and
// it wrote the counter directly. It was therefore the one path to a counter reset.
//
// Two facts bound the exposure, and neither makes the wrap safe. Only an authenticated peer
// reaches this producer, because IKE_AUTH is protected under the SK_* keys. And cacheResponse
// on the line above already marks the SA exhausted, so an SA that got there closed on the
// next tick. The window between the write and the tick is still a live SA whose next request
// carries an id it has already spent.
//
// RFC requirement: RFC7296-2.2-2 positive -- Section 2.2 requires the IKE SA to be closed or
// rekeyed once the Message ID no longer fits in 32 bits. The checklist row carries the
// sentence verbatim (rfc/full/rfc7296.txt:1437-1440). resumeRequestsAfter (msgid.go) freezes
// the counter at math.MaxUint32 and marks the SA exhausted, which is what closes it.
//
// RFC requirement: RFC7296-2.2-2 negative -- the freeze is scoped to the ceiling. An ordinary
// final IKE_AUTH at id 1 still leaves the counter at 2 and the SA usable. The frozen counter
// above is therefore the ceiling speaking, and not a producer that stopped setting it.
func TestMidResponderEstablishDoesNotWrapTheCounter(t *testing.T) {
	log := slogutil.DiscardLogger()

	// Negative control first. An ordinary IKE_AUTH leaves the counter one past the request
	// it answered, per Section 2.2's "the first pair of IKE_AUTH messages will have an ID
	// of 1, the second (when EAP is used) will be 2, and so on".
	ok := testSA()
	okPS := &PeerSession{peerName: ok.PeerName}
	okPS.finishResponderEstablish(ok, 1, []byte("cached response"), nil, nil, nil, log)
	if ok.NextMsgID != 2 {
		t.Errorf("NextMsgID = %d after the responder answered IKE_AUTH at id 1, want 2",
			ok.NextMsgID)
	}
	if ok.msgIDExhausted {
		t.Error("an SA established at id 1 is marked exhausted; the ceiling fired far below it")
	}
	if !ok.reserveRequestWindow() {
		t.Error("a freshly established SA cannot raise its first request")
	}

	// The ceiling. Section 2.2 leaves no id above math.MaxUint32.
	sa := testSA()
	ps := &PeerSession{peerName: sa.PeerName}
	ps.finishResponderEstablish(sa, math.MaxUint32, []byte("cached response"), nil, nil, nil, log)
	if sa.NextMsgID != math.MaxUint32 {
		t.Errorf("NextMsgID = %d after the responder answered at the last legal id, want %d; "+
			"a wrap spends an id this SA has already used under one set of keys, and "+
			"Section 2.2 calls the Message ID the replay protection",
			sa.NextMsgID, uint32(math.MaxUint32))
	}
	if !sa.msgIDExhausted {
		t.Fatal("the SA is not marked exhausted after the responder answered at the ceiling, " +
			"so nothing closes it")
	}
	if sa.reserveRequestWindow() {
		t.Error("an exhausted SA still reserved the request window; no later request may be " +
			"built on it")
	}
}
