// Design: docs/architecture/api/process-protocol.md — TLS transport for external plugins
// Related: rpc.go — PluginConn typed RPC wrapper
// Related: socketpair.go — package marker for plugin IPC
// Related: ../process/process.go — startExternal calls WaitForPlugin after forking
// Related: ../../../../pkg/plugin/sdk/sdk.go — NewFromTLSEnv dials TLS and calls SendAuth

package ipc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// setTCPNoDelay disables Nagle's algorithm on a connection if it wraps a TCP socket.
// Plugin IPC uses small request-response messages where Nagle adds latency
// without batching benefit.
func setTCPNoDelay(conn net.Conn) {
	type tcpConner interface{ NetConn() net.Conn }
	c := conn
	// Unwrap TLS to get the underlying TCP connection.
	if tc, ok := c.(tcpConner); ok {
		c = tc.NetConn()
	}
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
}

// maxAuthFrameSize is the maximum size of an auth RPC frame (4 KB).
const maxAuthFrameSize = 4096

// authMethod is the RPC method name for the auth handshake.
const authMethod = "auth"

// validPluginName matches alphanumeric names with hyphens, max 64 chars.
var validPluginName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

// authParams is the JSON payload for the #0 auth RPC.
type authParams struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// Authenticate reads the first RPC from conn and validates the auth token.
// Returns the plugin name on success. The context deadline controls the
// auth timeout -- unauthenticated connections are closed when ctx expires.
// On failure, the connection is closed and an error is returned.
//
// Uses byte-by-byte reading to avoid buffering ahead into the connection.
// This ensures the underlying net.Conn is clean for the caller to wrap
// in rpc.Conn + MuxConn without data loss from scanner buffering.
func Authenticate(ctx context.Context, conn net.Conn, expectedToken string) (string, error) {
	// Set read deadline from context to enforce auth timeout.
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
			return "", fmt.Errorf("set auth deadline: %w", err)
		}
	}

	// Read auth frame byte-by-byte to avoid buffering ahead.
	// No bufio.Scanner -- prevents stealing data from the production reader.
	line, err := readLineRaw(conn, maxAuthFrameSize)
	if err != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("read auth request: %w", err)
	}

	// Clear deadline after successful read.
	if clearErr := conn.SetReadDeadline(time.Time{}); clearErr != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("clear auth deadline: %w", clearErr)
	}

	id, verb, payload, parseErr := rpc.ParseLine(line)
	if parseErr != nil {
		writeErrorRaw(conn, 0, "malformed auth request")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: %w", parseErr)
	}

	if verb != authMethod {
		writeErrorRaw(conn, id, "expected auth")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: expected method auth, got %q", verb)
	}

	var params authParams
	if err := json.Unmarshal(payload, &params); err != nil {
		writeErrorRaw(conn, id, "malformed auth params")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: malformed params: %w", err)
	}

	// Constant-time comparison to prevent timing side-channel attacks.
	if subtle.ConstantTimeCompare([]byte(params.Token), []byte(expectedToken)) != 1 {
		writeErrorRaw(conn, id, "auth failed")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: invalid token")
	}

	if !validPluginName.MatchString(params.Name) {
		writeErrorRaw(conn, id, "invalid plugin name")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: invalid plugin name %q", params.Name)
	}

	// Send OK response directly (no rpc.Conn to avoid reader goroutine).
	if _, writeErr := conn.Write(append(rpc.FormatOK(id), '\n')); writeErr != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("send auth ok: %w", writeErr)
	}

	return params.Name, nil
}

// AuthenticateWithLookup reads the first RPC from conn and validates the auth token
// using a per-name secret lookup. The lookup function returns the expected secret
// for the given name, or false if the name is unknown. This supports per-client
// secrets where each managed client has its own token.
//
// Falls back to sharedSecret if lookup returns false (plugin connections use shared secret).
func AuthenticateWithLookup(ctx context.Context, conn net.Conn, sharedSecret string, lookup func(name string) (string, bool)) (string, error) {
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
			return "", fmt.Errorf("set auth deadline: %w", err)
		}
	}

	line, err := readLineRaw(conn, maxAuthFrameSize)
	if err != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("read auth request: %w", err)
	}

	if clearErr := conn.SetReadDeadline(time.Time{}); clearErr != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("clear auth deadline: %w", clearErr)
	}

	id, verb, payload, parseErr := rpc.ParseLine(line)
	if parseErr != nil {
		writeErrorRaw(conn, 0, "malformed auth request")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: %w", parseErr)
	}

	if verb != authMethod {
		writeErrorRaw(conn, id, "expected auth")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: expected method auth, got %q", verb)
	}

	var params authParams
	if err := json.Unmarshal(payload, &params); err != nil {
		writeErrorRaw(conn, id, "malformed auth params")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: malformed params: %w", err)
	}

	if !validPluginName.MatchString(params.Name) {
		writeErrorRaw(conn, id, "invalid plugin name")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: invalid plugin name %q", params.Name)
	}

	// Try per-client secret first, fall back to shared secret.
	expectedToken := sharedSecret
	if lookup != nil {
		if clientSecret, found := lookup(params.Name); found {
			expectedToken = clientSecret
		}
	}

	if subtle.ConstantTimeCompare([]byte(params.Token), []byte(expectedToken)) != 1 {
		writeErrorRaw(conn, id, "auth failed")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: invalid token")
	}

	if _, writeErr := conn.Write(append(rpc.FormatOK(id), '\n')); writeErr != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("send auth ok: %w", writeErr)
	}

	return params.Name, nil
}

// readLineRaw reads bytes one at a time until newline or maxSize.
// Avoids bufio.Scanner to prevent buffering ahead into the connection.
func readLineRaw(conn net.Conn, maxSize int) ([]byte, error) {
	buf := make([]byte, 0, 256)
	b := make([]byte, 1)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if b[0] == '\n' {
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) >= maxSize {
			return nil, fmt.Errorf("auth frame exceeds %d bytes", maxSize)
		}
	}
}

// writeErrorRaw writes an error response directly to conn without creating rpc.Conn.
func writeErrorRaw(conn net.Conn, id uint64, message string) {
	payload := rpc.NewErrorPayload("error", message)
	line := rpc.FormatError(id, payload)
	_, _ = conn.Write(append(line, '\n')) //nolint:errcheck // best-effort error response
}

// SendAuth sends the auth RPC to the engine as #0 auth {"token":"...","name":"..."}.
// Writes directly to conn without creating rpc.Conn (avoids reader goroutine leak).
func SendAuth(_ context.Context, conn net.Conn, token, name string) error {
	params := authParams{Token: token, Name: name}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal auth params: %w", err)
	}

	line := rpc.FormatRequest(0, authMethod, paramsJSON)
	if _, writeErr := conn.Write(append(line, '\n')); writeErr != nil {
		return fmt.Errorf("send auth: %w", writeErr)
	}
	return nil
}

// AuthenticateWithName reads the first RPC from conn and validates that both
// the auth token and the plugin name match the expected values. This enforces
// name binding: a plugin cannot use its token to impersonate another plugin.
// On failure, the connection is closed and an error is returned.
func AuthenticateWithName(ctx context.Context, conn net.Conn, expectedToken, expectedName string) (string, error) {
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
			return "", fmt.Errorf("set auth deadline: %w", err)
		}
	}

	line, err := readLineRaw(conn, maxAuthFrameSize)
	if err != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("read auth request: %w", err)
	}

	if clearErr := conn.SetReadDeadline(time.Time{}); clearErr != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("clear auth deadline: %w", clearErr)
	}

	id, verb, payload, parseErr := rpc.ParseLine(line)
	if parseErr != nil {
		writeErrorRaw(conn, 0, "malformed auth request")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: %w", parseErr)
	}

	if verb != authMethod {
		writeErrorRaw(conn, id, "expected auth")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: expected method auth, got %q", verb)
	}

	var params authParams
	if err := json.Unmarshal(payload, &params); err != nil {
		writeErrorRaw(conn, id, "malformed auth params")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: malformed params: %w", err)
	}

	if !validPluginName.MatchString(params.Name) {
		writeErrorRaw(conn, id, "invalid plugin name")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: invalid plugin name %q", params.Name)
	}

	// Constant-time token comparison first (prevents timing side-channel).
	if subtle.ConstantTimeCompare([]byte(params.Token), []byte(expectedToken)) != 1 {
		writeErrorRaw(conn, id, "auth failed")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: invalid token")
	}

	// Name binding: the presented name must match what the engine expects for this token.
	if params.Name != expectedName {
		writeErrorRaw(conn, id, "auth failed")
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("auth failed: name mismatch (expected %q)", expectedName)
	}

	if _, writeErr := conn.Write(append(rpc.FormatOK(id), '\n')); writeErr != nil {
		conn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return "", fmt.Errorf("send auth ok: %w", writeErr)
	}

	return params.Name, nil
}

// CertFingerprint returns the hex-encoded SHA-256 fingerprint of a TLS certificate's
// DER-encoded bytes. Used to pass the server cert identity to plugins for pinning.
func CertFingerprint(cert tls.Certificate) string {
	if len(cert.Certificate) == 0 {
		return ""
	}
	sum := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(sum[:])
}

// TLSConfigWithFingerprint returns a TLS client config that verifies the server
// certificate matches the given SHA-256 fingerprint. If fingerprint is empty,
// uses InsecureSkipVerify (useful during development or when fingerprint is unavailable).
func TLSConfigWithFingerprint(fingerprint string) *tls.Config {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // no fingerprint available
			MinVersion:         tls.VersionTLS13,
		}
	}

	return &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // cert verified via VerifyConnection fingerprint check
		MinVersion:         tls.VersionTLS13,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("server presented no certificates")
			}
			actual := sha256.Sum256(cs.PeerCertificates[0].Raw)
			actualHex := hex.EncodeToString(actual[:])
			if subtle.ConstantTimeCompare([]byte(actualHex), []byte(fingerprint)) != 1 {
				return fmt.Errorf("certificate fingerprint mismatch")
			}
			return nil
		},
	}
}

// GenerateSelfSignedCert creates an ephemeral self-signed TLS certificate.
// Used when no user-provided certificate is configured.
func GenerateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour), // Short-lived ephemeral cert.
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

// StartListeners creates TLS listeners on each of the given addresses.
// Returns all listeners on success, or an error if any address fails to bind.
// Returns an error if addrs is empty.
// On error, all successfully created listeners are closed before returning.
func StartListeners(addrs []string, cert tls.Certificate) ([]net.Listener, error) {
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no listen addresses configured")
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}

	var listeners []net.Listener
	for _, addr := range addrs {
		ln, err := tls.Listen("tcp", addr, tlsConf)
		if err != nil {
			// Clean up already-started listeners.
			for _, prev := range listeners {
				prev.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
			}
			return nil, fmt.Errorf("listen %s: %w", addr, err)
		}
		listeners = append(listeners, ln)
	}

	return listeners, nil
}

// maxPendingConns limits concurrent unauthenticated connections.
const maxPendingConns = 32

// PluginAcceptor manages a TLS listener and routes authenticated connections
// to waiting plugin processes by name. The server creates one acceptor from
// the hub config and shares it with all external processes.
type PluginAcceptor struct {
	listener     net.Listener
	secret       string
	secretLookup func(name string) (string, bool) // Per-client secret lookup (nil = shared secret only)
	pluginTokens map[string]string                // Per-plugin tokens: name -> random token
	tokenMu      sync.Mutex                       // Protects pluginTokens
	certFP       string                           // Hex-encoded SHA-256 of server cert DER
	pending      sync.Map                         // name (string) -> chan net.Conn
	sem          chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewPluginAcceptor creates an acceptor that authenticates connections on the
// given listener using the shared secret. Call Start() to begin accepting.
// certFP is the hex-encoded SHA-256 fingerprint of the server cert (from CertFingerprint).
func NewPluginAcceptor(listener net.Listener, secret, certFP string) *PluginAcceptor {
	ctx, cancel := context.WithCancel(context.Background())
	return &PluginAcceptor{
		listener:     listener,
		secret:       secret,
		pluginTokens: make(map[string]string),
		certFP:       certFP,
		sem:          make(chan struct{}, maxPendingConns),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// SetSecretLookup sets a per-client secret lookup function.
// When set, auth first checks per-client secrets by name, falling back
// to the shared secret if the name is not found in the lookup.
func (pa *PluginAcceptor) SetSecretLookup(lookup func(name string) (string, bool)) {
	pa.secretLookup = lookup
}

// Addr returns the listener's address (useful when bound to port 0 in tests).
func (pa *PluginAcceptor) Addr() net.Addr {
	return pa.listener.Addr()
}

// Token returns the shared auth token. Used by startExternal to pass via env var.
func (pa *PluginAcceptor) Token() string {
	return pa.secret
}

// TokenForPlugin returns a unique random token for the given plugin name.
// Generates a new 32-byte token on first call for each name; subsequent calls
// return the same token. Safe for concurrent use.
func (pa *PluginAcceptor) TokenForPlugin(name string) (string, error) {
	pa.tokenMu.Lock()
	defer pa.tokenMu.Unlock()

	if token, ok := pa.pluginTokens[name]; ok {
		return token, nil
	}

	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return "", fmt.Errorf("generate plugin token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes[:])
	pa.pluginTokens[name] = token
	return token, nil
}

// CertFP returns the hex-encoded SHA-256 fingerprint of the server certificate.
func (pa *PluginAcceptor) CertFP() string {
	return pa.certFP
}

// Start begins the accept loop in a goroutine. Each accepted connection is
// authenticated and routed to the matching WaitForPlugin caller.
func (pa *PluginAcceptor) Start() {
	go pa.acceptLoop()
}

// Stop closes the listener and cancels the accept loop.
func (pa *PluginAcceptor) Stop() {
	pa.cancel()
	pa.listener.Close() //nolint:errcheck,gosec // shutdown cleanup
}

// WaitForPlugin blocks until a plugin with the given name connects and
// authenticates, or until ctx expires. Returns the authenticated connection.
func (pa *PluginAcceptor) WaitForPlugin(ctx context.Context, name string) (net.Conn, error) {
	ch := make(chan net.Conn, 1)
	pa.pending.Store(name, ch)

	select {
	case conn := <-ch:
		pa.pending.Delete(name)
		return conn, nil
	case <-ctx.Done():
		pa.pending.Delete(name)
		// Drain any connection that arrived concurrently.
		// The channel may have received a conn between ctx.Done firing
		// and pending.Delete -- must close it to prevent leak.
		pa.drainConnChan(ch)
		return nil, ctx.Err()
	case <-pa.ctx.Done():
		pa.pending.Delete(name)
		pa.drainConnChan(ch)
		return nil, fmt.Errorf("acceptor stopped")
	}
}

// drainConnChan spawns a goroutine that waits for a connection on ch
// and closes it. Uses the acceptor context to avoid leaking the goroutine
// if no connection ever arrives (e.g., plugin process crashed before connecting).
func (pa *PluginAcceptor) drainConnChan(ch chan net.Conn) {
	go func() {
		select {
		case conn, ok := <-ch:
			if ok && conn != nil {
				conn.Close() //nolint:errcheck,gosec // close leaked connection
			}
		case <-pa.ctx.Done():
			// Acceptor stopped; no connection will arrive.
		}
	}()
}

// combinedLookup returns a lookup function that checks per-plugin tokens first,
// then the external secretLookup. Returns false if neither has a match,
// causing AuthenticateWithLookup to fall back to the shared secret.
func (pa *PluginAcceptor) combinedLookup() func(string) (string, bool) {
	return func(name string) (string, bool) {
		pa.tokenMu.Lock()
		token, ok := pa.pluginTokens[name]
		pa.tokenMu.Unlock()
		if ok {
			return token, true
		}
		if pa.secretLookup != nil {
			return pa.secretLookup(name)
		}
		return "", false
	}
}

func (pa *PluginAcceptor) acceptLoop() {
	for {
		conn, err := pa.listener.Accept()
		if err != nil {
			if pa.ctx.Err() != nil {
				return // Acceptor stopped.
			}
			slog.Debug("acceptor: accept error", "error", err)
			continue
		}

		// Disable Nagle's algorithm for plugin IPC. Plugin RPCs are
		// small request-response messages; Nagle adds latency without
		// batching benefit.
		setTCPNoDelay(conn)

		// Limit concurrent unauthenticated connections.
		select {
		case pa.sem <- struct{}{}:
			go pa.handleConn(conn)
		case <-pa.ctx.Done():
			conn.Close() //nolint:errcheck,gosec // shutting down
			return
		}
	}
}

func (pa *PluginAcceptor) handleConn(conn net.Conn) {
	defer func() { <-pa.sem }() // Release semaphore slot.

	authCtx, cancel := context.WithTimeout(pa.ctx, 10*time.Second)
	defer cancel()

	// Build a combined lookup: per-plugin tokens first, then external secretLookup.
	// AuthenticateWithLookup falls back to the shared secret if lookup returns false.
	// Per-plugin tokens provide natural name binding: the lookup is by the name
	// the client claims, so impersonation attempts get the wrong expected token.
	lookup := pa.combinedLookup()
	name, err := AuthenticateWithLookup(authCtx, conn, pa.secret, lookup)
	if err != nil {
		return // Authenticate already closed conn on failure.
	}

	val, ok := pa.pending.LoadAndDelete(name)
	if !ok {
		// No one waiting for this plugin name. Close.
		conn.Close() //nolint:errcheck,gosec // unexpected plugin
		return
	}

	ch, chOK := val.(chan net.Conn)
	if !chOK {
		conn.Close() //nolint:errcheck,gosec // type assertion failed
		return
	}

	// Non-blocking send: if the waiter already left (context expired),
	// close the connection instead of leaking it in the channel buffer.
	select {
	case ch <- conn:
		// Delivered to WaitForPlugin.
	case <-pa.ctx.Done():
		conn.Close() //nolint:errcheck,gosec // acceptor stopped
	}
}
