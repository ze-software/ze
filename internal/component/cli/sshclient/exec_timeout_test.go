// Design: docs/architecture/doctor-and-health-checks.md — bounded SSH daemon ask
// Related: client.go — ExecCommandRawContext, execCommandContext
// Related: answer.go — dialDaemonContext

package sshclient

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestExecCommandRawContextBoundsHandshakeAndExec proves AC-3 on the SSH client
// transport: a context deadline bounds a listener that stalls before sending
// any SSH banner, a server that stalls during password authentication after
// host-key acceptance, and a server that stalls during command execution.
//
// VALIDATES: ExecCommandRawContext returns context.DeadlineExceeded promptly
// on all three stalls and never misclassifies an auth-phase timeout as
// *LoginRefused.
// PREVENTS: ze doctor hanging indefinitely when a daemon accepts TCP but
// stalls in SSH handshake, authentication, or show doctor execution.
func TestExecCommandRawContextBoundsHandshakeAndExec(t *testing.T) {
	t.Run("stalled-before-ssh-banner", func(t *testing.T) {
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}

		release := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close() //nolint:errcheck // test fixture cleanup
			<-release
		}()
		t.Cleanup(func() {
			close(release)
			_ = listener.Close()
			<-done
		})

		ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err = ExecCommandRawContext(ctx, Credentials{
			Host:     "127.0.0.1",
			Port:     port,
			Username: "operator",
			Auth:     "test-pass",
		}, "show doctor")
		elapsed := time.Since(start)

		if elapsed >= 2*time.Second {
			t.Fatalf("stalled banner took %v, expected prompt deadline return", elapsed)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled banner returned %v, want context.DeadlineExceeded", err)
		}
		if _, refused := errors.AsType[*LoginRefused](err); refused {
			t.Fatalf("stalled banner timeout was misclassified as *LoginRefused: %v", err)
		}
	})

	t.Run("stalled-during-password-auth", func(t *testing.T) {
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}

		release := make(chan struct{})
		done := make(chan struct{})
		signer := hostKeyTestSigner(t)
		go func() {
			defer close(done)
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close() //nolint:errcheck // test fixture cleanup
			cfg := &ssh.ServerConfig{
				PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
					<-release
					return nil, errors.New("stalled auth released")
				},
			}
			cfg.AddHostKey(signer)
			server, _, _, srvErr := ssh.NewServerConn(conn, cfg)
			if srvErr == nil {
				_ = server.Close()
			}
		}()
		t.Cleanup(func() {
			close(release)
			_ = listener.Close()
			<-done
		})

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err = ExecCommandRawContext(ctx, Credentials{
			Host:     "127.0.0.1",
			Port:     port,
			Username: "operator",
			Auth:     "test-pass",
		}, "show doctor")
		elapsed := time.Since(start)

		if elapsed >= 2*time.Second {
			t.Fatalf("stalled auth took %v, expected prompt deadline return", elapsed)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled auth returned %v, want context.DeadlineExceeded", err)
		}
		if _, refused := errors.AsType[*LoginRefused](err); refused {
			t.Fatalf("stalled auth timeout was misclassified as *LoginRefused: %v", err)
		}
	})

	t.Run("stalled-during-exec-command", func(t *testing.T) {
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}

		release := make(chan struct{})
		done := make(chan struct{})
		signer := hostKeyTestSigner(t)
		go func() {
			defer close(done)
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close() //nolint:errcheck // test fixture cleanup
			cfg := &ssh.ServerConfig{
				PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
					return &ssh.Permissions{}, nil
				},
			}
			cfg.AddHostKey(signer)
			server, channels, requests, srvErr := ssh.NewServerConn(conn, cfg)
			if srvErr != nil {
				return
			}
			defer server.Close() //nolint:errcheck // test fixture cleanup
			var wg sync.WaitGroup
			wg.Go(func() {
				ssh.DiscardRequests(requests)
			})
			for newCh := range channels {
				if newCh.ChannelType() != "session" {
					_ = newCh.Reject(ssh.UnknownChannelType, "unknown channel type")
					continue
				}
				ch, chReqs, chErr := newCh.Accept()
				if chErr != nil {
					break
				}
				for req := range chReqs {
					switch req.Type {
					case "env":
						_ = req.Reply(true, nil)
					case "exec":
						_ = req.Reply(true, nil)
						<-release
						_ = ch.Close()
						goto finish
					default:
						_ = req.Reply(false, nil)
					}
				}
				_ = ch.Close()
			}
		finish:
			_ = server.Close()
			wg.Wait()
		}()
		t.Cleanup(func() {
			close(release)
			_ = listener.Close()
			<-done
		})

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err = ExecCommandRawContext(ctx, Credentials{
			Host:     "127.0.0.1",
			Port:     port,
			Username: "operator",
			Auth:     "test-pass",
		}, "show doctor")
		elapsed := time.Since(start)

		if elapsed >= 2*time.Second {
			t.Fatalf("stalled exec took %v, expected prompt deadline return", elapsed)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled exec returned %v, want context.DeadlineExceeded", err)
		}
		if _, refused := errors.AsType[*LoginRefused](err); refused {
			t.Fatalf("stalled exec timeout was misclassified as *LoginRefused: %v", err)
		}
	})

	t.Run("refused-login-stays-login-refused", func(t *testing.T) {
		var listen net.ListenConfig
		listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}

		done := make(chan struct{})
		signer := hostKeyTestSigner(t)
		go func() {
			defer close(done)
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close() //nolint:errcheck // test fixture cleanup
			cfg := &ssh.ServerConfig{
				PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
					return nil, errors.New("bad password")
				},
			}
			cfg.AddHostKey(signer)
			server, _, _, srvErr := ssh.NewServerConn(conn, cfg)
			if srvErr == nil {
				_ = server.Close()
			}
		}()
		t.Cleanup(func() {
			_ = listener.Close()
			<-done
		})

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		_, err = ExecCommandRawContext(ctx, Credentials{
			Host:     "127.0.0.1",
			Port:     port,
			Username: "operator",
			Auth:     "wrong-pass",
		}, "show doctor")
		if _, refused := errors.AsType[*LoginRefused](err); !refused {
			t.Fatalf("refused password returned %v, want *LoginRefused", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("refused password returned context.DeadlineExceeded: %v", err)
		}
	})
}
