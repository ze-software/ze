package ipc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// selfSignedTLSConfig returns a TLS config with an auto-generated self-signed cert
// for testing. Both server and client configs are returned.
func selfSignedTLSConfig(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	server = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	client = &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test only
		MinVersion:         tls.VersionTLS12,
	}
	return server, client
}

// startTestListener starts a TLS listener on a random port and returns it.
func startTestListener(t *testing.T, tlsConf *tls.Config) net.Listener {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsConf)
	require.NoError(t, err)
	return ln
}

// authResult holds the outcome of an Authenticate call (for testing).
type authResult struct {
	Name string
	Conn net.Conn
	Err  error
}

// TestTLSAuthSuccess verifies that a plugin connecting with the correct token
// is accepted and returns the plugin name.
//
// VALIDATES: AC-3 -- correct token -> auth succeeds.
// PREVENTS: Valid plugins being rejected.
func TestTLSAuthSuccess(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server accepts and authenticates.
	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := Authenticate(ctx, conn, "test-secret-42")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	// Client connects and sends auth.
	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	require.NoError(t, SendAuth(ctx, conn, "test-secret-42", "bgp-rib"))

	result := <-resultCh
	require.NoError(t, result.Err)
	assert.Equal(t, "bgp-rib", result.Name)
	assert.NotNil(t, result.Conn)
}

// TestTLSAuthWrongToken verifies that a wrong token is rejected.
//
// VALIDATES: AC-4 -- wrong token -> auth fails.
// PREVENTS: Unauthorized plugins being accepted.
func TestTLSAuthWrongToken(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := Authenticate(ctx, conn, "correct-secret")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	require.NoError(t, SendAuth(ctx, conn, "wrong-secret", "evil-plugin"))

	result := <-resultCh
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "auth failed")
}

// TestTLSAuthTimeout verifies that connections without auth are closed after timeout.
//
// VALIDATES: AC-7 -- no auth RPC within timeout -> connection closed.
// PREVENTS: Unauthenticated connections lingering indefinitely.
func TestTLSAuthTimeout(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	// Use a short timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := Authenticate(ctx, conn, "secret")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	// Connect but never send auth.
	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	result := <-resultCh
	require.Error(t, result.Err, "should timeout without auth")
}

// TestTLSAuthMalformed verifies that a malformed auth RPC is rejected.
//
// VALIDATES: Malformed auth frame handled gracefully.
// PREVENTS: Panics on garbage input.
func TestTLSAuthMalformed(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := Authenticate(ctx, conn, "secret")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	// Send garbage instead of proper auth RPC.
	_, writeErr := conn.Write([]byte("not-an-rpc\n"))
	require.NoError(t, writeErr)

	result := <-resultCh
	require.Error(t, result.Err)
}

// TestTLSListenerMultiAddr verifies that multiple listeners can be started.
//
// VALIDATES: AC-2 -- multiple listen addresses each start a listener.
// PREVENTS: Only the first address being bound.
func TestTLSListenerMultiAddr(t *testing.T) {
	t.Parallel()

	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	addrs := []string{"127.0.0.1:0", "127.0.0.1:0"}
	listeners, listenErr := StartListeners(addrs, cert)
	require.NoError(t, listenErr)
	require.Len(t, listeners, 2)

	// Verify both are listening on different ports.
	addr1 := listeners[0].Addr().String()
	addr2 := listeners[1].Addr().String()
	assert.NotEqual(t, addr1, addr2)

	for _, ln := range listeners {
		require.NoError(t, ln.Close())
	}
}

// TestGenerateSelfSignedCert verifies cert generation produces a valid TLS certificate.
//
// VALIDATES: Self-signed cert is valid for TLS handshake.
// PREVENTS: TLS handshake failures from bad cert generation.
func TestGenerateSelfSignedCert(t *testing.T) {
	t.Parallel()

	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	// Verify cert can be used in a TLS config.
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	ln, listenErr := tls.Listen("tcp", "127.0.0.1:0", conf)
	require.NoError(t, listenErr)
	defer ln.Close() //nolint:errcheck // test cleanup

	// Verify a client can connect.
	clientConf := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test only
		MinVersion:         tls.VersionTLS12,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close() //nolint:errcheck // test cleanup
		// Complete TLS handshake before closing.
		if tlsConn, ok := conn.(*tls.Conn); ok {
			_ = tlsConn.HandshakeContext(ctx)
		}
	}()

	conn, dialErr := (&tls.Dialer{Config: clientConf}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, dialErr)
	require.NoError(t, conn.Close())
	<-doneCh
}

// TestSendAuthFormat verifies SendAuth uses the expected RPC framing.
//
// VALIDATES: Auth RPC uses #0 auth format.
// PREVENTS: Auth frame being unparseable by engine.
func TestSendAuthFormat(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer serverEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = SendAuth(ctx, clientEnd, "tok", "plug")
	}()

	c := rpc.NewConn(serverEnd, serverEnd)
	req, err := c.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), req.ID)
	assert.Equal(t, "auth", req.Method)

	var params struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(req.Params, &params))
	assert.Equal(t, "tok", params.Token)
	assert.Equal(t, "plug", params.Name)
}

// --- PluginAcceptor Tests ---

// TestPluginAcceptorStartStop verifies basic lifecycle.
//
// VALIDATES: Acceptor starts, stops cleanly, idempotent Stop.
// PREVENTS: Goroutine leaks on acceptor shutdown.
func TestPluginAcceptorStartStop(t *testing.T) {
	t.Parallel()

	serverTLS, _ := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)

	acceptor := NewPluginAcceptor(ln, "test-secret-that-is-long-enough-32ch", "")
	acceptor.Start()

	addr := acceptor.Addr()
	require.NotNil(t, addr)
	assert.NotEmpty(t, addr.String())

	// Stop should be safe to call multiple times.
	acceptor.Stop()
	acceptor.Stop()
}

// TestPluginAcceptorWaitForPlugin verifies end-to-end connect-back flow.
//
// VALIDATES: Plugin connects via TLS, authenticates, WaitForPlugin returns the connection.
// PREVENTS: Auth or routing failure in the acceptor pipeline.
func TestPluginAcceptorWaitForPlugin(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)

	acceptor := NewPluginAcceptor(ln, "acceptor-secret-at-least-32-chars", "")
	acceptor.Start()
	defer acceptor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Plugin connects and authenticates in background.
	go func() {
		conn, dialErr := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", acceptor.Addr().String())
		if dialErr != nil {
			return
		}
		if authErr := SendAuth(ctx, conn, "acceptor-secret-at-least-32-chars", "test-plugin"); authErr != nil {
			conn.Close() //nolint:errcheck // test cleanup
			return
		}
		// Read auth OK response.
		buf := make([]byte, 64)
		if _, readErr := conn.Read(buf); readErr != nil {
			conn.Close() //nolint:errcheck // test cleanup
		}
	}()

	// Engine waits for the plugin.
	conn, err := acceptor.WaitForPlugin(ctx, "test-plugin")
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}

// TestPluginAcceptorWaitTimeout verifies WaitForPlugin returns on context expiry.
//
// VALIDATES: WaitForPlugin respects context deadline.
// PREVENTS: Indefinite blocking when plugin never connects.
func TestPluginAcceptorWaitTimeout(t *testing.T) {
	t.Parallel()

	serverTLS, _ := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)

	acceptor := NewPluginAcceptor(ln, "timeout-secret-at-least-32-chars", "")
	acceptor.Start()
	defer acceptor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := acceptor.WaitForPlugin(ctx, "never-connects")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestPluginAcceptorWaitAfterStop verifies WaitForPlugin returns when acceptor stops.
//
// VALIDATES: WaitForPlugin unblocks on acceptor stop.
// PREVENTS: Goroutine hanging after server shutdown.
func TestPluginAcceptorWaitAfterStop(t *testing.T) {
	t.Parallel()

	serverTLS, _ := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)

	acceptor := NewPluginAcceptor(ln, "stop-secret-at-least-32-chars-x", "")
	acceptor.Start()

	errCh := make(chan error, 1)
	go func() {
		_, waitErr := acceptor.WaitForPlugin(context.Background(), "will-stop")
		errCh <- waitErr
	}()

	// Verify WaitForPlugin is blocked (not returning prematurely).
	require.Never(t, func() bool {
		select {
		case <-errCh:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 10*time.Millisecond, "WaitForPlugin should be blocked before Stop")
	acceptor.Stop()

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "acceptor stopped")
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForPlugin did not unblock after Stop")
	}
}

// TestAuthenticateWrongMethod verifies auth rejection for non-auth RPC method.
//
// VALIDATES: Non-auth method is rejected with clear error.
// PREVENTS: Arbitrary RPCs being accepted as auth.
func TestAuthenticateWrongMethod(t *testing.T) {
	t.Parallel()

	clientEnd, serverEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer serverEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		// Send a valid RPC but with wrong method name.
		line := rpc.FormatRequest(1, "not-auth", json.RawMessage(`{"token":"x","name":"y"}`))
		if _, writeErr := clientEnd.Write(append(line, '\n')); writeErr != nil {
			return
		}
		// Read the error response (net.Pipe blocks writes until reader is ready).
		buf := make([]byte, 256)
		if _, readErr := clientEnd.Read(buf); readErr != nil {
			return
		}
	}()

	_, err := Authenticate(ctx, serverEnd, "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected method auth")
}

// --- Per-Client Secret Tests ---

// TestPerClientSecretLookup verifies that a client with a per-client secret is accepted.
//
// VALIDATES: Per-client secret found by name (AC-10 positive case).
// PREVENTS: Per-client secrets being ignored in favor of shared secret only.
func TestPerClientSecretLookup(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientSecrets := map[string]string{
		"edge-01": "edge01-secret-that-is-at-least-32",
	}
	lookup := func(name string) (string, bool) {
		s, ok := clientSecrets[name]
		return s, ok
	}

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := AuthenticateWithLookup(ctx, conn, "shared-secret-at-least-32-chars!", lookup)
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	require.NoError(t, SendAuth(ctx, conn, "edge01-secret-that-is-at-least-32", "edge-01"))

	result := <-resultCh
	require.NoError(t, result.Err)
	assert.Equal(t, "edge-01", result.Name)
}

// TestPerClientSecretReject verifies that a wrong per-client token is rejected.
//
// VALIDATES: Wrong token for known name rejected (AC-10).
// PREVENTS: Client A's token authenticating as client B.
func TestPerClientSecretReject(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientSecrets := map[string]string{
		"edge-01": "edge01-secret-that-is-at-least-32",
	}
	lookup := func(name string) (string, bool) {
		s, ok := clientSecrets[name]
		return s, ok
	}

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := AuthenticateWithLookup(ctx, conn, "shared-secret-at-least-32-chars!", lookup)
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	require.NoError(t, SendAuth(ctx, conn, "wrong-secret-for-edge-01-client!", "edge-01"))

	result := <-resultCh
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "auth failed")
}

// TestPerClientSecretUnknownName verifies that unknown names fall back to shared secret.
//
// VALIDATES: Unknown client name falls back to shared secret (AC-11).
// PREVENTS: Plugin connections breaking when per-client lookup is enabled.
func TestPerClientSecretUnknownName(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Lookup returns false for unknown names -- falls back to shared secret.
	lookup := func(name string) (string, bool) {
		return "", false
	}

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := AuthenticateWithLookup(ctx, conn, "shared-secret-at-least-32-chars!", lookup)
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	// Plugin uses shared secret, not per-client.
	require.NoError(t, SendAuth(ctx, conn, "shared-secret-at-least-32-chars!", "bgp-rib"))

	result := <-resultCh
	require.NoError(t, result.Err)
	assert.Equal(t, "bgp-rib", result.Name)
}

// --- Per-Plugin Token and Name Binding Tests ---

// TestPerPluginTokenNameBinding verifies that the correct token with the wrong name
// is rejected when name binding is enforced.
//
// VALIDATES: AC-4 -- correct token + wrong name -> auth rejected.
// PREVENTS: A plugin impersonating another by sending a different name with a valid token.
func TestPerPluginTokenNameBinding(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		// Expect name "bgp-rib" but client will send "bgp-gr".
		name, authErr := AuthenticateWithName(ctx, conn, "per-plugin-secret-at-least-32-ch", "bgp-rib")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	// Send correct token but wrong name.
	require.NoError(t, SendAuth(ctx, conn, "per-plugin-secret-at-least-32-ch", "bgp-gr"))

	result := <-resultCh
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "name mismatch")
}

// TestPerPluginTokenWrongToken verifies that a wrong per-plugin token is rejected.
//
// VALIDATES: AC-5 -- another plugin's token -> auth rejected.
// PREVENTS: Cross-plugin token reuse.
func TestPerPluginTokenWrongToken(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := AuthenticateWithName(ctx, conn, "correct-token-at-least-32-chars!", "bgp-rib")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	// Send wrong token with correct name.
	require.NoError(t, SendAuth(ctx, conn, "wrong-token-at-least-32-chars!!!", "bgp-rib"))

	result := <-resultCh
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "invalid token")
}

// TestPerPluginTokenNameBindingSuccess verifies that the correct token with the correct
// name succeeds when name binding is enforced.
//
// VALIDATES: AC-3 -- correct per-plugin token + matching name -> auth succeeds.
// PREVENTS: False rejection of valid per-plugin auth.
func TestPerPluginTokenNameBindingSuccess(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan authResult, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			resultCh <- authResult{Err: acceptErr}
			return
		}
		name, authErr := AuthenticateWithName(ctx, conn, "per-plugin-secret-at-least-32-ch", "bgp-rib")
		resultCh <- authResult{Name: name, Conn: conn, Err: authErr}
	}()

	conn, err := (&tls.Dialer{Config: clientTLS}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // test cleanup

	require.NoError(t, SendAuth(ctx, conn, "per-plugin-secret-at-least-32-ch", "bgp-rib"))

	result := <-resultCh
	require.NoError(t, result.Err)
	assert.Equal(t, "bgp-rib", result.Name)
}

// TestTokenForPluginUniqueness verifies that TokenForPlugin generates different
// tokens for different plugin names.
//
// VALIDATES: AC-1 -- different plugins get different tokens.
// PREVENTS: All plugins sharing the same token.
func TestTokenForPluginUniqueness(t *testing.T) {
	t.Parallel()

	serverTLS, _ := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)

	acceptor := NewPluginAcceptor(ln, "shared-secret-at-least-32-chars!", "")
	defer acceptor.Stop()

	token1, err1 := acceptor.TokenForPlugin("bgp-rib")
	token2, err2 := acceptor.TokenForPlugin("bgp-gr")
	require.NoError(t, err1)
	require.NoError(t, err2)

	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2, "different plugins must get different tokens")

	// Same plugin name returns the same token.
	token1again, err3 := acceptor.TokenForPlugin("bgp-rib")
	require.NoError(t, err3)
	assert.Equal(t, token1, token1again, "same plugin must get same token")
}

// TestCertFingerprintComputation verifies that CertFingerprint returns a stable
// SHA-256 hex digest of the DER-encoded certificate.
//
// VALIDATES: Cert fingerprint is deterministic SHA-256 of DER bytes.
// PREVENTS: Wrong hash algorithm or encoding producing unstable fingerprints.
func TestCertFingerprintComputation(t *testing.T) {
	t.Parallel()

	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	fp1 := CertFingerprint(cert)
	fp2 := CertFingerprint(cert)

	assert.NotEmpty(t, fp1)
	assert.Equal(t, fp1, fp2, "fingerprint must be deterministic")
	assert.Len(t, fp1, 64, "SHA-256 hex is 64 characters")
}

// TestCertFingerprintVerification verifies that a TLS connection succeeds when the
// client verifies the server cert fingerprint.
//
// VALIDATES: AC-6 -- cert fingerprint matches -> TLS connects.
// PREVENTS: Valid fingerprints being rejected.
func TestCertFingerprintVerification(t *testing.T) {
	t.Parallel()

	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)
	fp := CertFingerprint(cert)

	serverConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	ln, listenErr := tls.Listen("tcp", "127.0.0.1:0", serverConf)
	require.NoError(t, listenErr)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doneCh := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			doneCh <- acceptErr
			return
		}
		defer conn.Close() //nolint:errcheck // test cleanup
		if tlsConn, ok := conn.(*tls.Conn); ok {
			doneCh <- tlsConn.HandshakeContext(ctx)
		}
	}()

	clientConf := TLSConfigWithFingerprint(fp)
	conn, dialErr := (&tls.Dialer{Config: clientConf}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, dialErr)
	require.NoError(t, conn.Close())

	require.NoError(t, <-doneCh)
}

// TestCertFingerprintMismatch verifies that a TLS connection fails when the
// fingerprint doesn't match.
//
// VALIDATES: AC-7 -- wrong fingerprint -> TLS handshake fails.
// PREVENTS: Accepting connections to a server with a different cert.
func TestCertFingerprintMismatch(t *testing.T) {
	t.Parallel()

	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	serverConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	ln, listenErr := tls.Listen("tcp", "127.0.0.1:0", serverConf)
	require.NoError(t, listenErr)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close() //nolint:errcheck // test cleanup
		if tlsConn, ok := conn.(*tls.Conn); ok {
			_ = tlsConn.HandshakeContext(ctx) // Will fail because client rejects
		}
	}()

	// Use a fake fingerprint that doesn't match.
	wrongFP := "deadbeef" + strings.Repeat("00", 28)
	clientConf := TLSConfigWithFingerprint(wrongFP)
	_, dialErr := (&tls.Dialer{Config: clientConf}).DialContext(ctx, "tcp", ln.Addr().String())
	require.Error(t, dialErr)
	assert.Contains(t, dialErr.Error(), "fingerprint mismatch")
}

// TestCertFingerprintFallback verifies that an empty fingerprint falls back
// to InsecureSkipVerify behavior.
//
// VALIDATES: AC-8 -- no fingerprint -> InsecureSkipVerify (backwards compat).
// PREVENTS: Breaking manual external plugins that don't have a fingerprint.
func TestCertFingerprintFallback(t *testing.T) {
	t.Parallel()

	cert, err := GenerateSelfSignedCert()
	require.NoError(t, err)

	serverConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	ln, listenErr := tls.Listen("tcp", "127.0.0.1:0", serverConf)
	require.NoError(t, listenErr)
	defer ln.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close() //nolint:errcheck // test cleanup
		if tlsConn, ok := conn.(*tls.Conn); ok {
			_ = tlsConn.HandshakeContext(ctx)
		}
	}()

	// Empty fingerprint should fall back to InsecureSkipVerify.
	clientConf := TLSConfigWithFingerprint("")
	conn, dialErr := (&tls.Dialer{Config: clientConf}).DialContext(ctx, "tcp", ln.Addr().String())
	require.NoError(t, dialErr)
	require.NoError(t, conn.Close())
}
