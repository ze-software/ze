package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/env"
)

func initTestAppliance(t *testing.T, name string, passphrase []byte) string {
	t.Helper()
	dir := t.TempDir()
	baseDir = dir

	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "test-password")
	if len(passphrase) > 0 {
		t.Setenv("ZE_APPLIANCE_PASSPHRASE", string(passphrase))
	}
	env.ResetCache()

	cfg := DefaultConfig(name)
	cfgPath := filepath.Join(dir, "input.json")
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0o644) //nolint:errcheck,gosec // test

	args := []string{"--config", cfgPath, name}
	code := runInit(args)
	if code != exitOK {
		t.Fatalf("init returned %d, want 0", code)
	}
	return dir
}

func TestInitCreatesApplianceDir(t *testing.T) {
	dir := initTestAppliance(t, "lab", nil)

	appDir := filepath.Join(dir, "lab")
	if _, err := os.Stat(appDir); err != nil {
		t.Fatalf("appliance dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "appliance.json")); err != nil {
		t.Fatalf("appliance.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "secrets")); err != nil {
		t.Fatalf("secrets dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "secrets", "tls", "cert.pem")); err != nil {
		t.Fatalf("cert.pem missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "secrets", "password.hash")); err != nil {
		t.Fatalf("password.hash missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "secrets", "update.token")); err != nil {
		t.Fatalf("update.token missing: %v", err)
	}
}

func TestInitPasswordNeverInJSON(t *testing.T) {
	dir := initTestAppliance(t, "lab", nil)

	data, err := os.ReadFile(filepath.Join(dir, "lab", "appliance.json"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "test-password") {
		t.Error("appliance.json must not contain the password")
	}
	if strings.Contains(content, "password") {
		t.Error("appliance.json must not have a password field")
	}
}

func TestInitGeneratesCert(t *testing.T) {
	dir := initTestAppliance(t, "lab", nil)

	certPath := filepath.Join(dir, "lab", "secrets", "tls", "cert.pem")
	certData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(certData), "BEGIN CERTIFICATE") {
		t.Error("cert.pem should contain PEM certificate")
	}

	keyPath := filepath.Join(dir, "lab", "secrets", "tls", "key.pem")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(keyData) == 0 {
		t.Error("key.pem should not be empty")
	}
}

func TestInitFromConfigFile(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	cfg := DefaultConfig("fromfile")
	cfg.Credentials.Username = "operator"
	cfg.Managed = true
	cfgPath := filepath.Join(dir, "input.json")
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0o644) //nolint:errcheck,gosec // test

	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "pw")
	env.ResetCache()
	code := runInit([]string{"--config", cfgPath, "fromfile"})
	if code != exitOK {
		t.Fatalf("init returned %d", code)
	}

	loaded, err := LoadConfig(ConfigPath(dir, "fromfile"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Credentials.Username != "operator" {
		t.Errorf("username = %q, want operator", loaded.Credentials.Username)
	}
	if !loaded.Managed {
		t.Error("managed should be true")
	}
}

func TestInitManagedFlag(t *testing.T) {
	dir := initTestAppliance(t, "managed-test", nil)

	loaded, err := LoadConfig(ConfigPath(dir, "managed-test"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Managed {
		t.Error("managed should be false by default")
	}
}

func TestEncryptedInitCreatesMarker(t *testing.T) {
	dir := initTestAppliance(t, "enc", []byte("my-passphrase"))

	if !isEncrypted(dir, "enc") {
		t.Error("should have .encrypted marker")
	}

	keyPath := filepath.Join(dir, "enc", "secrets", "tls", "key.pem")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(keyData), "BEGIN") {
		t.Error("key.pem should be encrypted, not plaintext PEM")
	}

	certPath := filepath.Join(dir, "enc", "secrets", "tls", "cert.pem")
	certData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(certData), "BEGIN CERTIFICATE") {
		t.Error("cert.pem should be plaintext (not encrypted)")
	}
}

func TestPlaintextInitNoMarker(t *testing.T) {
	dir := initTestAppliance(t, "plain", nil)

	if isEncrypted(dir, "plain") {
		t.Error("should not have .encrypted marker")
	}

	keyPath := filepath.Join(dir, "plain", "secrets", "tls", "key.pem")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(keyData), "BEGIN") {
		t.Error("key.pem should be plaintext PEM without encryption")
	}
}

func TestInitGeneratesUpdateToken(t *testing.T) {
	dir := initTestAppliance(t, "tok", nil)

	tokenPath := filepath.Join(dir, "tok", "secrets", "update.token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("update.token should not be empty")
	}
}

func TestInitWithAuthorizedKeys(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	cfg := DefaultConfig("keys")
	cfg.Credentials.SSHAuthorizedKeys = []string{
		"ssh-ed25519 AAAA... test@host",
		"ssh-rsa BBBB... other@host",
	}
	cfgPath := filepath.Join(dir, "input.json")
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0o644) //nolint:errcheck,gosec // test

	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "pw")
	env.ResetCache()
	code := runInit([]string{"--config", cfgPath, "keys"})
	if code != exitOK {
		t.Fatalf("init returned %d", code)
	}

	authPath := filepath.Join(dir, "keys", "secrets", "authorized_keys")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(authData)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 authorized keys, got %d", len(lines))
	}
}

func TestInitAdminDisabled(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	cfg := DefaultConfig("noadmin")
	cfg.Credentials.AdminEnabled = false
	cfgPath := filepath.Join(dir, "input.json")
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0o644) //nolint:errcheck,gosec // test

	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "pw")
	env.ResetCache()
	code := runInit([]string{"--config", cfgPath, "noadmin"})
	if code != exitOK {
		t.Fatalf("init returned %d", code)
	}

	loaded, err := LoadConfig(ConfigPath(dir, "noadmin"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Credentials.AdminEnabled {
		t.Error("admin-enabled should be false")
	}
}

func TestInitPasswordHashIsBcrypt(t *testing.T) {
	dir := initTestAppliance(t, "bcr", nil)

	hashPath := filepath.Join(dir, "bcr", "secrets", "password.hash")
	hashData, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(hashData, []byte("test-password")); err != nil {
		t.Errorf("bcrypt verify failed: %v", err)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	dir := initTestAppliance(t, "dup", nil)

	baseDir = dir
	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "pw")
	env.ResetCache()

	cfg := DefaultConfig("dup")
	cfgPath := filepath.Join(dir, "input2.json")
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0o644) //nolint:errcheck,gosec // test

	code := runInit([]string{"--config", cfgPath, "dup"})
	if code != exitError {
		t.Errorf("init should fail for existing appliance, got %d", code)
	}
}

func TestBatchInitCreatesMultiple(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	manifest := `[
		{"name": "edge-01", "hostname": "edge-01.lab", "password": "pw1"},
		{"name": "edge-02", "hostname": "edge-02.lab", "password": "pw2"},
		{"name": "edge-03", "hostname": "edge-03.lab", "password": "pw3"}
	]`
	manifestPath := filepath.Join(dir, "manifest.json")
	os.WriteFile(manifestPath, []byte(manifest), 0o644) //nolint:errcheck,gosec // test

	env.ResetCache()
	code := runInit([]string{"--batch", manifestPath})
	if code != exitOK {
		t.Fatalf("batch init returned %d, want 0", code)
	}

	for _, name := range []string{"edge-01", "edge-02", "edge-03"} {
		appDir := filepath.Join(dir, name)
		if _, err := os.Stat(filepath.Join(appDir, "appliance.json")); err != nil {
			t.Errorf("%s: appliance.json missing", name)
		}
		if _, err := os.Stat(filepath.Join(appDir, "secrets", "password.hash")); err != nil {
			t.Errorf("%s: password.hash missing", name)
		}
		if _, err := os.Stat(filepath.Join(appDir, "secrets", "update.token")); err != nil {
			t.Errorf("%s: update.token missing", name)
		}
	}
}

func TestBatchInitMissingPasswordFails(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	manifest := `[{"name": "nopass"}]`
	manifestPath := filepath.Join(dir, "manifest.json")
	os.WriteFile(manifestPath, []byte(manifest), 0o644) //nolint:errcheck,gosec // test

	t.Setenv("ZE_APPLIANCE_SSH_PASSWORD", "")
	env.ResetCache()
	code := runInit([]string{"--batch", manifestPath})
	if code != exitError {
		t.Errorf("batch init should fail without password, got %d", code)
	}
}

func TestBatchInitPerDevicePasswords(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	manifest := `[
		{"name": "gen1", "password": "generate"},
		{"name": "gen2", "password": "generate"}
	]`
	manifestPath := filepath.Join(dir, "manifest.json")
	os.WriteFile(manifestPath, []byte(manifest), 0o644) //nolint:errcheck,gosec // test

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	env.ResetCache()
	code := runInit([]string{"--batch", manifestPath})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	if code != exitOK {
		t.Fatalf("batch init returned %d, want 0", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "gen1:") {
		t.Errorf("output should contain gen1 password, got: %q", output)
	}
	if !strings.Contains(output, "gen2:") {
		t.Errorf("output should contain gen2 password, got: %q", output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	passwords := make(map[string]bool)
	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			passwords[parts[1]] = true
		}
	}
	if len(passwords) < 2 {
		t.Errorf("generated passwords should be unique, got %d distinct values", len(passwords))
	}
}
