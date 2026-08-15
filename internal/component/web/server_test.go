package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/selfcert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpsGet fetches a URL with a TLS-InsecureSkipVerify client and returns the
// body. Used by the multi-listener test to prove both endpoints serve.
func httpsGet(t *testing.T, url string) (int, string) {
	t.Helper()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test client against self-signed cert
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("body close: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// TestWebServer_MultiListener verifies that every address in WebConfig.ListenAddrs
// is bound, Addresses() reports every bound ip:port, and both endpoints serve
// the same mux concurrently.
//
// VALIDATES: AC-1 (web config with two server entries binds both endpoints).
// VALIDATES: AC-14 (graceful Shutdown closes every listener).
// PREVENTS: Regression where a multi-listener binder silently serves only the
// first endpoint.
func TestWebServer_MultiListener(t *testing.T) {
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	srv, err := NewWebServer(WebConfig{
		ListenAddrs: []string{"127.0.0.1:0", "127.0.0.1:0"},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	require.NoError(t, err)

	srv.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte("pong")); writeErr != nil {
			t.Logf("write: %v", writeErr)
		}
	})

	serveErrCh := make(chan error, 1)
	go func() {
		serveErr := srv.ListenAndServe(context.Background())
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serveErrCh <- serveErr
			return
		}
		close(serveErrCh)
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	require.NoError(t, srv.WaitReady(readyCtx))

	addrs := srv.Addresses()
	require.Len(t, addrs, 2, "expected 2 bound addresses")
	assert.NotEqual(t, addrs[0], addrs[1], "two listeners must bind distinct ports")
	assert.Contains(t, addrs[0], "127.0.0.1:")
	assert.Contains(t, addrs[1], "127.0.0.1:")

	// Address() should return the first bound address for backward compat.
	assert.Equal(t, addrs[0], srv.Address())

	// Fetch from each listener independently.
	for i, addr := range addrs {
		status, body := httpsGet(t, fmt.Sprintf("https://%s/ping", addr))
		assert.Equal(t, http.StatusOK, status, "listener %d (%s)", i, addr)
		assert.Equal(t, "pong", body, "listener %d (%s)", i, addr)
	}

	// Graceful shutdown should close both listeners.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	require.NoError(t, srv.Shutdown(shutdownCtx))

	select {
	case err := <-serveErrCh:
		if err != nil {
			t.Fatalf("ListenAndServe returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe did not return after Shutdown")
	}
}

// TestWebServer_BindFailureClosesPartialListeners verifies that when the
// second listen address fails to bind (because it is already in use), the
// first successfully-bound listener is closed and ListenAndServe returns
// the bind error instead of leaking a half-bound server.
//
// VALIDATES: AC-15 (fail-fast on partial bind).
// PREVENTS: Silently ending up with N-1 listeners live after a bind failure.
func TestWebServer_BindFailureClosesPartialListeners(t *testing.T) {
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	// Grab a port that is guaranteed to be in use by binding it ourselves.
	squatter, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		if closeErr := squatter.Close(); closeErr != nil {
			t.Logf("squatter close: %v", closeErr)
		}
	}()
	squattedAddr := squatter.Addr().String()

	srv, err := NewWebServer(WebConfig{
		// First entry should succeed; second entry should fail because the
		// port is held by the squatter above.
		ListenAddrs: []string{"127.0.0.1:0", squattedAddr},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	require.NoError(t, err)

	err = srv.ListenAndServe(context.Background())
	require.Error(t, err, "ListenAndServe must fail when any bind fails")
	assert.Contains(t, err.Error(), "bind")
	assert.Contains(t, err.Error(), squattedAddr)
}

// TestGenerateWebCert verifies that GenerateWebCert produces valid PEM-encoded
// ECDSA P-256 certificate and key material suitable for TLS.
// VALIDATES: AC-9 (self-signed cert generated).
// PREVENTS: invalid or unparseable certificate material.
func TestGenerateWebCert(t *testing.T) {
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	require.NotEmpty(t, certPEM, "certPEM must not be empty")
	require.NotEmpty(t, keyPEM, "keyPEM must not be empty")

	// Parse the certificate PEM block.
	certBlock, rest := pem.Decode(certPEM)
	require.NotNil(t, certBlock, "certPEM must contain a valid PEM block")
	assert.Equal(t, "CERTIFICATE", certBlock.Type)
	assert.Empty(t, rest, "certPEM must contain exactly one PEM block")

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err, "certificate must be valid X.509")

	// Verify ECDSA P-256 key type.
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "certificate public key must be ECDSA")
	assert.Equal(t, elliptic.P256(), pub.Curve, "certificate must use P-256 curve")

	// Verify SANs include localhost and loopback addresses.
	assert.Contains(t, cert.DNSNames, "localhost", "certificate must have localhost SAN")

	foundIPv4Loopback := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			foundIPv4Loopback = true
		}
	}
	assert.True(t, foundIPv4Loopback, "certificate must have 127.0.0.1 SAN")

	// Verify key usage.
	assert.Equal(t, x509.KeyUsageDigitalSignature, cert.KeyUsage)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

	// Parse the private key PEM block.
	keyBlock, rest := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock, "keyPEM must contain a valid PEM block")
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)
	assert.Empty(t, rest, "keyPEM must contain exactly one PEM block")

	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	require.NoError(t, err, "private key must be valid ECDSA")
	assert.Equal(t, elliptic.P256(), key.Curve, "private key must use P-256 curve")
}

// TestGenerateWebCertWithAddr verifies that GenerateWebCertWithAddr adds the
// listen address as an extra SAN when it is a non-loopback IP.
// VALIDATES: AC-9 (cert includes listen address SAN).
// PREVENTS: TLS errors when accessing ze via non-loopback address.
func TestGenerateWebCertWithAddr(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		expectIP  string
		expectAdd bool // whether the IP should be added beyond the defaults
	}{
		{
			name:      "non-loopback IP added as SAN",
			addr:      "192.168.1.100:8443",
			expectIP:  "192.168.1.100",
			expectAdd: true,
		},
		{
			name:      "loopback IP not duplicated",
			addr:      "127.0.0.1:8443",
			expectIP:  "127.0.0.1",
			expectAdd: false, // already in defaults
		},
		{
			name:      "empty addr uses defaults only",
			addr:      "",
			expectIP:  "",
			expectAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPEM, _, err := selfcert.GenerateWebCertWithAddr(tt.addr)
			require.NoError(t, err)

			block, _ := pem.Decode(certPEM)
			require.NotNil(t, block)

			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			if tt.expectAdd {
				found := false
				for _, ip := range cert.IPAddresses {
					if ip.String() == tt.expectIP {
						found = true
						break
					}
				}
				assert.True(t, found, "certificate must include %s as SAN", tt.expectIP)
			}

			// Default SANs must always be present.
			assert.Contains(t, cert.DNSNames, "localhost")
		})
	}
}

// TestGenerateWebCertWithNames verifies that extra DNS names and IP addresses
// are added as SANs to the generated certificate.
// VALIDATES: extra names appear in cert SANs.
// PREVENTS: cert missing hostname SAN when --web-cert-name is used.
func TestGenerateWebCertWithNames(t *testing.T) {
	tests := []struct {
		name       string
		listenAddr string
		extraNames []string
		expectDNS  []string
		expectIPs  []string
	}{
		{
			name:       "nil extra names",
			listenAddr: "0.0.0.0:8080",
			extraNames: nil,
			expectDNS:  []string{"localhost"},
		},
		{
			name:       "name only without listen addr",
			listenAddr: "",
			extraNames: []string{"router.example.com"},
			expectDNS:  []string{"localhost", "router.example.com"},
		},
		{
			name:       "DNS hostname added",
			listenAddr: "0.0.0.0:8080",
			extraNames: []string{"router.example.com"},
			expectDNS:  []string{"localhost", "router.example.com"},
		},
		{
			name:       "IP address added as IP SAN",
			listenAddr: "0.0.0.0:8080",
			extraNames: []string{"10.0.0.1"},
			expectDNS:  []string{"localhost"},
			expectIPs:  []string{"10.0.0.1"},
		},
		{
			name:       "mixed DNS and IP",
			listenAddr: "0.0.0.0:8080",
			extraNames: []string{"router.local", "10.0.0.1"},
			expectDNS:  []string{"localhost", "router.local"},
			expectIPs:  []string{"10.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPEM, _, err := selfcert.GenerateWebCertWithNames(tt.listenAddr, tt.extraNames, 0)
			require.NoError(t, err)

			block, _ := pem.Decode(certPEM)
			require.NotNil(t, block)

			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			for _, dns := range tt.expectDNS {
				assert.Contains(t, cert.DNSNames, dns, "certificate must include DNS SAN %s", dns)
			}
			for _, ipStr := range tt.expectIPs {
				found := false
				for _, ip := range cert.IPAddresses {
					if ip.String() == ipStr {
						found = true
						break
					}
				}
				assert.True(t, found, "certificate must include IP SAN %s", ipStr)
			}
		})
	}
}

// TestGenerateWebCertWithAddr_UnspecifiedIncludesInterfaceIPs verifies that
// listening on 0.0.0.0 adds the machine's non-loopback interface IPs as SANs.
// VALIDATES: cert is valid for any local IP when listening on all interfaces.
// PREVENTS: TLS errors when accessing ze via a non-loopback IP (e.g., 10.x.x.x).
func TestGenerateWebCertWithAddr_UnspecifiedIncludesInterfaceIPs(t *testing.T) {
	certPEM, _, err := selfcert.GenerateWebCertWithAddr("0.0.0.0:8443")
	require.NoError(t, err)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Must have more than the 2 IP defaults (127.0.0.1, ::1) since every
	// machine with networking has at least one non-loopback IP. ("localhost"
	// is in DNSNames, not IPAddresses.)
	assert.Greater(t, len(cert.IPAddresses), 2,
		"certificate for 0.0.0.0 must include interface IPs beyond defaults (got %v)", cert.IPAddresses)

	// 0.0.0.0 itself should NOT be in the SANs (it is replaced by real IPs).
	for _, ip := range cert.IPAddresses {
		assert.False(t, ip.IsUnspecified(),
			"certificate must not include unspecified address 0.0.0.0 as SAN")
	}
}

// TestNewTLSConfig verifies that NewTLSConfig produces a valid tls.Config from
// generated PEM material with the expected minimum TLS version.
// VALIDATES: TLS works with generated cert.
// PREVENTS: misconfigured TLS settings, missing certificates.
func TestNewTLSConfig(t *testing.T) {
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	tlsCfg, err := selfcert.NewTLSConfig(certPEM, keyPEM)
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)

	// Must have exactly one certificate loaded.
	require.Len(t, tlsCfg.Certificates, 1, "TLS config must have one certificate")

	// Must enforce TLS 1.2 minimum.
	assert.Equal(t, uint16(tls.VersionTLS12), tlsCfg.MinVersion,
		"TLS config must enforce minimum TLS 1.2")
}

// TestNewTLSConfigInvalidPEM verifies that NewTLSConfig rejects invalid PEM data.
// PREVENTS: silent acceptance of corrupt certificate material.
func TestNewTLSConfigInvalidPEM(t *testing.T) {
	tests := []struct {
		name    string
		certPEM []byte
		keyPEM  []byte
	}{
		{
			name:    "empty cert",
			certPEM: []byte{},
			keyPEM:  []byte("not empty"),
		},
		{
			name:    "empty key",
			certPEM: []byte("not empty"),
			keyPEM:  []byte{},
		},
		{
			name:    "garbage data",
			certPEM: []byte("not a cert"),
			keyPEM:  []byte("not a key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := selfcert.NewTLSConfig(tt.certPEM, tt.keyPEM)
			assert.Error(t, err, "NewTLSConfig must reject invalid PEM data")
		})
	}
}

// TestNewWebServerRequiresFields verifies that NewWebServer rejects configurations
// with missing required fields.
// PREVENTS: server creation with no listen address or TLS material.
func TestNewWebServerRequiresFields(t *testing.T) {
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	tests := []struct {
		name    string
		cfg     WebConfig
		wantErr string
	}{
		{
			name:    "missing listen addresses",
			cfg:     WebConfig{CertPEM: certPEM, KeyPEM: keyPEM},
			wantErr: "at least one listen address is required",
		},
		{
			name:    "empty string in listen addresses",
			cfg:     WebConfig{ListenAddrs: []string{""}, CertPEM: certPEM, KeyPEM: keyPEM},
			wantErr: "listen address must not be empty",
		},
		{
			name:    "missing cert and key",
			cfg:     WebConfig{ListenAddrs: []string{"127.0.0.1:0"}},
			wantErr: "certificate and key PEM data are required",
		},
		{
			name:    "missing key only",
			cfg:     WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: certPEM},
			wantErr: "certificate and key PEM data are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWebServer(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// mockCertStore implements CertStore for testing LoadOrGenerateCert.
// It tracks which methods were called so tests can verify the store interaction.
type mockCertStore struct {
	certData []byte
	keyData  []byte
	exists   bool

	readCertCalled  bool
	readKeyCalled   bool
	writeCertCalled bool
	writeKeyCalled  bool

	writeCertErr error
	writeKeyErr  error
}

func (m *mockCertStore) ReadCert() ([]byte, error) {
	m.readCertCalled = true
	if m.certData == nil {
		return nil, fmt.Errorf("no cert stored")
	}
	return m.certData, nil
}

func (m *mockCertStore) ReadKey() ([]byte, error) {
	m.readKeyCalled = true
	if m.keyData == nil {
		return nil, fmt.Errorf("no key stored")
	}
	return m.keyData, nil
}

func (m *mockCertStore) WriteCert(data []byte) error {
	m.writeCertCalled = true
	m.certData = data
	return m.writeCertErr
}

func (m *mockCertStore) WriteKey(data []byte) error {
	m.writeKeyCalled = true
	m.keyData = data
	return m.writeKeyErr
}

func (m *mockCertStore) Exists() bool {
	return m.exists
}

// TestLoadOrGenerateCert_GenerateNew verifies that LoadOrGenerateCert generates
// a new self-signed certificate and persists it when the store is empty.
// VALIDATES: AC-9 (cert generated and stored when none exists).
// PREVENTS: Missing WriteCert/WriteKey calls, invalid generated PEM.
func TestLoadOrGenerateCert_GenerateNew(t *testing.T) {
	store := &mockCertStore{exists: false}

	certPEM, keyPEM, err := selfcert.LoadOrGenerateCert(store, "127.0.0.1:8443")
	require.NoError(t, err)
	require.NotEmpty(t, certPEM, "certPEM must not be empty")
	require.NotEmpty(t, keyPEM, "keyPEM must not be empty")

	// Verify WriteCert and WriteKey were called (certificate persisted).
	assert.True(t, store.writeCertCalled, "WriteCert must be called for new cert")
	assert.True(t, store.writeKeyCalled, "WriteKey must be called for new cert")

	// Verify ReadCert and ReadKey were NOT called (no existing cert to load).
	assert.False(t, store.readCertCalled, "ReadCert must not be called when store is empty")
	assert.False(t, store.readKeyCalled, "ReadKey must not be called when store is empty")

	// Verify the returned PEM is valid and usable for TLS.
	certBlock, _ := pem.Decode(certPEM)
	require.NotNil(t, certBlock, "returned certPEM must be valid PEM")
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	keyBlock, _ := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock, "returned keyPEM must be valid PEM")
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)

	// Verify the cert and key form a valid TLS pair.
	_, err = tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err, "generated cert and key must form a valid TLS pair")
}

// TestLoadOrGenerateCert_LoadExisting verifies that LoadOrGenerateCert loads
// existing certificate material from the store without generating new certs.
// VALIDATES: AC-9 (existing cert loaded from store).
// PREVENTS: Regenerating certs when store already has valid material.
func TestLoadOrGenerateCert_LoadExisting(t *testing.T) {
	// Pre-generate valid cert material to store.
	origCert, origKey, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	store := &mockCertStore{
		exists:   true,
		certData: origCert,
		keyData:  origKey,
	}

	certPEM, keyPEM, err := selfcert.LoadOrGenerateCert(store, "127.0.0.1:8443")
	require.NoError(t, err)

	// Verify ReadCert and ReadKey were called (loading from store).
	assert.True(t, store.readCertCalled, "ReadCert must be called when store has certs")
	assert.True(t, store.readKeyCalled, "ReadKey must be called when store has certs")

	// Verify WriteCert and WriteKey were NOT called (no generation needed).
	assert.False(t, store.writeCertCalled, "WriteCert must not be called when loading existing")
	assert.False(t, store.writeKeyCalled, "WriteKey must not be called when loading existing")

	// Verify the returned PEM matches what was stored.
	assert.Equal(t, origCert, certPEM, "returned certPEM must match stored cert")
	assert.Equal(t, origKey, keyPEM, "returned keyPEM must match stored key")
}

// --- Reconfigure tests ---

// newTestServer creates a WebServer bound to N loopback listeners (port 0),
// starts it, waits for ready, and returns it with a cleanup function.
func newTestServer(t *testing.T, n int) *WebServer {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	addrs := make([]string, n)
	for i := range addrs {
		addrs[i] = "127.0.0.1:0"
	}

	srv, err := NewWebServer(WebConfig{
		ListenAddrs: addrs,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	require.NoError(t, err)

	srv.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte("pong")); writeErr != nil {
			return
		}
	})

	go func() {
		if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Logf("ListenAndServe: %v", serveErr)
		}
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	require.NoError(t, srv.WaitReady(readyCtx))

	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			t.Logf("shutdown: %v", shutErr)
		}
	})

	return srv
}

// TestListenerDiffKeepAddRemove verifies the set diff computation.
// VALIDATES: listenerDiff correctly classifies addresses.
func TestListenerDiffKeepAddRemove(t *testing.T) {
	tests := []struct {
		name       string
		old, new   []string
		wantKeep   []string
		wantAdd    []string
		wantRemove []string
	}{
		{
			name:       "no change",
			old:        []string{"127.0.0.1:3443"},
			new:        []string{"127.0.0.1:3443"},
			wantKeep:   []string{"127.0.0.1:3443"},
			wantAdd:    nil,
			wantRemove: nil,
		},
		{
			name:       "add one",
			old:        []string{"127.0.0.1:3443"},
			new:        []string{"127.0.0.1:3443", "127.0.0.1:8080"},
			wantKeep:   []string{"127.0.0.1:3443"},
			wantAdd:    []string{"127.0.0.1:8080"},
			wantRemove: nil,
		},
		{
			name:       "remove one",
			old:        []string{"127.0.0.1:3443", "127.0.0.1:8080"},
			new:        []string{"127.0.0.1:3443"},
			wantKeep:   []string{"127.0.0.1:3443"},
			wantAdd:    nil,
			wantRemove: []string{"127.0.0.1:8080"},
		},
		{
			name:       "swap",
			old:        []string{"127.0.0.1:3443"},
			new:        []string{"127.0.0.1:8080"},
			wantKeep:   nil,
			wantAdd:    []string{"127.0.0.1:8080"},
			wantRemove: []string{"127.0.0.1:3443"},
		},
		{
			name:       "complete replace",
			old:        []string{"127.0.0.1:3443", "127.0.0.1:4443"},
			new:        []string{"127.0.0.1:8080", "127.0.0.1:9090"},
			wantKeep:   nil,
			wantAdd:    []string{"127.0.0.1:8080", "127.0.0.1:9090"},
			wantRemove: []string{"127.0.0.1:3443", "127.0.0.1:4443"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep, add, remove := webListenerDiff(tt.old, tt.new)
			assert.Equal(t, tt.wantKeep, keep, "keep")
			assert.Equal(t, tt.wantAdd, add, "add")
			assert.Equal(t, tt.wantRemove, remove, "remove")
		})
	}
}

// TestWebServerReconfigureNoop verifies that Reconfigure with unchanged
// addresses is a no-op.
// VALIDATES: AC-3.
func TestWebServerReconfigureNoop(t *testing.T) {
	srv := newTestServer(t, 1)
	origAddrs := srv.Addresses()

	err := srv.Reconfigure(context.Background(), origAddrs)
	require.NoError(t, err)

	assert.Equal(t, origAddrs, srv.Addresses(), "addresses must not change on no-op reconfigure")

	status, body := httpsGet(t, fmt.Sprintf("https://%s/ping", origAddrs[0]))
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "pong", body)
}

// TestWebServerReconfigureAddresses verifies Addresses() after Reconfigure.
// VALIDATES: AC-6.
func TestWebServerReconfigureAddresses(t *testing.T) {
	srv := newTestServer(t, 1)
	origAddrs := srv.Addresses()
	require.Len(t, origAddrs, 1)

	newAddrs := []string{origAddrs[0], "127.0.0.1:0"}
	err := srv.Reconfigure(context.Background(), newAddrs)
	require.NoError(t, err)

	addrs := srv.Addresses()
	assert.Len(t, addrs, 2, "must have two addresses after adding one")
	assert.Equal(t, origAddrs[0], addrs[0], "original address must be first")
}

// TestWebServerReconfigureAddListener verifies adding a listener to a running server.
// VALIDATES: AC-4.
func TestWebServerReconfigureAddListener(t *testing.T) {
	srv := newTestServer(t, 1)
	origAddrs := srv.Addresses()

	newAddrs := []string{origAddrs[0], "127.0.0.1:0"}
	err := srv.Reconfigure(context.Background(), newAddrs)
	require.NoError(t, err)

	addrs := srv.Addresses()
	require.Len(t, addrs, 2)

	for i, addr := range addrs {
		status, body := httpsGet(t, fmt.Sprintf("https://%s/ping", addr))
		assert.Equal(t, http.StatusOK, status, "listener %d (%s)", i, addr)
		assert.Equal(t, "pong", body, "listener %d (%s)", i, addr)
	}
}

// TestWebServerReconfigureRemoveListener verifies removing a listener.
// VALIDATES: AC-5.
func TestWebServerReconfigureRemoveListener(t *testing.T) {
	srv := newTestServer(t, 2)
	origAddrs := srv.Addresses()
	require.Len(t, origAddrs, 2)

	err := srv.Reconfigure(context.Background(), origAddrs[:1])
	require.NoError(t, err)

	addrs := srv.Addresses()
	require.Len(t, addrs, 1)
	assert.Equal(t, origAddrs[0], addrs[0])

	status, body := httpsGet(t, fmt.Sprintf("https://%s/ping", addrs[0]))
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "pong", body)

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s/ping", origAddrs[1]), http.NoBody)
	require.NoError(t, reqErr)
	_, doErr := client.Do(req) //nolint:bodyclose // connection refused, no body
	assert.Error(t, doErr, "removed listener must refuse connections")
}

// TestWebServerReconfigureSwapListener verifies swapping one address for another.
// VALIDATES: AC-1 (new bound before old closed).
func TestWebServerReconfigureSwapListener(t *testing.T) {
	srv := newTestServer(t, 1)
	origAddrs := srv.Addresses()

	err := srv.Reconfigure(context.Background(), []string{"127.0.0.1:0"})
	require.NoError(t, err)

	newAddrs := srv.Addresses()
	require.Len(t, newAddrs, 1)
	assert.NotEqual(t, origAddrs[0], newAddrs[0], "address must have changed")

	status, body := httpsGet(t, fmt.Sprintf("https://%s/ping", newAddrs[0]))
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "pong", body)
}

// TestWebServerReconfigureBindFails verifies rollback on bind failure.
// VALIDATES: AC-2.
func TestWebServerReconfigureBindFails(t *testing.T) {
	srv := newTestServer(t, 1)
	origAddrs := srv.Addresses()

	squatter, listenErr := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	require.NoError(t, listenErr)
	defer func() {
		if closeErr := squatter.Close(); closeErr != nil {
			t.Logf("squatter close: %v", closeErr)
		}
	}()
	squattedAddr := squatter.Addr().String()

	err := srv.Reconfigure(context.Background(), []string{squattedAddr})
	require.Error(t, err, "reconfigure must fail when bind fails")
	assert.Contains(t, err.Error(), "reconfigure bind")

	assert.Equal(t, origAddrs, srv.Addresses(), "addresses must not change on failed reconfigure")

	status, body := httpsGet(t, fmt.Sprintf("https://%s/ping", origAddrs[0]))
	assert.Equal(t, http.StatusOK, status, "original listener must still serve")
	assert.Equal(t, "pong", body)
}

// TestWebServerSetsSecurityHeadersOnEveryRoute verifies the headers reach a
// response served by a handler that never touches the authentication
// middleware.
//
// VALIDATES: the four browser-facing security headers on a route registered
// straight onto the mux, which is what "/", "GET /favicon.ico" and "/assets/"
// are in the hub.
// PREVENTS: the root document, the favicon and every static asset shipping with
// no Content-Security-Policy, X-Frame-Options, X-Content-Type-Options or HSTS.
// addSecurityHeaders was called inside AuthMiddlewareWithAudit alone, so a route
// outside it carried none while every sibling page carried all four.
//
// It drives the real entry point: NewWebServer builds the handler chain, and a
// test over SecurityHeaders alone would pass with the server still unwrapped.
func TestWebServerSetsSecurityHeadersOnEveryRoute(t *testing.T) {
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)

	srv, err := NewWebServer(WebConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	require.NoError(t, err)

	// No auth wrapper: this is the shape of the asset and favicon routes.
	srv.HandleFunc("/assets/ze.svg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, writeErr := w.Write([]byte("<svg/>")); writeErr != nil {
			t.Logf("write: %v", writeErr)
		}
	})

	serveErrCh := make(chan error, 1)
	go func() {
		serveErr := srv.ListenAndServe(context.Background())
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serveErrCh <- serveErr
			return
		}
		close(serveErrCh)
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	require.NoError(t, srv.WaitReady(readyCtx))

	resp := httpsHead(t, fmt.Sprintf("https://%s/assets/ze.svg", srv.Address()))

	assert.Equal(t, "DENY", resp.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", resp.Get("X-Content-Type-Options"))
	assert.Equal(t, "default-src 'self'; script-src 'self'; style-src 'self'",
		resp.Get("Content-Security-Policy"))
	assert.Equal(t, "max-age=63072000; includeSubDomains", resp.Get("Strict-Transport-Security"))

	// An asset stays cacheable. no-store belongs to an authenticated page, and
	// putting it here would refetch the stylesheet on every page load.
	assert.Equal(t, "public, max-age=86400", resp.Get("Cache-Control"))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	require.NoError(t, srv.Shutdown(shutdownCtx))
	require.NoError(t, <-serveErrCh)
}

// httpsHead fetches a URL over TLS and returns the response headers.
func httpsHead(t *testing.T, url string) http.Header {
	t.Helper()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test client against self-signed cert
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("body close: %v", closeErr)
		}
	}()

	if _, readErr := io.ReadAll(resp.Body); readErr != nil {
		t.Logf("body read: %v", readErr)
	}

	return resp.Header
}
