// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- the Section 2 retransmission row
//
// RFC 3748 Section 2 describes the round that repeats: the authenticator asks,
// the peer answers, and a Request with no valid answer to it stands until one
// arrives. One obligation states what the authenticator may NOT do while it
// stands, and this file proves it on ze's authenticator: no terminal packet is
// emitted for a round the exchange could not read.
//
// VALIDATES: the three shapes of packet that leave an outstanding Request
// unanswered draw no packet at all and leave the outstanding Identifier where it
// was, while the valid Response at the same point draws the EAP-Success.
// PREVENTS: an authenticator that answers an unreadable packet with a terminal
// one, which would let anybody on the path end an EAP conversation, or grant it,
// with a single unauthenticated packet.

package eap

import "testing"

// TestRFC3748NoTerminalPacketWhileTheRequestStands feeds one authenticator, at
// one point of one conversation, the three packets that answer its outstanding
// Request with nothing, then the Response that answers it.
func TestRFC3748NoTerminalPacketWhileTheRequestStands(t *testing.T) {
	ex := openMSCHAPv2Exchange(t, "secret", "secret")

	ack := ex.peer.Process(ex.indication)
	if ack.Err != nil || ack.Response == nil {
		t.Fatalf("the peer did not acknowledge the success indication: %+v", ack)
	}
	outstanding := ex.auth.identifier

	// RFC requirement: RFC3748-2-3 positive -- RFC 3748 Section 2: "The
	// authenticator is responsible for retransmitting requests as described in
	// Section 4.1.  After a suitable number of retransmissions, the authenticator
	// SHOULD end the EAP conversation.  The authenticator MUST NOT send a Success
	// or Failure packet when retransmitting or when it fails to get a response
	// from the peer." Each packet below leaves the exchange without the valid
	// Response Section 4.1 makes it wait for, which is the state it retransmits
	// from: a foreign Identifier, a Code EAP does not define, and a Type this
	// conversation never asked for. Each draws no packet at all, so neither a
	// Success nor a Failure leaves, and the outstanding Identifier does not move,
	// so the Request that stands is still the one the peer owes an answer to.
	unanswered := []struct {
		name   string
		packet *Packet
	}{
		{
			name: "a Response carrying an Identifier the exchange never asked with",
			packet: &Packet{
				Code:       CodeResponse,
				Identifier: ack.Response.Identifier + 7,
				Type:       TypeMSCHAPv2,
				TypeData:   ack.Response.TypeData,
			},
		},
		{
			name: "a packet carrying a Code EAP does not define",
			packet: &Packet{
				Code:       0x7F,
				Identifier: ack.Response.Identifier,
				Type:       TypeMSCHAPv2,
				TypeData:   ack.Response.TypeData,
			},
		},
		{
			name: "a Response carrying a Type this conversation never asked for",
			packet: &Packet{
				Code:       CodeResponse,
				Identifier: ack.Response.Identifier,
				Type:       TypeIdentity,
				TypeData:   []byte("user"),
			},
		},
	}
	for _, c := range unanswered {
		got := ex.auth.Process(c.packet)
		if got != nil {
			t.Fatalf("%s drew %+v, want no packet at all", c.name, got)
		}
		if ex.auth.identifier != outstanding {
			t.Fatalf("%s moved the outstanding Identifier to %d, want %d", c.name, ex.auth.identifier, outstanding)
		}
	}

	// RFC requirement: RFC3748-2-3 negative -- the silence above is the missing
	// Response and nothing wider. The same authenticator, at the same point of the
	// same conversation, given the Response it was waiting for, answers with the
	// EAP-Success its method is entitled to finish on, so an exchange that never
	// receives a valid Response is distinguishable from one that does.
	terminal := ex.auth.Process(ack.Response)
	if terminal == nil {
		t.Fatal("the valid acknowledgement drew no packet, so the exchange stalled on the answer it was waiting for")
	}
	if terminal.Code != CodeSuccess {
		t.Fatalf("the valid acknowledgement drew Code %d, want an EAP-Success (%d)", terminal.Code, CodeSuccess)
	}
}
