// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Overview: model_commands_commit.go -- cmdCommitRequest, the commit dispatcher
// Related: commit_window.go -- WindowCommit, the path the web editor shares
// Related: ../config/confirm/confirm.go -- the daemon's window worker

package cli

import (
	"errors"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// WindowWatch is what one editor remembers of the daemon window it last saw
// open, so its poll can say how the window ended. The SSH session editor
// keeps one per session; the web keeps one per user (WindowNotices). The zero
// value has seen no window open. Not safe for concurrent use.
type WindowWatch struct {
	open     bool
	timeouts uint64
}

// WindowNews is what one poll of the daemon window has to say to its viewer:
// the line to show, and whether the window ended, so the viewer's tree must
// be rebuilt over the configuration it left.
type WindowNews struct {
	Line  string
	Ended bool
}

// Poll looks at the daemon window for viewer. An open window answers its
// status line (AC-17): its owner and the seconds left, or that its deadline
// revert failed and what each user may do. A window this watch saw open and
// that is now gone answers how it ended (AC-14): its deadline reverted it, or
// another session closed it. False means there is nothing to say.
func (w *WindowWatch) Poll(window *confirm.Window, viewer string) (WindowNews, bool) {
	status, open := window.Status()
	if open {
		if !w.open {
			if watched := WatchWindow(window); watched != nil {
				*w = *watched
			}
		}
		return WindowNews{Line: status.Line(viewer)}, true
	}
	if !w.open {
		return WindowNews{}, false
	}
	watched := *w
	*w = WindowWatch{}
	timeouts, err := window.Timeouts()
	if err != nil {
		return WindowNews{}, false
	}
	if timeouts > watched.timeouts {
		return WindowNews{Line: contract.CommitTimedOut, Ended: true}, true
	}
	return WindowNews{Line: contract.CommitClosedElsewhere, Ended: true}, true
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

// RefreshCommittedView rebuilds the session view over the committed config,
// after the daemon's window reverted it: the user's pending changes replay
// over the restored config.
func (e *Editor) RefreshCommittedView() error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return err
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	return e.reloadSessionView(guard)
}

// windowCommit is this session's commit through the daemon's window.
func (m *Model) windowCommit(window *confirm.Window) WindowCommit {
	return WindowCommit{Window: window, User: m.editor.session.User, Store: m.editor.store, ConfigPath: m.editor.originalPath}
}

// cmdCommitWindowRequest runs a session commit subcommand against the daemon's
// window through WindowCommit, the path the web editor shares; the window
// decides who may commit while it is open (AC-17, AC-18).
func (m *Model) cmdCommitWindowRequest(window *confirm.Window, req contract.CommitRequest) (commandResult, error) {
	commit := m.windowCommit(window)
	switch req.Action {
	case contract.CommitNow:
		var result commandResult
		err := commit.Run(req, m.applySessionCommit(req, &result))
		return windowCommitAnswer(result, err)
	case contract.CommitConfirmed:
		return m.cmdCommitConfirmedWindow(commit, req)
	case contract.CommitAccept:
		if err := commit.Run(req, nil); err != nil {
			return commandResult{}, err
		}
		return commandResult{
			statusMessage: contract.CommitAccepted,
			windowWatch:   &WindowWatch{},
		}, nil
	case contract.CommitAbort:
		if err := commit.Run(req, nil); err != nil {
			return commandResult{}, err
		}
		result := m.windowReverted(contract.CommitAborted)
		result.windowWatch = &WindowWatch{}
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
func (m *Model) cmdCommitConfirmedWindow(commit WindowCommit, req contract.CommitRequest) (commandResult, error) {
	var result commandResult
	if err := commit.Run(req, m.applySessionCommit(req, &result)); err != nil {
		return windowCommitAnswer(result, err)
	}
	result.windowWatch = WatchWindow(commit.Window)
	var tb textbuf.Buffer
	result.statusMessage = tb.Str(result.statusMessage).Str(". ").Str(contract.ConfirmWithin(int64(req.Seconds))).String()
	return result, nil
}

// applySessionCommit is the Apply a window runs: the session commit, its
// status kept in result, and ErrCommitNotApplied when it did not happen.
func (m *Model) applySessionCommit(req contract.CommitRequest, result *commandResult) func() error {
	return func() error {
		answer, committed, err := m.runCommitSession(req)
		*result = answer
		if err != nil {
			return err
		}
		if !committed {
			return ErrCommitNotApplied
		}
		return nil
	}
}

// windowCommitAnswer turns a window's answer into the command's: a commit the
// session refused shows its own status, any other error is the answer.
func windowCommitAnswer(result commandResult, err error) (commandResult, error) {
	if errors.Is(err, ErrCommitNotApplied) {
		return result, nil
	}
	if err != nil {
		return commandResult{}, err
	}
	return result, nil
}

// WatchWindow is what an editor remembers when it sees the window open, so
// its poll can report the window's end; nil when the window has stopped.
func WatchWindow(window *confirm.Window) *WindowWatch {
	timeouts, err := window.Timeouts()
	if err != nil {
		return nil
	}
	return &WindowWatch{open: true, timeouts: timeouts}
}

// windowReverted rebuilds the view over the restored config and reports msg.
func (m *Model) windowReverted(msg string) commandResult {
	m.searchCache = ""
	if err := m.editor.RefreshCommittedView(); err != nil {
		var tb textbuf.Buffer
		msg = tb.Str(msg).Str(" (view not refreshed: ").Err(err).Byte(')').String()
	}
	return commandResult{statusMessage: msg, configView: m.configViewAtPath(m.contextPath), revalidate: true}
}

// pollDaemonWindow is the draft poll's look at the daemon window
// (WindowWatch.Poll). A window that ended rebuilds the view over the
// configuration it left. It answers the status line, and false when there is
// nothing to say.
func (m *Model) pollDaemonWindow() (string, bool) {
	window := m.editor.daemonWindow()
	if window == nil {
		return "", false
	}
	news, ok := m.windowWatch.Poll(window, m.editor.session.User)
	if !ok {
		return "", false
	}
	if news.Ended {
		return m.windowReverted(news.Line).statusMessage, true
	}
	return news.Line, true
}
