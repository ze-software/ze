// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- the Section 2.2 and Section 4.1 lock-step rows
//
// RFC 3748 calls EAP a "'lock step' protocol". Three obligations state what that
// means for the packets a peer reads: it answers a valid Request and only a
// valid one, it reads each Request against the state its predecessors left, and
// the framework's own messages carry nothing to an EAP method.
//
// Both roles ze plays are driven against each other with real credentials: the
// authenticator through Session and the peer through PeerSession. A recorded
// flight is never replayed at a second peer, because the MS-CHAPv2 Response
// carries a peer challenge drawn fresh for each PeerSession and the
// authenticator response computed over one peer's challenge never matches
// another's.
//
// VALIDATES: the peer answers every Request of a complete conversation and
// discards an Identity Requery in silence; two Requests delivered in the order
// they were sent are both answered while the same two delivered in the reverse
// order are not; and neither a Nak Response nor a Notification Response nor a
// terminal packet reaches an EAP method.
// PREVENTS: a peer that answers an Identity Requery or stays silent on a Request
// it must answer; a peer that buffers or reorders Requests; and a framework
// message whose contents are handed to the method layer.

package eap

import (
	"bytes"
	"testing"
)

// tapMethod is a Method that answers every Response with the same Request and
// records what it was handed, so a test can ask which packets reached the method
// layer at all.
type tapMethod struct {
	seen []*Packet
}

func (m *tapMethod) Type() uint8 { return TypeMD5Challenge }

func (m *tapMethod) Start(identifier uint8) *Packet {
	return &Packet{Code: CodeRequest, Identifier: identifier, Type: TypeMD5Challenge, TypeData: []byte{1}}
}

func (m *tapMethod) Process(response *Packet) MethodResult {
	m.seen = append(m.seen, response)
	return MethodResult{Response: &Packet{Code: CodeRequest, Type: TypeMD5Challenge, TypeData: []byte{2}}}
}

func (m *tapMethod) DerivesKey() bool { return false }

func (m *tapMethod) Close() {}

// sessionOverTap builds an authenticator whose method is the tap, parked in the
// state that follows an accepted Identity Response.
func sessionOverTap(m Method, identifier uint8) *Session {
	return &Session{method: m, identifier: identifier, state: stateMethod}
}

// mschapv2Opening is the first two Requests of one live MS-CHAPv2 conversation,
// with the peer that answered the first of them.
type mschapv2Opening struct {
	auth      *Session
	peer      *PeerSession
	identity  *Packet // the EAP-Request/Identity
	challenge *Packet // the MS-CHAPv2 Challenge Request that follows it
}

// openMSCHAPv2 drives the Identity round of a live pair and returns both
// Requests, so a caller can deliver them to a second peer in either order.
func openMSCHAPv2(t *testing.T) *mschapv2Opening {
	t.Helper()

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	identity := auth.Begin()
	res := peer.Process(identity)
	if res.Err != nil || res.Response == nil {
		t.Fatalf("the peer did not answer the Identity Request: %+v", res)
	}
	challenge := auth.Process(res.Response)
	if challenge == nil || challenge.Type != TypeMSCHAPv2 {
		t.Fatalf("the Identity Response drew %+v, want an MS-CHAPv2 Challenge", challenge)
	}
	return &mschapv2Opening{auth: auth, peer: peer, identity: identity, challenge: challenge}
}

// TestRFC3748PeerAnswersAValidRequestAndDiscardsAnInvalidOne drives one complete
// conversation, then asks a committed peer for its identity again.
func TestRFC3748PeerAnswersAValidRequestAndDiscardsAnInvalidOne(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")

	// RFC requirement: RFC3748-4.1-7 positive -- RFC 3748 Section 4.1: "The peer
	// MUST send a Response packet in reply to a valid Request packet." Every
	// Request of a complete MS-CHAPv2 conversation drew a Response, and each
	// Response carries the Response Code and the Identifier of the Request at its
	// own position in the flight.
	if len(fl.responses) != len(fl.requests) {
		t.Fatalf("the peer answered %d of %d Requests", len(fl.responses), len(fl.requests))
	}
	for i, req := range fl.requests {
		if fl.responses[i].Code != CodeResponse {
			t.Fatalf("the answer to Request %d has Code %d, want a Response (%d)", i, fl.responses[i].Code, CodeResponse)
		}
		if fl.responses[i].Identifier != req.Identifier {
			t.Fatalf("the answer to Request %d carries Identifier %d, want %d", i, fl.responses[i].Identifier, req.Identifier)
		}
	}

	// RFC requirement: RFC3748-4.1-7 negative -- the duty is owed to a VALID
	// Request and to no other. RFC 3748 Section 2.1: "a peer receiving such
	// Requests MUST treat them as invalid, and silently discard them.  As a
	// result, Identity Requery is not supported." A peer that has answered its
	// method's Challenge and is then asked for its identity again returns no
	// Response at all, so the peer above is not answering every packet that
	// reaches it.
	opening := openMSCHAPv2(t)
	if res := opening.peer.Process(opening.challenge); res.Err != nil || res.Response == nil {
		t.Fatalf("the peer did not answer the Challenge: %+v", res)
	}
	requery := &Packet{Code: CodeRequest, Identifier: 200, Type: TypeIdentity}
	if res := opening.peer.Process(requery); res.Response != nil {
		t.Fatalf("the peer answered an Identity Requery with %+v", res.Response)
	}
}

// TestRFC3748PeerReadsEachRequestAgainstItsPredecessors delivers the same two
// Requests to two fresh peers, once in the order they were sent and once
// reversed.
func TestRFC3748PeerReadsEachRequestAgainstItsPredecessors(t *testing.T) {
	opening := openMSCHAPv2(t)

	// RFC requirement: RFC3748-4.1-8 positive -- RFC 3748 Section 4.1: "Requests
	// MUST be processed in the order that they are received, and MUST be processed
	// to their completion before inspecting the next Request." Read in the order
	// the authenticator sent them, both Requests are answered, and each Response
	// carries the Type of the Request at its own position, so the first Request's
	// processing had completed when the second was read.
	ordered := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(ordered.Close)
	first := ordered.Process(opening.identity)
	if first.Err != nil || first.Response == nil || first.Response.Type != TypeIdentity {
		t.Fatalf("the Identity Request read first drew %+v", first)
	}
	second := ordered.Process(opening.challenge)
	if second.Err != nil || second.Response == nil || second.Response.Type != TypeMSCHAPv2 {
		t.Fatalf("the Challenge read second drew %+v", second)
	}

	// RFC requirement: RFC3748-4.1-8 negative -- the same two Requests read in the
	// reverse order are not both answered. The peer that read the Challenge first
	// has committed to its method, so the Identity Request behind it draws no
	// Response, where the peer above answered it. A peer that buffered the pair
	// and sorted them, or that read the second before finishing the first, would
	// answer both either way round.
	reversed := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(reversed.Close)
	if res := reversed.Process(opening.challenge); res.Err != nil || res.Response == nil {
		t.Fatalf("the Challenge read first drew %+v", res)
	}
	late := reversed.Process(opening.identity)
	if late.Response != nil {
		t.Fatalf("the Identity Request read after the Challenge drew %+v", late.Response)
	}
}

// TestRFC3748NoFrameworkMessageReachesAnEAPMethod drives the framework's own
// messages at an authenticator whose method records everything handed to it, and
// reads the Notification Response the peer composes.
func TestRFC3748NoFrameworkMessageReachesAnEAPMethod(t *testing.T) {
	tap := &tapMethod{}
	refusing := sessionOverTap(tap, 4)

	// RFC requirement: RFC3748-2.2-1 positive -- RFC 3748 Section 2.2: "the
	// Success, Failure, Nak Response(s), and Notification Request/Response messages
	// MUST NOT be used to carry data destined for delivery to other EAP methods."
	// The Nak Response ends the exchange without its octets ever being handed to
	// the method; Success and Failure encode to the four octets RFC 3748 Section
	// 4.2 gives them, which leave no field to carry anything in; and the peer's
	// Notification Response carries a Type-Data field of zero octets.
	nak := &Packet{Code: CodeResponse, Identifier: 4, Type: TypeNAK, TypeData: []byte{TypeTLS}}
	if got := refusing.Process(nak); got == nil || got.Code != CodeFailure {
		t.Fatalf("the Nak Response drew %+v, want an EAP-Failure", got)
	}
	if len(tap.seen) != 0 {
		t.Fatalf("the method was handed %d framework packet(s): %+v", len(tap.seen), tap.seen)
	}

	for _, code := range []uint8{CodeSuccess, CodeFailure} {
		wire := (&Packet{Code: code, Identifier: 4, Type: TypeMD5Challenge, TypeData: []byte("payload")}).Encode()
		if len(wire) != 4 {
			t.Fatalf("Code %d encoded to %d octets, want 4", code, len(wire))
		}
	}

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)
	notify := &Packet{Code: CodeRequest, Identifier: 11, Type: TypeNotification, TypeData: []byte("display me")}
	res := peer.Process(notify)
	if res.Response == nil || res.Response.Type != TypeNotification {
		t.Fatalf("the Notification Request drew %+v, want a Notification Response", res.Response)
	}
	if len(res.Response.TypeData) != 0 {
		t.Fatalf("the Notification Response carries %d octets of Type-Data, want none", len(res.Response.TypeData))
	}

	// RFC requirement: RFC3748-2.2-1 negative -- a Response of the method's own
	// Type IS delivered to the method, with its Type-Data intact, so the silence
	// above is a property of the four framework messages rather than of a method
	// layer nothing ever reaches.
	// The tap is held by name rather than asserted back out of the session: a
	// failed assertion answers an empty slice, which reads here as "the method was
	// handed nothing", the very thing this arm exists to refute.
	delivered := &tapMethod{}
	delivering := sessionOverTap(delivered, 4)
	answer := &Packet{Code: CodeResponse, Identifier: 4, Type: TypeMD5Challenge, TypeData: []byte{0xAA}}
	if got := delivering.Process(answer); got == nil {
		t.Fatal("a Response of the method's own Type drew nothing")
	}
	seen := delivered.seen
	if len(seen) != 1 {
		t.Fatalf("the method was handed %d Response(s), want 1", len(seen))
	}
	if !bytes.Equal(seen[0].TypeData, []byte{0xAA}) {
		t.Fatalf("the method was handed Type-Data %#x, want %#x", seen[0].TypeData, []byte{0xAA})
	}
}
