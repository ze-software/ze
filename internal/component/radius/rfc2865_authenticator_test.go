package radius

import (
	"bytes"
	"crypto/md5" //nolint:gosec // RFC 2865 Section 2.2 mandates MD5 for the CHAP response
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// newReplyServer answers every Access-Request with the given code and reply
// attributes, signing the response so the client's authenticator check passes.
func newReplyServer(t *testing.T, sharedKey []byte, code uint8, reply []Attr) *mockRADIUSServer {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	m := &mockRADIUSServer{conn: conn, addr: conn.LocalAddr().String(), done: make(chan struct{})}
	m.handler = func(req []byte) []byte { return buildReplyResponse(code, req, sharedKey, reply) }
	go m.serve()
	return m
}

func buildReplyResponse(code uint8, req, sharedKey []byte, attrs []Attr) []byte {
	if len(req) < MinPacketLen {
		return nil
	}
	body := make([]byte, 0, 64)
	for _, a := range attrs {
		body = append(body, a.Type, byte(2+len(a.Value)))
		body = append(body, a.Value...)
	}
	total := HeaderLen + len(body)
	resp := make([]byte, total)
	resp[0] = code
	resp[1] = req[1]
	binary.BigEndian.PutUint16(resp[2:4], uint16(total))
	copy(resp[HeaderLen:], body)
	var reqAuth [AuthenticatorLen]byte
	copy(reqAuth[:], req[4:4+AuthenticatorLen])
	auth := ResponseAuthenticator(code, req[1], uint16(total), reqAuth, body, sharedKey)
	copy(resp[4:4+AuthenticatorLen], auth[:])
	return resp
}

func testAuthenticator(t *testing.T, srvAddr string, sharedKey []byte, cfg ExtractedConfig) *radiusAuthenticator {
	t.Helper()
	client, err := NewClient(ClientConfig{
		Servers: []Server{{Address: srvAddr, SharedKey: sharedKey}},
		Timeout: 300 * time.Millisecond,
		Retries: 2,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	cfg.Servers = []Server{{Address: srvAddr, SharedKey: sharedKey}}
	return newRadiusAuthenticator(client, cfg, "ze-test", nil)
}

// VALIDATES: AC-3 Access-Accept -> AuthResult{Authenticated,Profiles,Source}.
// PREVENTS: accepted logins silently getting no profile or wrong source tag.
func TestRadiusAuthenticateAccept(t *testing.T) {
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessAccept, nil)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, DefaultProfiles: []string{"operator"},
	})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw", Service: "ssh"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, "radius", res.Source)
	assert.Equal(t, []string{"operator"}, res.Profiles, "no reply attr -> default profiles")
}

// VALIDATES: AC-4 Access-Reject -> ErrAuthRejected so the chain stops.
// PREVENTS: a rejected login falling through to another backend.
func TestRadiusAuthenticateReject(t *testing.T) {
	key := []byte("testing123")
	srv := newReplyServer(t, key, CodeAccessReject, nil)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "mallory", Password: "bad"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, aaa.ErrAuthRejected), "reject must be ErrAuthRejected")
	assert.False(t, res.Authenticated)
}

// VALIDATES: AC-5 timeout/unreachable -> infra error (not ErrAuthRejected).
// PREVENTS: an unreachable server locking operators out instead of falling
// through to the local backend.
func TestRadiusAuthenticateInfraError(t *testing.T) {
	// A bound-but-silent UDP socket: writes succeed, no reply ever comes.
	dead, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer func() { _ = dead.Close() }()

	a := testAuthenticator(t, dead.LocalAddr().String(), []byte("k"), ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, aaa.ErrAuthRejected), "infra failure must NOT be a rejection")
	assert.False(t, res.Authenticated)
}

// VALIDATES: AC-6 Filter-Id reply attributes map to profiles; multiple
// instances become multiple profiles.
// PREVENTS: dropping or mis-parsing server-assigned authorization profiles.
func TestRadiusProfileMapping(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{
		{Type: AttrFilterID, Value: []byte("netops")},
		{Type: AttrFilterID, Value: []byte("read-only")},
	}
	srv := newReplyServer(t, key, CodeAccessAccept, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, DefaultProfiles: []string{"fallback"},
	})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"netops", "read-only"}, res.Profiles)
}

// TestRadiusDropsReservedProfileName pins the fail-closed wire-ingress guard
// (spec-fixit-authz-admin-fallthrough review finding 1): a Filter-Id value is
// untrusted server input, so a reserved name in it (e.g. the break-glass recovery
// profile) must be DROPPED, never mapped to a profile. Otherwise a hostile or
// compromised RADIUS server sends Filter-Id = the reserved recovery name, which
// flows to LoginProfiles and authz.Store.Authorize grants allow-all admin.
//
// VALIDATES: a reserved Filter-Id is dropped; a mixed reply keeps only real names.
// PREVENTS: a RADIUS server spoofing the recovery/internal identity over the wire.
func TestRadiusDropsReservedProfileName(t *testing.T) {
	key := []byte("testing123")

	// Reserved-only reply: after dropping there are no profiles and no default, so
	// the login is rejected (never authenticated as recovery admin).
	srv := newReplyServer(t, key, CodeAccessAccept,
		[]Attr{{Type: AttrFilterID, Value: []byte(aaa.ReservedRecoveryProfile)}})
	defer srv.close()
	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	assert.ErrorIs(t, err, aaa.ErrAuthRejected, "reserved-only reply must be rejected after dropping")
	assert.False(t, res.Authenticated)
	assert.NotContains(t, res.Profiles, aaa.ReservedRecoveryProfile)

	// Mixed reply: the real profile survives, the reserved name is dropped.
	srv2 := newReplyServer(t, key, CodeAccessAccept, []Attr{
		{Type: AttrFilterID, Value: []byte("read-only")},
		{Type: AttrFilterID, Value: []byte(aaa.ReservedRecoveryProfile)},
	})
	defer srv2.close()
	a2 := testAuthenticator(t, srv2.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res2, err2 := a2.Authenticate(aaa.AuthRequest{Username: "bob", Password: "pw"})
	require.NoError(t, err2)
	assert.True(t, res2.Authenticated)
	assert.Equal(t, []string{"read-only"}, res2.Profiles)
	assert.NotContains(t, res2.Profiles, aaa.ReservedRecoveryProfile)
}

// VALIDATES: authBudget clamps to [minAuthBudget, maxAuthBudget] across the
// configured timeout/retries boundaries (R-5: login can never hang unbounded).
// PREVENTS: a zero budget cutting logins off instantly, or retries=10 producing
// a multi-minute hang.
func TestAuthBudgetBounds(t *testing.T) {
	// Tiny config clamps up to the floor.
	assert.Equal(t, minAuthBudget, authBudget(ExtractedConfig{Timeout: time.Second, Retries: 0}))
	// Pathological retries clamp down to the ceiling.
	assert.Equal(t, maxAuthBudget, authBudget(ExtractedConfig{
		Timeout: 60 * time.Second, Retries: 10,
		Servers: []Server{{}, {}},
	}))
	// A mid-range config lands strictly between the bounds.
	mid := authBudget(ExtractedConfig{Timeout: 3 * time.Second, Retries: 3, Servers: []Server{{}}})
	assert.Greater(t, mid, minAuthBudget)
	assert.Less(t, mid, maxAuthBudget)
}

// TestRadiusAuthenticateProfileResolutionShapes covers the shapes an
// Access-Accept's profile resolution can take, so the deny cases are read
// against the allow cases rather than in isolation. The no-attrs/no-default row
// is the regression: an Accept that names no profile must deny, not succeed with
// an empty set.
//
// VALIDATES: AC-1 -- an Access-Accept resolving to ZERO profile names denies,
// and an authenticated result never carries an empty profile set. AC-3 -- a
// configured default-profile still applies when the reply carries no attribute.
// PREVENTS: a RADIUS server that sends Access-Accept without a Filter-Id, against
//
//	out-of-the-box config with no default-profile, authenticating with zero
//	profiles. The result-scoped authorizer would fail closed, but this test
//	enforces the primary guard: authentication rejects the login before
//	authorization runs. Before that guard, the empty profile set granted admin
//	because GetSlice returns nil for an absent leaf-list.
func TestRadiusAuthenticateProfileResolutionShapes(t *testing.T) {
	filterID := func(vals ...string) []Attr {
		attrs := make([]Attr, 0, len(vals))
		for _, v := range vals {
			attrs = append(attrs, Attr{Type: AttrFilterID, Value: []byte(v)})
		}
		return attrs
	}

	tests := []struct {
		name         string
		reply        []Attr
		defaults     []string
		wantProfiles []string // non-nil => expect success with these profiles
	}{
		{"attrs present authenticate", filterID("netops"), nil, []string{"netops"}},
		{"attrs present beat configured default", filterID("netops"), []string{"fallback"}, []string{"netops"}},
		{"no attrs with default authenticates", nil, []string{"read-only"}, []string{"read-only"}},
		{"no attrs no default denies", nil, nil, nil},
		// Every default-profile member deactivated: Tree.GetSlice returns nil
		// (tree.go:183-185), reaching the same shape without an empty leaf-list.
		{"no attrs empty default denies", nil, []string{}, nil},
		// mapProfiles skips empty attribute values (authenticator.go:153), so an
		// Accept carrying Filter-Id = "" resolves to nothing just like no attr.
		{"empty attr values no default denies", filterID("", ""), nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := []byte("testing123")
			srv := newReplyServer(t, key, CodeAccessAccept, tt.reply)
			defer srv.close()

			a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
				ProfileAttr: AttrFilterID, DefaultProfiles: tt.defaults,
			})
			res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})

			if tt.wantProfiles != nil {
				require.NoError(t, err)
				assert.True(t, res.Authenticated)
				assert.Equal(t, tt.wantProfiles, res.Profiles)
				return
			}

			assert.ErrorIs(t, err, aaa.ErrAuthRejected,
				"an Access-Accept resolving to no profiles must be rejected")
			assert.False(t, res.Authenticated,
				"must not authenticate with an empty profile set")
			assert.Empty(t, res.Profiles)
			assert.Equal(t, "radius", res.Source)
		})
	}
}

// TestRadiusAuthenticatedImpliesProfiles states the invariant the fix protects,
// independent of any particular reply or config shape: this authenticator never
// reports success without naming at least one profile. authz treats "no profiles"
// as "no opinion" and falls back to admin, so success with an empty set is
// indistinguishable from an unrestricted login.
//
// VALIDATES: AC-2 -- Authenticated==true implies len(Profiles)>0.
// PREVENTS: a future profile source (a new profile-attribute carrier, a
//
//	deactivated default-profile member, a new caller of newRadiusAuthenticator)
//	reintroducing the empty-profile escalation through a path the table above
//	does not enumerate.
func TestRadiusAuthenticatedImpliesProfiles(t *testing.T) {
	shapes := []struct {
		reply    []Attr
		defaults []string
	}{
		{nil, nil},
		{nil, []string{}},
		{nil, []string{"read-only"}},
		{[]Attr{{Type: AttrFilterID, Value: []byte("")}}, nil},
		{[]Attr{{Type: AttrFilterID, Value: []byte("netops")}}, nil},
	}

	for _, shape := range shapes {
		key := []byte("testing123")
		srv := newReplyServer(t, key, CodeAccessAccept, shape.reply)

		a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
			ProfileAttr: AttrFilterID, DefaultProfiles: shape.defaults,
		})
		res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
		srv.close()

		if res.Authenticated {
			require.NoError(t, err)
			assert.NotEmpty(t, res.Profiles,
				"authenticated result must name at least one profile (reply=%v defaults=%v)",
				shape.reply, shape.defaults)
		}
	}
}

// VALIDATES: a Class attribute in an Access-Accept names no ze profile.
// PREVENTS: reading an authorization decision out of an attribute the RFC
// reserves as opaque accounting correlation.
// RFC requirement: RFC2865-5.25-1 positive -- an Access-Accept whose only
// candidate carrier is Class (25) resolves to no profile, so the login is
// rejected rather than authorized from a locally interpreted Class value.
func TestRadiusClassIsNotInterpretedLocally(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{{Type: 25, Value: []byte("admins")}}
	srv := newReplyServer(t, key, CodeAccessAccept, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.ErrorIs(t, err, aaa.ErrAuthRejected)
	assert.Empty(t, res.Profiles)
}

// fixedRandom supplies a deterministic CHAP seed: one identifier octet then a
// filled challenge.
func fixedRandom(identifier, challengeFill byte) io.Reader {
	seed := make([]byte, chapSeedLen)
	seed[0] = identifier
	for i := 1; i < len(seed); i++ {
		seed[i] = challengeFill
	}
	return bytes.NewReader(seed)
}

// verifyCHAPAsServer is the verification RFC 2865 Section 2.2 gives the SERVER,
// written here from the RFC text rather than by calling the producer: it looks
// up the password, hashes the CHAP ID octet, that password and the CHAP
// challenge, and compares the result to the CHAP-Password.
func verifyCHAPAsServer(t *testing.T, req *Packet, password string) {
	t.Helper()
	chapPassword := req.FindAttr(AttrCHAPPassword)
	require.Len(t, chapPassword, 17, "RFC 2865 Section 5.3: CHAP Ident plus a 16-octet String")
	challenge := req.FindAttr(AttrCHAPChallenge)
	require.Len(t, challenge, 16)

	h := md5.New() //nolint:gosec // RFC 2865 Section 2.2 mandates MD5
	h.Write(chapPassword[:1])
	h.Write([]byte(password))
	h.Write(challenge)
	assert.Equal(t, hex.EncodeToString(h.Sum(nil)), hex.EncodeToString(chapPassword[1:]),
		"a server verifying per RFC 2865 Section 2.2 must reproduce the CHAP-Password")
}

// TestRadiusAdminPapIsTheDefault pins the credential an operator who never
// touched auth-method keeps getting.
//
// VALIDATES: AC-1 and AC-2 -- an absent auth-method and an explicit `pap` both
// send User-Password, and neither sends a CHAP attribute.
// PREVENTS: the CHAP branch changing the shipped default, which would break
// every deployment whose RADIUS server stores password hashes.
func TestRadiusAdminPapIsTheDefault(t *testing.T) {
	// The zero value of AuthMethod is PAP, which is what an absent leaf leaves
	// in ExtractedConfig; AuthMethodPAP is what an explicit `pap` parses to.
	cases := []struct {
		name   string
		method AuthMethod
	}{
		{"auth-method absent", ExtractedConfig{}.AuthMethod},
		{"auth-method pap", AuthMethodPAP},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			key := []byte("testing123")
			srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("admin")}})
			defer srv.close()

			a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: AttrFilterID, AuthMethod: method})
			res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
			require.NoError(t, err)
			assert.True(t, res.Authenticated)

			req := srv.captured(t)
			assert.NotNil(t, req.FindAttr(AttrUserPassword), "PAP carries User-Password")
			assert.Nil(t, req.FindAttr(AttrCHAPPassword))
			assert.Nil(t, req.FindAttr(AttrCHAPChallenge))
		})
	}
}

// TestRadiusAdminChapAttributes asserts what a CHAP Access-Request carries and,
// as importantly, what it does not.
//
// VALIDATES: AC-3 -- CHAP-Password (Type 3, Length 19) and CHAP-Challenge
// (Type 60, 16 octets) are on the request, and the digest verifies the way a
// server verifies it. AC-4 -- the request carries NO User-Password.
// PREVENTS: a builder that APPENDS the CHAP credential to the PAP one. RFC 2865
// Section 4.1: "An Access-Request MUST NOT contain both a User-Password and a
// CHAP-Password." Both attributes present would still authenticate against a
// permissive server, so only the absence assertion catches it.
//
// TestAdminAccessRequestCarriesExactlyOneCredential (rfc2865_walk_test.go)
// carries the same requirement on the PAP arm. This is the CHAP arm, which no
// test could reach before auth-method existed.
//
// RFC requirement: RFC2865-4.1-4 positive -- an Access-Request built for
// auth-method chap carries a CHAP-Password and no User-Password, so the two
// credentials are never both present.
func TestRadiusAdminChapAttributes(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, AuthMethod: AuthMethodCHAP,
	})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)

	req := srv.captured(t)
	assert.Nil(t, req.FindAttr(AttrUserPassword),
		"RFC 2865 Section 4.1 forbids User-Password beside CHAP-Password")
	require.Len(t, req.FindAttr(AttrCHAPPassword), 17)
	require.Len(t, req.FindAttr(AttrCHAPChallenge), 16)
	verifyCHAPAsServer(t, req, "Hello")

	// The rest of the request is what the PAP path already sends.
	assert.Equal(t, []byte("alice"), req.FindAttr(AttrUserName))
	assert.Equal(t, AttrUint32(serviceTypeLogin), req.FindAttr(AttrServiceType))
	assert.Equal(t, []byte("ze-test"), req.FindAttr(AttrNASIdentifier))
}

// TestRadiusAdminChapChallengeIsFreshPerLogin proves the challenge is drawn per
// login rather than once per authenticator.
//
// VALIDATES: AC-6 -- two successive logins carry a different challenge and a
// different identifier.
// PREVENTS: a challenge cached in the authenticator, which makes every login
// produce the same CHAP-Password and turns a captured request into a replayable
// credential.
func TestRadiusAdminChapChallengeIsFreshPerLogin(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, AuthMethod: AuthMethodCHAP,
	})
	_, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	first := srv.captured(t)
	_, err = a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	second := srv.captured(t)

	firstPassword, secondPassword := first.FindAttr(AttrCHAPPassword), second.FindAttr(AttrCHAPPassword)
	// Required before the identifier is read: a missing attribute is a failure to
	// report, and indexing it instead would kill the test binary and take every
	// later test in the package with it.
	require.Len(t, firstPassword, 17)
	require.Len(t, secondPassword, 17)

	assert.NotEqual(t, first.FindAttr(AttrCHAPChallenge), second.FindAttr(AttrCHAPChallenge),
		"each login draws its own challenge")
	assert.NotEqual(t, firstPassword[0], secondPassword[0],
		"each login draws its own CHAP Identifier")
}

// TestRadiusAdminChapProfileMapping checks that selecting CHAP changes the
// credential and nothing else.
//
// VALIDATES: AC-10 -- an Access-Accept carrying Filter-Id maps to profiles
// exactly as it does on the PAP path, and the session is tagged source=radius.
// PREVENTS: the credential branch being wired somewhere that also changes the
// reply handling.
func TestRadiusAdminChapProfileMapping(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{
		{Type: AttrFilterID, Value: []byte("netops")},
		{Type: AttrFilterID, Value: []byte("read-only")},
	}
	srv := newRequestCaptureServer(t, key, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, AuthMethod: AuthMethodCHAP, DefaultProfiles: []string{"fallback"},
	})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"netops", "read-only"}, res.Profiles)
	assert.Equal(t, "radius", res.Source)
}

// TestRadiusAdminUnknownAuthMethodSendsNothing states the guard on the switch
// that selects the credential. A method neither constant names cannot be built
// by ExtractConfig, so this covers the one remaining producer: a future caller
// of newRadiusAuthenticator that sets the field itself.
//
// VALIDATES: an unknown method aborts the login and sends no packet.
// PREVENTS: an Access-Request with no credential attribute at all, which RFC
// 2865 Section 4.1 forbids and a server answers with a reject that reads to the
// operator like a wrong password.
func TestRadiusAdminUnknownAuthMethodSendsNothing(t *testing.T) {
	key := []byte("testing123")
	srv := newRequestCaptureServer(t, key, nil)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
		ProfileAttr: AttrFilterID, AuthMethod: AuthMethod(9),
	})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.Error(t, err)
	assert.False(t, res.Authenticated)
	srv.noRequest(t)
}
