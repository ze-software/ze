// RFC: rfc/short/rfc2869.md -- RFC2869-5.14-1, for the Access-Reject and Access-Challenge codes
// Related: rfc2869_message_authenticator_test.go -- the Access-Accept half and the test server
// Related: client.go -- dispatchResponse, packet.go -- verifyResponseMessageAuthenticator

// The Section 5.14 client rule names three reply codes. The Access-Accept is
// driven in rfc2869_message_authenticator_test.go; this file drives the other
// two through the same real entry point, Client.Exchange.
package radius

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 2869 Section 5.14 mandates HMAC-MD5
	"sync"
	"testing"
)

// rfc2869SignedReply builds a reply of the given code that carries a
// Reply-Message and a Message-Authenticator. The Message-Authenticator is an
// HMAC-MD5 under maSecret over the packet with the Request Authenticator in the
// authenticator field and the value zeroed (RFC 2869 Section 5.14). The Response
// Authenticator is then computed under respSecret (RFC 2865 Section 3), so a
// test can make the Message-Authenticator the only wrong field.
func rfc2869SignedReply(code uint8, req, maSecret, respSecret []byte) []byte {
	if len(req) < MinPacketLen {
		return nil
	}
	var requestAuth [AuthenticatorLen]byte
	copy(requestAuth[:], req[4:4+AuthenticatorLen])

	pkt := &Packet{Code: code, Identifier: req[1], Authenticator: requestAuth, Attrs: []Attr{
		{Type: AttrReplyMessage, Value: []byte("five-fourteen")},
		{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)},
	}}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		return nil
	}
	wire := buf[:n]

	mac := hmac.New(md5.New, maSecret)
	mac.Write(wire)
	copy(wire[n-AuthenticatorLen:], mac.Sum(nil))

	auth := ResponseAuthenticator(code, req[1], uint16(n), requestAuth, wire[HeaderLen:n], respSecret)
	copy(wire[4:4+AuthenticatorLen], auth[:])
	return append([]byte{}, wire...)
}

// TestRFC2869RejectAndChallengeWithValidMessageAuthenticatorAreDelivered drives
// Client.Exchange against a server whose Access-Reject, then Access-Challenge,
// carries a correctly signed Message-Authenticator.
//
// RFC requirement: RFC2869-5.14-1 positive -- an Access-Reject and an
// Access-Challenge whose Message-Authenticator matches the value the client
// computes are each returned by Client.Exchange with their code and their
// Reply-Message (client.go dispatchResponse, packet.go
// verifyResponseMessageAuthenticator).
func TestRFC2869RejectAndChallengeWithValidMessageAuthenticatorAreDelivered(t *testing.T) {
	secret := []byte("testing123")
	for _, code := range []uint8{CodeAccessReject, CodeAccessChallenge} {
		srv := newRFC2869Server(t, func(req []byte) []byte {
			return rfc2869SignedReply(code, req, secret, secret)
		})

		resp, err := rfc2869Exchange(t, srv.addr, secret)
		if err != nil {
			t.Fatalf("code %d: Exchange: %v", code, err)
		}
		if resp.Code != code {
			t.Fatalf("Code = %d, want %d", resp.Code, code)
		}
		if string(resp.FindAttr(AttrReplyMessage)) != "five-fourteen" {
			t.Fatalf("code %d: the delivered reply lost its Reply-Message", code)
		}
	}
}

// TestRFC2869RejectAndChallengeWithWrongMessageAuthenticatorAreDiscarded
// presents an Access-Reject, then an Access-Challenge, whose Response
// Authenticator is correct and whose Message-Authenticator is signed with
// another secret. Only the Message-Authenticator rule can refuse either reply.
//
// RFC 2869 Section 5.14: "A RADIUS Client receiving an Access-Accept,
// Access-Reject or Access-Challenge with a Message-Authenticator Attribute
// present MUST calculate the correct value of the Message-Authenticator and
// silently discard the packet if it does not match the value sent."
//
// RFC requirement: RFC2869-5.14-1 negative -- an Access-Reject and an
// Access-Challenge whose Message-Authenticator does not match are discarded:
// the reply passes VerifyResponseAuth, and Client.Exchange returns an error and
// no packet (client.go dispatchResponse).
func TestRFC2869RejectAndChallengeWithWrongMessageAuthenticatorAreDiscarded(t *testing.T) {
	secret := []byte("testing123")
	for _, code := range []uint8{CodeAccessReject, CodeAccessChallenge} {
		var mu sync.Mutex
		var sampleReq []byte
		srv := newRFC2869Server(t, func(req []byte) []byte {
			mu.Lock()
			sampleReq = append([]byte{}, req...)
			mu.Unlock()
			return rfc2869SignedReply(code, req, []byte("wrong-secret"), secret)
		})

		resp, err := rfc2869Exchange(t, srv.addr, secret)
		if err == nil {
			t.Fatalf("code %d: Exchange returned code %d; the reply must be silently discarded", code, resp.Code)
		}

		mu.Lock()
		req := sampleReq
		mu.Unlock()
		if req == nil {
			t.Fatalf("code %d: the server never answered", code)
		}
		var requestAuth [AuthenticatorLen]byte
		copy(requestAuth[:], req[4:4+AuthenticatorLen])
		if !VerifyResponseAuth(rfc2869SignedReply(code, req, []byte("wrong-secret"), secret), requestAuth, secret) {
			t.Fatalf("code %d: the Response Authenticator must be correct, so only the Message-Authenticator is wrong", code)
		}
	}
}
