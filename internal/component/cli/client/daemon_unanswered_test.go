// Design: docs/architecture/api/commands.md -- the offline fallback and the cause it is handed
// Related: daemon_unanswered.go -- the causes these judge
// Related: main.go -- endFailedAsk and runOfflineFallback, the route these drive

package client

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
)

// fallbackProbePath is a real model path that ships no fallback of its own
// (leaf name, length 1..128, mandatory), so a test registers one there, and
// fallbackProbeLine is a line that satisfies its model.
const (
	fallbackProbePath = "show metrics name"
	fallbackProbeLine = fallbackProbePath + " m"
)

// handedCause is what a recording fallback was run with.
type handedCause struct {
	ran   int
	cause command.DaemonUnanswered
}

// recordFallback registers a fallback at fallbackProbePath that records the
// cause it is handed and exits 1, as a fallback that can only refuse does. The
// registry is process-wide: every test that reads the recording registers its
// own first. Not safe for parallel tests.
func recordFallback(t *testing.T) *handedCause {
	t.Helper()
	handed := &handedCause{}
	if err := command.RegisterOfflineFallback(fallbackProbePath, func(_ command.ValidatedArgs, cause command.DaemonUnanswered) int {
		handed.ran++
		handed.cause = cause
		return 1
	}); err != nil {
		t.Fatal(err)
	}
	return handed
}

// listenLoopback opens a TCP listener on loopback that stays open for the
// test: a connect to it succeeds, as a probe of a running daemon does.
func listenLoopback(t *testing.T) (host, port string) {
	t.Helper()
	var listen net.ListenConfig
	listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	host, port, err = net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

// closedLoopbackPort answers a loopback port nothing listens on: one the
// kernel just handed out, closed again before it is used.
func closedLoopbackPort(t *testing.T) (host, port string) {
	t.Helper()
	var listen net.ListenConfig
	listener, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, port, err = net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return host, port
}

// localDaemonAt makes host:port the address this client resolves for the
// local daemon, over an empty store, as an operator's environment would. Not
// safe for parallel tests: the environment is process state.
func localDaemonAt(t *testing.T, host, port string) {
	t.Helper()
	for key, value := range map[string]string{
		"ze.config.dir": t.TempDir(),
		"ze.ssh.host":   host,
		"ze.ssh.port":   port,
	} {
		t.Setenv(key, value)
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// VALIDATES: AC-32, no listener -- a failed connection to an address nothing
// listens on is "no ze listens at <address>", and its fix is the start.
// Method: the cause of a failed first request to a closed loopback port.
func TestOfflineCauseNoListenerIsAStart(t *testing.T) {
	host, port := closedLoopbackPort(t)
	address := net.JoinHostPort(host, port)
	cause := unansweredAfterConnect("show host trend", host, port, errors.New("connect to "+address+": connection refused"))
	if !strings.HasPrefix(cause.Why(), "no ze listens at "+address+" (") {
		t.Errorf("why = %q, want it to say no ze listens at %s", cause.Why(), address)
	}
	if cause.Fix() != "ze start" {
		t.Errorf("fix = %q, want %q", cause.Fix(), "ze start")
	}
}

// VALIDATES: AC-32, a listener whose connection failed -- the cause says
// something listens there and names the failure, and the fix is never a
// start: `ze status` for a failure with no repair of its own, and the
// failure's own repair when it carries one (an untrusted host key).
// PREVENTS: `ze start` named as the fix while a daemon listens.
// Method: the cause of a failed first request to an open loopback listener,
// once with a plain error and once with an operator error.
func TestOfflineCauseFailedConnectionToAListenerIsNotAStart(t *testing.T) {
	host, port := listenLoopback(t)
	address := net.JoinHostPort(host, port)

	cause := unansweredAfterConnect("show host trend", host, port, errors.New("handshake cut short"))
	want := "something listens at " + address + ", but the connection to it failed (handshake cut short)"
	if cause.Why() != want {
		t.Errorf("why = %q, want %q", cause.Why(), want)
	}
	if cause.Fix() != "ze status" {
		t.Errorf("fix = %q, want %q", cause.Fix(), "ze status")
	}

	trust := opererr.New("The SSH host key changed for "+address, "Verify the offered fingerprint with the device operator", "ssh-keygen -F '[127.0.0.1]:"+port+"'")
	cause = unansweredAfterConnect("show host trend", host, port, errors.Join(errors.New("connect to "+address), trust))
	want = "something listens at " + address + ", but the connection to it failed (" + trust.Problem() + ")"
	if cause.Why() != want {
		t.Errorf("why = %q, want %q", cause.Why(), want)
	}
	if cause.Fix() != trust.Fix() {
		t.Errorf("fix = %q, want the trust refusal's own %q", cause.Fix(), trust.Fix())
	}
}

// VALIDATES: AC-32, credentials unavailable -- credentials that cannot be
// loaded do not say whether a daemon runs, so the listener decides: a daemon
// that listens is "this account has no credentials to log in to it", fixed by
// asking from an account that can, and nothing listening is the start.
// PREVENTS: `ze start` named as the fix for an account that cannot read the
// store while the daemon runs.
// Method: the cause of a failed credential load, with the local daemon's
// address resolved to an open loopback listener, then to a closed port.
func TestOfflineCauseCredentialsUnavailableFollowsTheListener(t *testing.T) {
	missing := errors.New("no stored username")

	t.Run("a-daemon-listens", func(t *testing.T) {
		host, port := listenLoopback(t)
		localDaemonAt(t, host, port)
		address := net.JoinHostPort(host, port)
		cause := unansweredWithoutCredentials("show host trend", missing)
		want := "a ze listens at " + address + ", but this account has no credentials to log in to it (no stored username)"
		if cause.Why() != want {
			t.Errorf("why = %q, want %q", cause.Why(), want)
		}
		if want := `ze cli --user <name> -c "show host trend"`; cause.Fix() != want {
			t.Errorf("fix = %q, want %q", cause.Fix(), want)
		}
	})

	t.Run("nothing-listens", func(t *testing.T) {
		host, port := closedLoopbackPort(t)
		localDaemonAt(t, host, port)
		address := net.JoinHostPort(host, port)
		cause := unansweredWithoutCredentials("show host trend", missing)
		if !strings.HasPrefix(cause.Why(), "no ze listens at "+address+" (") {
			t.Errorf("why = %q, want it to say no ze listens at %s", cause.Why(), address)
		}
		if cause.Fix() != "ze start" {
			t.Errorf("fix = %q, want %q", cause.Fix(), "ze start")
		}
	})
}

// VALIDATES: AC-32, login refused -- a daemon that answered and refused the
// login ends the run with the login refusal, and no offline fallback is run:
// its answer would stand in for a daemon that is there.
// PREVENTS: a refused login served by an offline fallback, which read as "no
// daemon answered".
// Method: endFailedAsk, the decision runBGP takes on a failed first request,
// with a *sshclient.LoginRefused and a fallback registered for the command.
func TestFailedAskRefusedLoginRunsNoFallback(t *testing.T) {
	localDaemonAt(t, "127.0.0.1", "2222")
	handed := recordFallback(t)
	refused := &sshclient.LoginRefused{
		Address: "127.0.0.1:2222", User: "admin", Cause: errors.New("ssh: unable to authenticate"),
	}
	var code int
	var ended bool
	stderr := captureOutput(t, true, func() {
		code, ended = endFailedAsk(fallbackProbeLine, "", sshclient.Credentials{Host: "127.0.0.1", Port: "2222"}, refused)
	})
	if !ended || code != 1 {
		t.Errorf("code %d ended %v, want 1 true", code, ended)
	}
	if handed.ran != 0 {
		t.Errorf("the offline fallback ran %d times for a daemon that answered", handed.ran)
	}
	if !strings.Contains(stderr, "the daemon at 127.0.0.1:2222 refused the login of admin") {
		t.Errorf("stderr does not name the refused login: %q", stderr)
	}
	if !strings.Contains(stderr, "fix: ze cli --user <name>") || strings.Contains(stderr, "ze start") {
		t.Errorf("stderr does not end on the login fix alone: %q", stderr)
	}
}

// VALIDATES: AC-32 -- a failed first request to an address nothing listens on
// runs the command's offline fallback and hands it that cause, through the
// route the operator takes.
// PREVENTS: a fallback run with no cause, left to word one itself.
// Method: endFailedAsk with a dial error to a closed loopback port.
func TestFailedAskNoListenerHandsTheFallbackItsCause(t *testing.T) {
	host, port := closedLoopbackPort(t)
	address := net.JoinHostPort(host, port)
	handed := recordFallback(t)
	code, ended := endFailedAsk(fallbackProbeLine, "", sshclient.Credentials{Host: host, Port: port},
		errors.New("connect to "+address+": connection refused"))
	if !ended || code != 1 {
		t.Errorf("code %d ended %v, want the fallback's 1 and true", code, ended)
	}
	if handed.ran != 1 {
		t.Fatalf("the offline fallback ran %d times, want 1", handed.ran)
	}
	if !strings.HasPrefix(handed.cause.Why(), "no ze listens at "+address+" (") || handed.cause.Fix() != "ze start" {
		t.Errorf("the fallback was handed %q / %q, want no ze listens at %s / ze start",
			handed.cause.Why(), handed.cause.Fix(), address)
	}
}

// VALIDATES: an authorization refusal of the first request still proves the
// daemon answers: the run goes on and no fallback is run.
// Method: endFailedAsk with an error that carries the dispatcher's
// unauthorized text.
func TestFailedAskUnauthorizedGoesOn(t *testing.T) {
	handed := recordFallback(t)
	code, ended := endFailedAsk(fallbackProbeLine, "", sshclient.Credentials{Host: "127.0.0.1", Port: "2222"},
		errors.New(plugin.UnauthorizedMessage))
	if ended || code != 0 {
		t.Errorf("code %d ended %v, want 0 false", code, ended)
	}
	if handed.ran != 0 {
		t.Errorf("the offline fallback ran %d times for a daemon that answered", handed.ran)
	}
}

// VALIDATES: an offline fallback never answers for a daemon on another host:
// its answer is this host's own data.
// PREVENTS: `ze cli --remote <router> -c "show host"` printing this host's
// inventory when the router does not answer.
// Method: runOfflineFallback with a --remote address; the cause is not even
// asked for.
func TestOfflineFallbackNeverStandsInForARemoteDaemon(t *testing.T) {
	handed := recordFallback(t)
	code, served := runOfflineFallback(fallbackProbeLine, "router.example:2222", func() command.DaemonUnanswered {
		t.Error("the cause was asked for a remote daemon")
		return command.NewDaemonUnanswered("unused", "unused")
	})
	if served || code != 0 {
		t.Errorf("code %d served %v, want 0 false", code, served)
	}
	if handed.ran != 0 {
		t.Errorf("the offline fallback ran %d times for a remote daemon", handed.ran)
	}
}
