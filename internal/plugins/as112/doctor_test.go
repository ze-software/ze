package as112

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

// VALIDATES: AC-3 -- no cert diagnostic when neither DoT nor DoH is enabled.
func TestAS112TLSDiagnostic_DisabledSecure(t *testing.T) {
	if d := as112TLSDiagnostic(false, false, "/x.pem", "/x.key", "", nil, time.Now()); len(d) != 0 {
		t.Fatalf("diagnostics = %v, want none (no secure listener)", d)
	}
}

// VALIDATES: AC-3 -- with DoT enabled and a missing cert file, the shared
// cert check surfaces doctor-tls-missing.
func TestAS112TLSDiagnostic_MissingCert(t *testing.T) {
	d := as112TLSDiagnostic(true, false, "/does/not/exist.pem", "/does/not/exist.key", "", nil, time.Now())
	if len(d) != 1 || d[0].Code != "doctor-tls-missing" {
		t.Fatalf("diagnostics = %v, want one doctor-tls-missing", d)
	}
}

// VALIDATES: AC-3 -- self-signed fallback (no cert files) with DoH enabled is
// not a diagnostic.
func TestAS112TLSDiagnostic_SelfSigned(t *testing.T) {
	if d := as112TLSDiagnostic(false, true, "", "", "", nil, time.Now()); len(d) != 0 {
		t.Fatalf("diagnostics = %v, want none (self-signed fallback)", d)
	}
}

// VALIDATES: AC-7 -- as112 enabled on a privileged port the process cannot
// bind produces the doctor-as112-port-unavailable diagnostic.
func TestAS112ListenDiagnostic_UnbindablePrivilegedPort(t *testing.T) {
	diags := as112ListenDiagnostic(true, addressFamilyBoth, func(string, int) bool { return false })
	if len(diags) != 1 || diags[0].Code != "doctor-as112-port-unavailable" {
		t.Fatalf("diagnostics = %v, want exactly one doctor-as112-port-unavailable", diags)
	}
}

// VALIDATES: no diagnostic when the port IS bindable.
func TestAS112ListenDiagnostic_Bindable(t *testing.T) {
	diags := as112ListenDiagnostic(true, addressFamilyBoth, func(string, int) bool { return true })
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none (port bindable)", diags)
	}
}

// VALIDATES: no diagnostic when as112 is disabled, regardless of bindability.
func TestAS112ListenDiagnostic_Disabled(t *testing.T) {
	diags := as112ListenDiagnostic(false, addressFamilyBoth, func(string, int) bool { return false })
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none (as112 disabled)", diags)
	}
}

// VALIDATES: an ipv6-only node probes the IPv6 wildcard, never IPv4 -- a v4
// bind failure (or success) must not influence a v6-only node's diagnostic.
func TestAS112ListenDiagnostic_IPv6OnlyProbesIPv6Only(t *testing.T) {
	var probed []string
	as112ListenDiagnostic(true, addressFamilyIPv6Only, func(host string, _ int) bool {
		probed = append(probed, host)
		return true
	})
	if len(probed) != 1 || probed[0] != "::" {
		t.Fatalf("probed hosts = %v, want exactly [\"::\"]", probed)
	}
}

// VALIDATES: an ipv4-only node probes the IPv4 wildcard, never IPv6.
func TestAS112ListenDiagnostic_IPv4OnlyProbesIPv4Only(t *testing.T) {
	var probed []string
	as112ListenDiagnostic(true, addressFamilyIPv4Only, func(host string, _ int) bool {
		probed = append(probed, host)
		return true
	})
	if len(probed) != 1 || probed[0] != "0.0.0.0" {
		t.Fatalf("probed hosts = %v, want exactly [\"0.0.0.0\"]", probed)
	}
}

// VALIDATES: "both" probes both wildcards, and an IPv6-only bind failure
// (IPv4 bindable) still produces the diagnostic -- neither family alone
// gives false confidence for a dual-stack node.
func TestAS112ListenDiagnostic_BothProbesBothFamilies(t *testing.T) {
	var probed []string
	diags := as112ListenDiagnostic(true, addressFamilyBoth, func(host string, _ int) bool {
		probed = append(probed, host)
		return host != "::" // IPv6 unbindable, IPv4 bindable.
	})
	if len(probed) != 2 || probed[0] != "0.0.0.0" || probed[1] != "::" {
		t.Fatalf("probed hosts = %v, want [\"0.0.0.0\" \"::\"]", probed)
	}
	if len(diags) != 1 || diags[0].Code != "doctor-as112-port-unavailable" {
		t.Fatalf("diagnostics = %v, want exactly one doctor-as112-port-unavailable (IPv6 unbindable)", diags)
	}
}

// TestAS112TLSDiagnosticPKIReference validates AC-8: a tls container naming a PKI
// store certificate is checked against the pki block of the SAME tree, offline,
// before the config is committed.
// PREVENTS: an operator discovering a typo'd certificate name only when the DoT
// listener fails to start on the next reload.
func TestAS112TLSDiagnosticPKIReference(t *testing.T) {
	now := time.Now()
	cfg := pkiRefTestConfig(t, now.Add(365*24*time.Hour))

	t.Run("healthy reference is clean", func(t *testing.T) {
		if d := as112TLSDiagnostic(true, false, "", "", "svc-cert", cfg, now); len(d) != 0 {
			t.Fatalf("diagnostics = %v, want none", d)
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		d := as112TLSDiagnostic(true, false, "", "", "typo", cfg, now)
		if len(d) != 1 || d[0].Code != "doctor-tls-reference" || d[0].Severity != "error" {
			t.Fatalf("diagnostics = %v, want one doctor-tls-reference error", d)
		}
	})

	t.Run("keyless entry", func(t *testing.T) {
		d := as112TLSDiagnostic(false, true, "", "", "svc-keyless", cfg, now)
		if len(d) != 1 || d[0].Code != "doctor-tls-reference" {
			t.Fatalf("diagnostics = %v, want one doctor-tls-reference", d)
		}
	})

	t.Run("expired certificate", func(t *testing.T) {
		expired := pkiRefTestConfig(t, now.Add(-time.Hour))
		d := as112TLSDiagnostic(true, false, "", "", "svc-cert", expired, now)
		if len(d) == 0 || d[0].Code != "doctor-tls-expired" {
			t.Fatalf("diagnostics = %v, want doctor-tls-expired", d)
		}
	})

	t.Run("no secure listener means no check", func(t *testing.T) {
		// A reference on a container whose DoT and DoH are both off serves
		// nothing, so it raises nothing.
		if d := as112TLSDiagnostic(false, false, "", "", "typo", cfg, now); len(d) != 0 {
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
