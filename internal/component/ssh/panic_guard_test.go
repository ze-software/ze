// Design: docs/guide/operations.md -- SSH session panic containment
// Related: panic_guard.go -- sessionTask and authTask, the boundaries under test
// Related: answer_test.go -- answerServer, the exec path the session test drives

package ssh

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/crashlog"
)

// TestSSHHandlerPanicContained proves the session boundary ends only the SSH
// session whose handler met a Ze defect: its user is told in plain words on
// stderr and the session exits 1, while a second session on the same
// connection and a new connection are answered; the panic is counted and
// crash-recorded on the real guard.
// METHOD: answerServer's executor panics for the command "boom" on the exec
// goroutine inside execMiddleware. One client runs "boom", then "ok" on a new
// session of the same connection; a second client runs "ok".
// PREVENTS: an operator command that reaches a Ze defect killing the daemon
// and every SSH session with it, or ending the session with no word to its user.
func TestSSHHandlerPanicContained(t *testing.T) {
	before, declared := sessionGuard.Panics(taskSession)
	require.True(t, declared, "task %q is not declared on the ssh boundary", taskSession)
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	srv := answerServer(t, func(input string) (*plugin.Response, error) {
		if input == "boom" {
			panic("boom: handler defect")
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Records{Key: "commands", Rows: commandRecords(1)},
		}, nil
	})
	client := dialPanicTest(t, srv, "operator")

	stderr, err := runPanicTest(t, client, "boom")
	var exit *gossh.ExitError
	require.ErrorAs(t, err, &exit, "the panicking session must end with an exit status")
	require.Equal(t, 1, exit.ExitStatus())
	require.Contains(t, stderr, "error: an internal Ze defect ended this SSH session")
	require.Contains(t, stderr, "ze show crashes name ")

	_, err = runPanicTest(t, client, "ok")
	require.NoError(t, err, "a later session on the same connection must be answered")
	_, err = runPanicTest(t, dialPanicTest(t, srv, "operator"), "ok")
	require.NoError(t, err, "a new connection must be answered")
	require.Eventually(t, func() bool { return srv.ActiveSessions() == 0 }, 5*time.Second, 10*time.Millisecond,
		"the panicking session's slot must be released")

	count, _ := sessionGuard.Panics(taskSession)
	require.Equal(t, before+1, count, "the panic was not counted")
	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1)
	require.Equal(t, crashlog.KindRecovered, crashes[0].Kind)
	require.Contains(t, crashlog.ReadCrash(crashes[0].Name), "ssh "+taskSession+", unit user operator from 127.0.0.1:")
}

// panickingAuthenticator panics for the user "crash" and answers every other
// login as passwordProfilesAuthenticator does.
type panickingAuthenticator struct{}

func (panickingAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	if request.Username == "crash" {
		panic("boom: authenticator defect")
	}
	return passwordProfilesAuthenticator{}.Authenticate(request)
}

// TestSSHAuthPanicContained proves the auth boundary refuses a login whose
// check met a Ze defect, and the listener goes on accepting other logins.
// METHOD: the server's authenticator panics for the user "crash" on the
// connection's goroutine; "operator" then logs in and runs a command.
// PREVENTS: a login attempt that reaches a Ze defect in an authentication
// backend killing the daemon.
func TestSSHAuthPanicContained(t *testing.T) {
	before, declared := sessionGuard.Panics(taskAuth)
	require.True(t, declared, "task %q is not declared on the ssh boundary", taskAuth)
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	srv, err := NewServer(Config{
		Listen:        "127.0.0.1:0",
		HostKeyPath:   t.TempDir() + "/test_host_key",
		Authenticator: panickingAuthenticator{},
	})
	require.NoError(t, err)
	srv.SetExecutorFactory(func(_, _ string, _ plugin.Authorizer) CommandExecutor {
		return func(string) (*plugin.RenderedResponse, error) {
			return &plugin.RenderedResponse{Output: "pong"}, nil
		}
	})
	require.NoError(t, srv.Start(t.Context(), nil, nil))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(ctx))
	})

	_, err = gossh.Dial("tcp", srv.Address(), panicTestClientConfig("crash"))
	require.Error(t, err, "a login whose check panicked must be refused")

	_, err = runPanicTest(t, dialPanicTest(t, srv, "operator"), "ok")
	require.NoError(t, err, "a later login must be accepted and answered")

	count, _ := sessionGuard.Panics(taskAuth)
	require.Equal(t, before+1, count, "the panic was not counted")
	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1)
	require.Equal(t, crashlog.KindRecovered, crashes[0].Kind)
	require.Contains(t, crashlog.ReadCrash(crashes[0].Name), "ssh "+taskAuth+", unit login crash from 127.0.0.1:")
}

// panicTestClientConfig logs in as user with the read-only test password.
func panicTestClientConfig(user string) *gossh.ClientConfig {
	return &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // the test server key is generated per run
		Timeout:         5 * time.Second,
	}
}

// dialPanicTest opens one SSH connection to srv as user.
func dialPanicTest(t *testing.T, srv *Server, user string) *gossh.Client {
	t.Helper()
	client, err := gossh.Dial("tcp", srv.Address(), panicTestClientConfig(user))
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // best-effort cleanup
	return client
}

// runPanicTest runs command in a new session of client and returns its stderr
// and the run's error, an *gossh.ExitError when the session exits non-zero.
func runPanicTest(t *testing.T, client *gossh.Client, command string) (string, error) {
	t.Helper()
	session, err := client.NewSession()
	require.NoError(t, err)
	defer session.Close() //nolint:errcheck // best-effort cleanup

	var stdout, stderr strings.Builder
	session.Stdout = &stdout
	session.Stderr = &stderr
	runErr := session.Run(command)
	if runErr != nil && !errors.As(runErr, new(*gossh.ExitError)) {
		t.Fatalf("session %q failed outside an exit status: %v", command, runErr)
	}
	return stderr.String(), runErr
}
