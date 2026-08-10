package appliance

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

func assembleTestAppliance(t *testing.T, name string, passphrase []byte) string {
	t.Helper()
	dir := initTestAppliance(t, name, passphrase)
	baseDir = dir
	return dir
}

func TestAssembleProducesZeFS(t *testing.T) {
	dir := assembleTestAppliance(t, "asm", nil)

	code := runAssemble([]string{"--keep", "asm"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "asm")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	cfg := DefaultConfig("asm")
	for _, key := range []string{
		zefs.KeySSHUsername.Key(cfg.SSH.Host, cfg.SSH.Port),
		zefs.KeySSHPassword.Key(cfg.SSH.Host, cfg.SSH.Port),
		zefs.KeyLocalAdminUsername.Pattern,
		zefs.KeyLocalAdminPassword.Pattern,
		zefs.KeyInstanceName.Pattern,
		zefs.KeyInstanceManaged.Pattern,
		zefs.KeyWebCert.Pattern,
		zefs.KeyWebKey.Pattern,
	} {
		data, readErr := store.ReadFile(key)
		if readErr != nil {
			t.Errorf("missing key %s: %v", key, readErr)
			continue
		}
		if len(data) == 0 {
			t.Errorf("key %s is empty", key)
		}
	}
}

func TestAssembleReusesExistingCert(t *testing.T) {
	dir := assembleTestAppliance(t, "cert-reuse", nil)

	certPath := filepath.Join(dir, "cert-reuse", "secrets", "tls", "cert.pem")
	certBefore, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}

	code := runAssemble([]string{"--keep", "cert-reuse"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "cert-reuse")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	certInDB, err := store.ReadFile(zefs.KeyWebCert.Pattern)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(certInDB, certBefore) {
		t.Error("cert in ZeFS should match cert on disk")
	}
}

func TestAssembleMissingSecretsFails(t *testing.T) {
	dir := assembleTestAppliance(t, "missing", nil)

	os.Remove(filepath.Join(dir, "missing", "secrets", "password.hash")) //nolint:errcheck // test setup

	code := runAssemble([]string{"missing"})
	if code != exitError {
		t.Errorf("assemble should fail with missing secrets, got %d", code)
	}
}

func TestAssembleConfigLayering(t *testing.T) {
	dir := assembleTestAppliance(t, "layered", nil)
	appDir := filepath.Join(dir, "layered")

	sharedDir := filepath.Join(dir, "_shared")
	os.MkdirAll(sharedDir, 0o755)                                                                        //nolint:errcheck // test
	os.WriteFile(filepath.Join(sharedDir, "ze.conf"), []byte("set environment log level info\n"), 0o644) //nolint:errcheck,gosec // test
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("set bgp local-as 65001\n"), 0o644)            //nolint:errcheck,gosec // test

	cfg, _ := LoadConfig(ConfigPath(dir, "layered"))
	cfg.ConfigBase = "../_shared/ze.conf"
	saveConfig(ConfigPath(dir, "layered"), cfg) //nolint:errcheck // test

	code := runAssemble([]string{"--keep", "layered"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "layered")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	seedKey := zefs.KeyFileTemplate.Key("ze.conf")
	data, err := store.ReadFile(seedKey)
	if err != nil {
		t.Fatalf("read seed config: %v", err)
	}
	content := string(data)
	if !contains(content, "log level info") {
		t.Error("seed config should contain base config")
	}
	if !contains(content, "local-as 65001") {
		t.Error("seed config should contain overlay config")
	}
}

func TestAssembleDefaultZeConf(t *testing.T) {
	dir := assembleTestAppliance(t, "noconf", nil)

	code := runAssemble([]string{"--keep", "noconf"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "noconf")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	cfg := DefaultConfig("noconf")
	data, readErr := store.ReadFile(zefs.KeySSHUsername.Key(cfg.SSH.Host, cfg.SSH.Port))
	if readErr != nil {
		t.Fatalf("read username: %v", readErr)
	}
	if string(data) != "admin" {
		t.Errorf("username = %q, want admin", data)
	}
}

func TestAssembleHostnamePatch(t *testing.T) {
	dir := assembleTestAppliance(t, "hostname", nil)

	code := runAssemble([]string{"--keep", "hostname"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "hostname")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	nameData, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if err != nil {
		t.Fatal(err)
	}
	if string(nameData) != "hostname" {
		t.Errorf("instance name = %q, want hostname", nameData)
	}
}

func TestAssembleWrongPassphraseFails(t *testing.T) {
	dir := assembleTestAppliance(t, "wrongpw", []byte("correct-pass"))

	baseDir = dir
	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "wrong-pass")
	env.ResetCache()

	code := runAssemble([]string{"wrongpw"})
	if code != exitError {
		t.Errorf("assemble should fail with wrong passphrase, got %d", code)
	}

	dbPath := databasePath(dir, "wrongpw")
	if _, err := os.Stat(dbPath); err == nil {
		t.Error("no partial database should remain after failed assemble")
	}
}

func TestAssembleAutoDeleteZeFS(t *testing.T) {
	dir := assembleTestAppliance(t, "autodel", nil)

	code := runAssemble([]string{"autodel"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "autodel")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("database.zefs should be auto-deleted without --keep")
	}
}

func TestAssembleKeepRetainsZeFS(t *testing.T) {
	dir := assembleTestAppliance(t, "kept", nil)

	code := runAssemble([]string{"--keep", "kept"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "kept")
	if _, err := os.Stat(dbPath); err != nil {
		t.Error("database.zefs should be retained with --keep")
	}
}

func TestAssembleSeedConfigIncludesSSHPort(t *testing.T) {
	dir := assembleTestAppliance(t, "sshport", nil)
	appDir := filepath.Join(dir, "sshport")

	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("set environment ssh enabled true\n"), 0o644) //nolint:errcheck,gosec // test

	cfg, _ := LoadConfig(ConfigPath(dir, "sshport"))
	cfg.SSH.Host = "0.0.0.0"
	cfg.SSH.Port = "8822"
	saveConfig(ConfigPath(dir, "sshport"), cfg) //nolint:errcheck // test

	code := runAssemble([]string{"--keep", "sshport"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "sshport")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	seedKey := zefs.KeyFileTemplate.Key("ze.conf")
	data, err := store.ReadFile(seedKey)
	if err != nil {
		t.Fatalf("read seed config: %v", err)
	}
	content := string(data)
	if !contains(content, "ssh server default port 8822") {
		t.Errorf("seed config should contain SSH port override, got:\n%s", content)
	}
}

func TestAssembleSeedConfigIncludesWebPort(t *testing.T) {
	dir := assembleTestAppliance(t, "webport", nil)
	appDir := filepath.Join(dir, "webport")

	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("set environment web enabled true\n"), 0o644) //nolint:errcheck,gosec // test

	cfg, _ := LoadConfig(ConfigPath(dir, "webport"))
	cfg.Web.Enabled = true
	cfg.Web.Host = "0.0.0.0"
	cfg.Web.Port = "9443"
	saveConfig(ConfigPath(dir, "webport"), cfg) //nolint:errcheck // test

	code := runAssemble([]string{"--keep", "webport"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "webport")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	seedKey := zefs.KeyFileTemplate.Key("ze.conf")
	data, err := store.ReadFile(seedKey)
	if err != nil {
		t.Fatalf("read seed config: %v", err)
	}
	content := string(data)
	if !contains(content, "web server default port 9443") {
		t.Errorf("seed config should contain web port override, got:\n%s", content)
	}
}

func TestAssembleIncludesAuthorizedKeys(t *testing.T) {
	dir := assembleTestAppliance(t, "authkeys", nil)

	authPath := filepath.Join(dir, "authkeys", "secrets", "authorized_keys")
	os.WriteFile(authPath, []byte("ssh-ed25519 AAAA... test@host\n"), 0o600) //nolint:errcheck,gosec // test

	code := runAssemble([]string{"--keep", "authkeys"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	dbPath := databasePath(dir, "authkeys")
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile(zefs.KeySSHAuthorizedKeys.Pattern)
	if err != nil {
		t.Fatalf("read authorized keys: %v", err)
	}
	if !contains(string(data), "ssh-ed25519") {
		t.Error("authorized keys should be in ZeFS")
	}
}
