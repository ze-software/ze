// RFC: rfc/short/rfc3579.md -- RFC3579-3.1-6, an EAP answer without a Message-Authenticator
// Related: client.go -- dispatchResponse, packet.go -- verifyResponseMessageAuthenticator
// Related: rfc2869_message_authenticator_test.go -- the test server

package radius

import (
	"context"
	"testing"
	"time"
)

// unsignedReply builds a reply of the given code with a correct Response
// Authenticator and no Message-Authenticator. With carryEAP it also carries an
// EAP-Message holding an EAP-Success, so the reply differs from the control
// only in the attribute RFC 3579 Section 3.1 keys on.
func unsignedReply(code uint8, req, secret []byte, carryEAP bool) []byte {
	if len(req) < MinPacketLen {
		return nil
	}
	var requestAuth [AuthenticatorLen]byte
	copy(requestAuth[:], req[4:4+AuthenticatorLen])

	attrs := []Attr{{Type: AttrReplyMessage, Value: AttrString("three-one")}}
	if carryEAP {
		attrs = append(attrs, Attr{Type: AttrEAPMessage, Value: []byte{3, req[1], 0, 4}})
	}
	pkt := &Packet{Code: code, Identifier: req[1], Authenticator: requestAuth, Attrs: attrs}
	buf := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		return nil
	}
	auth := ResponseAuthenticator(code, req[1], uint16(n), requestAuth, buf[HeaderLen:n], secret)
	copy(buf[4:4+AuthenticatorLen], auth[:])
	return append([]byte{}, buf[:n]...)
}

// unsignedExchange sends one Access-Request through a fresh client and returns
// the outcome.
func unsignedExchange(t *testing.T, serverAddr string, secret []byte) (*Packet, error) {
	t.Helper()
	client, err := NewClient(ClientConfig{Timeout: 300 * time.Millisecond, Retries: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { closeSilent(client) })

	auth, err := RandomAuthenticator()
	if err != nil {
		t.Fatalf("RandomAuthenticator: %v", err)
	}
	req := &Packet{
		Code:          CodeAccessRequest,
		Identifier:    client.NextID(),
		Authenticator: auth,
		Attrs:         []Attr{{Type: AttrUserName, Value: AttrString("dave")}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.Exchange(ctx, req, secret, serverAddr)
}

// TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator drives Client.Exchange
// against a server whose Access-Accept, Access-Reject and Access-Challenge each
// carry an EAP-Message and a correct Response Authenticator, but no
// Message-Authenticator. Each must be discarded and counted. The control
// reply, identical except for the EAP-Message, must still be delivered, so the
// discard keys on the attribute RFC 3579 Section 3.1 names.
//
// RFC 3579 Section 3.1: "Access-Challenge, Access-Accept, or Access-Reject
// packets including EAP-Message attribute(s) without a Message-Authenticator
// attribute SHOULD be silently discarded by the NAS."
//
// RFC requirement: RFC3579-3.1-6 negative -- an Access-Accept, Access-Reject or
// Access-Challenge carrying an EAP-Message and no Message-Authenticator is not
// returned by Client.Exchange, and ze_radius_unsigned_eap_discards_total counts it
// (client.go dispatchResponse, packet.go verifyResponseMessageAuthenticator).
func TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	reg := bindTestRadiusMetrics(t)
	for _, code := range []uint8{CodeAccessAccept, CodeAccessReject, CodeAccessChallenge} {
		before := scrapeUnsignedEAPDiscards(t, reg)
		srv := newRFC2869Server(t, func(req []byte) []byte {
			return unsignedReply(code, req, secret, true)
		})
		resp, err := unsignedExchange(t, srv.addr, secret)
		if err == nil {
			t.Fatalf("code %d: Exchange returned code %d; an unsigned EAP answer must be discarded", code, resp.Code)
		}
		counted := scrapeUnsignedEAPDiscards(t, reg)
		if counted <= before {
			t.Fatalf("code %d: the discard was not counted (series %v, was %v)", code, counted, before)
		}

		control := newRFC2869Server(t, func(req []byte) []byte {
			return unsignedReply(code, req, secret, false)
		})
		resp, err = unsignedExchange(t, control.addr, secret)
		if err != nil {
			t.Fatalf("code %d: control reply with no EAP-Message was refused: %v", code, err)
		}
		if resp.Code != code {
			t.Fatalf("code %d: control delivered code %d", code, resp.Code)
		}
		if scrapeUnsignedEAPDiscards(t, reg) != counted {
			t.Fatalf("code %d: a reply with no EAP-Message was counted as an unsigned EAP answer", code)
		}
	}
}
