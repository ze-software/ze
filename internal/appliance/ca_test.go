// Design: docs/architecture/appliance/builder.md -- the appliance certificate authority

package appliance

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

// readCertFile returns every certificate in a PEM file, in the order the file
// holds them. The order is the property several tests here assert, so nothing
// sorts or filters.
func readCertFile(t *testing.T, path string) []*x509.Certificate {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != pemBlockCertificate {
			continue
		}
		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			t.Fatalf("parse certificate in %s: %v", path, parseErr)
		}
		certs = append(certs, cert)
	}
	return certs
}

// TestInitWritesLeafBeforeRoot covers AC-9's file format and R-5: the appliance
// certificate file holds the issued leaf FIRST and the root SECOND. Four
// readers take the first block only (certExpiry, validateTLSPair,
// checkCertExpiry, selfcert.NewTLSConfig on the device), so a reversed file
// hands each of them the certificate authority instead of the serving
// certificate.
func TestInitWritesLeafBeforeRoot(t *testing.T) {
	dir := initTestAppliance(t, "twoblock", nil)

	certPath := filepath.Join(dir, "twoblock", "secrets", "tls", "cert.pem")
	certs := readCertFile(t, certPath)
	if len(certs) != 2 {
		t.Fatalf("cert.pem holds %d certificates, want the leaf and the root", len(certs))
	}

	leaf, root := certs[0], certs[1]
	if leaf.IsCA {
		t.Error("the first block is a certificate authority; the serving leaf must come first")
	}
	if !root.IsCA {
		t.Error("the second block is not a certificate authority")
	}
	if err := leaf.CheckSignatureFrom(root); err != nil {
		t.Errorf("the leaf was not signed by the root in the same file: %v", err)
	}

	// certExpiry reads the FIRST block, and every operator asking when the
	// appliance certificate expires means the serving one.
	expiry, err := certExpiry(certPath)
	if err != nil {
		t.Fatalf("certExpiry: %v", err)
	}
	if !expiry.Equal(leaf.NotAfter) {
		t.Errorf("certExpiry reported %s, want the leaf's %s", expiry, leaf.NotAfter)
	}

	// The root is the appliance's own, kept for the next issuance.
	storedRoot := readCertFile(t, filepath.Join(dir, "twoblock", "secrets", "tls", caCertFileName))
	if len(storedRoot) != 1 {
		t.Fatalf("%s holds %d certificates, want 1", caCertFileName, len(storedRoot))
	}
	if storedRoot[0].SerialNumber.Cmp(root.SerialNumber) != 0 {
		t.Error("the root in cert.pem is not the appliance's stored root")
	}
}

// TestApplianceRootOutlivesItsLeaf holds the lifetime rule: tls.validity-years
// is the life of the SERVING certificate, and the root that signs it takes that
// life plus a margin. A leaf that outlived its issuer would stop verifying
// while it still claimed to be valid.
func TestApplianceRootOutlivesItsLeaf(t *testing.T) {
	dir := initTestAppliance(t, "lifetime", nil)

	cfg, err := LoadConfig(ConfigPath(dir, "lifetime"))
	if err != nil {
		t.Fatal(err)
	}

	certs := readCertFile(t, filepath.Join(dir, "lifetime", "secrets", "tls", "cert.pem"))
	if len(certs) != 2 {
		t.Fatalf("cert.pem holds %d certificates, want the leaf and the root", len(certs))
	}
	leaf, root := certs[0], certs[1]

	if !root.NotAfter.After(leaf.NotAfter) {
		t.Fatalf("root expires %s, leaf expires %s: the root must outlive the leaf", root.NotAfter, leaf.NotAfter)
	}

	// Both certificates are minted milliseconds apart, so the margin is exact
	// to well inside a minute.
	margin := root.NotAfter.Sub(leaf.NotAfter)
	if margin < applianceRootMargin-time.Minute || margin > applianceRootMargin+time.Minute {
		t.Errorf("root outlives the leaf by %s, want %s", margin, applianceRootMargin)
	}

	askedFor := time.Duration(cfg.TLS.ValidityYears) * yearDuration
	leafLife := time.Until(leaf.NotAfter)
	if leafLife < askedFor-time.Hour {
		t.Errorf("leaf lives %s, want the configured %d years (%s)", leafLife, cfg.TLS.ValidityYears, askedFor)
	}
}

// TestRekeyReEncryptsTheCertificateAuthorityKey covers the file cmd_rekey.go
// would otherwise miss. A CA key left under the old passphrase is unreadable
// after a rekey, and every later `ze appliance replace-cert` fails to load the
// root, which is exactly when the operator needs it.
func TestRekeyReEncryptsTheCertificateAuthorityKey(t *testing.T) {
	dir := initTestAppliance(t, "carekey", []byte("old-pass"))
	baseDir = dir

	rootBefore := readCertFile(t, filepath.Join(dir, "carekey", "secrets", "tls", caCertFileName))
	if len(rootBefore) != 1 {
		t.Fatalf("%s holds %d certificates, want 1", caCertFileName, len(rootBefore))
	}

	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "old-pass")
	t.Setenv("ZE_APPLIANCE_NEW_PASSPHRASE", "new-pass")
	env.ResetCache()
	if code := runRekey([]string{"carekey"}); code != exitOK {
		t.Fatalf("rekey returned %d", code)
	}

	caKeyPath := filepath.Join(dir, "carekey", "secrets", "tls", caKeyFileName)
	if _, err := readSecret(caKeyPath, []byte("new-pass")); err != nil {
		t.Fatalf("the CA key does not read under the new passphrase: %v", err)
	}
	if _, err := readSecret(caKeyPath, []byte("old-pass")); err == nil {
		t.Error("the CA key still reads under the old passphrase")
	}

	// The operator's next certificate replacement has to reach the same root.
	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "new-pass")
	env.ResetCache()
	if code := runReplaceCert([]string{"carekey"}); code != exitOK {
		t.Fatalf("replace-cert after rekey returned %d", code)
	}

	certs := readCertFile(t, filepath.Join(dir, "carekey", "secrets", "tls", "cert.pem"))
	if len(certs) != 2 {
		t.Fatalf("cert.pem holds %d certificates, want the leaf and the root", len(certs))
	}
	if certs[1].SerialNumber.Cmp(rootBefore[0].SerialNumber) != 0 {
		t.Error("replace-cert issued from a new root; every device already trusting the old one would refuse the push")
	}
}

// TestApplianceRootStoreRefusesAnUnknownKey holds the store's fail-closed rule.
// A name the store does not hold is an error, never a path a later write would
// create under the appliance's TLS directory.
func TestApplianceRootStoreRefusesAnUnknownKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(tLSDir(dir, "unknown"), secretsDirPerm); err != nil {
		t.Fatal(err)
	}
	store := newApplianceRootStore(dir, "unknown", nil)

	if _, err := store.ReadFile(zefs.KeyWebKey.Pattern); err == nil {
		t.Error("the store read a key it does not hold")
	}
	if err := store.WriteFile(zefs.KeyWebKey.Pattern, []byte("material"), 0o600); err == nil {
		t.Error("the store wrote a key it does not hold")
	}
	if store.Exists(zefs.KeyWebKey.Pattern) {
		t.Error("the store claims to hold a key it does not")
	}

	// The two keys it does hold answer for the file behind them.
	if store.Exists(zefs.KeyCACert.Pattern) {
		t.Error("the store reports a root certificate before one is written")
	}
	if err := store.WriteFile(zefs.KeyCACert.Pattern, []byte("root material"), 0o600); err != nil {
		t.Fatalf("write the root certificate: %v", err)
	}
	if !store.Exists(zefs.KeyCACert.Pattern) {
		t.Error("the store does not report the root certificate it just wrote")
	}
	stored, err := store.ReadFile(zefs.KeyCACert.Pattern)
	if err != nil {
		t.Fatalf("read the root certificate back: %v", err)
	}
	if string(stored) != "root material" {
		t.Errorf("read back %q, want the bytes written", stored)
	}
}
