package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
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

// VALIDATES: AC-6 -- a YANG user supplies a username and password and logs in
// when no credential store is available to them.
//
// PREVENTS: the production lockout. The store is a shared 0600 file under
// /etc/ze, so every user who did not install ze failed with "open database:
// permission denied" -- before their credentials were ever considered, and even
// though the flag, env and defaults supplied everything needed.
func TestReadCredentialsNoStoreFlagAndEnv(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database.zefs") // never created

	t.Setenv("ze_ssh_password", "alicepw")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true)

	creds, err := ReadCredentialsWithFlags(dbPath, "alice")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags with no store: %v", err)
	}
	if creds.Username != "alice" {
		t.Errorf("Username: got %q, want %q (flag)", creds.Username, "alice")
	}
	if creds.Auth != "alicepw" {
		t.Errorf("Auth: got %q, want %q (env)", creds.Auth, "alicepw")
	}
	if creds.Host != defaultHost || creds.Port != defaultPort {
		t.Errorf("Host:Port got %s:%s, want %s:%s (built-in defaults, no pointer to read)",
			creds.Host, creds.Port, defaultHost, defaultPort)
	}
	if *prompted {
		t.Error("prompted despite an env password being available")
	}
}

// VALIDATES: AC-6 for the real production condition -- the store EXISTS but is
// owned by someone else, which is what a non-installing user actually hits.
// PREVENTS: classifying permission-denied as fatal rather than "no store for me".
func TestReadCredentialsUnreadableStoreFlagAndEnv(t *testing.T) {
	// chmod cannot deny access to root, so this permission-denied
	// condition is unobservable when the suite runs as root. The identical
	// resolution path is covered unconditionally by TestReadCredentialsNoStoreFlagAndEnv
	// (not-exist branch); only the errors.Is(fs.ErrPermission) classification is
	// skipped here.
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 does not deny access")
	}

	dbPath := seedSuperAdminZefs(t, t.TempDir())
	if err := os.Chmod(dbPath, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dbPath, 0o600); err != nil {
			t.Logf("restoring store mode: %v", err)
		}
	})

	t.Setenv("ze_ssh_password", "alicepw")
	env.ResetCache()
	stubPromptPolicy(t, true)

	creds, err := ReadCredentialsWithFlags(dbPath, "alice")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags with an unreadable store: %v", err)
	}
	if creds.Username != "alice" || creds.Auth != "alicepw" {
		t.Errorf("got %s/%s, want alice/alicepw", creds.Username, creds.Auth)
	}
}

// VALIDATES: with no store but an explicit --user, resolution PROMPTS rather
// than failing -- it does not silently fall back to "no credentials".
//
// This pins a deliberate consequence of making the store optional. Before that,
// `ze cli --user alice` with no readable store died in zefs.Open, and
// internal/component/cli/client/main.go routed the resulting credential error to
// the in-process offline fallback, so `-c "show crashes"` answered from local
// data. Now resolution gets as far as the password, so an operator on a terminal
// is asked for one and the command reaches the daemon they explicitly named.
// That is the intended trade: --user is a request to talk to the daemon, and a
// daemon answer beats a local guess. The no-username case still errors
// (TestReadCredentialsNoStoreNoUsername), which is what keeps the offline
// fallback reachable for a plain `ze cli -c "show crashes"`.
//
// PREVENTS: a future "don't prompt without a store" "fix" silently reverting
// --user to the pre-fix behavior of never reaching the daemon.
func TestReadCredentialsNoStoreWithUserPrompts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database.zefs") // never created

	t.Setenv("ze_ssh_password", "")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true) // operator is on a terminal

	creds, err := ReadCredentialsWithFlags(dbPath, "alice")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags with no store and an explicit user: %v", err)
	}
	if !*prompted {
		t.Error("no prompt issued: --user with no store must ask for a password, not give up")
	}
	if creds.Username != "alice" {
		t.Errorf("Username: got %q, want %q", creds.Username, "alice")
	}
	if creds.Auth != "typed-password" {
		t.Errorf("Auth: got %q, want the prompted value", creds.Auth)
	}
}

// VALIDATES: AC-7 -- with no store and no username, resolution fails with an
// error naming the way forward, and never attempts the super-admin path.
//
// PREVENTS: an empty username comparing equal to an empty stored username and
// sending resolvePassword down hash-as-token with no store to read (a nil
// dereference), and preserves the credential error that routes `ze cli` to its
// offline fallback.
func TestReadCredentialsNoStoreNoUsername(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database.zefs") // never created

	t.Setenv("ze_ssh_password", "")
	t.Setenv("ze_ssh_username", "")
	env.ResetCache()
	prompted := stubPromptPolicy(t, false)

	_, err := ReadCredentialsWithFlags(dbPath, "")
	if err == nil {
		t.Fatal("expected an error when no store and no username are available")
	}
	got := err.Error()
	if !strings.Contains(got, "--user") || !strings.Contains(got, "ze.ssh.password") {
		t.Errorf("error %q must name --user and ze.ssh.password as the way forward", got)
	}
	if *prompted {
		t.Error("prompted with no username resolved")
	}
}

// VALIDATES: AC-8 -- a corrupt store is reported, not silently treated as "no
// credentials".
// PREVENTS: masking a real store bug as a confusing authentication failure.
func TestReadCredentialsCorruptStoreReportsError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "database.zefs")
	if err := os.WriteFile(dbPath, []byte("this is not a zefs store"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("ze_ssh_password", "alicepw")
	env.ResetCache()
	stubPromptPolicy(t, false)

	_, err := ReadCredentialsWithFlags(dbPath, "alice")
	if err == nil {
		t.Fatal("expected an error for a corrupt store, got a successful resolution")
	}
	if !strings.Contains(err.Error(), "open database") {
		t.Errorf("error %q must report the store failure, not hide it", err.Error())
	}
}

// VALIDATES: AC-9 -- a readable store with no flag or env still resolves to the
// super-admin via hash-as-token, exactly as before.
// PREVENTS: the store-optional change regressing the default local admin path.
func TestReadCredentialsSuperAdminStillResolves(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir())
	t.Setenv("ze_ssh_password", "")
	t.Setenv("ze_ssh_username", "")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true)

	creds, err := ReadCredentialsWithFlags(dbPath, "")
	if err != nil {
		t.Fatalf("ReadCredentialsWithFlags: %v", err)
	}
	if creds.Username != "admin" {
		t.Errorf("Username: got %q, want %q (zefs super-admin)", creds.Username, "admin")
	}
	if creds.Auth != "adminhash" {
		t.Errorf("Auth: got %q, want %q (hash-as-token)", creds.Auth, "adminhash")
	}
	if *prompted {
		t.Error("prompted on the super-admin path")
	}
}

// VALIDATES: a readable store with no entry for the requested host:port still
// resolves when the flag and env supply everything.
//
// PREVENTS: `ze cli --remote 192.0.2.10:2222 --user noc` failing with "no
// credentials for ..." despite a username and password being supplied -- the
// documented remote flow in docs/guide/ubuntu-build-install.md.
func TestReadCredentialsStoreWithoutEntryForRemote(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir()) // holds 10.0.0.1/2222 only

	t.Setenv("ze_ssh_password", "nocpw")
	env.ResetCache()
	stubPromptPolicy(t, false)

	creds, err := ReadCredentialsForRemote(dbPath, "noc", "192.0.2.10", "2222")
	if err != nil {
		t.Fatalf("ReadCredentialsForRemote for an unknown remote: %v", err)
	}
	if creds.Username != "noc" || creds.Auth != "nocpw" {
		t.Errorf("got %s/%s, want noc/nocpw", creds.Username, creds.Auth)
	}
	if creds.Host != "192.0.2.10" {
		t.Errorf("Host: got %q, want %q", creds.Host, "192.0.2.10")
	}
}

// stubPromptPolicy forces the tty answer and captures whether the password
// prompt was reached, restoring both seams when the test ends.
//
// The caller MUST NOT be a parallel test: this swaps package-level vars
// (isStdinTTY, passwordPrompter), so two tests doing it at once would race on
// the assignment and silently observe each other's stub. The t.Setenv calls
// these tests also make already make them non-parallel -- Go panics on
// t.Setenv in a parallel test -- so this is belt and braces, but the vars are
// the part with no runtime guard of their own.
func stubPromptPolicy(t *testing.T, tty bool) *bool {
	t.Helper()
	prompted := false

	origTTY := isStdinTTY
	isStdinTTY = func() bool { return tty }
	t.Cleanup(func() { isStdinTTY = origTTY })

	origPrompter := passwordPrompter
	passwordPrompter = func(string) (string, error) {
		prompted = true
		return "typed-password", nil
	}
	t.Cleanup(func() { passwordPrompter = origPrompter })

	return &prompted
}

// VALIDATES: AC-1 -- with prompting declined, resolution errors instead of
// blocking, EVEN when stdin is a terminal.
//
// PREVENTS: the tab-completion hang. Completion runs with stdin on the
// operator's terminal, so a tty check alone would prompt and freeze the shell
// mid-completion. The caller's policy, not the tty state, must decide.
func TestResolvePasswordNoPromptWhenDeclined(t *testing.T) {
	t.Setenv("ze_ssh_password", "")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true) // a terminal IS present

	_, err := resolvePassword(nil, "alice", "10.0.0.1", "2222", false, false)
	if err == nil {
		t.Fatal("expected an error when prompting is declined and no password source exists")
	}
	if *prompted {
		t.Error("passwordPrompter was called despite allowPrompt=false: completion would hang")
	}
	if got := err.Error(); !strings.Contains(got, "alice") || !strings.Contains(got, "ze.ssh.password") {
		t.Errorf("error %q must name the user and ze.ssh.password", got)
	}
}

// VALIDATES: AC-4 -- interactive callers still get their password prompt.
// PREVENTS: the no-prompt policy silently removing interactive login for
// `ze cli -u alice`, which docs/guide/authentication.md documents as supported.
func TestResolvePasswordPromptsWhenAllowed(t *testing.T) {
	t.Setenv("ze_ssh_password", "")
	env.ResetCache()
	prompted := stubPromptPolicy(t, true)

	pw, err := resolvePassword(nil, "alice", "10.0.0.1", "2222", false, true)
	if err != nil {
		t.Fatalf("resolvePassword: %v", err)
	}
	if !*prompted {
		t.Error("passwordPrompter was NOT called: interactive login is broken")
	}
	if pw != "typed-password" {
		t.Errorf("password: got %q, want the prompted value", pw)
	}
}

// VALIDATES: AC-5 -- a non-tty caller errors rather than prompting, even when
// prompting is allowed. This is the scripted `ze cli -u alice -c ...` path.
// PREVENTS: a script blocking forever on a prompt it can never answer.
func TestResolvePasswordNoTTYNeverPrompts(t *testing.T) {
	t.Setenv("ze_ssh_password", "")
	env.ResetCache()
	prompted := stubPromptPolicy(t, false) // no terminal

	if _, err := resolvePassword(nil, "alice", "10.0.0.1", "2222", false, true); err == nil {
		t.Fatal("expected an error when no tty and no password source")
	}
	if *prompted {
		t.Error("passwordPrompter was called with no tty")
	}
}

// VALIDATES: AC-2/AC-3 -- the env password and super-admin hash-as-token paths
// return before any prompt decision, so declining prompts cannot break them.
// PREVENTS: the no-prompt policy regressing super-admin completion, which must
// keep resolving via the zefs hash.
func TestResolvePasswordSourcesBypassPromptPolicy(t *testing.T) {
	dbPath := seedSuperAdminZefs(t, t.TempDir())
	store, err := zefs.Open(dbPath)
	if err != nil {
		t.Fatalf("zefs.Open: %v", err)
	}
	defer store.Close() //nolint:errcheck // read-only test access

	t.Run("env password wins even with prompting declined", func(t *testing.T) {
		t.Setenv("ze_ssh_password", "frompw")
		env.ResetCache()
		prompted := stubPromptPolicy(t, true)

		pw, err := resolvePassword(store, "alice", "10.0.0.1", "2222", false, false)
		if err != nil {
			t.Fatalf("resolvePassword: %v", err)
		}
		if pw != "frompw" {
			t.Errorf("password: got %q, want %q (env)", pw, "frompw")
		}
		if *prompted {
			t.Error("prompted despite an env password being available")
		}
	})

	t.Run("super-admin hash-as-token works with prompting declined", func(t *testing.T) {
		t.Setenv("ze_ssh_password", "")
		env.ResetCache()
		prompted := stubPromptPolicy(t, true)

		pw, err := resolvePassword(store, "admin", "10.0.0.1", "2222", true, false)
		if err != nil {
			t.Fatalf("resolvePassword: %v", err)
		}
		if pw != "adminhash" {
			t.Errorf("password: got %q, want %q (zefs hash-as-token)", pw, "adminhash")
		}
		if *prompted {
			t.Error("prompted despite the super-admin hash being available")
		}
	})
}

// VALIDATES: trimErrorPrefix strips the daemon's "error: " display prefix so a
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
			if got := trimErrorPrefix(tt.in); got != tt.want {
				t.Errorf("trimErrorPrefix(%q) = %q, want %q", tt.in, got, tt.want)
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

// TestReadAnswerFrameTakesItsCountsFromTheTerminator checks that the exec
// client reads the answer's outcome from the line whose kind says it ends the
// answer, and from no other line. The method: a frame carrying a head, its
// terminator and one more head is read, and the counts and verdict are compared
// with the terminator's.
//
// VALIDATES: the reader takes the kind directly rather than deriving it, so a
// line that is not the terminator moves nothing.
// PREVENTS: a stray frame line resetting the counts an operator's tooling acts
// on, which would report a complete answer as truncated.
func TestReadAnswerFrameTakesItsCountsFromTheTerminator(t *testing.T) {
	frame := strings.Join([]string{
		"top map 1:5:peers 1:0:",
		"end 1:2 1:1 1:0:",
		"top doc 1:0: 1:0:",
		"",
	}, "\n")

	answer, text := readAnswerFrame(strings.NewReader(frame))

	if answer.Count != 2 {
		t.Errorf("the frame reports count %d, want the terminator's 2", answer.Count)
	}
	if answer.Faults != 1 {
		t.Errorf("the frame reports faults %d, want the terminator's 1", answer.Faults)
	}
	if answer.Verdict != rpc.VerdictPartial {
		t.Errorf("the frame reports verdict %q, want %q from the terminator's counts", answer.Verdict, rpc.VerdictPartial)
	}
	if text != "" {
		t.Errorf("every line of the frame parsed as an answer line, so none is operator text; got %q", text)
	}
}
