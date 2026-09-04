// Related: authenticator_eap.go -- the challenge loop under test
// Related: eap.go -- the EAP-Message framing the loop drives
// RFC: rfc/short/rfc3579.md -- Sections 2.6.3, 2.6.4, 3.1, 3.2
// RFC: rfc/short/rfc2865.md -- Section 5.24 State
//
// VALIDATES: the RADIUS/EAP admin login end to end, against a mock RADIUS
// server that runs the EAP AUTHENTICATOR half and verifies every
// Message-Authenticator ze sends with its own HMAC.
// PREVENTS: a loop that answers a challenge without carrying State back, one
// that reads State, one that signs nothing, and one that lets an unverified
// challenge decide the login.

package radius

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 3579 Section 3.2 mandates HMAC-MD5
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/eap"
)

// hmacMD5 computes the Message-Authenticator the RFC's own recipe defines,
// written out here rather than by calling SignMessageAuthenticator, so a signer
// that covers the wrong octets disagrees with this file rather than with
// itself.
//
// RFC 3579 Section 3.2: "Message-Authenticator = HMAC-MD5 (Type, Identifier,
// Length, Request Authenticator, Attributes)", with the signature string
// "considered to be sixteen octets of zero".
func hmacMD5(packet, secret []byte) []byte {
	mac := hmac.New(md5.New, secret) //nolint:gosec // RFC 3579 Section 3.2 mandates HMAC-MD5
	mac.Write(packet)
	return mac.Sum(nil)
}

// messageAuthenticatorOffsetIn walks a raw RADIUS packet and returns the offset
// of the Message-Authenticator value. It is the test's own walk, independent of
// messageAuthenticatorValueOffset in packet.go.
func messageAuthenticatorOffsetIn(t *testing.T, raw []byte) int {
	t.Helper()
	for off := HeaderLen; off+2 <= len(raw); {
		attrLen := int(raw[off+1])
		require.GreaterOrEqual(t, attrLen, 2, "attribute Length at %d", off)
		require.LessOrEqual(t, off+attrLen, len(raw), "attribute at %d runs past the packet", off)
		if raw[off] == AttrMessageAuthenticator {
			require.Equal(t, 18, attrLen, "RFC 3579 Section 3.2 gives Message-Authenticator Length 18")
			return off + 2
		}
		off += attrLen
	}
	t.Fatal("the request carries no Message-Authenticator")
	return 0
}

// verifyRequestSignature recomputes the Message-Authenticator of a received
// Access-Request and fails when it does not match.
func verifyRequestSignature(t *testing.T, raw, secret []byte) {
	t.Helper()
	off := messageAuthenticatorOffsetIn(t, raw)
	covered := make([]byte, len(raw))
	copy(covered, raw)
	clear(covered[off : off+AuthenticatorLen])
	assert.Equal(t, hmacMD5(covered, secret), raw[off:off+AuthenticatorLen],
		"RFC 3579 Section 3.1 requires a correct Message-Authenticator on an EAP Access-Request")
}

// eapMockServer is a RADIUS server that speaks EAP. It runs eap.Session, the
// authenticator half, so ze's peer talks to a real EAP state machine rather
// than to a scripted reply.
type eapMockServer struct {
	*mockRADIUSServer

	mu       sync.Mutex
	requests [][]byte
	rounds   int

	// forge corrupts the Message-Authenticator of every reply after this many
	// challenges have been sent. Zero never forges.
	forgeAfter int
	// challengeForever answers every request with a fresh challenge, so the
	// conversation can only end at a bound ze imposes.
	challengeForever bool
	// dropState omits the State attribute from every challenge.
	dropState bool
	// acceptWith replaces the concluding packet's code, so a test can assert
	// that the login follows the RADIUS code and not the EAP packet.
	acceptWith uint8
	// concludeWithEAPFailure sends an EAP-Failure inside the concluding reply
	// while leaving the RADIUS code alone, which is the other way the two
	// sources of a verdict can disagree.
	concludeWithEAPFailure bool
	// lastState is the State value the server sent with its last challenge.
	lastState []byte
	// echoedState is the State each Access-Request after the first carried.
	echoedState [][]byte
}

// newEAPMockServer starts a RADIUS server running the EAP authenticator for
// method with the given password.
func newEAPMockServer(t *testing.T, secret []byte, method uint8, password string, reply []Attr) *eapMockServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)

	session, err := eap.NewSession(method, eap.MethodConfig{Password: password})
	require.NoError(t, err)

	s := &eapMockServer{
		mockRADIUSServer: &mockRADIUSServer{
			conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{}),
		},
		acceptWith: CodeAccessAccept,
	}
	s.handler = func(req []byte) []byte { return s.answer(t, req, secret, session, reply) }
	go s.serve()
	t.Cleanup(s.close)
	return s
}

// answer verifies one Access-Request, drives the EAP authenticator with the EAP
// packet it carries, and builds the reply.
func (s *eapMockServer) answer(t *testing.T, req, secret []byte, session *eap.Session, reply []Attr) []byte {
	t.Helper()
	s.mu.Lock()
	s.requests = append(s.requests, append([]byte{}, req...))
	round := s.rounds
	s.rounds++
	forge := s.forgeAfter > 0 && round >= s.forgeAfter
	forever, dropState, code := s.challengeForever, s.dropState, s.acceptWith
	failEAP := s.concludeWithEAPFailure
	s.mu.Unlock()

	verifyRequestSignature(t, req, secret)

	decoded, err := Decode(req)
	require.NoError(t, err)

	if round > 0 {
		s.mu.Lock()
		s.echoedState = append(s.echoedState, decoded.FindAttr(AttrState))
		s.mu.Unlock()
	}

	encoded, err := eapPacketFrom(decoded)
	require.NoError(t, err)
	require.NotNil(t, encoded, "every EAP Access-Request carries an EAP-Message")
	response, err := eap.DecodePacket(encoded)
	require.NoError(t, err)

	next := session.Process(response)
	if next == nil {
		return nil
	}
	if forever {
		// A server that never concludes: re-issue the method's first Request.
		next = &eap.Packet{
			Code: eap.CodeRequest, Identifier: response.Identifier + 1,
			Type: eap.TypeMD5Challenge, TypeData: []byte{16, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		}
	}

	if failEAP && next.Code == eap.CodeSuccess {
		// The RADIUS code stays Access-Accept below; only the encapsulated packet
		// changes, which is the disagreement RFC 3579 Section 2.6.3 is written for.
		next = &eap.Packet{Code: eap.CodeFailure, Identifier: next.Identifier}
	}
	replyCode := uint8(CodeAccessChallenge)
	attrs, err := appendEAPMessage(nil, next.Encode())
	require.NoError(t, err)
	switch {
	case forever:
	case failEAP && next.Code == eap.CodeFailure:
		replyCode = code
		attrs = append(attrs, reply...)
		attrs = append(attrs, Attr{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)})
	case next.Code == eap.CodeSuccess:
		replyCode = code
		attrs = append(attrs, reply...)
		attrs = append(attrs, Attr{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)})
	case next.Code == eap.CodeFailure:
		replyCode = CodeAccessReject
	}
	if replyCode == CodeAccessChallenge && !dropState {
		state := []byte{'z', 'e', byte(round), 0xa5}
		s.mu.Lock()
		s.lastState = state
		s.mu.Unlock()
		attrs = append(attrs, Attr{Type: AttrState, Value: state})
	}
	attrs = append(attrs, Attr{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)})

	return buildSignedReply(t, replyCode, req, secret, attrs, forge)
}

// buildSignedReply encodes a reply, signs its Message-Authenticator with the
// Request Authenticator substituted, then computes the Response Authenticator
// over the signed bytes.
//
// RFC 3579 Section 3.2: "The Message-Authenticator is calculated and inserted
// in the packet before the Response Authenticator is calculated." When forge is
// set the signature is corrupted after both steps, which is what a spoofed
// packet looks like to the client.
func buildSignedReply(t *testing.T, code uint8, req, secret []byte, attrs []Attr, forge bool) []byte {
	t.Helper()
	pkt := &Packet{Code: code, Identifier: req[1], Attrs: attrs}
	copy(pkt.Authenticator[:], req[4:4+AuthenticatorLen])

	raw := make([]byte, MaxPacketLen)
	n, err := pkt.EncodeTo(raw, 0)
	require.NoError(t, err)
	raw = raw[:n]

	off := messageAuthenticatorOffsetIn(t, raw)
	copy(raw[off:off+AuthenticatorLen], hmacMD5(raw, secret))

	var requestAuth [AuthenticatorLen]byte
	copy(requestAuth[:], req[4:4+AuthenticatorLen])
	auth := ResponseAuthenticator(code, req[1], uint16(n), requestAuth, raw[HeaderLen:], secret)
	copy(raw[4:4+AuthenticatorLen], auth[:])

	if forge {
		raw[off] ^= 0xff
	}
	return raw
}

// captured returns every Access-Request the server received, decoded.
func (s *eapMockServer) captured(t *testing.T) []*Packet {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Packet, 0, len(s.requests))
	for _, raw := range s.requests {
		pkt, err := Decode(raw)
		require.NoError(t, err)
		out = append(out, pkt)
	}
	return out
}

func (s *eapMockServer) states() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte{}, s.echoedState...)
}

func (s *eapMockServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func eapAuthenticator(t *testing.T, addr string, secret []byte, method AuthMethod) *radiusAuthenticator {
	t.Helper()
	return testAuthenticator(t, addr, secret, ExtractedConfig{
		ProfileAttr: AttrFilterID, AuthMethod: method,
	})
}

// TestRadiusAdminEapAccessRequestIsSignedAndCarriesEAPMessage is the first
// round's assertion: what ze puts on the wire before any challenge comes back.
//
// VALIDATES: AC-2 -- the first Access-Request carries the peer's
// EAP-Response/Identity in EAP-Message attributes. AC-3 -- it carries a
// Message-Authenticator the server can recompute. RFC 3579 Section 3.3 Note 1 --
// it carries no User-Password and no CHAP-Password.
// PREVENTS: an EAP login that sends the password in the clear beside the EAP
// packet, and one whose signature no server can verify.
//
// RFC requirement: RFC3579-3.1-3 positive -- an Access-Request containing an
// EAP-Message carries a Message-Authenticator, and the server's independent
// HMAC matches it (client.go encodeRequest, packet.go
// SignMessageAuthenticator).
// RFC requirement: RFC3579-3.3-2 positive -- the same request carries no
// User-Password and no CHAP-Password, so only one of the four credential types
// is present (authenticator_eap.go eapCredential).
func TestRadiusAdminEapAccessRequestIsSignedAndCarriesEAPMessage(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)

	requests := srv.captured(t)
	require.NotEmpty(t, requests)
	first := requests[0]

	// The peer's own Identity Response, carried whole.
	encoded, err := eapPacketFrom(first)
	require.NoError(t, err)
	identity, err := eap.DecodePacket(encoded)
	require.NoError(t, err)
	assert.Equal(t, eap.CodeResponse, identity.Code)
	assert.Equal(t, eap.TypeIdentity, identity.Type)
	assert.Equal(t, []byte("alice"), identity.TypeData)

	// RFC 3579 Section 2.1: the User-Name attribute carries the Type-Data of the
	// EAP-Response/Identity, in this and every later Access-Request.
	for index, req := range requests {
		assert.Equal(t, []byte("alice"), req.FindAttr(AttrUserName), "request %d", index)
		assert.Nil(t, req.FindAttr(AttrUserPassword), "request %d carries no User-Password", index)
		assert.Nil(t, req.FindAttr(AttrCHAPPassword), "request %d carries no CHAP-Password", index)
		require.Len(t, req.FindAttr(AttrMessageAuthenticator), AuthenticatorLen, "request %d", index)
	}
	// The server recomputed the signature of every request it answered, inside
	// verifyRequestSignature, so reaching an Access-Accept is that proof.
	assert.GreaterOrEqual(t, len(requests), 2, "MD5-Challenge needs at least one challenge round")
}

// TestRadiusAdminEapChallengeLoopCarriesState is the State assertion.
//
// VALIDATES: AC-7 -- every Access-Request answering a challenge carries that
// challenge's State byte for byte. AC-8 -- a challenge with no State produces a
// request with no State attribute.
// PREVENTS: a loop that drops State, which makes a stateful server lose the
// conversation, and one that rewrites it.
//
// RFC requirement: RFC2865-5.25-1 positive -- "The client MUST NOT interpret
// the State (Section 5.24) or Class (Section 5.25) attribute locally." The
// observable form of not interpreting it is returning it unchanged, which
// Section 5.24 states as its own obligation: State "MUST be sent unmodified
// from the client to the server in the new Access-Request reply to that
// challenge". Each round's expected value is reproduced from the server's own
// recipe in this file rather than read back from the server, so a client that
// parsed the attribute and rebuilt it would fail
// (authenticator_eap.go authenticateEAP).
func TestRadiusAdminEapChallengeLoopCarriesState(t *testing.T) {
	secret := []byte("testing123")

	t.Run("state echoed unmodified", func(t *testing.T) {
		srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
			[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
		a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
		res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
		require.NoError(t, err)
		assert.True(t, res.Authenticated)

		echoed := srv.states()
		require.NotEmpty(t, echoed, "MS-CHAPv2 runs several challenge rounds")
		for index, state := range echoed {
			// The server's State for round N is {'z','e',N,0xa5}: the value is
			// reproduced here rather than read back from the server, so a loop that
			// echoed its own invention would fail.
			assert.Equal(t, []byte{'z', 'e', byte(index), 0xa5}, state,
				"request %d MUST carry the challenge's State unmodified", index+1)
		}
	})

	t.Run("no state, no attribute", func(t *testing.T) {
		srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
			[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
		srv.mu.Lock()
		srv.dropState = true
		srv.mu.Unlock()

		a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
		_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
		require.NoError(t, err)

		for index, req := range srv.captured(t) {
			assert.Nil(t, req.FindAttr(AttrState),
				"request %d MUST carry no State when the challenge sent none", index)
		}
	})
}

// TestRadiusAdminEapDiscardsUnauthenticatedChallenge is the fail-closed case.
//
// VALIDATES: AC-6 -- a challenge whose Message-Authenticator does not verify is
// silently discarded, so the request stays outstanding and the client
// retransmits. The login ends as an infrastructure error, never as a rejection.
// PREVENTS: a forged packet stopping the AAA chain. Reporting the mismatch as a
// rejection would let anybody on the path deny a login with one datagram, which
// is exactly the attack the attribute exists to stop.
//
// RFC requirement: RFC3579-3.1-4 negative -- "A NAS supporting the EAP-Message
// attribute MUST calculate the correct value of the Message-Authenticator and
// MUST silently discard the packet if it does not match the value sent"
// (client.go dispatchResponse, packet.go verifyResponseMessageAuthenticator).
func TestRadiusAdminEapDiscardsUnauthenticatedChallenge(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello", nil)
	srv.mu.Lock()
	srv.forgeAfter = 1 // the identity round answers honestly; the next reply is forged
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.Error(t, err)
	assert.NotErrorIs(t, err, aaa.ErrAuthRejected,
		"a discarded reply MUST NOT stop the AAA chain")
	assert.False(t, res.Authenticated)

	// The discard left the request outstanding, so the client retransmitted it:
	// the server saw the same round more than once.
	assert.Greater(t, srv.requestCount(), 2,
		"a discarded reply leaves the Access-Request outstanding for retransmission")
}

// TestRadiusAdminEapStopsAtTheRoundCap bounds a server that never concludes.
//
// VALIDATES: AC-9 -- a server that answers every request with another challenge
// ends the login with an error, and the chain falls through to the next
// backend. The bound is the EAP peer's own round cap, which no server input can
// raise.
// PREVENTS: a login held open forever by a cooperative-looking server, and a
// bound the server can push out by sending more packets.
func TestRadiusAdminEapStopsAtTheRoundCap(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello", nil)
	srv.mu.Lock()
	srv.challengeForever = true
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	start := time.Now()
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.Error(t, err)
	assert.NotErrorIs(t, err, aaa.ErrAuthRejected, "the chain MUST try the next backend")
	assert.False(t, res.Authenticated)
	assert.Less(t, time.Since(start), a.budget,
		"the round cap MUST end the conversation before the time budget does")
	// 20 is eap.maxEAPRounds. The identity round is one of them, so the server
	// can never see more than that many requests.
	assert.LessOrEqual(t, srv.requestCount(), 20,
		"the peer's round cap bounds the number of Access-Requests")
}

// TestRadiusAdminEapOneEAPPacketPerRadiusPacket holds the rule that gives the
// concatenation on the way in a meaning.
//
// VALIDATES: AC-13 -- every Access-Request ze sends encapsulates exactly one
// EAP packet: its EAP-Message attributes are one consecutive run, and the
// octets they carry decode as a single packet whose Length is the whole run.
// PREVENTS: a loop that accumulates EAP packets across rounds, or that appends
// a second EAP-Message run beside the first.
//
// RFC requirement: RFC3579-3.1-2 positive -- "Multiple EAP packets MUST NOT be
// encoded within EAP-Message attributes contained within a single
// Access-Challenge, Access-Accept, Access-Reject or Access-Request packet"
// (authenticator_eap.go eapCredential, eap.go appendEAPMessage).
func TestRadiusAdminEapOneEAPPacketPerRadiusPacket(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)

	for index, req := range srv.captured(t) {
		values := eapMessageValues(t, req.Attrs)
		require.NotEmpty(t, values, "request %d carries an EAP-Message", index)

		encoded, err := eapPacketFrom(req)
		require.NoError(t, err)
		// The EAP Length field must account for every octet in the run: an octet
		// past it would be a second EAP packet riding along.
		require.GreaterOrEqual(t, len(encoded), 4)
		eapLen := int(encoded[2])<<8 | int(encoded[3])
		assert.Equal(t, len(encoded), eapLen,
			"request %d encapsulates exactly one EAP packet", index)
	}
}

// TestRadiusAdminEapDecisionFollowsTheRadiusCode separates the two sources a
// NAS could read a verdict from.
//
// VALIDATES: an Access-REJECT concluding a successful EAP conversation stops
// the chain, even though the EAP-Message it carries says EAP-Success. The
// decision comes from the RADIUS code alone.
// PREVENTS: a NAS that logs an operator in because the encapsulated EAP packet
// said Success. RFC 3579 Section 2.6.3 exists because the two can disagree, and
// the RADIUS packet type is the authorization.
//
// RFC requirement: RFC3579-2.6.3-1 positive -- "The NAS MUST make its access
// control decision based solely on the RADIUS Packet Type
// (Access-Accept/Access-Reject)" (authenticator.go result).
// RFC requirement: RFC3579-2.6.3-2 negative -- "The access control decision
// MUST NOT be based on the contents of the EAP packet encapsulated in one or
// more EAP-Message attributes, if present": the reply here carries an
// EAP-Success and the login is refused.
func TestRadiusAdminEapDecisionFollowsTheRadiusCode(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	srv.mu.Lock()
	srv.acceptWith = CodeAccessReject
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.ErrorIs(t, err, aaa.ErrAuthRejected, "an Access-Reject stops the chain")
	assert.False(t, res.Authenticated)
	assert.Equal(t, aaaName, res.Source)
}

// TestRadiusAdminEapProfileMapping proves selecting EAP changes the credential
// and nothing downstream of the verdict.
//
// VALIDATES: AC-10 -- an Access-Accept concluding an EAP exchange maps
// Filter-Id to profiles exactly as the PAP path maps it, and the session is
// tagged source=radius.
// PREVENTS: an EAP path with its own profile mapping, which would drift from
// the one PAP and CHAP share.
func TestRadiusAdminEapProfileMapping(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", []Attr{
		{Type: AttrFilterID, Value: []byte("netops")},
		{Type: AttrFilterID, Value: []byte("audit")},
	})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)

	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"netops", "audit"}, res.Profiles)
	assert.Equal(t, aaaName, res.Source)
}

// TestRadiusAdminEapWrongPasswordIsRejected is the negative credential case.
//
// VALIDATES: a password the server does not hold ends the EAP conversation in
// an EAP-Failure inside an Access-Reject, and the chain stops.
// PREVENTS: a loop that reports success whenever the conversation completes,
// whatever the method decided.
func TestRadiusAdminEapWrongPasswordIsRejected(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "TheServersPassword", nil)

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "wrong"})

	require.Error(t, err)
	assert.False(t, res.Authenticated)
}

// TestRadiusAdminEapPapPathUnchanged is the AC-1 control.
//
// VALIDATES: AC-1 -- with auth-method absent the request carries User-Password
// and no EAP-Message, and an Access-Challenge is still a rejection.
// PREVENTS: the EAP work leaking into the default path, which every deployment
// that never sets the leaf is running.
func TestRadiusAdminEapPapPathUnchanged(t *testing.T) {
	key := []byte("testing123")
	accept := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	defer accept.close()

	a := testAuthenticator(t, accept.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)

	req := accept.captured(t)
	assert.NotNil(t, req.FindAttr(AttrUserPassword), "PAP still carries User-Password")
	assert.Nil(t, req.FindAttr(AttrEAPMessage), "PAP carries no EAP-Message")
	assert.Nil(t, req.FindAttr(AttrMessageAuthenticator),
		"RFC 3579 Section 3.2 leaves the attribute optional on a User-Password request")

	challenge := newReplyServer(t, key, CodeAccessChallenge, nil)
	defer challenge.close()
	b := testAuthenticator(t, challenge.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	_, err = b.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	assert.ErrorIs(t, err, aaa.ErrAuthRejected,
		"RFC 2865 Section 4.4 keeps an Access-Challenge a rejection for PAP")
}

// TestAccountingRequestRefusesEAPAttributes holds the one prohibition RFC 3579
// puts on the accounting path.
//
// VALIDATES: an Accounting-Request carrying an EAP-Message or a
// Message-Authenticator is refused at the socket rather than sent.
// PREVENTS: a future accounting builder adding either attribute. Ze's builder
// adds neither today, so without this the prohibition holds only by accident.
//
// RFC requirement: RFC3579-3.3-1 negative -- "The EAP-Message and
// Message-Authenticator attributes specified in this document MUST NOT be
// present in an Accounting-Request" (client.go encodeRequest).
// RFC requirement: RFC3579-3.3-1 positive -- the control at the end of this
// body: an Accounting-Request carrying neither attribute encodes and is sent,
// so the refusal above is not a client that refuses every accounting record
// (client.go encodeRequest).
func TestAccountingRequestRefusesEAPAttributes(t *testing.T) {
	secret := []byte("testing123")
	buf := make([]byte, MaxPacketLen)

	for _, attrType := range []uint8{AttrEAPMessage, AttrMessageAuthenticator} {
		pkt := &Packet{
			Code:       CodeAccountingReq,
			Identifier: 3,
			Attrs: []Attr{
				{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)},
				{Type: attrType, Value: make([]byte, AuthenticatorLen)},
			},
		}
		_, _, err := encodeRequest(pkt, secret, buf)
		require.Error(t, err, "attribute %d MUST NOT reach an Accounting-Request", attrType)
	}

	// The control: the same record without either attribute encodes.
	ok := &Packet{
		Code:       CodeAccountingReq,
		Identifier: 3,
		Attrs:      []Attr{{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStart)}},
	}
	n, _, err := encodeRequest(ok, secret, buf)
	require.NoError(t, err)
	assert.Positive(t, n)
}

// TestEAPRequestWithoutMessageAuthenticatorIsRefused proves the socket-side
// half of the signing obligation.
//
// VALIDATES: an Access-Request carrying an EAP-Message and no
// Message-Authenticator placeholder is refused rather than sent unprotected.
// PREVENTS: a builder that forgets the placeholder. The signer would then find
// nothing to sign and report it, and without this check the caller would send
// the packet anyway.
//
// RFC requirement: RFC3579-3.1-3 negative -- an EAP-bearing Access-Request with
// no Message-Authenticator never reaches the wire (client.go encodeRequest).
func TestEAPRequestWithoutMessageAuthenticatorIsRefused(t *testing.T) {
	buf := make([]byte, MaxPacketLen)
	pkt := &Packet{
		Code:       CodeAccessRequest,
		Identifier: 5,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("alice")},
			{Type: AttrEAPMessage, Value: []byte{0x02, 0x00, 0x00, 0x0a, 0x01, 'a', 'l', 'i', 'c', 'e'}},
		},
	}
	_, _, err := encodeRequest(pkt, []byte("testing123"), buf)
	require.Error(t, err)

	// The control: the same packet with the placeholder encodes and is signed.
	pkt.Attrs = append(pkt.Attrs, Attr{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)})
	n, _, err := encodeRequest(pkt, []byte("testing123"), buf)
	require.NoError(t, err)
	off := messageAuthenticatorOffsetIn(t, buf[:n])
	assert.NotEqual(t, make([]byte, AuthenticatorLen), buf[off:off+AuthenticatorLen],
		"the placeholder MUST be replaced by the computed HMAC")
}

// eapIdentifierEcho is a sanity check on the Identifier discipline the loop
// inherits from the peer.
//
// VALIDATES: each EAP-Response ze sends carries the Identifier of the
// EAP-Request it answers.
// PREVENTS: a loop that renumbers the EAP conversation, which a real server
// answers by discarding every Response.
//
// This case carries no RFC tag, and that is deliberate. The EAP Identifier
// discipline is RFC 3748 Section 4.1, the peer owns it, and
// internal/core/eap/rfc3748_walk_test.go proves it. What is asserted here is
// that the RADIUS carrier keeps its own Identifier space separate from the EAP
// one, which no RFC states as an obligation on its own.
func TestRadiusAdminEapIdentifiersSurviveTheCarrier(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)

	// The server's eap.Session refuses a Response whose Identifier does not match
	// its Request, so reaching an Access-Accept already proves the echo. This
	// asserts the RADIUS Identifier is INDEPENDENT of it: the two counters must
	// not be conflated.
	requests := srv.captured(t)
	require.GreaterOrEqual(t, len(requests), 2)
	radiusIDs := map[uint8]bool{}
	for _, req := range requests {
		radiusIDs[req.Identifier] = true
	}
	assert.Len(t, radiusIDs, len(requests),
		"each round is a NEW Access-Request with its own Identifier, not a retransmission")
}

// binaryLengthOfEAPRun is used by the boundary case below.
func binaryLengthOfEAPRun(values [][]byte) int {
	total := 0
	for _, v := range values {
		total += len(v)
	}
	return total
}

// TestRadiusAdminEapLongPacketCrossesTheAttributeBoundary drives a real login
// whose EAP packet needs more than one attribute.
//
// VALIDATES: AC-4 end to end -- an EAP packet longer than 253 octets reaches
// the server intact after the split, and the server's own reassembly of it
// decodes.
// PREVENTS: a split that is correct in a unit test and wrong on the wire,
// because the encoder placed something between the pieces.
func TestRadiusAdminEapLongPacketCrossesTheAttributeBoundary(t *testing.T) {
	secret := []byte("testing123")
	// 250 octets of identity: the EAP-Response/Identity is 5 + 250 = 255, which
	// needs two EAP-Message attributes, while the User-Name attribute beside it
	// is 252 octets and still fits under the 255-octet attribute limit RFC 2865
	// Section 5 imposes. A longer identity would fail at User-Name instead, and
	// would prove nothing about the EAP split.
	longName := strings.Repeat("u", 250)
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello", nil)

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	// The login itself fails on the password check; what this asserts is the
	// framing of the request that carried the long identity.
	_, _ = a.Authenticate(aaa.AuthRequest{Username: longName, Password: "wrong"})

	requests := srv.captured(t)
	require.NotEmpty(t, requests)
	values := eapMessageValues(t, requests[0].Attrs)
	require.Len(t, values, 2, "a 255-octet EAP packet is two EAP-Message attributes")
	assert.Len(t, values[0], maxEAPMessageValue)
	assert.Len(t, values[1], 2)
	assert.Equal(t, 255, binaryLengthOfEAPRun(values))

	encoded, err := eapPacketFrom(requests[0])
	require.NoError(t, err)
	assert.Equal(t, 255, int(binary.BigEndian.Uint16(encoded[2:4])))
}

// TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes is the other side of
// TestRadiusAdminEapDecisionFollowsTheRadiusCode, and it is the case that
// matters most: the RADIUS code says yes and the encapsulated EAP packet says
// no.
//
// VALIDATES: an Access-Accept carrying an EAP-Failure logs the operator in, and
// the profile mapping runs as usual. The peer's own verdict is reported and
// then dropped.
// PREVENTS: the defect this pair found on 2026-09-04. authenticateEAP returned
// the peer's error before it read resp.Code, so an EAP-Failure inside an
// Access-Accept ended the login as an infrastructure error and the AAA chain
// carried on to the next backend. That put the encapsulated packet back in
// charge of the access decision through the error path, which is exactly what
// Section 2.6.3 forbids, and no test could see it while only the
// Reject-plus-Success direction was covered.
//
// RFC requirement: RFC3579-2.6.3-1 negative -- a decision that followed
// anything OTHER than the RADIUS Packet Type would refuse this login; it
// succeeds, so the packet type alone decided it (authenticator_eap.go
// authenticateEAP, authenticator.go result).
// RFC requirement: RFC3579-2.6.3-2 positive -- "The access control decision
// MUST NOT be based on the contents of the EAP packet encapsulated in one or
// more EAP-Message attributes, if present": the reply here carries an
// EAP-Failure and the login is granted.
func TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	srv.mu.Lock()
	srv.concludeWithEAPFailure = true
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

	require.NoError(t, err, "an Access-Accept MUST authorize whatever the EAP packet says")
	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"admin"}, res.Profiles)
	assert.Equal(t, aaaName, res.Source)
}

// TestRadiusAdminEapVerifiedChallengeIsAccepted is the positive half of the
// silent-discard rule.
//
// VALIDATES: a challenge whose Message-Authenticator verifies is delivered to
// the loop and answered, so the conversation advances. Without it the negative
// case above is satisfied by a client that discards every reply.
// PREVENTS: a verifier that rejects a correct signature, which would make every
// EAP login fail as an infrastructure error and silently drop the whole method
// to the next backend.
//
// RFC requirement: RFC3579-3.1-4 positive -- "A NAS supporting the EAP-Message
// attribute MUST calculate the correct value of the Message-Authenticator": a
// reply whose value matches is accepted and answered, and the exchange reaches
// an Access-Accept (client.go dispatchResponse, packet.go
// verifyResponseMessageAuthenticator).
func TestRadiusAdminEapVerifiedChallengeIsAccepted(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMD5Challenge, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMD5)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)

	// The conversation ADVANCED past the challenge, which is what makes this the
	// positive of the discard: a discarded challenge would have produced a
	// retransmission of the first request instead of a second, different one.
	requests := srv.captured(t)
	require.GreaterOrEqual(t, len(requests), 2)
	first, err := eapPacketFrom(requests[0])
	require.NoError(t, err)
	second, err := eapPacketFrom(requests[1])
	require.NoError(t, err)
	assert.NotEqual(t, first, second,
		"the second request answers the challenge rather than repeating the first")
	assert.Equal(t, eap.TypeMD5Challenge, second[4],
		"the peer answered the MD5-Challenge the verified reply carried")
}

// TestAccessRequestRefusesTwoCredentialTypes is the negative of the exclusive
// credential rule.
//
// VALIDATES: an Access-Request carrying an EAP-Message beside a User-Password,
// or beside a CHAP-Password, is refused at the socket rather than sent.
// PREVENTS: a builder that APPENDS the EAP credential to a password one. Both
// present would still authenticate against a permissive server, and the
// password would travel under the reversible RFC 2865 Section 5.2 hiding while
// the operator believed EAP was in use, so only the refusal catches it.
//
// RFC requirement: RFC3579-3.3-2 negative -- "An Access-Request that contains
// either a User-Password or CHAP-Password or ARAP-Password or one or more
// EAP-Message attributes MUST NOT contain more than one type of those four
// attributes" (client.go encodeRequest, oneCredentialType).
func TestAccessRequestRefusesTwoCredentialTypes(t *testing.T) {
	secret := []byte("testing123")
	buf := make([]byte, MaxPacketLen)
	eapMessage := Attr{Type: AttrEAPMessage, Value: []byte{0x02, 0x00, 0x00, 0x0a, 0x01, 'a', 'l', 'i', 'c', 'e'}}
	signature := Attr{Type: AttrMessageAuthenticator, Value: make([]byte, AuthenticatorLen)}

	for _, beside := range []Attr{
		{Type: AttrUserPassword, Value: []byte("Hello")},
		{Type: AttrCHAPPassword, Value: make([]byte, 17)},
	} {
		pkt := &Packet{
			Code:       CodeAccessRequest,
			Identifier: 11,
			Attrs:      []Attr{eapMessage, beside, signature},
		}
		_, _, err := encodeRequest(pkt, secret, buf)
		require.Errorf(t, err, "attribute %d MUST NOT ride beside an EAP-Message", beside.Type)
		assert.Contains(t, err.Error(), "one credential type")
	}

	// The control: the EAP credential alone encodes.
	ok := &Packet{Code: CodeAccessRequest, Identifier: 11, Attrs: []Attr{eapMessage, signature}}
	n, _, err := encodeRequest(ok, secret, buf)
	require.NoError(t, err)
	assert.Positive(t, n)
}
