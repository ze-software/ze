//go:build linux

// Design: docs/architecture/cli/error-surface.md -- how an operator error reads on each surface
// Related: answer.go -- dialDaemon, the dial these cases drive
// Related: refusal.go -- LoginRefused and LoginRefusal

package sshclient

import (
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh/knownhosts"
)

// TestDialRefusedLoginIsLoginRefused tells a daemon that refused the login
// from one that was never reached, over a real SSH handshake.
//
// Method: a real SSH server on loopback accepts only operator/host-key-proof.
// The client dials it with the wrong password: the error MUST be a
// *LoginRefused naming the address and the user, and its refusal MUST name the
// login fix, never a start. Two negatives hold the condition, not a blanket
// classification: a port nothing listens on, and a server whose host key the
// client refuses, each answer a dial error that is no *LoginRefused, because
// neither proves a daemon answered.
// PREVENTS: `ze cli` serving an offline fallback ("no daemon answered") for a
// daemon that runs and refused the stored credentials.
func TestDialRefusedLoginIsLoginRefused(t *testing.T) {
	t.Run("refused-login", func(t *testing.T) {
		hostKeyTestEnvironment(t)
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- serveKnownHostCommand(listener, hostKeyTestSigner(t)) }()
		t.Cleanup(func() {
			_ = listener.Close()
			<-done // the server's own error is the refused login it was asked to make
		})

		_, err = ExecCommand(Credentials{Host: "127.0.0.1", Port: port, Username: "operator", Auth: "wrong"}, "show version")
		refused, ok := errors.AsType[*LoginRefused](err)
		if !ok {
			t.Fatalf("a refused login answered %v, want a *LoginRefused", err)
		}
		address := net.JoinHostPort("127.0.0.1", port)
		if refused.Address != address || refused.User != "operator" {
			t.Fatalf("refusal names %s for %s, want %s for operator", refused.Address, refused.User, address)
		}
		message := LoginRefusal(refused).Error()
		if !strings.HasPrefix(message, "the daemon at "+address+" refused the login of operator\n") {
			t.Fatalf("refusal does not open with the refused login:\n%s", message)
		}
		if !strings.Contains(message, "\nfix: ze cli --user <name>") {
			t.Fatalf("refusal does not name the login fix:\n%s", message)
		}
	})

	t.Run("nothing-listening", func(t *testing.T) {
		hostKeyTestEnvironment(t)
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}

		_, err = ExecCommand(Credentials{Host: "127.0.0.1", Port: port, Username: "operator", Auth: "wrong"}, "show version")
		if err == nil {
			t.Fatal("a dial to a closed port succeeded")
		}
		if _, ok := errors.AsType[*LoginRefused](err); ok {
			t.Fatalf("a port nothing listens on answered a refused login: %v", err)
		}
	})

	t.Run("host-key-refused", func(t *testing.T) {
		home := hostKeyTestEnvironment(t)
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.2:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		address := net.JoinHostPort("127.0.0.2", port)
		// The recorded key is not the one the server presents, so the client
		// refuses the host key before any login is tried.
		writeHostKeyTestFile(t, filepath.Join(home, ".ssh", "known_hosts"),
			knownhosts.Line([]string{address}, hostKeyTestSigner(t).PublicKey())+"\n")
		done := make(chan error, 1)
		go func() { done <- serveKnownHostCommand(listener, hostKeyTestSigner(t)) }()
		t.Cleanup(func() {
			_ = listener.Close()
			<-done // the server's own error is the handshake the client refused
		})

		_, err = ExecCommand(Credentials{Host: "127.0.0.2", Port: port, Username: "operator", Auth: "host-key-proof"}, "show version")
		if err == nil {
			t.Fatal("a changed host key was accepted")
		}
		if _, ok := errors.AsType[*LoginRefused](err); ok {
			t.Fatalf("a refused host key answered a refused login: %v", err)
		}
	})
}
