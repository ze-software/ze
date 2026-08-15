// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_keys.go — keyboard input handling
// Detail: model_render.go — View rendering, dropdown, message lines
// Detail: model_mode.go — editor mode switching (config/operational)
// Detail: model_search.go — config search and prefix-token matching
// Detail: history.go — command history persistence to zefs
// Detail: model_dashboard.go — dashboard session lifecycle

package cli

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Styles for the editor UI.
var (
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	welcomeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("73"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	contextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Background(lipgloss.Color("236"))
	// errorLineStyle highlights lines with validation errors (red text on dark background).
	errorLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Background(lipgloss.Color("52"))
	// warningLineStyle highlights lines with validation warnings (yellow text on dark background).
	warningLineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")).
				Background(lipgloss.Color("58"))
)

// viewportData bundles content with its line mapping for display.
// This ensures content and mapping always travel together, avoiding implicit coupling.
type viewportData struct {
	content         string      // The text content to display
	originalContent string      // Original content for diff gutter
	hasOriginal     bool        // True when originalContent was explicitly set (distinguishes "not set" from "empty = new block")
	forceChanges    bool        // Force diff gutter even when changes column is disabled (used by show changes and show | compare)
	lineMapping     map[int]int // Maps displayed line (1-based) to original line (1-based), nil for full content

	// noValidationHighlight suppresses validation error/warning styling for views
	// whose lines do not correspond to the validated content.
	//
	// runValidation validates ContentAtPath(nil) -- the full config serialized at
	// root -- specifically "so that line numbers align with what the user sees"
	// (model_commands_commit.go). A pruned compare view is a DIFFERENT string, so
	// its working-line numbers do not index the validated content and
	// highlightValidationIssues would style an unrelated line. Mapping pruned lines
	// back is not reliable either: identical lines (closing braces, or the same leaf
	// value under two peers) bind to the wrong parent. Showing no marker beats
	// showing a wrong one -- `show | errors` remains the accurate view.
	noValidationHighlight bool

	// secretChanges names each secret leaf whose value moved between the two
	// sides of this view, as a dotted path. Both sides render the same
	// placeholder, so the text diff reads them as equal and marks the line
	// unchanged. setViewportData writes one line per path, and nothing else can:
	// both values are gone by the time the diff has text.
	secretChanges []string
}

// CommandModeCompleter provides completions for command mode.
// Implemented by CommandCompleter (operational commands) and PluginCompleter (plugin SDK methods).
type CommandModeCompleter interface {
	Complete(input string) []Completion
	GhostText(input string) string
}

// Model is the Bubble Tea model for the editor.
type Model struct {
	editor      *Editor
	completer   *Completer
	validator   *ConfigValidator
	textInput   textinput.Model
	viewport    viewport.Model
	contextPath []string // Current edit context (e.g., ["neighbor", "192.168.1.1"])
	isTemplate  bool     // true when editing with wildcard (*)

	// dispatch orders this session's config commands. A pointer, so the Model
	// copies Update makes share one queue -- as they share editor and
	// completer. See dispatchQueue in model_commands.go.
	dispatch *dispatchQueue

	// Completion state
	completions       []Completion
	selected          int    // Selected index in dropdown (-1 for ghost mode)
	ghostText         string // Inline ghost suggestion
	showDropdown      bool   // Whether to show dropdown
	completionHint    string // Transient description shown on second message line (clears on typing/Enter)
	completionHintDim bool   // When true, render hint in dim style (partial input); false = bright (confirmed)
	searchCache       string // Cached SetView() output for / search (invalidated on tree change)

	// Validation state
	validationErrors   []ConfigValidationError
	validationWarnings []ConfigValidationError
	validationID       int // Incremented on each text change for debounce

	// Reload errors from the last commit attempt. Shown by the errors command.
	reloadErrors []string

	// Display state
	viewportContent string // Content shown in viewport
	showViewport    bool   // Whether viewport is active (for scrolling)
	showingConfig   bool   // Whether viewport shows config (false after command output)
	showHelp        bool   // Whether help overlay is shown
	showHints       bool   // Whether inline diagnostic hints are shown (← missing: ...)
	statusMessage   string // Temporary status message (clears on next command)

	// cliFormat is this session's `set cli format` override. Empty means "use the
	// configured default" (ze.cli.format, from the environment cli format default
	// YANG leaf). Session-scoped on purpose: the Model is per-session
	// (cmd/ze/hub/session_factory.go), and one operator's display choice must not
	// change what other concurrent SSH/web sessions see. Storing it via env.Set
	// would, because env.Set writes a process-global cache and os.Setenv.
	cliFormat string

	err      error
	width    int
	height   int
	quitting bool

	// Quit confirmation state
	confirmQuit       bool // True if waiting for y/n/Esc to confirm quit
	confirmExitConfig bool // True when confirmQuit was triggered by "exit" in config mode (switch to operational, not quit)

	// Commit confirmed state (VyOS-style commit with auto-revert)
	confirmTimerActive bool   // True if waiting for confirm/abort
	confirmSecondsLeft int    // Countdown seconds remaining
	confirmBackupPath  string // Path to backup for rollback on timeout/abort

	// Paste mode state (for load terminal ...)
	pasteMode         bool             // True if accumulating paste input
	pasteBuffer       *strings.Builder // Accumulates pasted lines
	pasteModeLocation string           // "absolute" or "relative"
	pasteModeAction   string           // "replace" or "merge"

	// Command history (browsing, entries, and persistence)
	history *History

	// Accumulating output buffer (command-only mode)
	outputBuf   *strings.Builder // Scroll-back buffer for command-only mode
	lastCommand string           // Most recently dispatched command (for echo in output buffer)

	// Mode state
	mode             EditorMode                   // Current editor mode (config or operational)
	modeStates       map[EditorMode]modeState     // Saved screen state per mode
	commandCompleter CommandModeCompleter         // Completer for command mode (nil if no daemon)
	commandExecutor  func(string) (string, error) // Executes operational commands via RPC (nil if no daemon)

	// Monitor streaming state (generic monitor view; not a registered live view)
	monitorFactory MonitorFactory  // Creates monitor sessions (nil if unavailable)
	monitorSession *MonitorSession // Active monitor session (nil when not monitoring)

	// Live view registry state. The dashboard/ping/traceroute rich live views
	// register a viewSpec (view_registry.go) instead of each adding a field and
	// switch arm here. activeView is the single active full-screen view (nil
	// when none); viewFactories keys each view's injected concrete factory.
	activeView    activeView     // Active live view (nil when none)
	viewFactories map[string]any // Per-view factory, keyed by viewSpec.key

	// Login warnings (set by SSH session, displayed on first render)
	loginWarnings []LoginWarning

	// Audit context (set by SSH session for config commit/discard attribution)
	auditRecorder   audit.Recorder
	auditSurface    string
	auditUsername   string
	auditRemoteAddr string

	// Daemon lifecycle callbacks (set by SSH session for stop/restart commands)
	shutdownFunc func() // Called on "stop" in interactive CLI (no GR marker)
	restartFunc  func() // Called on "restart" in interactive CLI (writes GR marker)

	// Lifecycle confirmation state
	confirmStop    bool // True if waiting for y/n to confirm stop
	confirmRestart bool // True if waiting for y/n to confirm restart
}

// PipeFilter represents a filter in a pipe chain.
type PipeFilter struct {
	Type string // "grep", "head", "tail", "format", "compare"
	Arg  string // Pattern or count
}

// Debounce delay for validation after keystroke.
const validationDebounce = 100 * time.Millisecond

// Command names (used in multiple switch statements).
const (
	cmdSet        = "set"
	cmdShow       = "show"
	cmdOption     = "option"
	cmdDelete     = "delete"
	cmdCompare    = "compare"
	cmdEdit       = "edit"
	cmdCommit     = "commit"
	cmdConfirm    = "confirm"
	cmdConfirmed  = "confirmed"
	cmdAbort      = "abort"
	cmdDiscard    = "discard"
	cmdHistory    = "history"
	cmdRollback   = "rollback"
	cmdLoad       = "load"
	cmdSave       = "save"
	cmdErrors     = "errors"
	cmdTop        = "top"
	cmdUp         = "up"
	cmdExit       = "exit"
	cmdQuit       = "quit"
	cmdHelp       = "help"
	cmdRun        = "run"
	cmdConfigure  = "configure"
	cmdMatch      = "match"
	cmdWho        = "who"
	cmdDisconnect = "disconnect"
	cmdAll        = "all"
	cmdBlame      = "blame"
	cmdStop       = "stop"
	cmdRestart    = "restart"
	cmdChanges    = "changes"
	cmdHead       = "head"
	cmdTail       = "tail"
	cmdNone       = "none"
	cmdFormat     = "format"
	cmdEnable     = "enable"
	cmdDisable    = "disable"
	cmdActivate   = "activate"
	cmdDeactivate = "deactivate"
	cmdActive     = "active"
	cmdInactive   = "inactive"
	cmdRename     = "rename"
	cmdCopy       = "copy"
	cmdInsert     = "insert"
)

// Show column names used as DB keys under /meta/show/<column>.
const (
	colAuthor  = "author"
	colDate    = "date"
	colSource  = "source"
	colChanges = "changes"
)

// Show format and source names for pipes and version display.
const (
	fmtTree   = "tree"
	fmtConfig = "config"
	FmtTree   = fmtTree
	FmtConfig = fmtConfig

	srcSaved     = "saved"
	srcConfirmed = "confirmed"
	SrcSaved     = srcSaved
	SrcConfirmed = srcConfirmed
	CmpRollback  = cmdRollback

	loadLocationAbsolute = "absolute"
	loadLocationRelative = "relative"
	loadActionReplace    = "replace"
	loadActionMerge      = "merge"
)

// Key string constants for v2 bubbletea key matching.
const (
	keyCtrlC = "ctrl+c"
	keyEsc   = "esc"
)

const footerQuitHint = "q/Esc Quit"

// Status messages for unavailable daemon operations.
const (
	msgStopNotAvailable    = "stop not available (not connected to daemon)"
	msgRestartNotAvailable = "restart not available (not connected to daemon)"
)

// commandResult carries state changes from a command back to Update.
// This allows commands to run in a tea.Cmd closure without losing state changes.
type commandResult struct {
	output        string        // Text to display in viewport (non-config content)
	configView    *viewportData // Config content to display with line mapping
	refreshConfig bool          // Recompute config view from editor state (use when original baseline changed)
	statusMessage string        // Temporary status message (shown above viewport, clears on next command)
	newContext    []string      // New context path (nil = no change)
	clearContext  bool          // True to clear context to root
	isTemplate    bool          // Template mode flag (used with newContext)
	showHelp      bool          // Show help overlay
	revalidate    bool          // Trigger re-validation after command

	// Commit confirmed state (must be propagated through result, not set directly on model)
	setConfirmTimer       bool   // True to set confirmTimerActive
	confirmTimerValue     bool   // Value to set confirmTimerActive to
	confirmBackupPath     string // Backup path for rollback (empty to clear)
	startConfirmCountdown int    // Seconds for countdown timer (0 = no countdown)

	// Paste mode state (for load terminal ...)
	enterPasteMode    bool   // True to enter paste mode
	pasteModeLocation string // "absolute" or "relative"
	pasteModeAction   string // "replace" or "merge"
}

// Message types for the editor.
type (
	// commandResultMsg carries command results back to Update for application.
	commandResultMsg struct {
		result commandResult
		err    error
	}
	contextChangedMsg struct{}
	successMsg        struct{}
	errorMsg          struct{ err error }
	outputMsg         struct{ text string }

	// validationTickMsg triggers debounced validation.
	// The id field is used to ignore stale ticks.
	validationTickMsg struct{ id int }

	// confirmCountdownMsg fires every second during a "commit confirmed" window.
	confirmCountdownMsg struct{}

	// draftPollMsg fires every 2 seconds to check if another session modified the draft.
	draftPollMsg struct{}
)

// NewModel creates a new editor model.
func NewModel(ed *Editor) (Model, error) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "type command or Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetWidth(120)
	ti.ShowSuggestions = true
	ti.SetVirtualCursor(false)

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(20))
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	comp := NewCompleter()
	comp.SetTree(ed.Tree())

	val, err := newConfigValidator()
	if err != nil {
		return Model{}, fmt.Errorf("failed to create validator: %w", err)
	}

	// Run initial validation against hierarchical content (matching viewport display)
	// so line numbers align with what the user sees.
	result := val.Validate(ed.ContentAtPath(nil))

	welcome := "welcome to ze!"
	if ed.session != nil && ed.session.User != "" {
		var tb textbuf.Buffer
		welcome = tb.Str("welcome to ze, ").Str(ed.session.User).Byte('!').String()
	}

	return Model{
		editor:             ed,
		dispatch:           newDispatchQueue(),
		completer:          comp,
		validator:          val,
		textInput:          ti,
		viewport:           vp,
		contextPath:        nil,
		selected:           -1,
		history:            NewHistory(nil, ""),
		validationErrors:   result.Errors,
		validationWarnings: result.Warnings,
		showHints:          true,
		mode:               ModeConfig,
		modeStates:         make(map[EditorMode]modeState),
		statusMessage:      welcome,
		pasteBuffer:        &strings.Builder{},
		outputBuf:          &strings.Builder{},
		viewFactories:      make(map[string]any),
	}, nil
}

// NewCommandModel creates a command-only model with no editor.
// Used by ze cli and plugin CLI where no config file is loaded.
// The model starts in ModeOperational with config commands unavailable.
func NewCommandModel() Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "type command or press Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetWidth(120)
	ti.ShowSuggestions = true
	ti.SetVirtualCursor(false)

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(20))
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	return Model{
		textInput:     ti,
		viewport:      vp,
		dispatch:      newDispatchQueue(),
		selected:      -1,
		history:       NewHistory(nil, ""),
		mode:          ModeOperational,
		modeStates:    make(map[EditorMode]modeState),
		statusMessage: "welcome to ze!",
		pasteBuffer:   &strings.Builder{},
		outputBuf:     &strings.Builder{},
		viewFactories: make(map[string]any),
	}
}

// hasEditor returns true if the model has a config editor attached.
func (m Model) hasEditor() bool {
	return m.editor != nil
}

// writeCommandEcho appends "ze> <command>\n" to the scroll-back buffer,
// separated from prior output by a blank line.
// Called once per command dispatch so individual handlers do not repeat it.
func (m *Model) writeCommandEcho() {
	if m.hasEditor() {
		return
	}
	if m.outputBuf.Len() > 0 {
		buf := m.outputBuf.String()
		switch {
		case strings.HasSuffix(buf, "\n\n"):
			// already blank line
		case strings.HasSuffix(buf, "\n"):
			m.outputBuf.WriteString("\n")
		default:
			m.outputBuf.WriteString("\n\n")
		}
	}
	var tb textbuf.Buffer
	m.outputBuf.WriteString(tb.Str("ze> ").Str(m.lastCommand).Byte('\n').Slice())
}

// draftPollInterval is how often the model checks for draft changes by other sessions.
const draftPollInterval = 2 * time.Second

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.hasEditor() && m.editor.HasSession() {
		return tea.Tick(draftPollInterval, func(time.Time) tea.Msg { return draftPollMsg{} })
	}
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.SetWidth(msg.Width - 4)
		// Resize viewport
		m.viewport.SetWidth(msg.Width - 4)
		m.viewport.SetHeight(max(msg.Height-10, 5))
		// Show config on first size event (startup)
		if !m.showViewport && m.viewportContent == "" {
			if m.hasEditor() {
				if m.editor.HasPendingEdit() {
					if err := m.editor.LoadPendingEdit(); err == nil {
						m.statusMessage = "Restored snapshot from previous session. Use 'commit' to apply or 'discard' to revert."
						m.runValidation()
					}
				}
				m.showConfigContent()
			} else {
				m.showViewport = true
			}
		}
		return m, nil

	case commandResultMsg:
		return m.handleCommandResult(msg)

	case successMsg:
		m.err = nil
		return m, nil

	case errorMsg:
		m.err = msg.err
		return m, nil

	case outputMsg:
		m.setViewportText(msg.text)
		return m, nil

	case contextChangedMsg:
		m.updateCompletions()
		return m, nil

	case validationTickMsg:
		// Only validate if this tick matches current ID (not stale)
		if msg.id == m.validationID {
			m.runValidation()
		}
		return m, nil

	case confirmCountdownMsg:
		return m.handleConfirmCountdown()

	case draftPollMsg:
		return m.handleDraftPoll()

	case monitorPollMsg:
		return m.handleMonitorPoll()

	case viewMsg:
		// Route every live-view tick/data message to the single active view.
		// One arm replaces the former per-feature dashboard/ping/traceroute cases.
		// A stale tick after the view stopped (activeView nil) is a no-op.
		if m.activeView != nil {
			return m.activeView.update(&m, msg)
		}
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleCommandResult applies the result of an executed command to the model.
func (m Model) handleCommandResult(msg commandResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.statusMessage = "" // Clear status on error
		return m, nil
	}
	r := msg.result

	// Apply context changes
	if r.clearContext {
		m.contextPath = nil
		m.isTemplate = false
	} else if r.newContext != nil {
		m.contextPath = r.newContext
		m.isTemplate = r.isTemplate
	}

	// Run validation before viewport update so highlightValidationIssues uses fresh errors.
	if r.revalidate {
		m.runValidation()
	}

	// Apply viewport changes
	switch {
	case r.refreshConfig && m.hasEditor():
		m.setViewportData(*m.configViewAtPath(m.contextPath))
	case r.configView != nil:
		m.setViewportData(*r.configView)
	case r.output != "":
		if !m.hasEditor() {
			// Command-only mode: accumulate output in scroll-back buffer.
			m.outputBuf.WriteString(r.output)
			m.setViewportText(m.outputBuf.String())
			m.viewport.GotoBottom()
		} else {
			m.setViewportText(r.output)
		}
	}

	// Status message (temporary notification)
	m.statusMessage = r.statusMessage

	// Other state
	if r.showHelp {
		m.showHelp = true
	}

	// Apply confirm timer state (must be propagated through result)
	if r.setConfirmTimer {
		m.confirmTimerActive = r.confirmTimerValue
		m.confirmBackupPath = r.confirmBackupPath
	}

	// Start countdown timer if requested
	if r.startConfirmCountdown > 0 {
		m.confirmSecondsLeft = r.startConfirmCountdown
		return m, tea.Tick(time.Second, func(_ time.Time) tea.Msg {
			return confirmCountdownMsg{}
		})
	}

	// Apply paste mode state
	if r.enterPasteMode {
		m.pasteMode = true
		m.pasteBuffer.Reset()
		m.pasteModeLocation = r.pasteModeLocation
		m.pasteModeAction = r.pasteModeAction
	}

	m.err = nil
	return m, nil
}

// applyCompletion applies a completion to the input.
func (m *Model) applyCompletion(comp Completion) {
	// Search results: replace input with the set command minus its last word (the value).
	if comp.Type == "search" {
		words := strings.Fields(comp.Text)
		if len(words) > 1 {
			var tb textbuf.Buffer
			m.textInput.SetValue(tb.Join(words[:len(words)-1], " ").Byte(' ').String())
		}
		m.textInput.CursorEnd()
		return
	}

	input := m.textInput.Value()
	words := tokenizeCommand(input)

	var tb textbuf.Buffer
	if len(words) > 0 && !strings.HasSuffix(input, " ") {
		words[len(words)-1] = comp.Text
		m.textInput.SetValue(tb.Str(joinTokensWithQuotes(words)).Byte(' ').String())
	} else {
		if strings.ContainsAny(comp.Text, " \t\"") {
			m.textInput.SetValue(tb.Str(input).Byte('"').Str(comp.Text).Str("\" ").String())
		} else {
			m.textInput.SetValue(tb.Reset().Str(input).Str(comp.Text).Byte(' ').String())
		}
	}
	m.textInput.CursorEnd()
}

// updateCompletions updates completions based on current input.
// Cross-mode completions:
//   - Edit mode with "run " prefix → operational command completions
//   - Command mode with config command prefix → YANG completions
//   - Command mode top-level → merge operational + config command completions
func (m *Model) updateCompletions() {
	input := m.textInput.Value()

	switch {
	case strings.HasPrefix(input, "/"):
		// Search mode: /prefix tokens filter config set-commands.
		m.completions = m.searchConfig(input[1:])
		m.ghostText = ""

	case m.mode == ModeConfig && strings.HasPrefix(input, cmdRun+" "):
		// Config mode with "run " prefix: delegate to command completer for operational completions.
		if m.commandCompleter != nil {
			args := input[len(cmdRun)+1:] // preserve trailing spaces
			m.completions = m.commandCompleter.Complete(args)
			m.ghostText = m.commandCompleter.GhostText(args)
		}

	case m.mode == ModeOperational && isConfigCommandWithArgs(input) && m.completer != nil:
		// Operational mode with a full config command followed by args: YANG completions.
		m.completions = m.completer.Complete(input, m.contextPath)
		m.ghostText = m.completer.GhostText(input, m.contextPath)

	case m.mode == ModeOperational:
		// Operational mode top-level: merge operational + config command completions.
		if m.commandCompleter != nil {
			m.completions = m.commandCompleter.Complete(input)
			m.ghostText = m.commandCompleter.GhostText(input)
		}
		m.completions = appendCLIFormatCompletions(m.completions, input)
		if m.hasEditor() && (input == "" || strings.HasPrefix(cmdConfigure, input)) {
			m.completions = append(m.completions, Completion{
				Text: cmdConfigure, Description: "Enter config mode", Type: "command",
			})
			if m.ghostText == "" && input != "" && strings.HasPrefix(cmdConfigure, input) {
				m.ghostText = cmdConfigure[len(input):]
			}
		}
		if m.completer != nil {
			configComps := m.completer.Complete(input, m.contextPath)
			m.completions = appendNewCompletions(m.completions, configComps)
			if m.ghostText == "" {
				m.ghostText = m.completer.GhostText(input, m.contextPath)
			}
		}

	case m.mode == ModeConfig:
		// Config mode: YANG completions.
		if m.completer != nil {
			m.completions = m.completer.Complete(input, m.contextPath)
			m.ghostText = m.completer.GhostText(input, m.contextPath)
		}
	}

	// Filter session-dependent commands (who, disconnect, blame, changes) when no session is active.
	if m.editor != nil && !m.editor.HasSession() {
		m.completions = filterOutSessionCommands(m.completions)
	}

	// Reset dropdown state when input changes
	if !m.showDropdown {
		m.selected = -1
	}

	// Hide dropdown if no completions or single match
	if len(m.completions) <= 1 {
		m.showDropdown = false
		m.completionHint = ""
		m.completionHintDim = false
		m.selected = -1
	}

	// Surface validation completions on line 2 (e.g., invalid list key).
	// "warning" = dim (still typing), "error" = bright (value confirmed with space).
	// Done after dropdown hide so the hint isn't cleared.
	if len(m.completions) == 1 {
		switch m.completions[0].Type {
		case "warning":
			m.completionHint = m.completions[0].Description
			m.completionHintDim = true
			m.completions = nil
		case "error":
			m.completionHint = m.completions[0].Description
			m.completionHintDim = false
			m.completions = nil
		}
	}

	m.syncGhostSuggestions()
}

// syncGhostSuggestions feeds the current ghost text into the textinput's
// native suggestion system so the inline hint is rendered by textinput.View()
// with correct cursor and padding handling.
func (m *Model) syncGhostSuggestions() {
	if m.ghostText == "" || m.showDropdown {
		m.textInput.SetSuggestions(nil)
		return
	}
	m.textInput.SetSuggestions([]string{m.textInput.Value() + m.ghostText})
}

// --- Public Accessor Methods for Testing ---

// InputValue returns the current text input value.
func (m Model) InputValue() string {
	return m.textInput.Value()
}

// ContextPath returns the current context path.
func (m Model) ContextPath() []string {
	return m.contextPath
}

// Completions returns the current completion list.
func (m Model) Completions() []Completion {
	return m.completions
}

// GhostText returns the current ghost text suggestion.
func (m Model) GhostText() string {
	return m.ghostText
}

// ValidationErrors returns the current validation errors.
func (m Model) ValidationErrors() []ConfigValidationError {
	return m.validationErrors
}

// ValidationWarnings returns the current validation warnings.
func (m Model) ValidationWarnings() []ConfigValidationError {
	return m.validationWarnings
}

// Dirty returns true if there are unsaved changes.
func (m Model) Dirty() bool {
	return m.hasEditor() && m.editor.Dirty()
}

// hasPendingChanges returns true if the editor has pending changes,
// using session-aware detection when a session is active.
func (m Model) hasPendingChanges() bool {
	if !m.hasEditor() {
		return false
	}
	if m.editor.HasSession() {
		return m.editor.hasPendingSessionChanges()
	}
	return m.editor.Dirty()
}

// handleDraftPoll checks if the draft file was modified by another session.
// Editor.CheckDraftChanged handles re-read internally. Reschedules the next poll.
func (m Model) handleDraftPoll() (tea.Model, tea.Cmd) {
	if !m.hasEditor() || !m.editor.HasSession() {
		return m, nil
	}

	changed, notification := m.editor.checkDraftChanged()
	if changed {
		m.statusMessage = notification
		m.showConfigContent()
	}

	// Reschedule next poll.
	return m, tea.Tick(draftPollInterval, func(time.Time) tea.Msg { return draftPollMsg{} })
}

// autoSaveOnQuit saves a .edit snapshot when force-quitting with unsaved changes.
// In session mode, write-through already persists to .draft, so no snapshot needed.
func (m *Model) autoSaveOnQuit() {
	if m.hasEditor() && !m.editor.HasSession() && m.editor.Dirty() {
		_ = m.editor.saveEditState() // Best effort — quitting anyway
	}
}

// StatusMessage returns the current status message.
func (m Model) StatusMessage() string {
	return m.statusMessage
}

// Error returns the current command error.
func (m Model) Error() error {
	return m.err
}

// IsTemplate returns true if in template editing mode.
func (m Model) IsTemplate() bool {
	return m.isTemplate
}

// ShowDropdown returns true if the completion dropdown is visible.
func (m Model) ShowDropdown() bool {
	return m.showDropdown
}

// SelectedIndex returns the currently selected dropdown index.
func (m Model) SelectedIndex() int {
	return m.selected
}

// ConfirmTimerActive returns true if a commit confirm timer is active.
func (m Model) ConfirmTimerActive() bool {
	return m.confirmTimerActive
}

// ViewportContent returns the content currently displayed in the viewport.
func (m Model) ViewportContent() string {
	return m.viewportContent
}

// refreshCompleter updates the config completer tree and propagates
// derived backend names to the command completer.
func (m *Model) refreshCompleter() {
	m.completer.SetTree(m.editor.Tree())
	if cc, ok := m.commandCompleter.(*CommandCompleter); ok {
		cc.SetActiveBackends(m.completer.Backends())
	}
}

// SetCommandCompleter sets the command mode completer.
// When set, command mode provides operational command completions.
// When nil, command mode has no completions (editor-only / standalone mode).
// Accepts any CommandModeCompleter (e.g., *CommandCompleter or *PluginCompleter).
func (m *Model) SetCommandCompleter(cc CommandModeCompleter) {
	m.commandCompleter = cc
	if tc, ok := cc.(*CommandCompleter); ok {
		tc.SetActiveBackends(m.completer.Backends())
	}
}

// SetCommandExecutor sets the function used to execute operational commands in command mode.
// The function receives a command string and returns the output or an error.
// When nil, command mode shows an error on Enter.
func (m *Model) SetCommandExecutor(fn func(string) (string, error)) {
	m.commandExecutor = fn
}

// SetLoginWarnings sets the login warnings to display in the welcome area.
// Called by the SSH session after collecting warnings from the daemon.
func (m *Model) SetLoginWarnings(warnings []LoginWarning) {
	m.loginWarnings = warnings
}

// SetAuditRecorder sets the audit sink and caller metadata for config mutations.
func (m *Model) SetAuditRecorder(recorder audit.Recorder, surface, username, remoteAddr string) {
	m.auditRecorder = recorder
	m.auditSurface = surface
	m.auditUsername = username
	m.auditRemoteAddr = remoteAddr
}

func (m *Model) recordConfigCommit(detail string) {
	m.recordAudit(audit.ActionConfigCommit, detail)
}

func (m *Model) recordConfigDiscard(detail string) {
	m.recordAudit(audit.ActionConfigDiscard, detail)
}

func (m *Model) recordAudit(action, detail string) {
	if m.auditRecorder == nil {
		return
	}
	actor := m.auditUsername
	if actor == "" && m.editor != nil && m.editor.session != nil {
		actor = m.editor.session.User
	}
	surface := m.auditSurface
	if surface == "" {
		surface = audit.CLI
	}
	_ = m.auditRecorder.Record(audit.Entry{
		Actor:      actor,
		RemoteAddr: m.auditRemoteAddr,
		Surface:    surface,
		Action:     action,
		Detail:     detail,
		Outcome:    audit.OutcomeSuccess,
	})
}

// SetShutdownFunc sets the callback for the "stop" interactive CLI command.
// When set, typing "stop" prompts for confirmation, then calls fn and quits.
func (m *Model) SetShutdownFunc(fn func()) {
	m.shutdownFunc = fn
}

// SetRestartFunc sets the callback for the "restart" interactive CLI command.
// When set, typing "restart" prompts for confirmation, then calls fn and quits.
func (m *Model) SetRestartFunc(fn func()) {
	m.restartFunc = fn
}

// SetHistory replaces the model's history with a persistent History.
// Loads saved entries for the current mode and pre-loads the other mode
// into modeStates so history is available on mode switch.
func (m *Model) SetHistory(h *History) {
	m.history = h
	// Load history for the current mode.
	if loaded := h.Load(m.mode.String()); len(loaded) > 0 {
		m.history.entries = loaded
	}
	// Pre-load history for the other mode into modeStates so it's
	// available when the user switches modes.
	other := ModeOperational
	if m.mode == ModeOperational {
		other = ModeConfig
	}
	if loaded := h.Load(other.String()); len(loaded) > 0 {
		saved := m.modeStates[other]
		saved.histSnap = historySnapshot{entries: loaded, idx: -1}
		m.modeStates[other] = saved
	}
}

// SetInput sets the text input value. Used by external packages (e.g. SSH)
// that cannot access the unexported textInput field directly.
func (m *Model) SetInput(value string) {
	m.textInput.SetValue(value)
}

// UpdateCompletions refreshes the completion list based on current input.
// Useful for testing to ensure completions are populated.
func (m *Model) UpdateCompletions() {
	m.updateCompletions()
}

// applyResult applies a commandResult to the model.
// Useful for testing to simulate what the Update handler does.
func (m *Model) applyResult(r commandResult) {
	if r.clearContext {
		m.contextPath = nil
		m.isTemplate = false
	} else if r.newContext != nil {
		m.contextPath = r.newContext
		m.isTemplate = r.isTemplate
	}
	if r.revalidate {
		m.runValidation()
	}
	switch {
	case r.refreshConfig && m.hasEditor():
		m.setViewportData(*m.configViewAtPath(m.contextPath))
	case r.configView != nil:
		m.setViewportData(*r.configView)
	case r.output != "":
		m.setViewportText(r.output)
	}
	m.statusMessage = r.statusMessage
	if r.showHelp {
		m.showHelp = true
	}
	if r.setConfirmTimer {
		m.confirmTimerActive = r.confirmTimerValue
		m.confirmBackupPath = r.confirmBackupPath
	}
	if r.startConfirmCountdown > 0 {
		m.confirmSecondsLeft = r.startConfirmCountdown
	}
}
