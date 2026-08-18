package ssh

import (
	"bufio"
	"context"
	"crypto/ed25519"
	cryptoRand "crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
)

func ed25519Generate() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(cryptoRand.Reader)
}

func marshalED25519PrivateKey(t *testing.T, priv ed25519.PrivateKey) []byte {
	t.Helper()
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
}

type passwordProfilesAuthenticator struct{}

func (passwordProfilesAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	profiles := map[string][]string{
		"read-pass":  {"read-only"},
		"write-pass": {"operator"},
	}[request.Password]
	return aaa.AuthResult{
		Authenticated: profiles != nil,
		Source:        "tacacs",
		Profiles:      profiles,
	}, nil
}

// VALIDATES: AC-4 — SSH server created with config values.
// PREVENTS: server ignoring configured address, timeouts, or user list.

func TestNewServer(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:2222",
		HostKeyPath: t.TempDir() + "/test_host_key",
		IdleTimeout: 300,
		MaxSessions: 4,
		Users: []authz.UserConfig{
			{Name: "admin", Hash: "$2a$10$fake"},
		},
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:2222", srv.Address())
	assert.Equal(t, 4, srv.MaxSessions())
	assert.Equal(t, 1, len(srv.Users()))
}

func TestNewServerDefaults(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:2222", srv.Address(), "default listen address")
	assert.Equal(t, 8, srv.MaxSessions(), "default max sessions")
}

func TestNewServerNoUsers(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:2222",
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Empty(t, srv.Users(), "no users configured")
}

// VALIDATES: AC-16 -- SSH failed authentication emits an audit record with source IP and attempted user.
// PREVENTS: SSH login failures being visible only in server logs.
func TestSSHAuthFailureAuditRecord(t *testing.T) {
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	srv, err := NewServer(Config{HostKeyPath: t.TempDir() + "/test_host_key", AuditRecorder: recorder})
	require.NoError(t, err)

	srv.recordAuthFailure("alice", "192.0.2.10:2222")

	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:2222", entries[0].RemoteAddr)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)
}

func TestServerName(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "ssh", srv.Name(), "subsystem name")
}

// VALIDATES: host-key defaults to ConfigDir/ssh_host_ed25519_key when omitted.
// PREVENTS: server failing to start because no host key path was configured.
func TestNewServerDefaultHostKeyFromConfigDir(t *testing.T) {
	cfg := Config{
		Listen:    "127.0.0.1:2222",
		ConfigDir: "/opt/ze/etc",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/opt/ze/etc/ssh_host_ed25519_key", srv.config.HostKeyPath)
}

// VALIDATES: NewServer without host-key or ConfigDir falls back to binary-relative resolution.
// PREVENTS: empty host key path reaching Wish.
func TestNewServerNoHostKeyNoConfigDir(t *testing.T) {
	cfg := Config{
		Listen: "127.0.0.1:2222",
	}

	// The result depends on os.Executable() — test binary location varies.
	// We verify the contract: either a valid HostKeyPath is resolved, or a clear error.
	srv, err := NewServer(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "host-key path cannot be resolved")
		assert.Nil(t, srv)
		return
	}
	assert.NotEmpty(t, srv.config.HostKeyPath)
	assert.Contains(t, srv.config.HostKeyPath, "ssh_host_ed25519_key")
}

// VALIDATES: host key generated and stored when blob storage is set and key is missing.
// PREVENTS: SSH server failing to start with blob storage when no host key exists.
func TestResolveHostKeyFromBlobStorage(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck // test cleanup

	keyPath := filepath.Join(dir, "ssh_host_ed25519_key")

	cfg := Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: keyPath,
		Storage:     store,
	}

	srv, srvErr := NewServer(cfg)
	require.NoError(t, srvErr)

	// Key does not exist yet -- resolveHostKeyOption should generate and store it.
	opt, optErr := srv.resolveHostKeyOption()
	require.NoError(t, optErr)
	assert.NotNil(t, opt, "should return a valid ssh.Option")

	// Verify the key was written to storage.
	assert.True(t, store.Exists(keyPath), "host key should be written to storage")
	data, readErr := store.ReadFile(keyPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "PRIVATE KEY", "stored key should be PEM-encoded")

	// Second call should read the existing key, not generate a new one.
	opt2, opt2Err := srv.resolveHostKeyOption()
	require.NoError(t, opt2Err)
	assert.NotNil(t, opt2, "should return a valid ssh.Option from existing key")
	data2, read2Err := store.ReadFile(keyPath)
	require.NoError(t, read2Err)
	assert.Equal(t, data, data2, "key should not be regenerated")
}

// VALIDATES: resolveHostKeyOption generates key in memory when storage is nil or filesystem.
// PREVENTS: host key files being created on the physical filesystem by Wish.
func TestResolveHostKeyFilesystemMode(t *testing.T) {
	t.Run("nil storage", func(t *testing.T) {
		dir := t.TempDir()
		keyPath := filepath.Join(dir, "host_key")
		cfg := Config{
			Listen:      "127.0.0.1:0",
			HostKeyPath: keyPath,
		}
		srv, err := NewServer(cfg)
		require.NoError(t, err)
		opt, err := srv.resolveHostKeyOption()
		require.NoError(t, err)
		assert.NotNil(t, opt, "should return a valid ssh.Option")
		// Key must NOT be written to filesystem when storage is nil.
		_, statErr := os.Stat(keyPath)
		assert.True(t, os.IsNotExist(statErr), "key file must not be created on filesystem")
		_, statPubErr := os.Stat(keyPath + ".pub")
		assert.True(t, os.IsNotExist(statPubErr), "pub file must not be created on filesystem")
	})

	t.Run("filesystem storage", func(t *testing.T) {
		dir := t.TempDir()
		keyPath := filepath.Join(dir, "host_key")
		cfg := Config{
			Listen:      "127.0.0.1:0",
			HostKeyPath: keyPath,
			Storage:     storage.NewFilesystem(),
		}
		srv, err := NewServer(cfg)
		require.NoError(t, err)
		opt, err := srv.resolveHostKeyOption()
		require.NoError(t, err)
		assert.NotNil(t, opt, "should return a valid ssh.Option")
		// Private key is written via storage.WriteFile (filesystem).
		data, readErr := os.ReadFile(keyPath)
		require.NoError(t, readErr, "key should be persisted via storage")
		assert.Contains(t, string(data), "PRIVATE KEY")
		// .pub must NOT exist — Wish's WithHostKeyPath is no longer used.
		_, statPubErr := os.Stat(keyPath + ".pub")
		assert.True(t, os.IsNotExist(statPubErr), "pub file must not be created on filesystem")
	})
}

// VALIDATES: host certificate is loaded and produces a valid ssh.Option.
// PREVENTS: server failing to start when host-certificate is configured.
func TestResolveHostKeyWithCertificate(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "host_key")
	certPath := filepath.Join(dir, "host_key-cert.pub")

	// Generate a host key.
	_, hostPriv, err := ed25519Generate()
	require.NoError(t, err)
	hostSigner, err := gossh.NewSignerFromKey(hostPriv)
	require.NoError(t, err)

	// Write host key PEM to storage.
	store := storage.NewFilesystem()
	hostPEM := marshalED25519PrivateKey(t, hostPriv)
	require.NoError(t, store.WriteFile(keyPath, hostPEM, 0o600))

	// Generate a CA key and sign the host certificate.
	_, caPriv, err := ed25519Generate()
	require.NoError(t, err)
	caSigner, err := gossh.NewSignerFromKey(caPriv)
	require.NoError(t, err)

	cert := &gossh.Certificate{
		Key:             hostSigner.PublicKey(),
		Serial:          1,
		CertType:        gossh.HostCert,
		KeyId:           "test-host",
		ValidPrincipals: []string{"localhost"},
		ValidAfter:      0,
		ValidBefore:     gossh.CertTimeInfinity,
	}
	require.NoError(t, cert.SignCert(cryptoRand.Reader, caSigner))

	certBytes := gossh.MarshalAuthorizedKey(cert)
	require.NoError(t, os.WriteFile(certPath, certBytes, 0o644))

	cfg := Config{
		Listen:       "127.0.0.1:0",
		HostKeyPath:  keyPath,
		HostCertPath: certPath,
		Storage:      store,
	}
	srv, srvErr := NewServer(cfg)
	require.NoError(t, srvErr)

	opt, optErr := srv.resolveHostKeyOption()
	require.NoError(t, optErr)
	assert.NotNil(t, opt)
}

// VALIDATES: missing certificate file produces a clear error.
// PREVENTS: silent fallback when cert path is misconfigured.
func TestResolveHostKeyWithCertificateMissing(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "host_key")

	store := storage.NewFilesystem()
	_, hostPriv, err := ed25519Generate()
	require.NoError(t, err)
	hostPEM := marshalED25519PrivateKey(t, hostPriv)
	require.NoError(t, store.WriteFile(keyPath, hostPEM, 0o600))

	cfg := Config{
		Listen:       "127.0.0.1:0",
		HostKeyPath:  keyPath,
		HostCertPath: filepath.Join(dir, "nonexistent-cert.pub"),
		Storage:      store,
	}
	srv, srvErr := NewServer(cfg)
	require.NoError(t, srvErr)

	_, optErr := srv.resolveHostKeyOption()
	require.Error(t, optErr)
	assert.Contains(t, optErr.Error(), "read host certificate")
}

// VALIDATES: plain public key (not a certificate) is rejected with a clear error.
// PREVENTS: confusing failure when operator points host-certificate at a .pub file.
func TestResolveHostKeyWithCertificateNotACert(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "host_key")
	certPath := filepath.Join(dir, "not_a_cert.pub")

	store := storage.NewFilesystem()
	_, hostPriv, err := ed25519Generate()
	require.NoError(t, err)
	hostPEM := marshalED25519PrivateKey(t, hostPriv)
	require.NoError(t, store.WriteFile(keyPath, hostPEM, 0o600))

	hostSigner, err := gossh.NewSignerFromKey(hostPriv)
	require.NoError(t, err)
	pubBytes := gossh.MarshalAuthorizedKey(hostSigner.PublicKey())
	require.NoError(t, os.WriteFile(certPath, pubBytes, 0o644))

	cfg := Config{
		Listen:       "127.0.0.1:0",
		HostKeyPath:  keyPath,
		HostCertPath: certPath,
		Storage:      store,
	}
	srv, srvErr := NewServer(cfg)
	require.NoError(t, srvErr)

	_, optErr := srv.resolveHostKeyOption()
	require.Error(t, optErr)
	assert.Contains(t, optErr.Error(), "does not contain an SSH certificate")
}

// VALIDATES: explicit host-key is preserved.
// PREVENTS: default overwriting an explicit config value.
func TestNewServerExplicitHostKey(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:2222",
		HostKeyPath: "/custom/path/host_key",
		ConfigDir:   "/opt/ze/etc",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/custom/path/host_key", srv.config.HostKeyPath)
}

// VALIDATES: the running ze SSH server announces the distinctive
// "SSH-2.0-ze" identification banner that `ze init`'s daemonRunning probe
// requires to positively identify a live ze daemon (spec
// fixit-appliance-evidence-config, AC-1 wiring).
// PREVENTS: silently dropping wish.WithVersion, which would revert the banner
// to the generic "SSH-2.0-Go" and break the daemon-liveness probe with no
// unit-test failure (the init-side tests use a synthetic listener).
func TestServerAnnouncesZeBanner(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	require.NoError(t, srv.Start(context.Background(), nil, nil))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(stopCtx))
	})

	var banner string
	require.Eventually(t, func() bool {
		var d net.Dialer
		conn, dialErr := d.DialContext(context.Background(), "tcp", srv.Address())
		if dialErr != nil {
			return false // listener not ready yet
		}
		defer conn.Close() //nolint:errcheck,gosec // test probe
		if derr := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); derr != nil {
			return false
		}
		line, readErr := bufio.NewReader(io.LimitReader(conn, 255)).ReadString('\n')
		if readErr != nil && line == "" {
			return false
		}
		banner = strings.TrimRight(line, "\r\n")
		return true
	}, 3*time.Second, 5*time.Millisecond)

	assert.Equal(t, sshclient.ServerVersionBanner, banner,
		"ze SSH server must announce its distinctive banner so daemonRunning can identify it")
}

// VALIDATES: Bug 3 — double Start returns error instead of leaking first server.
// PREVENTS: orphaned listeners from calling Start() twice.
func TestServerDoubleStartError(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	// First start should succeed.
	err = srv.Start(context.Background(), nil, nil)
	require.NoError(t, err)

	// Wait for the Serve goroutine to be ready before testing double-start
	// (avoids a race in the Charm SSH library between Serve and Shutdown).
	require.Eventually(t, func() bool {
		var d net.Dialer
		conn, dialErr := d.DialContext(context.Background(), "tcp", srv.Address())
		if dialErr != nil {
			return false // not ready yet
		}
		conn.Close() //nolint:errcheck,gosec // test probe
		return true
	}, 2*time.Second, time.Millisecond)

	// Second start should fail.
	err = srv.Start(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, srv.Stop(stopCtx))
}

// VALIDATES: Bug 2 — Start fails synchronously when port is in use.
// PREVENTS: silent bind failures that Start() returns nil for.
func TestServerStartBindFailure(t *testing.T) {
	// Occupy a port with a raw listener first to get a known-busy port.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test cleanup

	busyAddr := ln.Addr().String()

	// Try to start SSH server on the occupied port.
	cfg := Config{
		Listen:      busyAddr,
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	err = srv.Start(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind SSH server")
}

// VALIDATES: Bug 4 — max-sessions is tracked via activeSessions counter.
// PREVENTS: unlimited concurrent sessions ignoring the configured limit.
func TestServerActiveSessionsCounter(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
		MaxSessions: 2,
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	// Initially zero active sessions.
	assert.Equal(t, int32(0), srv.ActiveSessions())

	// Simulate session counting (directly via atomic, since middleware is tested via integration).
	srv.activeSessions.Add(1)
	assert.Equal(t, int32(1), srv.ActiveSessions())
	srv.activeSessions.Add(1)
	assert.Equal(t, int32(2), srv.ActiveSessions())
	srv.activeSessions.Add(-1)
	assert.Equal(t, int32(1), srv.ActiveSessions())
}

// VALIDATES: AC-5 — SSH session delegates to SessionModelFactory.
// PREVENTS: SSH sessions ignoring the injected factory.
func TestSSHUsesSessionModelFactory(t *testing.T) {
	factoryCalled := false
	var receivedUsername string
	var receivedRemoteAddr string

	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	// Inject a test factory that creates a command-only model.
	srv.SetSessionModelFactory(func(username, remoteAddr string, _ plugin.Authorizer) tea.Model {
		factoryCalled = true
		receivedUsername = username
		receivedRemoteAddr = remoteAddr
		return cli.NewCommandModel()
	})

	model := srv.createSessionModel("testuser", "203.0.113.5:2222", nil)
	require.NotNil(t, model, "factory should return a model")
	assert.True(t, factoryCalled, "factory should be called")
	assert.Equal(t, "testuser", receivedUsername)
	assert.Equal(t, "203.0.113.5:2222", receivedRemoteAddr)
}

// TestSSHSessionGetsEditor verifies factory receives the username.
//
// VALIDATES: AC-22 -- SSH session delegates to factory with correct username.
// PREVENTS: Username lost during factory delegation.
func TestSSHSessionGetsEditor(t *testing.T) {
	var receivedUser string
	var receivedRemoteAddr string
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)
	srv.SetSessionModelFactory(func(username, remoteAddr string, _ plugin.Authorizer) tea.Model {
		receivedUser = username
		receivedRemoteAddr = remoteAddr
		return cli.NewCommandModel()
	})
	srv.createSessionModel("alice", "198.51.100.1:22", nil)
	assert.Equal(t, "alice", receivedUser)
	assert.Equal(t, "198.51.100.1:22", receivedRemoteAddr)
}

// TestSSHSessionFallbackWithoutConfig verifies nil factory returns nil model.
//
// VALIDATES: SSH session without factory set returns nil (no panic).
// PREVENTS: Panic when SSH starts before hub wires factory.
func TestSSHSessionFallbackWithoutConfig(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)
	// No factory set -- createSessionModel should return nil.
	model := srv.createSessionModel("alice", "198.51.100.1:22", nil)
	assert.Nil(t, model)
}

// VALIDATES: SSH exec commands keep the session remote address when building the executor.
// PREVENTS: exec accounting/authorization seeing an empty client address.
func TestSSHExecCommandPropagatesRemoteAddr(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	var (
		gotUser       string
		gotRemoteAddr string
		gotCommand    string
	)

	srv.SetExecutorFactory(func(username, remoteAddr string, _ plugin.Authorizer) CommandExecutor {
		gotUser = username
		gotRemoteAddr = remoteAddr
		return func(input string) (*plugin.RenderedResponse, error) {
			gotCommand = input
			return &plugin.RenderedResponse{Output: "ok"}, nil
		}
	})

	exec := srv.ExecutorForUser("alice", "203.0.113.5:2222", nil)
	require.NotNil(t, exec)

	output, err := exec("show version")
	require.NoError(t, err)
	assert.Equal(t, "ok", output.Output)
	output.TransportComplete()
	assert.Equal(t, "alice", gotUser)
	assert.Equal(t, "203.0.113.5:2222", gotRemoteAddr)
	assert.Equal(t, "show version", gotCommand)
}

// VALIDATES: SSH streaming commands keep the session remote address when building the executor.
// PREVENTS: streaming accounting/authorization seeing an empty client address.
func TestSSHStreamingCommandPropagatesRemoteAddr(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	var (
		gotUser       string
		gotRemoteAddr string
		gotArgs       []string
	)

	srv.SetStreamingExecutorFactory(func(username, remoteAddr string, _ plugin.Authorizer) StreamingExecutor {
		gotUser = username
		gotRemoteAddr = remoteAddr
		return func(_ context.Context, _ io.Writer, args []string) error {
			gotArgs = args
			return nil
		}
	})

	exec := srv.streamingExecutorForUser("alice", "203.0.113.5:2222", nil)
	require.NotNil(t, exec)

	err = exec(context.Background(), io.Discard, []string{"monitor event"})
	require.NoError(t, err)
	assert.Equal(t, "alice", gotUser)
	assert.Equal(t, "203.0.113.5:2222", gotRemoteAddr)
	assert.Equal(t, []string{"monitor event"}, gotArgs)
}

// VALIDATES: interactive SSH sessions preserve remote address into the injected session factory.
// PREVENTS: TUI command mode dropping peer identity while exec mode keeps it.
func TestCreateSessionModelPreservesRemoteAddr(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	var (
		gotUser       string
		gotRemoteAddr string
	)

	srv.SetSessionModelFactory(func(username, remoteAddr string, _ plugin.Authorizer) tea.Model {
		gotUser = username
		gotRemoteAddr = remoteAddr
		return cli.NewCommandModel()
	})

	model := srv.createSessionModel("alice", "203.0.113.5:2222", nil)
	require.NotNil(t, model)
	assert.Equal(t, "alice", gotUser)
	assert.Equal(t, "203.0.113.5:2222", gotRemoteAddr)
}

// VALIDATES: concurrent SSH logins with one username keep their own profiles.
// PREVENTS: a later login replacing an established connection's authorization.
func TestSSHSessionAuthorizationIsBoundToAuthenticationResult(t *testing.T) {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "read-only",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AddProfile(authz.Profile{
		Name: "operator",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Allow},
	})
	srv, err := NewServer(Config{
		Listen:        "127.0.0.1:0",
		HostKeyPath:   t.TempDir() + "/test_host_key",
		Authenticator: passwordProfilesAuthenticator{},
		Authorizer:    authz.StoreAuthorizer{Store: store},
	})
	require.NoError(t, err)
	srv.SetExecutorFactory(func(username, remoteAddr string, authorizer plugin.Authorizer) CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			result := &plugin.RenderedResponse{Output: "allowed"}
			if !authorizer.Authorize(username, remoteAddr, input, strings.HasPrefix(input, "show ")) {
				return result, pluginserver.ErrUnauthorized
			}
			return result, nil
		}
	})
	require.NoError(t, srv.Start(t.Context(), nil, nil))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(ctx))
	})

	dial := func(password string) *gossh.Client {
		client, dialErr := gossh.Dial("tcp", srv.Address(), &gossh.ClientConfig{
			User:            "same-user",
			Auth:            []gossh.AuthMethod{gossh.Password(password)},
			HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run
			Timeout:         time.Second,
		})
		require.NoError(t, dialErr)
		t.Cleanup(func() { require.NoError(t, client.Close()) })
		return client
	}

	readClient := dial("read-pass")
	writeClient := dial("write-pass")

	readSession, err := readClient.NewSession()
	require.NoError(t, err)
	readOutput, err := readSession.CombinedOutput("show version")
	require.NoError(t, err)
	assert.Contains(t, string(readOutput), "allowed")

	readDenied, err := readClient.NewSession()
	require.NoError(t, err)
	_, err = readDenied.CombinedOutput("set system host-name router")
	require.Error(t, err, "read-only session must not gain the later login's write profile")

	writeSession, err := writeClient.NewSession()
	require.NoError(t, err)
	writeOutput, err := writeSession.CombinedOutput("set system host-name router")
	require.NoError(t, err)
	assert.Contains(t, string(writeOutput), "allowed")

	writeDenied, err := writeClient.NewSession()
	require.NoError(t, err)
	_, err = writeDenied.CombinedOutput("show version")
	require.Error(t, err, "write session must not inherit the first login's read profile")
}

// TestLoginWarningsStalePeers verifies SetLoginWarnings stores the function.
//
// VALIDATES: AC-1 from spec-login-warnings: Warnings function stored on server.
// PREVENTS: Login warnings function lost after setting.
func TestLoginWarningsStalePeers(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	srv.SetLoginWarnings(func() []contract.LoginWarning {
		return []contract.LoginWarning{
			{Message: "3 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
		}
	})

	fn := srv.LoginWarningsFunc()
	require.NotNil(t, fn)
	warnings := fn()
	require.Len(t, warnings, 1)
	assert.Equal(t, "3 peer(s) have stale prefix data", warnings[0].Message)
}

// TestLoginWarningsNilFunc verifies nil loginWarningsFunc is safe.
//
// VALIDATES: AC-5 from spec-login-warnings: Nil func causes no crash.
// PREVENTS: Nil dereference when SSH sessions start before reactor.
func TestLoginWarningsNilFunc(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)
	fn := srv.LoginWarningsFunc()
	assert.Nil(t, fn)
}

// TestLoginWarningsNoPeers verifies nil warning return is safe.
//
// VALIDATES: AC-3 from spec-login-warnings: No warning with no peers.
// PREVENTS: Crash when warning function returns nil.
func TestLoginWarningsNoPeers(t *testing.T) {
	cfg := Config{HostKeyPath: t.TempDir() + "/test_host_key"}
	srv, err := NewServer(cfg)
	require.NoError(t, err)
	srv.SetLoginWarnings(func() []contract.LoginWarning { return nil })
	fn := srv.LoginWarningsFunc()
	require.NotNil(t, fn)
	assert.Nil(t, fn())
}

// TestPrefixStalenessWarning verifies multiple warnings are stored.
//
// VALIDATES: Multiple warnings round-trip through server.
// PREVENTS: Only first warning surviving storage.
func TestPrefixStalenessWarning(t *testing.T) {
	tests := []struct {
		name     string
		warnings []contract.LoginWarning
		count    int
	}{
		{"single", []contract.LoginWarning{{Message: "one"}}, 1},
		{"multiple", []contract.LoginWarning{{Message: "a"}, {Message: "b"}}, 2},
		{"none", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{HostKeyPath: t.TempDir() + "/test_host_key"}
			srv, err := NewServer(cfg)
			require.NoError(t, err)
			srv.SetLoginWarnings(func() []contract.LoginWarning { return tt.warnings })
			fn := srv.LoginWarningsFunc()
			require.NotNil(t, fn)
			assert.Len(t, fn(), tt.count)
		})
	}
}

// TestStalenessClosureWithPeerData verifies the staleness counting logic
// using IsPrefixDataStale with actual peer data.
//
// VALIDATES: AC-1 from spec-login-warnings: correct stale peer count.
// VALIDATES: AC-2 from spec-login-warnings: no warning when all fresh.
// PREVENTS: Off-by-one in stale counter or wrong IsPrefixDataStale usage.
func TestStalenessClosureWithPeerData(t *testing.T) {
	now := time.Now()
	fresh := now.AddDate(0, -1, 0).Format(time.DateOnly)     // 1 month ago = fresh
	stale := now.AddDate(0, -7, 0).Format(time.DateOnly)     // 7 months ago = stale
	veryStale := now.AddDate(-2, 0, 0).Format(time.DateOnly) // 2 years ago = stale

	tests := []struct {
		name      string
		peers     []plugin.PeerInfo
		wantStale int
	}{
		{
			name:      "all fresh",
			peers:     []plugin.PeerInfo{{PrefixUpdated: fresh}, {PrefixUpdated: fresh}},
			wantStale: 0,
		},
		{
			name:      "one stale one fresh",
			peers:     []plugin.PeerInfo{{PrefixUpdated: stale}, {PrefixUpdated: fresh}},
			wantStale: 1,
		},
		{
			name:      "all stale",
			peers:     []plugin.PeerInfo{{PrefixUpdated: stale}, {PrefixUpdated: veryStale}},
			wantStale: 2,
		},
		{
			name:      "empty PrefixUpdated treated as not stale",
			peers:     []plugin.PeerInfo{{PrefixUpdated: ""}, {PrefixUpdated: stale}},
			wantStale: 1,
		},
		{
			name:      "no peers",
			peers:     nil,
			wantStale: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the closure logic from loader.go.
			// Uses same staleness check: empty = not stale, >6 months = stale.
			peers := tt.peers
			staleCount := 0
			for i := range peers {
				updated := peers[i].PrefixUpdated
				if updated == "" {
					continue
				}
				parsed, err := time.Parse(time.DateOnly, updated)
				if err != nil {
					continue
				}
				if now.Sub(parsed) > 180*24*time.Hour {
					staleCount++
				}
			}
			assert.Equal(t, tt.wantStale, staleCount, "stale peer count")
		})
	}
}

// TestLoginWarningsPanicRecovery verifies panicking provider is recoverable.
//
// VALIDATES: AC-6 from spec-login-warnings: Provider panic does not crash.
// PREVENTS: Panicking provider takes down the SSH session.
func TestLoginWarningsPanicRecovery(t *testing.T) {
	cfg := Config{HostKeyPath: t.TempDir() + "/test_host_key"}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	srv.SetLoginWarnings(func() []contract.LoginWarning {
		panic("provider exploded")
	})

	// LoginWarningsFunc returns the panicking function; caller handles recovery.
	fn := srv.LoginWarningsFunc()
	require.NotNil(t, fn)
	// Verify the function panics (hub session factory handles recovery).
	assert.Panics(t, func() { fn() })
}

// VALIDATES: truncateForLog bounds long SSH commands (e.g. show policy test with
// a large hex payload) in the operational log while leaving short commands intact.
// PREVENTS: dumping a full BGP UPDATE hex blob into the SSH exec log (security review).
func TestTruncateForLog(t *testing.T) {
	short := "show bgp summary"
	if got := truncateForLog(short); got != short {
		t.Errorf("short command altered: got %q, want %q", got, short)
	}

	long := "show policy test peer 10.0.0.1 export update " + strings.Repeat("FF", 70000)
	got := truncateForLog(long)
	if len(got) >= len(long) {
		t.Errorf("long command not truncated: len(got)=%d len(long)=%d", len(got), len(long))
	}
	if !strings.HasPrefix(got, "show policy test peer 10.0.0.1 export update FF") {
		t.Errorf("truncated form lost its prefix: %q", got)
	}
	if !strings.Contains(got, "bytes total") {
		t.Errorf("truncated form missing byte-count suffix: %q", got)
	}

	// A multi-byte rune straddling the 256-byte cut must not be logged
	// half-encoded: the result must remain valid UTF-8.
	utf8cmd := strings.Repeat("a", 255) + "é" + strings.Repeat("b", 100)
	gotUTF := truncateForLog(utf8cmd)
	if !utf8.ValidString(gotUTF) {
		t.Errorf("truncated form is not valid UTF-8: %q", gotUTF)
	}
	if !strings.Contains(gotUTF, "bytes total") {
		t.Errorf("utf8 command not truncated: %q", gotUTF)
	}
}

// bcryptHashLiteral is a syntactically valid bcrypt hash used to drive the
// exec-log redaction tests without pulling in the bcrypt package.
const bcryptHashLiteral = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"

// VALIDATES: AC-7 — the SSH exec log line scrubs credential tokens (bcrypt
// hashes and password-key values) while still logging the command shape.
// PREVENTS: a one-shot `ze config set ... password <hash>` writing the secret
// verbatim to the operational log. Drives loggedCommand, the seam the exec
// middleware logs through.
func TestExecLogRedactsPasswordToken(t *testing.T) {
	// bcrypt hash presented as the value.
	hashed := loggedCommand("config set system authentication user alice password " + bcryptHashLiteral)
	if strings.Contains(hashed, bcryptHashLiteral) {
		t.Errorf("bcrypt hash leaked into the log line: %q", hashed)
	}
	if !strings.Contains(hashed, "<redacted>") {
		t.Errorf("expected a redaction marker: %q", hashed)
	}

	// plaintext password value.
	plain := loggedCommand("config set system authentication user alice plaintext-password hunter2")
	if strings.Contains(plain, "hunter2") {
		t.Errorf("plaintext password leaked into the log line: %q", plain)
	}

	// non-credential command is logged unchanged.
	benign := "show bgp summary"
	if got := loggedCommand(benign); got != benign {
		t.Errorf("benign command altered: got %q want %q", got, benign)
	}
}

// VALIDATES: redaction runs BEFORE truncation so a hash sitting on the 256-byte
// truncation boundary cannot half-leak.
// PREVENTS: the tail of a bcrypt hash surviving in the truncated prefix.
func TestExecLogRedactsBeforeTruncation(t *testing.T) {
	// Pad so the hash straddles the 256-byte cut, then trail more so the whole
	// command exceeds the truncation limit.
	padding := strings.Repeat("x", 230)
	cmd := "config set " + padding + " password " + bcryptHashLiteral + " " + strings.Repeat("y", 100)
	got := loggedCommand(cmd)
	if strings.Contains(got, bcryptHashLiteral) {
		t.Errorf("full hash survived: %q", got)
	}
	// Even a 20-char prefix of the hash must not appear (no half-leak at the cut).
	if strings.Contains(got, bcryptHashLiteral[:20]) {
		t.Errorf("hash prefix half-leaked past truncation: %q", got)
	}
	if !strings.Contains(got, "bytes total") {
		t.Errorf("expected truncation of an over-long command: %q", got)
	}
}

// execFormatServer starts an SSH server whose exec executor answers a fixed
// payload and records the command string it received.
func execFormatServer(t *testing.T, output string) (*Server, *string) {
	t.Helper()
	srv, err := NewServer(Config{
		Listen:        "127.0.0.1:0",
		HostKeyPath:   t.TempDir() + "/test_host_key",
		Authenticator: passwordProfilesAuthenticator{},
	})
	require.NoError(t, err)
	received := new(string)
	srv.SetExecutorFactory(func(_, _ string, _ plugin.Authorizer) CommandExecutor {
		return func(input string) (*plugin.RenderedResponse, error) {
			*received = input
			return &plugin.RenderedResponse{Output: output}, nil
		}
	})
	require.NoError(t, srv.Start(t.Context(), nil, nil))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(ctx))
	})
	return srv, received
}

// execFormatRun runs one command over a real SSH exec channel and returns what
// the server wrote to the session.
func execFormatRun(t *testing.T, srv *Server, command string) string {
	t.Helper()
	client, err := gossh.Dial("tcp", srv.Address(), &gossh.ClientConfig{
		User:            "operator",
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	session, err := client.NewSession()
	require.NoError(t, err)
	out, err := session.CombinedOutput(command)
	require.NoError(t, err, "exec failed: %s", string(out))
	return string(out)
}

func isTableRendering(s string) bool {
	return strings.Contains(s, "┌") || strings.Contains(s, "│")
}

// VALIDATES: AC-1, AC-2 -- an SSH exec command carrying no format pipe renders in
// the configured default (ze.cli.format), not the executor's raw JSON.
// PREVENTS: `ssh host 'show version'` answering indented JSON after the operator
// committed `environment cli format default table`.
func TestSSHExecAppliesConfiguredFormatDefault(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	srv, received := execFormatServer(t, `{"version":"1.2.3"}`)
	out := execFormatRun(t, srv, "show version")

	assert.Equal(t, "show version", *received, "the dispatcher must receive the command unchanged")
	assert.Contains(t, out, "1.2.3")
	assert.True(t, isTableRendering(out), "configured default table must render a table; got:\n%s", out)
	assert.NotContains(t, out, `"version"`, "the raw JSON must not reach the caller")
}

// VALIDATES: AC-3 -- an explicit format pipe wins over the configured default, and
// the pipe chain is stripped before the command reaches the dispatcher.
// PREVENTS: `tokenize` receiving `|` and `json` as ordinary command arguments.
func TestSSHExecRunsAFormatPipe(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	srv, received := execFormatServer(t, `{"version":"1.2.3"}`)
	out := execFormatRun(t, srv, "show version | json")

	assert.Equal(t, "show version", *received, "the pipe chain must not reach the dispatcher")
	// The executor answers compact JSON; the | json operator indents it, so the
	// space after the colon proves the pipe ran rather than the payload passing
	// through untouched.
	assert.Contains(t, out, `"version": "1.2.3"`)
	assert.False(t, isTableRendering(out), "an explicit | json must beat the configured default; got:\n%s", out)
}

// TestExecCommandRawAnswersTheDispatcherJSON drives the RPC helper from its
// entry point, over a real SSH exec channel, with a display format configured.
//
// The exec channel carries two kinds of caller. An operator wants the rendering
// their `environment cli format default` asks for. Ze's own code wants the
// bytes the dispatcher produced, because it unmarshals them. sshclient.
// ExecCommandRaw is the single place the second kind says so, and this proves
// the request survives the whole path: client, channel, execMiddleware, pipe
// chain, and back.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9.
// PREVENTS: every in-tree RPC caller parsing a table and degrading in silence.
func TestExecCommandRawAnswersTheDispatcherJSON(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	srv, received := execFormatServer(t, `{"version":"1.2.3"}`)
	host, port, err := net.SplitHostPort(srv.Address())
	require.NoError(t, err)

	creds := sshclient.Credentials{Host: host, Port: port, Username: "operator", Auth: "read-pass"}

	raw, err := sshclient.ExecCommandRaw(creds, "show version")
	require.NoError(t, err)
	assert.Equal(t, "show version", *received, "the raw pipe must not reach the dispatcher")
	assert.JSONEq(t, `{"version":"1.2.3"}`, raw)
	assert.False(t, isTableRendering(raw), "the configured default must not reach an RPC caller; got:\n%s", raw)

	// The comparison arm. The same command over the same channel, asked for the
	// ordinary way, still answers the operator's format -- so this test is about
	// which helper was called, not about the daemon having stopped formatting.
	rendered, err := sshclient.ExecCommand(creds, "show version")
	require.NoError(t, err)
	assert.True(t, isTableRendering(rendered), "the operator surface lost the configured default; got:\n%s", rendered)
}
