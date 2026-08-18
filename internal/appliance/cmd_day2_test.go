package appliance

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswdUpdatesHash(t *testing.T) {
	dir := initTestAppliance(t, "pw", nil)
	baseDir = dir

	hashBefore, err := os.ReadFile(filepath.Join(dir, "pw", "secrets", "password.hash"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "new-password")
	env.ResetCache()
	code := runPasswd([]string{"pw"})
	if code != exitOK {
		t.Fatalf("passwd returned %d", code)
	}

	hashAfter, err := os.ReadFile(filepath.Join(dir, "pw", "secrets", "password.hash"))
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(hashBefore, hashAfter) {
		t.Error("password hash should change")
	}
	if err := bcrypt.CompareHashAndPassword(hashAfter, []byte("new-password")); err != nil {
		t.Errorf("new hash does not match new password: %v", err)
	}
}

// makeCertPair builds a self-signed ECDSA pair with the validity window the
// caller asks for. Every replace-cert test needs material the standard library
// accepts or rejects for one specific reason, so the window is a parameter.
func makeCertPair(t *testing.T, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// writeCertPair drops PEM material into a fresh directory and returns the paths
// the operator would pass to --cert and --key.
func writeCertPair(t *testing.T, dir string, certPEM, keyPEM []byte) (certPath, keyPath string) {
	t.Helper()
	src := filepath.Join(dir, "ca-certs")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(src, "ca.pem")
	keyPath = filepath.Join(src, "ca.key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

// applianceTLSPaths returns the appliance's stored certificate and key paths.
func applianceTLSPaths(dir, name string) (certPath, keyPath string) {
	return filepath.Join(dir, name, "secrets", "tls", "cert.pem"),
		filepath.Join(dir, name, "secrets", "tls", "key.pem")
}

// readTLSMaterial returns the bytes currently stored for the appliance, so a
// refusal can be checked against what was there before it.
func readTLSMaterial(t *testing.T, dir, name string) (cert, key []byte) {
	t.Helper()
	certPath, keyPath := applianceTLSPaths(dir, name)
	cert, err := os.ReadFile(certPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	key, err = os.ReadFile(keyPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func TestReplaceCertUpdatesSecrets(t *testing.T) {
	dir := initTestAppliance(t, "cert", nil)
	baseDir = dir

	certPEM, keyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, keyPath := writeCertPair(t, dir, certPEM, keyPEM)

	code := runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "cert"})
	if code != exitOK {
		t.Fatalf("replace-cert returned %d", code)
	}

	storedCert, storedKey := readTLSMaterial(t, dir, "cert")
	if !bytes.Equal(storedCert, certPEM) {
		t.Error("stored certificate should be the supplied certificate")
	}
	if !bytes.Equal(storedKey, keyPEM) {
		t.Error("stored key should be the supplied key")
	}
}

// TestReplaceCertRefusesMismatchedPair covers AC-1: a certificate and a key
// from two different pairs are refused and neither stored file changes.
func TestReplaceCertRefusesMismatchedPair(t *testing.T) {
	dir := initTestAppliance(t, "mismatch", nil)
	baseDir = dir

	certPEM, _ := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	_, otherKeyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, keyPath := writeCertPair(t, dir, certPEM, otherKeyPEM)

	certBefore, keyBefore := readTLSMaterial(t, dir, "mismatch")

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "mismatch"})
	})
	if code == exitOK {
		t.Fatal("replace-cert accepted a mismatched certificate and key")
	}
	if !strings.Contains(stderr, "are not a pair") {
		t.Errorf("error should name the mismatch, got: %s", stderr)
	}
	if !strings.Contains(stderr, certPath) || !strings.Contains(stderr, keyPath) {
		t.Errorf("error should name both files, got: %s", stderr)
	}

	certAfter, keyAfter := readTLSMaterial(t, dir, "mismatch")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("stored certificate changed despite the refusal")
	}
	if !bytes.Equal(keyBefore, keyAfter) {
		t.Error("stored key changed despite the refusal")
	}
}

// TestReplaceCertRefusesUnparseablePEM covers AC-2. It replaces the assertion
// this test used to make: a fake PEM body was accepted and written.
func TestReplaceCertRefusesUnparseablePEM(t *testing.T) {
	dir := initTestAppliance(t, "unparseable", nil)
	baseDir = dir

	certPath, keyPath := writeCertPair(t, dir,
		[]byte("-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n"),
		[]byte("-----BEGIN EC PRIVATE KEY-----\nfake\n-----END EC PRIVATE KEY-----\n"))

	certBefore, keyBefore := readTLSMaterial(t, dir, "unparseable")

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "unparseable"})
	})
	if code == exitOK {
		t.Fatal("replace-cert accepted material that does not parse")
	}
	if !strings.Contains(stderr, certPath) {
		t.Errorf("error should name the certificate file, got: %s", stderr)
	}

	certAfter, keyAfter := readTLSMaterial(t, dir, "unparseable")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("stored certificate changed despite the refusal")
	}
	if !bytes.Equal(keyBefore, keyAfter) {
		t.Error("stored key changed despite the refusal")
	}
}

// TestReplaceCertRefusesEmptyFile covers the zero-PEM-blocks boundary: an empty
// certificate file is named rather than reported as a pair mismatch.
func TestReplaceCertRefusesEmptyFile(t *testing.T) {
	dir := initTestAppliance(t, "empty", nil)
	baseDir = dir

	_, keyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, keyPath := writeCertPair(t, dir, nil, keyPEM)

	certBefore, _ := readTLSMaterial(t, dir, "empty")

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "empty"})
	})
	if code == exitOK {
		t.Fatal("replace-cert accepted an empty certificate file")
	}
	if !strings.Contains(stderr, "holds no PEM data") {
		t.Errorf("error should say the file holds no PEM data, got: %s", stderr)
	}

	certAfter, _ := readTLSMaterial(t, dir, "empty")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("stored certificate changed despite the refusal")
	}
}

// TestReplaceCertRestoresOnKeyWriteFailure covers AC-3. A directory standing
// where WriteSecret wants its temp file makes the key write fail after the
// certificate has already been replaced.
func TestReplaceCertRestoresOnKeyWriteFailure(t *testing.T) {
	dir := initTestAppliance(t, "restore", nil)
	baseDir = dir

	certPEM, keyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, keyPath := writeCertPair(t, dir, certPEM, keyPEM)

	certBefore, keyBefore := readTLSMaterial(t, dir, "restore")

	storedCert, storedKey := applianceTLSPaths(dir, "restore")
	if err := os.Mkdir(storedKey+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "restore"})
	})
	if code == exitOK {
		t.Fatal("replace-cert reported success despite a failed key write")
	}
	if !strings.Contains(stderr, "write key") {
		t.Errorf("error should report the key write failure, got: %s", stderr)
	}
	if !strings.Contains(stderr, "unchanged") {
		t.Errorf("error should report the restore outcome, got: %s", stderr)
	}

	certAfter, keyAfter := readTLSMaterial(t, dir, "restore")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("previous certificate was not restored after the key write failed")
	}
	if !bytes.Equal(keyBefore, keyAfter) {
		t.Error("stored key changed despite the failed write")
	}
	if _, err := os.Stat(storedCert + ".tmp"); err == nil {
		t.Error("certificate temp file left behind")
	}
}

// TestReplaceCertLeavesNoTempFile covers AC-4 and validates A-1: the temp file
// and rename happen in the secrets directory and leave nothing behind.
func TestReplaceCertLeavesNoTempFile(t *testing.T) {
	dir := initTestAppliance(t, "tidy", nil)
	baseDir = dir

	certPEM, keyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, keyPath := writeCertPair(t, dir, certPEM, keyPEM)

	code := runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "tidy"})
	if code != exitOK {
		t.Fatalf("replace-cert returned %d", code)
	}

	tlsDir := filepath.Join(dir, "tidy", "secrets", "tls")
	entries, err := os.ReadDir(tlsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}

	storedCert, storedKey := applianceTLSPaths(dir, "tidy")
	for _, path := range []string{storedCert, storedKey} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want -rw-------", path, info.Mode().Perm())
		}
	}
}

// TestReplaceCertWritesCertificateThroughTempFile covers AC-7. A directory
// standing where the certificate's temp file belongs proves the write goes
// through a temp file and a rename: a truncating write would not notice it, and
// a truncating write is the state in which an interrupted run leaves cert.pem
// half written.
func TestReplaceCertWritesCertificateThroughTempFile(t *testing.T) {
	dir := initTestAppliance(t, "atomic", nil)
	baseDir = dir

	certPEM, keyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, keyPath := writeCertPair(t, dir, certPEM, keyPEM)

	storedCert, _ := applianceTLSPaths(dir, "atomic")
	certBefore, keyBefore := readTLSMaterial(t, dir, "atomic")
	if err := os.Mkdir(storedCert+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "atomic"})
	})
	if code == exitOK {
		t.Fatal("the certificate was written without going through its temp file")
	}
	if !strings.Contains(stderr, "cert.pem.tmp") {
		t.Errorf("error should name the temp file the write uses, got: %s", stderr)
	}

	certAfter, keyAfter := readTLSMaterial(t, dir, "atomic")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("stored certificate changed despite the failed write")
	}
	if !bytes.Equal(keyBefore, keyAfter) {
		t.Error("stored key changed despite the failed certificate write")
	}
}

// TestReplaceCertExpiredCertificate covers AC-5 and pins the A-3 answer: a
// certificate past its not-after date is refused with both dates named, while a
// certificate whose validity starts in the future is accepted, because staging
// a renewal ahead of its start date is a supported workflow.
func TestReplaceCertExpiredCertificate(t *testing.T) {
	dir := initTestAppliance(t, "expiry", nil)
	baseDir = dir

	expiredCert, expiredKey := makeCertPair(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))
	certPath, keyPath := writeCertPair(t, dir, expiredCert, expiredKey)

	certBefore, keyBefore := readTLSMaterial(t, dir, "expiry")

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "--key", keyPath, "expiry"})
	})
	if code == exitOK {
		t.Fatal("replace-cert accepted an expired certificate")
	}
	if !strings.Contains(stderr, "expired on") || !strings.Contains(stderr, "valid from") {
		t.Errorf("error should name both validity dates, got: %s", stderr)
	}

	certAfter, keyAfter := readTLSMaterial(t, dir, "expiry")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("stored certificate changed despite the refusal")
	}
	if !bytes.Equal(keyBefore, keyAfter) {
		t.Error("stored key changed despite the refusal")
	}

	// A certificate that has not yet reached not-after is still valid material.
	// The window is 30 seconds rather than one: a one-second window would report
	// how loaded the machine was, not which branch the code took.
	edgeCert, edgeKey := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(30*time.Second))
	edgeCertPath, edgeKeyPath := writeCertPair(t, dir, edgeCert, edgeKey)
	if runReplaceCert([]string{"--cert", edgeCertPath, "--key", edgeKeyPath, "expiry"}) != exitOK {
		t.Error("a certificate that has not yet reached not-after should be accepted")
	}

	// A staged renewal starting tomorrow is accepted (A-3).
	futureCert, futureKey := makeCertPair(t, time.Now().Add(24*time.Hour), time.Now().Add(48*time.Hour))
	futureCertPath, futureKeyPath := writeCertPair(t, dir, futureCert, futureKey)
	if runReplaceCert([]string{"--cert", futureCertPath, "--key", futureKeyPath, "expiry"}) != exitOK {
		t.Error("a certificate whose validity starts in the future should be accepted")
	}
}

// TestReplaceCertRequiresBothFlags refuses half a replacement. Accepting --cert
// alone used to fall through to self-signed regeneration, which destroyed the
// material the operator was trying to keep.
func TestReplaceCertRequiresBothFlags(t *testing.T) {
	dir := initTestAppliance(t, "halfflag", nil)
	baseDir = dir

	certPEM, keyPEM := makeCertPair(t, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	certPath, _ := writeCertPair(t, dir, certPEM, keyPEM)

	certBefore, _ := readTLSMaterial(t, dir, "halfflag")

	var code int
	stderr := captureStderr(t, func() {
		code = runReplaceCert([]string{"--cert", certPath, "halfflag"})
	})
	if code == exitOK {
		t.Fatal("replace-cert accepted --cert without --key")
	}
	if !strings.Contains(stderr, "--cert and --key must be given together") {
		t.Errorf("error should name both flags, got: %s", stderr)
	}

	certAfter, _ := readTLSMaterial(t, dir, "halfflag")
	if !bytes.Equal(certBefore, certAfter) {
		t.Error("stored certificate was regenerated instead of the command refusing")
	}
}

func TestReplaceCertRegenerates(t *testing.T) {
	dir := initTestAppliance(t, "regen", nil)
	baseDir = dir

	certBefore, _ := os.ReadFile(filepath.Join(dir, "regen", "secrets", "tls", "cert.pem"))

	code := runReplaceCert([]string{"regen"})
	if code != exitOK {
		t.Fatalf("replace-cert returned %d", code)
	}

	certAfter, _ := os.ReadFile(filepath.Join(dir, "regen", "secrets", "tls", "cert.pem"))
	if bytes.Equal(certBefore, certAfter) {
		t.Error("regenerated cert should differ from original")
	}
}

func TestCloneCopiesConfigNotSecrets(t *testing.T) {
	dir := initTestAppliance(t, "src", nil)
	baseDir = dir

	code := runClone([]string{"src", "dst"})
	if code != exitOK {
		t.Fatalf("clone returned %d", code)
	}

	dstCfg, err := LoadConfig(ConfigPath(dir, "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if dstCfg.Identity.Name != "dst" {
		t.Errorf("name = %q, want dst", dstCfg.Identity.Name)
	}
	if dstCfg.Identity.Hostname != "dst" {
		t.Errorf("hostname = %q, want dst", dstCfg.Identity.Hostname)
	}

	srcCfg, _ := LoadConfig(ConfigPath(dir, "src"))
	if dstCfg.Credentials.Username != srcCfg.Credentials.Username {
		t.Error("username should be copied from source")
	}

	secretsDir := SecretsDir(dir, "dst")
	if _, err := os.Stat(secretsDir); err == nil {
		t.Error("secrets directory should not be copied")
	}
}

func TestRekeyChangesEncryption(t *testing.T) {
	dir := initTestAppliance(t, "rekey", []byte("old-pass"))
	baseDir = dir

	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "old-pass")
	t.Setenv("ZE_APPLIANCE_NEW_PASSPHRASE", "new-pass")
	env.ResetCache()
	code := runRekey([]string{"rekey"})
	if code != exitOK {
		t.Fatalf("rekey returned %d", code)
	}

	hashData, err := readSecret(secretFilePath(dir, "rekey", "password.hash"), []byte("new-pass"))
	if err != nil {
		t.Fatalf("read with new passphrase: %v", err)
	}
	if len(hashData) == 0 {
		t.Error("hash should not be empty")
	}

	_, err = readSecret(secretFilePath(dir, "rekey", "password.hash"), []byte("old-pass"))
	if err == nil {
		t.Error("old passphrase should no longer work")
	}
}

func TestRekeyPlaintextToEncrypted(t *testing.T) {
	dir := initTestAppliance(t, "toenc", nil)
	baseDir = dir

	if isEncrypted(dir, "toenc") {
		t.Fatal("should start unencrypted")
	}

	t.Setenv("ZE_APPLIANCE_NEW_PASSPHRASE", "add-pass")
	env.ResetCache()
	code := runRekey([]string{"toenc"})
	if code != exitOK {
		t.Fatalf("rekey returned %d", code)
	}

	if !isEncrypted(dir, "toenc") {
		t.Error("should be encrypted after rekey")
	}
}

func TestRekeyEncryptedToPlaintext(t *testing.T) {
	dir := initTestAppliance(t, "toplain", []byte("has-pass"))
	baseDir = dir

	if !isEncrypted(dir, "toplain") {
		t.Fatal("should start encrypted")
	}

	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "has-pass")
	t.Setenv("ZE_APPLIANCE_NEW_PASSPHRASE", "")
	env.ResetCache()
	code := runRekey([]string{"toplain"})
	if code != exitOK {
		t.Fatalf("rekey returned %d", code)
	}

	if isEncrypted(dir, "toplain") {
		t.Error("should be plaintext after rekey with empty passphrase")
	}

	hashData, err := readSecret(secretFilePath(dir, "toplain", "password.hash"), nil)
	if err != nil {
		t.Fatalf("read plaintext hash: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hashData, []byte("test-password")); err != nil {
		t.Errorf("plaintext hash should match original password: %v", err)
	}
}
