// Design: docs/architecture/config/yang-config-design.md — config editor
// Overview: model.go — TUI model struct and update dispatch

package cli

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg dispatches keyboard input to the appropriate handler.
func (m Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	key := tea.Key(msg)
	keyStr := key.String()

	// Dashboard mode intercepts all keys.
	if m.dashboard != nil {
		if m.handleDashboardKey(keyStr) {
			return m, nil
		}
	}

	// Lifecycle confirmation takes highest priority (quit, stop, restart).
	if m.confirmQuit || m.confirmStop || m.confirmRestart {
		confirmed := false
		isEscOrCtrlC := keyStr == keyCtrlC || keyStr == keyEsc
		if isEscOrCtrlC {
			if m.confirmQuit {
				pending := m.hasEditor() && m.hasPendingChanges()
				if !pending {
					m.autoSaveOnQuit()
					m.quitting = true
					return m, tea.Quit
				}
			}
			// Esc cancels stop/restart confirmation (fall through to cancel below)
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
			if m.confirmQuit {
				m.autoSaveOnQuit()
				m.quitting = true
				return m, tea.Quit
			}
		}
		// Any other key cancels
		m.confirmQuit = false
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
			m.completionHint = comp.Text + ": " + comp.Description
			m.completionHintDim = false
			return m, nil
		}
		// Show description of single ghost-text match
		if len(m.completions) == 1 && m.ghostText != "" {
			comp := m.completions[0]
			m.completionHint = comp.Text + ": " + comp.Description
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
		m.history.ResetBrowsing()
		// Pass to text input
		m.textInput, cmd = m.textInput.Update(msg)
		m.updateCompletions()
		return m, tea.Batch(cmd, m.scheduleValidation())
	}

	// All other key types (including Backspace): forward to text input for processing
	m.completionHint = ""
	m.completionHintDim = false
	m.history.ResetBrowsing()
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
	// "run" (bare) in edit mode -> switch to command mode.
	// "run <args>" in edit mode -> switch to command mode and execute.
	// "edit" (bare) in command mode -> switch to edit mode.
	// Config commands (set, delete, etc.) in command mode -> switch to edit mode and execute.
	if m.mode == ModeEdit && (input == cmdRun || strings.HasPrefix(input, cmdRun+" ")) {
		args := strings.TrimSpace(strings.TrimPrefix(input, cmdRun))
		m.textInput.SetValue("")
		m.SwitchMode(ModeCommand)
		m.updateCompletions()
		if args == "" {
			return m, nil
		}
		// Save to history and execute.
		if m.history.Append(args) {
			m.history.Save(m.mode.String())
		}
		m.showDropdown = false
		m.completionHint = ""
		m.completionHintDim = false
		m.selected = -1
		m.ghostText = ""
		m.completions = nil
		if isDashboardCommand(args) {
			dashCmd := m.startDashboard()
			return m, dashCmd
		}
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
		// Fall through to normal dispatch -- history/clear happens below,
		// executeCommand runs with the switched mode.
	}

	// Handle exit/quit directly (not via async command dispatch)
	if input == cmdExit || input == cmdQuit {
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
	m.completions = nil

	// Execute command -- dispatch based on mode
	if m.mode == ModeCommand {
		m.lastCommand = input
		if isDashboardCommand(input) {
			dashCmd := m.startDashboard()
			return m, dashCmd
		}
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
	m.ApplyResult(result)
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
