package radius

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
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

// VALIDATES: AC-6 the configured Class attribute is honored as the carrier.
// PREVENTS: hardcoding Filter-Id and ignoring the operator's profile-attribute.
func TestRadiusProfileMappingClass(t *testing.T) {
	key := []byte("testing123")
	reply := []Attr{{Type: attrClass, Value: []byte("admins")}}
	srv := newReplyServer(t, key, CodeAccessAccept, reply)
	defer srv.close()

	a := testAuthenticator(t, srv.addr, key, ExtractedConfig{ProfileAttr: attrClass})
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
	require.NoError(t, err)
	assert.Equal(t, []string{"admins"}, res.Profiles)
}
