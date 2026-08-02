// RFC 7296 Section 4, the four-message IKE_SA_INIT and IKE_AUTH capability.
//
// Section 4 names the one exchange every implementation must be able to perform: four
// messages, IKE_SA_INIT then IKE_AUTH, establishing two SAs, one for IKE and one for ESP
// or AH. The POSITIVE half of that obligation is proven by
// TestResponderHandshakePSKEndToEnd (responder_test.go), which drives all four messages
// between a real initiator and a real responder and asserts both SAs. This file carries
// the negative half: the establishment is caused by the four messages, and a prefix of
// them establishes nothing.
//
// Helpers here start with `fourm`.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: nothing is established by the first request/response pair alone, and the
// initiator is not established until the fourth message reaches it.
// PREVENTS: a handshake that reports an SA before the peer has authenticated, which would
// make the positive four-message proof vacuous by establishing on message one.
// RFC requirement: RFC7296-4-5 negative -- after IKE_SA_INIT and its response,
// handleSAInitRequest and handleSAInitResponse (fsm.go) leave the responder at
// StateSAInitReceived with no Child SA and the initiator at StateAuthSent. Only
// PeerSession.handleAuthRequest (auth.go) and handleAuthResponse (fsm.go), the third and
// fourth messages, reach StateEstablished and install the ESP SA. So the two SAs the
// positive asserts are delivered by the whole four-message exchange, not by its first pair.
func TestFourmFirstPairEstablishesNeitherSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "four-message-secret")

	table := NewSATable()

	// Message 1: the initiator's IKE_SA_INIT request.
	iniSA, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(iniSA)
	saInitReq := buildSAInitRequest(iniSA, ikeGroup)
	iniSA.InitiatorSAInitMsg = saInitReq
	iniSA.State = StateSAInitSent

	// Message 2: the responder's IKE_SA_INIT response.
	respSA, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, iniSA.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	handleSAInitRequest(respSA, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(iniSA, parseMsg(t, respSA.LastSentMsg), respSA.LastSentMsg, table, nil, nil, log)

	// Two messages in, neither SA of the pair exists.
	if respSA.State == StateEstablished {
		t.Error("the responder is established after IKE_SA_INIT, before the peer has authenticated")
	}
	if iniSA.State == StateEstablished {
		t.Error("the initiator is established after the IKE_SA_INIT response, before the peer has authenticated")
	}
	if ps.getChildSA() != nil {
		t.Error("a Child SA exists after IKE_SA_INIT, before any ESP SA was negotiated")
	}

	// Message 3: the initiator's IKE_AUTH request. The responder answers it and reaches
	// established, so the fourth message exists and is simply withheld here.
	authReq := iniSA.LastSentMsg
	ps.handleAuthRequest(respSA, parseMsg(t, authReq), authReq, nil, nil, log)
	if respSA.State != StateEstablished {
		t.Fatalf("responder state after IKE_AUTH = %v, want established", respSA.State)
	}
	if len(respSA.LastSentMsg) == 0 {
		t.Fatal("the responder produced no IKE_AUTH response, so there is no fourth message to withhold")
	}

	// The fourth message is never delivered. The initiator therefore has no IKE SA and no
	// ESP SA of its own, although the responder has both.
	if iniSA.State == StateEstablished {
		t.Error("the initiator is established without receiving the IKE_AUTH response")
	}
}
