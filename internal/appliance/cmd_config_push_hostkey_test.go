package appliance

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
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
// to an unverified connection (ai/rules/evidence.md).
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

// VALIDATES: AC-5 -- sshExecReal wires the verifying callback, not
// ssh.InsecureIgnoreHostKey.
// PREVENTS: The finding this change exists to close silently returning. The four
// tests above exercise applianceHostKeyCallback in isolation, so reverting the
// two lines in sshExecReal that consume it would leave every one of them green
// while the push went back to trusting any key. Production never runs
// sshExecReal in tests (the sshExecFunc seam replaces it wholesale), so the call
// site itself has to be asserted.
func TestSSHExecRealUsesVerifyingHostKeyCallback(t *testing.T) {
	src, err := os.ReadFile("cmd_config_push.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	body := string(src)

	if strings.Contains(body, "InsecureIgnoreHostKey") {
		t.Error("cmd_config_push.go references ssh.InsecureIgnoreHostKey: " +
			"the appliance push must verify the host key it talks to")
	}
	if !strings.Contains(body, "HostKeyCallback: hostKeys") {
		t.Error("sshExecReal does not set HostKeyCallback from applianceHostKeyCallback")
	}
	if !strings.Contains(body, "applianceHostKeyCallback(userKnownHostsPath())") {
		t.Error("sshExecReal does not resolve the callback from the operator's known_hosts")
	}
}

// VALIDATES: AC-5 -- with no resolvable home directory the push refuses rather
// than falling back to an unverified connection.
// PREVENTS: A guard that fails open when it cannot find its own trust store.
func TestApplianceHostKeyCallbackRefusesEmptyPath(t *testing.T) {
	cb, err := applianceHostKeyCallback("")

	if err == nil {
		t.Fatal("an empty known_hosts path produced a usable callback")
	}
	if cb != nil {
		t.Error("a failed lookup must not return a callback")
	}
	if !errors.Is(err, errNoKnownHostsPath) {
		t.Errorf("error = %v, want errNoKnownHostsPath", err)
	}
}

// VALIDATES: the ssh-keyscan remediation is correct for a non-default port.
// PREVENTS: Handing the operator a command that silently pins the wrong thing:
// `ssh-keyscan -H host:2222` treats the whole string as a hostname.
func TestKeyscanHintPortHandling(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"default port", "10.0.0.5:22", "ssh-keyscan -H 10.0.0.5 >> /kh"},
		{"custom port", "10.0.0.5:2222", "ssh-keyscan -H -p 2222 10.0.0.5 >> /kh"},
		{"no port", "10.0.0.5", "ssh-keyscan -H 10.0.0.5 >> /kh"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := keyscanHint(tc.host, "/kh"); got != tc.want {
				t.Errorf("keyscanHint(%q) = %q, want %q", tc.host, got, tc.want)
			}
		})
	}
}
