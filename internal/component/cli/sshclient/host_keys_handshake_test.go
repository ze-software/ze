//go:build linux

// Design: docs/guide/operations.md -- remote trust reaches the real SSH dial.

package sshclient

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Linux routes all of 127/8 to loopback. 127.0.0.2 deliberately exercises the
// remote trust policy rather than the preserved 127.0.0.1 implicit trust path.
func TestRemoteKnownHostExecCommandHandshake(t *testing.T) {
	for _, trusted := range []bool{true, false} {
		t.Run(map[bool]string{true: "trusted", false: "changed"}[trusted], func(t *testing.T) {
			home := hostKeyTestEnvironment(t)
			serverKey, recordedKey := hostKeyTestSigner(t), hostKeyTestSigner(t)
			if trusted {
				recordedKey = serverKey
			}
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
			writeHostKeyTestFile(t, filepath.Join(home, ".ssh", "known_hosts"), knownhosts.Line([]string{address}, recordedKey.PublicKey())+"\n")
			done := make(chan error, 1)
			go func() { done <- serveKnownHostCommand(listener, serverKey) }()
			t.Cleanup(func() {
				_ = listener.Close()
				if err := <-done; trusted && err != nil {
					t.Errorf("SSH proof server: %v", err)
				}
			})
			output, err := ExecCommand(Credentials{Host: "127.0.0.2", Port: port, Username: "operator", Auth: "host-key-proof"}, "show version")
			if trusted {
				if err != nil || output != "verified-host-key-command" {
					t.Fatalf("real trusted command output=%q err=%v", output, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "changed") {
				t.Fatalf("changed server key did not fail handshake: output=%q err=%v", output, err)
			}
		})
	}
}

func TestRemoteExecCommandUsesBracketedIPv6Address(t *testing.T) {
	hostKeyTestEnvironment(t)
	var listen net.ListenConfig
	listener, err := listen.Listen(t.Context(), "tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("host has no IPv6 loopback listener: %v", err)
	}
	signer := hostKeyTestSigner(t)
	done := make(chan error, 1)
	go func() { done <- serveKnownHostCommand(listener, signer) }()
	t.Cleanup(func() {
		_ = listener.Close()
		if err := <-done; err != nil {
			t.Errorf("SSH proof server: %v", err)
		}
	})
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	output, err := ExecCommand(Credentials{Host: "::1", Port: port, Username: "operator", Auth: "host-key-proof"}, "show version")
	if err != nil || output != "verified-host-key-command" {
		t.Fatalf("IPv6 client output=%q err=%v", output, err)
	}
}

func serveKnownHostCommand(listener net.Listener, signer ssh.Signer) error {
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != "operator" || string(password) != "host-key-proof" {
			return nil, errors.New("invalid test credentials")
		}
		return &ssh.Permissions{}, nil
	}}
	return serveKnownHostCommandConfig(listener, signer, config)
}

func serveKnownHostCommandConfig(listener net.Listener, signer ssh.Signer, config *ssh.ServerConfig) (resultErr error) {
	connection, err := listener.Accept()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		if err := connection.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}()
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	config.AddHostKey(signer)
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return err
	}
	// The accepted connection above owns transport cleanup on every path.
	go ssh.DiscardRequests(requests)
	for proposed := range channels {
		if proposed.ChannelType() != "session" {
			_ = proposed.Reject(ssh.UnknownChannelType, "session required")
			continue
		}
		channel, channelRequests, err := proposed.Accept()
		if err != nil {
			return err
		}
		for request := range channelRequests {
			if request.Type == "env" {
				_ = request.Reply(true, nil)
				continue
			}
			if request.Type != "exec" {
				_ = request.Reply(false, nil)
				continue
			}
			var payload struct{ Command string }
			if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
				_ = channel.Close()
				return err
			}
			if payload.Command != "show version" {
				_ = channel.Close()
				return fmt.Errorf("unexpected command %q", payload.Command)
			}
			if err := request.Reply(true, nil); err != nil {
				_ = channel.Close()
				return err
			}
			if _, err := channel.Write([]byte("verified-host-key-command\n")); err != nil {
				_ = channel.Close()
				return err
			}
			_, err := channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{}))
			closeErr := channel.Close()
			if err != nil || closeErr != nil {
				return errors.Join(err, closeErr)
			}
			_ = server.Wait()
			return nil
		}
		_ = channel.Close()
	}
	return nil
}
