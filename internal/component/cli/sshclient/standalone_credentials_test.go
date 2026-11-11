// Design: docs/guide/command-reference.md -- a remote login needs no local store.

package sshclient

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// A downloaded executable has no prefix-derived store path. Exercise the load
// entrypoint the real CLI uses, and make the current directory an unsafe store
// candidate so an empty path cannot accidentally borrow local credentials.
func TestStandaloneRemoteCredentialsNeedNoStore(t *testing.T) {
	standaloneCredentialEnvironment(t)
	t.Setenv("ze.ssh.password", "standalone-password")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true)

	creds, err := LoadCredentialsForRemote("standalone-user", "127.0.0.1", "2221")
	if err != nil {
		t.Fatal(err)
	}
	want := Credentials{Host: "127.0.0.1", Port: "2221", Username: "standalone-user", Auth: "standalone-password"}
	if creds != want {
		t.Fatalf("standalone remote credentials=%+v, want explicit target and identity", creds)
	}
	if *prompted {
		t.Fatal("explicit password unexpectedly prompted")
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("remote login changed its current directory: %v", entries)
	}
	if entries[0].Name() != "database" {
		t.Fatalf("remote login changed its current directory: %v", entries)
	}
}

func TestStandaloneCredentialLoadersUseExplicitEnvironment(t *testing.T) {
	standaloneCredentialEnvironment(t)
	t.Setenv("ze.ssh.host", "192.0.2.10")
	t.Setenv("ze.ssh.port", "2223")
	t.Setenv("ze.ssh.username", "standalone-user")
	t.Setenv("ze.ssh.password", "standalone-password")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true)

	want := Credentials{Host: "192.0.2.10", Port: "2223", Username: "standalone-user", Auth: "standalone-password"}
	for _, load := range []struct {
		name string
		call func() (Credentials, error)
	}{
		{"flag-aware", func() (Credentials, error) { return LoadCredentialsWithFlags("") }},
		{"no-prompt", LoadCredentialsNoPrompt},
	} {
		t.Run(load.name, func(t *testing.T) {
			creds, err := load.call()
			if err != nil {
				t.Fatal(err)
			}
			if creds != want {
				t.Fatalf("standalone credentials=%+v, want supplied environment", creds)
			}
		})
	}
	if *prompted {
		t.Fatal("noninteractive credential sources unexpectedly prompted")
	}
}

func TestStandaloneDaemonAddressNeedsNoStore(t *testing.T) {
	standaloneCredentialEnvironment(t)
	for _, target := range []struct{ host, port, wantHost, wantPort string }{
		{"", "", defaultHost, defaultPort},
		{"192.0.2.10", "2223", "192.0.2.10", "2223"},
	} {
		t.Setenv("ze.ssh.host", target.host)
		t.Setenv("ze.ssh.port", target.port)
		env.ResetCache()
		host, port, err := DaemonAddress()
		if err != nil {
			t.Fatal(err)
		}
		if host != target.wantHost || port != target.wantPort {
			t.Fatalf("standalone daemon address=%s:%s, want %s:%s", host, port, target.wantHost, target.wantPort)
		}
	}
}

func TestStandaloneCredentialsStillRequireIdentity(t *testing.T) {
	standaloneCredentialEnvironment(t)
	t.Setenv("ze.ssh.password", "standalone-password")
	env.ResetCache()
	_, err := LoadCredentialsForRemote("", "127.0.0.1", "2221")
	if err == nil {
		t.Fatal("remote login guessed an identity")
	}
	if !strings.Contains(err.Error(), "no stored username and none supplied") {
		t.Fatalf("missing identity did not name its actual cause: %v", err)
	}
}

func TestStandaloneNoPromptStillRequiresPassword(t *testing.T) {
	standaloneCredentialEnvironment(t)
	t.Setenv("ze.ssh.username", "standalone-user")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true)
	_, err := LoadCredentialsNoPrompt()
	if err == nil {
		t.Fatal("unattended login accepted no password source")
	}
	if !strings.Contains(err.Error(), "no password source") {
		t.Fatalf("missing password did not name its actual cause: %v", err)
	}
	if *prompted {
		t.Fatal("unattended credential loader prompted")
	}
}

func standaloneCredentialEnvironment(t *testing.T) {
	t.Helper()
	for _, setting := range []string{"ze.config.dir", "ze.ssh.host", "ze.ssh.port", "ze.ssh.username", "ze.ssh.password"} {
		for _, key := range []string{setting, strings.ReplaceAll(setting, ".", "_"), strings.ToUpper(strings.ReplaceAll(setting, ".", "_"))} {
			t.Setenv(key, "")
		}
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if got := ResolveStoreDir(""); got != "" {
		t.Fatalf("test executable unexpectedly has an installed store path %q", got)
	}
	t.Chdir(t.TempDir())
	if err := os.WriteFile("database", []byte("unrelated current-directory file"), 0o600); err != nil {
		t.Fatal(err)
	}
}
