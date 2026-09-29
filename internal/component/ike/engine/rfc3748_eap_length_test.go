package engine

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// eaplenDeclaredOctets is the EAP Length the overlong message claims, and
// eaplenCarriedOctets is what its EAP payload really carries: Code, Identifier,
// Length and a one-octet Type. The pair is RFC 3748 Section 4's "Length field
// set to a value larger than the number of received octets".
const (
	eaplenDeclaredOctets = 12
	eaplenCarriedOctets  = 5
)

// eaplenExchange drives a real EAP-MSCHAPv2 handshake up to the first IKE_AUTH. It
// returns both SAs, the table that holds the initiator, the session that owns the
// responder, and the IKE_AUTH request the initiator sent. It is
// eaprtxResponderMidExchange (rfc7296_eap_retransmit_test.go) with the initiator
// kept, because these tests seal messages under the initiator's keys and deliver the
// responder's answers back to it.
func eaplenExchange(t *testing.T) (ini, resp *SA, table *SATable, ps *PeerSession, authReq []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	autLoadPKI(t)
	iniPeer, respPeer := autPeers(ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPMSCHAPv2,
		PSK:           "eap-pass",
		Certificate:   autCertName,
		CACertificate: autCAName,
	})

	table = NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.setSA(resp)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)

	authReq = ini.LastSentMsg
	if len(authReq) == 0 {
		t.Fatal("the initiator sent no IKE_AUTH request")
	}
	return ini, resp, table, ps, authReq
}

// eaplenOverlong seals, under sender's keys, an IKE_AUTH message whose only inner
// payload is an EAP payload of eaplenCarriedOctets octets that declares
// eaplenDeclaredOctets. The IKE layer around it is valid: the checksum verifies and
// the generic payload header states the true payload length, so the one fault is
// the EAP Length field.
func eaplenOverlong(t *testing.T, sender *SA, code uint8, messageID uint32, flags uint8) []byte {
	t.Helper()
	payloadOctets := wire.GenericHeaderLen + eaplenCarriedOctets
	inner := []byte{
		0, 0, byte(payloadOctets >> 8), byte(payloadOctets),
		code, 7, 0, eaplenDeclaredOctets, 1,
	}

	var raw []byte
	var err error
	if sender.Proposal.Encryption.IsAEAD {
		raw, err = buildSKMessageAEADWithMsgID(sender, inner, wire.PayloadTypeEAP, messageID, wire.ExchangeIKEAuth, flags)
	} else {
		raw, err = buildSKMessageCBCWithMsgID(sender, inner, wire.PayloadTypeEAP, messageID, wire.ExchangeIKEAuth, flags)
	}
	if err != nil {
		t.Fatalf("seal the overlong EAP message: %v", err)
	}
	return raw
}

// eaplenMessageID reads the Message ID of a sealed IKE message.
func eaplenMessageID(raw []byte) uint32 {
	return binary.BigEndian.Uint32(raw[20:24])
}

// VALIDATES: ze as EAP authenticator silently discards an EAP Response whose Length
// field claims more octets than it carries, and keeps the EAP exchange running.
//
// METHOD: a real EAP-MSCHAPv2 exchange reaches its second IKE_AUTH round. The
// initiator's real round-2 request is held back, and an overlong EAP Response is
// sealed under the initiator's keys at the same Message ID and delivered first. The
// held-back request is then delivered.
//
// PREVENTS: handleResponderEAP ending the IKE SA (StateDead) on the parse error.
//
// RFC requirement: RFC3748-4-1 negative -- the overlong EAP Response leaves the responder in StateEAPInProgress and writes no datagram at all: no EAP message, no IKE notification.
// RFC requirement: RFC3748-4-1 positive -- the peer's real round-2 request, whose EAP Length equals its octets, is then processed at the same Message ID and draws an answer, so the discard consumed nothing of the exchange.
func TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, ps, authReq := eaplenExchange(t)
	peerTr, myTr := rtxPeerLink(t, resp)
	peerAddr := eaprtxPeerAddr(t, peerTr)

	ps.handleResponderInbound(resp, parseMsg(t, authReq), transport.Packet{Data: authReq, RemoteAddr: peerAddr}, myTr, log)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the first IKE_AUTH drew no EAP request")
	}
	handleInbound(ini, transport.Packet{Data: first}, table, nil, log)
	if ini.State != StateEAPInProgress {
		t.Fatalf("initiator state after the first EAP request = %v, want EAP in progress", ini.State)
	}
	round2 := ini.LastSentMsg

	overlong := eaplenOverlong(t, ini, wire.EAPCodeResponse, eaplenMessageID(round2), initiatorFlag(ini))
	ps.handleResponderInbound(resp, parseMsg(t, overlong), transport.Packet{Data: overlong, RemoteAddr: peerAddr}, myTr, log)
	if resp.State != StateEAPInProgress {
		t.Fatalf("responder state after the overlong EAP Response = %v, want EAP in progress", resp.State)
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "the overlong EAP Response")

	ps.handleResponderInbound(resp, parseMsg(t, round2), transport.Packet{Data: round2, RemoteAddr: peerAddr}, myTr, log)
	if resp.State == StateDead {
		t.Fatal("the well-formed round-2 request killed the IKE SA")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the well-formed round-2 request drew no answer after the discard")
	}
}

// VALIDATES: ze as EAP peer silently discards an EAP Request whose Length field claims
// more octets than it carries, on the first IKE_AUTH response and on a later EAP
// round, and keeps the exchange where it was.
//
// METHOD: the responder's real first IKE_AUTH response is held back, and an overlong
// EAP Request sealed under the responder's keys at the same Message ID reaches the
// initiator first through handleInbound. The same is done on the second round. After
// each discard the held-back real message is delivered.
//
// PREVENTS: handleAuthResponse and handleEAPResponse ending the IKE SA (StateDead) on
// the parse error.
//
// RFC requirement: RFC3748-4-1 negative -- each overlong EAP Request leaves the initiator's state, its next Message ID and its last sent message unchanged, so nothing was answered.
// RFC requirement: RFC3748-4-1 positive -- the real message at the same Message ID, whose EAP Length equals its octets, is then processed: the initiator answers it with a new IKE_AUTH request.
func TestRFC3748PeerDiscardsAnOverlongEAPLength(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, table, ps, authReq := eaplenExchange(t)
	peerTr, myTr := rtxPeerLink(t, resp)
	peerAddr := eaprtxPeerAddr(t, peerTr)

	ps.handleResponderInbound(resp, parseMsg(t, authReq), transport.Packet{Data: authReq, RemoteAddr: peerAddr}, myTr, log)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the first IKE_AUTH drew no EAP request")
	}

	responseFlags := initiatorFlag(resp) | wire.FlagResponse
	eaplenExpectDiscard(t, ini, table, eaplenOverlong(t, resp, wire.EAPCodeRequest, eaplenMessageID(first), responseFlags), "first IKE_AUTH response")

	handleInbound(ini, transport.Packet{Data: first}, table, nil, log)
	if ini.State != StateEAPInProgress {
		t.Fatalf("initiator state after the real first response = %v, want EAP in progress", ini.State)
	}
	round2 := ini.LastSentMsg
	if bytes.Equal(round2, authReq) {
		t.Fatal("the real first response drew no new IKE_AUTH request")
	}

	ps.handleResponderInbound(resp, parseMsg(t, round2), transport.Packet{Data: round2, RemoteAddr: peerAddr}, myTr, log)
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the round-2 request drew no answer")
	}

	eaplenExpectDiscard(t, ini, table, eaplenOverlong(t, resp, wire.EAPCodeRequest, eaplenMessageID(second), responseFlags), "round-2 EAP response")

	handleInbound(ini, transport.Packet{Data: second}, table, nil, log)
	if ini.State == StateDead {
		t.Fatal("the real round-2 answer killed the IKE SA")
	}
	if bytes.Equal(ini.LastSentMsg, round2) {
		t.Fatal("the real round-2 answer drew no new IKE_AUTH request after the discard")
	}
}

// eaplenExpectDiscard delivers overlong to the initiator and requires that it changed
// nothing the exchange depends on.
func eaplenExpectDiscard(t *testing.T, ini *SA, table *SATable, overlong []byte, what string) {
	t.Helper()
	state, nextMsgID, lastSent := ini.State, ini.NextMsgID, ini.LastSentMsg

	handleInbound(ini, transport.Packet{Data: overlong}, table, nil, slogutil.DiscardLogger())
	if ini.State != state {
		t.Fatalf("%s: initiator state after the overlong EAP Request = %v, want %v", what, ini.State, state)
	}
	if ini.NextMsgID != nextMsgID {
		t.Fatalf("%s: next Message ID moved from %d to %d", what, nextMsgID, ini.NextMsgID)
	}
	if !bytes.Equal(ini.LastSentMsg, lastSent) {
		t.Fatalf("%s: the initiator built a new message in answer to the overlong EAP Request", what)
	}
}
