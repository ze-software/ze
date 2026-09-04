package ipc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testServerCert returns a self-signed pair for a listener these tests dial
// with verification off. What signs the certificate is not what they check:
// the auth handshake, the token routing and the accept loop are. Chain
// validation against an issuer is tls_root_test.go.
func testServerCert(t *testing.T) tls.Certificate {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
	require.NoError(t, err)
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair
}

// selfSignedTLSConfig returns a TLS config with an auto-generated self-signed cert
// for testing. Both server and client configs are returned.
func selfSignedTLSConfig(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	cert := testServerCert(t)

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

	cert := testServerCert(t)

	addrs := []string{"127.0.0.1:0", "127.0.0.1:0"}
	listeners, listenErr := StartListeners(addrs, func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return &cert, nil
	})
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

// TestStartListenersRefusesWithNoCertificateSource: a listener with nothing to
// present binds and then fails every handshake with "no certificates
// configured", which reads to the operator as a client problem. Refuse at the
// bind instead, where the caller is named.
//
// MUTATION: drop the getCertificate nil check and this binds a listener that
// answers no handshake.
func TestStartListenersRefusesWithNoCertificateSource(t *testing.T) {
	t.Parallel()

	listeners, err := StartListeners([]string{"127.0.0.1:0"}, nil)
	if err == nil {
		for _, ln := range listeners {
			ln.Close() //nolint:errcheck,gosec // cleanup on a path the test says must not happen
		}
		t.Fatal("StartListeners bound a listener with no certificate source")
	}
	if len(listeners) != 0 {
		t.Fatalf("a refused StartListeners returned %d listeners", len(listeners))
	}
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

	acceptor := NewPluginAcceptor(ln, "test-secret-that-is-long-enough-32ch", nil)
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

	acceptor := NewPluginAcceptor(ln, "acceptor-secret-at-least-32-chars", nil)
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

	acceptor := NewPluginAcceptor(ln, "timeout-secret-at-least-32-chars", nil)
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

	acceptor := NewPluginAcceptor(ln, "stop-secret-at-least-32-chars-x", nil)
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
	case <-time.After(5 * time.Second):
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
		name, authErr := authenticateWithName(ctx, conn, "per-plugin-secret-at-least-32-ch", "bgp-rib")
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
		name, authErr := authenticateWithName(ctx, conn, "correct-token-at-least-32-chars!", "bgp-rib")
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
		name, authErr := authenticateWithName(ctx, conn, "per-plugin-secret-at-least-32-ch", "bgp-rib")
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

	acceptor := NewPluginAcceptor(ln, "shared-secret-at-least-32-chars!", nil)
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
