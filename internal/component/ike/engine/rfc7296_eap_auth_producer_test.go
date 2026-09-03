package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Every test in this file drives computeEAPAuth, which auth.go and responder.go
// call to build the AUTH payloads of messages 7 and 8, and each one is red under a
// mutation keying computeEAPAuth from a constant instead of sa.EAPMSK. The tests
// in rfc7296_wp2_test.go exercise computeAuthFromSharedSecret, the primitive, and stayed
// green under that mutation, which is why the RFC7296-2.16 tags sit here.

// eapProducerAUTH returns the AUTH payload the message-7/8 producer builds for sa.
func eapProducerAUTH(t *testing.T, sa *SA, what string) *wire.PayloadAUTH {
	t.Helper()
	auth, err := computeEAPAuth(sa)
	if err != nil {
		t.Fatalf("computeEAPAuth (%s): %v", what, err)
	}
	if auth == nil || len(auth.AuthData) == 0 {
		t.Fatalf("computeEAPAuth (%s) produced no AUTH data", what)
	}
	return auth
}

// VALIDATES: the AUTH payload ze puts in messages 7 and 8 is keyed by the shared key the
// EAP method produced, and by nothing else standing in for it.
//
// PREVENTS: the defect this file exists for. computeEAPAuth is the producer both auth.go
// and responder.go call; keying it from any constant, or from a field other than sa.EAPMSK,
// is invisible to a test that only exercises computeAuthFromSharedSecret with a hand-built key.
//
// The MSK is the ONLY input changed between the two calls, and the SA comes from a real EAP
// handshake, so the difference cannot come from anywhere else.
//
// RFC requirement: RFC7296-2.16-12 positive -- RFC 7296 Section 2.16: "For EAP methods that
// create a shared key as a side effect of authentication, that shared key MUST be used by
// both the initiator and responder to generate AUTH payloads in messages 7 and 8 using the
// syntax for shared secrets specified in Section 2.15". Both sides hold the same negotiated
// MSK, and the producer's output on each side is the value that MSK generates.
func TestEapAuthProducerIsKeyedByTheNegotiatedMSK(t *testing.T) {
	ini, resp, _ := autEAPHandshake(t)

	if resp.EAPMSK == ([64]byte{}) {
		t.Fatal("the responder holds no EAP MSK after the handshake, so nothing below is keyed by one")
	}
	if ini.EAPMSK != resp.EAPMSK {
		t.Fatal("the two sides hold different EAP MSK values, so no shared key generated both AUTH payloads")
	}

	// Both sides must generate the AUTH of messages 7 and 8 from that shared key.
	fromResponder := eapProducerAUTH(t, resp, "responder, negotiated MSK")
	expected, err := computeAuthFromSharedSecret(resp.Proposal.PRF.ID, resp.EAPMSK[:], mustSignedOctets(t, resp))
	if err != nil {
		t.Fatalf("computeAuthFromSharedSecret: %v", err)
	}
	if !bytes.Equal(fromResponder.AuthData, expected) {
		t.Error("the AUTH the producer builds is not the value the negotiated EAP shared key generates")
	}

	// The MSK is the only input that changes.
	swapped := resp.EAPMSK
	for i := range swapped {
		swapped[i] ^= 0xA5
	}
	resp.EAPMSK = swapped
	afterSwap := eapProducerAUTH(t, resp, "responder, swapped MSK")

	if bytes.Equal(fromResponder.AuthData, afterSwap.AuthData) {
		t.Error("the AUTH of messages 7 and 8 did not change when the EAP shared key changed, " +
			"so it is keyed by something other than that key")
	}
}

// RFC requirement: RFC7296-2.16-12 negative -- an AUTH built from a DIFFERENT shared key is
// not the one the negotiated key generates, and the verifier refuses it. The producer is
// therefore bound to the EAP result rather than emitting any well-formed AUTH.
func TestEapAuthProducerOutputIsRefusedUnderAnotherKey(t *testing.T) {
	_, resp, _ := autEAPHandshake(t)

	negotiated := resp.EAPMSK
	if negotiated == ([64]byte{}) {
		t.Fatal("the responder holds no EAP MSK, so this is not the negative case")
	}
	signed := mustSignedOctets(t, resp)

	// The AUTH the producer emits under a DIFFERENT key must not verify under the real one.
	other := negotiated
	for i := range other {
		other[i] ^= 0x5A
	}
	resp.EAPMSK = other
	underOther := eapProducerAUTH(t, resp, "responder, other MSK")

	if err := verifyAuthFromSharedSecret(resp.Proposal.PRF.ID, negotiated[:], signed, underOther.AuthData); err == nil {
		t.Error("an AUTH the producer keyed from another EAP shared key verified under the negotiated one")
	}

	// And the producer's output under the real key does verify, so the refusal above is the
	// key talking and not a verifier that rejects everything.
	resp.EAPMSK = negotiated
	underReal := eapProducerAUTH(t, resp, "responder, negotiated MSK")
	if err := verifyAuthFromSharedSecret(resp.Proposal.PRF.ID, negotiated[:], signed, underReal.AuthData); err != nil {
		t.Errorf("the AUTH the producer keyed from the negotiated shared key did not verify: %v", err)
	}
}

// mustSignedOctets returns the signed octets the producer signs for sa's own role.
func mustSignedOctets(t *testing.T, sa *SA) []byte {
	t.Helper()
	octets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		t.Fatalf("computeSignedOctets: %v", err)
	}
	return octets
}

// eapRoundShape records what one delivery of the EAP handshake carried.
type eapRoundShape struct {
	hasEAPSuccess bool
	hasAUTH       bool
}

// eapHandshakeShapes drives a full in-process EAP-MSCHAPv2 handshake and returns, per
// delivery after IKE_SA_INIT and in order, whether the decrypted payload chain carried an
// EAP Success and whether it carried an AUTH payload.
//
// It follows autEAPHandshake, and records the payload SHAPE rather than the exchange type,
// which is what the Section 2.16 ordering above is stated over. The chains are read from
// the real encrypted datagrams, so a producer that emits AUTH on a different message moves
// what this sees.
func eapHandshakeShapes(t *testing.T) []eapRoundShape {
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

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.setSA(resp)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)

	var shapes []eapRoundShape
	cur := ini.LastSentMsg
	toResponder := true
	for range 24 {
		receiver := resp
		if !toResponder {
			receiver = ini
		}
		shapes = append(shapes, eapShapeOf(t, receiver, cur))
		if toResponder {
			handleInbound(resp, transport.Packet{Data: cur}, table, nil, log)
			cur = resp.LastSentMsg
		} else {
			handleInbound(ini, transport.Packet{Data: cur}, table, nil, log)
			cur = ini.LastSentMsg
		}
		if resp.State == StateDead || ini.State == StateDead {
			t.Fatalf("the EAP handshake died at delivery %d (ini=%v resp=%v)", len(shapes), ini.State, resp.State)
		}
		if ini.State == StateEstablished && resp.State == StateEstablished {
			return shapes
		}
		toResponder = !toResponder
	}
	t.Fatalf("the EAP handshake did not establish (ini=%v resp=%v)", ini.State, resp.State)
	return nil
}

// eapShapeOf decrypts one delivery under the SA that received it and reports its shape.
func eapShapeOf(t *testing.T, sa *SA, raw []byte) eapRoundShape {
	t.Helper()
	inner, err := decryptAndParse(sa, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("an EAP handshake delivery did not authenticate under the receiving SA: %v", err)
	}
	var shape eapRoundShape
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadEAP:
			if p.Code == eap.CodeSuccess {
				shape.hasEAPSuccess = true
			}
		case *wire.PayloadAUTH:
			shape.hasAUTH = true
		}
	}
	return shape
}

// VALIDATES: the two messages following the one carrying EAP Success both carry an AUTH
// payload, and no message before Success does.
//
// PREVENTS: an implementation that emits the EAP AUTH early (keyed from a half-finished
// method) or late (leaving the peer without the AUTH it must verify).
//
// This walks the payload chains of a REAL handshake rather than reasoning from when the MSK
// field happens to be populated.
//
// RFC requirement: RFC7296-2.16-13 positive -- RFC 7296 Section 2.16: "Following such an
// extended exchange, the EAP AUTH payloads MUST be included in the two messages following
// the one containing the EAP Success message."
// RFC requirement: RFC7296-2.16-13 negative -- no delivery at or before the EAP Success
// carries an AUTH payload, so the two that do are ordered by the Success and are not simply
// every message of the exchange.
func TestEapAuthFollowsTheSuccessMessage(t *testing.T) {
	shapes := eapHandshakeShapes(t)

	success := -1
	for i := range shapes {
		t.Logf("delivery %d: eap-success=%v auth=%v", i, shapes[i].hasEAPSuccess, shapes[i].hasAUTH)
		if shapes[i].hasEAPSuccess && success < 0 {
			success = i
		}
	}
	if success < 0 {
		t.Fatal("the handshake carried no EAP Success, so the ordering below has no anchor")
	}
	if success < 1 {
		t.Fatal("the EAP Success was the first delivery, so there is no EAP method exchange before it")
	}
	if success+2 >= len(shapes) {
		t.Fatalf("EAP Success arrived at delivery %d of %d, leaving fewer than the two following messages the section names",
			success, len(shapes))
	}

	// Positive half: the two messages following the one carrying Success both carry AUTH.
	for _, i := range []int{success + 1, success + 2} {
		if !shapes[i].hasAUTH {
			t.Errorf("delivery %d follows the EAP Success at %d but carries no AUTH payload", i, success)
		}
	}
	// And the exchange ENDS there. Two messages, not a stream of them.
	if len(shapes) != success+3 {
		t.Errorf("the handshake ran to %d deliveries, want it to end at %d, two after the EAP Success",
			len(shapes), success+3)
	}

	// Negative half. The AUTH FOLLOWS the Success: neither the message carrying Success nor
	// the one before it carries an AUTH payload. Without this the positive half would hold
	// over an implementation that put an AUTH on every message of the exchange.
	if shapes[success].hasAUTH {
		t.Error("the message carrying the EAP Success also carries an AUTH payload, " +
			"so the AUTH accompanies the Success instead of following it")
	}
	if shapes[success-1].hasAUTH {
		t.Errorf("delivery %d, the last EAP method exchange before Success, carries an AUTH payload", success-1)
	}
}
