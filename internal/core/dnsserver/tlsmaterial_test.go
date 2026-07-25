package dnsserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// VALIDATES: AC-3 -- with no operator cert/key files, LoadTLSMaterial falls back
// to an ephemeral self-signed certificate (the selfcert precedent) so DoT/DoH
// can still come up in a dev/test deployment.
func TestLoadTLSMaterialSelfSigned(t *testing.T) {
	cfg, err := LoadTLSMaterial("", "", []string{"127.0.0.1"}, testLogger())
	if err != nil {
		t.Fatalf("LoadTLSMaterial self-signed: %v", err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %+v", cfg)
	}
}

// VALIDATES: AC-3 -- operator-provided PEM files are loaded verbatim.
func TestLoadTLSMaterialFiles(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1", nil, 0)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if werr := os.WriteFile(certFile, certPEM, 0o600); werr != nil {
		t.Fatalf("write cert: %v", werr)
	}
	if werr := os.WriteFile(keyFile, keyPEM, 0o600); werr != nil {
		t.Fatalf("write key: %v", werr)
	}
	cfg, err := LoadTLSMaterial(certFile, keyFile, nil, testLogger())
	if err != nil {
		t.Fatalf("LoadTLSMaterial files: %v", err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %+v", cfg)
	}
}

// VALIDATES: AC-3 -- a half-configured pair (cert without key) is rejected, so a
// mis-set config fails loudly instead of silently using a self-signed cert.
func TestLoadTLSMaterialMismatch(t *testing.T) {
	if _, err := LoadTLSMaterial("/tmp/cert.pem", "", nil, testLogger()); err == nil {
		t.Fatalf("expected error for cert-file without key-file")
	}
	if _, err := LoadTLSMaterial("", "/tmp/key.pem", nil, testLogger()); err == nil {
		t.Fatalf("expected error for key-file without cert-file")
	}
}

// VALIDATES: AC-3 -- a configured but missing cert file surfaces a read error.
func TestLoadTLSMaterialMissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadTLSMaterial(filepath.Join(dir, "absent.pem"), filepath.Join(dir, "absent-key.pem"), nil, testLogger()); err == nil {
		t.Fatalf("expected error for missing cert file")
	}
}
