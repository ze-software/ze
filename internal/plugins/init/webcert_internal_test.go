package init

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: `ze init --web-cert/--web-cert-name` generates and stores TLS
// material through internal/core/selfcert with the listen IP and DNS name as SANs.
// PREVENTS: a regression where the cert-extraction move (web -> selfcert) silently
// breaks install-time HTTPS bootstrap, which the web unit suite does not exercise.

// TestInitPlugin_WebCertUsesSelfcert verifies that `ze init --web-cert /
// --web-cert-name` generates TLS material through internal/core/selfcert and
// persists it in the zefs store, with the requested listen IP and DNS name as
// SANs. This is the install-path cert bootstrap that must keep working with web
// compiled out (feature-gate-3-web AC-1/AC-3; selfcert is always-on).
//
// White-box (package init) so it can drive runInit directly with a temp dbPath
// and piped credentials, rather than os.Stdin + ResolveDBPath via Run.
func TestInitPlugin_WebCertUsesSelfcert(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// username, password, host, port, name(blank -> hostname default).
	creds := "admin\nsecret123\n127.0.0.1\n2222\n\n"
	// 192.0.2.10 is TEST-NET-1 (RFC 5737): never a real interface, so the SAN
	// assertion is deterministic and does not depend on the host's IPs.
	const listenAddr = "192.0.2.10:8443"
	const dnsName = "router.example.com"

	if code := runInit(strings.NewReader(creds), nil, dbPath, false, listenAddr, dnsName, false); code != 0 {
		t.Fatalf("runInit with --web-cert/--web-cert-name exit = %d, want 0", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	certPEM, err := store.ReadFile(zefs.KeyWebCert.Pattern)
	if err != nil {
		t.Fatalf("read stored web cert: %v", err)
	}
	keyPEM, err := store.ReadFile(zefs.KeyWebKey.Pattern)
	if err != nil {
		t.Fatalf("read stored web key: %v", err)
	}

	// A valid keypair proves selfcert produced real material (not a placeholder).
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("stored web cert/key are not a valid pair: %v", err)
	}

	// ECDSA key (selfcert generates ECDSA P-256); guards against algorithm
	// weakening in the move (AC security review).
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatalf("stored web key block = %v, want EC PRIVATE KEY", keyBlock)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("stored web cert PEM did not decode")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse stored web cert: %v", err)
	}

	if !slices.Contains(cert.DNSNames, dnsName) {
		t.Fatalf("cert DNSNames = %v, want to contain %q", cert.DNSNames, dnsName)
	}
	if !slices.Contains(cert.DNSNames, "localhost") {
		t.Fatalf("cert DNSNames = %v, want to contain localhost", cert.DNSNames)
	}
	foundIP := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "192.0.2.10" {
			foundIP = true
		}
	}
	if !foundIP {
		t.Fatalf("cert IPAddresses = %v, want to contain 192.0.2.10", cert.IPAddresses)
	}
}
