// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_keys.go — keyboard input handling
// Detail: model_render.go — View rendering, dropdown, message lines
// Detail: model_mode.go — editor mode switching (edit/command)
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
)

// Styles for the editor UI.
var (
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	welcomeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	ghostStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
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
	forceChanges    bool        // Force diff gutter even when changes column is disabled (used by show changes)
	lineMapping     map[int]int // Maps displayed line (1-based) to original line (1-based), nil for full content
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
	err             error
	width           int
	height          int
	quitting        bool

	// Quit confirmation state
	confirmQuit bool // True if waiting for y/n/Esc to confirm quit

	// Commit confirmed state (VyOS-style commit with auto-revert)
	confirmTimerActive bool   // True if waiting for confirm/abort
	confirmSecondsLeft int    // Countdown seconds remaining
	confirmBackupPath  string // Path to backup for rollback on timeout/abort

	// Paste mode state (for load terminal ...)
	pasteMode         bool            // True if accumulating paste input
	pasteBuffer       strings.Builder // Accumulates pasted lines
	pasteModeLocation string          // "absolute" or "relative"
	pasteModeAction   string          // "replace" or "merge"

	// Command history (browsing, entries, and persistence)
	history *History

	// Accumulating output buffer (command-only mode)
	outputBuf   strings.Builder // Scroll-back buffer for command-only mode
	lastCommand string          // Most recently dispatched command (for echo in output buffer)

	// Mode state
	mode             EditorMode                   // Current editor mode (edit or command)
	modeStates       map[EditorMode]modeState     // Saved screen state per mode
	commandCompleter CommandModeCompleter         // Completer for command mode (nil if no daemon)
	commandExecutor  func(string) (string, error) // Executes operational commands via RPC (nil if no daemon)

	// Monitor streaming state
	monitorFactory MonitorFactory  // Creates monitor sessions (nil if unavailable)
	monitorSession *MonitorSession // Active monitor session (nil when not monitoring)

	// Dashboard state (bgp monitor live peer table)
	dashboardFactory DashboardFactory // Creates dashboard sessions (nil if unavailable)
	dashboard        *dashboardState  // Active dashboard (nil when not in dashboard mode)

	// Login warnings (set by SSH session, displayed on first render)
	loginWarnings []LoginWarning

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

// Show column names used in model.go constants block above.

// Show format names for the | format pipe.
const (
	fmtTree   = "tree"
	fmtConfig = "config"
)

// Show source names for selecting which version to display.
const (
	srcSaved     = "saved"
	srcConfirmed = "confirmed"
)

// Load command keywords.
const (
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
	ti.Placeholder = "type command or Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetWidth(120)

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(20))
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	comp := NewCompleter()
	comp.SetTree(ed.Tree())

	val, err := NewConfigValidator()
	if err != nil {
		return Model{}, fmt.Errorf("failed to create validator: %w", err)
	}

	// Run initial validation against hierarchical content (matching viewport display)
	// so line numbers align with what the user sees.
	result := val.Validate(ed.ContentAtPath(nil))

	welcome := "welcome to ze!"
	if ed.session != nil && ed.session.User != "" {
		welcome = "welcome to ze, " + ed.session.User + "!"
	}

	return Model{
		editor:             ed,
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
		mode:               ModeEdit,
		modeStates:         make(map[EditorMode]modeState),
		statusMessage:      welcome,
	}, nil
}

// NewCommandModel creates a command-only model with no editor.
// Used by ze cli and plugin CLI where no config file is loaded.
// The model starts in ModeCommand with edit commands unavailable.
func NewCommandModel() Model {
	ti := textinput.New()
	ti.Placeholder = "type command or press Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetWidth(120)

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(20))
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	return Model{
		textInput:  ti,
		viewport:   vp,
		selected:   -1,
		history:    NewHistory(nil, ""),
		mode:       ModeCommand,
		modeStates: make(map[EditorMode]modeState),
	}
}

// hasEditor returns true if the model has a config editor attached.
func (m Model) hasEditor() bool {
	return m.editor != nil
}

// draftPollInterval is how often the model checks for draft changes by other sessions.
const draftPollInterval = 2 * time.Second

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.hasEditor() && m.editor.HasSession() {
		return tea.Batch(textinput.Blink, tea.Tick(draftPollInterval, func(time.Time) tea.Msg { return draftPollMsg{} }))
	}
	return textinput.Blink
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
		if m.hasEditor() && !m.showViewport && m.viewportContent == "" {
			if m.editor.HasPendingEdit() {
				if err := m.editor.LoadPendingEdit(); err == nil {
					m.statusMessage = "Restored snapshot from previous session. Use 'commit' to apply or 'discard' to revert."
					m.runValidation()
				}
			}
			m.showConfigContent()
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

	case dashboardTickMsg:
		if m.dashboard != nil {
			pollCmd := m.dashboardPollCmd()
			return m, pollCmd
		}
		return m, nil

	case dashboardDataMsg:
		return m.handleDashboardData(msg)
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
			if m.outputBuf.Len() > 0 {
				m.outputBuf.WriteString("\n")
			}
			m.outputBuf.WriteString("ze> " + m.lastCommand + "\n")
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
			m.textInput.SetValue(strings.Join(words[:len(words)-1], " ") + " ")
		}
		m.textInput.CursorEnd()
		return
	}

	input := m.textInput.Value()
	words := tokenizeCommand(input)

	if len(words) > 0 && !strings.HasSuffix(input, " ") {
		// Replace last partial word
		words[len(words)-1] = comp.Text
		m.textInput.SetValue(joinTokensWithQuotes(words) + " ")
	} else {
		// Append completion (quote and escape if needed)
		if strings.ContainsAny(comp.Text, " \t\"") {
			escaped := strings.ReplaceAll(comp.Text, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			m.textInput.SetValue(input + "\"" + escaped + "\" ")
		} else {
			m.textInput.SetValue(input + comp.Text + " ")
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

	case m.mode == ModeEdit && strings.HasPrefix(input, cmdRun+" "):
		// Edit mode with "run " prefix: delegate to command completer for operational completions.
		if m.commandCompleter != nil {
			args := input[len(cmdRun)+1:] // preserve trailing spaces
			m.completions = m.commandCompleter.Complete(args)
			m.ghostText = m.commandCompleter.GhostText(args)
		}

	case m.mode == ModeCommand && isEditCommandWithArgs(input):
		// Command mode with a full config command followed by args: YANG completions.
		if m.completer != nil {
			m.completions = m.completer.Complete(input, m.contextPath)
			m.ghostText = m.completer.GhostText(input, m.contextPath)
		}

	case m.mode == ModeCommand:
		// Command mode top-level: merge operational + config command completions.
		if m.commandCompleter != nil {
			m.completions = m.commandCompleter.Complete(input)
			m.ghostText = m.commandCompleter.GhostText(input)
		}
		if m.completer != nil {
			editComps := m.completer.Complete(input, m.contextPath)
			m.completions = append(m.completions, editComps...)
			if m.ghostText == "" {
				m.ghostText = m.completer.GhostText(input, m.contextPath)
			}
		}

	case m.mode == ModeEdit:
		// Edit mode: YANG completions.
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
		return m.editor.HasPendingSessionChanges()
	}
	return m.editor.Dirty()
}

// handleDraftPoll checks if the draft file was modified by another session.
// Editor.CheckDraftChanged handles re-read internally. Reschedules the next poll.
func (m Model) handleDraftPoll() (tea.Model, tea.Cmd) {
	if !m.hasEditor() || !m.editor.HasSession() {
		return m, nil
	}

	changed, notification := m.editor.CheckDraftChanged()
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
		_ = m.editor.SaveEditState() // Best effort — quitting anyway
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

// SetCommandCompleter sets the command mode completer.
// When set, command mode provides operational command completions.
// When nil, command mode has no completions (editor-only / standalone mode).
// Accepts any CommandModeCompleter (e.g., *CommandCompleter or *PluginCompleter).
func (m *Model) SetCommandCompleter(cc CommandModeCompleter) {
	m.commandCompleter = cc
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
	other := ModeCommand
	if m.mode == ModeCommand {
		other = ModeEdit
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

// DisableBlink disables cursor blink for headless test models.
// Sets the textinput to use a real cursor (no-op in headless mode) which
// skips all virtual cursor processing including 530ms blink timers.
func (m *Model) DisableBlink() {
	m.textInput.SetVirtualCursor(false)
}

// UpdateCompletions refreshes the completion list based on current input.
// Useful for testing to ensure completions are populated.
func (m *Model) UpdateCompletions() {
	m.updateCompletions()
}

// ApplyResult applies a commandResult to the model.
// Useful for testing to simulate what the Update handler does.
func (m *Model) ApplyResult(r commandResult) {
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
