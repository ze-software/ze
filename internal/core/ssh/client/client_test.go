package client

import (
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// VALIDATES: ReadCredentials reads meta/ssh/* keys from zefs database
// PREVENTS: CLI commands failing after ze init writes namespaced keys

func TestReadCredentialsMeta(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	// Create a database with meta/ssh/* keys (as ze init would write)
	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	keys := map[string]string{
		"meta/ssh/10.0.0.1/2222/username": "admin",
		"meta/ssh/10.0.0.1/2222/password": "secret123",
		"meta/ssh/default":                "10.0.0.1/2222",
	}
	for k, v := range keys {
		if err := store.WriteFile(k, []byte(v), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", k, err)
		}
	}
	store.Close() //nolint:errcheck // test setup

	creds, err := ReadCredentials(dbPath)
	if err != nil {
		t.Fatalf("ReadCredentials: %v", err)
	}

	if creds.Username != "admin" {
		t.Errorf("Username: got %q, want %q", creds.Username, "admin")
	}
	if creds.Auth != "secret123" {
		t.Errorf("Auth: got %q, want %q", creds.Auth, "secret123")
	}
	if creds.Host != "10.0.0.1" {
		t.Errorf("Host: got %q, want %q", creds.Host, "10.0.0.1")
	}
	if creds.Port != "2222" {
		t.Errorf("Port: got %q, want %q", creds.Port, "2222")
	}
}

// VALIDATES: env vars override stored host/port values (#6)
// PREVENTS: env var overrides silently broken after key rename

func TestReadCredentialsEnvOverride(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	keys := map[string]string{
		"meta/ssh/10.0.0.1/2222/username":             "admin",
		"meta/ssh/10.0.0.1/2222/password":             "secret",
		"meta/ssh/override.example.com/2222/username": "admin",
		"meta/ssh/override.example.com/2222/password": "secret",
		"meta/ssh/default":                            "10.0.0.1/2222",
	}
	for k, v := range keys {
		if err := store.WriteFile(k, []byte(v), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", k, err)
		}
	}
	store.Close() //nolint:errcheck // test setup

	// Set env var to override host (t.Setenv auto-restores after test)
	t.Setenv("ze_ssh_host", "override.example.com")
	env.ResetCache()

	creds, err := ReadCredentials(dbPath)
	if err != nil {
		t.Fatalf("ReadCredentials: %v", err)
	}

	if creds.Host != "override.example.com" {
		t.Errorf("Host: got %q, want %q (env override)", creds.Host, "override.example.com")
	}
	if creds.Port != "2222" {
		t.Errorf("Port: got %q, want %q (from default)", creds.Port, "2222")
	}
}

// VALIDATES: setting ze.ssh.host bypasses default pointer entirely; port uses built-in default
// PREVENTS: partial env override mixing host from env with port from pointer

func TestReadCredentialsEnvHostBypassesPointer(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")

	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	keys := map[string]string{
		"meta/ssh/10.0.0.1/3333/username": "admin",
		"meta/ssh/10.0.0.1/3333/password": "secret",
		"meta/ssh/10.0.0.2/2222/username": "admin",
		"meta/ssh/10.0.0.2/2222/password": "secret2",
		"meta/ssh/default":                "10.0.0.1/3333",
	}
	for k, v := range keys {
		if err := store.WriteFile(k, []byte(v), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", k, err)
		}
	}
	store.Close() //nolint:errcheck // test setup

	// Set only host env; port should NOT come from default pointer (3333)
	// but from built-in default (2222)
	t.Setenv("ze_ssh_host", "10.0.0.2")
	env.ResetCache()

	creds, err := ReadCredentials(dbPath)
	if err != nil {
		t.Fatalf("ReadCredentials: %v", err)
	}

	if creds.Host != "10.0.0.2" {
		t.Errorf("Host: got %q, want %q", creds.Host, "10.0.0.2")
	}
	if creds.Port != "2222" {
		t.Errorf("Port: got %q, want %q (built-in default, not pointer's 3333)", creds.Port, "2222")
	}
	if creds.Auth != "secret2" {
		t.Errorf("Auth: got %q, want %q (from 10.0.0.2/2222)", creds.Auth, "secret2")
	}
}

// seedSuperAdminZefs creates a database.zefs in dir with a fixed super-admin
// entry (username "admin", auth "adminhash"). Used by the WithFlags
// credential resolution tests.
func seedSuperAdminZefs(t *testing.T, dir string) string {
	t.Helper()
	dbPath := filepath.Join(dir, "database.zefs")
	store, err := zefs.Create(dbPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for k, v := range map[string]string{
		"meta/ssh/10.0.0.1/2222/username": "admin",
		"meta/ssh/10.0.0.1/2222/password": "adminhash",
		"meta/ssh/default":                "10.0.0.1/2222",
	} {
		if err := store.WriteFile(k, []byte(v), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", k, err)
		}
	}
	store.Close() //nolint:errcheck // test setup
	return dbPath
}

// VALIDATES: --user flag overrides zefs username (D-4 precedence: flag > env > zefs).
// PREVENTS: regression where the CLI silently ignores --user and uses super-admin.
func TestReadCredentialsFlagWins(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir())
	t.Setenv("ze_ssh_username", "fromenv") // should LOSE to flag
	t.Setenv("ze_ssh_password", "frompw")  // ensure non-super-admin path has a password source
	env.ResetCache()

	creds, err := ReadCredentialsWithFlags(dbPath, "alice")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags: %v", err)
	}
	if creds.Username != "alice" {
		t.Errorf("Username: got %q, want %q", creds.Username, "alice")
	}
	if creds.Auth != "frompw" {
		t.Errorf("Auth: got %q, want %q (env)", creds.Auth, "frompw")
	}
}

// VALIDATES: ze.ssh.username env wins over zefs when no flag.
// PREVENTS: regression in env-only invocation paths.
func TestReadCredentialsEnvUsernameWins(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir())
	t.Setenv("ze_ssh_username", "audit-user")
	t.Setenv("ze_ssh_password", "auditpw")
	env.ResetCache()

	creds, err := ReadCredentialsWithFlags(dbPath, "")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags: %v", err)
	}
	if creds.Username != "audit-user" {
		t.Errorf("Username: got %q, want %q", creds.Username, "audit-user")
	}
}

// VALIDATES: super-admin path preserved when no flag/env -- backwards compat.
// PREVENTS: existing CLI binaries breaking after introduction of --user.
func TestReadCredentialsDefaultsToSuperAdmin(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir())
	env.ResetCache()

	creds, err := ReadCredentialsWithFlags(dbPath, "")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags: %v", err)
	}
	if creds.Username != "admin" {
		t.Errorf("Username: got %q, want %q", creds.Username, "admin")
	}
	if creds.Auth != "adminhash" {
		t.Errorf("Auth: got %q, want %q (zefs hash-as-token)", creds.Auth, "adminhash")
	}
}

// VALIDATES: non-super-admin user without password source returns clear error
// when stdin is not a TTY (CI / scripts).
//
// PREVENTS: silent password=empty connection attempts for YANG users.
func TestReadCredentialsNonInteractiveNoPassword(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir())
	env.ResetCache() // ensures ze.ssh.password is unset

	_, err := ReadCredentialsWithFlags(dbPath, "alice")
	if err == nil {
		t.Fatal("expected error when no password source for non-super-admin user")
	}
	// In `go test` stdin is not a TTY, so the prompt path is not taken
	// and the error message must name the user and mention the env var.
	got := err.Error()
	if !strings.Contains(got, "alice") || !strings.Contains(got, "ze.ssh.password") {
		t.Errorf("error %q must name user and ze.ssh.password env var", got)
	}
}

// VALIDATES: TrimErrorPrefix strips the daemon's "error: " display prefix so a
// caller that prints "error: %v" renders one prefix, not "error: error: ...".
// PREVENTS: regression of the doubled prefix operators saw on every failed
// command (e.g. "error: error: unknown command").
func TestTrimErrorPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "daemon-formatted failure loses exactly one prefix",
			in:   "error: command restricted by access control",
			want: "command restricted by access control",
		},
		{
			name: "unprefixed message is untouched",
			in:   "connection refused",
			want: "connection refused",
		},
		{
			name: "only a leading prefix is stripped",
			in:   "error: parse failed: error: inner detail",
			want: "parse failed: error: inner detail",
		},
		{
			name: "a doubled prefix loses only the outer one, so the daemon can still say 'error:'",
			in:   "error: error: unknown command",
			want: "error: unknown command",
		},
		{
			name: "multi-line output keeps its trailing lines",
			in:   "error: unknown command\nhint: run 'help'",
			want: "unknown command\nhint: run 'help'",
		},
		{
			name: "a message that merely mentions error: is untouched",
			in:   "peer reported error: hold timer expired",
			want: "peer reported error: hold timer expired",
		},
		{
			name: "empty stays empty",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimErrorPrefix(tt.in); got != tt.want {
				t.Errorf("TrimErrorPrefix(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// VALIDATES: ResolveDBPath resolves the store under ze.config.dir.
// PREVENTS: the CLI's credential lookup drifting from where ze init writes the
// store. That split is exactly how `ze data` broke: two resolvers disagreeing
// about the config dir, so one wrote a store the other could not find.
func TestResolveDBPath_HonorsConfigDirEnv(t *testing.T) {
	dir := t.TempDir()
	orig := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
	if err := env.Set("ze.config.dir", dir); err != nil {
		t.Fatalf("env.Set ze.config.dir: %v", err)
	}

	if got, want := ResolveDBPath(), filepath.Join(dir, "database.zefs"); got != want {
		t.Errorf("ResolveDBPath() = %q, want %q", got, want)
	}
}
