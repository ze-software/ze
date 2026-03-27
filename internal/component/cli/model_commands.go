// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_commands_show.go — show command content display
// Detail: model_commands_option.go — display option settings

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// executeCommand dispatches a command for execution.
// Returns a tea.Cmd that produces a commandResultMsg for the Update handler.
func (m Model) executeCommand(input string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.dispatchCommand(input)
		return commandResultMsg{result: result, err: err}
	}
}

// dispatchCommand parses and executes a command.
// Returns commandResult with all state changes for the Update handler to apply.
func (m *Model) dispatchCommand(input string) (commandResult, error) {
	tokens := tokenizeCommand(input)
	if len(tokens) == 0 {
		return commandResult{}, nil
	}

	cmd := tokens[0]
	args := tokens[1:]

	// Check for pipe in command
	if pipeIdx := findPipeIndex(tokens); pipeIdx > 0 {
		return m.dispatchWithPipe(tokens[:pipeIdx], tokens[pipeIdx+1:])
	}

	// Guard: edit commands require an editor.
	// Only exit/quit, help, and run work without one.
	if m.editor == nil && cmd != cmdExit && cmd != cmdQuit && cmd != cmdHelp && cmd != "?" && cmd != cmdRun {
		return commandResult{}, fmt.Errorf("command %q requires edit mode (no config file loaded)", cmd)
	}

	switch cmd {
	case cmdExit, cmdQuit:
		// Handled directly in handleEnter() before dispatch — should not reach here.
		return commandResult{}, nil

	case cmdHelp, "?":
		return commandResult{showHelp: true}, nil

	case cmdTop:
		return m.cmdTop()

	case cmdUp:
		return m.cmdUp()

	case cmdEdit:
		return m.cmdEdit(args)

	case cmdShow:
		return m.cmdShow(args)

	case cmdOption:
		return m.cmdOption(args)

	case cmdCompare:
		return commandResult{output: m.editor.Diff()}, nil

	case cmdCommit:
		// Session-aware commit: use CommitSession when a session is active.
		if m.editor.HasSession() {
			if len(args) >= 1 && args[0] == cmdConfirmed {
				return commandResult{}, fmt.Errorf("commit confirmed not yet supported in session mode (use 'commit')")
			}
			return m.cmdCommitSession()
		}
		// Check for "commit confirmed <N>"
		if len(args) >= 1 && args[0] == cmdConfirmed {
			if len(args) < 2 {
				return commandResult{}, fmt.Errorf("usage: commit confirmed <seconds>")
			}
			seconds, err := strconv.Atoi(args[1])
			if err != nil {
				return commandResult{}, fmt.Errorf("invalid seconds: %s", args[1])
			}
			return m.cmdCommitConfirmed(seconds)
		}
		return m.cmdCommit()

	case cmdConfirm:
		return m.cmdConfirm()

	case cmdAbort:
		return m.cmdAbort()

	case cmdDiscard:
		// Session-aware discard: requires path or cmdAll when session is active.
		if m.editor.HasSession() {
			return m.cmdDiscardSession(args)
		}
		return m.cmdDiscard()

	case cmdHistory:
		return m.cmdHistory()

	case cmdRollback:
		return m.cmdRollback(args)

	case cmdLoad:
		// New syntax: load <source> <location> <action> [file]
		return m.cmdLoadNew(args)

	case cmdSet:
		return m.cmdSet(args)

	case cmdDelete:
		return m.cmdDelete(args)

	case cmdSave:
		return m.cmdSave()

	case cmdErrors:
		return m.cmdErrors(args)

	case cmdWho:
		if !m.editor.HasSession() {
			return commandResult{}, fmt.Errorf("who requires an active editing session")
		}
		return m.cmdWho()

	case cmdDisconnect:
		if !m.editor.HasSession() {
			return commandResult{}, fmt.Errorf("disconnect requires an active editing session")
		}
		return m.cmdDisconnectSession(args)
	}

	return commandResult{}, fmt.Errorf("unknown command: %s", cmd)
}

// Command implementations

func (m *Model) cmdTop() (commandResult, error) {
	if m.editor.WorkingContent() == "" {
		return commandResult{clearContext: true, output: "(empty configuration)"}, nil
	}
	return commandResult{
		clearContext: true,
		configView:   m.configViewAtPath(nil),
	}, nil
}

func (m *Model) cmdUp() (commandResult, error) {
	if len(m.contextPath) == 0 {
		return commandResult{output: "Already at top level"}, nil
	}

	// Try removing elements from the end until we find a valid parent.
	// Containers are 1 element (e.g., "bgp"), list entries are 2 (e.g., "peer", "1.1.1.1").
	// Use WalkPath to verify the parent exists in the tree.
	for removeCount := 1; removeCount <= 2 && removeCount <= len(m.contextPath); removeCount++ {
		newContext := m.contextPath[:len(m.contextPath)-removeCount]

		if len(newContext) == 0 {
			return commandResult{
				clearContext: true,
				configView:   m.configViewAtPath(nil),
			}, nil
		}

		// Verify this parent path resolves in the tree
		if m.editor.WalkPath(newContext) != nil {
			return commandResult{
				newContext: newContext,
				isTemplate: false,
				configView: m.configViewAtPath(newContext),
			}, nil
		}
	}

	// Fallback: go to root
	return commandResult{
		clearContext: true,
		configView:   m.configViewAtPath(nil),
	}, nil
}

func (m *Model) cmdEdit(args []string) (commandResult, error) {
	if len(args) == 0 {
		return commandResult{}, fmt.Errorf("usage: edit <path>")
	}

	// Check for wildcard template (e.g., "edit peer *")
	if len(args) >= 2 && args[len(args)-1] == "*" {
		// Template editing deferred to Part 2/3
		return commandResult{}, fmt.Errorf("template editing (wildcard *) not yet supported in tree mode")
	}

	// Build full path: current context + args (JUNOS-style relative navigation)
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// Verify the path exists in the tree.
	// If it doesn't resolve (e.g., list without KeyDefault), try auto-selecting
	// a single list entry before giving up.
	if m.editor.WalkPath(fullPath) == nil {
		fullPath = m.editor.AutoSelectListEntry(fullPath)
		if m.editor.WalkPath(fullPath) == nil {
			return commandResult{}, fmt.Errorf("block not found: %s", strings.Join(args, " "))
		}
	}

	return commandResult{
		newContext: fullPath,
		isTemplate: false,
		configView: m.configViewAtPath(fullPath),
	}, nil
}

// showConfigContent displays config content in viewport with proper highlighting.
// Used only in WindowSizeMsg handler for initial display.
func (m *Model) showConfigContent() {
	if m.editor == nil {
		return
	}
	if m.editor.ContentAtPath(m.contextPath) == "" {
		m.setViewportText("(empty configuration)")
		return
	}
	m.setViewportData(*m.configViewAtPath(m.contextPath))
}

func (m *Model) cmdHistory() (commandResult, error) {
	backups, err := m.editor.ListBackups()
	if err != nil {
		return commandResult{}, err
	}

	if len(backups) == 0 {
		return commandResult{output: "No backups found"}, nil
	}

	var b strings.Builder
	for i, backup := range backups {
		fmt.Fprintf(&b, "%d. %s  %s\n",
			i+1,
			backup.Timestamp.Format("2006-01-02 15:04:05"),
			backup.Path)
	}
	return commandResult{output: b.String()}, nil
}

// formatValidationErrors formats a slice of validation errors into a human-readable string.
func formatValidationErrors(errs []ConfigValidationError) string {
	if len(errs) == 1 {
		e := errs[0]
		if e.Line > 0 {
			return fmt.Sprintf("line %d: %s", e.Line, e.Message)
		}
		return e.Message
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d validation error(s):", len(errs))
	for _, e := range errs {
		if e.Line > 0 {
			fmt.Fprintf(&b, "\n  line %d: %s", e.Line, e.Message)
		} else {
			fmt.Fprintf(&b, "\n  %s", e.Message)
		}
	}
	return b.String()
}

func (m *Model) cmdRollback(args []string) (commandResult, error) {
	if len(args) != 1 {
		return commandResult{}, fmt.Errorf("usage: rollback <number>")
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

	return commandResult{
		statusMessage: fmt.Sprintf("Rolled back to %s", backups[n-1].Path),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

func (m *Model) cmdSet(args []string) (commandResult, error) {
	if len(args) < 2 {
		return commandResult{}, fmt.Errorf("usage: set <path> <value>")
	}

	// tokenizeCommand already handles quotes, so args are clean tokens.
	// Last token is value, everything before (with context) is the path.
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	value := fullPath[len(fullPath)-1]
	path := fullPath[:len(fullPath)-1]

	if len(path) < 1 {
		return commandResult{}, fmt.Errorf("usage: set <path> <value>")
	}

	key := path[len(path)-1]
	containerPath := path[:len(path)-1]

	// Validate the full token path (with list keys) against schema.
	// This catches missing list keys and unknown path elements.
	if _, err := m.completer.validateTokenPath(path); err != nil {
		return commandResult{}, err
	}

	// Validate value against YANG type before applying
	if err := m.completer.ValidateValueAtPath(path, value); err != nil {
		return commandResult{}, err
	}

	// Mutate the tree directly
	if err := m.editor.SetValue(containerPath, key, value); err != nil {
		return commandResult{}, fmt.Errorf("set failed: %w", err)
	}

	// Update completer with mutated tree
	m.completer.SetTree(m.editor.Tree())

	displayPath := append(append([]string{}, containerPath...), key)
	msg := fmt.Sprintf("set %s = %s", strings.Join(displayPath, " "), value)

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		msg += fmt.Sprintf(" (conflict with %s on %s)", conflicts[0].OtherUser, conflicts[0].Path)
	}

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// tokenizeCommand splits a command string into tokens, respecting quoted strings.
// Supports backslash escapes inside quotes: \" for literal quote, \\ for literal backslash.
// Example: `set peer "my peer" description "test"` → ["set", "peer", "my peer", "description", "test"].
func tokenizeCommand(input string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		// Handle escape sequences inside quotes
		if inQuote && c == '\\' && i+1 < len(input) {
			next := input[i+1]
			if next == '"' || next == '\\' {
				current.WriteByte(next)
				i++ // Skip the escaped character
				continue
			}
			// Unrecognized escape - treat backslash as literal
		}

		isQuote := c == '"'
		isSpace := c == ' ' || c == '\t'

		// Handle quote toggle
		if isQuote {
			tokens, inQuote = handleQuoteChar(&current, tokens, inQuote)
			continue
		}

		// Handle whitespace (token separator when not in quotes)
		if isSpace && !inQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		// Regular character (or space inside quotes)
		current.WriteByte(c)
	}

	// Add final token if any
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// handleQuoteChar processes a quote character during tokenization.
func handleQuoteChar(current *strings.Builder, tokens []string, inQuote bool) ([]string, bool) {
	if inQuote {
		// End of quoted string - add token without quotes
		tokens = append(tokens, current.String())
		current.Reset()
		return tokens, false
	}
	// Start of quoted string - save any accumulated content first
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
		current.Reset()
	}
	return tokens, true
}

// joinTokensWithQuotes joins tokens into a command string, quoting tokens that need it.
// Tokens containing spaces, tabs, quotes, or empty strings are quoted.
// Embedded backslashes and quotes are escaped for round-trip compatibility with tokenizeCommand.
func joinTokensWithQuotes(tokens []string) string {
	var parts []string
	for _, t := range tokens {
		if t == "" || strings.ContainsAny(t, " \t\"") {
			// Escape backslashes first, then quotes (order matters!)
			escaped := strings.ReplaceAll(t, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			parts = append(parts, "\""+escaped+"\"")
		} else {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func (m *Model) cmdDelete(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: delete <path>")
	}

	// Build full path with context
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// Use schema-aware delete to handle leaf values, containers, and list entries.
	if err := m.editor.DeleteByPath(fullPath); err != nil {
		return commandResult{}, fmt.Errorf("delete failed: %w", err)
	}

	// Update completer with mutated tree
	m.completer.SetTree(m.editor.Tree())

	msg := fmt.Sprintf("Deleted %s", strings.Join(fullPath, " "))

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		msg += fmt.Sprintf(" (conflict with %s on %s)", conflicts[0].OtherUser, conflicts[0].Path)
	}

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// runValidation re-runs validation on current content.
func (m *Model) runValidation() {
	if m.editor == nil || m.validator == nil {
		return
	}
	result := m.validator.Validate(m.editor.WorkingContent())
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
// If a ReloadNotifier is set (daemon was reachable at startup), attempts to reload.
// Reload failure does not fail the commit — config is saved regardless.
// Both errors and warnings block commit — config must be fully correct.
func (m *Model) cmdCommit() (commandResult, error) {
	// Validate inline - don't rely on m.validationErrors which may be stale
	// (m is captured by value in the tea.Cmd closure)
	result := m.validator.Validate(m.editor.WorkingContent())
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	issues = append(issues, result.Warnings...)
	if len(issues) > 0 {
		return commandResult{
			statusMessage: fmt.Sprintf("commit blocked: %d issue(s), type 'errors' for details", len(issues)),
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	// Save changes
	if err := m.editor.Save(); err != nil {
		return commandResult{}, err
	}
	m.searchCache = "" // tree changed, invalidate cached set-view

	// Archive config to remote locations (best-effort, non-fatal)
	var archiveMsg string
	if m.editor.HasArchiveNotifier() {
		content := []byte(m.editor.WorkingContent())
		if errs := m.editor.NotifyArchive(content); len(errs) > 0 {
			archiveMsg = fmt.Sprintf(" (archive: %d error(s))", len(errs))
		}
	}

	// Notify daemon of config change (best-effort)
	// refreshConfig tells handleCommandResult to recompute the viewport from the editor's
	// updated state — after Save(), original matches working so diff gutter clears.
	if !m.editor.HasReloadNotifier() {
		return commandResult{statusMessage: "Configuration committed (daemon not running)" + archiveMsg, refreshConfig: true, revalidate: true}, nil
	}
	if err := m.editor.NotifyReload(); err != nil {
		return commandResult{statusMessage: fmt.Sprintf("Configuration committed (reload failed: %v)", err) + archiveMsg, refreshConfig: true, revalidate: true}, nil
	}

	return commandResult{statusMessage: "Configuration committed and reloaded" + archiveMsg, refreshConfig: true, revalidate: true}, nil
}

// cmdCommitSession commits only the current session's changes with conflict detection.
// Validates the resulting config before committing (same check as non-session commit).
func (m *Model) cmdCommitSession() (commandResult, error) {
	// Validate the current config before attempting commit.
	// Session mode uses set/delete commands that validate per-field, but
	// whole-config validation catches semantic issues (mandatory fields, etc.).
	result := m.validator.Validate(m.editor.WorkingContent())
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	issues = append(issues, result.Warnings...)
	if len(issues) > 0 {
		return commandResult{
			statusMessage: fmt.Sprintf("commit blocked: %d issue(s), type 'errors' for details", len(issues)),
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	commitResult, err := m.editor.CommitSession()
	if err != nil {
		return commandResult{}, err
	}

	if len(commitResult.Conflicts) > 0 {
		var b strings.Builder
		b.WriteString("Commit blocked by conflicts:\n")
		for _, c := range commitResult.Conflicts {
			switch c.Type {
			case ConflictLive:
				fmt.Fprintf(&b, "  LIVE %s: you=%s, %s=%s\n", c.Path, c.MyValue, c.OtherUser, c.OtherValue)
			case ConflictStale:
				fmt.Fprintf(&b, "  STALE %s: you=%s, committed=%s (was %s)\n", c.Path, c.MyValue, c.OtherValue, c.PreviousValue)
			}
		}
		b.WriteString("Re-set conflicting values to resolve.")
		return commandResult{
			output:        b.String(),
			statusMessage: fmt.Sprintf("commit blocked: %d conflict(s)", len(commitResult.Conflicts)),
		}, nil
	}

	m.searchCache = "" // tree changed, invalidate cached set-view

	msg := fmt.Sprintf("Session committed: %d change(s) applied", commitResult.Applied)
	if commitResult.MigrationWarning != "" {
		msg += fmt.Sprintf(" (warning: %s)", commitResult.MigrationWarning)
	}

	// Archive config to remote locations (best-effort, non-fatal).
	if m.editor.HasArchiveNotifier() {
		content := []byte(m.editor.OriginalContent())
		if errs := m.editor.NotifyArchive(content); len(errs) > 0 {
			msg += fmt.Sprintf(" (archive: %d error(s))", len(errs))
		}
	}

	// Notify daemon of config change (best-effort).
	if m.editor.HasReloadNotifier() {
		if err := m.editor.NotifyReload(); err != nil {
			msg += fmt.Sprintf(" (reload failed: %v)", err)
		} else {
			msg += " and reloaded"
		}
	}

	return commandResult{statusMessage: msg, refreshConfig: true, revalidate: true}, nil
}

// cmdDiscardSession discards session changes, requiring path or cmdAll.
func (m *Model) cmdDiscardSession(args []string) (commandResult, error) {
	if len(args) == 0 {
		return commandResult{}, fmt.Errorf("discard requires path or 'all' in session mode")
	}

	var path []string
	if args[0] != cmdAll {
		path = args
	}

	if err := m.editor.DiscardSessionPath(path); err != nil {
		return commandResult{}, err
	}
	m.searchCache = "" // tree changed, invalidate cached set-view

	msg := "Session changes discarded"
	if len(path) > 0 {
		msg = fmt.Sprintf("Discarded: %s", strings.Join(path, " "))
	}

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdShowBlame displays blame-annotated configuration with per-line authorship.
func (m *Model) cmdShowBlame() (commandResult, error) {
	return commandResult{output: m.editor.BlameView()}, nil
}

// cmdShowChanges displays pending changes for the current session (default) or all sessions.
func (m *Model) cmdShowChanges(args []string) (commandResult, error) {
	showAll := len(args) > 0 && args[0] == cmdAll

	if showAll {
		return m.cmdShowChangesAll()
	}

	entries := m.editor.SessionChanges(m.editor.SessionID())
	if len(entries) == 0 {
		return commandResult{
			statusMessage: "No pending changes",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	msg := fmt.Sprintf("%d pending", len(entries))
	if len(entries) == 1 {
		msg += " change"
	} else {
		msg += " changes"
	}

	// Show tree with diff gutter, even if changes column is disabled.
	view := m.configViewAtPath(m.contextPath)
	view.forceChanges = true
	return commandResult{
		statusMessage: msg,
		configView:    view,
	}, nil
}

// formatChangeEntry writes a single change entry with appropriate marker and command.
// Handles set (new/modified) and delete entries.
func formatChangeEntry(b *strings.Builder, se config.SessionEntry) {
	if se.Entry.Value == "" {
		// Delete: no value in entry, Previous holds what was deleted.
		fmt.Fprintf(b, "  - delete %s  (was: %s)\n", se.Path, se.Entry.Previous)
		return
	}
	marker := '+'
	annotation := "(new)"
	if se.Entry.Previous != "" {
		marker = '*'
		annotation = fmt.Sprintf("(was: %s)", se.Entry.Previous)
	}
	fmt.Fprintf(b, "  %c set %s %s  %s\n", marker, se.Path, se.Entry.Value, annotation)
}

// cmdShowChangesAll displays pending changes summary grouped by session.
func (m *Model) cmdShowChangesAll() (commandResult, error) {
	sessions := m.editor.ActiveSessions()
	if len(sessions) == 0 {
		return commandResult{
			statusMessage: "No pending changes",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	total := 0
	for _, sid := range sessions {
		total += len(m.editor.SessionChanges(sid))
	}
	msg := fmt.Sprintf("%d pending", total)
	if total == 1 {
		msg += " change"
	} else {
		msg += " changes"
	}
	msg += fmt.Sprintf(" across %d sessions", len(sessions))
	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
	}, nil
}

// cmdWho lists active sessions with pending changes and change counts.
func (m *Model) cmdWho() (commandResult, error) {
	sessions := m.editor.ActiveSessions()
	if len(sessions) == 0 {
		return commandResult{output: "No active sessions."}, nil
	}

	var b strings.Builder
	b.WriteString("Active editing sessions:\n")
	myID := m.editor.SessionID()
	for _, sid := range sessions {
		marker := "  "
		if sid == myID {
			marker = "* "
		}
		entries := m.editor.SessionChanges(sid)
		changeWord := "changes"
		if len(entries) == 1 {
			changeWord = "change"
		}
		fmt.Fprintf(&b, "%s%s - %d pending %s\n", marker, sid, len(entries), changeWord)
	}
	return commandResult{output: b.String()}, nil
}

// cmdDisconnectSession removes another session's pending changes from the draft.
// Unrestricted for this spec -- any session can disconnect any other session.
// RBAC gating deferred to a future spec when ze gains a role/permission system.
func (m *Model) cmdDisconnectSession(args []string) (commandResult, error) {
	if len(args) == 0 {
		return commandResult{}, fmt.Errorf("usage: disconnect <session-id>")
	}
	targetSession := args[0]
	if targetSession == m.editor.SessionID() {
		return commandResult{}, fmt.Errorf("cannot disconnect own session (use 'discard %s' instead)", cmdAll)
	}

	if err := m.editor.DisconnectSession(targetSession); err != nil {
		return commandResult{}, err
	}

	return commandResult{
		statusMessage: fmt.Sprintf("Disconnected session: %s", targetSession),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdDiscard reverts all changes.
func (m *Model) cmdDiscard() (commandResult, error) {
	if err := m.editor.Discard(); err != nil {
		return commandResult{}, err
	}
	m.searchCache = "" // tree changed, invalidate cached set-view

	return commandResult{
		statusMessage: "Changes discarded",
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdErrors handles the errors command with subcommands:
//
//	errors / errors show — display validation issues in viewport.
//	errors hints — toggle inline diagnostic hints (← missing: ...).
//	errors hide — return to config view.
func (m *Model) cmdErrors(args []string) (commandResult, error) {
	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "show":
		issues := make([]ConfigValidationError, 0, len(m.validationErrors)+len(m.validationWarnings))
		issues = append(issues, m.validationErrors...)
		issues = append(issues, m.validationWarnings...)
		if len(issues) == 0 {
			return commandResult{output: "No validation issues"}, nil
		}
		return commandResult{output: formatIssueList(issues)}, nil

	case "hints":
		m.showHints = !m.showHints
		msg := "Inline hints disabled"
		if m.showHints {
			msg = "Inline hints enabled"
		}
		return commandResult{
			statusMessage: msg,
			configView:    m.configViewAtPath(m.contextPath),
		}, nil

	case "hide":
		return commandResult{
			statusMessage: "Errors hidden",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	return commandResult{}, fmt.Errorf("unknown errors subcommand: %s (use show, hints, or hide)", sub)
}

// formatIssueList formats validation issues for viewport display.
// Used by both cmdErrors and cmdCommit failure output.
func formatIssueList(issues []ConfigValidationError) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d issue(s):\n", len(issues))
	for _, e := range issues {
		if e.Line > 0 {
			fmt.Fprintf(&b, "  line %d: %s\n", e.Line, e.Message)
		} else {
			fmt.Fprintf(&b, "  %s\n", e.Message)
		}
	}
	return b.String()
}

// filterOutSessionCommands removes session-dependent commands and show subcommands
// (who, disconnect, blame, changes) from completions when no editing session is active.
func filterOutSessionCommands(completions []Completion) []Completion {
	result := make([]Completion, 0, len(completions))
	for _, c := range completions {
		if c.Text == cmdBlame || c.Text == cmdChanges || c.Text == cmdWho || c.Text == cmdDisconnect {
			continue
		}
		result = append(result, c)
	}
	return result
}
