package init

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: ze init with piped stdin creates zefs database with SSH credentials
// PREVENTS: missing bootstrap step leaves CLI unable to connect to daemon

// runWithReader creates a live store in dir with SSH credentials read from r,
// without prompts.
func runWithReader(r io.Reader, dir string, managed bool) int {
	return runInit(r, nil, dir, managed, "", "", false, false)
}

// runWithReaderForce stages a replacement under exclusive storage ownership,
// as ze init --force --yes does once the operator confirmed.
func runWithReaderForce(r io.Reader, dir string, managed bool) int {
	return runInit(r, nil, dir, managed, "", "", false, true)
}

// runInteractive creates a live store with the prompts written to w, as
// ze init does on a terminal.
func runInteractive(r io.Reader, w io.Writer, dir string) int {
	return runInit(r, w, dir, false, "", "", false, false)
}

func TestZeInitPipedStdin(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Pipe credentials via stdin: username, password, host, port
	input := "admin\nsecret123\n127.0.0.1\n2222\n"

	code := runWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dbPath, "database")); os.IsNotExist(err) {
		t.Fatal("database directory was not created")
	}

	// Verify credentials can be read back
	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "meta/ssh/127.0.0.1/2222/username", "admin")
	assertBcryptPassword(t, store, "meta/ssh/127.0.0.1/2222/password", "secret123")
	assertStoreFile(t, store, zefs.KeyLocalAdminUsername.Pattern, "admin")
	assertBcryptPassword(t, store, zefs.KeyLocalAdminPassword.Pattern, "secret123")
	assertStoreFile(t, store, "meta/ssh/default", "127.0.0.1/2222")
}

// VALIDATES: ze init refuses to overwrite existing database
// PREVENTS: accidental credential loss

func TestZeInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Create existing database
	store, err := storage.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.WriteKey("meta/ssh/127.0.0.1/2222/username", []byte("existing")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store.Close() //nolint:errcheck // test setup

	// Try to init again -- should fail
	input := "admin\nsecret123\n127.0.0.1\n2222\n"
	code := runWithReader(strings.NewReader(input), dbPath, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code when database already exists")
	}

	// Verify original data preserved
	store2, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store2.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store2, "meta/ssh/127.0.0.1/2222/username", "existing")
}

// VALIDATES: ze init with default host/port when not provided
// PREVENTS: missing defaults break CLI connectivity

func TestZeInitDefaults(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Only provide username and password, empty lines for host and port
	input := "admin\nsecret123\n\n\n"

	code := runWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "meta/ssh/default", "127.0.0.1/2222")
}

// VALIDATES: ze init requires username and password
// PREVENTS: empty credentials allow unauthenticated access

func TestZeInitRequiresCredentials(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Empty username
	code := runWithReader(strings.NewReader("\nsecret\n\n\n"), dbPath, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code for empty username")
	}

	// Empty password
	code = runWithReader(strings.NewReader("admin\n\n\n\n"), dbPath, false)
	if code == 0 {
		t.Fatal("expected non-zero exit code for empty password")
	}
}

// VALIDATES: ze init interactive mode prints prompts and reads credentials
// PREVENTS: silent stdin read confusing users in terminal mode

func TestZeInitInteractive(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Simulate interactive input
	input := "admin\nsecret123\n127.0.0.1\n2222\n"
	var prompts strings.Builder

	code := runInteractive(strings.NewReader(input), &prompts, dbPath)
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
	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "meta/ssh/127.0.0.1/2222/username", "admin")
	assertBcryptPassword(t, store, "meta/ssh/127.0.0.1/2222/password", "secret123")
}

// VALIDATES: ze init writes meta/instance/name when provided
// PREVENTS: missing instance identity in managed deployments

func TestZeInitIdentityName(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Provide all fields: username, password, host, port, name
	input := "admin\nsecret123\n127.0.0.1\n2222\nmy-router\n"

	code := runWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store, "meta/instance/name", "my-router")
}

// VALIDATES: ze init defaults meta/instance/name to hostname when name is empty
// PREVENTS: missing instance identity when name not provided

func TestZeInitEmptyName(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Provide credentials but empty name -- should default to hostname
	input := "admin\nsecret123\n127.0.0.1\n2222\n\n"

	code := runWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup

	// meta/instance/name should be the hostname when name is empty
	hostname, _ := os.Hostname()
	assertStoreFile(t, store, "meta/instance/name", hostname)
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
			dbPath := dir

			code := runWithReader(strings.NewReader(tt.input), dbPath, false)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d", code)
			}

			store, err := storage.OpenReadOnly(dbPath)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer store.Close() //nolint:errcheck // test cleanup

			// Name is stored as a VALUE under a fixed key, not as a key path
			assertStoreFile(t, store, "meta/instance/name", tt.want)
		})
	}
}

// VALIDATES: ze init writes meta/instance/managed with managed flag
// PREVENTS: managed mode not stored in database

func TestZeInitManagedKey(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Without managed: default false
	input := "admin\nsecret123\n127.0.0.1\n2222\n\n"
	code := runWithReader(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	assertStoreFile(t, store, "meta/instance/managed", "false")
	store.Close() //nolint:errcheck // test cleanup

	// With managed=true
	dir2 := t.TempDir()
	dbPath2 := dir2

	code = runWithReader(strings.NewReader(input), dbPath2, true)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store2, err := storage.OpenReadOnly(dbPath2)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store2.Close() //nolint:errcheck // test cleanup

	assertStoreFile(t, store2, "meta/instance/managed", "true")
}

// VALIDATES: ze init --force moves existing database aside and creates new one
// PREVENTS: no way to reinitialize without manual file deletion

func TestZeInitForce(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	// Create existing database with a known value
	store, err := storage.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.WriteKey("meta/ssh/127.0.0.1/2222/username", []byte("old-admin")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store.Close() //nolint:errcheck // test setup

	// Force reinitialize
	input := "new-admin\nnewpass\n127.0.0.1\n2222\n"
	code := runWithReaderForce(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	// New database should have new credentials
	store2, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open new: %v", err)
	}
	defer store2.Close() //nolint:errcheck // test cleanup
	assertStoreFile(t, store2, "meta/ssh/127.0.0.1/2222/username", "new-admin")

	// Old database should exist as .replaced-<date>
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "database.replaced-") {
			continue
		}
		found = true
		// Verify old data is in the backup
		backupPath := filepath.Join(dir, e.Name())
		frame, err := os.ReadFile(filepath.Join(backupPath, "meta", "ssh", "127.0.0.1", "2222", "username"))
		if err != nil {
			t.Fatalf("Read backup: %v", err)
		}
		value, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
		if err != nil {
			t.Fatalf("Decode backup: %v", err)
		}
		if next != len(frame) || string(value) != "old-admin" {
			t.Fatalf("backup credentials = %q, consumed %d/%d", value, next, len(frame))
		}
		break
	}
	if !found {
		t.Fatal("old database was not moved to .replaced-<date> backup")
	}
}

// VALIDATES: ze init --force with no existing database succeeds normally
// PREVENTS: --force failing when there is nothing to replace

func TestZeInitForceNoExisting(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir

	input := "admin\nsecret\n127.0.0.1\n2222\n"
	code := runWithReaderForce(strings.NewReader(input), dbPath, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	store, err := storage.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	assertStoreFile(t, store, "meta/ssh/127.0.0.1/2222/username", "admin")
}

func assertStoreFile(t *testing.T, store storage.Storage, key, expected string) {
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

func assertKeyAbsent(t *testing.T, store storage.Storage, key string) { //nolint:unused // retained for future tests
	t.Helper()
	if store.Exists(key) {
		t.Errorf("key %q should not exist but does", key)
	}
}

func assertBcryptPassword(t *testing.T, store storage.Storage, key, plaintext string) {
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
