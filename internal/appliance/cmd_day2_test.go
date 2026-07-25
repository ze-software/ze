package appliance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestReplaceCertUpdatesSecrets(t *testing.T) {
	dir := initTestAppliance(t, "cert", nil)
	baseDir = dir

	certDir := filepath.Join(dir, "ca-certs")
	os.MkdirAll(certDir, 0o700)                                                                                                           //nolint:errcheck // test
	os.WriteFile(filepath.Join(certDir, "ca.pem"), []byte("-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n"), 0o600)       //nolint:errcheck,gosec // test
	os.WriteFile(filepath.Join(certDir, "ca.key"), []byte("-----BEGIN EC PRIVATE KEY-----\nfake\n-----END EC PRIVATE KEY-----\n"), 0o600) //nolint:errcheck,gosec // test

	code := runReplaceCert([]string{
		"--cert", filepath.Join(certDir, "ca.pem"),
		"--key", filepath.Join(certDir, "ca.key"),
		"cert",
	})
	if code != exitOK {
		t.Fatalf("replace-cert returned %d", code)
	}

	certData, err := os.ReadFile(filepath.Join(dir, "cert", "secrets", "tls", "cert.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(certData), "fake") {
		t.Error("cert should contain the provided CA cert content")
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

	hashData, err := ReadSecret(secretFilePath(dir, "rekey", "password.hash"), []byte("new-pass"))
	if err != nil {
		t.Fatalf("read with new passphrase: %v", err)
	}
	if len(hashData) == 0 {
		t.Error("hash should not be empty")
	}

	_, err = ReadSecret(secretFilePath(dir, "rekey", "password.hash"), []byte("old-pass"))
	if err == nil {
		t.Error("old passphrase should no longer work")
	}
}

func TestRekeyPlaintextToEncrypted(t *testing.T) {
	dir := initTestAppliance(t, "toenc", nil)
	baseDir = dir

	if IsEncrypted(dir, "toenc") {
		t.Fatal("should start unencrypted")
	}

	t.Setenv("ZE_APPLIANCE_NEW_PASSPHRASE", "add-pass")
	env.ResetCache()
	code := runRekey([]string{"toenc"})
	if code != exitOK {
		t.Fatalf("rekey returned %d", code)
	}

	if !IsEncrypted(dir, "toenc") {
		t.Error("should be encrypted after rekey")
	}
}

func TestRekeyEncryptedToPlaintext(t *testing.T) {
	dir := initTestAppliance(t, "toplain", []byte("has-pass"))
	baseDir = dir

	if !IsEncrypted(dir, "toplain") {
		t.Fatal("should start encrypted")
	}

	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "has-pass")
	t.Setenv("ZE_APPLIANCE_NEW_PASSPHRASE", "")
	env.ResetCache()
	code := runRekey([]string{"toplain"})
	if code != exitOK {
		t.Fatalf("rekey returned %d", code)
	}

	if IsEncrypted(dir, "toplain") {
		t.Error("should be plaintext after rekey with empty passphrase")
	}

	hashData, err := ReadSecret(secretFilePath(dir, "toplain", "password.hash"), nil)
	if err != nil {
		t.Fatalf("read plaintext hash: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hashData, []byte("test-password")); err != nil {
		t.Errorf("plaintext hash should match original password: %v", err)
	}
}
