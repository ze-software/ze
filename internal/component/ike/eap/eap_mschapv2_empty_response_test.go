// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 handler tests
//
// Lives beside eap_mschapv2_test.go rather than inside it: that file carries
// `RFC requirement:` tags, and .claude/hooks/pretool-writeedit.py refuses every edit to a
// tagged test file, an addition included. This file changes no tagged assertion.

package eap

import "testing"

// TestMSCHAPv2ProcessRefusesEmptyTypeData drives an EAP-Response/MSCHAPv2 that carries no
// MS-CHAPv2 octets at all, from the wire bytes a peer can send, through the
// authenticator's own entry point.
//
// The length guard in mschapv2Method.Process is what keeps that packet from indexing
// TypeData[0].
//
// The packet is reachable from the network, and the producer that builds it is in the
// engine package rather than this one. PayloadEAP.ReadFrom
// (internal/component/ike/wire/payload_eap.go) allocates EAPData only when the payload
// length is more than 4, so a length of exactly 5 -- four header octets and the Type --
// yields one octet. wireEAPToPacket (internal/component/ike/engine/fsm.go) then reads
// EAPData[0] into Type and leaves TypeData nil, because it fills TypeData only when
// EAPData is longer than one octet. handleResponderEAP
// (internal/component/ike/engine/responder_eap.go) hands that packet straight to
// Session.Process. The length is peer-controlled and the peer is unauthenticated while it
// sends it.
//
// This test cannot call those producers: engine imports eap, so the fixture is built with
// DecodePacket, which yields the identical shape for the same wire bytes. It allocates
// TypeData only when the EAP Length is more than 5, so a Length of exactly 5 decodes to
// Type 26 and an empty TypeData.
//
// Driven from Session.Process rather than from mschapv2Method.Process, because the guard
// defends an authenticator that is reading the network (`ai/rules/evidence.md`).
//
// Discrimination, measured 2026-08-15: with the `len(response.TypeData) == 0` guard
// deleted from Process, this test panics with "index out of range [0] with length 0" at
// eap_mschapv2.go, and the eap package is RED. With the guard restored it is GREEN.
func TestMSCHAPv2ProcessRefusesEmptyTypeData(t *testing.T) {
	session, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "TestPassword"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(session.Close)

	// The Identity round, so the authenticator has sent its Challenge and waits for the
	// MS-CHAPv2 Response. That is the state the guard defends.
	request := session.Begin()
	challenge := session.Process(&Packet{
		Code:       CodeResponse,
		Identifier: request.Identifier,
		Type:       TypeIdentity,
		TypeData:   []byte("testuser"),
	})
	if challenge == nil || challenge.Type != TypeMSCHAPv2 {
		t.Fatalf("expected an MS-CHAPv2 Challenge after the Identity round, got %+v", challenge)
	}

	// The wire bytes. An EAP Length of 5 leaves no room for even the OpCode.
	wire := []byte{CodeResponse, challenge.Identifier, 0x00, 0x05, TypeMSCHAPv2}
	response, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("DecodePacket refused a well-formed 5-octet EAP response: %v", err)
	}
	if len(response.TypeData) != 0 {
		t.Fatalf("fixture is not the empty-TypeData case: %d octets", len(response.TypeData))
	}

	// No panic, and the exchange ends in an EAP-Failure. The error text is deliberately
	// not asserted: a reword must not redden this (spec R-1).
	out := session.Process(response)
	if out == nil {
		t.Fatal("an empty MS-CHAPv2 response produced no answer at all")
	}
	if out.Code != CodeFailure {
		t.Fatalf("expected EAP-Failure for an empty MS-CHAPv2 response, got code %d", out.Code)
	}
	if session.Succeeded() {
		t.Fatal("an empty MS-CHAPv2 response authenticated the peer")
	}
}
