package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// podPair builds an initiator and a responder that hold one shared key and name each
// other by identity. The two identities differ, so a check that reads the local
// configuration in place of the peer's ID payload produces the wrong octets.
func podPair(t *testing.T) (ini, resp *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()

	iniAuth := ipsec.AuthConfig{
		Mode:     ipsec.AuthPreSharedSecret,
		PSK:      "pod-shared-secret",
		LocalID:  "initiator.example.com",
		RemoteID: "responder.example.com",
	}
	respAuth := ipsec.AuthConfig{
		Mode:     ipsec.AuthPreSharedSecret,
		PSK:      "pod-shared-secret",
		LocalID:  "responder.example.com",
		RemoteID: "initiator.example.com",
	}
	iniPeer := ipsec.SiteToSitePeer{
		Name: "ze", IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionInitiate,
		LocalAddress:   "10.0.0.1", RemoteAddress: "10.0.0.2", Auth: iniAuth,
	}
	respPeer := ipsec.SiteToSitePeer{
		Name: "ze", IKEGroup: "test-ike", ESPGroup: "test-esp",
		ConnectionType: ipsec.ConnectionRespond,
		LocalAddress:   "10.0.0.2", RemoteAddress: "10.0.0.1", Auth: respAuth,
	}

	table := NewSATable()
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
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("podPair: responder is %v, want StateSAInitReceived", resp.State)
	}
	ini.State = StateAuthSent
	return ini, resp
}

// podAuthResponse encodes an IKE_AUTH response carrying IDr and AUTH in the requested
// order. authFirst puts the AUTH payload before the identification payload, which is the
// order RFC 7296 Section 2.5 permits and Section 1.2 does not show.
func podAuthResponse(t *testing.T, resp *SA, authFirst, corrupt bool) []byte {
	t.Helper()
	idPayload := buildIDPayload(resp, false)
	authPayload, err := computeLocalAuth(resp)
	if err != nil {
		t.Fatalf("compute the responder AUTH: %v", err)
	}
	if corrupt {
		authPayload.AuthData[0] ^= 0xff
	}

	inner := []wire.PayloadEntry{{Payload: idPayload}, {Payload: authPayload}}
	if authFirst {
		inner = []wire.PayloadEntry{{Payload: authPayload}, {Payload: idPayload}}
	}
	raw, err := buildEncryptedMessageEx(resp, inner, 1, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the IKE_AUTH response: %v", err)
	}
	return raw
}

// VALIDATES: RFC7296-2.5-13. An IKE_AUTH response whose AUTH payload precedes its
// identification payload authenticates, exactly as the shown order does.
// PREVENTS: the initiator refusing a conformant peer over payload order.
// handleAuthResponse verified AUTH from inside the payload walk, so the identity the
// signature covers was still unread. The peer saw AUTHENTICATION_FAILED, and the
// operator saw "peer sent no identification payload" about a peer that sent one.
// RFC requirement: RFC7296-2.5-13 positive -- handleAuthResponse (fsm.go) collects the
// whole payload chain and verifies AUTH after the walk, so neither order is refused.
// RFC requirement: RFC7296-2.5-13 negative -- the same unconventional order with a
// corrupted AUTH payload is still refused, so acceptance of an order is not acceptance
// of the message.
func TestPodAuthResponseAcceptsAuthBeforeIdentity(t *testing.T) {
	log := slogutil.DiscardLogger()

	// Control. The order RFC 7296 Section 1.2 shows establishes the SA.
	shown, respShown := podPair(t)
	raw := podAuthResponse(t, respShown, false, false)
	handleAuthResponse(shown, parseMsg(t, raw), raw, nil, nil, log)
	if shown.State != StateEstablished {
		t.Fatalf("the shown payload order left the SA at %v, want StateEstablished", shown.State)
	}

	// Positive. AUTH before IDr carries the same message and MUST NOT be rejected.
	reordered, respReordered := podPair(t)
	raw = podAuthResponse(t, respReordered, true, false)
	handleAuthResponse(reordered, parseMsg(t, raw), raw, nil, nil, log)
	if reordered.State != StateEstablished {
		t.Fatalf("an IKE_AUTH response with AUTH before IDr left the SA at %v, "+
			"want StateEstablished; RFC 7296 Section 2.5 forbids rejecting it for order",
			reordered.State)
	}

	// Negative. The same order with a corrupted AUTH payload is still refused.
	forged, respForged := podPair(t)
	raw = podAuthResponse(t, respForged, true, true)
	handleAuthResponse(forged, parseMsg(t, raw), raw, nil, nil, log)
	if forged.State == StateEstablished {
		t.Fatal("a corrupted AUTH payload established the SA once the order was unconventional")
	}
}
