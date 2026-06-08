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
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

var (
	errCommitConfirmedNotYetSupportedIn         = errors.New("commit confirmed not yet supported in session mode (use 'commit')")
	errUsageCommitforceConfirmedSeconds         = errors.New("usage: commit [force] confirmed <seconds>")
	errWhoRequiresAnActiveEditingSession        = errors.New("who requires an active editing session")
	errDisconnectRequiresAnActiveEditingSession = errors.New("disconnect requires an active editing session")
	errUsageEditPath                            = errors.New("usage: edit <path>")
	errTemplateEditingWildcardNotYetSupported   = errors.New("template editing (wildcard *) not yet supported in tree mode")
	errUsageRollbackNumber                      = errors.New("usage: rollback <number>")
	errUsageSetPathValue                        = errors.New("usage: set <path> <value>")
	errUsageDeletePath                          = errors.New("usage: delete <path>")
	errUsageInsertPathValueFirstlastbeforeRef   = errors.New("usage: insert <path> <value> first|last|before <ref>|after <ref>")
	errInsertFailedTargetIsNotA                 = errors.New("insert failed: target is not a leaf-list")
	errCommitForceNotYetSupportedIn             = errors.New("commit force not yet supported in session mode (use 'commit')")
	errDiscardRequiresPathOrAllIn               = errors.New("discard requires path or 'all' in session mode")
	errUsageDisconnectSessionId                 = errors.New("usage: disconnect <session-id>")
	errUsageRenamePathOldNameTo                 = errors.New("usage: rename <path> <old-name> to <new-name>")
	errUsageCopyPathSourceToDestination         = errors.New("usage: copy <path> <source> to <destination>")
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
		return commandResult{}, fmt.Errorf("command %q requires config mode (no config file loaded)", cmd)
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
				return commandResult{}, errCommitConfirmedNotYetSupportedIn
			}
			if len(commitArgs) < 2 {
				return commandResult{}, errUsageCommitforceConfirmedSeconds
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
		if len(args) >= 1 && args[0] == cmdAbort {
			return m.cmdAbort()
		}
		return m.cmdConfirm()

	case cmdDiscard:
		// Session-aware discard: requires path or cmdAll when session is active.
		if m.editor.HasSession() {
			return m.cmdDiscardSession(args)
		}
		return m.cmdDiscard()

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

	case cmdWho:
		if !m.editor.HasSession() {
			return commandResult{}, errWhoRequiresAnActiveEditingSession
		}
		return m.cmdWho()

	case cmdDisconnect:
		if !m.editor.HasSession() {
			return commandResult{}, errDisconnectRequiresAnActiveEditingSession
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
		return commandResult{}, errUsageEditPath
	}

	// Check for wildcard template (e.g., "edit peer *")
	if len(args) >= 2 && args[len(args)-1] == "*" {
		// Template editing deferred to Part 2/3
		return commandResult{}, errTemplateEditingWildcardNotYetSupported
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
			return commandResult{}, fmt.Errorf("block not found: %s", textbuf.Join(args, " "))
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

func (m *Model) cmdSet(args []string) (commandResult, error) {
	if len(args) < 2 {
		return commandResult{}, errUsageSetPathValue
	}

	// tokenizeCommand already handles quotes, so args are clean tokens.
	// Last token is value, everything before (with context) is the path.
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	value := fullPath[len(fullPath)-1]
	path := fullPath[:len(fullPath)-1]

	if len(path) < 1 {
		return commandResult{}, errUsageSetPathValue
	}

	key := path[len(path)-1]
	containerPath := path[:len(path)-1]

	// When the path ends at a list's key leaf keyword (e.g., "next-hop address"),
	// the value is the key for a new list entry. Check BEFORE validateTokenPath
	// because the keyword at end-of-path would be rejected as "missing key value".
	isListKey := m.editor.IsListKeyLeafPath(path)
	if isListKey {
		listName := containerPath[len(containerPath)-1]
		listParent := containerPath[:len(containerPath)-1]
		if err := m.editor.EnsureListEntry(listParent, listName, value); err != nil {
			return commandResult{}, fmt.Errorf("set failed: %w", err)
		}
	} else {
		// Validate the full token path (with list keys) against schema.
		if _, err := m.completer.validateTokenPath(path); err != nil {
			return commandResult{}, err
		}
		// Validate value against YANG type before applying
		if err := m.completer.ValidateValueAtPath(path, value); err != nil {
			return commandResult{}, err
		}
		if err := m.editor.SetValue(containerPath, key, value); err != nil {
			return commandResult{}, fmt.Errorf("set failed: %w", err)
		}
	}

	// Update completer with mutated tree
	m.refreshCompleter()

	var tb textbuf.Buffer
	if isListKey {
		tb.Str("created ").Str(containerPath[len(containerPath)-1]).Byte(' ').Str(value)
	} else {
		displayPath := append(append([]string{}, containerPath...), key)
		tb.Str("set ").Join(displayPath, " ").Str(" = ").Str(value)
	}

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// tokenizeCommand splits a command string into tokens, respecting quoted strings.
// Backslash has no special meaning (no escape sequences).
// Example: `set peer "my peer" description "test"` → ["set", "peer", "my peer", "description", "test"].
func tokenizeCommand(input string) []string {
	var tokens []string
	var current textbuf.Buffer
	inQuote := false

	for i := range len(input) {
		c := input[i]

		isQuote := c == '"'
		isSpace := c == ' ' || c == '\t'

		if isQuote {
			tokens, inQuote = handleQuoteChar(&current, tokens, inQuote)
			continue
		}

		if isSpace && !inQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.Byte(c)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// handleQuoteChar processes a quote character during tokenization.
func handleQuoteChar(current *textbuf.Buffer, tokens []string, inQuote bool) ([]string, bool) {
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
// Tokens containing spaces, tabs, or empty strings are quoted.
func joinTokensWithQuotes(tokens []string) string {
	var b textbuf.Buffer
	for i, t := range tokens {
		if i > 0 {
			b.Byte(' ')
		}
		if t == "" || strings.ContainsAny(t, " \t") {
			b.Byte('"').Str(t).Byte('"')
		} else {
			b.Str(t)
		}
	}
	return b.String()
}

func (m *Model) cmdDelete(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, errUsageDeletePath
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
	m.refreshCompleter()

	var tb textbuf.Buffer
	tb.Str("Deleted ").Join(fullPath, " ")

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb.String()

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
			m.refreshCompleter()
			var tb textbuf.Buffer
			tb.Str(pastTense).Byte(' ').Str(value).Str(" in ").Str(leafListName)
			if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
				tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
			}
			msg := tb.String()
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
			var tb textbuf.Buffer
			return commandResult{
				statusMessage: tb.Join(fullPath, " ").Str(" already ").Str(alreadyState).String(),
				configView:    m.configViewAtPath(m.contextPath),
			}, nil
		}
		return commandResult{}, fmt.Errorf("%s failed: %w", verb, opErr)
	}

	m.refreshCompleter()
	var tb2 textbuf.Buffer
	tb2.Str(pastTense).Byte(' ').Join(fullPath, " ")
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		tb2.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb2.String()
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
		return commandResult{}, errUsageInsertPathValueFirstlastbeforeRef
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
		return commandResult{}, errUsageInsertPathValueFirstlastbeforeRef
	}

	if len(pathAndValue) < 2 {
		return commandResult{}, errUsageInsertPathValueFirstlastbeforeRef
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
		return commandResult{}, errInsertFailedTargetIsNotA
	}

	if err := m.editor.InsertLeafListValue(containerPath, leafListName, value, position, ref); err != nil {
		return commandResult{}, fmt.Errorf("insert failed: %w", err)
	}

	m.refreshCompleter()
	m.searchCache = ""

	var tb textbuf.Buffer
	tb.Str("Inserted ").Str(value).Str(" into ").Str(leafListName).Byte(' ').Str(position)
	if ref != "" {
		tb.Byte(' ').Str(ref)
	}

	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		tb.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb.String()

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

// cmdShowBlame displays blame-annotated configuration with per-line authorship.
func (m *Model) cmdShowBlame() (commandResult, error) { //nolint:unparam // dispatch table requires (commandResult, error)
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

	var tb5 textbuf.Buffer
	tb5.Int(int64(len(changes))).Str(" pending")
	if len(changes) == 1 {
		tb5.Str(" change")
	} else {
		tb5.Str(" changes")
	}
	msg := tb5.String()

	// Show tree with diff gutter, even if changes column is disabled.
	view := m.configViewAtPath(m.contextPath)
	view.forceChanges = true
	return commandResult{
		statusMessage: msg,
		configView:    view,
	}, nil
}

// formatChangeEntry writes a single change entry with appropriate marker and command.
func formatChangeEntry(b *textbuf.Buffer, change config.PendingChange) {
	switch change.Kind {
	case config.PendingChangeDelete:
		b.Str("  - delete ").Str(change.Path).Str("  (was: ").Str(change.Previous).Str(")\n")
	case config.PendingChangeRename:
		b.Str("  ~ rename ").Str(change.OldPath).Str(" to ").Str(change.NewPath).Byte('\n')
	default:
		marker := byte('+')
		annotation := "(new)"
		if change.Previous != "" {
			marker = '*'
			var tb textbuf.Buffer
			annotation = tb.Str("(was: ").Str(change.Previous).Byte(')').String()
		}
		b.Str("  ").Byte(marker).Str(" set ").Str(change.Path).Byte(' ').Str(change.Value).Str("  ").Str(annotation).Byte('\n')
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
	var tb6 textbuf.Buffer
	tb6.Int(int64(total)).Str(" pending")
	if total == 1 {
		tb6.Str(" change")
	} else {
		tb6.Str(" changes")
	}
	tb6.Str(" across ").Int(int64(len(sessions))).Str(" sessions")
	msg := tb6.String()
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

	var b textbuf.Buffer
	b.Str("Active editing sessions:\n")
	myID := m.editor.SessionID()
	for _, sid := range sessions {
		if sid == myID {
			b.Str("* ")
		} else {
			b.Str("  ")
		}
		changes := m.editor.PendingChanges(sid)
		changeWord := "changes"
		if len(changes) == 1 {
			changeWord = "change"
		}
		b.Str(sid).Str(" - ").Int(int64(len(changes))).Str(" pending ").Str(changeWord).Byte('\n')
	}
	return commandResult{output: b.String()}, nil
}

// cmdDisconnectSession removes another session's pending changes from the draft.
// Unrestricted for this spec -- any session can disconnect any other session.
// RBAC gating deferred to a future spec when ze gains a role/permission system.
func (m *Model) cmdDisconnectSession(args []string) (commandResult, error) {
	if len(args) == 0 {
		return commandResult{}, errUsageDisconnectSessionId
	}
	targetSession := args[0]
	if targetSession == m.editor.SessionID() {
		return commandResult{}, fmt.Errorf("cannot disconnect own session (use 'discard %s' instead)", cmdAll)
	}

	if err := m.editor.DisconnectSession(targetSession); err != nil {
		return commandResult{}, err
	}

	var tb7 textbuf.Buffer
	return commandResult{
		statusMessage: tb7.Str("Disconnected session: ").Str(targetSession).String(),
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

// cmdRename renames a list entry key, preserving its subtree and position.
// JunOS syntax: rename <list> <old-key> to <new-key>
// Works relative to current context.
//
//nolint:dupl // shares structure with cmdCopy but different operations (rename vs copy)
func (m *Model) cmdRename(args []string) (commandResult, error) {
	// "to" must be second-to-last: <path...> <old-key> to <new-key>
	// Searching from a fixed position avoids ambiguity when a list key is literally "to".
	if len(args) < 4 {
		return commandResult{}, errUsageRenamePathOldNameTo
	}
	toIdx := len(args) - 2
	if args[toIdx] != "to" {
		return commandResult{}, errUsageRenamePathOldNameTo
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
	m.refreshCompleter()
	m.searchCache = "" // tree changed, invalidate cached set-view

	var tb8 textbuf.Buffer
	tb8.Str("Renamed ").Str(listName).Byte(' ').Str(oldKey).Str(" to ").Str(newKey)

	// Detect conflicts with other users' change files after each edit.
	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		tb8.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb8.String()

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
		return commandResult{}, errUsageCopyPathSourceToDestination
	}
	toIdx := len(args) - 2
	if args[toIdx] != "to" {
		return commandResult{}, errUsageCopyPathSourceToDestination
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
	m.refreshCompleter()
	m.searchCache = "" // tree changed, invalidate cached set-view

	var tb9 textbuf.Buffer
	tb9.Str("Copied ").Str(listName).Byte(' ').Str(srcKey).Str(" to ").Str(dstKey)

	if conflicts := m.editor.DetectConflicts(); len(conflicts) > 0 {
		tb9.Str(" (conflict with ").Str(conflicts[0].OtherUser).Str(" on ").Str(conflicts[0].Path).Byte(')')
	}
	msg := tb9.String()

	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// filterOutSessionCommands removes session-dependent commands
// (who, disconnect) from completions when no editing session is active.
func filterOutSessionCommands(completions []Completion) []Completion {
	result := make([]Completion, 0, len(completions))
	for _, c := range completions {
		if c.Text == cmdWho || c.Text == cmdDisconnect {
			continue
		}
		result = append(result, c)
	}
	return result
}
