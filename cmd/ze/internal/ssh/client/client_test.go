package client

import (
	"path/filepath"
	"testing"

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
		"meta/ssh/username": "admin",
		"meta/ssh/password": "secret123",
		"meta/ssh/host":     "10.0.0.1",
		"meta/ssh/port":     "2222",
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
		"meta/ssh/username": "admin",
		"meta/ssh/password": "secret",
		"meta/ssh/host":     "10.0.0.1",
		"meta/ssh/port":     "2222",
	}
	for k, v := range keys {
		if err := store.WriteFile(k, []byte(v), 0); err != nil {
			t.Fatalf("WriteFile(%s): %v", k, err)
		}
	}
	store.Close() //nolint:errcheck // test setup

	// Set env var to override host (t.Setenv auto-restores after test)
	t.Setenv("ze_ssh_host", "override.example.com")
	// Clear the other form to avoid interference
	t.Setenv("ze.ssh.host", "")

	creds, err := ReadCredentials(dbPath)
	if err != nil {
		t.Fatalf("ReadCredentials: %v", err)
	}

	if creds.Host != "override.example.com" {
		t.Errorf("Host: got %q, want %q (env override)", creds.Host, "override.example.com")
	}
	// Port should still come from store (no env override set)
	if creds.Port != "2222" {
		t.Errorf("Port: got %q, want %q (from store)", creds.Port, "2222")
	}
}
