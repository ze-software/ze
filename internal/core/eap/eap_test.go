// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework tests

package eap

import (
	"testing"
)

func TestEAPDispatch(t *testing.T) {
	_, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "test"})
	if err != nil {
		t.Fatalf("create MSCHAPv2 session: %v", err)
	}

	_, err = NewSession(99, MethodConfig{})
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestEAPIdentityExchange(t *testing.T) {
	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "testpass"})
	if err != nil {
		t.Fatal(err)
	}

	// Begin sends Identity request.
	req := sess.Begin()
	if req.Code != CodeRequest || req.Type != TypeIdentity {
		t.Fatalf("Begin: code=%d type=%d", req.Code, req.Type)
	}

	// Respond with Identity.
	identResp := &Packet{
		Code:       CodeResponse,
		Identifier: req.Identifier,
		Type:       TypeIdentity,
		TypeData:   []byte("testuser"),
	}
	next := sess.Process(identResp)
	if next == nil {
		t.Fatal("expected method request after identity")
	}
	if sess.Identity() != "testuser" {
		t.Fatalf("identity: got %q, want %q", sess.Identity(), "testuser")
	}
	if next.Type != TypeMSCHAPv2 {
		t.Fatalf("method type: got %d, want %d", next.Type, TypeMSCHAPv2)
	}
}

func TestEAPPacketEncodeDecode(t *testing.T) {
	pkt := &Packet{
		Code:       CodeRequest,
		Identifier: 42,
		Type:       TypeIdentity,
	}
	encoded := pkt.Encode()
	decoded, err := DecodePacket(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Code != CodeRequest || decoded.Identifier != 42 || decoded.Type != TypeIdentity {
		t.Fatalf("round-trip: code=%d id=%d type=%d", decoded.Code, decoded.Identifier, decoded.Type)
	}
}

func TestEAPSuccessEncodeDecode(t *testing.T) {
	pkt := &Packet{
		Code:       CodeSuccess,
		Identifier: 5,
	}
	encoded := pkt.Encode()
	if len(encoded) != 4 {
		t.Fatalf("success length: got %d, want 4", len(encoded))
	}
	decoded, err := DecodePacket(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Code != CodeSuccess || decoded.Identifier != 5 {
		t.Fatalf("round-trip: code=%d id=%d", decoded.Code, decoded.Identifier)
	}
}

func TestEAPNAKRejection(t *testing.T) {
	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "testpass"})
	if err != nil {
		t.Fatal(err)
	}

	req := sess.Begin()

	// Send NAK instead of identity.
	nak := &Packet{
		Code:       CodeResponse,
		Identifier: req.Identifier,
		Type:       TypeNAK,
		TypeData:   []byte{TypeTLS},
	}
	result := sess.Process(nak)
	if result == nil || result.Code != CodeFailure {
		t.Fatal("expected failure after NAK")
	}
	if sess.Succeeded() {
		t.Fatal("session should not have succeeded")
	}
}
