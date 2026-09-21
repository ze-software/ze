//go:build ze_mcp

// RFC: rfc/short/rfc9728.md -- Section 7.1, TLS requirements
// Related: service_mcp.go -- loadMCPTLSConfig, the tls.Config the MCP listener serves

// Goal: prove the MCP listener's TLS material is the tls.Config Ze installs
// and that it refuses the protocol versions BCP 195 deprecates.
// Method: load a generated certificate pair through loadMCPTLSConfig and run
// real handshakes against it over an in-memory pipe.
//
// VALIDATES: a valid pair yields a serving tls.Config, a mismatched pair is
// refused, TLS 1.2 handshakes succeed, and TLS 1.0 and 1.1 clients are
// refused by the server side.
// PREVENTS: a listener that silently accepts a deprecated TLS version, or
// that serves without valid material.
//
// These lines are not requirement tags: the tag scanner walks internal/,
// pkg/ and test/ only (internal/le/rfc/rfc.go, testRoots), so a tag here
// would be read by nothing and could carry no discrimination record.

package hub

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// writeMCPTLSPair writes a fresh certificate pair into dir with the key
// permissions loadMCPTLSConfig requires, and returns the two paths.
func writeMCPTLSPair(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("127.0.0.1:0")
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}
	certFile = filepath.Join(dir, "mcp.pem")
	keyFile = filepath.Join(dir, "mcp.key")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}

// handshake runs one TLS handshake between a server built from serverCfg
// and a client built from clientCfg over an in-memory pipe, and returns the
// server-side error. The goroutine is one-shot and is joined through errCh
// before the function returns.
func handshake(t *testing.T, serverCfg, clientCfg *tls.Config) error {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	defer func() { _ = clientConn.Close() }()
	errCh := make(chan error, 1)
	go func() {
		errCh <- tls.Server(serverConn, serverCfg).HandshakeContext(t.Context())
	}()
	clientErr := tls.Client(clientConn, clientCfg).HandshakeContext(t.Context())
	serverErr := <-errCh
	if serverErr == nil && clientErr != nil {
		t.Fatalf("client failed where server succeeded: %v", clientErr)
	}
	return serverErr
}

// RFC 9728 Section 7.1, RFC9728-7.1-1 positive: loadMCPTLSConfig turns a valid certificate pair into a tls.Config carrying that certificate, and a TLS client completes a handshake against it.
// RFC 9728 Section 7.1, RFC9728-7.1-1 negative: a certificate whose key belongs to another pair is refused with an error and no tls.Config, so the listener never serves without valid TLS material.
func TestRFC9728MCPListenerSupportsTLS(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeMCPTLSPair(t, dir)
	cfg, err := loadMCPTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatalf("loadMCPTLSConfig: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("certificates = %d, want 1", len(cfg.Certificates))
	}
	if err := handshake(t, cfg, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}); err != nil { //nolint:gosec // test client against a self-signed pair
		t.Fatalf("TLS handshake: %v", err)
	}

	otherDir := t.TempDir()
	_, otherKey := writeMCPTLSPair(t, otherDir)
	mismatched, err := loadMCPTLSConfig(certFile, otherKey)
	if err == nil {
		t.Fatal("mismatched pair accepted, want an error")
	}
	if mismatched != nil {
		t.Fatal("mismatched pair returned a tls.Config, want nil")
	}
}

// RFC 9728 Section 7.1, RFC9728-7.1-2 positive: the MCP tls.Config accepts a TLS 1.2 handshake, the version RFC 9325 Section 3.1.1 requires implementations to support.
// RFC 9728 Section 7.1, RFC9728-7.1-2 negative: a client offering at most TLS 1.1 is refused by the server side, the versions RFC 8996 forbids negotiating.
func TestRFC9728MCPListenerFollowsBCP195Versions(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeMCPTLSPair(t, dir)
	cfg, err := loadMCPTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatalf("loadMCPTLSConfig: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %#x, want TLS 1.2", cfg.MinVersion)
	}
	accepted := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12} //nolint:gosec // test client against a self-signed pair
	if err := handshake(t, cfg, accepted); err != nil {
		t.Fatalf("TLS 1.2 handshake refused: %v", err)
	}
	deprecated := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11} //nolint:gosec // deliberately deprecated versions, the refusal is the assertion
	if err := handshake(t, cfg, deprecated); err == nil {
		t.Fatal("TLS 1.1 handshake accepted, want the server to refuse it")
	}
}
