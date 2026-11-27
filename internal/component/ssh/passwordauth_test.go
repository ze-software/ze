package ssh

import (
	"context"
	"log/slog"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/loginservice"
	_ "github.com/ze-software/ze/internal/component/tacacs/yang"
	"github.com/ze-software/ze/internal/core/audit"
)

// newHashUserServer returns a Server with a single user whose bcrypt hash is
// known, plus the raw hash string. The server has a logger and no audit
// recorder (recordAuthFailure is nil-safe), so password authentication can be
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

func authenticateTestPassword(srv *Server, auth authz.Authenticator, pass string, peer net.Addr) bool {
	ok, _ := srv.authenticatePasswordResult(auth, "admin", pass, peer)
	return ok
}

// VALIDATES: AC-1 — presenting the stored bcrypt hash as the password over a
// non-loopback SSH peer is rejected; the real plaintext still authenticates.
// PREVENTS: a leaked config backup being replayed as an SSH credential remotely.
func TestSSHPasswordCallbackRejectsHashFromRemotePeer(t *testing.T) {
	srv, auth, hash := newHashUserServer(t)
	remote := &net.TCPAddr{IP: net.IPv4(10, 0, 2, 2), Port: 40000}

	assert.False(t, authenticateTestPassword(srv, auth, hash, remote),
		"hash-as-token MUST be rejected from a remote peer")
	assert.True(t, authenticateTestPassword(srv, auth, "realpass", remote),
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

	assert.True(t, authenticateTestPassword(srv, auth, hash, loopbackV4),
		"hash-as-token MUST be accepted from an IPv4 loopback peer")
	assert.True(t, authenticateTestPassword(srv, auth, hash, loopbackV6),
		"hash-as-token MUST be accepted from an IPv6 loopback peer")
	assert.True(t, authenticateTestPassword(srv, auth, hash, unix),
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

// SSH operational logs must mask a quoted key before truncating the command.
func TestSSHExecLogMasksQuotedTacacsSharedSecret(t *testing.T) {
	output := loggedCommand(`config set system authentication tacacs server 192.0.2.1 key "private first middle tail-value"`)
	assert.NotContains(t, output, "private")
	assert.NotContains(t, output, "middle")
	assert.NotContains(t, output, "tail-value")
	assert.Contains(t, output, "192.0.2.1 key <redacted>")
}

type recordingPasswordAuthenticator struct {
	request authz.AuthRequest
}

func (r *recordingPasswordAuthenticator) Authenticate(request authz.AuthRequest) (authz.AuthResult, error) {
	r.request = request
	return authz.AuthResult{Authenticated: true, Source: aaa.SourceLocal}, nil
}

func dialWithPassword(addr, username, password string) error {
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            username,
		Auth:            []gossh.AuthMethod{gossh.Password(password)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // the test server generates a fresh host key per run
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return err
	}
	return client.Close()
}

// VALIDATES: AC-19 -- when no AAA bundle is supplied, the SSH server wraps its
// fallback LocalAuthenticator with aaa.WithProfileAuthorizer so login-service
// restrictions still gate SSH password logins while local recovery accounts stay
// admitted.
// PREVENTS: a daemon with no composed AAA bundle silently bypassing
// login-service authorization on SSH password login.
func TestSSHLocalAuthenticatorWithoutBundleIsGated(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secretpass"), bcrypt.MinCost)
	require.NoError(t, err)

	const (
		setAPIOnly = "api-only"
		setSSHOk   = "ssh-ok"
	)
	aaa.SetAcceptedLoginSets(map[string][]string{
		setAPIOnly: {"rest"},
		setSSHOk:   {loginServiceSSH},
	})
	t.Cleanup(func() { aaa.SetAcceptedLoginSets(nil) })

	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)

	srv, err := NewServer(Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: t.TempDir() + "/test_host_key",
		Users: []authz.UserConfig{
			{Name: "excluded", Hash: string(hash), Profiles: []string{"admin"}, LoginSet: setAPIOnly},
			{Name: "allowed", Hash: string(hash), Profiles: []string{"admin"}, LoginSet: setSSHOk},
			{Name: "recovery", Hash: string(hash), Profiles: []string{aaa.ReservedRecoveryProfile}, LoginSet: setAPIOnly, UserGeneration: 1},
		},
		AuditRecorder: recorder,
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background(), nil, nil))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(stopCtx))
	})

	addr := srv.Address()
	require.Eventually(t, func() bool {
		return dialWithPassword(addr, "allowed", "secretpass") == nil
	}, 5*time.Second, 20*time.Millisecond, "allowed account must authenticate on SSH without an AAA bundle")

	assert.Error(t, dialWithPassword(addr, "excluded", "secretpass"),
		"AC-19: account whose login set excludes ssh must be refused on the fallback local authenticator")
	assert.NoError(t, dialWithPassword(addr, "recovery", "secretpass"),
		"AC-19: local recovery account must stay admitted even when every config set excludes ssh")

	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.NotEmpty(t, entries, "AC-19: login-service refusal must record an auth failure audit entry")
	assert.Equal(t, "excluded", entries[0].Actor)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)
}

// VALIDATES: AC-18 -- the registered SSH login-service name matches the exact
// wire value ("ssh") sent in AuthRequest.Service to TACACS+ port and RADIUS
// NAS-Port-Id.
// PREVENTS: a registry rename drifting away from the TACACS+/RADIUS wire value.
func TestRegisteredNamesMatchWireValues(t *testing.T) {
	assert.Equal(t, "ssh", loginServiceSSH)
	assert.Equal(t, componentSSH, loginServiceSSH)
	assert.True(t, loginservice.Registered(loginServiceSSH), "ssh must be registered in loginservice at init")
	assert.True(t, slices.Contains(loginservice.Names(), "ssh"), "loginservice.Names must include ssh")

	rec := &recordingPasswordAuthenticator{}
	srv := &Server{logger: slog.Default()}
	ok, _ := srv.authenticatePasswordResult(rec, "alice", "pass", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222})
	require.True(t, ok)
	assert.Equal(t, "ssh", rec.request.Service, "SSH password auth must send the registered ssh service name")
}
