// RFC 2865 NAS obligations the admin-authentication path owes and did not meet
// before this file existed.
//
// VALIDATES: the Request Authenticator changes with the Identifier on failover
// (Section 4.1), an empty shared secret never reaches a live client (Section 3),
// an Access-Challenge is treated as an Access-Reject (Section 4.4), an
// Access-Accept naming a Service-Type this NAS does not offer is treated as an
// Access-Reject (Sections 5.6 and 1.1), and a zero-length text attribute is
// omitted rather than sent (Section 5).
// PREVENTS: a second server receiving a fresh Identifier under the previous
// Request Authenticator, a forgeable session under a zero-length secret, an
// Access-Challenge letting the AAA chain try another backend, an Access-Accept
// authorizing a service the NAS cannot provide, and a zero-length User-Name.

package radius

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
)

// capturingServer answers with a fixed code and records every request it read.
type capturingServer struct {
	mock *mockRADIUSServer
	reqs chan []byte
}

// newCapturingServer answers every request with code plus attrs. When reply is
// false it records the request and stays silent, so the client fails over.
func newCapturingServer(t *testing.T, sharedKey []byte, code uint8, attrs []Attr, reply bool) *capturingServer {
	t.Helper()
	// The listener is built here rather than by newMockServer, because that
	// helper starts serve() before it returns and this server needs its own
	// handler installed FIRST. Writing m.handler after the goroutine started is
	// a data race, and it is the one the suite reported on 2026-09-04
	// (plan/journal/false-synchronization-claim.md). Every other server in
	// this package already installs the handler before `go serve()`.
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	m := &mockRADIUSServer{conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{})}
	c := &capturingServer{mock: m, reqs: make(chan []byte, 8)}
	m.handler = func(req []byte) []byte {
		cp := make([]byte, len(req))
		copy(cp, req)
		select {
		case c.reqs <- cp:
		default:
		}
		if !reply {
			return nil
		}
		return buildReplyResponse(code, req, sharedKey, attrs)
	}
	go m.serve()
	t.Cleanup(m.close)
	return c
}

func (c *capturingServer) request(t *testing.T) []byte {
	t.Helper()
	select {
	case raw := <-c.reqs:
		return raw
	case <-time.After(5 * time.Second):
		t.Fatal("no Access-Request reached the server")
		return nil
	}
}

// TestRFC2865FailoverRegeneratesRequestAuthenticator drives SendToServers across
// two servers and reads both Access-Requests off the wire. The second server
// gets a new Identifier, so RFC 2865 Section 4.1 requires a new Request
// Authenticator with it, and the User-Password keystream is derived from that
// authenticator, so the ciphertext must be re-derived too.
func TestRFC2865FailoverRegeneratesRequestAuthenticator(t *testing.T) {
	secret1 := []byte("secret-one")
	secret2 := []byte("secret-two")
	password := []byte("hunter2!")

	silent := newCapturingServer(t, secret1, CodeAccessRequest, nil, false)
	answering := newCapturingServer(t, secret2, CodeAccessAccept, nil, true)

	client, err := NewClient(ClientConfig{
		Servers: []Server{
			{Address: silent.mock.addr, SharedKey: secret1},
			{Address: answering.mock.addr, SharedKey: secret2},
		},
		Timeout: 100 * time.Millisecond,
		Retries: 1,
	})
	require.NoError(t, err)
	defer closeSilent(client)

	auth, err := RandomAuthenticator()
	require.NoError(t, err)
	pkt := &Packet{
		Code:          CodeAccessRequest,
		Authenticator: auth,
		Attrs: []Attr{
			{Type: AttrUserName, Value: AttrString("alice")},
			{Type: AttrUserPassword, Value: password},
		},
	}
	_, err = client.SendToServers(t.Context(), pkt)
	require.NoError(t, err)

	first := silent.request(t)
	second := answering.request(t)

	// RFC requirement: RFC2865-4.1-6 positive -- the failover assigns a new
	// Identifier to the second server, and Section 4.1 requires a new Request
	// Authenticator each time a new Identifier is used, so the two requests carry
	// different Identifiers AND different Request Authenticators.
	assert.NotEqual(t, first[1], second[1], "each server MUST get a new Identifier")
	assert.NotEqual(t, first[4:4+AuthenticatorLen], second[4:4+AuthenticatorLen],
		"a new Identifier MUST carry a new Request Authenticator")

	var secondAuth, firstAuth [AuthenticatorLen]byte
	copy(secondAuth[:], second[4:4+AuthenticatorLen])
	copy(firstAuth[:], first[4:4+AuthenticatorLen])

	decoded, err := Decode(second)
	require.NoError(t, err)
	sentCipher := decoded.FindAttr(AttrUserPassword)
	require.NotNil(t, sentCipher, "the second request MUST carry the User-Password")

	// RFC requirement: RFC2865-4.1-6 positive -- the User-Password keystream is
	// MD5(secret + Request Authenticator), so the ciphertext the second server
	// receives is the one derived from the Request Authenticator that request
	// actually carries.
	assert.Equal(t, EncodeUserPassword(password, secret2, secondAuth), sentCipher,
		"User-Password MUST be re-encoded under the request's own authenticator")

	// RFC requirement: RFC2865-4.1-6 negative -- the previous request's
	// authenticator is NOT reused: encoding under it yields a different
	// ciphertext than the one sent, so no keystream was carried over.
	assert.NotEqual(t, EncodeUserPassword(password, secret2, firstAuth), sentCipher,
		"the previous Request Authenticator MUST NOT still key the User-Password")
}

// TestRFC2865EmptySharedSecretBuildsNoClient drives the AAA backend build, the
// entry point an operator's configuration reaches, and requires that a server
// row with no key contributes no authenticator.
func TestRFC2865EmptySharedSecretBuildsNoClient(t *testing.T) {
	build := func(key string) aaa.Contribution {
		inner := config.NewTree()
		srv := config.NewTree()
		srv.Set("port", "1812")
		srv.Set("key", key)
		inner.AddListEntry("server", "10.0.0.1", srv)
		contribution, err := radiusBackend{}.Build(aaa.BuildParams{ConfigTree: radiusTree(inner)})
		require.NoError(t, err)
		t.Cleanup(func() {
			if contribution.Close != nil {
				_ = contribution.Close()
			}
		})
		return contribution
	}

	// RFC requirement: RFC2865-3-8 positive -- a server row carrying a non-empty
	// shared secret builds the RADIUS admin authenticator.
	assert.NotNil(t, build("secret-one").Authenticator,
		"a non-empty shared secret MUST build the RADIUS backend")

	// RFC requirement: RFC2865-3-8 negative -- a server row whose key is the
	// empty string builds no authenticator, so no packet is ever signed with a
	// zero-length secret (Section 3, "would allow packets to be trivially
	// forged").
	assert.Nil(t, build("").Authenticator,
		"a zero-length shared secret MUST NOT reach a live RADIUS client")
}

// TestRFC2865AccessChallengeIsRejection covers a NAS that does not implement
// challenge/response. Ze's admin login sends one Access-Request and has no path
// to answer a challenge, so Section 4.4 makes an Access-Challenge a rejection.
func TestRFC2865AccessChallengeIsRejection(t *testing.T) {
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessChallenge, nil)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, DefaultProfiles: []string{"operator"},
	})

	// RFC requirement: RFC2865-4.4-1 positive -- an Access-Challenge returns the
	// same rejection an Access-Reject returns, so the login is denied.
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	assert.False(t, res.Authenticated, "an Access-Challenge MUST NOT authenticate")
	require.ErrorIs(t, err, aaa.ErrAuthRejected,
		"an Access-Challenge MUST be treated as an Access-Reject")

	// RFC requirement: RFC2865-4.4-1 negative -- treating it as a rejection stops
	// the AAA chain, so a later backend that would have accepted the same
	// credentials is never consulted. A plain error would fall through to it.
	next := &countingAuthenticator{}
	chain := &aaa.ChainAuthenticator{Backends: []aaa.Authenticator{a, next}}
	_, chainErr := chain.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, chainErr, aaa.ErrAuthRejected)
	assert.Zero(t, next.calls, "the chain MUST NOT try another backend after a challenge")
}

// countingAuthenticator accepts every request and counts how often the chain
// reached it.
type countingAuthenticator struct{ calls int }

func (c *countingAuthenticator) Authenticate(aaa.AuthRequest) (aaa.AuthResult, error) {
	c.calls++
	return aaa.AuthResult{Authenticated: true, Profiles: []string{"admin"}, Source: "counting"}, nil
}

// TestRFC2865UnsupportedServiceTypeIsRejection drives the admin login against an
// Access-Accept carrying a Service-Type. The admin path asks for Login-User and
// implements nothing else, so any other value is unsupported.
func TestRFC2865UnsupportedServiceTypeIsRejection(t *testing.T) {
	key := []byte("testing123")
	filterID := Attr{Type: AttrFilterID, Value: []byte("operator")}

	login := func(t *testing.T, reply []Attr) (aaa.AuthResult, error) {
		t.Helper()
		srv := newReplyServer(t, key, CodeAccessAccept, reply)
		defer srv.close()
		a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
		return a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	}

	// RFC requirement: RFC2865-1.1-2 positive -- an Access-Accept whose
	// Service-Type is the Login-User value the Access-Request asked for names a
	// service this NAS implements, so the login is accepted.
	res, err := login(t, []Attr{filterID, {Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)}})
	require.NoError(t, err)
	assert.True(t, res.Authenticated, "Login-User is the service the admin path offers")

	// RFC requirement: RFC2865-1.1-2 negative -- an Access-Accept whose
	// Service-Type is Framed-User names a service the admin login path does not
	// implement, so it is treated as an Access-Reject.
	res, err = login(t, []Attr{filterID, {Type: AttrServiceType, Value: AttrUint32(ServiceTypeFramed)}})
	assert.False(t, res.Authenticated)
	require.ErrorIs(t, err, aaa.ErrAuthRejected,
		"an unsupported Service-Type MUST be treated as an Access-Reject")

	// RFC requirement: RFC2865-1.1-2 negative -- Service-Type 7 (NAS-Prompt-User)
	// authorizes a service this NAS cannot offer at all, so the Access-Accept is
	// treated as an Access-Reject rather than granting an unavailable service.
	res, err = login(t, []Attr{filterID, {Type: AttrServiceType, Value: AttrUint32(7)}})
	assert.False(t, res.Authenticated)
	require.ErrorIs(t, err, aaa.ErrAuthRejected,
		"an Access-Accept authorizing an unavailable service MUST be an Access-Reject")

	// RFC requirement: RFC2865-1.1-2 positive -- an Access-Accept carrying no
	// Service-Type authorizes the service the Access-Request named, which this
	// NAS does offer, so the login is accepted.
	res, err = login(t, []Attr{filterID})
	require.NoError(t, err)
	assert.True(t, res.Authenticated, "an Accept with no Service-Type authorizes what was asked")
}

// TestRFC2865ZeroLengthTextIsOmitted reads the Access-Request off the wire and
// requires that an empty User-Name produces no attribute at all.
func TestRFC2865ZeroLengthTextIsOmitted(t *testing.T) {
	key := []byte("testing123")
	srv := newCapturingServer(t, key, CodeAccessAccept,
		[]Attr{{Type: AttrFilterID, Value: []byte("operator")}}, true)

	a := testAuthenticator(t, srv.mock.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})

	// RFC requirement: RFC2865-5-8 positive -- a non-empty User-Name is text of
	// non-zero length, so the attribute is sent and carries the name.
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	named, err := Decode(srv.request(t))
	require.NoError(t, err)
	assert.Equal(t, "alice", string(named.FindAttr(AttrUserName)))

	// RFC requirement: RFC2865-5-8 negative -- an empty User-Name would be text of
	// length zero, so the entire attribute is omitted rather than sent empty
	// (Section 5, "omit the entire attribute instead").
	_, err = a.Authenticate(aaa.AuthRequest{Username: "", Password: "pw"})
	require.NoError(t, err)
	anonymous, err := Decode(srv.request(t))
	require.NoError(t, err)
	assert.Nil(t, anonymous.FindAttr(AttrUserName),
		"a zero-length User-Name MUST be omitted, not sent empty")
	assert.NotNil(t, anonymous.FindAttr(AttrUserPassword),
		"omitting User-Name MUST NOT drop the rest of the request")
}
