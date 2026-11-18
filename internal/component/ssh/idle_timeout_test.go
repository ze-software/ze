// Design: docs/architecture/api/architecture.md -- monitor streaming over SSH
// Related: ssh.go -- Start applies the configured connection timeout

package ssh

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	sshIdleTimeoutSeconds = 2
	sshIdleTimeoutPeriod  = sshIdleTimeoutSeconds * time.Second
	sshIdleTimeoutCommand = "monitor ssh-idle-timeout-test"
	sshIdleTimeoutEvents  = 12
)

type sshIdleTimeoutEvent struct {
	Sequence int `json:"sequence"`
}

// TestSSHIdleTimeoutKeepsAnActiveSession reads ordered monitor events through a
// real authenticated exec channel for longer than the configured idle timeout.
// Every event is useful output on the same session, rather than a transport-only
// keepalive. The final event and clean exit prove that output survived the bound.
// VALIDATES: spec-web-cli-config-one-off-defects AC-24.
// PREVENTS: treating idle-timeout as an absolute connection lifetime.
func TestSSHIdleTimeoutKeepsAnActiveSession(t *testing.T) {
	srv := newSSHIdleTimeoutServer(t, true)
	session, output := openSSHIdleTimeoutSession(t, srv)
	decoder := json.NewDecoder(output)
	var ready sshIdleTimeoutEvent
	require.NoError(t, decoder.Decode(&ready), "the monitor must be ready before measuring its lifetime")
	require.Equal(t, 0, ready.Sequence)
	started := time.Now()

	for sequence := 1; sequence <= sshIdleTimeoutEvents; sequence++ {
		var event sshIdleTimeoutEvent
		require.NoError(t, decoder.Decode(&event), "active monitor ended before event %d", sequence)
		require.Equal(t, sequence, event.Sequence, "every ordered event must reach the same session")
	}
	require.Greater(t, time.Since(started), sshIdleTimeoutPeriod,
		"the final event must arrive after the configured idle duration")
	require.NoError(t, session.Wait(), "the active monitor must finish normally")
}

// TestSSHIdleTimeoutClosesAnIdleSession receives the same readiness event, then
// leaves the connection silent. EOF must come from the server; the client's
// deadline only bounds a failure if idle-timeout stops being enforced.
// VALIDATES: spec-web-cli-config-one-off-defects AC-24, the idle control.
// PREVENTS: preserving active sessions by disabling the timeout altogether.
func TestSSHIdleTimeoutClosesAnIdleSession(t *testing.T) {
	srv := newSSHIdleTimeoutServer(t, false)
	_, output := openSSHIdleTimeoutSession(t, srv)
	decoder := json.NewDecoder(output)
	var ready sshIdleTimeoutEvent
	require.NoError(t, decoder.Decode(&ready), "an unreachable monitor cannot prove idle expiry")
	require.Equal(t, 0, ready.Sequence)
	var unexpected sshIdleTimeoutEvent
	require.ErrorIs(t, decoder.Decode(&unexpected), io.EOF,
		"the server must close the idle connection before the client deadline")
}

// newSSHIdleTimeoutServer uses the production streaming exec path. Its stream
// runs on the server-owned session goroutine: context cancellation stops it,
// and Server.Stop joins it before the test releases the server.
func newSSHIdleTimeoutServer(t *testing.T, active bool) *Server {
	t.Helper()
	pluginserver.RegisterStreamingHandler(sshIdleTimeoutCommand, pluginserver.OneShotSample,
		func(context.Context, *pluginserver.Server, io.Writer, string, command.ValidatedArgs) error {
			return nil
		})
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(sshIdleTimeoutCommand) })
	srv, err := NewServer(Config{
		Listen:        "127.0.0.1:0",
		HostKeyPath:   t.TempDir() + "/test_host_key",
		Authenticator: passwordProfilesAuthenticator{},
		IdleTimeout:   sshIdleTimeoutSeconds,
	})
	require.NoError(t, err)
	srv.SetStreamingExecutorFactory(func(_, _ string, _ plugin.Authorizer) StreamingExecutor {
		return func(ctx context.Context, output io.Writer, _ []string) error {
			encoder := json.NewEncoder(output)
			if err := encoder.Encode(sshIdleTimeoutEvent{Sequence: 0}); err != nil {
				return err
			}
			if !active {
				<-ctx.Done()
				return nil
			}
			// The eight-to-one margin keeps normal scheduling jitter below the
			// idle deadline. Twelve ticks carry this session beyond that deadline.
			ticker := time.NewTicker(sshIdleTimeoutPeriod / 8)
			defer ticker.Stop()
			for sequence := 1; sequence <= sshIdleTimeoutEvents; sequence++ {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := encoder.Encode(sshIdleTimeoutEvent{Sequence: sequence}); err != nil {
						return err
					}
				}
			}
			return nil
		}
	})
	require.NoError(t, srv.Start(t.Context(), nil, nil))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(ctx))
	})
	return srv
}

// openSSHIdleTimeoutSession bounds every client operation with one connection
// deadline. Closing and waiting for the client joins its transport goroutines
// before Server.Stop runs; an already-expired connection can return EOF on close.
func openSSHIdleTimeoutSession(t *testing.T, srv *Server) (*gossh.Session, io.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", srv.Address(), time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // the client may already have closed this connection
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*sshIdleTimeoutPeriod)))
	transport, channels, requests, err := gossh.NewClientConn(conn, srv.Address(), &gossh.ClientConfig{
		User:            "operator",
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run
	})
	require.NoError(t, err)
	client := gossh.NewClient(transport, channels, requests)
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // expected when the idle timeout has already closed the connection
		client.Wait()  //nolint:errcheck // EOF is the expected transport result after closing it
	})
	session, err := client.NewSession()
	require.NoError(t, err)
	output, err := session.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, session.Start(sshIdleTimeoutCommand+" | raw"))
	return session, output
}
