// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 handler tests

package eap

import (
	"strings"
	"testing"
)

// validMSCHAPv2Response drives a real peer through the Challenge step and returns the
// authenticator method (holding the matching Authenticator Challenge) together with the
// raw MS-CHAPv2 Response TypeData the peer produced. Feeding this into handleResponse
// exercises the real wire-format validation path.
func validMSCHAPv2Response(t *testing.T) (*mschapv2Method, []byte) {
	t.Helper()
	const password = "TestPassword"
	const identity = "testuser"

	server := &mschapv2Method{password: password}
	challengePkt := server.Start(0x42)

	peer := NewPeerSession(TypeMSCHAPv2, identity, password)
	peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	result := peer.Process(&Packet{
		Code:       challengePkt.Code,
		Identifier: challengePkt.Identifier,
		Type:       challengePkt.Type,
		TypeData:   challengePkt.TypeData,
	})
	if result.Err != nil {
		t.Fatalf("peer failed to build response: %v", result.Err)
	}
	if result.Response == nil {
		t.Fatal("peer produced no response to challenge")
	}
	return server, result.Response.TypeData
}

func TestMSCHAPv2ResponseFieldValidation(t *testing.T) {
	t.Run("valid response accepted", func(t *testing.T) {
		server, td := validMSCHAPv2Response(t)

		// RFC requirement: RFC2759-x-1 positive -- a Response whose 8 Reserved octets are
		// all zero passes validation and the authenticator accepts it.
		// RFC requirement: RFC2759-x-2 positive -- a Response whose Flags octet is zero
		// passes validation and is accepted.
		// RFC requirement: RFC2759-x-3 positive -- a Response whose Value-Size is 49 passes
		// validation and is accepted.
		// RFC requirement: RFC2759-x-11 positive -- a Response with zero Reserved and zero
		// Flags is accepted; the reject path fires only on non-zero fields.
		res := server.handleResponse(td)
		if res.Err != nil {
			t.Fatalf("valid Response rejected: %v", res.Err)
		}
		if res.Response == nil {
			t.Fatal("expected a Success response for a valid Response")
		}
	})

	t.Run("non-zero reserved rejected", func(t *testing.T) {
		server, td := validMSCHAPv2Response(t)
		bad := append([]byte(nil), td...)
		bad[21] = 0x01 // first of the 8 Reserved octets (offsets 21..28)

		// RFC requirement: RFC2759-x-1 negative -- a Response with a non-zero Reserved octet
		// is rejected as malformed (rejected before the NT-Response is even verified).
		// RFC requirement: RFC2759-x-11 negative -- a non-zero Reserved octet in the Response
		// MUST be rejected.
		res := server.handleResponse(bad)
		if res.Err == nil {
			t.Fatal("non-zero Reserved octet must be rejected")
		}
	})

	t.Run("non-zero flags rejected", func(t *testing.T) {
		server, td := validMSCHAPv2Response(t)
		bad := append([]byte(nil), td...)
		bad[53] = 0x01 // Flags octet

		// RFC requirement: RFC2759-x-2 negative -- a Response with a non-zero Flags octet is
		// rejected as malformed.
		// RFC requirement: RFC2759-x-11 negative -- a non-zero Flags octet in the Response
		// MUST be rejected.
		res := server.handleResponse(bad)
		if res.Err == nil {
			t.Fatal("non-zero Flags octet must be rejected")
		}
	})

	t.Run("wrong value-size rejected", func(t *testing.T) {
		server, td := validMSCHAPv2Response(t)
		bad := append([]byte(nil), td...)
		bad[4] = 50 // Value-Size MUST be 49

		// RFC requirement: RFC2759-x-3 negative -- a Response whose Value-Size is not 49 is
		// rejected as malformed.
		res := server.handleResponse(bad)
		if res.Err == nil {
			t.Fatal("Value-Size other than 49 must be rejected")
		}
	})
}

func TestMSCHAPv2SuccessUppercaseHex(t *testing.T) {
	m := &mschapv2Method{msID: 0x42}

	// An Authenticator Response whose bytes force hex letters in the encoding, so the
	// uppercase assertion is not vacuous.
	var authResp [20]byte
	for i := range authResp {
		authResp[i] = 0xab
	}

	// RFC requirement: RFC2759-x-5 positive -- sendSuccess emits the 40-hex-digit S= field
	// in uppercase (A-F), so a compliant peer's uppercase comparison matches.
	res := m.sendSuccess(authResp)
	if res.Response == nil {
		t.Fatal("sendSuccess produced no packet")
	}
	td := res.Response.TypeData
	if len(td) < 4+42 {
		t.Fatalf("Success TypeData too short: %d", len(td))
	}
	msg := string(td[4:])
	if !strings.HasPrefix(msg, "S=") {
		t.Fatalf("expected S= prefix, got %q", msg)
	}
	sHex := msg[2:42]
	if !strings.ContainsAny(sHex, "ABCDEF") {
		t.Fatalf("test vector produced no hex letters; uppercase assertion would be vacuous: %q", sHex)
	}
	if strings.ContainsAny(sHex, "abcdef") {
		t.Fatalf("S= hex contains lowercase letters: %q", sHex)
	}
	if sHex != strings.ToUpper(sHex) {
		t.Fatalf("S= hex not uppercase: %q", sHex)
	}
}

func TestMSCHAPv2AuthChallengeRandom16(t *testing.T) {
	m1 := &mschapv2Method{}
	pkt1 := m1.Start(1)
	m2 := &mschapv2Method{}
	m2.Start(1)

	// RFC requirement: RFC2759-x-8 positive -- Start fills the 16-octet Authenticator
	// Challenge from crypto/rand: the field is exactly 16 octets, the Challenge packet
	// advertises Value-Size 16, the value is not all-zero, and it differs between sessions.
	if len(m1.authChallenge) != 16 {
		t.Fatalf("authChallenge length: got %d, want 16", len(m1.authChallenge))
	}
	if pkt1 == nil || len(pkt1.TypeData) < 5 || pkt1.TypeData[4] != 16 {
		t.Fatal("Challenge packet must advertise Value-Size 16")
	}
	var zero [16]byte
	if m1.authChallenge == zero {
		t.Fatal("authChallenge is all-zero; not randomized")
	}
	if m1.authChallenge == m2.authChallenge {
		t.Fatal("two sessions produced identical Authenticator Challenge")
	}
}
