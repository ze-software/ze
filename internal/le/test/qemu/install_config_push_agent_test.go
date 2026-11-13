// Design: docs/architecture/testing/qemu-integration.md -- private config-push identity ownership.
// Upstream: golang.org/x/crypto v0.58.0, ssh/agent/client.go -- one pipeline reader per client connection.
// https://github.com/golang/crypto/tree/v0.58.0/ssh/agent.
package testqemu

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type configPushIdentityStarter interface {
	configPushIdentity(string, ed25519.PrivateKey) (string, string, func() error, error)
}

// TestApplianceConfigPushAgentOwnsWorkers proves the real agent protocol and
// shutdown of an accepted, idle connection; closing only the listener leaks it.
func TestApplianceConfigPushAgentOwnsWorkers(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	starter, ok := any(installer).(configPushIdentityStarter)
	if !ok {
		t.Fatal("native proof has no owned real ssh-agent")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	beforeHome, beforeSocket := os.Getenv("HOME"), os.Getenv("SSH_AUTH_SOCK")
	work := t.TempDir()
	home, socket, stop, err := starter.configPushIdentity(work, private)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Error(err)
		}
	})
	if filepath.Dir(home) != work || filepath.Dir(socket) != work {
		t.Fatalf("identity escapes owned scratch: home=%q socket=%q", home, socket)
	}
	for _, path := range []string{home, filepath.Join(home, ".ssh"), socket} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("identity mode %s: %v", path, info.Mode())
		}
	}
	connection, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := agent.NewClient(connection)
	keys, err := client.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("agent keys=%d, want one", len(keys))
	}
	signer, err := client.Signers()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := signer[0].Sign(rand.Reader, []byte("owned native proof"))
	if err != nil {
		t.Fatal(err)
	}
	if err := keys[0].Verify([]byte("owned native proof"), signature); err != nil {
		t.Fatal(err)
	}
	wantKey, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(keys[0].Marshal(), wantKey.Marshal()) {
		t.Fatal("agent serves an unrelated identity")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- stop() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		_ = connection.Close()
		<-stopped
		t.Fatal("agent cleanup did not close and join its accepted idle connection")
	}
	if _, err := client.List(); err == nil {
		t.Fatal("agent connection survived cleanup")
	}
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("agent socket survived cleanup: %v", err)
	}
	if os.Getenv("HOME") != beforeHome || os.Getenv("SSH_AUTH_SOCK") != beforeSocket {
		t.Fatal("native identity changed process-wide authentication environment")
	}
}
