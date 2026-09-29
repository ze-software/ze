// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the IKE header flags
// Related: rfc7296_critical_bit_test.go -- engineBuiltChains; rfc7296_wp2_test.go -- the DPD probe
// VALIDATES: the I (Initiator) flag of every message kind the engine builds follows the
// ORIGINAL role of the sender in the IKE SA, in requests and in responses alike (RFC 7296
// Section 3.1).
// PREVENTS: a builder that sets the I flag from the direction of the exchange, so the
// original responder's request or the original initiator's response carries the wrong bit.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296InitiatorBitFollowsTheOriginalRole reads the I flag of one message of every
// kind the engine emits, and of the two messages where the exchange direction and the
// original role disagree.
//
// Goal: the flags are passed per call site, so one message kind proves nothing about the
// others. Method: every message engineBuiltChains returns is sent by the original initiator
// when it is a request and by the original responder when it is a response, so its I flag
// MUST be set on a request and clear on a response. Two more messages break that pairing:
// an INFORMATIONAL Delete REQUEST built for the original responder MUST carry a clear I
// flag, and the INFORMATIONAL RESPONSE the original initiator answers a probe from the
// original responder with MUST carry a set I flag.
//
// RFC 7296 Section 3.1: "I (Initiator) - This bit MUST be set in messages sent by the
// original initiator of the IKE SA and MUST be cleared in messages sent by the original
// responder."
//
// RFC requirement: RFC7296-3.1-13 positive -- the I flag is set on the IKE_SA_INIT, IKE_AUTH, Delete and CREATE_CHILD_SA requests of the original initiator and on its INFORMATIONAL response, and clear on the IKE_SA_INIT, IKE_AUTH and INFORMATIONAL responses of the original responder and on its Delete request.
func TestRFC7296InitiatorBitFollowsTheOriginalRole(t *testing.T) {
	for _, m := range engineBuiltChains(t) {
		hdr := parseMsg(t, m.raw).Header
		fromInitiator := !m.isResponse
		if got := hdr.Flags&wire.FlagInitiator != 0; got != fromInitiator {
			t.Errorf("%s: I flag = %v, want %v", m.name, got, fromInitiator)
		}
	}

	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)

	responderDelete, err := buildEncryptedMessageEx(resp,
		[]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}},
		resp.NextMsgID, wire.ExchangeInformational, initiatorFlag(resp))
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	if parseMsg(t, responderDelete).Header.Flags&wire.FlagInitiator != 0 {
		t.Error("the original responder's Delete request carries the I flag, want it clear")
	}

	probe := &wire.Message{Header: wire.Header{
		InitiatorSPI: ini.InitiatorSPI, ResponderSPI: ini.ResponderSPI,
		MajorVersion: 2, ExchangeType: wire.ExchangeInformational,
		MessageID: ini.ExpectedMsgID,
	}}
	ini.lastResponse = nil
	ini.lastResponseSet = false
	ps.handleInformationalOwned(ini, probe, nil, false, nil, nil, log)
	if ini.lastResponse == nil {
		t.Fatal("the original initiator built no INFORMATIONAL response")
	}
	hdr := parseMsg(t, ini.lastResponse).Header
	if hdr.Flags&wire.FlagResponse == 0 {
		t.Fatal("the message the original initiator built is not a response")
	}
	if hdr.Flags&wire.FlagInitiator == 0 {
		t.Error("the original initiator's INFORMATIONAL response carries a clear I flag, want it set")
	}
}
