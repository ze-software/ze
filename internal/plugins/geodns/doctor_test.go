package geodns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/pki"
)

// VALIDATES: AC-3 -- geodns cert diagnostic is silent when no secure listener is
// enabled, flags doctor-tls-missing for an enabled DoT with an absent cert, and
// is silent for the self-signed fallback.
func TestGeodnsTLSDiagnostic(t *testing.T) {
	t.Parallel()
	if d := geodnsTLSDiagnostic(false, false, "/x.pem", "/x.key", "", nil, time.Now()); len(d) != 0 {
		t.Fatalf("diagnostics = %v, want none (no secure listener)", d)
	}
	d := geodnsTLSDiagnostic(true, false, "/does/not/exist.pem", "/does/not/exist.key", "", nil, time.Now())
	if len(d) != 1 || d[0].Code != "doctor-tls-missing" {
		t.Fatalf("diagnostics = %v, want one doctor-tls-missing", d)
	}
	if d := geodnsTLSDiagnostic(false, true, "", "", "", nil, time.Now()); len(d) != 0 {
		t.Fatalf("diagnostics = %v, want none (self-signed fallback)", d)
	}
}

// VALIDATES: the listen-capability check warns only for an enabled geodns on a
// privileged port (<1024) that cannot be bound; disabled geodns, non-privileged
// ports, and bindable privileged ports produce no diagnostic.
// PREVENTS: a false positive on the default port 5300, and a missed
// CAP_NET_BIND_SERVICE warning when an operator moves geodns onto port 53.
func TestGeodnsListenDiagnostic(t *testing.T) {
	t.Parallel()
	bindOK := func(string, int) bool { return true }
	bindFail := func(string, int) bool { return false }
	priv := []probeTarget{{host: "127.0.0.1", port: 53}, {host: "::1", port: 53}}
	unpriv := []probeTarget{{host: "127.0.0.1", port: 5300}}

	if d := geodnsListenDiagnostic(false, priv, bindFail); d != nil {
		t.Errorf("disabled geodns should produce no diagnostic, got %v", d)
	}
	if d := geodnsListenDiagnostic(true, unpriv, bindFail); d != nil {
		t.Errorf("non-privileged port should produce no diagnostic, got %v", d)
	}
	if d := geodnsListenDiagnostic(true, priv, bindOK); d != nil {
		t.Errorf("bindable privileged port should produce no diagnostic, got %v", d)
	}
	d := geodnsListenDiagnostic(true, priv, bindFail)
	if len(d) != 1 || d[0].Code != "doctor-geodns-port-unavailable" || d[0].Severity != "warning" {
		t.Errorf("expected a port-unavailable warning, got %v", d)
	}
	// Boundary: 1023 is the last privileged port (checked), 1024 is the first
	// unprivileged port (skipped).
	if d := geodnsListenDiagnostic(true, []probeTarget{{host: "127.0.0.1", port: 1023}}, bindFail); len(d) != 1 {
		t.Errorf("port 1023 (privileged boundary) should warn, got %v", d)
	}
	if d := geodnsListenDiagnostic(true, []probeTarget{{host: "127.0.0.1", port: 1024}}, bindFail); d != nil {
		t.Errorf("port 1024 (first unprivileged) should not warn, got %v", d)
	}
}

// TestGeoDNSTLSDiagnosticPKIReference validates AC-8: a tls container naming a PKI
// store certificate is checked against the pki block of the SAME tree, offline,
// before the config is committed.
// PREVENTS: an operator discovering a typo'd certificate name only when the DoT
// listener fails to start on the next reload.
func TestGeoDNSTLSDiagnosticPKIReference(t *testing.T) {
	now := time.Now()
	cfg := pkiRefTestConfig(t, now.Add(365*24*time.Hour))

	t.Run("healthy reference is clean", func(t *testing.T) {
		if d := geodnsTLSDiagnostic(true, false, "", "", "svc-cert", cfg, now); len(d) != 0 {
			t.Fatalf("diagnostics = %v, want none", d)
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		d := geodnsTLSDiagnostic(true, false, "", "", "typo", cfg, now)
		if len(d) != 1 || d[0].Code != "doctor-tls-reference" || d[0].Severity != "error" {
			t.Fatalf("diagnostics = %v, want one doctor-tls-reference error", d)
		}
	})

	t.Run("keyless entry", func(t *testing.T) {
		d := geodnsTLSDiagnostic(false, true, "", "", "svc-keyless", cfg, now)
		if len(d) != 1 || d[0].Code != "doctor-tls-reference" {
			t.Fatalf("diagnostics = %v, want one doctor-tls-reference", d)
		}
	})

	t.Run("expired certificate", func(t *testing.T) {
		expired := pkiRefTestConfig(t, now.Add(-time.Hour))
		d := geodnsTLSDiagnostic(true, false, "", "", "svc-cert", expired, now)
		if len(d) == 0 || d[0].Code != "doctor-tls-expired" {
			t.Fatalf("diagnostics = %v, want doctor-tls-expired", d)
		}
	})

	t.Run("no secure listener means no check", func(t *testing.T) {
		// A reference on a container whose DoT and DoH are both off serves
		// nothing, so it raises nothing.
		if d := geodnsTLSDiagnostic(false, false, "", "", "typo", cfg, now); len(d) != 0 {
			t.Fatalf("diagnostics = %v, want none", d)
		}
	})
}

// pkiRefTestConfig builds a parsed pki config holding a CA, a chain-bearing
// certificate named svc-cert, and a keyless entry svc-keyless.
func pkiRefTestConfig(t *testing.T, notAfter time.Time) *pki.PKIConfig {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "svc ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "svc leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}

	return &pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"svc-ca": {Name: "svc-ca", Certificate: caCert, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			"svc-cert":    {Name: "svc-cert", Certificate: leafCert, Raw: leafDER, PrivateKey: leafKey},
			"svc-keyless": {Name: "svc-keyless", Certificate: leafCert, Raw: leafDER},
		},
	}
}
