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
	webCommitVerified = "commit verify: the candidate is valid; nothing was applied"
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
// refused a commit, the validation refusal that blocked it, or the line to
// show. A refusal is a failed commit, so a caller shows it as an error and
// never as the answer to a commit that happened.
type webCommitAnswer struct {
	conflicts []contract.Conflict
	refusal   string
	message   string
	applied   bool // the subcommand changed the config or the window
}

// runCommit runs one parsed commit subcommand for username. Every subcommand
// but verify goes through cli.WindowCommit, the path the SSH session editor
// takes, so the window's owner rules apply to a web user unchanged. A commit
// is judged by the SSH editor's validation and refused with its words
// (cli.CommitRefusal): a warning blocks unless force, an error always does.
// The commit is in flight for WindowNotices until its answer has updated
// username's watch, so the notices never misreport the user's own commit.
func (m *EditorManager) runCommit(username string, req contract.CommitRequest) (webCommitAnswer, error) {
	if req.Action == contract.CommitVerify {
		return m.verifyCommit(username)
	}
	m.beginWindowCommit(username)
	defer m.endWindowCommit(username)
	commit := cli.WindowCommit{Window: m.daemonWindow(), User: username, Store: m.store, ConfigPath: m.configPath}
	forced := contract.ForcedCommand(req)
	var refusal string
	var skipped int
	check := func(ed contract.Editor) error {
		validation, err := m.validateTransition(ed)
		if err != nil {
			return err
		}
		if text, blocked := cli.CommitRefusal(validation, req.Force, forced); blocked {
			refusal = text
			return cli.ErrCommitNotApplied
		}
		skipped = len(validation.Warnings)
		return nil
	}
	var result *contract.CommitResult
	err := commit.Run(req, func() error {
		answer, err := m.commit(username, req.Force, check)
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
		if refusal != "" {
			return webCommitAnswer{refusal: refusal}, nil
		}
		return notAppliedAnswer(req, result), nil
	}
	if err != nil {
		return webCommitAnswer{}, err
	}
	answer := m.appliedAnswer(username, req)
	answer.applied = true
	answer.message = cli.WithSkippedWarnings(forced, skipped, answer.message)
	if result != nil && len(result.Warnings) > 0 {
		// The SSH editor's words for what the applied commit warns of,
		// a forced commit's failed discard among them.
		var tb textbuf.Buffer
		tb.Str(answer.message)
		cli.AppendCommitWarnings(&tb, result.Warnings)
		answer.message = tb.String()
	}
	return answer, nil
}

// CommitNow commits username's changes as the "Review & Commit" button does:
// `commit now` through runCommit, the daemon window and the SSH editor's
// validation included. It answers the line shown, and an error naming why
// when nothing was applied.
func (m *EditorManager) CommitNow(username string) (string, error) {
	req := contract.CommitRequest{Action: contract.CommitNow}
	answer, err := m.runCommit(username, req)
	if err != nil {
		return "", err
	}
	if answer.refusal != "" {
		return "", errors.New(answer.refusal)
	}
	if len(answer.conflicts) > 0 {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("commit conflicts with ").Int(int64(len(answer.conflicts))).Str(" change(s) of other users").String())
	}
	if !answer.applied {
		return "", errors.New(answer.message)
	}
	return answer.message, nil
}

// validateTransition runs the validation the SSH editor's commit runs over
// ed's transition from the committed config to the user's view (AC-29). The
// validator loads the YANG modules, so it is built once, at the first commit;
// it is not safe for concurrent use, so one validation runs at a time.
func (m *EditorManager) validateTransition(ed contract.Editor) (cli.ConfigValidationResult, error) {
	m.validatorOnce.Do(func() {
		m.validator, m.validatorErr = cli.NewConfigValidator()
	})
	if m.validatorErr != nil {
		return cli.ConfigValidationResult{}, m.validatorErr
	}
	m.validateMu.Lock()
	defer m.validateMu.Unlock()
	return m.validator.ValidateTransition(ed.OriginalContent(), ed.WorkingContent()), nil
}

// notAppliedAnswer is the answer to a commit that did not happen: its
// conflicts, or that nothing was pending, in the words the SSH editor uses
// (AC-29), never a success.
func notAppliedAnswer(req contract.CommitRequest, result *contract.CommitResult) webCommitAnswer {
	if result != nil && len(result.Conflicts) > 0 {
		return webCommitAnswer{conflicts: result.Conflicts}
	}
	return webCommitAnswer{message: contract.NothingToCommit(req)}
}

// appliedAnswer is the line a subcommand that succeeded shows. An abort also
// rebuilds the user's view over the restored config. Each records what it
// did to the window in username's watch (WindowNotices).
func (m *EditorManager) appliedAnswer(username string, req contract.CommitRequest) webCommitAnswer {
	switch req.Action {
	case contract.CommitNow:
		return webCommitAnswer{message: terminalOutputCommitSuccessful}
	case contract.CommitConfirmed:
		if window := m.daemonWindow(); window != nil {
			m.setWindowWatch(username, cli.WatchWindow(window))
		}
		var tb textbuf.Buffer
		return webCommitAnswer{message: tb.Str(terminalOutputCommitSuccessful).Str(". ").
			Str(contract.ConfirmWithin(int64(req.Seconds))).String()}
	case contract.CommitAccept:
		m.setWindowWatch(username, nil)
		return webCommitAnswer{message: contract.CommitAccepted}
	case contract.CommitAbort:
		m.setWindowWatch(username, nil)
		if err := m.refreshCommittedView(username); err != nil {
			var tb textbuf.Buffer
			return webCommitAnswer{message: tb.Str(contract.CommitAborted).Str(" (view not refreshed: ").Err(err).Byte(')').String()}
		}
		return webCommitAnswer{message: contract.CommitAborted}
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
