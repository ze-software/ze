package init_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

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

	code := zeinit.RunWithReader(strings.NewReader(input), dbPath, false)
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

	assertStoreFile(t, store, "meta/ssh/username", "admin")
	assertBcryptPassword(t, store, "meta/ssh/password", "secret123")
	assertStoreFile(t, store, "meta/ssh/host", "127.0.0.1")
	assertStoreFile(t, store, "meta/ssh/port", "2222")

	// Verify old ssh/* keys do NOT exist (#16)
	assertKeyAbsent(t, store, "ssh/username")
	assertKeyAbsent(t, store, "ssh/password")
	assertKeyAbsent(t, store, "ssh/host")
	assertKeyAbsent(t, store, "ssh/port")
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
	if err := store.WriteFile("meta/ssh/username", []byte("existing"), 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store.Close() //nolint:errcheck // test setup

	// Try to init again -- should fail
	input := "admin\nsecret123\n127.0.0.1\n2222\n"
	code := zeinit.RunWithReader(strings.NewReader(input), dbPath, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code when database already exists")
	}

	// Verify original data preserved
	store2, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store2.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store2, "meta/ssh/username", "existing")
}

// VALIDATES: ze init with default host/port when not provided
// PREVENTS: missing defaults break CLI connectivity

func TestZeInitDefaults(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Only provide username and password, empty lines for host and port
	input := "admin\nsecret123\n\n\n"

	code := zeinit.RunWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "meta/ssh/host", "127.0.0.1")
	assertStoreFile(t, store, "meta/ssh/port", "2222")
}

// VALIDATES: ze init requires username and password
// PREVENTS: empty credentials allow unauthenticated access

func TestZeInitRequiresCredentials(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Empty username
	code := zeinit.RunWithReader(strings.NewReader("\nsecret\n\n\n"), dbPath, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code for empty username")
	}

	// Empty password
	code = zeinit.RunWithReader(strings.NewReader("admin\n\n\n\n"), dbPath, false)
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

	assertStoreFile(t, store, "meta/ssh/username", "admin")
	assertBcryptPassword(t, store, "meta/ssh/password", "secret123")
}

// VALIDATES: ze init writes meta/identity/name when provided
// PREVENTS: missing instance identity in managed deployments

func TestZeInitIdentityName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Provide all fields: username, password, host, port, name
	input := "admin\nsecret123\n127.0.0.1\n2222\nmy-router\n"

	code := zeinit.RunWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "meta/identity/name", "my-router")
}

// VALIDATES: ze init does NOT write meta/identity/name when name is empty (#4)
// PREVENTS: empty identity key polluting the blob

func TestZeInitEmptyName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Provide credentials but empty name
	input := "admin\nsecret123\n127.0.0.1\n2222\n\n"

	code := zeinit.RunWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	// meta/identity/name should NOT exist when name is empty
	assertKeyAbsent(t, store, "meta/identity/name")
}

// VALIDATES: ze init stores name with special characters as opaque value (#10)
// PREVENTS: path separators in name creating unexpected blob keys

func TestZeInitNameSpecialChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with slash", "admin\nsecret123\n\n\nmy/router\n", "my/router"},
		{"with dots", "admin\nsecret123\n\n\n../escape\n", "../escape"},
		{"with spaces", "admin\nsecret123\n\n\nmy router\n", "my router"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "database.zefs")

			code := zeinit.RunWithReader(strings.NewReader(tt.input), dbPath, false)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d", code)
			}

			store, err := zefs.Open(dbPath)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer store.Close() //nolint:errcheck // test cleanup

			// Name is stored as a VALUE under a fixed key, not as a key path
			assertStoreFile(t, store, "meta/identity/name", tt.want)
		})
	}
}

// VALIDATES: ze init writes meta/managed with managed flag
// PREVENTS: managed mode not stored in database

func TestZeInitManagedKey(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Without managed: default false
	input := "admin\nsecret123\n127.0.0.1\n2222\n\n"
	code := zeinit.RunWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	assertStoreFile(t, store, "meta/managed", "false")
	store.Close() //nolint:errcheck // test cleanup

	// With managed=true
	dir2 := t.TempDir()
	dbPath2 := filepath.Join(dir2, "database.zefs")

	code = zeinit.RunWithReader(strings.NewReader(input), dbPath2, true)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store2, err := zefs.Open(dbPath2)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store2.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store2, "meta/managed", "true")
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

func assertKeyAbsent(t *testing.T, store *zefs.BlobStore, key string) {
	t.Helper()
	if store.Has(key) {
		t.Errorf("key %q should not exist but does", key)
	}
}

func assertBcryptPassword(t *testing.T, store *zefs.BlobStore, key, plaintext string) {
	t.Helper()
	data, err := store.ReadFile(key)
	if err != nil {
		t.Errorf("ReadFile(%s): %v", key, err)
		return
	}
	hash := string(data)
	if !strings.HasPrefix(hash, "$2a$") {
		t.Errorf("ReadFile(%s): expected bcrypt hash, got %q", key, hash)
		return
	}
	if err := bcrypt.CompareHashAndPassword(data, []byte(plaintext)); err != nil {
		t.Errorf("ReadFile(%s): bcrypt hash does not match plaintext %q", key, plaintext)
	}
}
