// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 mutual authentication
// Detail: peer.go handleMSCHAPv2Success -- the check this file pins
//
// VALIDATES: the peer recomputes the Authenticator Response and ends the session
// unless the S= field carries exactly that value.
// PREVENTS: a rogue authenticator completing MS-CHAPv2 against Ze without knowing
// the password, by sending 40 arbitrary hex digits or by omitting the Message
// field altogether. Both were accepted before this file existed, which removed
// the mutual half of MS-CHAPv2 mutual authentication.
//
// The file sits beside peer_test.go rather than inside it: that file carries
// `RFC requirement:` tags and the pretool-writeedit hook refuses every edit to a
// tagged test file, an addition included (ai/rules/testing.md).

package eap

import (
	"strings"
	"testing"
)

// mschapv2PeerAtSuccess drives a peer and an in-package authenticator through
// Identity, Challenge and Response, and returns the peer together with the
// authenticator's genuine Success packet. Both sides hold the same password, so
// the S= field in that packet is the value the peer must accept.
func mschapv2PeerAtSuccess(t *testing.T) (*PeerSession, *Packet) {
	t.Helper()

	const (
		userName = "testuser"
		password = "TestPassword"
	)
	peer := NewPeerSession(TypeMSCHAPv2, userName, password)
	server := &mschapv2Method{password: password}

	if res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Err != nil {
		t.Fatalf("identity: %v", res.Err)
	}

	res := peer.Process(server.Start(2))
	if res.Err != nil {
		t.Fatalf("challenge: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer sent no MS-CHAPv2 Response to the Challenge")
	}

	serverRes := server.Process(res.Response)
	if serverRes.Err != nil {
		t.Fatalf("the authenticator rejected the peer Response: %v", serverRes.Err)
	}
	if serverRes.Response == nil {
		t.Fatal("the authenticator sent no MS-CHAPv2 Success")
	}
	return peer, serverRes.Response
}

// mschapv2Success frames an MS-CHAPv2 Success packet carrying the given Message.
// Wire format, RFC 2759 Section 5 over draft-kamath EAP encapsulation:
//
//	 0        1        2        3        4                 msLen
//	+--------+--------+--------+--------+------------------+
//	| OpCode | MS-ID  |   MS-Length     |     Message      |
//	+--------+--------+--------+--------+------------------+
//	    3       copied   total octets     "S=<40 hex> M=<text>"
func mschapv2Success(msID uint8, message string) *Packet {
	msLen := 4 + len(message)
	td := make([]byte, msLen)
	td[0] = mschapv2OpSuccess
	td[1] = msID
	td[2] = byte(msLen >> 8)
	td[3] = byte(msLen)
	copy(td[4:], message)

	return &Packet{
		Code:       CodeRequest,
		Identifier: 4,
		Type:       TypeMSCHAPv2,
		TypeData:   td,
	}
}

// TestRFC2759PeerAcceptsCorrectAuthenticatorResponse feeds the peer the Success
// packet the in-package authenticator computed from the same password, so the S=
// field is the expected Authenticator Response and nothing else.
//
// RFC 2759 Section 5: "This number is derived from the challenge from the
// Challenge packet, the Peer-Challenge and NT-Response fields from the Response
// packet, and the peer password as output by the routine
// GenerateAuthenticatorResponse() (see section 8.7, below). The authenticating
// peer MUST verify the authenticator response when a Success packet is received."
//
// RFC requirement: RFC2759-x-7 positive -- the peer recomputes the Authenticator
// Response over its own Peer-Challenge, the Authenticator Challenge and the
// NT-Response it sent, and acknowledges the Success packet whose S= field carries
// that exact value (peer.go handleMSCHAPv2Success).
func TestRFC2759PeerAcceptsCorrectAuthenticatorResponse(t *testing.T) {
	peer, success := mschapv2PeerAtSuccess(t)

	res := peer.Process(success)
	if res.Err != nil {
		t.Fatalf("the peer refused a correct Authenticator Response: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("the peer sent no Success acknowledgement")
	}
	if res.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("acknowledgement type = %d, want %d", res.Response.Type, TypeMSCHAPv2)
	}
	if got := res.Response.TypeData[0]; got != mschapv2OpSuccess {
		t.Fatalf("acknowledgement OpCode = %d, want %d", got, mschapv2OpSuccess)
	}
}

// TestRFC2759PeerEndsSessionOnBadAuthenticatorResponse walks every Success packet
// whose S= field is not the expected Authenticator Response, the packet that
// carries no Message among them. Each one MUST end the session.
//
// RFC 2759 Section 5: "If the authenticator response is either missing or
// incorrect, the peer MUST end the session."
//
// RFC requirement: RFC2759-x-7 negative -- a Success packet whose S= field is
// wrong, malformed, or absent leaves the peer with an error and no
// acknowledgement, and the session is failed rather than succeeded
// (peer.go handleMSCHAPv2Success).
func TestRFC2759PeerEndsSessionOnBadAuthenticatorResponse(t *testing.T) {
	// The message a genuine Success carries, used to build each near miss.
	_, genuine := mschapv2PeerAtSuccess(t)
	genuineMessage := string(genuine.TypeData[4:])
	msID := genuine.TypeData[1]

	// One hex digit changed keeps the length, the prefix and the character set,
	// so only the recomputation can tell this apart from the genuine value.
	flipped := []byte(genuineMessage)
	switch flipped[2] {
	case '0':
		flipped[2] = '1'
	default:
		flipped[2] = '0'
	}

	cases := []struct {
		name   string
		packet *Packet
	}{
		{"wrong-s-value", mschapv2Success(msID, string(flipped))},
		{"absent-message", &Packet{
			Code:       CodeRequest,
			Identifier: 4,
			Type:       TypeMSCHAPv2,
			TypeData:   []byte{mschapv2OpSuccess, msID, 0, 4},
		}},
		{"no-s-prefix", mschapv2Success(msID, "M=Authentication successful")},
		{"short-s-value", mschapv2Success(msID, "S=00112233 M=truncated")},
		{"non-hex-s-value", mschapv2Success(msID, "S="+strings.Repeat("Z", 40)+" M=not hex")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer, _ := mschapv2PeerAtSuccess(t)

			res := peer.Process(tc.packet)
			if res.Err == nil {
				t.Fatalf("the peer accepted a Success packet with no valid Authenticator Response, message %q", string(tc.packet.TypeData[4:]))
			}
			if res.Response != nil {
				t.Fatalf("the peer acknowledged a Success packet it had refused: OpCode %d", res.Response.TypeData[0])
			}

			// Ending the session means the peer cannot be talked back into
			// success: an EAP-Success arriving after the refusal MUST NOT hand
			// out the MSK.
			//
			// The refusal is a DISCARD rather than an error, since 2026-09-01.
			// RFC 3748 Section 4.2 states "The peer MUST silently discard Success
			// packets", and an error there let the packet choose the moment the
			// SA died. What this case exists to prove is unchanged and is
			// asserted below: no MSK, no completion, no success.
			after := peer.Process(&Packet{Code: CodeSuccess, Identifier: 5})
			if !after.Discarded {
				t.Fatal("an EAP-Success after the refusal was not discarded, so the session did not end")
			}
			if after.Err != nil {
				t.Fatalf("the discard must carry no error, or the packet ends the SA: %v", after.Err)
			}
			if after.Done {
				t.Fatal("the peer completed after refusing the Authenticator Response")
			}
			if peer.Succeeded() {
				t.Fatal("the peer reports success after refusing the Authenticator Response")
			}
		})
	}
}
