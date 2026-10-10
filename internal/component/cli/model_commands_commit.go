// Design: docs/architecture/config/yang-config-design.md — commit, rollback, and discard lifecycle
// Overview: model_commands.go — command dispatch

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errUsageRollbackNumber        = errors.New("usage: rollback <number>")
	errDiscardRequiresPathOrAllIn = errors.New("discard requires path or 'all' in session mode")
)

func (m *Model) cmdHistory() (commandResult, error) {
	backups, err := m.editor.ListBackups()
	if err != nil {
		return commandResult{}, err
	}

	if len(backups) == 0 && !m.editor.HasDraft() {
		return commandResult{output: "No backups found"}, nil
	}

	var b textbuf.Buffer
	if m.editor.HasDraft() {
		b.Str("draft  (editing in progress)\n")
	}
	for i, backup := range backups {
		b.Int(int64(i + 1)).Str(". ").Str(backup.Timestamp.Format("2006-01-02 15:04:05")).Str("  ").Str(backup.Path).Byte('\n')
	}
	return commandResult{output: b.String()}, nil
}

// formatValidationErrors formats a slice of validation errors into a human-readable string.
func formatValidationErrors(errs []ConfigValidationError) string {
	if len(errs) == 1 {
		e := errs[0]
		if e.Line > 0 {
			var b textbuf.Buffer
			return b.Reset().Str("line ").Int(int64(e.Line)).Str(": ").Str(e.Message).String()
		}
		return e.Message
	}
	var b textbuf.Buffer
	b.Int(int64(len(errs))).Str(" validation error(s):")
	for _, e := range errs {
		if e.Line > 0 {
			b.Str("\n  line ").Int(int64(e.Line)).Str(": ").Str(e.Message)
		} else {
			b.Str("\n  ").Str(e.Message)
		}
	}
	return b.String()
}

func (m *Model) cmdRollback(args []string) (commandResult, error) {
	if len(args) != 1 {
		return commandResult{}, errUsageRollbackNumber
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return commandResult{}, fmt.Errorf("invalid backup number: %s", args[0])
	}

	backups, err := m.editor.ListBackups()
	if err != nil {
		return commandResult{}, err
	}

	if n < 1 || n > len(backups) {
		return commandResult{}, fmt.Errorf("backup %d not found (have %d backups)", n, len(backups))
	}

	if err := m.editor.Rollback(backups[n-1].Path); err != nil {
		return commandResult{}, err
	}
	m.searchCache = "" // tree changed, invalidate cached set-view
	var tb textbuf.Buffer
	m.recordConfigDiscard(tb.Str("rollback ").Str(backups[n-1].Path).String())

	return commandResult{
		statusMessage: tb.Reset().Str("Rolled back to ").Str(backups[n-1].Path).String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// runValidation re-runs validation on current content.
// Validates hierarchical content (matching the viewport display format)
// so that line numbers align with what the user sees.
func (m *Model) runValidation() {
	if m.editor == nil || m.validator == nil {
		return
	}
	result := m.validator.Validate(m.editor.ContentAtPath(nil))
	m.validationErrors = result.Errors
	m.validationWarnings = result.Warnings
}

// scheduleValidation returns a command to trigger validation after debounce delay.
func (m *Model) scheduleValidation() tea.Cmd {
	if m.editor == nil {
		return nil
	}
	m.validationID++
	id := m.validationID
	return tea.Tick(validationDebounce, func(_ time.Time) tea.Msg {
		return validationTickMsg{id: id}
	})
}

// cmdSave persists work-in-progress. In session mode, applies changes from the
// per-user change file to config.conf.draft. In non-session mode, writes a .edit snapshot.
func (m *Model) cmdSave() (commandResult, error) {
	if m.editor.HasSession() {
		if err := m.editor.SaveDraft(); err != nil {
			return commandResult{}, err
		}
		return commandResult{statusMessage: "Changes saved to draft"}, nil
	}
	if err := m.editor.saveEditState(); err != nil {
		return commandResult{}, err
	}
	return commandResult{statusMessage: "Configuration saved (snapshot)"}, nil
}

// commitNowForce is `commit now force`, spelled by the grammar.
var commitNowForce = contract.ForcedCommand(contract.CommitRequest{Action: contract.CommitNow, Force: true})

// cmdCommitRequest runs one parsed commit subcommand. A pending confirm window
// refuses `commit now` and a plain nested `commit confirmed`, because a commit
// inside the window would be reverted with it (AC-18, AC-23); the owner's
// nested `commit confirmed <seconds> force` is taken and restarts the window.
// A session editor of the daemon's own config commits through the daemon's
// window (model_commit_window.go); file mode keeps its in-process countdown.
func (m *Model) cmdCommitRequest(req contract.CommitRequest) (commandResult, error) {
	if window := m.editor.daemonWindow(); window != nil {
		return m.cmdCommitWindowRequest(window, req)
	}
	switch req.Action {
	case contract.CommitNow:
		if m.confirmTimerActive {
			return commandResult{}, confirm.ErrPending
		}
		if m.editor.HasSession() {
			return m.cmdCommitSession(req.Force)
		}
		if req.Force {
			return m.cmdCommitForce()
		}
		return m.cmdCommit()
	case contract.CommitConfirmed:
		if m.editor.HasSession() {
			return commandResult{}, errCommitConfirmedNeedsDaemon
		}
		if !m.confirmTimerActive {
			return m.cmdCommitConfirmed(req.Seconds, req.Force)
		}
		if !req.Force {
			return commandResult{}, confirm.ErrPending
		}
		return m.cmdCommitConfirmedNested(req.Seconds)
	case contract.CommitAccept:
		return m.cmdConfirm()
	case contract.CommitAbort:
		return m.cmdAbort()
	case contract.CommitVerify:
		return m.cmdCommitVerify()
	case contract.CommitActionUnspecified:
		panic("BUG: commit request carries no action")
	}
	panic("BUG: unknown commit action")
}

// cmdCommitConfirmedNested takes the window owner's `commit confirmed <seconds>
// force` inside file mode's own window (AC-23): the new changes apply and the
// countdown restarts at <seconds>, but the revert target stays the backup the
// FIRST commit of the window recorded. The nested commit records its own
// backup, which holds the first commit's result, so taking it would make an
// abort or a timeout restore only half of what the window covers.
func (m *Model) cmdCommitConfirmedNested(seconds int) (commandResult, error) {
	result, err := m.cmdCommitConfirmed(seconds, true)
	if err != nil {
		return result, err
	}
	if result.setConfirmTimer {
		result.confirmBackupPath = m.confirmBackupPath
	}
	return result, nil
}

// cmdCommitVerify runs the validation `commit now` runs and applies nothing:
// the candidate, its pending changes and any window stay as they are (AC-26).
func (m *Model) cmdCommitVerify() (commandResult, error) {
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	if len(result.Errors) == 0 && len(result.Warnings) == 0 {
		return commandResult{statusMessage: "commit verify: the candidate is valid; nothing was applied"}, nil
	}
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	issues = append(issues, result.Warnings...)
	var b textbuf.Buffer
	b.Str("commit verify: nothing was applied; ").Int(int64(len(result.Errors))).Str(" error(s), ").
		Int(int64(len(result.Warnings))).Str(" warning(s): ")
	appendIssueSummary(&b, issues)
	return commandResult{statusMessage: b.String(), configView: m.configViewAtPath(m.contextPath)}, nil
}

// commitValidationRefusal is CommitRefusal as this session's status, with the
// config kept in the viewport with its issue markers.
func (m *Model) commitValidationRefusal(result ConfigValidationResult, force bool, forced string) (commandResult, bool) {
	refusal, blocked := CommitRefusal(result, force, forced)
	if !blocked {
		return commandResult{}, false
	}
	return commandResult{statusMessage: refusal, configView: m.configViewAtPath(m.contextPath)}, true
}

// CommitRefusal answers whether validation blocks a commit, and the line that
// says why: every error blocks, and a warning blocks unless force. The line
// counts what blocks, names forced (the command that commits over the
// warnings) when only warnings block (AC-12), then lists every blocking issue,
// the list `commit verify` gives. The hint comes before the list, so a status
// line the terminal cuts short still carries it. Every editor that refuses a
// commit over validation words it here: the SSH and file-mode Model and the
// web terminal (AC-29).
func CommitRefusal(result ConfigValidationResult, force bool, forced string) (string, bool) {
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	if !force {
		issues = append(issues, result.Warnings...)
	}
	if len(issues) == 0 {
		return "", false
	}
	var b textbuf.Buffer
	b.Str("commit blocked: ").Int(int64(len(result.Errors))).Str(" error(s), ").
		Int(int64(len(issues) - len(result.Errors))).Str(" warning(s)")
	switch {
	case force:
		b.Str("; force commits over warnings, never errors")
	case len(result.Errors) == 0:
		b.Str("; '").Str(forced).Str("' commits over the warnings")
	}
	b.Str(": ")
	appendIssueSummary(&b, issues)
	return b.String(), true
}

// appendIssueSummary writes issues on one line, each as formatIssueList
// words it, separated by "; ".
func appendIssueSummary(b *textbuf.Buffer, issues []ConfigValidationError) {
	for i, e := range issues {
		if i > 0 {
			b.Str("; ")
		}
		if e.Line > 0 {
			b.Str("line ").Int(int64(e.Line)).Str(": ")
		}
		b.Str(e.Message)
	}
}

// cmdCommit saves changes with validation check.
// If a ReloadNotifier is set, stages a transactional candidate and asks the daemon to reload.
// Reload failure fails the commit and leaves the editor dirty.
// Both errors and warnings block commit — config must be fully correct.
func (m *Model) cmdCommit() (commandResult, error) {
	// Validate inline - don't rely on m.validationErrors which may be stale
	// (m is captured by value in the tea.Cmd closure)
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	if refusal, blocked := m.commitValidationRefusal(result, false, commitNowForce); blocked {
		return refusal, nil
	}

	return m.commitSaveAndReload()
}

// tryReload attempts a config reload and stores errors for the errors command.
// Returns a suffix string for the status message.
func (m *Model) tryReload() string {
	m.reloadErrors = nil
	if err := m.editor.NotifyReload(); err != nil {
		m.reloadErrors = []string{err.Error()}
		return " (reload errors, type 'errors' for details)"
	}
	return " and reloaded"
}

// cmdCommitForce saves changes, skipping warnings but still blocking on errors.
// Used when the operator explicitly overrides warnings (e.g., dangling profile references).
func (m *Model) cmdCommitForce() (commandResult, error) {
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	if refusal, blocked := m.commitValidationRefusal(result, true, commitNowForce); blocked {
		return refusal, nil
	}

	committed, err := m.commitSaveAndReload()
	if err != nil {
		return committed, err
	}
	committed.statusMessage = WithSkippedWarnings(commitNowForce, len(result.Warnings), committed.statusMessage)
	return committed, nil
}

// WithSkippedWarnings prefixes a forced commit's status with how many warnings
// forced committed over (AC-12), and returns the status alone when it skipped
// none. The web terminal words its forced commit with it too (AC-29).
func WithSkippedWarnings(forced string, skipped int, status string) string {
	if skipped == 0 {
		return status
	}
	var b textbuf.Buffer
	return b.Str(forced).Str(": skipping ").Int(int64(skipped)).Str(" warning(s). ").Str(status).String()
}

// commitSaveAndReload performs the save, archive, and reload steps shared
// by cmdCommit and cmdCommitForce. Called after validation has passed.
func (m *Model) commitSaveAndReload() (commandResult, error) {
	detail := m.editor.Diff()
	if m.editor.HasReloadNotifier() {
		return m.commitCandidateAndReload(detail)
	}

	warnings, err := m.editor.Save()
	if err != nil {
		return commandResult{}, err
	}
	m.recordConfigCommit(detail)
	m.searchCache = ""

	var archiveMsg string
	if m.editor.hasArchiveNotifier() {
		content := []byte(m.editor.WorkingContent())
		if errs := m.editor.notifyArchive(content); len(errs) > 0 {
			archiveMsg = textbuf.StrIntStr(" (archive: ", int64(len(errs)), " error(s))")
		}
	}

	var tb textbuf.Buffer
	tb.Str("Configuration committed (daemon not running)").Str(archiveMsg)
	AppendCommitWarnings(&tb, warnings)
	return commandResult{statusMessage: tb.String(), refreshConfig: true, revalidate: true}, nil
}

// AppendCommitWarnings writes one " (warning: <line>)" for each advisory line a
// commit produced. Every caller of it has already succeeded: a warning tells the
// operator what to look at and never says the commit failed.
func AppendCommitWarnings(tb *textbuf.Buffer, warnings []string) {
	for _, warning := range warnings {
		tb.Str(" (warning: ").Str(warning).Byte(')')
	}
}

func (m *Model) commitCandidateAndReload(detail string) (commandResult, error) {
	content, _, warnings, err := m.editor.StageCandidate(time.Now())
	if err != nil {
		return commandResult{}, err
	}
	m.searchCache = ""
	m.reloadErrors = nil
	if err := m.editor.NotifyReload(); err != nil {
		m.reloadErrors = []string{err.Error()}
		if clearErr := storage.ClearCandidate(m.editor.store, m.editor.originalPath); clearErr != nil {
			m.reloadErrors = append(m.reloadErrors, clearErr.Error())
		}
		var tb textbuf.Buffer
		return commandResult{
			statusMessage: tb.Str("commit failed: ").Err(err).String(),
			configView:    m.configViewAtPath(m.contextPath),
			revalidate:    true,
		}, nil
	}
	if err := m.editor.MarkCommittedContent(content); err != nil {
		// The commit landed: what its cleanup left undone is a warning on it.
		warnings = append(warnings, err.Error())
	}
	m.recordConfigCommit(detail)

	var archiveMsg string
	if m.editor.hasArchiveNotifier() {
		if errs := m.editor.notifyArchive([]byte(content)); len(errs) > 0 {
			archiveMsg = textbuf.StrIntStr(" (archive: ", int64(len(errs)), " error(s))")
		}
	}
	var tb2 textbuf.Buffer
	tb2.Str("Configuration committed and ").Str(m.editor.acceptedVerb()).Str(archiveMsg)
	AppendCommitWarnings(&tb2, warnings)
	return commandResult{statusMessage: tb2.String(), refreshConfig: true, revalidate: true}, nil
}

// cmdCommitSession commits only the current session's changes with conflict detection.
// Validates the resulting config before committing (same check as non-session commit).
// With force, warnings and conflicts do not block and the status counts the
// warnings; errors always block.
func (m *Model) cmdCommitSession(force bool) (commandResult, error) {
	result, _, err := m.runCommitSession(contract.CommitRequest{Action: contract.CommitNow, Force: force})
	return result, err
}

// runCommitSession is cmdCommitSession, also answering whether the commit
// reached the config. A commit that validation, a conflict or the daemon's
// reload refused answers false with the status saying why, so the daemon's
// confirm window opens only over a commit that happened; so does a commit
// with nothing pending, which answers contract.NothingToCommit. req is the
// subcommand typed, which names the forced form a validation refusal offers.
func (m *Model) runCommitSession(req contract.CommitRequest) (commandResult, bool, error) {
	force, forced := req.Force, contract.ForcedCommand(req)
	detail := m.editor.Diff()
	// Validate the current config before attempting commit.
	// Session mode uses set/delete commands that validate per-field, but
	// whole-config validation catches semantic issues (mandatory fields, etc.).
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	if refusal, blocked := m.commitValidationRefusal(result, force, forced); blocked {
		return refusal, false, nil
	}

	var (
		commitResult *CommitResult
		content      string
		err          error
	)
	transactional := m.editor.HasReloadNotifier()
	// Force also overrides LIVE and STALE conflicts (AC-32).
	switch {
	case transactional && force:
		commitResult, content, err = m.editor.CommitSessionCandidateForce(time.Now())
	case transactional:
		commitResult, content, err = m.editor.CommitSessionCandidate(time.Now())
	case force:
		commitResult, err = m.editor.CommitSessionForce()
	default:
		commitResult, err = m.editor.CommitSession()
	}
	if err != nil {
		return commandResult{}, false, err
	}

	if len(commitResult.Conflicts) > 0 {
		var b textbuf.Buffer
		b.Str("Commit blocked by conflicts:\n")
		for _, c := range commitResult.Conflicts {
			switch c.Type {
			case ConflictLive:
				b.Str("  LIVE ").Str(c.Path).Str(": you=").Str(c.MyValue).Str(", ").Str(c.OtherUser).Byte('=').Str(c.OtherValue).Byte('\n')
			case ConflictStale:
				b.Str("  STALE ").Str(c.Path).Str(": you=").Str(c.MyValue).Str(", committed=").Str(c.OtherValue).Str(" (was ").Str(c.PreviousValue).Str(")\n")
			default:
				panic("BUG: invalid commit conflict type")
			}
		}
		b.Str("Re-set conflicting values to resolve.")
		return commandResult{
			output:        b.String(),
			statusMessage: textbuf.StrIntStr("commit blocked: ", int64(len(commitResult.Conflicts)), " conflict(s)"),
		}, false, nil
	}

	// AC-29: nothing pending is no commit, so no window opens over it and
	// every editor answers the same words.
	if commitResult.Applied == 0 {
		return commandResult{statusMessage: contract.NothingToCommit(req)}, false, nil
	}

	if transactional && commitResult.Applied > 0 {
		m.searchCache = ""
		m.reloadErrors = nil
		if err := m.editor.NotifyReload(); err != nil {
			m.reloadErrors = []string{err.Error()}
			if clearErr := storage.ClearCandidate(m.editor.store, m.editor.originalPath); clearErr != nil {
				m.reloadErrors = append(m.reloadErrors, clearErr.Error())
			}
			var tb3 textbuf.Buffer
			return commandResult{
				statusMessage: tb3.Str("commit failed: ").Err(err).String(),
				configView:    m.configViewAtPath(m.contextPath),
				revalidate:    true,
			}, false, nil
		}
		if err := m.editor.MarkCommittedContent(content); err != nil {
			// The commit landed: what its cleanup left undone is a warning on it.
			commitResult.Warnings = append(commitResult.Warnings, err.Error())
		}
	}

	m.searchCache = "" // tree changed, invalidate cached set-view
	m.recordConfigCommit(detail)

	var tb4 textbuf.Buffer
	if force {
		tb4.Str(WithSkippedWarnings(forced, len(result.Warnings), ""))
	}
	tb4.Str("Session committed: ").Int(int64(commitResult.Applied)).Str(" change(s) applied")
	AppendCommitWarnings(&tb4, commitResult.Warnings)
	if transactional && commitResult.Applied > 0 {
		tb4.Str(" and ").Str(m.editor.acceptedVerb())
	}

	// Archive config to remote locations (best-effort, non-fatal).
	if m.editor.hasArchiveNotifier() {
		archiveContent := m.editor.OriginalContent()
		if transactional {
			archiveContent = content
		}
		if errs := m.editor.notifyArchive([]byte(archiveContent)); len(errs) > 0 {
			tb4.Str(" (archive: ").Int(int64(len(errs))).Str(" error(s))")
		}
	}

	return commandResult{statusMessage: tb4.String(), refreshConfig: true, revalidate: true}, true, nil
}

// cmdDiscardSession discards session changes, requiring path or cmdAll.
func (m *Model) cmdDiscardSession(args []string) (commandResult, error) {
	if len(args) == 0 {
		return commandResult{}, errDiscardRequiresPathOrAllIn
	}

	var path []string
	if args[0] != cmdAll {
		path = args
	}

	detail := m.editor.Diff()
	if err := m.editor.DiscardSessionPath(path); err != nil {
		return commandResult{}, err
	}
	m.searchCache = "" // tree changed, invalidate cached set-view
	m.recordConfigDiscard(detail)

	msg := "Session changes discarded"
	if len(path) > 0 {
		var tb textbuf.Buffer
		msg = tb.Str("Discarded: ").Join(path, " ").String()
	}

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdDiscard reverts all changes.
func (m *Model) cmdDiscard() (commandResult, error) {
	detail := m.editor.Diff()
	if err := m.editor.Discard(); err != nil {
		return commandResult{}, err
	}
	m.searchCache = "" // tree changed, invalidate cached set-view
	m.recordConfigDiscard(detail)

	return commandResult{
		statusMessage: "Changes discarded",
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdErrors displays validation issues in the viewport.
// Called by the show | errors pipe filter.
func (m *Model) cmdErrors(_ []string) (commandResult, error) { //nolint:unparam // signature matches pipe filter pattern
	issues := make([]ConfigValidationError, 0, len(m.validationErrors)+len(m.validationWarnings))
	issues = append(issues, m.validationErrors...)
	issues = append(issues, m.validationWarnings...)

	var parts []string
	if len(issues) > 0 {
		parts = append(parts, formatIssueList(issues))
	}
	if len(m.reloadErrors) > 0 {
		parts = append(parts, "Reload errors:")
		parts = append(parts, m.reloadErrors...)
	}
	if len(parts) == 0 {
		return commandResult{output: "No issues"}, nil
	}
	return commandResult{output: textbuf.Join(parts, "\n")}, nil
}

// formatIssueList formats validation issues for viewport display.
// Used by both cmdErrors and cmdCommit failure output.
func formatIssueList(issues []ConfigValidationError) string {
	var b textbuf.Buffer
	b.Int(int64(len(issues))).Str(" issue(s):\n")
	for _, e := range issues {
		if e.Line > 0 {
			b.Str("  line ").Int(int64(e.Line)).Str(": ").Str(e.Message).Byte('\n')
		} else {
			b.Str("  ").Str(e.Message).Byte('\n')
		}
	}
	return b.String()
}
