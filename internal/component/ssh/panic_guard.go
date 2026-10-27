// Design: docs/guide/operations.md -- SSH session panic containment
// Related: ssh.go -- Start, which installs panicMiddleware outermost and wraps the auth callbacks

package ssh

import (
	"io"

	"charm.land/ssh"
	"charm.land/wish/v2"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// Task labels in ze_task_panics_total{component="ssh"}.
const (
	// taskSession is one SSH session's handler: interactive CLI or exec.
	taskSession = "session"
	// taskAuth is one public-key or password login check.
	taskAuth = "auth"
)

// sessionRestart and authRestart say what follows a contained panic, in the
// words the guard's log line gives the operator.
const (
	sessionRestart = "only this SSH session closed, after telling its user; the listener and " +
		"every other session keep running, and the user reconnects"
	authRestart = "this login attempt was refused; the listener and every other session keep " +
		"running, and the user can try again"
)

// sessionGuard is the panic boundary of every SSH session and login check. A
// contained panic is logged, counted in ze_task_panics_total{component="ssh"}
// and crash-recorded. Safe for concurrent use.
var sessionGuard = taskguard.New("ssh", slogutil.Logger("ssh"))

// sessionTask and authTask are declared once so their counts exist before the
// first panic.
var (
	sessionTask = sessionGuard.Task(taskSession, sessionRestart)
	authTask    = sessionGuard.Task(taskAuth, authRestart)
)

// SetPanicMetrics publishes the SSH panic boundary's counts on reg. The hub
// calls it where it builds the server; panics contained before then are carried
// over. reg MUST be non-nil.
func SetPanicMetrics(reg metrics.Registry) {
	sessionGuard.SetMetricsRegistry(reg)
}

// panicMiddleware is the outermost middleware: a Ze defect met anywhere in one
// session's handler chain ends that session alone. The inner middlewares
// release what they hold through their own defers (session slot, closer) as
// the panic unwinds, before this boundary runs.
func (s *Server) panicMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			defer sessionTask.Contain(
				func() taskguard.Context { return sessionContext(sess) },
				func(p *taskguard.Panic) { endSessionAfterPanic(sess, p) },
			)
			next(sess)
		}
	}
}

// endSessionAfterPanic tells the user in the session, while the channel still
// takes writes, what ended it and what to do, then exits it with status 1.
func endSessionAfterPanic(sess ssh.Session, p *taskguard.Panic) {
	newline := "\n"
	if _, _, interactive := sess.Pty(); interactive {
		newline = "\r\n"
	}
	next := "run 'ze show crashes' and attach the newest record to a bug report"
	if p.Report != "" {
		next = "run 'ze show crashes name " + p.Report + "' and attach it to a bug report"
	}
	message := newline + "error: an internal Ze defect ended this SSH session; the daemon and " +
		"every other session keep running. Reconnect to continue, and " + next + "." + newline
	io.WriteString(sess.Stderr(), message) //nolint:errcheck // best effort: the channel may already be gone
	sess.Exit(1)                           //nolint:errcheck // best effort: the channel may already be gone
}

// sessionContext names the user and the client for the guard's log line and
// crash record. It reads fields the session already holds, so it never blocks.
func sessionContext(sess ssh.Session) taskguard.Context {
	return taskguard.Context{Unit: "user " + sess.User() + " from " + sess.RemoteAddr().String()}
}

// containAuth runs one login check under the auth boundary. A check that
// panics refuses the login: granted keeps its zero, false, which is the
// refusal the boundary owes, never a default.
func containAuth(ctx ssh.Context, method string, check func() bool) (granted bool) {
	defer authTask.Contain(func() taskguard.Context {
		return taskguard.Context{
			Unit:        "login " + ctx.User() + " from " + ctx.RemoteAddr().String(),
			MessageType: method,
		}
	}, nil)
	return check()
}
