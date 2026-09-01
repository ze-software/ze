// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP MD5-Challenge, both roles
// RFC: rfc/short/rfc3748.md -- Section 5.4: MD5-Challenge, and Section 5: Types 1-4
// RFC: rfc/short/rfc1994.md -- PPP CHAP: the challenge, the response and their fields
//
// MD5-Challenge is the fourth of the four Types RFC 3748 Section 5 makes every
// EAP implementation support, and ze implemented none of it until 2026-09-01.
// Section 5.4 states the obligation twice: once for the mechanism ("EAP peer and
// EAP server implementations MUST support the MD5-Challenge mechanism") and once
// for the packet ("A Response MUST be sent in reply to the Request").
//
// The tests drive both roles. The authenticator is Session with a
// md5ChallengeMethod under it; the peer is PeerSession, which runs the method
// inline. Both halves compute the same value, so a shared defect in one function
// would agree with itself: the positive test recomputes the expected Response
// Value from crypto/md5 directly, in the field order RFC 1994 Section 4.1 gives,
// rather than from the code under test.
//
// VALIDATES: an MD5-Challenge Request draws a Type-4 Response carrying Value-Size
// 16 and the MD5 of the Identifier, the secret and the challenge; the exchange
// reaches EAP-Success on both sides; one peer answers Types 1, 2, 3 and 4 each in
// its own way; a wrong secret is refused with no MSK and no Success; a malformed
// Type-Data draws an error from either role; and MD5-Challenge reports that it
// derives no key while EAP-TLS and EAP-MSCHAPv2 report that they do.
// PREVENTS: a Response computed over the wrong field order, which interoperates
// with nothing; a Value-Size read as an index into a packet that is shorter than
// it claims; a peer that answers any Request with an MD5-Challenge Response; and
// an all-zero MSK reaching the IKEv2 AUTH payload as if it were a key, which
// RFC 7296 Section 2.16 answers with SK_pi and SK_pr instead.

package eap

import (
	"bytes"
	"crypto/md5" //nolint:gosec // The test recomputes the RFC 1994 Section 4.1 response independently of the code under test, and RFC 3748 Section 5.4 prescribes MD5.
	"testing"
)

// md5TestSecret is the shared secret both roles are configured with, except
// where a test deliberately configures two different ones.
const md5TestSecret = "correct horse battery staple"

// md5TestIdentity is the peer identity, and it is also the Name the peer writes
// into its Response, which RFC 1994 Section 4.1 defines as "one or more octets
// representing the identification of the system transmitting the packet".
const md5TestIdentity = "road-warrior@example.net"

// md5ExpectedResponse recomputes the CHAP Response Value from the RFC's own
// words, without calling the code under test.
//
// RFC 1994 Section 4.1: "The Response Value is the one-way hash calculated over
// a stream of octets consisting of the Identifier, followed by (concatenated
// with) the "secret", followed by (concatenated with) the Challenge Value."
//
// A test that called md5ChallengeResponse would agree with any field order that
// function happened to use, including a wrong one, because the peer and the
// authenticator both call it.
func md5ExpectedResponse(identifier uint8, secret string, challenge []byte) []byte {
	stream := []byte{identifier}
	stream = append(stream, secret...)
	stream = append(stream, challenge...)
	sum := md5.Sum(stream) //nolint:gosec // See the import comment: this is the RFC's algorithm, recomputed independently.
	return sum[:]
}

// md5Authenticator returns an MD5-Challenge authenticator that has issued its
// Identity Request, been answered, and issued its MD5-Challenge Request, which is
// returned beside it.
func md5Authenticator(t *testing.T) (*Session, *Packet) {
	t.Helper()

	auth, err := NewSession(TypeMD5Challenge, MethodConfig{Password: md5TestSecret})
	if err != nil {
		t.Fatalf("NewSession(TypeMD5Challenge) = %v, want a session: RFC 3748 Section 5.4 makes the server support the mechanism", err)
	}
	t.Cleanup(auth.Close)

	identityRequest := auth.Begin()
	if identityRequest.Type != TypeIdentity {
		t.Fatalf("Begin sent Type %d, want %d (Identity)", identityRequest.Type, TypeIdentity)
	}

	challenge := auth.Process(&Packet{
		Code:       CodeResponse,
		Identifier: identityRequest.Identifier,
		Type:       TypeIdentity,
		TypeData:   []byte(md5TestIdentity),
	})
	if challenge == nil {
		t.Fatal("the Identity Response drew no packet, want the MD5-Challenge Request")
	}
	if challenge.Code != CodeRequest || challenge.Type != TypeMD5Challenge {
		t.Fatalf("the Identity Response drew Code %d Type %d, want Code %d Type %d",
			challenge.Code, challenge.Type, CodeRequest, TypeMD5Challenge)
	}
	return auth, challenge
}

// md5Peer returns an MD5-Challenge peer that has answered an Identity Request,
// which is the state an authenticator's first method Request arrives in.
func md5Peer(t *testing.T, secret string) *PeerSession {
	t.Helper()

	peer := NewPeerSession(TypeMD5Challenge, md5TestIdentity, secret)
	t.Cleanup(peer.Close)

	res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	if res.Response == nil || res.Response.Type != TypeIdentity {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", res)
	}
	return peer
}

// TestRFC3748MD5ChallengeRequestDrawsAResponse drives the whole exchange, from
// the Identity Request to the EAP-Success, and checks the Response the
// MD5-Challenge Request draws field by field.
//
// RFC requirement: RFC3748-5.4-1 positive -- an MD5-Challenge Request draws an
// MD5-Challenge Response from the peer, carrying the Request's Identifier,
// Value-Size 16, the MD5 of the Identifier, the secret and the challenge, and
// then the peer identity as the Name; the authenticator accepts that Response
// with an EAP-Success.
//
// VALIDATES: the Response is composed from the Request's own challenge and
// Identifier, in the field order RFC 1994 Section 4.1 gives, and the
// authenticator's verification agrees with a value computed outside this package.
// PREVENTS: a response hashed in a different field order, which no other CHAP
// implementation would accept, and a Response the authenticator's own verifier
// accepts only because both halves share the same defect.
func TestRFC3748MD5ChallengeRequestDrawsAResponse(t *testing.T) {
	auth, challenge := md5Authenticator(t)
	peer := md5Peer(t, md5TestSecret)

	// The Request body is the CHAP Challenge of RFC 1994 Section 4.1: Value-Size,
	// then the challenge Value, then the Name to the end of the packet.
	if len(challenge.TypeData) < 1 {
		t.Fatal("the MD5-Challenge Request carries no Type-Data")
	}
	challengeSize := int(challenge.TypeData[0])
	if challengeSize != md5ChallengeValueSize {
		t.Fatalf("the Request declares Value-Size %d, want %d", challengeSize, md5ChallengeValueSize)
	}
	if len(challenge.TypeData) < 1+challengeSize {
		t.Fatalf("the Request declares Value-Size %d but carries %d octets after it",
			challengeSize, len(challenge.TypeData)-1)
	}
	challengeValue := challenge.TypeData[1 : 1+challengeSize]
	if bytes.Equal(challengeValue, make([]byte, challengeSize)) {
		t.Fatal("the challenge is all zero, so it is not the fresh random value RFC 1994 Section 2.3 asks for")
	}
	if name := string(challenge.TypeData[1+challengeSize:]); name != md5ChallengeName {
		t.Fatalf("the Request Name is %q, want %q", name, md5ChallengeName)
	}

	res := peer.Process(challenge)
	if res.Err != nil {
		t.Fatalf("the MD5-Challenge Request drew the error %v; RFC 3748 Section 5.4 answers it with a Response", res.Err)
	}
	if res.Discarded {
		t.Fatal("the peer discarded the MD5-Challenge Request; RFC 3748 Section 5.4 answers it with a Response")
	}
	if res.Response == nil {
		t.Fatal("the peer sent no Response at all")
	}
	if res.Response.Code != CodeResponse {
		t.Fatalf("Response Code = %d, want %d", res.Response.Code, CodeResponse)
	}
	if res.Response.Type != TypeMD5Challenge {
		t.Fatalf("Response Type = %d, want %d", res.Response.Type, TypeMD5Challenge)
	}
	// RFC 1994 Section 4.1: "The Response Identifier MUST be copied from the
	// Identifier field of the Challenge which caused the Response."
	if res.Response.Identifier != challenge.Identifier {
		t.Fatalf("Response Identifier = %d, want the Request's %d", res.Response.Identifier, challenge.Identifier)
	}

	td := res.Response.TypeData
	want := md5ExpectedResponse(challenge.Identifier, md5TestSecret, challengeValue)
	if len(td) != 1+len(want)+len(md5TestIdentity) {
		t.Fatalf("the Response Type-Data is %d octets, want %d (Value-Size, %d-octet Value, %d-octet Name)",
			len(td), 1+len(want)+len(md5TestIdentity), len(want), len(md5TestIdentity))
	}
	if int(td[0]) != md5ChallengeValueSize {
		t.Fatalf("the Response declares Value-Size %d, and MD5 produces %d octets", td[0], md5ChallengeValueSize)
	}
	if !bytes.Equal(td[1:1+md5ChallengeValueSize], want) {
		t.Fatalf("Response Value = %x, want %x (MD5 of the Identifier, the secret and the challenge, RFC 1994 Section 4.1)",
			td[1:1+md5ChallengeValueSize], want)
	}
	if name := string(td[1+md5ChallengeValueSize:]); name != md5TestIdentity {
		t.Fatalf("the Response Name is %q, want the peer identity %q", name, md5TestIdentity)
	}

	success := auth.Process(res.Response)
	if success == nil {
		t.Fatal("the MD5-Challenge Response drew no packet, want an EAP-Success")
	}
	if success.Code != CodeSuccess {
		t.Fatalf("the MD5-Challenge Response drew Code %d (%v), want %d (Success)", success.Code, auth.Err(), CodeSuccess)
	}
}

// TestRFC3748MD5ChallengeRequeryDrawsNoResponse is the discrimination for the
// test above: the peer answers the MD5-Challenge Request because of what that
// Request is, not because it answers every Request.
//
// RFC requirement: RFC3748-5.4-1 negative -- an Identity Requery arriving after
// the peer has answered the MD5-Challenge Request draws no Response at all, so
// the Response above is the Section 5.4 rule acting rather than the peer
// answering whatever arrives.
//
// RFC 3748 Section 2.1: "a peer receiving such Requests MUST treat them as
// invalid, and silently discard them.  As a result, Identity Requery is not
// supported."
//
// VALIDATES: the Response above is bound to the MD5-Challenge Request.
// PREVENTS: a peer whose answer to everything happens to look right for Type 4.
func TestRFC3748MD5ChallengeRequeryDrawsNoResponse(t *testing.T) {
	_, challenge := md5Authenticator(t)
	peer := md5Peer(t, md5TestSecret)

	if res := peer.Process(challenge); res.Response == nil {
		t.Fatalf("the MD5-Challenge Request drew %+v, want an MD5-Challenge Response", res)
	}

	requery := peer.Process(&Packet{Code: CodeRequest, Identifier: challenge.Identifier + 1, Type: TypeIdentity})
	if requery.Response != nil {
		t.Fatalf("the Identity Requery drew a Type-%d Response; Section 2.1 discards it", requery.Response.Type)
	}
	if !requery.Discarded {
		t.Fatalf("the Identity Requery drew %+v, want the explicit discard outcome", requery)
	}
}

// TestRFC3748MD5ChallengeSupportedByBothRoles drives ze's authenticator against
// ze's peer and asserts that both reach the successful end of the exchange.
//
// RFC requirement: RFC3748-5.4-2 positive -- ze's EAP server and ze's EAP peer
// each run the MD5-Challenge mechanism: NewSession accepts Type 4, NewPeerSession
// answers a Type-4 Request with a Type-4 Response, and the exchange ends with the
// authenticator reporting success and the peer accepting the EAP-Success.
//
// VALIDATES: both roles, driven end to end, over the same shared secret.
// PREVENTS: half an implementation, which is the shape this obligation has been
// unmet in since 2026-08-30: the peer could Nak Type 4 and the server could offer
// it, and neither could run it.
func TestRFC3748MD5ChallengeSupportedByBothRoles(t *testing.T) {
	auth, challenge := md5Authenticator(t)
	peer := md5Peer(t, md5TestSecret)

	res := peer.Process(challenge)
	if res.Response == nil {
		t.Fatalf("the peer drew %+v, want an MD5-Challenge Response", res)
	}

	success := auth.Process(res.Response)
	if success == nil || success.Code != CodeSuccess {
		t.Fatalf("the authenticator answered %+v (%v), want an EAP-Success", success, auth.Err())
	}
	if !auth.Succeeded() {
		t.Fatal("the authenticator sent an EAP-Success without recording the success")
	}

	final := peer.Process(success)
	if final.Err != nil {
		t.Fatalf("the peer reported %v on the EAP-Success, want the exchange to complete", final.Err)
	}
	if !final.Done {
		t.Fatalf("the EAP-Success drew %+v, want Done", final)
	}
	if !peer.Succeeded() {
		t.Fatal("the peer accepted the EAP-Success without recording the success")
	}
}

// TestRFC3748MD5ChallengeIsTheConfiguredMethod is the discrimination for the
// test above: MD5-Challenge runs because the operator configured it.
//
// RFC requirement: RFC3748-5.4-2 negative -- a peer configured for EAP-MSCHAPv2
// answers the same MD5-Challenge Request with a legacy Nak naming Type 26 and
// never with a Type-4 Response, so the support above is the configured method
// running rather than any peer answering any Type-4 Request.
//
// VALIDATES: Type 4 is dispatched from ps.method, like every other method.
// PREVENTS: an MD5-Challenge handler wired above the configured-method test,
// which would answer a Type-4 Request on a session the operator configured for
// something else, and hand a downgrading authenticator the method RFC 7296
// Section 2.16 says not to use.
func TestRFC3748MD5ChallengeIsTheConfiguredMethod(t *testing.T) {
	_, challenge := md5Authenticator(t)

	peer := NewPeerSession(TypeMSCHAPv2, md5TestIdentity, md5TestSecret)
	t.Cleanup(peer.Close)

	if res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Response == nil {
		t.Fatalf("the Identity Request drew %+v, want an Identity Response", res)
	}

	res := peer.Process(challenge)
	if res.Response == nil {
		t.Fatalf("the MD5-Challenge Request drew %+v, want a legacy Nak", res)
	}
	if res.Response.Type == TypeMD5Challenge {
		t.Fatal("a peer configured for EAP-MSCHAPv2 answered with an MD5-Challenge Response")
	}
	if res.Response.Type != TypeNAK {
		t.Fatalf("the MD5-Challenge Request drew Type %d, want %d (legacy Nak)", res.Response.Type, TypeNAK)
	}
	if !bytes.Equal(res.Response.TypeData, []byte{TypeMSCHAPv2}) {
		t.Fatalf("the Nak asks for %#x, want %#x (the configured method)", res.Response.TypeData, []byte{TypeMSCHAPv2})
	}
}

// TestRFC3748PeerSupportsTypesOneToFour drives all four Types into one peer and
// checks that each draws the answer its own section defines.
//
// RFC requirement: RFC3748-5-2 positive -- one peer configured for
// MD5-Challenge answers a Type-1 Request with an Identity Response, a Type-2
// Request with a five-octet Notification Response, an unacceptable
// authentication Type with a Type-3 legacy Nak, and a Type-4 Request with an
// MD5-Challenge Response.
//
// Type 3 is shown by the peer SENDING one, because RFC 3748 Section 5 makes the
// Nak "valid only for Response packets": a Type-3 Request is a packet the RFC
// forbids, so answering one is not what supporting Type 3 means.
//
// VALIDATES: the four Types Section 5 names, each through the peer's real entry
// point, in one session and in the order a conversation produces them.
// PREVENTS: three of four, which is what ze had until 2026-09-01.
func TestRFC3748PeerSupportsTypesOneToFour(t *testing.T) {
	peer := NewPeerSession(TypeMD5Challenge, md5TestIdentity, md5TestSecret)
	t.Cleanup(peer.Close)

	// Type 1. RFC 3748 Section 5.1: "A Response of Type 1 (Identity) SHOULD be
	// sent in Response to a Request with a Type of 1 (Identity)."
	identity := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	if identity.Response == nil || identity.Response.Type != TypeIdentity {
		t.Fatalf("the Type-1 Request drew %+v, want an Identity Response", identity)
	}
	if got := string(identity.Response.TypeData); got != md5TestIdentity {
		t.Fatalf("the Identity Response carries %q, want %q", got, md5TestIdentity)
	}

	// Type 2. RFC 3748 Section 5.2: "A Response MUST be sent in reply to the
	// Request with a Type field of 2 (Notification).  The Type-Data field of the
	// Response is zero octets in length."
	notification := peer.Process(&Packet{
		Code:       CodeRequest,
		Identifier: 2,
		Type:       TypeNotification,
		TypeData:   []byte("password expires tomorrow"),
	})
	if notification.Response == nil || notification.Response.Type != TypeNotification {
		t.Fatalf("the Type-2 Request drew %+v, want a Notification Response", notification)
	}
	if len(notification.Response.TypeData) != 0 {
		t.Fatalf("the Notification Response carries %d Type-Data octets, want 0", len(notification.Response.TypeData))
	}

	// Type 3. RFC 3748 Section 5.3.1: "Where a peer receives a Request for an
	// unacceptable authentication Type (4-253,255), or a peer lacking support for
	// Expanded Types receives a Request for Type 254, a Nak Response (Type 3) MUST
	// be sent."
	nak := peer.Process(&Packet{Code: CodeRequest, Identifier: 3, Type: TypeMSCHAPv2, TypeData: nakMSCHAPv2Challenge()})
	if nak.Response == nil || nak.Response.Type != TypeNAK {
		t.Fatalf("the Type-26 Request drew %+v, want a Type-3 legacy Nak", nak)
	}
	if !bytes.Equal(nak.Response.TypeData, []byte{TypeMD5Challenge}) {
		t.Fatalf("the Nak asks for %#x, want %#x (the configured method)", nak.Response.TypeData, []byte{TypeMD5Challenge})
	}

	// Type 4. RFC 3748 Section 5.4: "A Response MUST be sent in reply to the
	// Request."
	challengeData := append([]byte{md5ChallengeValueSize}, bytes.Repeat([]byte{0xA5}, md5ChallengeValueSize)...)
	md5 := peer.Process(&Packet{Code: CodeRequest, Identifier: 4, Type: TypeMD5Challenge, TypeData: challengeData})
	if md5.Response == nil || md5.Response.Type != TypeMD5Challenge {
		t.Fatalf("the Type-4 Request drew %+v, want an MD5-Challenge Response", md5)
	}
	want := md5ExpectedResponse(4, md5TestSecret, challengeData[1:])
	if !bytes.Equal(md5.Response.TypeData[1:1+md5ChallengeValueSize], want) {
		t.Fatalf("Response Value = %x, want %x", md5.Response.TypeData[1:1+md5ChallengeValueSize], want)
	}
}

// TestRFC3748PeerRefusesATypeOutsideOneToFour is the discrimination for the test
// above: the claim is about the four Types Section 5 names, not about every Type.
//
// RFC requirement: RFC3748-5-2 negative -- a Type-5 Request and a Type-6 Request
// each draw a Type-3 legacy Nak naming the configured method rather than a
// Response of their own Type, so the support above is bounded at Types 1-4.
//
// RFC 3748 Section 5 lists Type 5 (OTP) and Type 6 (GTC) after the four, under
// "Implementations MAY support other Types defined here or in future RFCs". Ze
// implements neither, and Section 5.3.1 is what a peer owes for a Type it does
// not run.
//
// VALIDATES: the bound of the claim above.
// PREVENTS: reading the positive as "the peer answers anything", which would make
// it agree with an implementation that answered a Type-5 Request with an OTP
// Response it cannot compute.
func TestRFC3748PeerRefusesATypeOutsideOneToFour(t *testing.T) {
	const (
		typeOTP uint8 = 5
		typeGTC uint8 = 6
	)

	for _, unsupported := range []uint8{typeOTP, typeGTC} {
		peer := NewPeerSession(TypeMD5Challenge, md5TestIdentity, md5TestSecret)
		t.Cleanup(peer.Close)

		if res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity}); res.Response == nil {
			t.Fatalf("type %d: the Identity Request drew %+v, want an Identity Response", unsupported, res)
		}

		res := peer.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: unsupported, TypeData: []byte("challenge")})
		if res.Response == nil {
			t.Fatalf("type %d: the Request drew %+v, want a legacy Nak", unsupported, res)
		}
		if res.Response.Type == unsupported {
			t.Fatalf("type %d: the peer answered with a Response of that Type, which it cannot compute", unsupported)
		}
		if res.Response.Type != TypeNAK {
			t.Fatalf("type %d: the Request drew Type %d, want %d (legacy Nak)", unsupported, res.Response.Type, TypeNAK)
		}
		if !bytes.Equal(res.Response.TypeData, []byte{TypeMD5Challenge}) {
			t.Fatalf("type %d: the Nak asks for %#x, want %#x", unsupported, res.Response.TypeData, []byte{TypeMD5Challenge})
		}
	}
}

// TestMD5ChallengeRefusesAWrongSecret drives a peer holding one secret against an
// authenticator holding another and checks that the exchange ends in refusal.
//
// RFC 1994 Section 4.2: "If the Value received in a Response is not equal to the
// expected value, then the implementation MUST transmit a CHAP packet with the
// Code field set to 4 (Failure), and SHOULD take action to terminate the link."
// Inside EAP that packet is the EAP-Failure, which Session.handleMethod sends for
// an Err result.
//
// The comparison itself is constant time by construction:
// md5ChallengeMethod.Process compares with subtle.ConstantTimeCompare, so a peer
// that can retry cannot learn the expected value one octet at a time from how
// quickly the refusal comes back. A timing measurement is not what this test
// asserts, because a Go unit test cannot separate that signal from the scheduler.
//
// VALIDATES: a wrong Value draws an EAP-Failure, leaves the session unsucceeded,
// records a reason, and sets no MSK.
// PREVENTS: an authenticator that accepts any well-formed Response, which is what
// a comparison written against the wrong buffer, or omitted, would produce.
func TestMD5ChallengeRefusesAWrongSecret(t *testing.T) {
	auth, challenge := md5Authenticator(t)
	peer := md5Peer(t, "the wrong secret")

	res := peer.Process(challenge)
	if res.Response == nil {
		t.Fatalf("the peer drew %+v, want an MD5-Challenge Response", res)
	}

	failure := auth.Process(res.Response)
	if failure == nil {
		t.Fatal("the wrong Response drew no packet, want an EAP-Failure")
	}
	if failure.Code != CodeFailure {
		t.Fatalf("the wrong Response drew Code %d, want %d (Failure)", failure.Code, CodeFailure)
	}
	if auth.Succeeded() {
		t.Fatal("the authenticator recorded success for a Response computed from another secret")
	}
	if auth.Err() == nil {
		t.Fatal("the refusal recorded no reason, so the operator reads an EAP-Failure with nothing to act on")
	}
	if msk := auth.MSK(); msk != [64]byte{} {
		t.Fatal("the refused exchange left a non-zero MSK")
	}
}

// TestMD5ChallengeRefusesMalformedTypeData feeds each malformed shape to both
// roles through their real entry points.
//
// The packet arrives unauthenticated from the network and before any key exists,
// so a Value-Size that overruns the packet is a shape both roles must expect. Each
// case must draw an error and must never index past the slice.
//
// VALIDATES: the three ways the Value-Size octet and the Type-Data can disagree,
// on the authenticator and on the peer.
// PREVENTS: a panic reachable by anybody on the path with one short packet, which
// takes the whole daemon down rather than the one exchange.
func TestMD5ChallengeRefusesMalformedTypeData(t *testing.T) {
	cases := []struct {
		name     string
		typeData []byte
	}{
		{"zero length", []byte{}},
		{"value-size larger than the packet", []byte{md5ChallengeValueSize, 1, 2, 3}},
		{"value-size of zero", []byte{0}},
	}

	for _, tc := range cases {
		t.Run("authenticator/"+tc.name, func(t *testing.T) {
			auth, challenge := md5Authenticator(t)

			failure := auth.Process(&Packet{
				Code:       CodeResponse,
				Identifier: challenge.Identifier,
				Type:       TypeMD5Challenge,
				TypeData:   tc.typeData,
			})
			if failure == nil {
				t.Fatalf("the malformed Response drew no packet, want an EAP-Failure")
			}
			if failure.Code != CodeFailure {
				t.Fatalf("the malformed Response drew Code %d, want %d (Failure)", failure.Code, CodeFailure)
			}
			if auth.Err() == nil {
				t.Fatal("the refusal recorded no reason")
			}
			if auth.Succeeded() {
				t.Fatal("the authenticator recorded success for a malformed Response")
			}
		})

		t.Run("peer/"+tc.name, func(t *testing.T) {
			peer := md5Peer(t, md5TestSecret)

			res := peer.Process(&Packet{
				Code:       CodeRequest,
				Identifier: 2,
				Type:       TypeMD5Challenge,
				TypeData:   tc.typeData,
			})
			if res.Err == nil {
				t.Fatalf("the malformed Request drew %+v, want an error", res)
			}
			if res.Response != nil {
				t.Fatalf("the malformed Request drew a Response of %d octets, want none", len(res.Response.TypeData))
			}
		})
	}
}

// TestMD5ChallengeDerivesNoKey asserts the answer the IKEv2 carrier reads before
// it builds an AUTH payload, on both roles and for all three methods.
//
// RFC 7296 Section 2.16: "For EAP methods that create a shared key as a side
// effect of authentication, that shared key MUST be used by both the initiator
// and responder to generate AUTH payloads in messages 7 and 8", and "If EAP
// methods that do not generate a shared key are used, the AUTH payloads in
// messages 7 and 8 MUST be generated using SK_pi and SK_pr, respectively."
//
// RFC 3748 Section 5.4 Security Claims for MD5-Challenge: "Key derivation:
// No".
//
// VALIDATES: Session.DerivesKey and PeerSession.DerivesKey answer false for
// MD5-Challenge and true for EAP-TLS and EAP-MSCHAPv2, and a Session with no
// method answers false.
// PREVENTS: a carrier inferring the answer from an all-zero MSK, which cannot
// distinguish a method that derives no key from one that failed, and which would
// send an AUTH payload computed over 64 zero octets.
func TestMD5ChallengeDerivesNoKey(t *testing.T) {
	auth, err := NewSession(TypeMD5Challenge, MethodConfig{Password: md5TestSecret})
	if err != nil {
		t.Fatalf("NewSession(TypeMD5Challenge) = %v, want a session", err)
	}
	t.Cleanup(auth.Close)

	if auth.DerivesKey() {
		t.Fatal("the MD5-Challenge authenticator claims to derive a key; Section 5.4 says it derives none")
	}

	// EAP-TLS and EAP-MSCHAPv2 are asked through a Session holding the method, so
	// the answer comes from the same Method.DerivesKey the carrier reaches.
	// Building an EAP-TLS Session through NewSession would need certificate
	// material this question does not depend on.
	if !(&Session{method: &tlsMethod{}}).DerivesKey() {
		t.Fatal("EAP-TLS claims to derive no key; RFC 5216 Section 2.3 defines its MSK")
	}
	if !(&Session{method: &mschapv2Method{}}).DerivesKey() {
		t.Fatal("EAP-MSCHAPv2 claims to derive no key; DeriveMSK produces one")
	}
	if (&Session{}).DerivesKey() {
		t.Fatal("a Session with no method claims to derive a key")
	}

	peers := []struct {
		name    string
		session *PeerSession
		want    bool
	}{
		{"MD5-Challenge", NewPeerSession(TypeMD5Challenge, md5TestIdentity, md5TestSecret), false},
		{"EAP-MSCHAPv2", NewPeerSession(TypeMSCHAPv2, md5TestIdentity, md5TestSecret), true},
		{"EAP-TLS", NewPeerSessionTLS(md5TestIdentity, &PeerTLSConfig{}), true},
	}
	for _, p := range peers {
		t.Cleanup(p.session.Close)
		if got := p.session.DerivesKey(); got != p.want {
			t.Fatalf("%s peer DerivesKey() = %t, want %t", p.name, got, p.want)
		}
	}
}
