// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Overview: model_commands_commit.go -- cmdCommitRequest, the commit dispatcher
// Related: ../config/confirm/confirm.go -- the daemon's window worker

package cli

import (
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	// errCommitConfirmedNeedsDaemon refuses a session `commit confirmed` with
	// no daemon window to own the countdown: a session editor of a config the
	// daemon does not run, or a daemon that does not run it from its store.
	errCommitConfirmedNeedsDaemon = errors.New("commit confirmed needs a daemon that runs this configuration from its store")
	// errCommitNotApplied tells the window a session commit did not happen
	// (validation, a conflict or the reload refused it), so no window opens
	// and the commit's own status is shown.
	errCommitNotApplied = errors.New("commit not applied")
)

// windowWatch is what a session editor remembers of the daemon window it last
// saw open, so the draft poll can say how it ended.
type windowWatch struct {
	open     bool
	timeouts uint64
}

// SetConfirmWindow gives a session editor the daemon's confirmed-commit
// window. The getter is read at each commit, so an editor built before the
// daemon finished starting still finds it. A nil getter, or a getter that
// answers nil, leaves the editor with no window.
func (e *Editor) SetConfirmWindow(window func() *confirm.Window) {
	e.confirmWindow = window
}

// daemonWindow returns the daemon's window for a session editor, or nil.
func (e *Editor) daemonWindow() *confirm.Window {
	if e.session == nil {
		return nil
	}
	if e.confirmWindow == nil {
		return nil
	}
	return e.confirmWindow()
}

// refreshCommittedView rebuilds the session view over the committed config,
// after the daemon's window reverted it: the user's pending changes replay
// over the restored config.
func (e *Editor) refreshCommittedView() error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return err
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	return e.reloadSessionView(guard)
}

// cmdCommitWindowRequest runs a session commit subcommand against the daemon's
// window, which decides who may commit while a window is open (AC-17, AC-18).
// The commit itself runs on the window's worker, so it never interleaves with
// a revert.
func (m *Model) cmdCommitWindowRequest(window *confirm.Window, req contract.CommitRequest) (commandResult, error) {
	user := m.editor.session.User
	switch req.Action {
	case contract.CommitNow:
		var result commandResult
		err := window.Now(user, confirm.Commit{Apply: m.applySessionCommit(req.Force, &result)})
		return windowCommitAnswer(result, err)
	case contract.CommitConfirmed:
		return m.cmdCommitConfirmedWindow(window, user, req)
	case contract.CommitAccept:
		if err := window.Accept(user); err != nil {
			return commandResult{}, err
		}
		return commandResult{
			statusMessage: "Commit accepted: the confirmed configuration is saved permanently.",
			windowWatch:   &windowWatch{},
		}, nil
	case contract.CommitAbort:
		if err := window.Abort(user); err != nil {
			return commandResult{}, err
		}
		result := m.windowReverted("Changes rolled back to previous configuration.")
		result.windowWatch = &windowWatch{}
		return result, nil
	case contract.CommitVerify:
		return m.cmdCommitVerify()
	case contract.CommitActionUnspecified:
		panic("BUG: commit request carries no action")
	}
	panic("BUG: unknown commit action")
}

// cmdCommitConfirmedWindow runs `commit confirmed <seconds> [force]`: the
// window snapshots the running config, applies the commit, and counts down.
func (m *Model) cmdCommitConfirmedWindow(window *confirm.Window, user string, req contract.CommitRequest) (commandResult, error) {
	// AC-21: the window's revert restores a config version, so a store with
	// no history refuses before anything is written.
	if !m.editor.HasHistory() {
		return commandResult{}, errCommitConfirmedNeedsHistory
	}
	var result commandResult
	err := window.Confirmed(user, time.Duration(req.Seconds)*time.Second, req.Force, confirm.Commit{
		Snapshot: func() ([]byte, error) { return storage.ReadActiveConfig(m.editor.store, m.editor.originalPath) },
		Apply:    m.applySessionCommit(req.Force, &result),
	})
	if err != nil {
		return windowCommitAnswer(result, err)
	}
	result.windowWatch = watchedWindow(window)
	var tb textbuf.Buffer
	result.statusMessage = tb.Str(result.statusMessage).Str(". Confirm within ").Int(int64(req.Seconds)).
		Str("s or auto-revert. Use 'commit accept' or 'commit abort'.").String()
	return result, nil
}

// applySessionCommit is the Apply a window runs: the session commit, its
// status kept in result, and errCommitNotApplied when it did not happen.
func (m *Model) applySessionCommit(force bool, result *commandResult) func() error {
	return func() error {
		answer, committed, err := m.runCommitSession(force)
		*result = answer
		if err != nil {
			return err
		}
		if !committed {
			return errCommitNotApplied
		}
		return nil
	}
}

// windowCommitAnswer turns a window's answer into the command's: a commit the
// session refused shows its own status, any other error is the answer.
func windowCommitAnswer(result commandResult, err error) (commandResult, error) {
	if errors.Is(err, errCommitNotApplied) {
		return result, nil
	}
	if err != nil {
		return commandResult{}, err
	}
	return result, nil
}

// watchedWindow is what a session remembers when it sees the window open, so
// the draft poll can report its end; nil when the window has stopped.
func watchedWindow(window *confirm.Window) *windowWatch {
	timeouts, err := window.Timeouts()
	if err != nil {
		return nil
	}
	return &windowWatch{open: true, timeouts: timeouts}
}

// windowReverted rebuilds the view over the restored config and reports msg.
func (m *Model) windowReverted(msg string) commandResult {
	m.searchCache = ""
	if err := m.editor.refreshCommittedView(); err != nil {
		var tb textbuf.Buffer
		msg = tb.Str(msg).Str(" (view not refreshed: ").Err(err).Byte(')').String()
	}
	return commandResult{statusMessage: msg, configView: m.configViewAtPath(m.contextPath), revalidate: true}
}

// pollDaemonWindow is the draft poll's look at the daemon window. An open
// window shows its owner and the seconds left (AC-17); a window this session
// saw open and that is now gone says how it ended (AC-14). It answers the
// status line, and false when there is nothing to say.
func (m *Model) pollDaemonWindow() (string, bool) {
	window := m.editor.daemonWindow()
	if window == nil {
		return "", false
	}
	status, open := window.Status()
	if open {
		if !m.windowWatch.open {
			if watched := watchedWindow(window); watched != nil {
				m.windowWatch = *watched
			}
		}
		left := int64(status.Left().Round(time.Second) / time.Second)
		var tb textbuf.Buffer
		if status.User == m.editor.session.User {
			return tb.Str("Confirm within ").Int(left).Str("s or auto-revert. Use 'commit accept' or 'commit abort'.").String(), true
		}
		return tb.Str("A confirmed commit by ").Str(status.User).Str(" is pending: ").Int(left).Str("s left.").String(), true
	}
	if !m.windowWatch.open {
		return "", false
	}
	watched := m.windowWatch
	m.windowWatch = windowWatch{}
	timeouts, err := window.Timeouts()
	if err != nil {
		return "", false
	}
	if timeouts > watched.timeouts {
		return m.windowReverted("Timeout: configuration automatically rolled back.").statusMessage, true
	}
	return m.windowReverted("The confirmed commit window was closed by another session.").statusMessage, true
}
