// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_commands_show.go — show command content display
// Detail: model_commands_option.go — display option settings
// Detail: model_commands_edit.go — config tree mutation commands (set, delete, insert, activate, rename, copy)
// Detail: model_commands_commit.go — commit, rollback, and discard lifecycle
// Detail: model_commands_session.go — session visibility commands (who, disconnect, show changes)

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errCommitConfirmedNotYetSupportedIn         = errors.New("commit confirmed not yet supported in session mode (use 'commit')")
	errUsageCommitforceConfirmedSeconds         = errors.New("usage: commit [force] confirmed <seconds>")
	errWhoRequiresAnActiveEditingSession        = errors.New("who requires an active editing session")
	errDisconnectRequiresAnActiveEditingSession = errors.New("disconnect requires an active editing session")
	errUsageEditPath                            = errors.New("usage: edit <path>")
	errTemplateEditingWildcardNotYetSupported   = errors.New("template editing (wildcard *) not yet supported in tree mode")
)

// dispatchQueue makes the config commands of one CLI session run one at a
// time, in the order the operator entered them.
//
// Bubble Tea starts a new goroutine for every tea.Cmd and never waits for it
// (Program.handleCommands). Model has a value receiver, so every copy shares
// one *Editor, one *Completer, one *ConfigValidator and one *History. Update
// runs serially. The commands it returns do not.
//
// An operator who pastes a block over SSH therefore had two mutating
// goroutines on one editor and its backing files. Bubble Tea enables no
// bracketed paste, so each pasted newline arrives as its own KeyPressMsg. Each
// one dispatches before the previous command answered.
//
// Ordering is the point, not only exclusion. A plain mutex lets "commit" win
// the race against the "set" typed before it. The config written then misses
// that edit, and nothing on screen says so. Refusing the second command while
// the first is in flight is a different product again. It drops half of what
// the operator pasted.
type dispatchQueue struct {
	mu   sync.Mutex
	prev chan struct{} // closed when the turn before the next one ends
}

// newDispatchQueue returns an empty queue whose first reserved turn starts
// immediately.
func newDispatchQueue() *dispatchQueue {
	first := make(chan struct{})
	close(first)
	return &dispatchQueue{prev: first}
}

// reserve claims the next turn. It returns the channel that closes when the
// turn before this one ended, and the function that ends this turn.
//
// Call reserve from Update, never from inside the tea.Cmd closure. Update is
// the only place Bubble Tea runs serially, so the caller's order is the
// operator's order. The caller MUST call done exactly once. A turn that never
// ends blocks every command after it.
func (q *dispatchQueue) reserve() (wait <-chan struct{}, done func()) {
	mine := make(chan struct{})
	q.mu.Lock()
	prev := q.prev
	q.prev = mine
	q.mu.Unlock()
	return prev, sync.OnceFunc(func() { close(mine) })
}

// executeCommand dispatches a command for execution.
// Returns a tea.Cmd that produces a commandResultMsg for the Update handler.
//
// The turn is reserved here, on the Update goroutine, so pasted commands keep
// their order. The closure waits for the previous command before it touches
// the shared editor. See dispatchQueue.
func (m Model) executeCommand(input string) tea.Cmd {
	wait, done := m.dispatch.reserve()
	return func() tea.Msg {
		<-wait
		defer done()
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
		fullPath = m.editor.autoSelectListEntry(fullPath)
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

// appendNewCompletions appends only completions whose Text is not already present.
func appendNewCompletions(existing, extra []Completion) []Completion {
	if len(extra) == 0 {
		return existing
	}
	seen := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		seen[c.Text] = struct{}{}
	}
	for _, c := range extra {
		if _, ok := seen[c.Text]; !ok {
			existing = append(existing, c)
		}
	}
	return existing
}
