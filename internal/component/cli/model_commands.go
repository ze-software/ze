// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_commands_show.go — show command content display
// Detail: model_commands_option.go — display option settings

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

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
	if pipeIdx := FindPipeIndex(tokens); pipeIdx > 0 {
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
		// Parse force flag: "commit force", "commit force confirmed <N>"
		force := len(args) >= 1 && args[0] == "force"
		commitArgs := args
		if force {
			commitArgs = args[1:] // strip "force" for further parsing
		}

		// "commit [force] confirmed <N>" -- commit with auto-rollback
		if len(commitArgs) >= 1 && commitArgs[0] == cmdConfirmed {
			if m.editor.HasSession() {
				return commandResult{}, fmt.Errorf("commit confirmed not yet supported in session mode (use 'commit')")
			}
			if len(commitArgs) < 2 {
				return commandResult{}, fmt.Errorf("usage: commit [force] confirmed <seconds>")
			}
			seconds, err := strconv.Atoi(commitArgs[1])
			if err != nil {
				return commandResult{}, fmt.Errorf("invalid seconds: %s", commitArgs[1])
			}
			return m.cmdCommitConfirmed(seconds, force)
		}

		// "commit force" -- skip warnings
		if force {
			return m.cmdCommitForce()
		}

		// Session-aware commit: use CommitSession when a session is active.
		if m.editor.HasSession() {
			return m.cmdCommitSession()
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

	case cmdDeactivate:
		return m.cmdDeactivate(args)

	case cmdActivate:
		return m.cmdActivate(args)

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
	case cmdRename:
		return m.cmdRename(args)
	case cmdCopy:
		return m.cmdCopy(args)
	case cmdInsert:
		return m.cmdInsert(args)
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

// cmdDeactivate marks a config node as inactive.
func (m *Model) cmdDeactivate(args []string) (commandResult, error) {
	return m.runActivation(args, false)
}

// cmdActivate clears the inactive flag from a config node.
func (m *Model) cmdActivate(args []string) (commandResult, error) {
	return m.runActivation(args, true)
}

// runActivation backs both cmdDeactivate (activate=false) and cmdActivate
// (activate=true). The two verbs share path resolution, leaf-list-value
// detection, and idempotent-error mapping; only the editor methods and
// the wording of the status messages differ.
//
//nolint:cyclop // exhaustive node-type dispatch
func (m *Model) runActivation(args []string, activate bool) (commandResult, error) {
	verb := "deactivate"
	pastTense := "Deactivated"
	alreadyState := "deactivated"
	if activate {
		verb = "activate"
		pastTense = "Activated"
		alreadyState = "active"
	}

	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: %s <path>", verb)
	}

	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// Leaf-list value path.
	if len(fullPath) >= 2 {
		parentPath, leafListName, isLeafList := m.resolveLeafListValue(fullPath)
		if isLeafList {
			value := fullPath[len(fullPath)-1]
			var llErr error
			if activate {
				llErr = m.editor.ActivateLeafListValue(parentPath, leafListName, value)
			} else {
				llErr = m.editor.DeactivateLeafListValue(parentPath, leafListName, value)
			}
			if llErr != nil {
				return commandResult{}, fmt.Errorf("%s failed: %w", verb, llErr)
			}
			m.completer.SetTree(m.editor.Tree())
			msg := fmt.Sprintf("%s %s in %s", pastTense, value, leafListName)
			if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
				msg += fmt.Sprintf(" (conflict with %s on %s)", conflicts[0].OtherUser, conflicts[0].Path)
			}
			return commandResult{
				statusMessage: msg,
				configView:    m.configViewAtPath(m.contextPath),
				revalidate:    true,
			}, nil
		}
	}

	// Schema-validated leaf vs container/list-entry dispatch.
	entry, err := m.completer.validateTokenPath(fullPath)
	if err != nil {
		return commandResult{}, err
	}
	var opErr error
	switch {
	case entry != nil && entry.IsLeaf():
		parentPath := fullPath[:len(fullPath)-1]
		leafName := fullPath[len(fullPath)-1]
		if activate {
			opErr = m.editor.ActivateLeaf(parentPath, leafName)
		} else {
			opErr = m.editor.DeactivateLeaf(parentPath, leafName)
		}
	case activate:
		opErr = m.editor.ActivatePath(fullPath)
	default:
		opErr = m.editor.DeactivatePath(fullPath)
	}

	if opErr != nil {
		// Idempotent: already-in-state becomes a status message.
		if errors.Is(opErr, ErrLeafAlreadyInactive) || errors.Is(opErr, ErrPathAlreadyInactive) ||
			errors.Is(opErr, ErrLeafNotInactive) || errors.Is(opErr, ErrPathNotInactive) {
			return commandResult{
				statusMessage: fmt.Sprintf("%s already %s", strings.Join(fullPath, " "), alreadyState),
				configView:    m.configViewAtPath(m.contextPath),
			}, nil
		}
		return commandResult{}, fmt.Errorf("%s failed: %w", verb, opErr)
	}

	m.completer.SetTree(m.editor.Tree())
	msg := fmt.Sprintf("%s %s", pastTense, strings.Join(fullPath, " "))
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		msg += fmt.Sprintf(" (conflict with %s on %s)", conflicts[0].OtherUser, conflicts[0].Path)
	}
	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// resolveLeafListValue is a thin wrapper around Editor.ResolveLeafListValue.
// Kept for the existing call sites; new code should use the Editor method
// directly.
func (m *Model) resolveLeafListValue(fullPath []string) (parentPath []string, leafListName string, ok bool) {
	if m.editor == nil {
		return nil, "", false
	}
	return m.editor.ResolveLeafListValue(fullPath)
}

// cmdInsert inserts a value into a leaf-list at a specified position.
// Syntax: insert <path> <value> first|last|before <ref>|after <ref>.
// Limitation: values named "first", "last", "before", or "after" are
// ambiguous with position keywords. Quote them if needed.
func (m *Model) cmdInsert(args []string) (commandResult, error) {
	if len(args) < 3 {
		return commandResult{}, fmt.Errorf("usage: insert <path> <value> first|last|before <ref>|after <ref>")
	}

	// Parse position from the end of args.
	var position, ref string
	var pathAndValue []string

	lastArg := args[len(args)-1]
	if lastArg == config.InsertFirst || lastArg == config.InsertLast {
		position = lastArg
		pathAndValue = args[:len(args)-1]
	} else if len(args) >= 4 {
		secondLast := args[len(args)-2]
		if secondLast == config.InsertBefore || secondLast == config.InsertAfter {
			position = secondLast
			ref = lastArg
			pathAndValue = args[:len(args)-2]
		}
	}

	if position == "" {
		return commandResult{}, fmt.Errorf("usage: insert <path> <value> first|last|before <ref>|after <ref>")
	}

	if len(pathAndValue) < 2 {
		return commandResult{}, fmt.Errorf("usage: insert <path> <value> first|last|before <ref>|after <ref>")
	}

	value := pathAndValue[len(pathAndValue)-1]
	pathTokens := pathAndValue[:len(pathAndValue)-1]

	// Build full path to the leaf-list: context + path tokens
	fullPath := make([]string, 0, len(m.contextPath)+len(pathTokens))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, pathTokens...)

	// Validate the target is a leaf-list using schema-aware path walk.
	// Append a dummy value so resolveLeafListValue sees the leaf-list as second-to-last.
	probePath := make([]string, len(fullPath)+1)
	copy(probePath, fullPath)
	probePath[len(fullPath)] = "__probe__"
	containerPath, leafListName, isLeafList := m.resolveLeafListValue(probePath)
	if !isLeafList {
		return commandResult{}, fmt.Errorf("insert failed: target is not a leaf-list")
	}

	if err := m.editor.InsertLeafListValue(containerPath, leafListName, value, position, ref); err != nil {
		return commandResult{}, fmt.Errorf("insert failed: %w", err)
	}

	m.completer.SetTree(m.editor.Tree())
	m.searchCache = ""

	msg := fmt.Sprintf("Inserted %s into %s %s", value, leafListName, position)
	if ref != "" {
		msg += " " + ref
	}

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
// If a ReloadNotifier is set (daemon was reachable at startup), attempts to reload.
// Reload failure does not fail the commit — config is saved regardless.
// Both errors and warnings block commit — config must be fully correct.
func (m *Model) cmdCommit() (commandResult, error) {
	// Validate inline - don't rely on m.validationErrors which may be stale
	// (m is captured by value in the tea.Cmd closure)
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
	issues = append(issues, result.Errors...)
	issues = append(issues, result.Warnings...)
	if len(issues) > 0 {
		return commandResult{
			statusMessage: fmt.Sprintf("commit blocked: %d issue(s), type 'errors' for details", len(issues)),
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
		return commandResult{}, fmt.Errorf("commit force not yet supported in session mode (use 'commit')")
	}

	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
	if len(result.Errors) > 0 {
		return commandResult{
			statusMessage: fmt.Sprintf("commit blocked: %d error(s), type 'errors' for details", len(result.Errors)),
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	if len(result.Warnings) > 0 {
		m.statusMessage = fmt.Sprintf("commit force: skipping %d warning(s)", len(result.Warnings))
	}

	return m.commitSaveAndReload()
}

// commitSaveAndReload performs the save, archive, and reload steps shared
// by cmdCommit and cmdCommitForce. Called after validation has passed.
func (m *Model) commitSaveAndReload() (commandResult, error) {
	if err := m.editor.Save(); err != nil {
		return commandResult{}, err
	}
	m.searchCache = ""

	var archiveMsg string
	if m.editor.HasArchiveNotifier() {
		content := []byte(m.editor.WorkingContent())
		if errs := m.editor.NotifyArchive(content); len(errs) > 0 {
			archiveMsg = fmt.Sprintf(" (archive: %d error(s))", len(errs))
		}
	}

	if !m.editor.HasReloadNotifier() {
		return commandResult{statusMessage: "Configuration committed (daemon not running)" + archiveMsg, refreshConfig: true, revalidate: true}, nil
	}
	reloadMsg := m.tryReload()
	return commandResult{statusMessage: "Configuration committed" + reloadMsg + archiveMsg, refreshConfig: true, revalidate: true}, nil
}

// cmdCommitSession commits only the current session's changes with conflict detection.
// Validates the resulting config before committing (same check as non-session commit).
func (m *Model) cmdCommitSession() (commandResult, error) {
	// Validate the current config before attempting commit.
	// Session mode uses set/delete commands that validate per-field, but
	// whole-config validation catches semantic issues (mandatory fields, etc.).
	result := m.validator.ValidateTransition(m.editor.OriginalContent(), m.editor.WorkingContent())
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
			switch c.Type { //nolint:exhaustive // only two conflict types exist
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
		msg += m.tryReload()
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

	changes := m.editor.PendingChanges(m.editor.SessionID())
	if len(changes) == 0 {
		return commandResult{
			statusMessage: "No pending changes",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	msg := fmt.Sprintf("%d pending", len(changes))
	if len(changes) == 1 {
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
func formatChangeEntry(b *strings.Builder, change config.PendingChange) {
	switch change.Kind {
	case config.PendingChangeDelete:
		fmt.Fprintf(b, "  - delete %s  (was: %s)\n", change.Path, change.Previous)
	case config.PendingChangeRename:
		fmt.Fprintf(b, "  ~ rename %s to %s\n", change.OldPath, change.NewPath)
	default:
		marker := '+'
		annotation := "(new)"
		if change.Previous != "" {
			marker = '*'
			annotation = fmt.Sprintf("(was: %s)", change.Previous)
		}
		fmt.Fprintf(b, "  %c set %s %s  %s\n", marker, change.Path, change.Value, annotation)
	}
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
		total += len(m.editor.PendingChanges(sid))
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
		changes := m.editor.PendingChanges(sid)
		changeWord := "changes"
		if len(changes) == 1 {
			changeWord = "change"
		}
		fmt.Fprintf(&b, "%s%s - %d pending %s\n", marker, sid, len(changes), changeWord)
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
		return commandResult{output: strings.Join(parts, "\n")}, nil

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

// cmdRename renames a list entry key, preserving its subtree and position.
// JunOS syntax: rename <list> <old-key> to <new-key>
// Works relative to current context.
//
//nolint:dupl // shares structure with cmdCopy but different operations (rename vs copy)
func (m *Model) cmdRename(args []string) (commandResult, error) {
	// "to" must be second-to-last: <path...> <old-key> to <new-key>
	// Searching from a fixed position avoids ambiguity when a list key is literally "to".
	if len(args) < 4 {
		return commandResult{}, fmt.Errorf("usage: rename <path> <old-name> to <new-name>")
	}
	toIdx := len(args) - 2
	if args[toIdx] != "to" {
		return commandResult{}, fmt.Errorf("usage: rename <path> <old-name> to <new-name>")
	}

	newKey := args[toIdx+1]
	oldTokens := args[:toIdx]

	// Build full path to old entry: context + args before "to"
	fullPath := make([]string, 0, len(m.contextPath)+len(oldTokens))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, oldTokens...)

	// Identify list name, old key, and parent path using schema
	parentPath, listName, oldKey, err := m.editor.resolveListTarget(fullPath)
	if err != nil {
		return commandResult{}, err
	}

	// Validate new key against YANG schema (same validation as set paths).
	newPath := make([]string, 0, len(parentPath)+2)
	newPath = append(newPath, parentPath...)
	newPath = append(newPath, listName, newKey)
	if _, err := m.completer.validateTokenPath(newPath); err != nil {
		return commandResult{}, fmt.Errorf("invalid new name: %w", err)
	}

	// Perform the rename
	if err := m.editor.RenameListEntry(parentPath, listName, oldKey, newKey); err != nil {
		return commandResult{}, fmt.Errorf("rename failed: %w", err)
	}

	// Update completer with mutated tree
	m.completer.SetTree(m.editor.Tree())
	m.searchCache = "" // tree changed, invalidate cached set-view

	msg := fmt.Sprintf("Renamed %s %s to %s", listName, oldKey, newKey)

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

// cmdCopy clones a list entry under a new key, preserving the source.
// JunOS syntax: copy <list> <old-key> to <new-key>
// Works relative to current context.
//
//nolint:dupl // shares structure with cmdRename but different operations (copy vs rename)
func (m *Model) cmdCopy(args []string) (commandResult, error) {
	// "to" must be second-to-last: <path...> <src-key> to <dst-key>
	if len(args) < 4 {
		return commandResult{}, fmt.Errorf("usage: copy <path> <source> to <destination>")
	}
	toIdx := len(args) - 2
	if args[toIdx] != "to" {
		return commandResult{}, fmt.Errorf("usage: copy <path> <source> to <destination>")
	}

	dstKey := args[toIdx+1]
	srcTokens := args[:toIdx]

	// Build full path to source entry: context + args before "to"
	fullPath := make([]string, 0, len(m.contextPath)+len(srcTokens))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, srcTokens...)

	// Identify list name, source key, and parent path using schema
	parentPath, listName, srcKey, err := m.editor.resolveListTarget(fullPath)
	if err != nil {
		return commandResult{}, err
	}

	// Validate destination key against YANG schema.
	newPath := make([]string, 0, len(parentPath)+2)
	newPath = append(newPath, parentPath...)
	newPath = append(newPath, listName, dstKey)
	if _, err := m.completer.validateTokenPath(newPath); err != nil {
		return commandResult{}, fmt.Errorf("invalid destination name: %w", err)
	}

	// Perform the copy
	if err := m.editor.CopyListEntry(parentPath, listName, srcKey, dstKey); err != nil {
		return commandResult{}, fmt.Errorf("copy failed: %w", err)
	}

	// Update completer with mutated tree
	m.completer.SetTree(m.editor.Tree())
	m.searchCache = "" // tree changed, invalidate cached set-view

	msg := fmt.Sprintf("Copied %s %s to %s", listName, srcKey, dstKey)

	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		msg += fmt.Sprintf(" (conflict with %s on %s)", conflicts[0].OtherUser, conflicts[0].Path)
	}

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
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
