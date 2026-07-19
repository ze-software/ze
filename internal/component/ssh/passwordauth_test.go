package ssh

import (
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
)

// newHashUserServer returns a Server with a single user whose bcrypt hash is
// known, plus the raw hash string. The server has a logger and no audit
// recorder (recordAuthFailure is nil-safe), so authenticatePassword can be
// driven directly without a live SSH listener.
func newHashUserServer(t *testing.T) (*Server, authz.Authenticator, string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("realpass"), bcrypt.MinCost)
	require.NoError(t, err)
	srv := &Server{logger: slog.Default()}
	auth := &authz.LocalAuthenticator{
		Users: []authz.UserConfig{{Name: "admin", Hash: string(hash), Profiles: []string{"admin"}}},
	}
	return srv, auth, string(hash)
}

// VALIDATES: AC-1 — presenting the stored bcrypt hash as the password over a
// non-loopback SSH peer is rejected; the real plaintext still authenticates.
// PREVENTS: a leaked config backup being replayed as an SSH credential remotely.
func TestSSHPasswordCallbackRejectsHashFromRemotePeer(t *testing.T) {
	srv, auth, hash := newHashUserServer(t)
	remote := &net.TCPAddr{IP: net.IPv4(10, 0, 2, 2), Port: 40000}

	assert.False(t, srv.authenticatePassword(auth, "admin", hash, remote),
		"hash-as-token MUST be rejected from a remote peer")
	assert.True(t, srv.authenticatePassword(auth, "admin", "realpass", remote),
		"plaintext password MUST still authenticate from a remote peer")
}

// VALIDATES: AC-2 — the on-box CLI presenting the zefs hash over a loopback (or
// unix) peer still authenticates.
// PREVENTS: breaking the local operator CLI login.
func TestSSHPasswordCallbackAcceptsHashFromLoopback(t *testing.T) {
	srv, auth, hash := newHashUserServer(t)

	loopbackV4 := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}
	loopbackV6 := &net.TCPAddr{IP: net.IPv6loopback, Port: 2222}
	unix := &net.UnixAddr{Name: "/run/ze/ssh.sock", Net: "unix"}

	assert.True(t, srv.authenticatePassword(auth, "admin", hash, loopbackV4),
		"hash-as-token MUST be accepted from an IPv4 loopback peer")
	assert.True(t, srv.authenticatePassword(auth, "admin", hash, loopbackV6),
		"hash-as-token MUST be accepted from an IPv6 loopback peer")
	assert.True(t, srv.authenticatePassword(auth, "admin", hash, unix),
		"hash-as-token MUST be accepted from a unix-socket peer")
}

// VALIDATES: fail-closed transport classification boundary.
// PREVENTS: a non-loopback or unclassifiable peer being treated as local.
func TestIsLocalTransport(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{"ipv4 loopback", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}, true},
		{"ipv4 loopback 127.x.y.z", &net.TCPAddr{IP: net.IPv4(127, 5, 6, 7), Port: 2222}, true},
		{"ipv6 loopback", &net.TCPAddr{IP: net.IPv6loopback, Port: 2222}, true},
		{"unix socket", &net.UnixAddr{Name: "/run/ze.sock", Net: "unix"}, true},
		{"private lan", &net.TCPAddr{IP: net.IPv4(10, 0, 2, 2), Port: 40000}, false},
		{"public", &net.TCPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 40000}, false},
		{"unspecified 0.0.0.0", &net.TCPAddr{IP: net.IPv4zero, Port: 2222}, false},
		{"nil address", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLocalTransport(tt.addr))
		})
	}
}
