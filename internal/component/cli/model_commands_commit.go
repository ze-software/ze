// Design: docs/architecture/config/yang-config-design.md — commit, rollback, and discard lifecycle
// Overview: model_commands.go — command dispatch

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errUsageRollbackNumber          = errors.New("usage: rollback <number>")
	errCommitForceNotYetSupportedIn = errors.New("commit force not yet supported in session mode (use 'commit')")
	errDiscardRequiresPathOrAllIn   = errors.New("discard requires path or 'all' in session mode")
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
	if err := m.editor.SaveEditState(); err != nil {
		return commandResult{}, err
	}
	return commandResult{statusMessage: "Configuration saved (snapshot)"}, nil
}

// cmdCommit saves changes with validation check.
// If a ReloadNotifier is set, stages a transactional candidate and asks the daemon to reload.
// Reload failure fails the commit and leaves the editor dirty.
// Both errors and warnings block commit — config must be fully correct.
func (m *Model) cmdCommit() (commandResult, error) {
	// Validate inline - don't rely on m.validationErrors which may be stale
	// (m is captured by value in the tea.Cmd closure)
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	issues = append(issues, result.Warnings...)
	if len(issues) > 0 {
		var b textbuf.Buffer
		return commandResult{
			statusMessage: b.Reset().Str("commit blocked: ").Int(int64(len(issues))).Str(" issue(s), type 'errors' for details").String(),
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
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
	// Session mode uses CommitSession which has its own validation path.
	// Force-skip of warnings is not yet supported there.
	if m.editor.HasSession() {
		return commandResult{}, errCommitForceNotYetSupportedIn
	}

	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	if len(result.Errors) > 0 {
		return commandResult{
			statusMessage: textbuf.StrIntStr("commit blocked: ", int64(len(result.Errors)), " error(s), type 'errors' for details"),
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	if len(result.Warnings) > 0 {
		m.statusMessage = textbuf.StrIntStr("commit force: skipping ", int64(len(result.Warnings)), " warning(s)")
	}

	return m.commitSaveAndReload()
}

// commitSaveAndReload performs the save, archive, and reload steps shared
// by cmdCommit and cmdCommitForce. Called after validation has passed.
func (m *Model) commitSaveAndReload() (commandResult, error) {
	detail := m.editor.Diff()
	if m.editor.HasReloadNotifier() {
		return m.commitCandidateAndReload(detail)
	}

	if err := m.editor.Save(); err != nil {
		return commandResult{}, err
	}
	m.recordConfigCommit(detail)
	m.searchCache = ""

	var archiveMsg string
	if m.editor.HasArchiveNotifier() {
		content := []byte(m.editor.WorkingContent())
		if errs := m.editor.NotifyArchive(content); len(errs) > 0 {
			archiveMsg = textbuf.StrIntStr(" (archive: ", int64(len(errs)), " error(s))")
		}
	}

	var tb textbuf.Buffer
	return commandResult{statusMessage: tb.Str("Configuration committed (daemon not running)").Str(archiveMsg).String(), refreshConfig: true, revalidate: true}, nil
}

func (m *Model) commitCandidateAndReload(detail string) (commandResult, error) {
	content, _, err := m.editor.StageCandidate(time.Now())
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
	m.editor.MarkCommittedContent(content)
	m.recordConfigCommit(detail)

	var archiveMsg string
	if m.editor.HasArchiveNotifier() {
		if errs := m.editor.NotifyArchive([]byte(content)); len(errs) > 0 {
			archiveMsg = textbuf.StrIntStr(" (archive: ", int64(len(errs)), " error(s))")
		}
	}
	var tb2 textbuf.Buffer
	return commandResult{statusMessage: tb2.Str("Configuration committed and reloaded").Str(archiveMsg).String(), refreshConfig: true, revalidate: true}, nil
}

// cmdCommitSession commits only the current session's changes with conflict detection.
// Validates the resulting config before committing (same check as non-session commit).
func (m *Model) cmdCommitSession() (commandResult, error) {
	detail := m.editor.Diff()
	// Validate the current config before attempting commit.
	// Session mode uses set/delete commands that validate per-field, but
	// whole-config validation catches semantic issues (mandatory fields, etc.).
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	issues = append(issues, result.Warnings...)
	if len(issues) > 0 {
		return commandResult{
			statusMessage: textbuf.StrIntStr("commit blocked: ", int64(len(issues)), " issue(s), type 'errors' for details"),
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	var (
		commitResult *CommitResult
		content      string
		err          error
	)
	transactional := m.editor.HasReloadNotifier()
	if transactional {
		commitResult, content, err = m.editor.CommitSessionCandidate(time.Now())
	} else {
		commitResult, err = m.editor.CommitSession()
	}
	if err != nil {
		return commandResult{}, err
	}

	if len(commitResult.Conflicts) > 0 {
		var b textbuf.Buffer
		b.Str("Commit blocked by conflicts:\n")
		for _, c := range commitResult.Conflicts {
			switch c.Type { //nolint:exhaustive // only two conflict types exist
			case ConflictLive:
				b.Str("  LIVE ").Str(c.Path).Str(": you=").Str(c.MyValue).Str(", ").Str(c.OtherUser).Byte('=').Str(c.OtherValue).Byte('\n')
			case ConflictStale:
				b.Str("  STALE ").Str(c.Path).Str(": you=").Str(c.MyValue).Str(", committed=").Str(c.OtherValue).Str(" (was ").Str(c.PreviousValue).Str(")\n")
			}
		}
		b.Str("Re-set conflicting values to resolve.")
		return commandResult{
			output:        b.String(),
			statusMessage: textbuf.StrIntStr("commit blocked: ", int64(len(commitResult.Conflicts)), " conflict(s)"),
		}, nil
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
			}, nil
		}
		m.editor.MarkCommittedContent(content)
	}

	m.searchCache = "" // tree changed, invalidate cached set-view
	m.recordConfigCommit(detail)

	var tb4 textbuf.Buffer
	tb4.Str("Session committed: ").Int(int64(commitResult.Applied)).Str(" change(s) applied")
	if commitResult.MigrationWarning != "" {
		tb4.Str(" (warning: ").Str(commitResult.MigrationWarning).Byte(')')
	}
	if transactional && commitResult.Applied > 0 {
		tb4.Str(" and reloaded")
	}

	// Archive config to remote locations (best-effort, non-fatal).
	if m.editor.HasArchiveNotifier() {
		archiveContent := m.editor.OriginalContent()
		if transactional {
			archiveContent = content
		}
		if errs := m.editor.NotifyArchive([]byte(archiveContent)); len(errs) > 0 {
			tb4.Str(" (archive: ").Int(int64(len(errs))).Str(" error(s))")
		}
	}

	return commandResult{statusMessage: tb4.String(), refreshConfig: true, revalidate: true}, nil
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
