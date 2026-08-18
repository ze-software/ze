package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestDPDSendReceive(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 30,
		Timeout:  90,
		Action:   ipsec.DPDActionRestart,
	})
	if dpd == nil {
		t.Fatal("newDPDState returned nil")
	}

	now := time.Now()
	if dpd.shouldSend(now) {
		t.Error("should not send immediately after creation")
	}

	dpd.lastSent = now.Add(-31 * time.Second)
	if !dpd.shouldSend(now) {
		t.Error("should send after interval elapsed")
	}

	// A real transport, because a probe is awaited only once its datagram is written
	// (sendDPD, dpd.go). The nil transport this used to pass now returns early.
	sa, _, _, _, myTr := dpdProbeLink(t)
	sa.NextMsgID = 1
	sendDPD(sa, myTr, dpd, slogutil.DiscardLogger())

	if !dpd.awaitReply {
		t.Error("awaitReply should be true after send")
	}
	if dpd.shouldSend(now) {
		t.Error("should not send while awaiting reply")
	}

	handleDPDResponse(dpd, slogutil.DiscardLogger(), "test-peer")
	if dpd.awaitReply {
		t.Error("awaitReply should be false after response")
	}
}

func TestDPDTimeout(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 10,
		Timeout:  30,
		Action:   ipsec.DPDActionClear,
	})

	now := time.Now()
	dpd.sentAt = now.Add(-31 * time.Second)
	dpd.awaitReply = true

	if !dpd.timedOut(now) {
		t.Error("should be timed out after timeout period")
	}

	if dpd.action != ipsec.DPDActionClear {
		t.Errorf("action = %v, want clear", dpd.action)
	}
}

func TestDPDDisabled(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{Interval: 0})
	if dpd != nil {
		t.Error("DPD with interval 0 should return nil")
	}
}

func TestDPDNotTimedOutBeforeTimeout(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 10,
		Timeout:  60,
		Action:   ipsec.DPDActionRestart,
	})

	now := time.Now()
	dpd.sentAt = now.Add(-30 * time.Second)
	dpd.awaitReply = true

	if dpd.timedOut(now) {
		t.Error("should not be timed out before timeout period")
	}
}

// VALIDATES: a liveness probe that cannot be written claims nothing. sendDPD on an SA
// with no send path takes no request window, spends no Message ID, and awaits nothing.
// The SA here holds no socket of its own, so a nil fallback leaves sendPath (sa.go)
// with nothing to answer. TestDPDFloatedSAProbesWithoutTheFallback is the other side of
// that predicate: a nil fallback alone never stops a probe.
// PREVENTS: the request window a probe reserved and no path released. RFC 7296 Section
// 2.3 allows one outstanding request per IKE SA, so a held window blocks the rekey and
// the Delete behind it. serviceRequestWindow (established.go) returns early while a
// probe awaits its reply, and shouldRetransmit finds no datagram to repeat, so the one
// exit left was the dead-peer verdict of Section 2.4 after zero attempts.
//
// RFC requirement: RFC7296-2.4-11 negative -- the verdict must follow repeated attempts,
// so the state that leads to it is not entered for a probe no send path ever wrote.
// RFC requirement: RFC7296-2.3-2 negative -- the one request window is not spent by a
// request that was never raised.
func TestDPDNoTransportTakesNoWindow(t *testing.T) {
	ini, _, _, peerTr, myTr := dpdProbeLink(t)
	log := slogutil.DiscardLogger()

	dpd := winDueDPD()
	firstID := ini.NextMsgID
	lastSent := dpd.lastSent

	sendDPD(ini, nil, dpd, log)

	if ini.requestOutstanding {
		t.Error("a probe that was never built holds the one request window")
	}
	if dpd.awaitingReply() {
		t.Error("a probe that was never built is awaited")
	}
	if len(dpd.probeMsg) != 0 {
		t.Errorf("the state kept %d bytes of a probe that was never built", len(dpd.probeMsg))
	}
	if ini.NextMsgID != firstID {
		t.Errorf("Message ID moved to %d, want %d: no request was sent", ini.NextMsgID, firstID)
	}
	if !dpd.lastSent.Equal(lastSent) {
		t.Error("the probe clock advanced for a probe that was never built")
	}

	// The control. The same call with a transport does write the probe and does take
	// the window, so the assertions above measure the missing transport and not a
	// sendDPD that never does anything.
	sendDPD(ini, myTr, dpd, log)

	if raw := rtxRecv(t, peerTr); raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	if !ini.requestOutstanding {
		t.Error("the written probe holds no request window")
	}
	if !dpd.awaitingReply() {
		t.Error("the written probe is not awaited")
	}
	if len(dpd.probeMsg) == 0 {
		t.Error("the written probe kept no datagram to retransmit")
	}
	if ini.NextMsgID != firstID+1 {
		t.Errorf("Message ID = %d after one probe, want %d", ini.NextMsgID, firstID+1)
	}
}

// VALIDATES: a floated SA raises its liveness probe with a nil fallback transport.
// sendDPD asks the SA for its send path, and RFC 7296 Section 2.23 makes that the NAT-T
// socket once a NAT is discovered, whatever the caller passes.
// PREVENTS: a guard written on the fallback argument instead of the send path. The
// session hands maintainSA the port-500 transport, which is nil when that socket failed
// to open, while the floated SA sends from its own nattSocket. Reading the argument
// would stop every probe for the life of a working NAT-traversing tunnel, and RFC 7296
// Section 2.4 asks liveness checks to prevent exactly that black hole.
func TestDPDFloatedSAProbesWithoutTheFallback(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, resp, _ := establishPSK(t)
	peerTr, ikeTr, nattTr := nttNATTLink(t)
	resp.bindSockets(ikeTr, nattTr)
	resp.floatToNATTPort()
	resp.peerEndpoint = nttPeerAddr(t, peerTr)

	dpd := winDueDPD()
	firstID := resp.NextMsgID

	// Nil, as maintainSA passes it when the port-500 socket is absent.
	sendDPD(resp, nil, dpd, log)

	gotPort, gotData := nttSourcePortOf(t, peerTr)
	if want := nttPort(t, nattTr); gotPort != want {
		t.Errorf("the probe left from port %d, want the NAT-T socket %d", gotPort, want)
	}
	if len(gotData) == 0 {
		t.Error("the probe carried no bytes")
	}
	if !resp.requestOutstanding {
		t.Error("the probe holds no request window")
	}
	if !dpd.awaitingReply() {
		t.Error("the probe is not awaited, so no answer can clear it")
	}
	if len(dpd.probeMsg) == 0 {
		t.Error("the probe kept no datagram to retransmit")
	}
	if resp.NextMsgID != firstID+1 {
		t.Errorf("Message ID = %d after one probe, want %d", resp.NextMsgID, firstID+1)
	}
}

func TestDPDNextDeadline(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 30,
		Timeout:  90,
		Action:   ipsec.DPDActionRestart,
	})

	now := time.Now()
	dpd.lastSent = now

	deadline := dpd.nextDeadline()
	expected := now.Add(30 * time.Second)
	if deadline.Sub(expected) > time.Millisecond {
		t.Errorf("nextDeadline = %v, want ~%v", deadline, expected)
	}

	dpd.awaitReply = true
	dpd.sentAt = now
	deadline = dpd.nextDeadline()
	expected = now.Add(90 * time.Second)
	if deadline.Sub(expected) > time.Millisecond {
		t.Errorf("nextDeadline (await) = %v, want ~%v", deadline, expected)
	}
}

// VALIDATES: the dead-peer verdict follows a REPEAT that went unanswered, not the
// first probe. maintainSA (established.go) reads timedOut before shouldRetransmit in
// the same tick. With the smallest timeout parseDPD accepts, the first tick lands
// past the whole budget with zero repeats behind it.
// PREVENTS: `dead-peer-detection timeout 1` tearing down a live tunnel on one lost
// datagram. The budget alone was the verdict. It is now the budget AND a repeat.
//
// RFC requirement: RFC7296-2.4-11 negative -- one unanswered attempt is not "repeated
// attempts", so it does not reach the verdict however far past the budget it sits.
// RFC requirement: RFC7296-2.4-11 positive -- a repeat that also goes unanswered for
// the timeout period does reach it.
func TestDPDVerdictNeedsARepeatedAttempt(t *testing.T) {
	ini, _, _, peerTr, myTr := dpdProbeLink(t)
	log := slogutil.DiscardLogger()

	dpd := winDueDPD()
	dpd.timeout = time.Second // the smallest value parseDPD accepts

	sendDPD(ini, myTr, dpd, log)
	if raw := rtxRecv(t, peerTr); raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	if dpd.retries != 0 {
		t.Fatalf("retries = %d before any repeat, want 0", dpd.retries)
	}

	// The moment maintainSA ticks, past the whole liveness budget.
	past := dpd.sentAt.Add(2 * time.Second)

	if dpd.timedOut(past) {
		t.Error("the peer was declared dead on one unanswered attempt")
	}
	if !dpd.shouldRetransmit(past) {
		t.Fatal("the probe was never repeated, so no repeat can go unanswered")
	}

	retransmitDPD(ini, myTr, dpd, past, log)
	if raw := rtxRecv(t, peerTr); raw == nil {
		t.Fatal("the repeat never reached the peer")
	}
	if dpd.retries != 1 {
		t.Fatalf("retries = %d after one repeat, want 1", dpd.retries)
	}
	if !dpd.timedOut(past) {
		t.Error("a repeat went unanswered past the budget and the peer survived it")
	}
}

// VALIDATES: an awaited probe with no stored datagram still ends. Waiting for a
// repeat that can never be made would hold the SA, and the one request window it
// took, open for ever.
// PREVENTS: the repeat requirement above turning a bug state into a permanent one.
//
// RFC requirement: RFC7296-2.4-11 positive -- the timeout period ends a probe whose
// repeat is impossible, because no further attempt can be made to contact the peer.
func TestDPDVerdictEndsAProbeThatCannotBeRepeated(t *testing.T) {
	dpd := winDueDPD()
	dpd.timeout = time.Second
	dpd.awaitReply = true
	dpd.sentAt = time.Now().Add(-2 * time.Second)
	dpd.lastAttempt = dpd.sentAt

	if len(dpd.probeMsg) != 0 {
		t.Fatal("the fixture stored a datagram, so it does not measure an unrepeatable probe")
	}
	if dpd.shouldRetransmit(time.Now()) {
		t.Fatal("a probe with no datagram was offered for repeat")
	}
	if !dpd.timedOut(time.Now()) {
		t.Error("a probe that can never be repeated holds the SA open for ever")
	}
}
