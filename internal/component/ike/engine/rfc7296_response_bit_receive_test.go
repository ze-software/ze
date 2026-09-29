// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the IKE header flags
// Related: rfc7296_header_test.go -- TestResponseBitMatchesDirection, the sending side
// VALIDATES: the initiator does not take a peer's IKE_SA_INIT answer whose R bit is clear
// as the response, and does take the same answer with the R bit set (RFC 7296 Section 3.1).
// PREVENTS: an initiator that recognizes a response by its exchange type alone, so a
// peer that violates the R-bit rule advances the handshake.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296InitiatorIgnoresAResponseWithTheRBitClear feeds the initiator's inbound
// handler the responder's real IKE_SA_INIT response, first with its R bit cleared and then
// as sent.
//
// Goal: RFC 7296 Section 3.1 binds the sender, and Ze's side of a peer's violation is
// that the violating message is not treated as a response. Method: the initiator waits in
// StateSAInitSent. handleInbound gets the response with the R bit cleared, which a
// compliant responder never sends, and the initiator MUST stay in StateSAInitSent. The
// untouched response then MUST move it on, so the refusal is the R bit's doing.
//
// RFC 7296 Section 3.1: "R (Response) - This bit indicates that this message is a response
// to a message containing the same Message ID. This bit MUST be cleared in all request
// messages and MUST be set in all responses."
//
// RFC requirement: RFC7296-3.1-9 negative -- a peer's IKE_SA_INIT response with the R bit cleared is not processed as a response by the initiator's handleInbound (it stays in StateSAInitSent); the same response with the R bit set is processed.
func TestRFC7296InitiatorIgnoresAResponseWithTheRBitClear(t *testing.T) {
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rbit-receive-psk")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	request := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = request
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, request), request, nil, nil, log)
	response := resp.LastSentMsg
	if len(response) < wire.HeaderLen || response[19]&wire.FlagResponse == 0 {
		t.Fatal("the responder built no IKE_SA_INIT response with the R bit set")
	}

	cleared := append([]byte(nil), response...)
	cleared[19] &^= wire.FlagResponse
	handleInbound(ini, transport.Packet{Data: cleared}, table, nil, log)
	if ini.State != StateSAInitSent {
		t.Fatalf("initiator state = %v after an IKE_SA_INIT answer with the R bit clear, want %v",
			ini.State, StateSAInitSent)
	}

	handleInbound(ini, transport.Packet{Data: response}, table, nil, log)
	if ini.State == StateSAInitSent {
		t.Fatal("the same answer with the R bit set was not processed, so the refusal above proves nothing")
	}
}
