package completion

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

// seedZefs writes a store with a super-admin entry so credential resolution gets
// past the store and reaches the password step, which is what this test targets.
func seedZefs(t *testing.T, dir string) {
	t.Helper()
	store, err := zefs.Create(filepath.Join(dir, "database.zefs"))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
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
	if err := store.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
}

// TestWritePeersNeverPromptsOnTTY is the wiring test for the completion hang.
//
// Tab completion runs with stdin attached to the operator's terminal. Credential
// resolution prompts for a password for any non-super-admin user with no
// ze.ssh.password (internal/core/ssh/client/client.go resolvePassword). An
// operator who follows the documented completion setup in
// docs/guide/authentication.md ("export ZE_SSH_USERNAME=alice", password left to a
// secret store) therefore blocks their own shell on TAB: the prompt reads stdin
// and never returns. peers.go guards resolution *errors*, but a prompt is not an
// error, it is a block.
//
// The test replaces stdin with a real pty so isStdinTTY() reports true, then
// requires writePeers to return. Before the fix it blocks and this test fails on
// the deadline; after it, resolution declines the prompt, returns an error, and
// writePeers falls back to "no completions".
//
// This test replaces os.Stdin, a process-wide global, so it must never call
// t.Parallel: a concurrent test would observe the pty as its own stdin. The
// t.Setenv calls below already enforce that (the testing package refuses
// t.Parallel for any test using t.Setenv), but os.Stdin has no such guard of
// its own.
//
// VALIDATES: AC-1 -- completion never issues a password prompt, even on a TTY.
// PREVENTS: a hung shell on TAB for any operator with a username but no password.
func TestWritePeersNeverPromptsOnTTY(t *testing.T) {
	dir := t.TempDir()
	seedZefs(t, dir)

	t.Setenv("ze_config_dir", dir)
	t.Setenv("ze_ssh_username", "alice") // not the super-admin -> needs a real password
	t.Setenv("ze_ssh_password", "")      // no password source -> prompt path on a TTY
	env.ResetCache()
	defer env.ResetCache()

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer func() {
		if err := ptmx.Close(); err != nil {
			t.Logf("ptmx.Close: %v", err)
		}
	}()
	defer func() {
		if err := tty.Close(); err != nil {
			t.Logf("tty.Close: %v", err)
		}
	}()

	// Drain the master so a prompt written to the tty cannot block on a full pipe:
	// the failure to observe is the blocking READ, not a blocked write.
	go func() {
		if _, err := io.Copy(io.Discard, ptmx); err != nil {
			return // master closed at test end; nothing to report
		}
	}()

	origStdin := os.Stdin
	os.Stdin = tty
	defer func() { os.Stdin = origStdin }()

	done := make(chan int, 1)
	go func() {
		var buf strings.Builder
		done <- writePeers(&buf)
	}()

	select {
	case code := <-done:
		assert.Equal(t, 0, code, "writePeers must degrade to no completions, not fail")
	case <-time.After(5 * time.Second):
		// Unblock the prompt so the leaked goroutine can finish and the test
		// binary can exit cleanly.
		if _, err := ptmx.WriteString("\n"); err != nil {
			t.Logf("unblocking pty write: %v", err)
		}
		t.Fatal("writePeers blocked on a password prompt: tab completion would hang the operator's shell")
	}
}

// TestCompletionPeersOutput verifies the formatting of peer selector completions.
//
// VALIDATES: AC-8 — output contains peer names, IPs, and as<N> values.
// VALIDATES: AC-11 — ASN values are deduplicated across peers.
// PREVENTS: Completion output with wrong format or missing selectors.
func TestCompletionPeersOutput(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"name":"upstream","remote-as":65001,"state":"established"},` +
		`"10.0.0.2":{"name":"downstream","remote-as":65002,"state":"established"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// Each peer produces 3 entries (name + IP + ASN) = 6 total
	assert.Len(t, lines, 6, "expected 6 completion lines (2 peers x 3 entries)")

	// Verify name entries
	assert.Contains(t, out, "upstream\tpeer name (10.0.0.1 AS 65001)")
	assert.Contains(t, out, "downstream\tpeer name (10.0.0.2 AS 65002)")

	// Verify IP entries
	assert.Contains(t, out, "10.0.0.1\tpeer ip (upstream AS 65001)")
	assert.Contains(t, out, "10.0.0.2\tpeer ip (downstream AS 65002)")

	// Verify ASN entries
	assert.Contains(t, out, "as65001\tpeer asn (upstream 10.0.0.1)")
	assert.Contains(t, out, "as65002\tpeer asn (downstream 10.0.0.2)")
}

// TestCompletionPeersASNDedup verifies that shared ASNs produce one completion entry.
//
// VALIDATES: AC-11 — two peers with same ASN produce one as<N> entry.
// PREVENTS: Duplicate ASN entries cluttering completion output.
func TestCompletionPeersASNDedup(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"name":"ibgp-a","remote-as":65000,"state":"established"},` +
		`"10.0.0.2":{"name":"ibgp-b","remote-as":65000,"state":"established"},` +
		`"10.0.1.1":{"name":"ebgp","remote-as":65001,"state":"established"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()

	// Count as65000 entries — should be exactly 1 despite 2 peers with that ASN
	count := strings.Count(out, "as65000\t")
	assert.Equal(t, 1, count, "as65000 should appear exactly once (deduplicated)")

	// as65001 should appear once
	count = strings.Count(out, "as65001\t")
	assert.Equal(t, 1, count, "as65001 should appear exactly once")
}

// TestCompletionPeersNoName verifies output for peers without configured names.
//
// VALIDATES: Peers without names still produce IP and ASN entries.
// PREVENTS: Missing completions for unnamed peers.
func TestCompletionPeersNoName(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"remote-as":65001,"state":"established"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// No name entry, just IP + ASN = 2 entries
	assert.Len(t, lines, 2, "expected 2 completion lines (IP + ASN, no name)")

	assert.Contains(t, out, "10.0.0.1\tpeer ip (AS 65001)")
	assert.Contains(t, out, "as65001\tpeer asn (10.0.0.1)")
}

// TestCompletionPeersEmptyJSON verifies graceful handling of empty peer list.
//
// VALIDATES: Empty peer list produces no output and returns 0.
// PREVENTS: Crash or error on empty peer list.
func TestCompletionPeersEmptyJSON(t *testing.T) {
	var buf strings.Builder
	code := formatPeerCompletions(&buf, `{"peers":{}}`)
	assert.Equal(t, 0, code)
	assert.Empty(t, buf.String())
}

// TestCompletionPeersInvalidJSON verifies graceful handling of invalid JSON.
//
// VALIDATES: AC-9 — invalid input produces no output and returns 0.
// PREVENTS: Crash on malformed daemon response.
func TestCompletionPeersInvalidJSON(t *testing.T) {
	var buf strings.Builder
	code := formatPeerCompletions(&buf, "not json")
	assert.Equal(t, 0, code)
	assert.Empty(t, buf.String())
}

// TestCompletionPeersASNZero verifies output for peers with ASN 0 (reserved).
//
// VALIDATES: RemoteAS=0 produces "as0" in completion output.
// PREVENTS: Empty or missing ASN string for zero-value ASN.
func TestCompletionPeersASNZero(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"name":"misconfigured","remote-as":0,"state":"Idle"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()
	assert.Contains(t, out, "as0\tpeer asn (misconfigured 10.0.0.1)")
	assert.Contains(t, out, "10.0.0.1\tpeer ip (misconfigured AS 0)")
}

// TestCompletionPeersDispatch verifies the peers subcommand is reachable via Run.
//
// VALIDATES: "ze completion peers" dispatch works.
// PREVENTS: peers being wired to internal function but not to Run.
func TestCompletionPeersDispatch(t *testing.T) {
	// Will return 0 even without daemon (graceful fallback)
	code := Run([]string{"peers"})
	assert.Equal(t, 0, code)
}

// TestShellScriptsIncludeCompletionPeers verifies all 4 shell scripts call
// ze completion peers for dynamic peer selector completion.
//
// VALIDATES: AC-10 — shell completion after "peer" includes dynamic selectors.
// PREVENTS: Shell scripts missing the peer selector integration.
func TestShellScriptsIncludeCompletionPeers(t *testing.T) {
	shells := []struct {
		name string
		gen  string
	}{
		{"bash", "bash"},
		{"zsh", "zsh"},
		{"fish", "fish"},
		{"nushell", "nushell"},
	}

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			var buf strings.Builder
			code := generate(sh.gen, &buf)
			assert.Equal(t, 0, code)
			assert.Contains(t, buf.String(), "completion peers",
				"%s script should call 'ze completion peers' for dynamic peer selectors", sh.name)
		})
	}
}
