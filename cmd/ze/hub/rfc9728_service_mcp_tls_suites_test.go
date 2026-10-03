//go:build ze_mcp

// RFC: rfc/short/rfc9728.md -- RFC9728-7.1-2, BCP 195 (RFC 9325) cipher suites
// Related: service_mcp.go -- loadMCPTLSConfig and startMCPServer, the listener probed here

package hub

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	zemcp "github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/test/tlsprobe"
)

// VALIDATES: RFC 9728 section 7.1 beyond the version floor that
// TestRFC9728MCPServingTLSVersions proves: the listener startMCPServer opens
// negotiates only what the RFC 9325 guidance allows. Method: the listener is
// started with an ECDSA pair (writeMCPTLSPair) and with an RSA pair, and a
// hand-built ClientHello (tlsprobe) offers each suite on its own, including the
// NULL, export and single-DES suites Go's client cannot offer.
// PREVENTS: an MCP listener that selects a NULL, RC4, export, DES, 3DES, static
// RSA, static ECDH or DHE suite, selects DEFLATE compression, or omits
// renegotiation_info, while the version tests stay green.
// RFC requirement: RFC9728-7.1-2 positive -- with an ECDSA and with an RSA certificate, the MCP listener selects the ECDHE AES-128-GCM suite, null compression and answers with renegotiation_info, also when that suite is offered after every forbidden one.
// RFC requirement: RFC9728-7.1-2 negative -- each NULL, RC4, export, DES, 3DES, static-RSA, static-ECDH and DHE suite offered alone is never selected by the MCP listener (alert or close), and DEFLATE offered alone is refused.
func TestRFC9728MCPListenerNegotiatesOnlyBCP195Suites(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tlsprobe.KeyKind
	}{
		{"ecdsa", tlsprobe.KeyECDSA},
		{"rsa", tlsprobe.KeyRSA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			certFile, keyFile := writeMCPTLSPair(t, dir)
			if tc.key == tlsprobe.KeyRSA {
				certFile, keyFile = writeMCPRSAPair(t, dir)
			}
			h := startMCPServer([]string{"127.0.0.1:0"}, nil, nil,
				zemcp.StreamableConfig{AuthMode: zemcp.AuthNone}, certFile, keyFile)
			if h == nil {
				t.Fatal("MCP listener did not start")
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := h.Shutdown(ctx); err != nil {
					t.Errorf("MCP shutdown: %v", err)
				}
			})
			addresses := h.Addresses()
			if len(addresses) != 1 {
				t.Fatalf("listener addresses = %v", addresses)
			}
			tlsprobe.AssertBCP195(t, addresses[0], tc.key)
		})
	}
}

// writeMCPRSAPair writes an RSA certificate pair into dir with the key
// permissions loadMCPTLSConfig requires, and returns the two paths.
func writeMCPRSAPair(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM, err := tlsprobe.SelfSignedRSA("127.0.0.1")
	if err != nil {
		t.Fatalf("generate RSA cert: %v", err)
	}
	certFile = filepath.Join(dir, "mcp-rsa.pem")
	keyFile = filepath.Join(dir, "mcp-rsa.key")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}
