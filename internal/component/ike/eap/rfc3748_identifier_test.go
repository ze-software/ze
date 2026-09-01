package eap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 3748 Section 4.2, the Success and Failure packet format:
//
//	"The Identifier field MUST match the Identifier field of the Response
//	 packet that it is sent in response to."
//
// Both terminal packets used to increment s.identifier and THEN stamp it, so
// each carried the Response Identifier plus one. Ze talking to Ze agreed with
// itself, because PeerSession.Process switches on request.Code alone and never
// compares the Identifier, so a conforming peer discarded both packets while the
// suite stayed green.

// TestFailureIdentifierMatchesResponse drives Session.Process to a Failure and
// asserts the Identifier is the one the Response carried.
//
// RFC requirement: RFC3748-4.2-4 positive -- a Failure answering a Response
// carries that Response's Identifier, unchanged.
//
// VALIDATES: the terminal packet is addressed to the exchange it ends.
// PREVENTS: the off-by-one that made every EAP-Failure discardable by a peer
// enforcing Section 4.2.
func TestFailureIdentifierMatchesResponse(t *testing.T) {
	const responseID = 0x42

	// A non-Response code is refused by Process before any state is consulted,
	// which is the shortest path to failure() that does not need a live method.
	s, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "p"})
	require.NoError(t, err)

	out := s.Process(&Packet{Code: CodeRequest, Identifier: responseID})

	require.NotNil(t, out, "a refused code must still produce a Failure")
	assert.Equal(t, CodeFailure, out.Code)
	assert.Equal(t, uint8(responseID), out.Identifier,
		"RFC 3748 Section 4.2: Failure MUST carry the Identifier of the packet it answers")
}

// TestFailureIdentifierMatchesResponseOnNAK covers the second producer: a NAK
// inside the method state reaches failure() by a different branch.
//
// RFC requirement: RFC3748-4.2-4 positive -- the NAK branch obeys the same rule.
//
// VALIDATES: every failure() caller stamps the answered Identifier, not just the
// one the first test happens to reach.
// PREVENTS: a fix applied at one call site while the others keep incrementing.
func TestFailureIdentifierMatchesResponseOnNAK(t *testing.T) {
	const responseID = 0x7E

	s, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "p"})
	require.NoError(t, err)

	// Identity first, so the session leaves stateIdentity and a NAK lands in
	// handleMethod rather than handleIdentity.
	s.identifier = 1
	req := s.Process(&Packet{Code: CodeResponse, Identifier: 1, Type: TypeIdentity, TypeData: []byte("u")})
	require.NotNil(t, req, "the identity response must draw the method's first request")

	// The Response must answer the OUTSTANDING Request: RFC 3748 Section 4.1
	// makes the authenticator discard one whose Identifier does not match, so a
	// fixture that mismatches drives the discard rather than the arm under test.
	// Setting the outstanding value to the distinctive one keeps what this case
	// discriminates: a terminal packet built from the session counter would carry
	// the counter's own value, not this one.
	s.identifier = responseID
	out := s.Process(&Packet{Code: CodeResponse, Identifier: responseID, Type: TypeNAK})

	require.NotNil(t, out)
	assert.Equal(t, CodeFailure, out.Code)
	assert.Equal(t, uint8(responseID), out.Identifier,
		"RFC 3748 Section 4.2: a NAK-driven Failure carries the NAK Response's Identifier")
}

// TestIdentityFailureIdentifierMatchesResponse covers handleIdentity's two
// failure() calls, the third producer.
//
// RFC requirement: RFC3748-4.2-4 positive -- a Failure raised while still in the
// identity exchange carries the answered Identifier.
//
// VALIDATES: the identity state's refusal path is not exempt.
// PREVENTS: a peer that sends a wrong-Type identity response receiving an
// unaddressable Failure.
func TestIdentityFailureIdentifierMatchesResponse(t *testing.T) {
	const responseID = 0x11

	s, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "p"})
	require.NoError(t, err)

	// The Response must answer the OUTSTANDING Request: RFC 3748 Section 4.1
	// makes the authenticator discard one whose Identifier does not match, so a
	// fixture that mismatches drives the discard rather than the arm under test.
	// Setting the outstanding value to the distinctive one keeps what this case
	// discriminates: a terminal packet built from the session counter would carry
	// the counter's own value, not this one.
	s.identifier = responseID
	// A Response in stateIdentity whose Type is neither Identity nor NAK.
	out := s.Process(&Packet{Code: CodeResponse, Identifier: responseID, Type: TypeMSCHAPv2})

	require.NotNil(t, out)
	assert.Equal(t, CodeFailure, out.Code)
	assert.Equal(t, uint8(responseID), out.Identifier,
		"RFC 3748 Section 4.2: an identity-stage Failure carries the answered Identifier")
}

// TestRequestIdentifierStillIncrements pins the boundary of the fix: Section 4.2
// governs Success and Failure ONLY. A Request opens a new exchange and must keep
// advancing the Identifier.
//
// VALIDATES: the fix is scoped to terminal packets.
// PREVENTS: freezing the Identifier for every packet, which would make each
// Request indistinguishable from the last and break retransmission matching. A
// test asserting only that Failure matches would pass with that break in place.
func TestRequestIdentifierStillIncrements(t *testing.T) {
	s, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "p"})
	require.NoError(t, err)

	s.identifier = 5
	first := s.Process(&Packet{Code: CodeResponse, Identifier: 5, Type: TypeIdentity, TypeData: []byte("u")})
	require.NotNil(t, first)
	assert.Equal(t, CodeRequest, first.Code, "the identity response draws a method Request")
	assert.NotEqual(t, uint8(5), first.Identifier,
		"a Request opens a new exchange and must not reuse the answered Identifier")
}

// doneMethod is a Method that completes on its first Response, so a test can
// reach the CodeSuccess arm of handleMethod without driving a real credential
// exchange to completion.
type doneMethod struct{ closed bool }

func (m *doneMethod) Type() uint8 { return TypeMSCHAPv2 }

func (m *doneMethod) Start(identifier uint8) *Packet {
	return &Packet{Code: CodeRequest, Identifier: identifier, Type: TypeMSCHAPv2}
}

func (m *doneMethod) Process(_ *Packet) MethodResult {
	return MethodResult{Done: true}
}

func (m *doneMethod) Close() { m.closed = true }

// TestSuccessIdentifierMatchesResponse covers the OTHER terminal packet.
//
// RFC requirement: RFC3748-4.2-4 positive -- a Success answering a Response
// carries that Response's Identifier, unchanged.
//
// VALIDATES: the CodeSuccess arm of handleMethod obeys Section 4.2, not only
// failure().
// PREVENTS: exactly what the first round of these tests missed. Reverting the
// Success arm to `s.identifier++` while leaving failure() correct passed the
// whole package, because every test here drove a Failure. One mutant per CLAIM,
// and "both terminal packets" is two claims.
func TestSuccessIdentifierMatchesResponse(t *testing.T) {
	const responseID = 0x5A

	s := &Session{method: &doneMethod{}, state: stateMethod, identifier: responseID}

	// The Response must answer the OUTSTANDING Request: RFC 3748 Section 4.1
	// makes the authenticator discard one whose Identifier does not match, so a
	// fixture that mismatches drives the discard rather than the arm under test.
	// Setting the outstanding value to the distinctive one keeps what this case
	// discriminates: a terminal packet built from the session counter would carry
	// the counter's own value, not this one.
	out := s.Process(&Packet{Code: CodeResponse, Identifier: responseID, Type: TypeMSCHAPv2})

	require.NotNil(t, out)
	assert.Equal(t, CodeSuccess, out.Code, "a Done method must end the exchange with Success")
	assert.Equal(t, uint8(responseID), out.Identifier,
		"RFC 3748 Section 4.2: Success MUST carry the Identifier of the Response it answers")
	assert.True(t, s.Succeeded(), "the session must record success")
}

// RFC requirement: RFC3748-4.1-10 positive -- the authenticator silently discards
// a Response whose Identifier does not match the outstanding Request, answering
// with no packet at all rather than acting on it.
//
// RFC 3748 Section 4.1: "An authenticator receiving a Response whose Identifier
// value does not match that of the currently outstanding Request MUST silently
// discard the Response."
//
// The method must not run either. doneMethod completes on its first Response, so
// a discard that still reached handleMethod would return an EAP-Success and the
// nil assertion alone would not catch a discard applied after the method ran.
func TestAuthenticatorDiscardsAResponseAnsweringNoOutstandingRequest(t *testing.T) {
	method := &doneMethod{}
	s := &Session{method: method, state: stateMethod, identifier: 7}

	out := s.Process(&Packet{Code: CodeResponse, Identifier: 8, Type: TypeMSCHAPv2})

	assert.Nil(t, out, "a Response answering no outstanding Request must draw no packet")
	assert.False(t, s.Succeeded(), "the discarded Response must not complete the exchange")
}

// RFC requirement: RFC3748-4.1-10 negative -- a Response carrying the outstanding
// Identifier IS processed, so the discard above is the Identifier check acting
// and not the authenticator refusing every Response.
func TestAuthenticatorProcessesAResponseAnsweringTheOutstandingRequest(t *testing.T) {
	s := &Session{method: &doneMethod{}, state: stateMethod, identifier: 7}

	out := s.Process(&Packet{Code: CodeResponse, Identifier: 7, Type: TypeMSCHAPv2})

	require.NotNil(t, out, "the matching Response must be processed")
	assert.Equal(t, CodeSuccess, out.Code, "doneMethod completes, so the exchange ends in Success")
}
