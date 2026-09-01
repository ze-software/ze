package engine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// The three tests in this file are WIRING tests. Each one drives handleInbound,
// the function the UDP dispatch calls for every datagram that belongs to an
// existing SA, and reads what ze wrote back off a loopback socket. Nothing here
// calls eap.PeerSession.Process, because a green from that call proves the peer
// logic and says nothing about whether an IKE_AUTH message can reach it.
//
// The chain each test exercises is handleInbound -> handleEAPResponse ->
// PeerSession.Process -> handleRequest, and back out through
// sendEAPResponsePacket -> buildEAPResponse -> sendRaw.
//
// They carry no `RFC requirement:` tag. The RFC proofs for Types 2 and 3 live in
// internal/component/ike/eap; these prove reachability.

const (
	// eapwIdentity and eapwPassword are the EAP-MSCHAPv2 credentials both ends of
	// the stand-in exchange share.
	eapwIdentity = "eap-testuser"
	eapwPassword = "eap-secret"

	// eapwUnacceptableType is an authentication Type inside RFC 3748 Section
	// 5.3.1's 4-253 range that ze does not run. 40 is EAP-SIM.
	eapwUnacceptableType uint8 = 40
)

// eapwSession stands up an initiator SA in the middle of an EAP exchange, wired
// to a loopback stand-in for the authenticator.
//
// The SA holds real SK keys, so every message in and out of it is encrypted and
// integrity protected the way a real IKE_AUTH round is. peerTr reads every
// datagram ze sends and myTr is the socket ze sends from. The returned buffer
// collects the engine's own log lines, which is where handleEAPResponse renders
// PeerResult.Discarded and PeerResult.Notified.
func eapwSession(t *testing.T) (sa *SA, peerTr, myTr *transport.UDPTransport, logged *bytes.Buffer, log *slog.Logger) {
	t.Helper()

	sa = testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthEAPMSCHAPv2
	sa.PeerCfg.Auth.PSK = eapwPassword
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	sa.EAPSession = eap.NewPeerSession(eap.TypeMSCHAPv2, eapwIdentity, eapwPassword)
	sa.State = StateEAPInProgress

	peerTr, myTr = rtxPeerLink(t)

	logged = &bytes.Buffer{}
	log = slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return sa, peerTr, myTr, logged, log
}

// eapwFarEnd is the authenticator's view of the same IKE SA: one SKKeys, the
// opposite role. It seals what the authenticator sends and opens what ze sends.
func eapwFarEnd(sa *SA) *SA {
	far := *sa
	far.IsInitiator = !sa.IsInitiator
	return &far
}

// eapwDeliver hands the SA one IKE_AUTH response carrying a single EAP Request,
// through the entry point the UDP dispatch uses.
//
// The datagram is a real encrypted IKE_AUTH message built by the far end, so the
// SK decrypt, the inner payload chain parse and the StateEAPInProgress arm of
// handleInbound all run before the EAP peer sees anything.
func eapwDeliver(t *testing.T, sa *SA, tr *transport.UDPTransport, log *slog.Logger, req *eap.Packet) {
	t.Helper()

	inner := []wire.PayloadEntry{{Payload: eapToWire(req)}}
	raw, err := buildEncryptedMessageEx(eapwFarEnd(sa), inner, sa.NextMsgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the authenticator's IKE_AUTH carrying an EAP Request of Type %d: %v", req.Type, err)
	}
	handleInbound(sa, transport.Packet{Data: raw}, nil, tr, log)
}

// eapwAnswer returns the EAP payload ze put on the wire, read the way the far end
// reads it. It fails the test when ze sent nothing, when the message does not
// authenticate under the authenticator's keys, or when it carries no EAP payload.
func eapwAnswer(t *testing.T, sa *SA, peerTr *transport.UDPTransport) *wire.PayloadEAP {
	t.Helper()

	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("ze wrote no IKE_AUTH datagram")
	}
	inner, err := decryptAndParse(eapwFarEnd(sa), parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the datagram ze sent did not authenticate under the IKE SA: %v", err)
	}
	for i := range inner {
		if p, ok := inner[i].Payload.(*wire.PayloadEAP); ok {
			return p
		}
	}
	t.Fatal("the message ze sent carries no EAP payload")
	return nil
}

// eapwRequest builds one EAP Request as the authenticator would send it.
func eapwRequest(id, typ uint8, data []byte) *eap.Packet {
	return &eap.Packet{Code: eap.CodeRequest, Identifier: id, Type: typ, TypeData: data}
}

// TestEngineAnswersANotificationRequest proves an IKE_AUTH message carrying an EAP
// Request of Type 2 reaches the peer's Notification path and that the Type-2
// Response it produces leaves through the engine's send path.
//
// Method: an SA in StateEAPInProgress is given one encrypted IKE_AUTH response
// carrying a Type-2 Request, through handleInbound. The answer is read back off
// the loopback socket and decrypted under the authenticator's view of the SA.
// The Notification message is checked in the engine's log, which is the only
// place PeerResult.Notification surfaces, and the SA is checked to be alive.
func TestEngineAnswersANotificationRequest(t *testing.T) {
	sa, peerTr, myTr, logged, log := eapwSession(t)

	const notice = "your password expires in three days"
	eapwDeliver(t, sa, myTr, log, eapwRequest(7, eap.TypeNotification, []byte(notice)))

	got := eapwAnswer(t, sa, peerTr)
	if got.Code != eap.CodeResponse {
		t.Errorf("ze answered with Code %d, want %d (Response)", got.Code, eap.CodeResponse)
	}
	if got.Identifier != 7 {
		t.Errorf("ze answered with Identifier %d, want 7 (the Request's)", got.Identifier)
	}
	if len(got.EAPData) == 0 {
		t.Fatal("ze answered with no Type field")
	}
	if got.EAPData[0] != eap.TypeNotification {
		t.Errorf("ze answered with Type %d, want %d (Notification)", got.EAPData[0], eap.TypeNotification)
	}
	if len(got.EAPData) != 1 {
		t.Errorf("the Notification Response carries %d Type-Data octets, want 0", len(got.EAPData)-1)
	}
	if got.Len() != 5 {
		t.Errorf("the Notification Response is %d octets long, want 5", got.Len())
	}

	if !strings.Contains(logged.String(), notice) {
		t.Errorf("the engine never logged the Notification message; log was:\n%s", logged.String())
	}
	if sa.State != StateEAPInProgress {
		t.Errorf("the SA moved to %s, want it left in %s", sa.State, StateEAPInProgress)
	}
}

// TestEngineSendsANakForAnUnacceptableType proves an IKE_AUTH message carrying an
// EAP Request for a method ze does not run reaches the peer's Nak path, and that
// the Type-3 Response naming the configured method leaves through the engine.
//
// Method: an SA in StateEAPInProgress, configured for EAP-MSCHAPv2, is given one
// encrypted IKE_AUTH response carrying a Type-40 Request through handleInbound.
// The answer is read off the loopback socket and its single Type-Data octet is
// compared against eap.TypeMSCHAPv2, the method the SA is configured for.
func TestEngineSendsANakForAnUnacceptableType(t *testing.T) {
	sa, peerTr, myTr, _, log := eapwSession(t)

	eapwDeliver(t, sa, myTr, log, eapwRequest(11, eapwUnacceptableType, nil))

	got := eapwAnswer(t, sa, peerTr)
	if got.Code != eap.CodeResponse {
		t.Errorf("ze answered with Code %d, want %d (Response)", got.Code, eap.CodeResponse)
	}
	if got.Identifier != 11 {
		t.Errorf("ze answered with Identifier %d, want 11 (the Request's)", got.Identifier)
	}
	if len(got.EAPData) != 2 {
		t.Fatalf("the Nak carries %d octets after the EAP header, want 2 (Type and one desired Type)", len(got.EAPData))
	}
	if got.EAPData[0] != eap.TypeNAK {
		t.Errorf("ze answered with Type %d, want %d (Nak)", got.EAPData[0], eap.TypeNAK)
	}
	if got.EAPData[1] != eap.TypeMSCHAPv2 {
		t.Errorf("the Nak asks for Type %d, want %d (the configured method)", got.EAPData[1], eap.TypeMSCHAPv2)
	}
	if got.Len() != 6 {
		t.Errorf("the Nak is %d octets long, want 6", got.Len())
	}

	if sa.State != StateEAPInProgress {
		t.Errorf("the SA moved to %s, want it left in %s", sa.State, StateEAPInProgress)
	}
}

// TestPeerDiscardLeavesTheSAAlive proves an unexpected Request arriving after the
// peer has committed to its method is silently discarded by the whole engine
// path: no EAP message goes out, the SA stays in StateEAPInProgress, and the
// engine records the discard.
//
// This is A-3 and R-4 of plan/spec-eap-notification-and-nak.md. It is the case an
// all-zero PeerResult would also produce on the wire, so the wire alone cannot
// tell the two apart. The log line is what separates them: handleEAPResponse
// writes it only when PeerResult.Discarded is set.
//
// Method: a real EAP-MSCHAPv2 authenticator (eap.Session) drives the identity
// round and the challenge round over the same handleInbound path, which is what
// commits the peer to its method. A Type-40 Request then follows. Silence is
// proven by a sentinel datagram sent on the same loopback socket after the
// delivery: one socket keeps send order, so the sentinel arriving first proves ze
// wrote nothing.
func TestPeerDiscardLeavesTheSAAlive(t *testing.T) {
	sa, peerTr, myTr, logged, log := eapwSession(t)

	server, err := eap.NewSession(eap.TypeMSCHAPv2, eap.MethodConfig{Password: eapwPassword})
	if err != nil {
		t.Fatalf("stand up the EAP-MSCHAPv2 authenticator: %v", err)
	}
	t.Cleanup(server.Close)

	// Round one: the Identity Request, answered with the Identity Response.
	eapwDeliver(t, sa, myTr, log, server.Begin())
	identity := wireEAPToPacket(eapwAnswer(t, sa, peerTr))
	if identity.Type != eap.TypeIdentity {
		t.Fatalf("ze answered the Identity Request with Type %d, want %d", identity.Type, eap.TypeIdentity)
	}

	// Round two: the MS-CHAPv2 Challenge. Answering it is what sets the peer's
	// method commitment, so RFC 3748 Section 2.1 forbids a Nak from here on.
	challenge := server.Process(identity)
	if challenge == nil {
		t.Fatal("the authenticator produced no MS-CHAPv2 Challenge")
	}
	eapwDeliver(t, sa, myTr, log, challenge)
	answered := wireEAPToPacket(eapwAnswer(t, sa, peerTr))
	if answered.Type != eap.TypeMSCHAPv2 {
		t.Fatalf("ze answered the Challenge with Type %d, want %d, so the peer never committed",
			answered.Type, eap.TypeMSCHAPv2)
	}

	// Round three: a Request for a Type nothing asked for, after the commitment.
	before := logged.Len()
	eapwDeliver(t, sa, myTr, log, eapwRequest(0x51, eapwUnacceptableType, nil))

	rtxExpectSilence(t, peerTr, myTr, eaprtxPeerAddr(t, peerTr),
		"a Request of an unexpected Type after the peer committed to its method")

	if sa.State != StateEAPInProgress {
		t.Errorf("the SA moved to %s, want it left in %s", sa.State, StateEAPInProgress)
	}
	if sa.State == StateDead {
		t.Error("the discarded Request killed the IKE SA")
	}
	if round := logged.String()[before:]; !strings.Contains(round, "EAP packet discarded") {
		t.Errorf("the engine did not record the discard, so the outcome was not PeerResult.Discarded; log was:\n%s", round)
	}
}
