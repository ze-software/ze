package appliance

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// testHostKey returns a fresh ed25519 SSH public key for use as an appliance
// host key. Each call returns a distinct key.
func testHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("wrap ssh public key: %v", err)
	}
	return key
}

// writeKnownHosts writes a known_hosts file pinning key for addr and returns its path.
func writeKnownHosts(t *testing.T, addr string, key ssh.PublicKey) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{addr}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}

// VALIDATES: AC-5 -- pushing to an appliance whose host key is not in known_hosts
// fails, and the error names the host, the file, and the ssh-keyscan remediation.
// PREVENTS: ssh.InsecureIgnoreHostKey accepting any key, letting a machine on the
// path impersonate the appliance, read the pushed config, and forge the result.
func TestSSHExecRefusesUnknownHost(t *testing.T) {
	pinned := testHostKey(t)
	path := writeKnownHosts(t, "10.0.0.5:22", pinned)

	cb, err := applianceHostKeyCallback(path)
	if err != nil {
		t.Fatalf("applianceHostKeyCallback: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.9"), Port: 22}
	err = cb("10.0.0.9:22", addr, testHostKey(t))

	if err == nil {
		t.Fatal("unknown host was accepted, want refusal")
	}
	for _, want := range []string{"10.0.0.9", path, "ssh-keyscan"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// VALIDATES: AC-7 -- a host whose presented key differs from the pinned one is refused.
// PREVENTS: Treating "the host is known" as sufficient and skipping the key comparison,
// which would accept a substituted key for a host already in known_hosts.
func TestSSHExecRejectsChangedHostKey(t *testing.T) {
	pinned := testHostKey(t)
	path := writeKnownHosts(t, "10.0.0.5:22", pinned)

	cb, err := applianceHostKeyCallback(path)
	if err != nil {
		t.Fatalf("applianceHostKeyCallback: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 22}
	err = cb("10.0.0.5:22", addr, testHostKey(t))

	if err == nil {
		t.Fatal("changed host key was accepted, want refusal")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("error %q should report a host key mismatch", err)
	}
	if strings.Contains(err.Error(), "ssh-keyscan") {
		t.Errorf("a mismatch must not suggest ssh-keyscan (that would paper over an attack): %q", err)
	}
}

// VALIDATES: AC-6 -- a host pinned in known_hosts with a matching key is accepted.
// PREVENTS: The new guard failing closed on every host and breaking fleet push.
func TestSSHExecAcceptsPinnedHostKey(t *testing.T) {
	pinned := testHostKey(t)
	path := writeKnownHosts(t, "10.0.0.5:22", pinned)

	cb, err := applianceHostKeyCallback(path)
	if err != nil {
		t.Fatalf("applianceHostKeyCallback: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 22}
	if err := cb("10.0.0.5:22", addr, pinned); err != nil {
		t.Fatalf("pinned host key was refused: %v", err)
	}
}

// VALIDATES: AC-5 -- a missing known_hosts file fails closed with a readable error.
// PREVENTS: An absent file being treated as "nothing to check" and falling through
// to an unverified connection (ai/rules/fail-closed-guards.md).
func TestSSHExecRefusesMissingKnownHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "known_hosts")

	cb, err := applianceHostKeyCallback(path)

	if err == nil {
		t.Fatal("missing known_hosts produced a usable callback, want an error")
	}
	if cb != nil {
		t.Error("a failed callback lookup must not return a callback")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the missing file", err)
	}
}
