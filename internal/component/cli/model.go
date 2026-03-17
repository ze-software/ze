// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: model_mode.go — editor mode switching (edit/command)

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for the editor UI.
var (
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	ghostStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
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
	completions  []Completion
	selected     int    // Selected index in dropdown (-1 for ghost mode)
	ghostText    string // Inline ghost suggestion
	showDropdown bool   // Whether to show dropdown

	// Validation state
	validationErrors   []ConfigValidationError
	validationWarnings []ConfigValidationError
	validationID       int // Incremented on each text change for debounce

	// Display state
	viewportContent string // Content shown in viewport
	showViewport    bool   // Whether viewport is active (for scrolling)
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

	// Command history
	history    []string // Previous commands (oldest first)
	historyIdx int      // Current position in history (-1 = not browsing)
	historyTmp string   // Saved current input when browsing history

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
}

// PipeFilter represents a filter in a pipe chain.
type PipeFilter struct {
	Type string // "grep", "head", "tail"
	Arg  string // Pattern or count
}

// Debounce delay for validation after keystroke.
const validationDebounce = 100 * time.Millisecond

// Command names (used in multiple switch statements).
const (
	cmdSet        = "set"
	cmdShow       = "show"
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
	cmdGrep       = "grep"
	cmdWho        = "who"
	cmdDisconnect = "disconnect"
	cmdAll        = "all"
	cmdBlame      = "blame"
	cmdChanges    = "changes"
	cmdHead       = "head"
	cmdTail       = "tail"
)

// Load command keywords.
const (
	loadLocationAbsolute = "absolute"
	loadLocationRelative = "relative"
	loadActionReplace    = "replace"
	loadActionMerge      = "merge"
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
	ti.Width = 120

	vp := viewport.New(120, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	comp := NewCompleter()
	comp.SetTree(ed.Tree())

	val, err := NewConfigValidator()
	if err != nil {
		return Model{}, fmt.Errorf("failed to create validator: %w", err)
	}

	// Run initial validation
	result := val.Validate(ed.WorkingContent())

	return Model{
		editor:             ed,
		completer:          comp,
		validator:          val,
		textInput:          ti,
		viewport:           vp,
		contextPath:        nil,
		selected:           -1,
		historyIdx:         -1,
		validationErrors:   result.Errors,
		validationWarnings: result.Warnings,
		showHints:          true,
		mode:               ModeEdit,
		modeStates:         make(map[EditorMode]modeState),
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
	ti.Width = 120

	vp := viewport.New(120, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	return Model{
		textInput:  ti,
		viewport:   vp,
		selected:   -1,
		mode:       ModeCommand,
		modeStates: make(map[EditorMode]modeState),
		historyIdx: -1,
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
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 4
		// Resize viewport
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = max(msg.Height-10, 5)
		// Show config on first size event (startup)
		if m.hasEditor() && !m.showViewport && m.viewportContent == "" {
			if m.editor.HasPendingEdit() {
				if err := m.editor.LoadPendingEdit(); err == nil {
					m.statusMessage = "Restored snapshot from previous session. Use 'commit' to apply or 'discard' to revert."
					m.runValidation()
				}
			}
			if m.editor.WorkingContent() != "" {
				m.showConfigContent()
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
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleKeyMsg dispatches keyboard input to the appropriate handler.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Quit confirmation takes highest priority
	if m.confirmQuit {
		switch msg.Type { //nolint:exhaustive // only handle specific keys
		case tea.KeyEsc, tea.KeyCtrlC:
			// Second Escape or Ctrl-C confirms quit — auto-save snapshot
			m.autoSaveOnQuit()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
				m.autoSaveOnQuit()
				m.quitting = true
				return m, tea.Quit
			}
		}
		// Any other key cancels quit
		m.confirmQuit = false
		m.statusMessage = ""
		return m, nil
	}

	// Dropdown navigation takes priority
	if m.showDropdown && len(m.completions) > 0 {
		switch msg.Type { //nolint:exhaustive // only handle specific keys
		case tea.KeyUp:
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.completions) - 1
			}
			return m, nil
		case tea.KeyDown:
			m.selected = (m.selected + 1) % len(m.completions)
			return m, nil
		case tea.KeyEsc:
			m.showDropdown = false
			m.selected = -1
			return m, nil
		case tea.KeyEnter:
			return m.handleEnter()
		case tea.KeyTab:
			return m.handleTab()
		case tea.KeyShiftTab:
			return m.handleShiftTab()
		}
	}

	// Handle help overlay
	if m.showHelp {
		switch msg.Type { //nolint:exhaustive // only handle specific keys
		case tea.KeyEsc, tea.KeyCtrlC:
			m.showHelp = false
			return m, nil
		}
		return m, nil // Ignore other keys when help is shown
	}

	// Handle paste mode (for load terminal ...)
	if m.pasteMode {
		return m.handlePasteModeKey(msg)
	}

	// Handle viewport scrolling with Shift+Arrow and PgUp/PgDown (when no dropdown)
	if m.showViewport && !m.showDropdown {
		switch msg.Type { //nolint:exhaustive // only handle specific keys
		case tea.KeyShiftUp:
			m.viewport.ScrollUp(1)
			return m, nil
		case tea.KeyShiftDown:
			m.viewport.ScrollDown(1)
			return m, nil
		case tea.KeyPgUp, tea.KeyCtrlUp:
			m.viewport.PageUp()
			return m, nil
		case tea.KeyPgDown, tea.KeyCtrlDown:
			m.viewport.PageDown()
			return m, nil
		}
	}

	// Handle command history with Up/Down arrows
	switch msg.Type { //nolint:exhaustive // only handle specific keys
	case tea.KeyUp:
		return m.handleHistoryUp(), nil
	case tea.KeyDown:
		return m.handleHistoryDown(), nil
	}

	switch msg.Type { //nolint:exhaustive // only handle specific keys
	case tea.KeyCtrlC, tea.KeyEsc:
		// Stop active monitor session before considering quit.
		if m.monitorSession != nil {
			m.stopMonitorSession()
			return m, nil
		}
		if m.hasEditor() && m.hasPendingChanges() {
			m.confirmQuit = true
			m.statusMessage = "Pending changes. Use 'commit', 'discard all', or Esc to force quit."
			return m, nil
		}
		m.confirmQuit = true
		m.statusMessage = "Quit? (y/Esc to confirm, any other key to cancel)"
		return m, nil

	case tea.KeyTab:
		return m.handleTab()

	case tea.KeyShiftTab:
		return m.handleShiftTab()

	case tea.KeyRunes:
		// Check for ? to trigger completion like Tab
		if len(msg.Runes) == 1 && msg.Runes[0] == '?' {
			return m.handleTab()
		}
		// Otherwise pass to text input
		m.textInput, cmd = m.textInput.Update(msg)
		m.updateCompletions()
		return m, tea.Batch(cmd, m.scheduleValidation())

	case tea.KeyEnter:
		return m.handleEnter()
	}

	// All other key types: forward to text input for processing
	m.textInput, cmd = m.textInput.Update(msg)
	m.updateCompletions()
	return m, tea.Batch(cmd, m.scheduleValidation())
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

// handleTab handles Tab key press.
func (m Model) handleTab() (tea.Model, tea.Cmd) {
	// Ensure completions are populated
	if len(m.completions) == 0 {
		m.updateCompletions()
	}

	if m.ghostText != "" && !m.showDropdown {
		// Accept ghost text (common prefix of multiple matches, or single match)
		if len(m.completions) > 1 {
			// Multiple matches: apply common prefix without trailing space, show dropdown
			m.textInput.SetValue(m.textInput.Value() + m.ghostText)
			m.textInput.CursorEnd()
			m.updateCompletions()
			if len(m.completions) > 1 {
				m.showDropdown = true
				m.selected = 0
			}
		} else {
			// Single match: apply full completion with trailing space
			m.textInput.SetValue(m.textInput.Value() + m.ghostText + " ")
			m.textInput.CursorEnd()
			m.updateCompletions()
		}
		return m, nil
	}

	if m.showDropdown && len(m.completions) > 0 {
		// Cycle through dropdown
		m.selected = (m.selected + 1) % len(m.completions)
		return m, nil
	}

	if len(m.completions) > 1 {
		// Show dropdown on Tab when multiple matches
		m.showDropdown = true
		m.selected = 0
		return m, nil
	}

	if len(m.completions) == 1 {
		// Skip hint-only completions (e.g., <value>, <string>) — display-only, not applicable
		if m.completions[0].Type == "hint" {
			return m, nil
		}
		// Single completion: apply it and advance
		m.applyCompletion(m.completions[0])
		m.updateCompletions()
		// Auto-show dropdown if applying the completion reveals next-level options
		if len(m.completions) > 1 {
			m.showDropdown = true
			m.selected = 0
		}
		return m, nil
	}

	return m, nil
}

// handleShiftTab handles Shift+Tab key press.
func (m Model) handleShiftTab() (tea.Model, tea.Cmd) {
	if m.showDropdown && len(m.completions) > 0 {
		m.selected--
		if m.selected < 0 {
			m.selected = len(m.completions) - 1
		}
	}
	return m, nil
}

// handleEnter handles Enter key press.
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	// If dropdown is showing, apply selected completion
	if m.showDropdown && m.selected >= 0 && m.selected < len(m.completions) {
		m.applyCompletion(m.completions[m.selected])
		m.showDropdown = false
		m.selected = -1
		m.updateCompletions()
		return m, nil
	}

	input := strings.TrimSpace(m.textInput.Value())
	if input == "" {
		return m, nil
	}

	// Handle mode switching commands.
	// "run" (bare) in edit mode → switch to command mode.
	// "run <args>" in edit mode → switch to command mode and execute.
	// "edit" (bare) in command mode → switch to edit mode.
	// Config commands (set, delete, etc.) in command mode → switch to edit mode and execute.
	if m.mode == ModeEdit && (input == cmdRun || strings.HasPrefix(input, cmdRun+" ")) {
		args := strings.TrimSpace(strings.TrimPrefix(input, cmdRun))
		m.textInput.SetValue("")
		m.SwitchMode(ModeCommand)
		m.updateCompletions()
		if args == "" {
			return m, nil
		}
		// Save to history and execute.
		if len(m.history) == 0 || m.history[len(m.history)-1] != args {
			m.history = append(m.history, args)
		}
		m.historyIdx = -1
		m.historyTmp = ""
		m.showDropdown = false
		m.selected = -1
		m.ghostText = ""
		m.completions = nil
		if m.monitorFactory != nil && isMonitorCommand(args) {
			cmd := m.startMonitorSession(extractMonitorCmdArgs(args))
			return m, cmd
		}
		return m, m.executeOperationalCommand(args)
	}
	if m.mode == ModeCommand && input == cmdEdit {
		if !m.hasEditor() {
			m.textInput.SetValue("")
			m.statusMessage = "edit mode not available (no config file loaded)"
			return m, nil
		}
		m.textInput.SetValue("")
		m.SwitchMode(ModeEdit)
		m.updateCompletions()
		return m, nil
	}
	if m.mode == ModeCommand && isEditCommand(input) {
		if !m.hasEditor() {
			m.textInput.SetValue("")
			m.statusMessage = "edit mode not available (no config file loaded)"
			return m, nil
		}
		m.SwitchMode(ModeEdit)
		// Fall through to normal dispatch — history/clear happens below,
		// executeCommand runs with the switched mode.
	}

	// Handle exit/quit directly (not via async command dispatch)
	if input == cmdExit || input == cmdQuit {
		if m.hasPendingChanges() {
			m.textInput.SetValue("")
			m.statusMessage = "Pending changes. Use 'commit', 'discard all', or Esc to force quit."
			m.confirmQuit = true
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
	}

	// Save to history
	if len(m.history) == 0 || m.history[len(m.history)-1] != input {
		m.history = append(m.history, input)
	}
	m.historyIdx = -1
	m.historyTmp = ""

	// Clear input
	m.textInput.SetValue("")
	m.showDropdown = false
	m.selected = -1
	m.ghostText = ""
	m.completions = nil

	// Execute command — dispatch based on mode
	if m.mode == ModeCommand {
		m.lastCommand = input
		if m.monitorFactory != nil && isMonitorCommand(input) {
			cmd := m.startMonitorSession(extractMonitorCmdArgs(input))
			return m, cmd
		}
		return m, m.executeOperationalCommand(input)
	}
	return m, m.executeCommand(input)
}

// handlePasteModeKey handles key input during paste mode.
// Ctrl-D ends paste mode and processes the buffer.
// Enter adds a newline to the buffer.
// Other characters are accumulated.
func (m Model) handlePasteModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type { //nolint:exhaustive // only specific keys handled in paste mode
	case tea.KeyCtrlD:
		// End paste mode and process buffer
		return m.finishPasteMode()

	case tea.KeyCtrlC, tea.KeyEsc:
		// Cancel paste mode
		m.pasteMode = false
		m.pasteBuffer.Reset()
		m.statusMessage = "Paste mode canceled"
		return m, nil

	case tea.KeyEnter:
		// Add newline to buffer
		m.pasteBuffer.WriteString("\n")
		return m, nil

	case tea.KeyRunes:
		// Accumulate characters
		for _, r := range msg.Runes {
			m.pasteBuffer.WriteRune(r)
		}
		return m, nil

	case tea.KeySpace:
		m.pasteBuffer.WriteString(" ")
		return m, nil

	case tea.KeyBackspace:
		// Remove last character from buffer
		s := m.pasteBuffer.String()
		if s != "" {
			m.pasteBuffer.Reset()
			m.pasteBuffer.WriteString(s[:len(s)-1])
		}
		return m, nil
	}

	// Keyboard input: unhandled keys are intentionally ignored (no action needed)
	return m, nil
}

// finishPasteMode ends paste mode and applies the buffered content.
func (m Model) finishPasteMode() (tea.Model, tea.Cmd) {
	content := m.pasteBuffer.String()
	m.pasteMode = false
	m.pasteBuffer.Reset()

	if strings.TrimSpace(content) == "" {
		m.statusMessage = "Paste mode: no content to apply"
		return m, nil
	}

	// Apply content based on location and action
	var result commandResult
	var err error

	if m.pasteModeLocation == loadLocationAbsolute {
		result, err = m.applyLoadAbsolute(m.pasteModeAction, content, "terminal")
	} else {
		result, err = m.applyLoadRelative(m.pasteModeAction, content, "terminal")
	}

	if err != nil {
		m.err = err
		return m, nil
	}

	// Apply the result
	m.ApplyResult(result)
	return m, nil
}

// handleHistoryUp recalls the previous command from history.
func (m Model) handleHistoryUp() tea.Model {
	if len(m.history) == 0 {
		return m
	}

	if m.historyIdx == -1 {
		// Start browsing: save current input, go to most recent
		m.historyTmp = m.textInput.Value()
		m.historyIdx = len(m.history) - 1
	} else if m.historyIdx > 0 {
		m.historyIdx--
	}

	m.textInput.SetValue(m.history[m.historyIdx])
	m.textInput.CursorEnd()
	m.updateCompletions()
	return m
}

// handleHistoryDown recalls the next command from history, or restores the original input.
func (m Model) handleHistoryDown() tea.Model {
	if m.historyIdx == -1 {
		return m
	}

	if m.historyIdx < len(m.history)-1 {
		m.historyIdx++
		m.textInput.SetValue(m.history[m.historyIdx])
	} else {
		// Back to current input
		m.historyIdx = -1
		m.textInput.SetValue(m.historyTmp)
		m.historyTmp = ""
	}

	m.textInput.CursorEnd()
	m.updateCompletions()
	return m
}

// applyCompletion applies a completion to the input.
func (m *Model) applyCompletion(comp Completion) {
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
		m.selected = -1
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
