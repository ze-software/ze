// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_commands_show.go — show command content display
// Detail: model_commands_option.go — display option settings
// Detail: model_commands_edit.go — config tree mutation commands (set, delete, insert, activate, rename, copy)
// Detail: model_commands_commit.go — commit, rollback, and discard lifecycle
// Detail: model_commands_session.go — session visibility commands (who, disconnect, show changes)

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// emptyConfiguration is what every show path prints when the tree it was asked
// for holds nothing. It is one sentence, so the editor and the show commands
// cannot disagree about how an empty tree reads.
const emptyConfiguration = "(empty configuration)"

var (
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
//
// A turn ends when Update has applied the command's result, not when the
// command returns. Bubble Tea sends each tea.Cmd's message from that command's
// own goroutine, so a turn that ended on return let the next command answer
// first, and the earlier result then overwrote the later one: `commit abort`
// typed after `commit confirmed` left the confirmed commit's status and an
// open window watch on screen over a closed window.
//
// A turn that ends in Update needs a stop path for the Update that never
// comes. When the program ends (the SSH client is killed, the operator exits)
// with a result undelivered, that turn never ends, and every command queued
// behind it would block on its turn forever, holding the Model and the Editor.
// end releases them: the session's owner calls it through Model.Close once the
// program has returned.
// The same owner carries the context and clock for timers and pollers across
// model copies. The clock is fixed before any command starts.
type dispatchQueue struct {
	mu      sync.Mutex
	prev    chan struct{} // closed when the turn before the next one ends
	pending int           // turns reserved and not yet ended
	ctx     context.Context
	cancel  context.CancelFunc
	clock   clock.Clock
}

// newDispatchQueue returns an empty queue whose first reserved turn starts
// immediately.
func newDispatchQueue() *dispatchQueue {
	first := make(chan struct{})
	close(first)
	ctx, cancel := context.WithCancel(context.Background())
	return &dispatchQueue{prev: first, ctx: ctx, cancel: cancel, clock: clock.RealClock{}}
}

// end releases every turn still waiting, because the session that would end
// them is over. Safe for concurrent use, and safe to call more than once.
func (q *dispatchQueue) end() {
	q.cancel()
}

// await blocks until the turn before this one ends or the session ends. It
// reports whether the turn may run: false means the session is over and the
// command MUST NOT touch the editor. The session's end wins over a turn that
// became ready at the same moment, so no command runs after Close.
func (q *dispatchQueue) await(wait <-chan struct{}) bool {
	select {
	case <-wait:
	case <-q.ctx.Done():
	}
	select {
	case <-q.ctx.Done():
		return false
	default:
		return true
	}
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
	q.pending++
	q.mu.Unlock()
	return prev, sync.OnceFunc(func() {
		q.mu.Lock()
		q.pending--
		q.mu.Unlock()
		close(mine)
	})
}

// busy reports whether a command is in flight: reserved, and its result not
// yet applied. Safe for concurrent use.
func (q *dispatchQueue) busy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending > 0
}

// executeCommand dispatches a command for execution.
// Returns a tea.Cmd that produces a commandResultMsg for the Update handler.
//
// The turn is reserved here, on the Update goroutine, so pasted commands keep
// their order. The closure waits for the previous command before it touches
// the shared editor. The turn ends in handleCommandResult, once Update has
// applied the result, so results are applied in the order the operator
// entered the commands. See dispatchQueue.
func (m Model) executeCommand(input string) tea.Cmd {
	wait, done := m.dispatch.reserve()
	return func() tea.Msg {
		if !m.dispatch.await(wait) {
			done()
			return nil
		}
		result, err := m.dispatchCommand(input)
		return commandResultMsg{result: result, err: err, turnDone: done}
	}
}

// dispatchCommand parses and executes a command.
// Returns commandResult with all state changes for the Update handler to apply.
func (m *Model) dispatchCommand(input string) (commandResult, error) {
	parsed, err := ParseCommandWords(input)
	if err != nil {
		return commandResult{}, err
	}
	return m.dispatchCommandWords(parsed)
}

// dispatchCommandWords retains lexical pipe positions through top redispatch.
func (m *Model) dispatchCommandWords(parsed CommandWords) (commandResult, error) {
	tokens := parsed.Tokens()
	if len(tokens) == 0 {
		return commandResult{}, nil
	}

	cmd := tokens[0]
	args := tokens[1:]

	// `top <command>` is taken before the pipe split, so `top show | errors`
	// runs `show | errors` at the root. A pipe right after `top` leaves the
	// line to the split, which refuses `top` as unpipeable as before.
	rootCommand := cmd == cmdTop && len(args) > 0 && parsed.FirstPipe() != 1
	if rootCommand {
		if m.editor == nil {
			return commandResult{}, fmt.Errorf("command %q requires config mode (no config file loaded)", cmd)
		}
		return m.cmdTop(parsed.Tail())
	}

	// Check for pipe in command
	if parsed.FirstPipe() > 0 {
		return m.dispatchWithPipe(parsed)
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
		return m.cmdTop(CommandWords{})

	case cmdUp:
		return m.cmdUp()

	case cmdEdit:
		return m.cmdEdit(args)

	case cmdShow:
		return m.cmdShow(args)

	case cmdOption:
		return m.cmdOption(args)

	case cmdCommit:
		req, err := contract.ParseCommit(args)
		if err != nil {
			return commandResult{}, err
		}
		return m.cmdCommitRequest(req)

	case cmdDiscard:
		// Session-aware discard: requires path or cmdAll when session is active.
		if m.editor.HasSession() {
			return m.cmdDiscardSession(args)
		}
		// Without a session, bare discard and `discard all` drop every change,
		// and `discard <path>` drops only the changes under that path.
		if len(args) == 0 {
			return m.cmdDiscard()
		}
		if args[0] == cmdAll {
			return m.cmdDiscard()
		}
		return m.cmdDiscardPath(args)

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

// cmdTop moves to the root, or with a command runs that command once at the
// root, whatever the command is.
//
// The command runs on the per-command copy of the Model (executeCommand), so
// emptying the copy's position changes nothing the operator sees. The real
// position moves only when the inner result sets clearContext or newContext,
// which only edit, up and top do; nothing is put back by hand. The inner
// result is answered as built, so `top show` draws the root, and an inner
// refusal is the inner command's own.
//
// The operator's position is saved on the copy first (operatorPosition), and
// only by the outermost `top`, so `top top ...` keeps the real origin.
func (m *Model) cmdTop(words CommandWords) (commandResult, error) {
	if len(words.tokens) > 0 {
		if !m.underTop {
			m.topOrigin = m.contextPath
			m.underTop = true
		}
		m.contextPath = nil
		m.isTemplate = false
		return m.dispatchCommandWords(words)
	}
	if m.editor.WorkingContent() == "" {
		return commandResult{clearContext: true, output: emptyConfiguration}, nil
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
		m.setViewportText(emptyConfiguration)
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
		return tokens, false
	}
	// Start of quoted string - save any accumulated content first
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, true
}

// quoteOpen reports whether a double-quoted value is open at the end of
// prefix, under the operational invocation grammar. The key handler asks it of
// the input up to the cursor, so an escaped literal quote does not turn ? into
// help while the operator is still inside the quoted value.
func quoteOpen(prefix string) bool {
	_, _, open, _ := command.ScanInvocation(prefix)
	return open
}

// lineBehindTop answers the line a leading `top ` runs, every leading `top`
// word dropped, and whether one was there. A line with no `top` in front
// answers itself. Leading blanks of the rest are dropped and trailing ones
// kept, so completion still reads a finished last word.
//
// handleEnter tests its intercepted verbs (run, exit, quit, stop, restart,
// clear) on this line, and completion completes it at the root. The loop
// ends because each pass removes at least one word.
func lineBehindTop(input string) (string, bool) {
	rest, under := input, false
	for len(rest) > len(cmdTop) && strings.HasPrefix(rest, cmdTop) && isCommandBlank(rest[len(cmdTop)]) {
		rest, under = strings.TrimLeft(rest[len(cmdTop):], " \t"), true
	}
	return rest, under
}

// isCommandBlank reports whether c separates words, as tokenizeCommand reads
// a line.
func isCommandBlank(c byte) bool {
	return c == ' ' || c == '\t'
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
