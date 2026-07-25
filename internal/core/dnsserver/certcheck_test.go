package dnsserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/selfcert"
)

func writeCertPair(t *testing.T, validity time.Duration) (certFile, keyFile string) {
	t.Helper()
	dir := t.TempDir()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1", nil, validity)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	if werr := os.WriteFile(certFile, certPEM, 0o600); werr != nil {
		t.Fatalf("write cert: %v", werr)
	}
	if werr := os.WriteFile(keyFile, keyPEM, 0o600); werr != nil {
		t.Fatalf("write key: %v", werr)
	}
	return certFile, keyFile
}

// VALIDATES: AC-3 -- a valid, in-window certificate produces no doctor problems.
func TestCheckCertMaterialValid(t *testing.T) {
	cf, kf := writeCertPair(t, 40*24*time.Hour)
	if p := CheckCertMaterial(cf, kf, time.Now()); len(p) != 0 {
		t.Fatalf("valid cert produced problems: %+v", p)
	}
}

// VALIDATES: AC-3 -- an empty cert/key pair (self-signed fallback) is not a
// doctor problem: there is nothing operator-supplied to validate.
func TestCheckCertMaterialSelfSigned(t *testing.T) {
	if p := CheckCertMaterial("", "", time.Now()); len(p) != 0 {
		t.Fatalf("self-signed fallback produced problems: %+v", p)
	}
}

// VALIDATES: AC-3 -- a half-configured pair is flagged doctor-tls-invalid.
func TestCheckCertMaterialHalfConfigured(t *testing.T) {
	p := CheckCertMaterial("/x/cert.pem", "", time.Now())
	if len(p) != 1 || p[0].Code != "doctor-tls-invalid" {
		t.Fatalf("half-config: got %+v, want doctor-tls-invalid", p)
	}
}

// VALIDATES: AC-3 -- a missing cert file is flagged doctor-tls-missing.
func TestCheckCertMaterialMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := CheckCertMaterial(filepath.Join(dir, "absent.pem"), filepath.Join(dir, "absent.key"), time.Now())
	if len(p) != 1 || p[0].Code != "doctor-tls-missing" {
		t.Fatalf("missing file: got %+v, want doctor-tls-missing", p)
	}
}

// VALIDATES: AC-3 -- an expired certificate is flagged doctor-tls-expired
// (error), and a near-expiry one is a warning.
func TestCheckCertMaterialExpiry(t *testing.T) {
	cf, kf := writeCertPair(t, 40*24*time.Hour)
	now := time.Now()

	// 50 days after issuance: past NotAfter -> error.
	expired := CheckCertMaterial(cf, kf, now.Add(50*24*time.Hour))
	if len(expired) != 1 || expired[0].Code != "doctor-tls-expired" || expired[0].Severity != "error" {
		t.Fatalf("expired: got %+v, want doctor-tls-expired error", expired)
	}

	// 20 days after issuance: 20 days left (< 30) -> warning.
	soon := CheckCertMaterial(cf, kf, now.Add(20*24*time.Hour))
	if len(soon) != 1 || soon[0].Code != "doctor-tls-expired" || soon[0].Severity != "warning" {
		t.Fatalf("near-expiry: got %+v, want doctor-tls-expired warning", soon)
	}
}

// VALIDATES: AC-3 -- malformed PEM is flagged doctor-tls-invalid.
func TestCheckCertMaterialBadPEM(t *testing.T) {
	dir := t.TempDir()
	cf := filepath.Join(dir, "cert.pem")
	kf := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cf, []byte("not a pem"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(kf, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	p := CheckCertMaterial(cf, kf, time.Now())
	if len(p) != 1 || p[0].Code != "doctor-tls-invalid" {
		t.Fatalf("bad pem: got %+v, want doctor-tls-invalid", p)
	}
}
