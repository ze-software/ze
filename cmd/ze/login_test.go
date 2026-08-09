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

	code := loginMain()
	if code != 0 {
		t.Errorf("loginMain() = %d, want 0 (fail-open)", code)
	}
	if !execCalled {
		t.Error("execShellFn should be called on missing ZeFS (fail-open)")
	}
}

func TestLoginMissingCreds(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")
	db, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close() //nolint:errcheck // test cleanup

	var execCalled bool
	setupLoginMocks(t)
	execShellFn = func() int { execCalled = true; return 0 }
	isTerminalFn = func(_ int) bool { return true }

	setConfigDir(t, dir)

	code := loginMain()
	if code != 0 {
		t.Errorf("loginMain() = %d, want 0 (fail-open)", code)
	}
	if !execCalled {
		t.Error("execShellFn should be called on missing creds (fail-open)")
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

	code := loginMain()
	if code != 0 {
		t.Errorf("loginMain() = %d, want 0 (fail-open when /perm/ze missing)", code)
	}
}

// VALIDATES: admin-disabled in zefs blocks serial console login (fail-closed).
// PREVENTS: built-in admin remaining accessible on serial console after disable.
func TestLoginAdminDisabled(t *testing.T) {
	dir := t.TempDir()
	db := createTestDB(t, dir)
	if err := db.WriteFile(zefs.KeyInstanceAdminDisabled.Pattern, []byte("true"), 0); err != nil {
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

func createTestDB(t *testing.T, dir string) *zefs.BlobStore {
	t.Helper()
	dbPath := filepath.Join(dir, "database.zefs")
	db, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte("admin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteFile(zefs.KeyLocalAdminPassword.Pattern, hash, 0o600); err != nil {
		t.Fatal(err)
	}
	return db
}
