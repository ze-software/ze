// Design: docs/architecture/config/yang-config-design.md — config editor

package editor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	case cmdCompare:
		return commandResult{output: m.editor.Diff()}, nil

	case cmdCommit:
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

	case cmdErrors:
		return m.cmdErrors()
	}

	return commandResult{}, fmt.Errorf("unknown command: %s", cmd)
}

// Command implementations

func (m *Model) cmdTop() (commandResult, error) {
	content := m.editor.WorkingContent()
	if content == "" {
		return commandResult{clearContext: true, output: "(empty configuration)"}, nil
	}
	return commandResult{
		clearContext: true,
		configView:   &viewportData{content: content, lineMapping: nil},
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
			content := m.editor.WorkingContent()
			return commandResult{
				clearContext: true,
				configView:   &viewportData{content: content, lineMapping: nil},
			}, nil
		}

		// Verify this parent path resolves in the tree
		if m.editor.WalkPath(newContext) != nil {
			content := m.editor.ContentAtPath(newContext)
			return commandResult{
				newContext: newContext,
				isTemplate: false,
				configView: &viewportData{content: content, lineMapping: nil},
			}, nil
		}
	}

	// Fallback: go to root
	content := m.editor.WorkingContent()
	return commandResult{
		clearContext: true,
		configView:   &viewportData{content: content, lineMapping: nil},
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

	content := m.editor.ContentAtPath(fullPath)
	return commandResult{
		newContext: fullPath,
		isTemplate: false,
		configView: &viewportData{content: content, lineMapping: nil},
	}, nil
}

// showConfigContent displays config content in viewport with proper highlighting.
// Used only in WindowSizeMsg handler for initial display.
func (m *Model) showConfigContent() {
	content := m.editor.ContentAtPath(m.contextPath)
	if content == "" {
		m.setViewportText("(empty configuration)")
		return
	}
	m.setViewportData(viewportData{content: content, lineMapping: nil})
}

func (m *Model) cmdShow(_ []string) (commandResult, error) {
	content := m.editor.ContentAtPath(m.contextPath)
	if content == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}
	return commandResult{configView: &viewportData{content: content, lineMapping: nil}}, nil
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
		fmt.Fprintf(&b, "%d. %s (%s)\n",
			i+1,
			backup.Path,
			backup.Timestamp.Format("2006-01-02"))
	}
	return commandResult{output: b.String()}, nil
}

func (m *Model) cmdRollback(args []string) (commandResult, error) {
	if len(args) != 1 {
		return commandResult{}, fmt.Errorf("usage: rollback <number>")
	}

	var n int
	if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil {
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

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: fmt.Sprintf("Rolled back to %s", backups[n-1].Path),
		configView:    &viewportData{content: content, lineMapping: nil},
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

	content := m.editor.ContentAtPath(m.contextPath)
	displayPath := append(append([]string{}, containerPath...), key)
	return commandResult{
		statusMessage: fmt.Sprintf("Set %s = %s", strings.Join(displayPath, " "), value),
		configView:    &viewportData{content: content, lineMapping: nil},
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

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: fmt.Sprintf("Deleted %s", strings.Join(fullPath, " ")),
		configView:    &viewportData{content: content, lineMapping: nil},
		revalidate:    true,
	}, nil
}

// runValidation re-runs validation on current content.
func (m *Model) runValidation() {
	result := m.validator.Validate(m.editor.WorkingContent())
	m.validationErrors = result.Errors
	m.validationWarnings = result.Warnings
}

// scheduleValidation returns a command to trigger validation after debounce delay.
func (m *Model) scheduleValidation() tea.Cmd {
	m.validationID++
	id := m.validationID
	return tea.Tick(validationDebounce, func(_ time.Time) tea.Msg {
		return validationTickMsg{id: id}
	})
}

// cmdCommit saves changes with validation check.
// If a ReloadNotifier is set (daemon was reachable at startup), attempts to reload.
// Reload failure does not fail the commit — config is saved regardless.
func (m *Model) cmdCommit() (commandResult, error) {
	// Validate inline - don't rely on m.validationErrors which may be stale
	// (m is captured by value in the tea.Cmd closure)
	result := m.validator.Validate(m.editor.WorkingContent())
	if len(result.Errors) > 0 {
		return commandResult{}, fmt.Errorf("cannot commit: %d validation error(s). Use 'errors' to see details", len(result.Errors))
	}

	// Save changes
	if err := m.editor.Save(); err != nil {
		return commandResult{}, err
	}

	// Notify daemon of config change (best-effort)
	if !m.editor.HasReloadNotifier() {
		return commandResult{statusMessage: "Configuration committed (daemon not running)"}, nil
	}
	if err := m.editor.NotifyReload(); err != nil {
		return commandResult{statusMessage: fmt.Sprintf("Configuration committed (reload failed: %v)", err)}, nil
	}

	return commandResult{statusMessage: "Configuration committed and reloaded"}, nil
}

// cmdDiscard reverts all changes.
func (m *Model) cmdDiscard() (commandResult, error) {
	if err := m.editor.Discard(); err != nil {
		return commandResult{}, err
	}

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: "Changes discarded",
		configView:    &viewportData{content: content, lineMapping: nil},
		revalidate:    true,
	}, nil
}

// cmdErrors displays validation errors.
func (m *Model) cmdErrors() (commandResult, error) {
	if len(m.validationErrors) == 0 && len(m.validationWarnings) == 0 {
		return commandResult{output: "No validation issues"}, nil
	}

	var b strings.Builder

	if len(m.validationErrors) > 0 {
		fmt.Fprintf(&b, "Errors (%d):\n", len(m.validationErrors))
		for _, e := range m.validationErrors {
			if e.Line > 0 {
				fmt.Fprintf(&b, "  Line %d: %s\n", e.Line, e.Message)
			} else {
				fmt.Fprintf(&b, "  %s\n", e.Message)
			}
		}
	}

	if len(m.validationWarnings) > 0 {
		if len(m.validationErrors) > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Warnings (%d):\n", len(m.validationWarnings))
		for _, w := range m.validationWarnings {
			if w.Line > 0 {
				fmt.Fprintf(&b, "  Line %d: %s\n", w.Line, w.Message)
			} else {
				fmt.Fprintf(&b, "  %s\n", w.Message)
			}
		}
	}

	return commandResult{output: b.String()}, nil
}
