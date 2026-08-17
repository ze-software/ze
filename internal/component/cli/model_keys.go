// Design: docs/architecture/config/yang-config-design.md — config editor
// Overview: model.go — TUI model struct and update dispatch

package cli

import (
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// handleKeyMsg dispatches keyboard input to the appropriate handler.
func (m Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	key := tea.Key(msg)
	keyStr := key.String()

	// Active live view (dashboard / ping / traceroute, plain or piped) gets
	// first refusal on every key. Full-screen views absorb all keys; the piped
	// "| log" variants absorb only Esc/Ctrl-C/q and let other keys fall through
	// (so the scrollback stays scrollable). A single active-view delegate
	// replaces the former per-feature arms.
	if m.activeView != nil {
		if m.activeView.key(&m, keyStr) {
			return m, nil
		}
	}

	// Lifecycle confirmation takes highest priority (quit, stop, restart).
	if m.confirmQuit || m.confirmStop || m.confirmRestart {
		confirmed := false
		isEscOrCtrlC := keyStr == keyCtrlC || keyStr == keyEsc
		if isEscOrCtrlC {
			if m.confirmQuit && !m.confirmExitConfig {
				m.autoSaveOnQuit()
				m.quitting = true
				return m, tea.Quit
			}
			// Esc cancels config-exit and stop/restart confirmation
		} else if key.Text == "y" || key.Text == "Y" {
			confirmed = true
		}
		if confirmed {
			if m.confirmStop && m.shutdownFunc != nil {
				m.shutdownFunc()
				m.quitting = true
				return m, tea.Quit
			}
			if m.confirmRestart && m.restartFunc != nil {
				m.restartFunc()
				m.quitting = true
				return m, tea.Quit
			}
			if m.confirmQuit && m.confirmExitConfig {
				m.confirmQuit = false
				m.confirmExitConfig = false
				m.textInput.SetValue("")
				m.switchMode(ModeOperational)
				m.updateCompletions()
				return m, nil
			}
			if m.confirmQuit {
				m.autoSaveOnQuit()
				m.quitting = true
				return m, tea.Quit
			}
		}
		// Any other key cancels
		m.confirmQuit = false
		m.confirmExitConfig = false
		m.confirmStop = false
		m.confirmRestart = false
		m.statusMessage = ""
		return m, nil
	}

	// Dropdown navigation takes priority
	if m.showDropdown && len(m.completions) > 0 {
		switch key.Code {
		case tea.KeyUp:
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.completions) - 1
			}
			m.completionHint = ""
			m.completionHintDim = false
			return m, nil
		case tea.KeyDown:
			m.selected = (m.selected + 1) % len(m.completions)
			m.completionHint = ""
			m.completionHintDim = false
			return m, nil
		case tea.KeyEscape:
			m.showDropdown = false
			m.completionHint = ""
			m.completionHintDim = false
			m.selected = -1
			return m, nil
		case tea.KeyEnter:
			return m.handleEnter()
		case tea.KeyTab:
			if key.Mod.Contains(tea.ModShift) {
				return m.handleShiftTab()
			}
			return m.handleTab()
		}
	}

	// Handle help overlay
	if m.showHelp {
		if keyStr == keyEsc || keyStr == keyCtrlC {
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
		switch {
		case key.Code == tea.KeyUp && key.Mod.Contains(tea.ModShift):
			m.viewport.ScrollUp(1)
			return m, nil
		case key.Code == tea.KeyDown && key.Mod.Contains(tea.ModShift):
			m.viewport.ScrollDown(1)
			return m, nil
		case key.Code == tea.KeyPgUp || (key.Code == tea.KeyUp && key.Mod.Contains(tea.ModCtrl)):
			m.viewport.PageUp()
			return m, nil
		case key.Code == tea.KeyPgDown || (key.Code == tea.KeyDown && key.Mod.Contains(tea.ModCtrl)):
			m.viewport.PageDown()
			return m, nil
		}
	}

	// Handle command history with Up/Down arrows
	switch key.Code {
	case tea.KeyUp:
		return m.handleHistoryUp(), nil
	case tea.KeyDown:
		return m.handleHistoryDown(), nil
	}

	switch {
	case keyStr == keyCtrlC || keyStr == keyEsc:
		// Stop active monitor session before considering quit.
		if m.monitorSession != nil {
			m.stopMonitorSession()
			return m, nil
		}
		// Escape with input or non-config viewport: clear and return to config view.
		if keyStr == keyEsc && (m.textInput.Value() != "" || (m.hasEditor() && !m.showingConfig)) {
			m.textInput.SetValue("")
			m.showDropdown = false
			m.completionHint = ""
			m.completionHintDim = false
			m.selected = -1
			m.ghostText = ""
			m.syncGhostSuggestions()
			m.completions = nil
			m.statusMessage = ""
			if m.hasEditor() {
				m.showConfigContent()
			}
			m.updateCompletions()
			return m, nil
		}
		if m.hasEditor() && m.hasPendingChanges() {
			m.confirmQuit = true
			m.statusMessage = "Pending changes. Use 'commit', 'discard all', or type y to force quit."
			return m, nil
		}
		m.confirmQuit = true
		m.statusMessage = "Quit? (Esc/y to confirm, any other key to cancel)"
		return m, nil

	case key.Code == tea.KeyTab && key.Mod.Contains(tea.ModShift):
		return m.handleShiftTab()

	case key.Code == tea.KeyTab:
		return m.handleTab()

	case key.Text == "?":
		// ? shows full description when dropdown is open, otherwise triggers completion like Tab
		// Show description of selected item in dropdown
		if m.showDropdown && m.selected >= 0 && m.selected < len(m.completions) {
			comp := m.completions[m.selected]
			var tb textbuf.Buffer
			m.completionHint = tb.Str(comp.Text).Str(": ").Str(comp.Description).String()
			m.completionHintDim = false
			return m, nil
		}
		// Show description of single ghost-text match
		if len(m.completions) == 1 && m.ghostText != "" {
			comp := m.completions[0]
			var tb textbuf.Buffer
			m.completionHint = tb.Str(comp.Text).Str(": ").Str(comp.Description).String()
			m.completionHintDim = false
			return m, nil
		}
		return m.handleTab()

	case key.Code == tea.KeyEnter:
		m.completionHint = ""
		m.completionHintDim = false
		return m.handleEnter()

	case key.Text != "":
		// Typing clears transient completion hint and resets history browsing.
		m.completionHint = ""
		m.completionHintDim = false
		m.history.resetBrowsing()
		// Pass to text input
		m.textInput, cmd = m.textInput.Update(msg)
		m.updateCompletions()
		return m, tea.Batch(cmd, m.scheduleValidation())
	}

	// All other key types (including Backspace): forward to text input for processing
	m.completionHint = ""
	m.completionHintDim = false
	m.history.resetBrowsing()
	m.textInput, cmd = m.textInput.Update(msg)
	m.updateCompletions()
	return m, tea.Batch(cmd, m.scheduleValidation())
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
			var tb textbuf.Buffer
			m.textInput.SetValue(tb.Str(m.textInput.Value()).Str(m.ghostText).Byte(' ').String())
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
		// Skip hint-only completions (e.g., <value>, <string>) -- display-only, not applicable
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
		m.completionHint = ""
		m.completionHintDim = false
		m.selected = -1
		m.updateCompletions()
		return m, nil
	}

	input := strings.TrimSpace(m.textInput.Value())
	if input == "" {
		// Empty Enter refreshes config view when viewport shows non-config content.
		if m.hasEditor() {
			m.showConfigContent()
			m.statusMessage = ""
		}
		return m, nil
	}

	// Handle mode switching commands.
	// "run <args>" in config mode -> one-shot execution, stay in config mode.
	// "configure" in operational mode -> switch to config mode.
	// Config commands (set, delete, etc.) in operational mode -> switch to config mode and execute.
	if m.mode == ModeConfig && strings.HasPrefix(input, cmdRun+" ") {
		args := strings.TrimSpace(strings.TrimPrefix(input, cmdRun))
		m.textInput.SetValue("")
		if m.history.Append(input) {
			m.history.Save(m.mode.String())
		}
		m.showDropdown = false
		m.completionHint = ""
		m.completionHintDim = false
		m.selected = -1
		m.ghostText = ""
		m.syncGhostSuggestions()
		m.completions = nil
		if args == "clear" {
			m.outputBuf.Reset()
			m.showConfigContent()
			m.statusMessage = ""
			return m, nil
		}
		if spec, ok := resolveView(args); ok {
			// Registry longest-prefix match replaces the per-feature
			// dashboard/ping/traceroute isXCommand chain (editor "run" mode).
			// start mutates m (sets activeView/status), so evaluate it before
			// returning m.
			prev := m.activeView
			cmd := spec.start(&m, args)
			// If start installed a new view (success), release the replaced
			// view's live poller so switching does not leak it (spec Security
			// Review). A failed start leaves activeView == prev, so the old view
			// keeps running untouched.
			if prev != nil && m.activeView != prev {
				prev.release()
			}
			return m, cmd
		}
		if isMonitorCommand(args) {
			cmd := m.startMonitorSessionFromInput(extractMonitorCmdArgs(args), args)
			return m, cmd
		}
		m.statusMessage = "running..."
		return m, m.executeOperationalCommand(args)
	}
	if m.mode == ModeConfig && input == cmdRun {
		m.textInput.SetValue("")
		m.statusMessage = "usage: run <command>"
		return m, nil
	}
	if m.mode == ModeOperational && input == cmdConfigure {
		if !m.hasEditor() {
			m.textInput.SetValue("")
			m.statusMessage = "config mode not available (no config file loaded)"
			return m, nil
		}
		m.textInput.SetValue("")
		m.switchMode(ModeConfig)
		m.updateCompletions()
		return m, nil
	}
	if m.mode == ModeOperational && handleSetCLIFormat(input, &m) {
		if m.history.Append(input) {
			m.history.Save(m.mode.String())
		}
		return m, nil
	}
	if m.mode == ModeOperational && isConfigCommand(input) && !isOperationalVerb(input) {
		if m.hasEditor() {
			m.switchMode(ModeConfig)
			// Fall through to normal dispatch -- history/clear happens below,
			// executeCommand runs with the switched mode.
		} else {
			m.textInput.SetValue("")
			m.statusMessage = "config mode not available (no config file loaded)"
			return m, nil
		}
	}

	// Handle exit/quit directly (not via async command dispatch).
	// In config mode, "exit" returns to operational mode (like NOS convention).
	// In operational mode, "exit" and "quit" terminate the CLI.
	if input == cmdExit || input == cmdQuit {
		if m.mode == ModeConfig && input == cmdExit {
			if m.hasPendingChanges() {
				m.textInput.SetValue("")
				m.statusMessage = "Pending changes. Use 'commit', 'discard all', or type y to force exit."
				m.confirmQuit = true
				m.confirmExitConfig = true
				return m, nil
			}
			m.textInput.SetValue("")
			m.switchMode(ModeOperational)
			m.updateCompletions()
			return m, nil
		}
		if m.hasPendingChanges() {
			m.textInput.SetValue("")
			m.statusMessage = "Pending changes. Use 'commit', 'discard all', or type y to force quit."
			m.confirmQuit = true
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
	}

	// Handle stop/restart: daemon lifecycle commands with confirmation.
	// These affect all connected users, so require y/N confirmation.
	if input == cmdStop {
		if m.shutdownFunc == nil {
			m.textInput.SetValue("")
			m.statusMessage = "stop not available (not connected to daemon)"
			return m, nil
		}
		m.textInput.SetValue("")
		m.statusMessage = "This will shut down the daemon. Continue? [y/N]"
		m.confirmStop = true
		return m, nil
	}
	if input == cmdRestart {
		if m.restartFunc == nil {
			m.textInput.SetValue("")
			m.statusMessage = "restart not available (not connected to daemon)"
			return m, nil
		}
		m.textInput.SetValue("")
		m.statusMessage = "This will restart the daemon (GR marker written). Continue? [y/N]"
		m.confirmRestart = true
		return m, nil
	}

	// Save to history
	if m.history.Append(input) {
		m.history.Save(m.mode.String())
	}

	// Clear input
	m.textInput.SetValue("")
	m.showDropdown = false
	m.completionHint = ""
	m.completionHintDim = false
	m.selected = -1
	m.ghostText = ""
	m.syncGhostSuggestions()
	m.completions = nil

	// Clear scrollback.
	if input == "clear" {
		m.outputBuf.Reset()
		if m.hasEditor() {
			m.showConfigContent()
		} else {
			m.viewportContent = ""
			m.viewport.SetContent("")
			m.viewport.GotoTop()
		}
		m.statusMessage = ""
		return m, nil
	}

	// Execute command -- dispatch based on mode
	if m.mode == ModeOperational {
		m.lastCommand = input
		m.writeCommandEcho()
		if spec, ok := resolveView(input); ok {
			// Registry longest-prefix match replaces the per-feature
			// dashboard/ping/traceroute isXCommand chain (command-only mode).
			// start mutates m (sets activeView/status), so evaluate it before
			// returning m.
			prev := m.activeView
			cmd := spec.start(&m, input)
			// Release the replaced view's live poller on a successful switch so
			// it does not leak (spec Security Review); a failed start leaves
			// activeView == prev, so the old view keeps running untouched.
			if prev != nil && m.activeView != prev {
				prev.release()
			}
			return m, cmd
		}
		if isMonitorCommand(input) {
			cmd := m.startMonitorSessionFromInput(extractMonitorCmdArgs(input), input)
			return m, cmd
		}
		m.statusMessage = "running..."
		return m, m.executeOperationalCommand(input)
	}
	return m, m.executeCommand(input)
}

// handlePasteModeKey handles key input during paste mode.
// Ctrl-D ends paste mode and processes the buffer.
// Enter adds a newline to the buffer.
// Other characters are accumulated.
func (m Model) handlePasteModeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := tea.Key(msg)
	keyStr := key.String()

	switch {
	case keyStr == "ctrl+d":
		// End paste mode and process buffer
		return m.finishPasteMode()

	case keyStr == keyCtrlC || keyStr == keyEsc:
		// Cancel paste mode
		m.pasteMode = false
		m.pasteBuffer.Reset()
		m.statusMessage = "Paste mode canceled"
		return m, nil

	case key.Code == tea.KeyEnter:
		// Add newline to buffer
		m.pasteBuffer.WriteString("\n")
		return m, nil

	case key.Code == tea.KeyBackspace:
		// Remove last character from buffer
		s := m.pasteBuffer.String()
		if s != "" {
			m.pasteBuffer.Reset()
			m.pasteBuffer.WriteString(s[:len(s)-1])
		}
		return m, nil

	case key.Text != "":
		// Accumulate characters (includes space)
		m.pasteBuffer.WriteString(key.Text)
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
	m.applyResult(result)
	return m, nil
}

// handleHistoryUp recalls the previous command from history.
func (m Model) handleHistoryUp() tea.Model {
	value, ok := m.history.Up(m.textInput.Value())
	if !ok {
		return m
	}
	m.completionHint = ""
	m.completionHintDim = false
	m.textInput.SetValue(value)
	m.textInput.CursorEnd()
	m.updateCompletions()
	return m
}

// handleHistoryDown recalls the next command from history, or restores the original input.
func (m Model) handleHistoryDown() tea.Model {
	value, ok := m.history.Down()
	if !ok {
		return m
	}
	m.completionHint = ""
	m.completionHintDim = false
	m.textInput.SetValue(value)
	m.textInput.CursorEnd()
	m.updateCompletions()
	return m
}

var validCLIFormats = map[string]bool{
	"text": true, "table": true, "json": true, "yaml": true, "ndjson": true,
}

// validCLIFormatNames returns the accepted format names, sorted, for error text.
// Derived from validCLIFormats so the list is never duplicated (ai/rules/evidence.md).
func validCLIFormatNames() string {
	return textbuf.Join(slices.Sorted(maps.Keys(validCLIFormats)), ", ")
}

func appendCLIFormatCompletions(completions []Completion, input string) []Completion {
	const cmd = "set cli format"
	if input == "" || strings.HasPrefix(cmd, input) {
		return append(completions, Completion{
			Text: cmd, Description: "Set default output format", Type: "command",
		})
	}
	var tb textbuf.Buffer
	cmdSpace := tb.Str(cmd).Byte(' ').String()
	if input == cmd || input == cmdSpace {
		for name := range validCLIFormats {
			completions = append(completions, Completion{
				Text: tb.Reset().Str(cmd).Byte(' ').Str(name).String(), Description: tb.Reset().Str(name).Str(" format").String(), Type: "value",
			})
		}
		return completions
	}
	if strings.HasPrefix(input, cmdSpace) {
		partial := input[len(cmdSpace):]
		for name := range validCLIFormats {
			if strings.HasPrefix(name, partial) {
				completions = append(completions, Completion{
					Text: tb.Reset().Str(cmd).Byte(' ').Str(name).String(), Description: tb.Reset().Str(name).Str(" format").String(), Type: "value",
				})
			}
		}
	}
	return completions
}

// handleSetCLIFormat handles `set cli format [<value>]` in operational mode.
// Returns true if the input was handled.
func handleSetCLIFormat(input string, m *Model) bool {
	const prefix = "set cli format"
	if input != prefix && !strings.HasPrefix(input, prefix+" ") {
		return false
	}
	rest := strings.TrimSpace(input[len(prefix):])

	m.textInput.SetValue("")

	if rest == "" {
		current := m.sessionFormat()
		if current == "" {
			current = "text"
		}
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("cli format: ").Str(current).String()
		return true
	}

	if !validCLIFormats[rest] {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("invalid format: ").Str(rest).Str(" (valid: ").Str(validCLIFormatNames()).Byte(')').String()
		return true
	}

	// Record on the session, NOT via env.Set: env.Set writes a process-global cache
	// and os.Setenv (env.go), so one session's choice would change the default output
	// format for every other concurrent SSH and web CLI session.
	m.cliFormat = rest
	var tb textbuf.Buffer
	m.statusMessage = tb.Str("cli format set to ").Str(rest).String()
	return true
}

// sessionFormat returns the format this session should use: its `set cli format`
// override if any, otherwise the configured default (the environment cli format
// default YANG leaf, plumbed to ze.cli.format). Empty means neither is set.
func (m *Model) sessionFormat() string {
	if m.cliFormat != "" {
		return m.cliFormat
	}
	return env.Get("ze.cli.format")
}
