// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Overview: editor.go -- EditorManager, the per-user editors
// Related: cli_terminal.go -- the terminal and CLI bar commit verbs
// Related: ../cli/commit_window.go -- WindowCommit, the path the SSH editor shares

package web

import (
	"errors"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The answers a web commit subcommand gives when it changed something.
const (
	webCommitConfirmNoChanges = "no changes to commit: no confirmed commit was opened"
	webCommitAccepted         = "Commit accepted: the confirmed configuration is saved permanently."
	webCommitAborted          = "Changes rolled back to previous configuration."
	webCommitVerified         = "commit verify: the candidate is valid; nothing was applied"
)

// SetConfirmWindow gives the manager the daemon's confirmed-commit window,
// the one the SSH session editors use. The getter is read at each commit, so
// a manager built before the daemon finished starting still finds it. With no
// getter, or a getter that answers nil, `commit now` applies directly and the
// window subcommands are refused, as for an SSH editor with no daemon.
func (m *EditorManager) SetConfirmWindow(window func() *confirm.Window) {
	m.mu.Lock()
	m.confirmWindow = window
	m.mu.Unlock()
}

// daemonWindow returns the daemon's window, or nil when there is none.
func (m *EditorManager) daemonWindow() *confirm.Window {
	m.mu.RLock()
	window := m.confirmWindow
	m.mu.RUnlock()
	if window == nil {
		return nil
	}
	return window()
}

// webCommitAnswer is what one commit subcommand answered: the conflicts that
// refused a commit, or the line to show.
type webCommitAnswer struct {
	conflicts []contract.Conflict
	message   string
}

// runCommit runs one parsed commit subcommand for username. Every subcommand
// but verify goes through cli.WindowCommit, the path the SSH session editor
// takes, so the window's owner rules apply to a web user unchanged.
func (m *EditorManager) runCommit(username string, req contract.CommitRequest) (webCommitAnswer, error) {
	if req.Action == contract.CommitVerify {
		return m.verifyCommit(username)
	}
	commit := cli.WindowCommit{Window: m.daemonWindow(), User: username, Store: m.store, ConfigPath: m.configPath}
	var result *contract.CommitResult
	err := commit.Run(req, func() error {
		answer, err := m.commit(username, req.Force)
		result = answer
		if err != nil {
			return err
		}
		if len(answer.Conflicts) > 0 {
			return cli.ErrCommitNotApplied
		}
		if answer.Applied == 0 {
			return cli.ErrCommitNotApplied
		}
		return nil
	})
	if errors.Is(err, cli.ErrCommitNotApplied) {
		return notAppliedAnswer(req, result), nil
	}
	if err != nil {
		return webCommitAnswer{}, err
	}
	return m.appliedAnswer(username, req)
}

// notAppliedAnswer is the answer to a commit that did not happen: its
// conflicts, or that nothing was pending.
func notAppliedAnswer(req contract.CommitRequest, result *contract.CommitResult) webCommitAnswer {
	if result != nil && len(result.Conflicts) > 0 {
		return webCommitAnswer{conflicts: result.Conflicts}
	}
	if req.Action == contract.CommitConfirmed {
		return webCommitAnswer{message: webCommitConfirmNoChanges}
	}
	return webCommitAnswer{message: terminalOutputCommitSuccessful}
}

// appliedAnswer is the line a subcommand that succeeded shows. An abort also
// rebuilds the user's view over the restored config.
func (m *EditorManager) appliedAnswer(username string, req contract.CommitRequest) (webCommitAnswer, error) {
	switch req.Action {
	case contract.CommitNow:
		return webCommitAnswer{message: terminalOutputCommitSuccessful}, nil
	case contract.CommitConfirmed:
		var tb textbuf.Buffer
		return webCommitAnswer{message: tb.Str(terminalOutputCommitSuccessful).Str(". Confirm within ").
			Int(int64(req.Seconds)).Str("s or auto-revert. Use 'commit accept' or 'commit abort'.").String()}, nil
	case contract.CommitAccept:
		return webCommitAnswer{message: webCommitAccepted}, nil
	case contract.CommitAbort:
		if err := m.refreshCommittedView(username); err != nil {
			var tb textbuf.Buffer
			return webCommitAnswer{message: tb.Str(webCommitAborted).Str(" (view not refreshed: ").Err(err).Byte(')').String()}, nil
		}
		return webCommitAnswer{message: webCommitAborted}, nil
	case contract.CommitVerify, contract.CommitActionUnspecified:
		panic("BUG: commit verify and an empty action never reach the window")
	}
	panic("BUG: unknown commit action")
}

// verifyCommit validates username's view and applies nothing (AC-26).
func (m *EditorManager) verifyCommit(username string) (webCommitAnswer, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return webCommitAnswer{}, err
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	if err := us.editor.VerifySession(); err != nil {
		return webCommitAnswer{}, err
	}
	return webCommitAnswer{message: webCommitVerified}, nil
}

// refreshCommittedView rebuilds username's view after the window reverted
// the committed config. A user with no editor has no view to rebuild.
func (m *EditorManager) refreshCommittedView(username string) error {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.editor.RefreshCommittedView()
}
