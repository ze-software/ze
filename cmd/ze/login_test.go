// Design: docs/architecture/appliance-serial-login.md -- serial console login tests
// VALIDATES: AC-1 (argv[0] dispatch), AC-2 (valid creds), AC-3 (invalid creds),
//            AC-4 (missing ZeFS), AC-6 (non-terminal), AC-9 (fallback path)
// PREVENTS: unauthenticated serial console access on gokrazy appliance

//go:build ze_core

package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	_ "github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/pkg/zefs"
)

func setConfigDir(t *testing.T, dir string) {
	t.Helper()
	orig := env.Get("ze.config.dir")
	if err := env.Set("ze.config.dir", dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
}

func setupLoginMocks(t *testing.T) {
	t.Helper()
	origDelay := retryDelay
	retryDelay = 0
	t.Cleanup(func() {
		execShellFn = defaultExecShell
		readPasswordFn = defaultReadPassword
		isTerminalFn = defaultIsTerminal
		retryDelay = origDelay
	})
}

func pipeStdin(t *testing.T, content string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })

	go func() {
		w.WriteString(content) //nolint:errcheck // test input
		w.Close()              //nolint:errcheck // test cleanup
	}()
}

func TestShellArgvDispatch(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"ash triggers login", "ash", true},
		{"sh triggers login", "sh", true},
		{"ze does not trigger", "ze", false},
		{"ze-test does not trigger", "ze-test", false},
		{"empty does not trigger", "", false},
		{"bash does not trigger", "bash", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isShellInvocation(tt.input); got != tt.expect {
				t.Errorf("isShellInvocation(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestLoginValidCredentials(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	db.Close() //nolint:errcheck // test cleanup

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	readPasswordFn = func(_ int) ([]byte, error) { return []byte("secret123"), nil }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)
	pipeStdin(t, "admin\n")

	code := loginMain()
	if code != 0 {
		t.Errorf("loginMain() = %d, want 0", code)
	}
	if !execCalled {
		t.Error("execShellFn was not called")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	db.Close() //nolint:errcheck // test cleanup

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	readPasswordFn = func(_ int) ([]byte, error) { return []byte("wrongpassword"), nil }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)
	pipeStdin(t, "admin\nadmin\nadmin\n")

	code := loginMain()
	if code != 1 {
		t.Errorf("loginMain() = %d, want 1", code)
	}
	if execCalled {
		t.Error("execShellFn should not be called on wrong password")
	}
}

func TestLoginWrongUsernameCorrectPassword(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	db.Close() //nolint:errcheck // test cleanup

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	readPasswordFn = func(_ int) ([]byte, error) { return []byte("secret123"), nil }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)
	pipeStdin(t, "wronguser\nwronguser\nwronguser\n")

	code := loginMain()
	if code != 1 {
		t.Errorf("loginMain() = %d, want 1 (wrong username rejects even with correct password)", code)
	}
	if execCalled {
		t.Error("execShellFn should not be called with wrong username")
	}
}

func TestLoginMissingZeFS(t *testing.T) {
	dir := t.TempDir()

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)

	// VALIDATES: an absent store refuses the login (owner decision, fail closed).
	code := loginMain()
	if code == 0 {
		t.Errorf("loginMain() = %d, want non-zero (fail closed)", code)
	}
	if execCalled {
		t.Error("execShellFn must not be called on a missing store (fail closed)")
	}
}

func TestLoginMissingCreds(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close() //nolint:errcheck // test cleanup

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)

	// VALIDATES: a store with no credentials refuses the login (fail closed).
	code := loginMain()
	if code == 0 {
		t.Errorf("loginMain() = %d, want non-zero (fail closed)", code)
	}
	if execCalled {
		t.Error("execShellFn must not be called on missing credentials (fail closed)")
	}
}

func TestLoginMaxRetries(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	db.Close() //nolint:errcheck // test cleanup

	retryCount := 0
	setupLoginMocks(t)
	readPasswordFn = func(_ int) ([]byte, error) {
		retryCount++
		return []byte("wrong"), nil
	}
	isTerminalFn = func(_ int) bool { return true }
	execShellFn = func() int { return 0 }

	setConfigDir(t, dir)
	pipeStdin(t, "admin\nadmin\nadmin\n")

	code := loginMain()
	if code != 1 {
		t.Errorf("loginMain() = %d, want 1", code)
	}
	if retryCount != maxLoginRetries {
		t.Errorf("retryCount = %d, want %d", retryCount, maxLoginRetries)
	}
}

func TestLoginNonTerminal(t *testing.T) {
	setupLoginMocks(t)
	isTerminalFn = func(_ int) bool { return false }

	code := loginMain()
	if code != 1 {
		t.Errorf("loginMain() = %d, want 1 for non-terminal", code)
	}
}

func TestZeFSFallbackPath(t *testing.T) {
	setupLoginMocks(t)
	isTerminalFn = func(_ int) bool { return true }
	execShellFn = func() int { return 0 }

	setConfigDir(t, "")

	// VALIDATES: the default /perm/ze folder is used and, absent, refuses the
	// login (fail closed) rather than starting a shell.
	var execCalled bool
	execShellFn = func() int { execCalled = true; return 0 }
	code := loginMain()
	if code == 0 {
		t.Errorf("loginMain() = %d, want non-zero (fail closed when /perm/ze missing)", code)
	}
	if execCalled {
		t.Error("execShellFn must not be called when /perm/ze is missing (fail closed)")
	}
}

// VALIDATES: admin-disabled in zefs blocks serial console login (fail-closed).
// PREVENTS: built-in admin remaining accessible on serial console after disable.
func TestLoginAdminDisabled(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	if err := db.WriteKey(zefs.KeyInstanceAdminDisabled.Pattern, []byte("true")); err != nil {
		t.Fatal(err)
	}
	db.Close() //nolint:errcheck // test cleanup

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	readPasswordFn = func(_ int) ([]byte, error) { return []byte("secret123"), nil }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)
	pipeStdin(t, "admin\n")

	code := loginMain()
	if code != 1 {
		t.Errorf("loginMain() = %d, want 1 (admin disabled)", code)
	}
	if execCalled {
		t.Error("execShellFn should not be called when admin is disabled")
	}
}

func createTestDB(t *testing.T, dir string) storage.Storage {
	t.Helper()
	db, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WriteKey(zefs.KeyLocalAdminUsername.Pattern, []byte("admin")); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteKey(zefs.KeyLocalAdminPassword.Pattern, hash); err != nil {
		t.Fatal(err)
	}
	return db
}

// corruptFrame flips one payload byte of the tree frame that holds key, so the
// next ReadFile of that key fails the CRC with storage.ErrCorrupt.
func corruptFrame(t *testing.T, dir, key string) {
	t.Helper()
	path := filepath.Join(dir, "database", filepath.FromSlash(key))
	frame, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	frame[len(frame)-1] ^= 1
	if err := os.WriteFile(path, frame, 0o600); err != nil {
		t.Fatal(err)
	}
}

// refusedLogin runs loginMain against dir with valid credentials on stdin and
// asserts the login is refused before the shell starts.
func refusedLogin(t *testing.T, dir, why string) {
	t.Helper()
	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	readPasswordFn = func(_ int) ([]byte, error) { return []byte("secret123"), nil }
	isTerminalFn = func(_ int) bool { return true }
	setConfigDir(t, dir)
	pipeStdin(t, "admin\n")

	code := loginMain()
	if code == 0 {
		t.Errorf("loginMain() = %d, want non-zero (%s)", code, why)
	}
	if execCalled {
		t.Errorf("execShellFn must not be called: %s", why)
	}
}

// VALIDATES: a CRC-corrupt admin-disabled flag refuses the login (fail closed).
// PREVENTS: a corrupt flag being read as "not disabled".
func TestLoginCorruptAdminDisabledFlagRefused(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	if err := db.WriteKey(zefs.KeyInstanceAdminDisabled.Pattern, []byte("true")); err != nil {
		t.Fatal(err)
	}
	db.Close() //nolint:errcheck // test cleanup
	corruptFrame(t, dir, zefs.KeyInstanceAdminDisabled.Pattern)
	refusedLogin(t, dir, "corrupt admin-disabled flag")
}

// VALIDATES: a username with no password hash refuses the login.
func TestLoginUsernameWithoutHashRefused(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WriteKey(zefs.KeyLocalAdminUsername.Pattern, []byte("admin")); err != nil {
		t.Fatal(err)
	}
	db.Close() //nolint:errcheck // test cleanup
	refusedLogin(t, dir, "username present, hash absent")
}

// VALIDATES: a CRC-corrupt username frame refuses the login.
func TestLoginCorruptUsernameFrameRefused(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	db.Close() //nolint:errcheck // test cleanup
	corruptFrame(t, dir, zefs.KeyLocalAdminUsername.Pattern)
	refusedLogin(t, dir, "corrupt username frame")
}

// VALIDATES: a CRC-corrupt password hash frame refuses the login.
func TestLoginCorruptHashFrameRefused(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	db.Close() //nolint:errcheck // test cleanup
	corruptFrame(t, dir, zefs.KeyLocalAdminPassword.Pattern)
	refusedLogin(t, dir, "corrupt password hash frame")
}
