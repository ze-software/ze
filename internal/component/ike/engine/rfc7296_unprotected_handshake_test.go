// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the responder handshake
// Related: protected_input.go -- openProtected, the producer these tests hold
// Related: rfc3748_eap_length_test.go -- eaplenExchange, the real EAP-MSCHAPv2 handshake these tests drive
// Related: rfc4739_rounds_test.go -- maSAInitPair, the RFC 4739 two-round handshake the second-exchange tests drive
// Related: rfc7296_encrypt_test.go -- cbcSKPair, buildCBCSKWithIVAndPad and forgePadLength

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// unprotFloodDatagrams is how many forgeries TestUnprotectedInputDoesNotExtendTheHandshake
// delivers. It only has to exceed one, so a forgery that refreshed any handshake field
// would show; fifty keeps the test well under a millisecond.
const unprotFloodDatagrams = 50

// unprotDeliveriesMax bounds the EAP-MSCHAPv2 exchange unprotFinish drives to
// establishment. The exchange takes eight deliveries; the bound only stops a loop
// that would otherwise run forever on a regression.
const unprotDeliveriesMax = 24

// unprotBareHeader builds the cheapest forgery of an IKE_AUTH request: the 28-octet RFC
// 7296 Section 3.1 header alone, with no payload, under sa's SPIs and at messageID.
// Both SPIs and the Message ID travel in the clear in every IKE datagram, so an
// off-path sender who saw one datagram of the exchange can write these bytes.
func unprotBareHeader(sa *SA, messageID uint32) []byte {
	raw := make([]byte, wire.HeaderLen)
	copy(raw[0:8], sa.InitiatorSPI[:])
	copy(raw[8:16], sa.ResponderSPI[:])
	raw[17] = 0x20 // Version 2.0.
	raw[18] = wire.ExchangeIKEAuth
	raw[19] = wire.FlagInitiator
	binary.BigEndian.PutUint32(raw[20:24], messageID)
	binary.BigEndian.PutUint32(raw[24:28], wire.HeaderLen)
	return raw
}

// unprotTampered returns a copy of a genuine sealed request with its last octet
// flipped. That octet belongs to the integrity checksum (or the AEAD tag), so the copy
// carries an Encrypted payload, the right SPIs and the right Message ID, and fails only
// its cryptographic check: the "IKE messages that arrive without cryptographic
// protection" of RFC 7296 Section 2.4 in their most convincing form.
func unprotTampered(genuine []byte) []byte {
	forged := bytes.Clone(genuine)
	forged[len(forged)-1] ^= 0xff
	return forged
}

// unprotDeliver hands raw to the responder through handleInbound, the function the
// dispatch goroutine calls for every datagram that names a responder handshake.
func unprotDeliver(resp *SA, raw []byte, table *SATable, myTr *transport.UDPTransport, peerAddr *net.UDPAddr) {
	handleInbound(resp, transport.Packet{Data: raw, RemoteAddr: peerAddr}, table, myTr, slogutil.DiscardLogger())
}

// unprotFinish runs the rest of a real EAP-MSCHAPv2 exchange from toInitiator, the
// responder's latest answer, until both sides are established. It fails the test when
// either side dies or the exchange outruns unprotDeliveriesMax.
func unprotFinish(t *testing.T, ini, resp *SA, table *SATable, toInitiator []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	cur := toInitiator
	toResponder := false
	for range unprotDeliveriesMax {
		if toResponder {
			handleInbound(resp, transport.Packet{Data: cur}, table, nil, log)
			cur = resp.LastSentMsg
		} else {
			handleInbound(ini, transport.Packet{Data: cur}, table, nil, log)
			cur = ini.LastSentMsg
		}
		if resp.State == StateDead || ini.State == StateDead {
			t.Fatalf("the EAP handshake died after the forgeries (ini=%v resp=%v)", ini.State, resp.State)
		}
		if ini.State == StateEstablished && resp.State == StateEstablished {
			return
		}
		toResponder = !toResponder
	}
	t.Fatalf("the EAP handshake did not establish after the forgeries (ini=%v resp=%v)", ini.State, resp.State)
}

// VALIDATES: an IKE_AUTH that fails its cryptographic check in the middle of an EAP
// exchange leaves the responder's handshake as it was, draws no datagram, and the
// legitimate peer then completes the exchange to an established IKE SA.
//
// METHOD: a real EAP-MSCHAPv2 exchange between two Ze endpoints reaches its second
// IKE_AUTH round. The initiator's real round-2 request is held back. Two forgeries at
// its Message ID reach the responder through handleInbound first: a bare header with no
// Encrypted payload, and the real request with its checksum's last octet flipped. The
// held-back request is then delivered and the exchange runs to the end.
//
// PREVENTS: handleResponderEAP setting StateDead on any decryptAndParse error, so one
// datagram built from an observed header ended the handshake of a peer that had done
// nothing wrong.
//
// RFC requirement: RFC7296-2.4-3 negative -- two IKE_AUTH requests without valid cryptographic protection, one with no Encrypted payload and one whose checksum fails, leave the responder in StateEAPInProgress at the same next Message ID and draw no datagram, and the peer's real round-2 request then drives the exchange to an established IKE SA on both sides.
func TestResponderEAPSurvivesUnprotectedIKEAuth(t *testing.T) {
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
	round2 := ini.LastSentMsg
	if resp.State != StateEAPInProgress {
		t.Fatalf("responder state before the forgeries = %v, want EAP in progress", resp.State)
	}
	expected := resp.ExpectedMsgID
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	unprotDeliver(resp, unprotBareHeader(resp, eaplenMessageID(round2)), table, myTr, peerAddr)
	unprotDeliver(resp, unprotTampered(round2), table, myTr, peerAddr)

	if resp.State != StateEAPInProgress {
		t.Fatalf("responder state after the forgeries = %v, want EAP in progress", resp.State)
	}
	if resp.ExpectedMsgID != expected {
		t.Fatalf("the forgeries moved the next Message ID from %d to %d", expected, resp.ExpectedMsgID)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 2 {
		t.Fatalf("%d forgeries counted under %q, want 2", got, suppressedUnauthenticatedHandshake)
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "an unprotected IKE_AUTH mid-EAP")

	unprotDeliver(resp, round2, table, myTr, peerAddr)
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the real round-2 request drew no answer after the forgeries")
	}
	unprotFinish(t, ini, resp, table, second)
}

// VALIDATES: an IKE_AUTH that fails its cryptographic check before the first genuine one
// arrives leaves the responder in StateSAInitReceived, draws no datagram, and the
// legitimate first IKE_AUTH then starts the exchange, which runs to establishment.
//
// METHOD: as above, one step earlier. The two forgeries carry the Message ID of the
// initiator's first IKE_AUTH and reach handleAuthRequest, the sibling of
// handleResponderEAP that takes the first IKE_AUTH request.
//
// PREVENTS: handleAuthRequest setting StateDead on a decrypt failure, which let a
// forgery racing the real first IKE_AUTH end the handshake right after IKE_SA_INIT.
//
// RFC requirement: RFC7296-2.4-3 negative -- two first IKE_AUTH requests without valid cryptographic protection leave the responder in StateSAInitReceived and draw no datagram, and the peer's real first IKE_AUTH then drives the exchange to an established IKE SA on both sides.
func TestResponderAuthSurvivesUnprotectedIKEAuth(t *testing.T) {
	ini, resp, table, _, authReq := eaplenExchange(t)
	peerTr, myTr := rtxPeerLink(t, resp)
	peerAddr := eaprtxPeerAddr(t, peerTr)
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	unprotDeliver(resp, unprotBareHeader(resp, eaplenMessageID(authReq)), table, myTr, peerAddr)
	unprotDeliver(resp, unprotTampered(authReq), table, myTr, peerAddr)

	if resp.State != StateSAInitReceived {
		t.Fatalf("responder state after the forgeries = %v, want IKE_SA_INIT received", resp.State)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 2 {
		t.Fatalf("%d forgeries counted under %q, want 2", got, suppressedUnauthenticatedHandshake)
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "an unprotected first IKE_AUTH")

	unprotDeliver(resp, authReq, table, myTr, peerAddr)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the real first IKE_AUTH drew no answer after the forgeries")
	}
	unprotFinish(t, ini, resp, table, first)
}

// VALIDATES: an EAP round that passes its cryptographic check but whose inner payload
// chain does not parse still ends the handshake, and is not counted as unauthenticated.
//
// METHOD: the exchange reaches its second IKE_AUTH round. A request is sealed under the
// initiator's own keys at the round-2 Message ID whose single inner payload declares a
// length of two octets, shorter than its own four-octet generic header. Its checksum
// verifies, so decryptAndParse marks the failure errInnerParse.
//
// PREVENTS: the fix for the forgeries swallowing every decrypt error. A message the peer
// demonstrably sent is evidence about the peer; only an unprotected one is not, and the
// two tests above would pass just as well against a handler that ignored every error.
//
// RFC requirement: RFC7296-2.4-3 positive -- a malformed round that arrives WITH cryptographic protection is acted on: the responder ends the handshake (StateDead), and the unauthenticated-drop counter does not move.
func TestResponderEAPEndsOnAuthenticatedMalformedRound(t *testing.T) {
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
	round2 := ini.LastSentMsg
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	// A generic payload header claiming 2 octets: ParsePayloadChain refuses a payload
	// shorter than wire.GenericHeaderLen.
	inner := []byte{0, 0, 0, 2}
	var malformed []byte
	var err error
	if ini.Proposal.Encryption.IsAEAD {
		malformed, err = buildSKMessageAEADWithMsgID(ini, inner, wire.PayloadTypeEAP, eaplenMessageID(round2), wire.ExchangeIKEAuth, initiatorFlag(ini))
	} else {
		malformed, err = buildSKMessageCBCWithMsgID(ini, inner, wire.PayloadTypeEAP, eaplenMessageID(round2), wire.ExchangeIKEAuth, initiatorFlag(ini))
	}
	if err != nil {
		t.Fatalf("seal the malformed round: %v", err)
	}
	unprotDeliver(resp, malformed, table, myTr, peerAddr)

	if resp.State != StateDead {
		t.Fatalf("responder state after an authenticated malformed round = %v, want dead", resp.State)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("an authenticated message was counted %d times as unauthenticated", got)
	}
}

// VALIDATES: unprotected input cannot keep a responder handshake alive. A flood of
// forgeries leaves CreatedAt and the next Message ID untouched, and the half-open
// reaper still ends the handshake once responderHandshakeTimeout has passed.
//
// METHOD: the exchange sits in StateEAPInProgress. unprotFloodDatagrams forgeries at
// the next Message ID reach the responder; then the handshake is aged past the timeout,
// one more forgery arrives, and reapStaleHandshake runs.
//
// PREVENTS: a repair that kept the handshake by refreshing it, which would let an
// off-path sender hold the responder's single handshake slot for a peer forever.
func TestUnprotectedInputDoesNotExtendTheHandshake(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, ps, authReq := eaplenExchange(t)
	peerTr, myTr := rtxPeerLink(t, resp)
	peerAddr := eaprtxPeerAddr(t, peerTr)

	unprotDeliver(resp, authReq, table, myTr, peerAddr)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the first IKE_AUTH drew no EAP request")
	}
	handleInbound(ini, transport.Packet{Data: first}, table, nil, log)
	round2 := ini.LastSentMsg
	created := resp.CreatedAt
	expected := resp.ExpectedMsgID

	for i := range unprotFloodDatagrams {
		forged := unprotTampered(round2)
		if i%2 == 0 {
			forged = unprotBareHeader(resp, eaplenMessageID(round2))
		}
		unprotDeliver(resp, forged, table, myTr, peerAddr)
	}
	if !resp.CreatedAt.Equal(created) {
		t.Fatalf("the forgeries moved CreatedAt from %v to %v", created, resp.CreatedAt)
	}
	if resp.ExpectedMsgID != expected {
		t.Fatalf("the forgeries moved the next Message ID from %d to %d", expected, resp.ExpectedMsgID)
	}
	if resp.State != StateEAPInProgress {
		t.Fatalf("responder state after the flood = %v, want EAP in progress", resp.State)
	}

	respTable := NewSATable()
	respTable.Insert(resp)
	resp.CreatedAt = time.Now().Add(-2 * responderHandshakeTimeout)
	unprotDeliver(resp, unprotTampered(round2), table, myTr, peerAddr)
	if !ps.reapStaleHandshake(resp, respTable, log) {
		t.Fatal("a handshake past responderHandshakeTimeout was not reaped after a forgery arrived")
	}
}

// unprotSecondExchange runs an RFC 4739 handshake whose initiator authenticates in two
// rounds up to the responder waiting for the second IKE_AUTH request
// (StateAuthReceived), with the responder's peer session registered so handleInbound
// reaches it. It answers the pair and the initiator's real second request, held back.
func unprotSecondExchange(t *testing.T) (p *maPair, req2 []byte) {
	t.Helper()
	p = maSAInitPair(t, maAuth(maIniLocal(), nil), maAuth(nil, maRespRemote()))
	setActivePeers(map[string]*PeerSession{"ze": p.ps})
	t.Cleanup(func() { setActivePeers(nil) })
	p.respondWith(p.ini.LastSentMsg, false)
	p.answerWith(p.resp.LastSentMsg, false)
	if p.resp.State != StateAuthReceived {
		t.Fatalf("responder state after the first exchange = %v, want IKE_AUTH received", p.resp.State)
	}
	return p, p.ini.LastSentMsg
}

// VALIDATES: an IKE_AUTH request of the RFC 4739 second exchange that arrives without
// valid cryptographic protection is dropped the way the first IKE_AUTH and each EAP
// round drop one: counted under handshake-unauthenticated, logged below WARN, the SA's
// state and next Message ID unchanged, no datagram sent, and the peer's real second
// request then establishes the SA.
//
// METHOD: a Ze-to-Ze handshake whose initiator runs two rounds reaches the responder's
// StateAuthReceived. Two forgeries at the second request's Message ID reach the
// responder through handleInbound, the dispatch goroutine's entry point: a bare header
// with no Encrypted payload, and the real request with its checksum's last octet
// flipped. Every log line at debug and above is captured, so a WARN would show.
//
// PREVENTS: handleResponderAuthRound logging a WARN for every decrypt error and counting
// nothing, which let an off-path sender write one line to the operator's log per forged
// datagram, the defect openProtected exists to refuse.
func TestResponderSecondAuthRoundDropsUnprotectedRequestQuietly(t *testing.T) {
	p, req2 := unprotSecondExchange(t)
	expected := p.resp.ExpectedMsgID
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)
	p.logs.Reset()

	for _, forged := range [][]byte{unprotBareHeader(p.resp, eaplenMessageID(req2)), unprotTampered(req2)} {
		handleInbound(p.resp, transport.Packet{Data: forged, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)
	}

	if p.resp.State != StateAuthReceived {
		t.Fatalf("responder state after the forgeries = %v, want IKE_AUTH received", p.resp.State)
	}
	if p.resp.ExpectedMsgID != expected {
		t.Fatalf("the forgeries moved the next Message ID from %d to %d", expected, p.resp.ExpectedMsgID)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 2 {
		t.Fatalf("%d forgeries counted under %q, want 2", got, suppressedUnauthenticatedHandshake)
	}
	if strings.Contains(p.logs.String(), "level=WARN") {
		t.Fatalf("a forged second IKE_AUTH request wrote a WARN line:\n%s", p.logs.String())
	}
	rtxExpectSilence(t, p.peerTr, p.myTr, p.peerAddr, "an unprotected second IKE_AUTH request")

	p.respondWith(req2, false)
	if p.resp.State != StateEstablished {
		t.Fatalf("the real second request after the forgeries left the responder %v", p.resp.State)
	}
}

// VALIDATES: a second-exchange IKE_AUTH request that passes its cryptographic check but
// whose inner payload chain does not parse ends the handshake, and is not counted as
// unauthenticated.
//
// METHOD: as above, up to StateAuthReceived. A request sealed under the initiator's own
// keys at the second request's Message ID carries one inner payload whose generic header
// declares two octets, shorter than the header itself, so decryptAndParse marks the
// failure errInnerParse.
//
// PREVENTS: the drop for forgeries swallowing every error, which the quiet-drop test
// above would not notice: a message the peer demonstrably sent is evidence about it.
func TestResponderSecondAuthRoundEndsOnAuthenticatedMalformedRequest(t *testing.T) {
	p, req2 := unprotSecondExchange(t)
	before := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake)

	// A generic payload header claiming 2 octets: ParsePayloadChain refuses a payload
	// shorter than wire.GenericHeaderLen.
	inner := []byte{0, 0, 0, 2}
	var malformed []byte
	var err error
	if p.ini.Proposal.Encryption.IsAEAD {
		malformed, err = buildSKMessageAEADWithMsgID(p.ini, inner, wire.PayloadTypeAUTH, eaplenMessageID(req2), wire.ExchangeIKEAuth, initiatorFlag(p.ini))
	} else {
		malformed, err = buildSKMessageCBCWithMsgID(p.ini, inner, wire.PayloadTypeAUTH, eaplenMessageID(req2), wire.ExchangeIKEAuth, initiatorFlag(p.ini))
	}
	if err != nil {
		t.Fatalf("seal the malformed second request: %v", err)
	}
	handleInbound(p.resp, transport.Packet{Data: malformed, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.resp.State != StateDead {
		t.Fatalf("responder state after an authenticated malformed second request = %v, want dead", p.resp.State)
	}
	if got := errorNotifySuppressedCount(suppressedUnauthenticatedHandshake) - before; got != 0 {
		t.Fatalf("an authenticated message was counted %d times as unauthenticated", got)
	}
}

// VALIDATES: decryptAndParse marks a CBC message whose checksum verified but whose Pad
// Length is wrong as errInnerParse, authenticated content that does not parse, and
// leaves a message whose checksum fails unmarked.
//
// METHOD: one aligned CBC SK message is re-encrypted and re-MACed with a Pad Length of
// 255, longer than its plaintext, so only the pad is wrong; a second copy has its
// checksum's last octet flipped. Both go through decryptAndParse on the receiving SA.
//
// PREVENTS: a post-checksum failure of decryptSKPayload returning unmarked, which made
// openProtected drop and count as a forgery a message only the
// peer could have sealed, and kept the established path from answering it.
func TestDecryptMarksAuthenticatedPaddingFailureAsInner(t *testing.T) {
	sa, peer := cbcSKPair(t)
	// No inner payload: fifteen pad octets and the Pad Length fill one 16-octet block.
	raw := buildCBCSKWithIVAndPad(t, sa, nil, 3, bytes.Repeat([]byte{0x44}, 16), 15)
	msg := parseMsg(t, raw)
	sk, ok := msg.Payloads[0].Payload.(*wire.PayloadSK)
	if !ok {
		t.Fatalf("sole payload = %T, want *wire.PayloadSK", msg.Payloads[0].Payload)
	}

	overlong := forgePadLength(t, sa, peer, raw, sk, 0xff)
	_, err := decryptAndParse(peer, parseMsg(t, overlong), overlong)
	if !errors.Is(err, errInnerParse) {
		t.Fatalf("an authenticated message with a Pad Length past its plaintext = %v, want errInnerParse", err)
	}
	if !errors.Is(err, errInvalidMessage) {
		t.Fatalf("the authenticated padding failure lost its cause: %v, want errInvalidMessage", err)
	}

	tampered := unprotTampered(raw)
	_, err = decryptAndParse(peer, parseMsg(t, tampered), tampered)
	if err == nil {
		t.Fatal("a message whose checksum fails was accepted")
	}
	if errors.Is(err, errInnerParse) {
		t.Fatalf("a message whose checksum fails was marked authenticated: %v", err)
	}
}
