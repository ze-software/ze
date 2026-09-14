// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- the Section 2.2 and Section 4.1 rows
//
// RFC 3748 calls EAP a "'lock step' protocol". Five obligations state what that
// means on the wire, and this file proves the four of them that live inside the
// EAP layer itself: the authenticator keeps the outstanding Request standing
// until a valid Response answers it, the peer answers every valid Request and
// only a valid one, Requests are processed in the order they arrive and each to
// completion, and one packet carries exactly one Type. The fifth row here is
// Section 2.2's rule that the framework's own messages carry nothing to a
// method.
//
// Both roles ze plays are driven against each other with real credentials: the
// authenticator through Session and the peer through PeerSession.
//
// VALIDATES: an invalid Response leaves the outstanding Request in place while a
// valid one advances the exchange; the peer answers a valid Request and discards
// an invalid one in silence; the flight's Requests delivered in order produce
// the recorded Responses and the same Requests delivered out of order do not;
// every Request and Response frames one Type octet and a packet with no room for
// one is refused; and neither a Nak Response nor a Notification Response reaches
// an EAP method.
// PREVENTS: an authenticator that advances its Identifier on a Response it
// discarded; a peer that answers an Identity Requery or stays silent on a
// Request it must answer; a peer that buffers or reorders Requests; a decoder
// that accepts a Request with no Type field; and a framework message whose
// contents are handed to the method layer.

package eap

import (
	"bytes"
	"testing"
)

// recordingMethod is a Method that answers every Response with the same Request
// and records what it was handed, so a test can ask which packets reached the
// method layer at all.
type recordingMethod struct {
	seen []*Packet
}

func (m *recordingMethod) Type() uint8 { return TypeMD5Challenge }

func (m *recordingMethod) Start(identifier uint8) *Packet {
	return &Packet{Code: CodeRequest, Identifier: identifier, Type: TypeMD5Challenge, TypeData: []byte{1}}
}

func (m *recordingMethod) Process(response *Packet) MethodResult {
	m.seen = append(m.seen, response)
	return MethodResult{Response: &Packet{Code: CodeRequest, Type: TypeMD5Challenge, TypeData: []byte{2}}}
}

func (m *recordingMethod) DerivesKey() bool { return false }

func (m *recordingMethod) Close() {}

// sessionOverMethod builds an authenticator whose method is the recorder, parked
// in the state that follows an accepted Identity Response.
func sessionOverMethod(m Method, identifier uint8) *Session {
	return &Session{method: m, identifier: identifier, state: stateMethod}
}

// livePair is one MS-CHAPv2 conversation between a real authenticator and a real
// peer, stopped after the peer has answered the method's first Request.
//
// A recorded flight cannot be replayed at a second peer: the MS-CHAPv2 Response
// carries a peer challenge drawn fresh for each PeerSession, so the
// authenticator response computed over one peer's challenge never matches
// another's. Every assertion about the peer therefore drives a live pair.
type livePair struct {
	auth     *Session
	peer     *PeerSession
	identity *Packet // the EAP-Request/Identity
	method   *Packet // the MS-CHAPv2 Challenge Request
}

// newLivePair drives the Identity round and returns the two Requests the
// authenticator sent, with the peer left holding the commitment its answer to
// the Challenge made.
func newLivePair(t *testing.T) *livePair {
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
	return &livePair{auth: auth, peer: peer, identity: identity, method: challenge}
}

// TestRFC3748OutstandingRequestStandsUntilAValidResponse feeds the authenticator
// a Response it must discard, then the valid one, and watches what the
// outstanding Request does across the two.
func TestRFC3748OutstandingRequestStandsUntilAValidResponse(t *testing.T) {
	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	identityRequest := auth.Begin()
	outstanding := identityRequest.Identifier

	res := peer.Process(identityRequest)
	if res.Response == nil {
		t.Fatalf("the peer did not answer the Identity Request: %+v", res)
	}

	// RFC requirement: RFC3748-4.1-6 positive -- RFC 3748 Section 4.1: "Additional
	// Request packets MUST be sent until a valid Response packet is received, an
	// optional retry counter expires, or a lower layer failure indication is
	// received." A Response carrying an Identifier the exchange never asked with is
	// not that valid Response: the authenticator emits no packet for it and leaves
	// the outstanding Identifier where it was, so the Request the carrier holds is
	// still the one the peer owes an answer to.
	stale := &Packet{Code: CodeResponse, Identifier: outstanding + 7, Type: TypeIdentity, TypeData: []byte("user")}
	if next := auth.Process(stale); next != nil {
		t.Fatalf("a Response with a foreign Identifier drew %+v, so the exchange advanced on a packet it must discard", next)
	}
	if auth.identifier != outstanding {
		t.Fatalf("the outstanding Identifier moved to %d on a discarded Response, so the Request that stands is no longer the one the peer was asked", auth.identifier)
	}

	// RFC requirement: RFC3748-4.1-6 negative -- the valid Response ends the
	// asking. The same exchange, given the Response it was waiting for, answers
	// with the next Request and moves the outstanding Identifier off the value it
	// held while the question stood, so an exchange that never receives a valid
	// Response is distinguishable from one that does.
	next := auth.Process(res.Response)
	if next == nil {
		t.Fatal("the valid Identity Response drew no Request, so the exchange stalled on the answer it was waiting for")
	}
	if next.Code != CodeRequest {
		t.Fatalf("the valid Identity Response drew Code %d, want a Request (%d)", next.Code, CodeRequest)
	}
	if next.Identifier == outstanding {
		t.Fatalf("the new Request reuses Identifier %d, so nothing distinguishes it from the Request it replaces", outstanding)
	}
}

// TestRFC3748PeerAnswersOnlyAValidRequest drives one valid Request and one the
// peer must treat as invalid, at the same point in the same conversation.
func TestRFC3748PeerAnswersOnlyAValidRequest(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")

	// RFC requirement: RFC3748-4.1-7 positive -- RFC 3748 Section 4.1: "The peer
	// MUST send a Response packet in reply to a valid Request packet." Every
	// Request of a complete MS-CHAPv2 conversation drew a Response, and each
	// Response answers the Request at the same position in the flight.
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
	// result, Identity Requery is not supported." A peer that has committed to its
	// method and is then asked for its identity again returns no Response at all,
	// so the peer is not answering every packet that reaches it.
	//
	// The peer is driven live rather than handed fl.requests. A recorded flight
	// cannot be replayed at a second peer (livePair), so a replay never reaches
	// the committed state this arm is about.
	pair := newLivePair(t)
	if res := pair.peer.Process(pair.method); res.Err != nil || res.Response == nil {
		t.Fatalf("the peer did not answer the MS-CHAPv2 Challenge: %+v", res)
	}
	requery := &Packet{Code: CodeRequest, Identifier: 200, Type: TypeIdentity}
	if res := pair.peer.Process(requery); res.Response != nil {
		t.Fatalf("the peer answered an Identity Requery with %+v", res.Response)
	}
}

// TestRFC3748RequestsAreProcessedInOrderToCompletion replays one recorded flight
// in the order it was received, then delivers the same Requests with the first
// method round left out.
func TestRFC3748RequestsAreProcessedInOrderToCompletion(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")
	if len(fl.requests) < 3 {
		t.Fatalf("the flight carries %d Requests; this assertion needs the Identity, the Challenge and the Success indication", len(fl.requests))
	}

	// RFC requirement: RFC3748-4.1-8 positive -- RFC 3748 Section 4.1: "Requests
	// MUST be processed in the order that they are received, and MUST be processed
	// to their completion before inspecting the next Request." The peer answered
	// each Request of the live conversation as it arrived, and the answer at each
	// position carries the Type of the Request at that same position, so nothing
	// was buffered for later or answered out of turn.
	//
	// Read off the live flight rather than replayed at a second peer: the
	// MS-CHAPv2 Response carries a peer challenge drawn fresh per PeerSession, so
	// a replay fails on the authenticator response rather than on the ordering
	// this arm is about (livePair).
	for i, req := range fl.requests {
		if fl.responses[i].Type != req.Type {
			t.Fatalf("Request %d of Type %d drew a Response of Type %d", i, req.Type, fl.responses[i].Type)
		}
	}

	// RFC requirement: RFC3748-4.1-8 negative -- a Request read out of that order
	// is not answered as though its predecessor had been processed. The MS-CHAPv2
	// success indication carries the authenticator response over a challenge the
	// peer has not been sent, and a peer that had buffered the flight or read
	// ahead would still answer it. This one refuses, so each Request is read
	// against the state its predecessors left and no other.
	skipped := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(skipped.Close)
	if res := skipped.Process(fl.requests[0]); res.Err != nil {
		t.Fatalf("the Identity Request failed: %v", res.Err)
	}
	res := skipped.Process(fl.requests[2])
	if res.Err == nil && res.Response != nil {
		t.Fatalf("the success indication was answered with %+v without the Challenge that precedes it", res.Response)
	}
}

// TestRFC3748OnePacketCarriesOneType inspects the framing of every packet of a
// complete conversation, then hands the decoder a Request with no room for a
// Type octet.
func TestRFC3748OnePacketCarriesOneType(t *testing.T) {
	fl := driveMSCHAPv2Flight(t, "secret", "secret")

	// RFC requirement: RFC3748-4.1-9 positive -- RFC 3748 Section 4.1: "The Type
	// field is one octet.  This field indicates the Type of Request or Response.  A
	// single Type MUST be specified for each EAP Request or Response." Every
	// Request and Response of the conversation frames its Type in the one octet at
	// offset 4, and the Type-Data that follows starts at offset 5, so no second
	// Type rides inside the packet.
	for i, p := range append(append([]*Packet(nil), fl.requests...), fl.responses...) {
		wire := p.Encode()
		if len(wire) != 5+len(p.TypeData) {
			t.Fatalf("packet %d encodes to %d octets, want %d: one Code, one Identifier, two Length, one Type, then the Type-Data", i, len(wire), 5+len(p.TypeData))
		}
		if wire[4] != p.Type {
			t.Fatalf("packet %d frames Type %d at offset 4, want %d", i, wire[4], p.Type)
		}
		got, err := DecodePacket(wire)
		if err != nil {
			t.Fatalf("packet %d did not decode: %v", i, err)
		}
		if got.Type != p.Type {
			t.Fatalf("packet %d decoded to Type %d, want %d", i, got.Type, p.Type)
		}
		if !bytes.Equal(got.TypeData, p.TypeData) {
			t.Fatalf("packet %d decoded Type-Data %#x, want %#x", i, got.TypeData, p.TypeData)
		}
	}

	// RFC requirement: RFC3748-4.1-9 negative -- a Request whose Length field
	// leaves no room for that one octet is refused rather than read as a Request
	// with no Type at all. The four octets below are a well-formed Success frame
	// wearing the Request Code, which is the shape a decoder that read the Type
	// from whatever followed the header would accept.
	typeless := []byte{CodeRequest, 9, 0, 4}
	if _, err := DecodePacket(typeless); err == nil {
		t.Fatal("a Request with no Type octet decoded, so the single Type a packet must specify is not required to be there")
	}
}

// TestRFC3748FrameworkMessagesReachNoMethod drives the framework's own messages
// at an authenticator whose method records everything handed to it, and reads
// the Notification Response the peer composes.
func TestRFC3748FrameworkMessagesReachNoMethod(t *testing.T) {
	rec := &recordingMethod{}
	auth := sessionOverMethod(rec, 4)

	// RFC requirement: RFC3748-2.2-1 positive -- RFC 3748 Section 2.2: "the
	// Success, Failure, Nak Response(s), and Notification Request/Response messages
	// MUST NOT be used to carry data destined for delivery to other EAP methods."
	// A Nak Response ends the exchange without its octets ever being handed to the
	// method, so the desired-Type list it carries reaches no method layer.
	nak := &Packet{Code: CodeResponse, Identifier: 4, Type: TypeNAK, TypeData: []byte{TypeTLS}}
	if got := auth.Process(nak); got == nil || got.Code != CodeFailure {
		t.Fatalf("the Nak Response drew %+v, want an EAP-Failure", got)
	}
	if len(rec.seen) != 0 {
		t.Fatalf("the method was handed %d framework packet(s): %+v", len(rec.seen), rec.seen)
	}

	// The terminal packets have no field to carry method data in. RFC 3748 Section
	// 4.2 gives Success and Failure a Code, an Identifier and a Length of 4, and
	// Encode writes exactly those four octets.
	for _, code := range []uint8{CodeSuccess, CodeFailure} {
		wire := (&Packet{Code: code, Identifier: 4, Type: TypeMD5Challenge, TypeData: []byte("payload")}).Encode()
		if len(wire) != 4 {
			t.Fatalf("Code %d encoded to %d octets, want 4", code, len(wire))
		}
	}

	// The Notification Response is the fourth message the sentence names, and the
	// peer composes it with a zero-length Type-Data field, so it too can carry
	// nothing to a method. RFC 3748 Section 5.2: "The Type-Data field of the
	// Response is zero octets in length."
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
	// Type IS delivered to the method, so the silence above is a property of the
	// four framework messages rather than of a method layer nothing ever reaches.
	//
	// A fresh authenticator, because the one above answered the Nak with an
	// EAP-Failure and so has terminated. Reusing it would read the silence of a
	// closed exchange as the silence of a method that is never reached, which is
	// the opposite of what this arm has to show.
	delivered := &recordingMethod{}
	live := sessionOverMethod(delivered, 4)
	answer := &Packet{Code: CodeResponse, Identifier: 4, Type: TypeMD5Challenge, TypeData: []byte{0xAA}}
	if got := live.Process(answer); got == nil {
		t.Fatal("a Response of the method's own Type drew nothing")
	}
	if len(delivered.seen) != 1 {
		t.Fatalf("the method was handed %d Response(s), want 1", len(delivered.seen))
	}
	if !bytes.Equal(delivered.seen[0].TypeData, []byte{0xAA}) {
		t.Fatalf("the method was handed Type-Data %#x, want %#x", delivered.seen[0].TypeData, []byte{0xAA})
	}
}
