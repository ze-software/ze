// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the handshake, the initiator's half
// Related: fsm.go -- handleAuthResponse and handleEAPResponse, the producers these tests hold, and runInitiator's retransmit loop
// Related: rfc7296_unprotected_handshake_test.go -- unprotTampered and unprotFinish, the forgery and the EAP driver reused here
// Related: rfc4739_rounds_test.go -- maSAInitPair, the Ze-to-Ze pre-shared-secret handshake
// Related: rfc3748_eap_length_test.go -- eaplenExchange, the real EAP-MSCHAPv2 handshake
// Related: initiator_cycle_test.go -- icyParse, and the afterFunc seam that drives runInitiator
// Related: rfc7296_reauth_receive_test.go -- heldInitiatorSuccessor, the held reauthentication successor
// Related: held_successor.go -- handleHeldSuccessorInbound, the held successor's drop site

package engine

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// unprotRespPeerName is the session name TestInitiatorKeepsRetransmittingAfterUnprotectedResponse
// registers, unique so no other test's session in activePeersMap is replaced or found.
const unprotRespPeerName = "unprotected-response-peer"

// unprotRespStepsMax bounds how often the afterFunc stub of the retransmit test may run.
// The driven cycle asks for four waits; the bound only stops a regression that loops.
const unprotRespStepsMax = 16

// VALIDATES: an IKE_AUTH response that fails its cryptographic check leaves the
// initiator's handshake as it was, draws no datagram, and the responder's genuine
// response then establishes the same IKE SA.
//
// METHOD: a Ze-to-Ze pre-shared-secret handshake reaches the initiator's
// StateAuthSent, and the responder answers the IKE_AUTH request. The genuine response
// is held back; a copy with its checksum's last octet flipped reaches the initiator
// first, through handleInbound, the dispatch goroutine's entry point. The genuine
// response is then delivered.
//
// PREVENTS: handleAuthResponse setting StateDead on any decryptSKPayload error, so
// one datagram built from an observed header ended the initiator's handshake and
// runInitiator returned errSADead into a reconnect backoff.
//
// RFC requirement: RFC7296-2.4-3 negative -- an IKE_AUTH response whose checksum fails leaves the initiator in StateAuthSent with the same next Message ID, retransmit time, retransmit count and last sent message, draws no datagram, writes no WARN line and counts one handshake-unauthenticated drop, and the genuine response then establishes the IKE SA under the same SPI pair.
func TestInitiatorAuthSurvivesUnprotectedIKEAuthResponse(t *testing.T) {
	p := maSAInitPair(t, maAuth(nil, nil), maAuth(nil, nil))
	authReq := bytes.Clone(p.ini.LastSentMsg)
	p.respondWith(authReq, false)
	if p.resp.State != StateEstablished {
		t.Fatalf("responder state after the IKE_AUTH request = %v, want established", p.resp.State)
	}
	genuine := p.resp.LastSentMsg
	if p.ini.State != StateAuthSent {
		t.Fatalf("initiator state before the forgery = %v, want IKE_AUTH sent", p.ini.State)
	}
	initiatorSPI, responderSPI := p.ini.InitiatorSPI, p.ini.ResponderSPI
	nextMsgID := p.ini.NextMsgID
	retransmitTime := p.ini.RetransmitTime
	retransmitCount := p.ini.RetransmitCount
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
	p.logs.Reset()

	handleInbound(p.ini, transport.Packet{Data: unprotTampered(genuine), RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != StateAuthSent {
		t.Fatalf("initiator state after the forgery = %v, want IKE_AUTH sent", p.ini.State)
	}
	if p.ini.NextMsgID != nextMsgID {
		t.Fatalf("the forgery moved the next Message ID from %d to %d", nextMsgID, p.ini.NextMsgID)
	}
	if !p.ini.RetransmitTime.Equal(retransmitTime) {
		t.Fatalf("the forgery moved the retransmit time from %v to %v", retransmitTime, p.ini.RetransmitTime)
	}
	if p.ini.RetransmitCount != retransmitCount {
		t.Fatalf("the forgery moved the retransmit count from %d to %d", retransmitCount, p.ini.RetransmitCount)
	}
	if !bytes.Equal(p.ini.LastSentMsg, authReq) {
		t.Fatal("the forgery replaced the IKE_AUTH request the initiator retransmits")
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 1 {
		t.Fatalf("%d drops counted under %q for one forgery, want 1", got, suppressedUnauthenticatedHandshake)
	}
	if strings.Contains(p.logs.String(), "level=WARN") {
		t.Fatalf("a forged IKE_AUTH response wrote a WARN line:\n%s", p.logs.String())
	}
	rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "an unprotected IKE_AUTH response")

	handleInbound(p.ini, transport.Packet{Data: genuine, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != StateEstablished {
		t.Fatalf("the genuine response after the forgery left the initiator %v, want established", p.ini.State)
	}
	if p.ini.InitiatorSPI != initiatorSPI || p.ini.ResponderSPI != responderSPI {
		t.Fatal("the IKE SA that established is not the one the forgery was aimed at")
	}
	if p.ini.InitiatorSPI != p.resp.InitiatorSPI || p.ini.ResponderSPI != p.resp.ResponderSPI {
		t.Fatal("the initiator established under an SPI pair the responder does not hold")
	}
}

// VALIDATES: an IKE_AUTH response that fails its cryptographic check in the middle of
// an EAP exchange leaves the initiator's EAP state as it was, draws no datagram, and
// the authenticator's genuine round then carries the exchange to an established IKE SA.
//
// METHOD: a real EAP-MSCHAPv2 exchange between two Ze endpoints reaches the
// initiator's StateEAPInProgress, and the responder answers the initiator's round-2
// request. That answer is held back; a copy with its checksum's last octet flipped
// reaches the initiator first, through handleInbound. For that one delivery the
// initiator's remote address is pointed at the peer transport, so a datagram its send
// path wrote would arrive before the sentinel. The held-back answer then drives the
// exchange to the end.
//
// PREVENTS: handleEAPResponse setting StateDead on any decryptSKPayload error, so one
// datagram built from an observed header ended an EAP handshake the authenticator was
// still running.
//
// RFC requirement: RFC7296-2.4-3 negative -- an IKE_AUTH response whose checksum fails, mid-EAP, leaves the initiator in StateEAPInProgress with the same next Message ID, retransmit time, retransmit count and last sent message, draws no datagram and counts one handshake-unauthenticated drop, and the authenticator's genuine round then drives the exchange to an established IKE SA on both sides under the same SPI pair.
func TestInitiatorEAPSurvivesUnprotectedIKEAuthResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, _, authReq := eaplenExchange(t)
	peerTr, myTr := rtxPeerLink(t, resp)
	peerAddr := eaprtxPeerAddr(t, peerTr)

	unprotDeliver(resp, authReq, table, myTr, peerAddr)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the first IKE_AUTH drew no EAP request")
	}
	handleInbound(ini, transport.Packet{Data: first}, table, nil, log)
	if ini.State != StateEAPInProgress {
		t.Fatalf("initiator state after the first EAP request = %v, want EAP in progress", ini.State)
	}
	round2 := bytes.Clone(ini.LastSentMsg)
	unprotDeliver(resp, round2, table, myTr, peerAddr)
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the round-2 request drew no answer from the authenticator")
	}

	initiatorSPI, responderSPI := ini.InitiatorSPI, ini.ResponderSPI
	nextMsgID := ini.NextMsgID
	retransmitTime := ini.RetransmitTime
	retransmitCount := ini.RetransmitCount
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
	configuredRemote := ini.PeerCfg.RemoteAddress
	ini.PeerCfg.RemoteAddress = "127.0.0.1"

	handleInbound(ini, transport.Packet{Data: unprotTampered(second), RemoteAddr: peerAddr}, table, myTr, log)

	if ini.State != StateEAPInProgress {
		t.Fatalf("initiator state after the forgery = %v, want EAP in progress", ini.State)
	}
	if ini.NextMsgID != nextMsgID {
		t.Fatalf("the forgery moved the next Message ID from %d to %d", nextMsgID, ini.NextMsgID)
	}
	if !ini.RetransmitTime.Equal(retransmitTime) {
		t.Fatalf("the forgery moved the retransmit time from %v to %v", retransmitTime, ini.RetransmitTime)
	}
	if ini.RetransmitCount != retransmitCount {
		t.Fatalf("the forgery moved the retransmit count from %d to %d", retransmitCount, ini.RetransmitCount)
	}
	if !bytes.Equal(ini.LastSentMsg, round2) {
		t.Fatal("the forgery replaced the EAP round the initiator retransmits")
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 1 {
		t.Fatalf("%d drops counted under %q for one forgery, want 1", got, suppressedUnauthenticatedHandshake)
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "an unprotected IKE_AUTH response mid-EAP")
	ini.PeerCfg.RemoteAddress = configuredRemote

	unprotFinish(t, ini, resp, table, second)
	if ini.InitiatorSPI != initiatorSPI || ini.ResponderSPI != responderSPI {
		t.Fatal("the IKE SA that established is not the one the forgery was aimed at")
	}
	if ini.InitiatorSPI != resp.InitiatorSPI || ini.ResponderSPI != resp.ResponderSPI {
		t.Fatal("the initiator established under an SPI pair the responder does not hold")
	}
}

// unprotRespObserved is what the afterFunc stub of the retransmit test saw on the
// session goroutine. The test goroutine reads it after runInitiator returned.
type unprotRespObserved struct {
	mu      sync.Mutex
	err     error  // the far end could not be driven; the test fails with it
	saInit  []byte // the IKE_SA_INIT request, as the initiator sent it
	authReq []byte // the IKE_AUTH request, as the initiator sent it

	// The handshake fields when the forgery arrived, and when the loop next waited.
	stateBefore, stateAfter SAState
	timeBefore, timeAfter   time.Time
	countBefore, countAfter int
	msgIDBefore, msgIDAfter uint32
	drops                   uint64 // handshake-unauthenticated drops the forgery added

	// The loop's state when it waited after its first retransmission.
	countRetransmitted int
	lastRetransmitted  []byte
	waitRetransmitted  time.Duration
}

// unprotRespRegister makes ps the running session for unprotRespPeerName, as reconcile
// does, so handleInbound wakes its handshake wait, and removes it when the test ends.
func unprotRespRegister(t *testing.T, ps *PeerSession) {
	t.Helper()
	peersMu.Lock()
	if activePeersMap == nil {
		activePeersMap = make(map[string]*PeerSession)
	}
	activePeersMap[unprotRespPeerName] = ps
	peersMu.Unlock()
	t.Cleanup(func() {
		peersMu.Lock()
		delete(activePeersMap, unprotRespPeerName)
		peersMu.Unlock()
	})
}

// VALIDATES: after an IKE_AUTH response that fails its cryptographic check, the
// initiator's handshake loop keeps its ordinary schedule: it waits until the retransmit
// time it already had, then resends the identical IKE_AUTH request octets, and waits the
// first backoff step. The forgery changes neither the schedule nor what is resent.
//
// METHOD: runInitiator runs against a far-end responder driven in process from the
// afterFunc seam, on the session goroutine, sending through a loopback transport the
// test reads. The first wait completes IKE_SA_INIT. At the second wait the far end
// answers the IKE_AUTH request, and a copy of its answer with the checksum's last octet
// flipped reaches the initiator through handleInbound, which wakes the loop. At the next
// wait the stub records the handshake fields and rewinds the retransmit time, standing
// in for the time passing, so the loop retransmits. At the wait after that it records
// the retransmission and stops the session. The test then reads the datagrams the
// initiator wrote.
//
// PREVENTS: a drop that stalls or spins the loop, refreshes its timer, or replaces the
// request it retransmits; and, today, handleAuthResponse setting StateDead, which makes
// runInitiator return errSADead on the pass the forgery woke.
func TestInitiatorKeepsRetransmittingAfterUnprotectedResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)
	peerAddr := eaprtxPeerAddr(t, peerTr)
	ikeGroup, espGroup := testIKEGroup(), testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "unprotected-response-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ps := &PeerSession{
		peerName:      unprotRespPeerName,
		peerCfg:       iniPeer,
		ikeGroup:      ikeGroup,
		espGroup:      espGroup,
		stopCh:        make(chan struct{}),
		handshakeWake: make(chan struct{}, 1),
		inbound:       make(chan transport.Packet, inboundQueueDepth),
		supersede:     make(chan struct{}, 1),
	}
	unprotRespRegister(t, ps)
	respPS := &PeerSession{peerName: unprotRespPeerName, peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	table := NewSATable()
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { close(ps.stopCh) }) }

	rec := &unprotRespObserved{}
	var far *SA
	steps := 0
	never := make(chan time.Time)
	fired := func() <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}
	// fail records why the far end could not be driven and ends the cycle.
	fail := func(err error) <-chan time.Time {
		rec.err = err
		stop()
		return never
	}

	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	// The stub runs on the session goroutine, inside awaitHandshake, so every SA field it
	// reads or writes belongs to the goroutine it runs on.
	afterFunc = func(d time.Duration) <-chan time.Time {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		steps++
		ini := ps.getSA()
		if steps > unprotRespStepsMax || ini == nil {
			return fail(errors.New("the handshake loop ran past the steps the test drives"))
		}
		switch steps {
		case 1:
			// IKE_SA_INIT: the far end answers, and the initiator sends its IKE_AUTH.
			rec.saInit = bytes.Clone(ini.LastSentMsg)
			sa, err := newResponderSA(unprotRespPeerName, respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
			if err != nil {
				return fail(err)
			}
			far = sa
			req, err := icyParse(ini.LastSentMsg)
			if err != nil {
				return fail(err)
			}
			handleSAInitRequest(far, req, ini.LastSentMsg, nil, nil, log)
			answer, err := icyParse(far.LastSentMsg)
			if err != nil {
				return fail(err)
			}
			handleSAInitResponse(ini, answer, far.LastSentMsg, table, nil, nil, log)
			return fired()
		case 2:
			// The far end answers the IKE_AUTH request; only a forgery of that answer
			// reaches the initiator.
			rec.authReq = bytes.Clone(ini.LastSentMsg)
			req, err := icyParse(ini.LastSentMsg)
			if err != nil {
				return fail(err)
			}
			respPS.handleAuthRequest(far, req, ini.LastSentMsg, nil, nil, log)
			if far.State != StateEstablished {
				return fail(errors.New("the far end refused the IKE_AUTH request"))
			}
			rec.stateBefore, rec.timeBefore = ini.State, ini.RetransmitTime
			rec.countBefore, rec.msgIDBefore = ini.RetransmitCount, ini.NextMsgID
			before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
			handleInbound(ini, transport.Packet{Data: unprotTampered(far.LastSentMsg), RemoteAddr: peerAddr}, table, myTr, log)
			rec.drops = errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before
			// The forgery's wake, not the timer, ends this wait.
			return never
		case 3:
			// The pass the forgery woke ended in this wait. The retransmit time is
			// rewound to stand in for its arrival.
			rec.stateAfter, rec.timeAfter = ini.State, ini.RetransmitTime
			rec.countAfter, rec.msgIDAfter = ini.RetransmitCount, ini.NextMsgID
			ini.RetransmitTime = time.Now().Add(-time.Millisecond)
			return fired()
		default:
			// The wait after the first retransmission.
			rec.countRetransmitted = ini.RetransmitCount
			rec.lastRetransmitted = bytes.Clone(ini.LastSentMsg)
			rec.waitRetransmitted = d
			stop()
			return never
		}
	}

	var cycleErr error
	done := make(chan struct{})
	go func() {
		cycleErr = ps.runInitiator(iniPeer, ikeGroup, table, myTr, nil, log)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(rtxArrive):
		stop()
		<-done
		t.Fatal("the driven handshake never reached its verdict")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.err != nil {
		t.Fatalf("the far end could not be driven: %v", rec.err)
	}
	if !errors.Is(cycleErr, errStopped) {
		t.Fatalf("runInitiator after one forged IKE_AUTH response returned %v, want errStopped from a handshake still retransmitting", cycleErr)
	}
	if rec.stateBefore != StateAuthSent || rec.stateAfter != StateAuthSent {
		t.Fatalf("initiator state around the forgery = %v then %v, want IKE_AUTH sent both times", rec.stateBefore, rec.stateAfter)
	}
	if !rec.timeAfter.Equal(rec.timeBefore) {
		t.Fatalf("the forgery moved the retransmit time from %v to %v", rec.timeBefore, rec.timeAfter)
	}
	if rec.countAfter != rec.countBefore {
		t.Fatalf("the forgery moved the retransmit count from %d to %d", rec.countBefore, rec.countAfter)
	}
	if rec.msgIDAfter != rec.msgIDBefore {
		t.Fatalf("the forgery moved the next Message ID from %d to %d", rec.msgIDBefore, rec.msgIDAfter)
	}
	if rec.drops != 1 {
		t.Fatalf("%d drops counted under %q for one forgery, want 1", rec.drops, suppressedUnauthenticatedHandshake)
	}
	if rec.countRetransmitted != rec.countBefore+1 {
		t.Fatalf("retransmit count after the retransmission = %d, want %d", rec.countRetransmitted, rec.countBefore+1)
	}
	if !bytes.Equal(rec.lastRetransmitted, rec.authReq) {
		t.Fatal("the request the loop retransmits is not the IKE_AUTH request it sent")
	}
	// The first backoff step, measured from just before the wait was asked for.
	step := retransmitBackoff(rec.countBefore + 1)
	if rec.waitRetransmitted > step || rec.waitRetransmitted*4 <= step*3 {
		t.Fatalf("wait after the retransmission = %v, want at most %v and more than %v", rec.waitRetransmitted, step, step*3/4)
	}

	// On the wire: the IKE_SA_INIT, the IKE_AUTH request, and its one retransmission,
	// octet for octet, and nothing else. The forgery drew nothing.
	for i, want := range [][]byte{rec.saInit, rec.authReq, rec.authReq} {
		got := rtxRecv(t, peerTr)
		if got == nil {
			t.Fatalf("datagram %d the initiator should have sent never arrived", i+1)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("datagram %d the initiator sent (%d octets) is not the expected message (%d octets)", i+1, len(got), len(want))
		}
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "the handshake after the retransmission")
}

// unprotTimeoutStepsMax bounds the afterFunc stub of the timeout test: one step for
// IKE_SA_INIT, then a forgery and a rewind for the first send and for each
// retransmission, plus a margin that only stops a regression that loops.
const unprotTimeoutStepsMax = 2*(maxRetransmissions+1) + 8

// unprotSealResponse seals inner under sender's keys as an IKE_AUTH response at
// messageID. The checksum verifies, so whatever inner holds is the sender's own.
func unprotSealResponse(t *testing.T, sender *SA, inner []byte, firstType uint8, messageID uint32) []byte {
	t.Helper()
	flags := wire.FlagResponse | initiatorFlag(sender)
	var raw []byte
	var err error
	if sender.Proposal.Encryption.IsAEAD {
		raw, err = buildSKMessageAEADWithMsgID(sender, inner, firstType, messageID, wire.ExchangeIKEAuth, flags)
	} else {
		raw, err = buildSKMessageCBCWithMsgID(sender, inner, firstType, messageID, wire.ExchangeIKEAuth, flags)
	}
	if err != nil {
		t.Fatalf("seal the IKE_AUTH response: %v", err)
	}
	return raw
}

// unprotSealBadPadLength seals, under sender's CBC keys, an IKE_AUTH response at
// messageID whose one plaintext block ends in a Pad Length of 255, longer than the
// plaintext. The checksum covers it and verifies, so the Pad Length is the one fault.
func unprotSealBadPadLength(t *testing.T, sender *SA, messageID uint32) []byte {
	t.Helper()
	if sender.Proposal.Encryption.IsAEAD {
		t.Fatal("a Pad Length fault needs a CBC proposal; the test IKE group negotiated an AEAD cipher")
	}
	const blockSize = 16
	integTrunc := int(sender.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}
	total := wire.HeaderLen + wire.GenericHeaderLen + 2*blockSize + integTrunc
	buf := make([]byte, total)
	writeAuthHeaderWithMsgID(buf, sender, wire.PayloadTypeAUTH, uint32(total), messageID,
		wire.ExchangeIKEAuth, wire.FlagResponse|initiatorFlag(sender))
	iv := buf[skIVOffset : skIVOffset+blockSize]
	copy(iv, bytes.Repeat([]byte{0x5a}, blockSize))
	data := buf[skIVOffset+blockSize : skIVOffset+2*blockSize]
	data[blockSize-1] = 0xff
	block, err := aes.NewCipher(skSendEncKey(sender))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gocipher.NewCBCEncrypter(block, iv).CryptBlocks(data, data)
	mac, err := ikecrypto.ComputeIntegrity(sender.Proposal.Integrity.ID, skSendIntegKey(sender), buf[:total-integTrunc])
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}
	copy(buf[total-integTrunc:], mac)
	return buf
}

// unprotIniSecondExchange runs an RFC 4739 handshake whose initiator authenticates in
// two rounds up to the responder's answer to the second IKE_AUTH request. It answers the
// pair, the responder's first response (Message ID 1), the initiator's second request,
// and the responder's final response, held back from the initiator.
func unprotIniSecondExchange(t *testing.T) (p *maPair, firstResp, req2, final []byte) {
	t.Helper()
	p = maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
	p.respondWith(p.ini.LastSentMsg, false)
	firstResp = bytes.Clone(p.resp.LastSentMsg)
	p.answerWith(firstResp, false)
	if !p.ini.secondAuthExchange {
		t.Fatalf("the initiator did not start the second IKE_AUTH exchange (state %v)", p.ini.State)
	}
	req2 = bytes.Clone(p.ini.LastSentMsg)
	p.respondWith(req2, false)
	if p.resp.State != StateEstablished {
		t.Fatalf("responder state after the second request = %v, want established", p.resp.State)
	}
	return p, firstResp, req2, p.resp.LastSentMsg
}

// VALIDATES: in the RFC 4739 second exchange an IKE_AUTH response that fails its
// cryptographic check keeps the SA, as before, and is now counted under
// handshake-unauthenticated and logged below WARN; a late copy of the first response is
// still dropped before any decrypt and not counted; the genuine final response then
// establishes the SA.
//
// METHOD: a Ze-to-Ze handshake whose initiator runs two rounds reaches the responder's
// final response, held back. Its copy with the checksum's last octet flipped reaches the
// initiator through handleInbound, then a tampered copy of the first response.
//
// PREVENTS: the second exchange dropping a forgery with a WARN line and no count, so an
// off-path sender wrote the operator's log and the counter missed the drop.
//
// RFC requirement: RFC7296-2.4-3 negative -- in the RFC 4739 second IKE_AUTH exchange, a
// final response whose checksum fails leaves the initiator's state, next Message ID and
// retransmitted second request unchanged, raises handshake-unauthenticated by exactly
// one and writes no WARN line; a tampered late copy of the first response leaves the
// state unchanged and is not counted; neither draws a datagram before the sentinel; and
// the genuine final response then establishes the SA.
func TestInitiatorSecondAuthExchangeCountsUnprotectedResponse(t *testing.T) {
	p, firstResp, req2, final := unprotIniSecondExchange(t)
	state, nextMsgID := p.ini.State, p.ini.NextMsgID
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
	p.logs.Reset()

	handleInbound(p.ini, transport.Packet{Data: unprotTampered(final), RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != state {
		t.Fatalf("initiator state after the forgery = %v, want %v", p.ini.State, state)
	}
	if p.ini.NextMsgID != nextMsgID {
		t.Fatalf("the forgery moved the next Message ID from %d to %d", nextMsgID, p.ini.NextMsgID)
	}
	if !bytes.Equal(p.ini.LastSentMsg, req2) {
		t.Fatal("the forgery replaced the second IKE_AUTH request the initiator retransmits")
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 1 {
		t.Fatalf("%d drops counted under %q for one forgery, want 1", got, suppressedUnauthenticatedHandshake)
	}
	if strings.Contains(p.logs.String(), "level=WARN") {
		t.Fatalf("a forged second-exchange response wrote a WARN line:\n%s", p.logs.String())
	}

	before = errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
	handleInbound(p.ini, transport.Packet{Data: unprotTampered(firstResp), RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)
	if p.ini.State != state {
		t.Fatalf("initiator state after a late first response = %v, want %v", p.ini.State, state)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("a response at the first exchange's Message ID was counted %d times, want 0: it is dropped before any decrypt", got)
	}
	rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "an unprotected second-exchange response")

	p.answerWith(final, false)
	if p.ini.State != StateEstablished {
		t.Fatalf("the genuine final response after the forgery left the initiator %v, want established", p.ini.State)
	}
}

// VALIDATES: an IKE_AUTH response that passes its cryptographic check but whose inner
// payload chain does not parse still ends the initiator's handshake, sends nothing, and
// is not counted as unauthenticated.
//
// METHOD: the PSK handshake reaches StateAuthSent. A response is sealed under the
// responder's own keys at the IKE_AUTH Message ID, carrying one inner payload whose
// generic header declares two octets, shorter than the header itself.
//
// PREVENTS: the drop for forgeries swallowing every error. The negative tests pass just
// as well against a handler that ignores every error; a message the peer demonstrably
// sent is evidence about the peer.
//
// RFC requirement: RFC7296-2.4-3 positive -- an IKE_AUTH response that arrives WITH cryptographic protection and whose inner chain does not parse is acted on: the initiator ends the handshake (StateDead), sends no datagram, and the unauthenticated-drop counter does not move.
func TestInitiatorAuthEndsOnAuthenticatedMalformedResponse(t *testing.T) {
	p := maSAInitPair(t, maAuth(nil, nil), maAuth(nil, nil))
	// A generic payload header claiming 2 octets: ParsePayloadChain refuses a payload
	// shorter than wire.GenericHeaderLen.
	malformed := unprotSealResponse(t, p.resp, []byte{0, 0, 0, 2}, wire.PayloadTypeIDr, eaplenMessageID(p.ini.LastSentMsg))
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	handleInbound(p.ini, transport.Packet{Data: malformed, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != StateDead {
		t.Fatalf("initiator state after an authenticated malformed response = %v, want dead", p.ini.State)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("an authenticated response was counted %d times as unauthenticated", got)
	}
	rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "an authenticated malformed IKE_AUTH response")
}

// VALIDATES: an EAP round that passes its cryptographic check but whose inner chain does
// not parse still ends the initiator's handshake, and is not counted as unauthenticated.
//
// METHOD: a real EAP-MSCHAPv2 exchange reaches the initiator's StateEAPInProgress. A
// response sealed under the responder's keys at the round-2 Message ID carries one inner
// payload whose generic header declares two octets.
//
// PREVENTS: the EAP drop for forgeries swallowing authenticated content too.
func TestInitiatorEAPEndsOnAuthenticatedMalformedResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, _, authReq := eaplenExchange(t)
	handleInbound(resp, transport.Packet{Data: authReq}, table, nil, log)
	handleInbound(ini, transport.Packet{Data: resp.LastSentMsg}, table, nil, log)
	if ini.State != StateEAPInProgress {
		t.Fatalf("initiator state after the first EAP request = %v, want EAP in progress", ini.State)
	}
	malformed := unprotSealResponse(t, resp, []byte{0, 0, 0, 2}, wire.PayloadTypeEAP, eaplenMessageID(ini.LastSentMsg))
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	handleInbound(ini, transport.Packet{Data: malformed}, table, nil, log)

	if ini.State != StateDead {
		t.Fatalf("initiator state after an authenticated malformed EAP round = %v, want dead", ini.State)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("an authenticated EAP round was counted %d times as unauthenticated", got)
	}
}

// VALIDATES: in the RFC 4739 second exchange an authenticated response whose Pad Length
// is wrong keeps the SA, uncounted, as it did before the opener moved, and the genuine
// final response then establishes it.
//
// METHOD: the second exchange reaches the responder's final response, held back. A
// response sealed and MACed under the responder's CBC keys at the second request's
// Message ID, with a Pad Length of 255, reaches the initiator first.
//
// PREVENTS: the move onto openProtected changing the second exchange's reaction to its
// decrypt step, which the opener reports apart from a chain that does not parse.
func TestInitiatorSecondAuthExchangeKeepsSAOnAuthenticatedPaddingFailure(t *testing.T) {
	p, _, req2, final := unprotIniSecondExchange(t)
	state, nextMsgID := p.ini.State, p.ini.NextMsgID
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	badPad := unprotSealBadPadLength(t, p.resp, eaplenMessageID(req2))
	handleInbound(p.ini, transport.Packet{Data: badPad, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != state {
		t.Fatalf("initiator state after an authenticated padding failure = %v, want %v", p.ini.State, state)
	}
	if p.ini.NextMsgID != nextMsgID {
		t.Fatalf("the padding failure moved the next Message ID from %d to %d", nextMsgID, p.ini.NextMsgID)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("an authenticated padding failure was counted %d times as unauthenticated", got)
	}
	rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "an authenticated padding failure in the second exchange")

	p.answerWith(final, false)
	if p.ini.State != StateEstablished {
		t.Fatalf("the genuine final response left the initiator %v, want established", p.ini.State)
	}
}

// VALIDATES: an authenticated IKE_AUTH response whose EAP Length exceeds its octets is
// discarded with the initiator's SA kept and nothing counted, through the opener, and the
// genuine response then starts the EAP exchange.
//
// METHOD: the EAP-MSCHAPv2 handshake reaches StateAuthSent. An EAP Request whose Length
// claims more octets than it carries is sealed under the responder's keys at the IKE_AUTH
// Message ID and delivered first.
//
// PREVENTS: the opener's errInnerChain wrapping hiding wire.ErrEAPLengthExceedsData from
// eapMessageDiscarded, which would end the SA on a message RFC 3748 Section 4 discards.
func TestEAPLengthDiscardKeepsInitiatorSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, _, authReq := eaplenExchange(t)
	nextMsgID := ini.NextMsgID
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	overlong := eaplenOverlong(t, resp, 1, eaplenMessageID(authReq), wire.FlagResponse|initiatorFlag(resp))
	handleInbound(ini, transport.Packet{Data: overlong}, table, nil, log)

	if ini.State != StateAuthSent {
		t.Fatalf("initiator state after an overlong EAP Request = %v, want IKE_AUTH sent", ini.State)
	}
	if ini.NextMsgID != nextMsgID {
		t.Fatalf("the discard moved the next Message ID from %d to %d", nextMsgID, ini.NextMsgID)
	}
	if !bytes.Equal(ini.LastSentMsg, authReq) {
		t.Fatal("the discard replaced the IKE_AUTH request the initiator retransmits")
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("an authenticated EAP discard was counted %d times as unauthenticated", got)
	}

	handleInbound(resp, transport.Packet{Data: authReq}, table, nil, log)
	handleInbound(ini, transport.Packet{Data: resp.LastSentMsg}, table, nil, log)
	if ini.State != StateEAPInProgress {
		t.Fatalf("the genuine first EAP request after the discard left the initiator %v, want EAP in progress", ini.State)
	}
}

// VALIDATES: an IKE_AUTH response with no Encrypted payload is dropped with the
// initiator's SA unchanged, as before, and is now counted under handshake-unauthenticated.
//
// METHOD: the PSK handshake reaches StateAuthSent. A bare 28-octet header under the live
// SPIs at the IKE_AUTH Message ID, marked as a response, reaches the initiator.
//
// PREVENTS: a drop nobody can see: the bare header was dropped at debug and counted
// nowhere.
func TestInitiatorAuthResponseWithoutSKIsCounted(t *testing.T) {
	p := maSAInitPair(t, maAuth(nil, nil), maAuth(nil, nil))
	authReq := bytes.Clone(p.ini.LastSentMsg)
	nextMsgID := p.ini.NextMsgID
	bare := unprotBareHeader(p.ini, eaplenMessageID(authReq))
	bare[19] = wire.FlagResponse
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	handleInbound(p.ini, transport.Packet{Data: bare, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != StateAuthSent {
		t.Fatalf("initiator state after a bare response header = %v, want IKE_AUTH sent", p.ini.State)
	}
	if p.ini.NextMsgID != nextMsgID {
		t.Fatalf("the bare header moved the next Message ID from %d to %d", nextMsgID, p.ini.NextMsgID)
	}
	if !bytes.Equal(p.ini.LastSentMsg, authReq) {
		t.Fatal("the bare header replaced the IKE_AUTH request the initiator retransmits")
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 1 {
		t.Fatalf("%d drops counted under %q for one bare header, want 1", got, suppressedUnauthenticatedHandshake)
	}
	rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "an IKE_AUTH response with no Encrypted payload")
}

// VALIDATES: an initiator that receives only forgeries sends the identical IKE_AUTH
// request maxRetransmissions more times and then gives up with errTimeout, the count it
// spends with no input at all.
//
// METHOD: runInitiator runs against a far end driven from the afterFunc seam, as in
// TestInitiatorKeepsRetransmittingAfterUnprotectedResponse. After IKE_SA_INIT the stub
// alternates: one wait delivers a forgery of the far end's IKE_AUTH response (its wake
// ends that wait), the next rewinds the retransmit time so the loop retransmits. The
// test then reads every datagram the initiator wrote.
//
// PREVENTS: a drop that ends the handshake early (errSADead), or one that makes the loop
// retransmit more or less than an unanswered handshake does.
func TestInitiatorTimesOutWhenOnlyUnprotectedResponsesArrive(t *testing.T) {
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)
	peerAddr := eaprtxPeerAddr(t, peerTr)
	ikeGroup, espGroup := testIKEGroup(), testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "unprotected-timeout-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ps := &PeerSession{
		peerName:      unprotRespPeerName,
		peerCfg:       iniPeer,
		ikeGroup:      ikeGroup,
		espGroup:      espGroup,
		stopCh:        make(chan struct{}),
		handshakeWake: make(chan struct{}, 1),
		inbound:       make(chan transport.Packet, inboundQueueDepth),
		supersede:     make(chan struct{}, 1),
	}
	unprotRespRegister(t, ps)
	respPS := &PeerSession{peerName: unprotRespPeerName, peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	table := NewSATable()
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { close(ps.stopCh) }) }

	var mu sync.Mutex
	var driveErr error
	var saInit, authReq, forged []byte
	var far *SA
	steps, forgeries := 0, 0
	forgeryPending := false
	var drops uint64
	never := make(chan time.Time)
	fail := func(err error) <-chan time.Time {
		driveErr = err
		stop()
		return never
	}

	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	// The stub runs on the session goroutine, inside awaitHandshake.
	afterFunc = func(time.Duration) <-chan time.Time {
		mu.Lock()
		defer mu.Unlock()
		steps++
		ini := ps.getSA()
		if steps > unprotTimeoutStepsMax || ini == nil {
			return fail(errors.New("the handshake loop ran past the steps the test drives"))
		}
		switch {
		case steps == 1:
			saInit = bytes.Clone(ini.LastSentMsg)
			sa, err := newResponderSA(unprotRespPeerName, respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
			if err != nil {
				return fail(err)
			}
			far = sa
			req, err := icyParse(ini.LastSentMsg)
			if err != nil {
				return fail(err)
			}
			handleSAInitRequest(far, req, ini.LastSentMsg, nil, nil, log)
			answer, err := icyParse(far.LastSentMsg)
			if err != nil {
				return fail(err)
			}
			handleSAInitResponse(ini, answer, far.LastSentMsg, table, nil, nil, log)
		case forgeryPending:
			// The retransmit time arrives.
			forgeryPending = false
			ini.RetransmitTime = time.Now().Add(-time.Millisecond)
		default:
			if forged == nil {
				authReq = bytes.Clone(ini.LastSentMsg)
				req, err := icyParse(ini.LastSentMsg)
				if err != nil {
					return fail(err)
				}
				respPS.handleAuthRequest(far, req, ini.LastSentMsg, nil, nil, log)
				if far.State != StateEstablished {
					return fail(errors.New("the far end refused the IKE_AUTH request"))
				}
				forged = unprotTampered(far.LastSentMsg)
			}
			before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
			handleInbound(ini, transport.Packet{Data: forged, RemoteAddr: peerAddr}, table, myTr, log)
			drops += errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before
			forgeries++
			forgeryPending = true
			// The forgery's wake, not the timer, ends this wait.
			return never
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	var cycleErr error
	done := make(chan struct{})
	go func() {
		cycleErr = ps.runInitiator(iniPeer, ikeGroup, table, myTr, nil, log)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(rtxArrive):
		stop()
		<-done
		t.Fatal("the driven handshake never reached its verdict")
	}

	mu.Lock()
	defer mu.Unlock()
	if driveErr != nil {
		t.Fatalf("the far end could not be driven: %v", driveErr)
	}
	if !errors.Is(cycleErr, errTimeout) {
		t.Fatalf("runInitiator with only forged IKE_AUTH responses returned %v, want errTimeout", cycleErr)
	}
	if forgeries != maxRetransmissions+1 {
		t.Fatalf("the stub delivered %d forgeries, want one per send, %d", forgeries, maxRetransmissions+1)
	}
	if drops != uint64(forgeries) {
		t.Fatalf("%d drops counted under %q for %d forgeries", drops, suppressedUnauthenticatedHandshake, forgeries)
	}
	want := [][]byte{saInit}
	for range maxRetransmissions + 1 {
		want = append(want, authReq)
	}
	for i, w := range want {
		got := rtxRecv(t, peerTr)
		if got == nil {
			t.Fatalf("datagram %d the initiator should have sent never arrived", i+1)
		}
		if !bytes.Equal(got, w) {
			t.Fatalf("datagram %d the initiator sent (%d octets) is not the expected message (%d octets)", i+1, len(got), len(w))
		}
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "the handshake after its last retransmission")
}

// VALIDATES: an IKE_AUTH response that fails its cryptographic check, arriving from
// another address and port, leaves the initiator's stored peer endpoint as it was, and
// the genuine response then sets the endpoint to the address it came from.
//
// METHOD: the PSK handshake reaches StateAuthSent with no endpoint stored. The forgery
// comes from one loopback port, the genuine response from another.
//
// PREVENTS: an unauthenticated datagram steering where the initiator answers: RFC 7296
// Section 2.23 adopts an endpoint only from a packet "whose integrity protection
// validates".
func TestUnprotectedAuthResponseDoesNotMoveThePeerEndpoint(t *testing.T) {
	p := maSAInitPair(t, maAuth(nil, nil), maAuth(nil, nil))
	p.respondWith(bytes.Clone(p.ini.LastSentMsg), false)
	genuine := p.resp.LastSentMsg
	if p.ini.peerEndpoint != nil {
		t.Fatalf("the initiator stored a peer endpoint %v before any authenticated response", p.ini.peerEndpoint)
	}
	forger := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: p.peerAddr.Port + 1}
	answerer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: p.peerAddr.Port + 2}

	handleInbound(p.ini, transport.Packet{Data: unprotTampered(genuine), RemoteAddr: forger}, p.table, p.myTr, p.log)

	if p.ini.peerEndpoint != nil {
		t.Fatalf("the forgery stored the peer endpoint %v", p.ini.peerEndpoint)
	}

	handleInbound(p.ini, transport.Packet{Data: genuine, RemoteAddr: answerer}, p.table, p.myTr, p.log)

	if p.ini.State != StateEstablished {
		t.Fatalf("the genuine response after the forgery left the initiator %v, want established", p.ini.State)
	}
	if p.ini.peerEndpoint == nil || !sameUDPEndpoint(p.ini.peerEndpoint, answerer) {
		t.Fatalf("peer endpoint after the genuine response = %v, want %v", p.ini.peerEndpoint, answerer)
	}
}

// VALIDATES: AC-9, the in-window site. On an established SA, an in-window INFORMATIONAL
// request whose checksum fails is dropped: nothing is sent, no liveness is credited, the
// SA and its Message ID window are unchanged, and established-unauthenticated rises by
// exactly one. The genuine request is then answered and credited.
//
// METHOD: osrSession gives an established PSK pair on a loopback link. The peer seals the
// liveness probe at the expected Message ID; the forgery is that datagram with its last
// octet, which belongs to the checksum, flipped. Both go through handleOwnedInbound, the
// owner loop's entry for every established-SA datagram.
//
// PREVENTS: the in-window site dropping such a message with only a debug line, so no
// operator could see that someone sends datagrams under the live SPIs.
//
// RFC requirement: RFC7296-2.4-3 negative -- on an established SA, an in-window
// INFORMATIONAL request whose checksum fails is not credited as liveness, draws no
// datagram before the sentinel, raises established-unauthenticated by exactly one, and
// leaves the SA established with its expected Message ID and Child SA unchanged; the
// genuine request then draws an answer and is credited as liveness.
func TestEstablishedSACountsUnprotectedMessage(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()
	child := ps.getChildSA()
	expected := ini.ExpectedMsgID

	good := osrRequest(t, peer, expected)
	forged := unprotTampered(good)
	before := errorNotifySuppressedCount(suppressedUnauthenticatedEstablished)
	invalidBefore := errorNotifySuppressedCount(suppressedInvalidMsgIDUnauthenticated)
	out := ps.handleOwnedInbound(ini, transport.Packet{Data: forged}, myTr, nil, log)
	if out.peerAlive {
		t.Error("a message that failed its integrity check was credited as liveness")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "an in-window request that failed its integrity check")
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedEstablished) - before; got != 1 {
		t.Errorf("%s rose by %d, want 1", suppressedUnauthenticatedEstablished, got)
	}
	if got := errorNotifySuppressedCount(suppressedInvalidMsgIDUnauthenticated) - invalidBefore; got != 0 {
		t.Errorf("an in-window message was counted %d times as out of window", got)
	}
	if ini.State != StateEstablished {
		t.Fatalf("state after the forgery = %v, want established", ini.State)
	}
	if ini.ExpectedMsgID != expected {
		t.Fatalf("expected Message ID after the forgery = %d, want %d", ini.ExpectedMsgID, expected)
	}
	if ps.getChildSA() != child {
		t.Fatal("the forgery changed the Child SA")
	}

	// Control: the genuine request reaches the same site, is answered and credited.
	out = ps.handleOwnedInbound(ini, transport.Packet{Data: good}, myTr, nil, log)
	if !out.peerAlive {
		t.Error("the genuine request was not credited as liveness")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the genuine request drew no answer")
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedEstablished) - before; got != 1 {
		t.Errorf("the genuine request was counted: %s rose by %d, want 1", suppressedUnauthenticatedEstablished, got)
	}
}

// VALIDATES: AC-9, the out-of-window INFORMATIONAL response site. An INFORMATIONAL
// response at a Message ID no request of Ze's holds, whose checksum fails, is dropped and
// counted under established-unauthenticated, completes no probe and sends nothing; the
// genuine copy reaches the same arm and is reported as a probe answer.
//
// METHOD: osrSession's peer seals an empty INFORMATIONAL response at seven past Ze's next
// Message ID; the forgery flips its last octet. Both go through handleOwnedInbound.
//
// PREVENTS: the arm falling through to its out-of-window debug line on a forgery, uncounted.
//
// RFC requirement: RFC7296-2.4-3 negative -- on an established SA, an INFORMATIONAL
// response at a Message ID no request of Ze's holds, whose checksum fails, is reported
// neither as a probe answer nor as liveness, draws no datagram before the sentinel,
// raises established-unauthenticated by exactly one and leaves the SA established; the
// genuine copy is then reported as the probe answer at its Message ID.
func TestEstablishedSACountsUnprotectedOutOfWindowResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()
	msgID := ini.NextMsgID + 7

	good, err := buildEncryptedMessageEx(peer, nil, msgID, wire.ExchangeInformational,
		initiatorFlag(peer)|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the peer response at id %d: %v", msgID, err)
	}
	forged := unprotTampered(good)
	before := errorNotifySuppressedCount(suppressedUnauthenticatedEstablished)
	invalidBefore := errorNotifySuppressedCount(suppressedInvalidMsgIDUnauthenticated)
	out := ps.handleOwnedInbound(ini, transport.Packet{Data: forged}, myTr, nil, log)
	if out.dpdResp || out.peerAlive {
		t.Errorf("a response that failed its integrity check was reported: %+v", out)
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "an out-of-window response that failed its integrity check")
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedEstablished) - before; got != 1 {
		t.Errorf("%s rose by %d, want 1", suppressedUnauthenticatedEstablished, got)
	}
	if got := errorNotifySuppressedCount(suppressedInvalidMsgIDUnauthenticated) - invalidBefore; got != 0 {
		t.Errorf("a response was counted %d times as an out-of-window request", got)
	}
	if ini.State != StateEstablished {
		t.Fatalf("state after the forgery = %v, want established", ini.State)
	}

	// Control: the genuine copy authenticates in the same arm.
	out = ps.handleOwnedInbound(ini, transport.Packet{Data: good}, myTr, nil, log)
	if !out.dpdResp {
		t.Fatal("the genuine out-of-window response was not reported, so the forgery may not have reached its arm")
	}
	if out.dpdRespMsgID != msgID {
		t.Fatalf("reported Message ID = %d, want %d", out.dpdRespMsgID, msgID)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedEstablished) - before; got != 1 {
		t.Errorf("the genuine response was counted: %s rose by %d, want 1", suppressedUnauthenticatedEstablished, got)
	}
}

// VALIDATES: the held reauthentication successor's drop site. A request at the
// successor's expected Message ID whose checksum fails leaves the successor held and
// established beside the unchanged owned IKE SA, with its expected Message ID and cached
// response unchanged, sends nothing, and raises held-successor-decrypt-failed by exactly
// one. The genuine request is then answered, and the successor is still held.
//
// METHOD: heldInitiatorSuccessor holds a gateway reauthentication without a Child SA.
// The gateway seals a liveness probe at the successor's expected Message ID; the forgery
// is that datagram with its last octet, which belongs to the checksum, flipped. Both
// reach handleInbound, the dispatch goroutine's entry, from a loopback peer socket that
// rtxPeerLink opens, so a datagram Ze wrote would reach it before the sentinel.
//
// PREVENTS: handleHeldSuccessorInbound answering a forgery as an authenticated message
// whose payloads do not parse: INVALID_SYNTAX, which RFC 7296 Section 2.21.3 makes
// fatal, so one off-path datagram under the successor's SPIs would end the
// reauthentication.
//
// RFC requirement: RFC7296-2.4-3 negative -- on the held reauthentication successor, a
// request at its expected Message ID whose checksum fails leaves it held and established
// beside the unchanged owned IKE SA and Child SA, leaves its expected Message ID and
// cached response unchanged, draws no datagram before the sentinel, and raises
// held-successor-decrypt-failed by exactly one; the genuine request then draws a
// datagram and a response cached at its Message ID, and leaves it held.
func TestHeldSuccessorSurvivesUnprotectedMessage(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, old, gw, held, table := heldInitiatorSuccessor(t)
	oldChild := ps.getChildSA()
	peerTr, myTr := rtxPeerLink(t)
	remote := nttPeerAddr(t, peerTr)
	expected := held.ExpectedMsgID
	cachedSet, cachedID := held.lastResponseSet, held.lastResponseID

	good, err := buildEncryptedMessageEx(gw, nil, expected, wire.ExchangeInformational, initiatorFlag(gw))
	if err != nil {
		t.Fatalf("build the gateway's liveness probe at id %d: %v", expected, err)
	}
	before := errorNotifySuppressedCount(suppressedHeldSuccessorUnauthenticated)
	handleInbound(held, transport.Packet{Data: unprotTampered(good), RemoteAddr: remote}, table, myTr, log)

	rtxExpectSilence(t, peerTr, myTr, remote, "a held-successor request that failed its integrity check")
	if got := errorNotifySuppressedCount(suppressedHeldSuccessorUnauthenticated) - before; got != 1 {
		t.Errorf("%s rose by %d, want 1", suppressedHeldSuccessorUnauthenticated, got)
	}
	if ps.getPendingSA() != held {
		t.Fatal("the forgery ended the held successor")
	}
	if held.State != StateEstablished {
		t.Fatalf("held successor state after the forgery = %v, want established", held.State)
	}
	if held.ExpectedMsgID != expected {
		t.Fatalf("expected Message ID after the forgery = %d, want %d", held.ExpectedMsgID, expected)
	}
	if held.lastResponseSet != cachedSet || held.lastResponseID != cachedID {
		t.Fatal("the forgery changed the held successor's cached response")
	}
	if ps.getSA() != old || ps.getChildSA() != oldChild {
		t.Fatal("the forgery disturbed the owned IKE SA or its Child SA")
	}

	// Control: the genuine probe reaches the same site and is answered, so the silence
	// above is the drop and not a path that sends nowhere.
	handleInbound(held, transport.Packet{Data: good, RemoteAddr: remote}, table, myTr, log)
	if !held.lastResponseSet || held.lastResponseID != expected {
		t.Fatalf("the genuine probe at id %d on the held successor went unanswered", expected)
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the genuine probe drew no datagram")
	}
	if ps.getPendingSA() != held {
		t.Fatal("the genuine probe ended the held successor")
	}
	if got := errorNotifySuppressedCount(suppressedHeldSuccessorUnauthenticated) - before; got != 1 {
		t.Errorf("the genuine probe was counted: %s rose by %d, want 1", suppressedHeldSuccessorUnauthenticated, got)
	}
}
