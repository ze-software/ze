package init_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	zeinit "codeberg.org/thomas-mangin/ze/cmd/ze/init"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// VALIDATES: ze init with piped stdin creates zefs database with SSH credentials
// PREVENTS: missing bootstrap step leaves CLI unable to connect to daemon

func TestZeInitPipedStdin(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Pipe credentials via stdin: username, password, host, port
	input := "admin\nsecret123\n127.0.0.1\n2222\n"

	code := zeinit.RunWithReader(strings.NewReader(input), dbPath)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// Verify database was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database.zefs was not created")
	}

	// Verify credentials can be read back
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "ssh/username", "admin")
	assertStoreFile(t, store, "ssh/password", "secret123")
	assertStoreFile(t, store, "ssh/host", "127.0.0.1")
	assertStoreFile(t, store, "ssh/port", "2222")
}

// VALIDATES: ze init refuses to overwrite existing database
// PREVENTS: accidental credential loss

func TestZeInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Create existing database
	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.WriteFile("ssh/username", []byte("existing"), 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store.Close() //nolint:errcheck // test setup

	// Try to init again — should fail
	input := "admin\nsecret123\n127.0.0.1\n2222\n"
	code := zeinit.RunWithReader(strings.NewReader(input), dbPath)
	if code == 0 {
		t.Fatal("expected non-zero exit code when database already exists")
	}

	// Verify original data preserved
	store2, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store2.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store2, "ssh/username", "existing")
}

// VALIDATES: ze init with default host/port when not provided
// PREVENTS: missing defaults break CLI connectivity

func TestZeInitDefaults(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Only provide username and password, empty lines for host and port
	input := "admin\nsecret123\n\n\n"

	code := zeinit.RunWithReader(strings.NewReader(input), dbPath)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "ssh/host", "127.0.0.1")
	assertStoreFile(t, store, "ssh/port", "2222")
}

// VALIDATES: ze init requires username and password
// PREVENTS: empty credentials allow unauthenticated access

func TestZeInitRequiresCredentials(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Empty username
	code := zeinit.RunWithReader(strings.NewReader("\nsecret\n\n\n"), dbPath)
	if code == 0 {
		t.Fatal("expected non-zero exit code for empty username")
	}

	// Empty password
	code = zeinit.RunWithReader(strings.NewReader("admin\n\n\n\n"), dbPath)
	if code == 0 {
		t.Fatal("expected non-zero exit code for empty password")
	}
}

// VALIDATES: ze init interactive mode prints prompts and reads credentials
// PREVENTS: silent stdin read confusing users in terminal mode

func TestZeInitInteractive(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Simulate interactive input
	input := "admin\nsecret123\n127.0.0.1\n2222\n"
	var prompts strings.Builder

	code := zeinit.RunInteractive(strings.NewReader(input), &prompts, dbPath)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// Verify prompts were printed
	promptOutput := prompts.String()
	if !strings.Contains(promptOutput, "username:") {
		t.Errorf("missing username prompt, got: %q", promptOutput)
	}
	if !strings.Contains(promptOutput, "password:") {
		t.Errorf("missing password prompt, got: %q", promptOutput)
	}
	if !strings.Contains(promptOutput, "host") {
		t.Errorf("missing host prompt, got: %q", promptOutput)
	}
	if !strings.Contains(promptOutput, "port") {
		t.Errorf("missing port prompt, got: %q", promptOutput)
	}

	// Verify credentials stored
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "ssh/username", "admin")
	assertStoreFile(t, store, "ssh/password", "secret123")
}

func assertStoreFile(t *testing.T, store *zefs.BlobStore, key, expected string) {
	t.Helper()
	data, err := store.ReadFile(key)
	if err != nil {
		t.Errorf("ReadFile(%s): %v", key, err)
		return
	}
	if string(data) != expected {
		t.Errorf("ReadFile(%s): got %q, want %q", key, string(data), expected)
	}
}
