package engine

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rtxArrive bounds a datagram that ze really sends. Loopback delivery is
// immediate, so a generous deadline never shortens the wait on a fast host.
const rtxArrive = 5 * time.Second

// rtxSentinel is the marker a test sends after an action that must stay quiet.
// One UDP socket keeps send order on loopback, so the sentinel arriving first
// proves the action wrote nothing. This replaces a wait for an absence, which
// a loaded host can defeat. The payload is longer than the 28-byte floor that
// UDPTransport.Run applies to inbound datagrams.
var rtxSentinel = []byte("ze-rfc7296-retransmit-sentinel-datagram-payload")

// rtxPeerLink builds a loopback stand-in for the peer. peerTr receives what ze
// sends and myTr is the socket ze sends from. The ze.test.ike.port seam points
// SA.remoteUDPAddr at peerTr, so the caller must set RemoteAddress to 127.0.0.1.
func rtxPeerLink(t *testing.T) (peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	log := slogutil.DiscardLogger()
	peerTr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("peer transport: %v", err)
	}
	t.Cleanup(func() { _ = peerTr.Close() })
	go peerTr.Run()

	myTr, err = transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("sender transport: %v", err)
	}
	t.Cleanup(func() { _ = myTr.Close() })

	addr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	port := addr.Port
	oldPortFn := ikeTestPortFn
	ikeTestPortFn = func() string { return strconv.Itoa(port) }
	t.Cleanup(func() { ikeTestPortFn = oldPortFn })
	return peerTr, myTr
}

// rtxRecv returns the next datagram the peer reads, or nil when none arrives
// inside rtxArrive.
func rtxRecv(t *testing.T, peerTr *transport.UDPTransport) []byte {
	t.Helper()
	select {
	case pkt := <-peerTr.Recv():
		return pkt.Data
	case <-time.After(rtxArrive):
		return nil
	}
}

// rtxExpectSilence proves the action just performed wrote no datagram. It sends
// the sentinel and requires the sentinel to be the next datagram the peer reads.
func rtxExpectSilence(t *testing.T, peerTr, myTr *transport.UDPTransport, remote *net.UDPAddr, what string) {
	t.Helper()
	if err := myTr.Send(rtxSentinel, remote); err != nil {
		t.Fatalf("send sentinel: %v", err)
	}
	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatalf("%s: the sentinel never arrived", what)
	}
	if !bytes.Equal(got, rtxSentinel) {
		t.Fatalf("%s: ze wrote %d unexpected bytes before the sentinel", what, len(got))
	}
}

// rtxExpectNoAcknowledgement proves the action just performed ACKNOWLEDGED nothing.
//
// It is the correct assertion for a request Ze refused, and rtxExpectSilence is not.
// RFC 7296 Section 2.3 requires that "the invalid request MUST NOT be acknowledged".
// It also requires that the INVALID_MESSAGE_ID notification
// "MUST NOT be sent in a response". Neither sentence requires Ze to write nothing. The
// same paragraph tells Ze to
// "inform the other side by initiating an INFORMATIONAL exchange". That is a new
// REQUEST, and it carries a Message ID of its own (notify_invalid_msgid.go).
//
// So a datagram is allowed here, and an ACKNOWLEDGEMENT is not. The Response flag is
// what separates them, and every datagram that arrives before the sentinel is checked
// for it. An acknowledgement is by definition a message marked as a response. The cached
// response carries that flag too, so this one test also proves the refused request drew
// no replay of it.
//
// the Message ID comparison this helper carried for one revision was a FALSE
// POSITIVE, and it never shipped. RFC 7296 Section 2.2 gives each direction its own
// sequence. Ze's own next request id and the peer's refused request id are therefore
// independent counters, and they CAN hold the same value. They do exactly that
// in TestRtxResponderIgnoresRequestWithForgottenResponse, where the responder's outbound
// counter still reads 2 while the forgotten peer request is also id 2.
//
// The Response flag below is the sound discriminator. It also subsumes the case the id
// check reached for. A response that names the refused request fails on the flag first.
//
// This remains strictly stronger than the silence it replaces on the property that
// matters. Silence passed whenever Ze wrote nothing, including for the wrong reason.
// This fails on any response at all.
func rtxExpectNoAcknowledgement(t *testing.T, peerTr, myTr *transport.UDPTransport, remote *net.UDPAddr, what string) {
	t.Helper()
	for _, raw := range imiDrain(t, peerTr, myTr, remote, what) {
		hdr := parseMsg(t, raw).Header
		if hdr.Flags&wire.FlagResponse != 0 {
			t.Errorf("%s: ze answered with a response at Message ID %d; RFC 7296 Section 2.3 forbids acknowledging it",
				what, hdr.MessageID)
		}
	}
}

// rtxIKEDelete builds the initiator's INFORMATIONAL request that deletes the IKE
// SA, at the given Message ID. The Delete gives the request a visible effect on
// the responder, so a second pass over it is detectable.
func rtxIKEDelete(t *testing.T, ini *SA, msgID uint32) []byte {
	t.Helper()
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}}
	raw, err := buildEncryptedMessageEx(ini, inner, msgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("build IKE delete request at id %d: %v", msgID, err)
	}
	return raw
}

// VALIDATES: each side stores the message it sent for as long as the exchange runs.
// RFC requirement: RFC7296-2.1-5 positive -- the initiator keeps its sent request in
// sa.LastSentMsg (fsm.go:491). The responder keeps its sent response in
// sa.lastResponse (msgid.go:88-92). It still answers a repeat of that request id.
// RFC requirement: RFC7296-2.1-5 negative -- a fresh SA that sent nothing holds neither
// memory. The stored bytes are state a send creates, not a default value.
func TestRtxEachSideRemembersWhatItSent(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	// Initiator half. LastSentMsg holds the IKE_AUTH request it sent, and it
	// survived the arrival of the matching response.
	if len(ini.LastSentMsg) == 0 {
		t.Fatal("initiator forgot the request it sent")
	}
	sent := parseMsg(t, ini.LastSentMsg)
	if sent.Header.ExchangeType != wire.ExchangeIKEAuth {
		t.Errorf("remembered exchange = %d, want IKE_AUTH", sent.Header.ExchangeType)
	}
	if sent.Header.MessageID != 1 {
		t.Errorf("remembered Message ID = %d, want 1", sent.Header.MessageID)
	}
	if sent.Header.Flags&wire.FlagResponse != 0 {
		t.Error("the initiator remembered a response, not the request it sent")
	}

	// Responder half. lastResponse holds the IKE_AUTH response it sent, and the
	// request id it answers is still classified as a retransmission.
	if !resp.lastResponseSet || resp.lastResponseID != 1 {
		t.Fatalf("responder response memory = set:%v id:%d, want set:true id:1",
			resp.lastResponseSet, resp.lastResponseID)
	}
	if got := classifyInbound(resp, 1, false, nil); got != inboundRetransmit {
		t.Errorf("classifyInbound(id 1) = %d, want inboundRetransmit", got)
	}
	remembered := parseMsg(t, resp.lastResponse)
	inner, err := decryptAndParse(ini, remembered, resp.lastResponse)
	if err != nil {
		t.Fatalf("the remembered response did not decrypt on the peer: %v", err)
	}
	sawAuth := false
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadAUTH); ok {
			sawAuth = true
		}
	}
	if !sawAuth {
		t.Error("the remembered response is not the IKE_AUTH response that was sent")
	}

	// Negative. An SA that sent nothing remembers nothing.
	fresh := testSA()
	if len(fresh.LastSentMsg) != 0 || fresh.lastResponseSet {
		t.Fatal("a fresh SA already holds a remembered message")
	}
	if got := classifyInbound(fresh, 1, false, nil); got != inboundInvalid {
		t.Errorf("classifyInbound on a fresh SA = %d, want inboundInvalid", got)
	}
}

// VALIDATES: the responder answers a duplicate request from cache, and nothing else.
// RFC requirement: RFC7296-2.1-3 positive -- a duplicate draws the cached response back byte
// for byte (inbound.go:47-48). A rebuild cannot match it, because every build draws
// a fresh random CBC IV (auth.go:553-554).
// RFC requirement: RFC7296-2.1-3 negative -- an out-of-window request draws no datagram, so
// the responder resends for a duplicate alone.
// RFC requirement: RFC7296-2.1-4 positive -- the duplicate returns at inbound.go:50 and the
// engine never decrypts it, so its IKE Delete does not kill the SA twice.
// RFC requirement: RFC7296-2.1-4 negative -- the same bytes on first delivery DO kill the SA,
// so the quiet second pass comes from the duplicate path.
func TestRtxResponderReplaysCachedResponseOnlyForDuplicate(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	resp.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := resp.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the responder has no resolvable peer address")
	}

	// Message ID 2 is the next request the responder expects after IKE_AUTH.
	req := rtxIKEDelete(t, ini, 2)

	first := ps.handleOwnedInbound(resp, transport.Packet{Data: req}, myTr, nil, log)
	if !first.peerAlive {
		t.Fatal("the first delivery never reached the INFORMATIONAL handler")
	}
	if resp.State != StateDead {
		t.Fatalf("first delivery: state = %v, want dead because the Delete ran", resp.State)
	}
	answer := rtxRecv(t, peerTr)
	if answer == nil {
		t.Fatal("the responder sent no answer to a new request")
	}

	// Undo the effect of the Delete. A duplicate must not produce it again.
	resp.State = StateEstablished

	second := ps.handleOwnedInbound(resp, transport.Packet{Data: req}, myTr, nil, log)
	if second.peerAlive {
		t.Error("the duplicate request reached the INFORMATIONAL handler")
	}
	if resp.State != StateEstablished {
		t.Error("the duplicate request ran the Delete a second time")
	}
	replay := rtxRecv(t, peerTr)
	if replay == nil {
		t.Fatal("the duplicate request drew no answer")
	}
	if !bytes.Equal(replay, answer) {
		t.Fatal("the replayed answer differs from the cached answer, so it was rebuilt")
	}

	// Negative. A request outside the window is not a duplicate.
	other := rtxIKEDelete(t, ini, 99)
	out := ps.handleOwnedInbound(resp, transport.Packet{Data: other}, myTr, nil, log)
	if out.peerAlive {
		t.Error("an out-of-window request reached the INFORMATIONAL handler")
	}
	if resp.State != StateEstablished {
		t.Error("an out-of-window request ran the Delete")
	}
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only.
	// An authenticated out-of-window request now draws an INVALID_MESSAGE_ID.
	// RFC 7296 Section 2.3 raises that as a new REQUEST.
	// The old rtxExpectSilence asserted more than the RFC does.
	// The replacement still forbids every response.
	// The cached response carries that flag, so no replay of it can pass either.
	rtxExpectNoAcknowledgement(t, peerTr, myTr, remote, "out-of-window request")
}

// VALIDATES: a request whose response the responder no longer holds is dropped in full.
// RFC requirement: RFC7296-2.1-6 positive -- once the cache moves to a newer request id, the
// older request classifies as invalid (msgid.go:79-82). It draws no answer and its
// IKE Delete payload never runs.
// RFC requirement: RFC7296-2.1-6 negative -- that same request drew an answer while its
// response was still held, so the silence comes from the forgotten response.
func TestRtxResponderIgnoresRequestWithForgottenResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	resp.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := resp.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the responder has no resolvable peer address")
	}

	// Request A at id 2 is answered, so its response is held.
	reqA := rtxIKEDelete(t, ini, 2)
	if out := ps.handleOwnedInbound(resp, transport.Packet{Data: reqA}, myTr, nil, log); !out.peerAlive {
		t.Fatal("request A never reached the INFORMATIONAL handler")
	}
	if resp.State != StateDead {
		t.Fatal("request A did not run its Delete, so the later comparison proves nothing")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("request A drew no answer while its response was held")
	}
	resp.State = StateEstablished

	// Request B at id 3 replaces the cached response, so A is now forgotten.
	reqB, err := buildEncryptedMessageEx(ini, nil, 3, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("build probe request: %v", err)
	}
	if out := ps.handleOwnedInbound(resp, transport.Packet{Data: reqB}, myTr, nil, log); !out.peerAlive {
		t.Fatal("request B never reached the INFORMATIONAL handler")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("request B drew no answer")
	}
	if resp.lastResponseID != 3 {
		t.Fatalf("cached response id = %d, want 3 so that request A is forgotten", resp.lastResponseID)
	}

	// Positive. Request A now draws nothing and changes nothing.
	out := ps.handleOwnedInbound(resp, transport.Packet{Data: reqA}, myTr, nil, log)
	if out.peerAlive {
		t.Error("the forgotten request reached the INFORMATIONAL handler")
	}
	if resp.State != StateEstablished {
		t.Error("the forgotten request ran its Delete payload")
	}
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only.
	// The forgotten request is out of window, so it now draws an INVALID_MESSAGE_ID.
	// RFC7296-2.1-6 claims it "draws no answer".
	// The replacement asserts exactly that: no datagram marked as a response.
	//
	// This site is also why the helper cannot test the Message ID.
	// The responder's own outbound counter still reads 2 here.
	// That is the id of the forgotten peer request too (RFC 7296 Section 2.2).
	rtxExpectNoAcknowledgement(t, peerTr, myTr, remote, "request with a forgotten response")
}

// VALIDATES: an unanswered post-establishment request is resent until the session stops.
// RFC requirement: RFC7296-2.1-7 positive -- every expiry of the wait resends the request
// (established.go:281-283). At the cap the session declares the IKE SA failed and
// returns errTimeout (established.go:275-279).
// RFC requirement: RFC7296-2.1-7 negative -- a call before the wait expires sends nothing, so
// each resend answers a real timeout and is not an unconditional write.
func TestRtxInitiatorResendsUnansweredRekeyRequest(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	req, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	ps.pendingRekey = pending

	// The owner loop sends the built request once (established.go:234).
	sendRaw(ini, myTr, req, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the original rekey request never reached the peer")
	}

	// Negative. The wait has not expired, so nothing goes out.
	early := pending.sentAt.Add(rekeyRetransmitTimeout - time.Second)
	if err := ps.serviceRekeyRetransmit(ini, myTr, early, nil, nil, log); err != nil {
		t.Fatalf("an early service call returned %v, want nil", err)
	}
	if pending.retransmits != 0 {
		t.Fatalf("retransmits = %d after an early call, want 0", pending.retransmits)
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "rekey wait not yet expired")

	// Positive. Each expiry resends, up to the cap.
	for i := 1; i <= maxRetransmissions; i++ {
		due := pending.sentAt.Add(rekeyRetransmitTimeout)
		if err := ps.serviceRekeyRetransmit(ini, myTr, due, nil, nil, log); err != nil {
			t.Fatalf("resend %d returned %v, want nil", i, err)
		}
		if pending.retransmits != i {
			t.Fatalf("retransmits = %d after resend %d", pending.retransmits, i)
		}
		if rtxRecv(t, peerTr) == nil {
			t.Fatalf("resend %d wrote no datagram", i)
		}
	}

	// The cap is reached, so the session declares the IKE SA failed.
	due := pending.sentAt.Add(rekeyRetransmitTimeout)
	if err := ps.serviceRekeyRetransmit(ini, myTr, due, nil, nil, log); !errors.Is(err, errTimeout) {
		t.Fatalf("the exhausted rekey returned %v, want errTimeout", err)
	}
	if ps.pendingRekey != nil {
		t.Error("the failed exchange is still outstanding after the timeout")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "rekey declared failed")
}

// VALIDATES: the IKE_SA_INIT handshake also resends an unanswered request.
// RFC requirement: RFC7296-2.1-7 positive -- the handshake is the second producer of this
// rule. runInitiator resends sa.LastSentMsg when no response arrives in time
// (fsm.go:131-142), against a peer that stays silent here.
// RFC requirement: RFC7296-2.1-7 negative -- the resent datagram equals the first and keeps
// Message ID 0, so it is a retransmission and not a second fresh exchange.
func TestRtxInitiatorResendsUnansweredSAInit(t *testing.T) {
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)

	peer := testPeer()
	peer.LocalAddress = "127.0.0.1"
	peer.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{
		peerName: "ze",
		peerCfg:  peer,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
		stopCh:   make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		_ = ps.runInitiator(peer, testIKEGroup(), NewSATable(), myTr, nil, log)
		close(done)
	}()

	first := rtxRecv(t, peerTr)
	second := rtxRecv(t, peerTr)
	close(ps.stopCh)
	// Wait for the goroutine before Cleanup restores the port seam it reads.
	<-done

	if first == nil {
		t.Fatal("the initiator sent no IKE_SA_INIT request")
	}
	if second == nil {
		t.Fatal("the initiator never resent the unanswered IKE_SA_INIT request")
	}
	hdr := parseMsg(t, second).Header
	if hdr.ExchangeType != wire.ExchangeIKESAInit {
		t.Errorf("resent exchange = %d, want IKE_SA_INIT", hdr.ExchangeType)
	}
	if hdr.MessageID != 0 {
		t.Errorf("resent Message ID = %d, want 0", hdr.MessageID)
	}
	if !bytes.Equal(first, second) {
		t.Error("the resent request differs from the first, so it is a new exchange")
	}
}

// VALIDATES: a retransmission is a replay of the stored bytes, header included.
// RFC requirement: RFC7296-2.1-8 positive -- the resend writes the stored request bytes
// (established.go:283), so the peer reads a datagram equal to the original.
// RFC requirement: RFC7296-2.1-8 negative -- a rebuilt request differs, because every build
// draws a fresh random CBC IV (auth.go:553-554). Equality belongs to the replay.
// RFC requirement: RFC7296-2.2-1 positive -- one producer serves both rules, because the
// stored bytes carry the original header and the Message ID cannot differ.
// RFC requirement: RFC7296-2.2-1 negative -- a rebuilt request takes the next Message ID
// (rekey.go:323-329), so the reuse belongs to the replay and not to a fixed id.
func TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"

	req, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	ps.pendingRekey = pending

	sendRaw(ini, myTr, req, log)
	original := rtxRecv(t, peerTr)
	if original == nil {
		t.Fatal("the original rekey request never reached the peer")
	}

	due := pending.sentAt.Add(rekeyRetransmitTimeout)
	if err := ps.serviceRekeyRetransmit(ini, myTr, due, nil, nil, log); err != nil {
		t.Fatalf("serviceRekeyRetransmit: %v", err)
	}
	resent := rtxRecv(t, peerTr)
	if resent == nil {
		t.Fatal("the rekey request was never resent")
	}
	// The Message ID is read first so that it stays an observable check of its
	// own. A byte comparison alone would hide it behind an earlier failure.
	if got, want := parseMsg(t, resent).Header.MessageID, parseMsg(t, original).Header.MessageID; got != want {
		t.Fatalf("retransmitted Message ID = %d, want %d", got, want)
	}
	if !bytes.Equal(resent, original) {
		t.Fatal("the retransmission is not bitwise identical to the original request")
	}

	// Negative. A rebuild of the same logical request matches neither the bytes
	// nor the Message ID, so both claims above test the replay path.
	rebuilt, pending2, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("rebuild rekey request: %v", err)
	}
	defer pending2.clear()
	if bytes.Equal(rebuilt, req) {
		t.Fatal("a rebuilt request matched the original, so byte equality proves nothing")
	}
	if pending2.messageID == pending.messageID {
		t.Fatalf("a rebuilt request reused Message ID %d, so id equality proves nothing", pending.messageID)
	}
}

// VALIDATES: both halves of an IKE SA rekey return an SA whose counters are zero.
// RFC requirement: RFC7296-2.18-2 positive -- two producers start the new IKE SA at zero.
// They are applyIKERekeyResponse (rekey.go:404-405) on the rekey initiator and
// respondIKERekey (rekey.go:512-513) on the rekey responder.
// RFC requirement: RFC7296-2.18-2 negative -- the old SAs hold non-zero counters and the
// rekey runs on a non-zero Message ID. Zero is therefore a reset, not a copy.
func TestRtxRekeyedIKESAResetsMessageCounters(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)

	req, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()

	// Negative. The old state is not already zero, so a copy would be visible.
	if pending.messageID == 0 {
		t.Fatal("the rekey ran on Message ID 0, so a copy of the old counter looks like a reset")
	}
	if ini.NextMsgID == 0 || resp.ExpectedMsgID == 0 {
		t.Fatalf("old counters = ini:%d resp:%d, want both non-zero", ini.NextMsgID, resp.ExpectedMsgID)
	}

	reqInner, err := decryptAndParse(resp, parseMsg(t, req), req)
	if err != nil {
		t.Fatalf("responder decrypt of the rekey request: %v", err)
	}
	respBytes, newResp, err := respondIKERekey(resp, reqInner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	respInner, err := decryptAndParse(ini, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("initiator decrypt of the rekey response: %v", err)
	}
	newIni, err := applyIKERekeyResponse(ini, pending, respInner, log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}

	if newIni.NextMsgID != 0 || newIni.ExpectedMsgID != 0 {
		t.Errorf("rekey initiator new SA counters = next:%d expected:%d, want 0 and 0",
			newIni.NextMsgID, newIni.ExpectedMsgID)
	}
	if newResp.NextMsgID != 0 || newResp.ExpectedMsgID != 0 {
		t.Errorf("rekey responder new SA counters = next:%d expected:%d, want 0 and 0",
			newResp.NextMsgID, newResp.ExpectedMsgID)
	}
}
