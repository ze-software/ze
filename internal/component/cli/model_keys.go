// Design: docs/architecture/config/yang-config-design.md — config editor
// Overview: model.go — TUI model struct and update dispatch

package cli

import (
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// processStreamPipes selects the stream pipe boundary from the filesystem
// authority carried by this model. A daemon-hosted model must never open save
// destinations. The operator-local model owns the save lifecycle it receives.
func (m *Model) processStreamPipes(input string) (cmd string, format func(string) string, flags command.PipeFlags, saves *command.StreamSaves, errMsg string) {
	if m.filesystemAuthority == FilesystemAuthorityOperatorLocal {
		return command.ProcessStreamPipes(input, m.cliFormat)
	}
	cmd, format, flags, errMsg = command.ProcessRemoteStreamPipes(input, m.cliFormat)
	return cmd, format, flags, nil, errMsg
}

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

	// Escape descends exactly one reveal level, and the explanation is the
	// level it descends from first. It takes off the explanation and nothing
	// else: the operator typed those words to reach it, and the arm below
	// clears the whole input (AC-5).
	if keyStr == keyEsc && m.revealLevel() == revealExplanation {
		m.dismissReveal()
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
			m.dismissReveal()
			return m, nil
		case tea.KeyDown:
			m.selected = (m.selected + 1) % len(m.completions)
			m.dismissReveal()
			return m, nil
		case tea.KeyEscape:
			m.showDropdown = false
			m.dismissReveal()
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
		// Every branch below leaves the prompt, so the reveal goes first. Ctrl-C
		// reaches here at any level, and a quit confirmation under an
		// explanation box would ask a question the operator cannot read (AC-7).
		m.dismissReveal()
		// Stop active monitor session before considering quit.
		if m.monitorSession != nil {
			m.stopMonitorSession()
			return m, nil
		}
		// Escape with input or non-config viewport: clear and return to config view.
		if keyStr == keyEsc && (m.textInput.Value() != "" || (m.hasEditor() && !m.showingConfig)) {
			m.textInput.SetValue("")
			m.showDropdown = false
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
		// ? reveals the long explanation of the candidate the operator has
		// singled out, in the box Tab opens. The candidate's summary is
		// already on message line 2, read from the selection (model_render.go
		// warningText), so this key adds the text that row cannot hold.
		//
		// With no candidate singled out there is nothing to explain, so ? does
		// what Tab does.
		if comp, ok := m.selectedCandidate(); ok {
			m.revealCandidateExplanation(comp)
			return m, nil
		}
		return m.handleTab()

	case key.Code == tea.KeyEnter:
		m.dismissReveal()
		return m.handleEnter()

	case key.Text != "":
		// Typing dismisses whatever the completion machinery revealed and
		// resets history browsing. The rune still reaches the input below.
		m.dismissReveal()
		m.history.resetBrowsing()
		// Pass to text input
		m.textInput, cmd = m.textInput.Update(msg)
		m.updateCompletions()
		return m, tea.Batch(cmd, m.scheduleValidation())
	}

	// All other key types (including Backspace): forward to text input for processing
	m.dismissReveal()
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
		if m.completions[0].Type == completionHint {
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

	// The completion list is exhausted, so Tab has nothing left to add. It
	// reveals the explanation the command declares instead (AC-4).
	m.revealExplanation()
	return m, nil
}

// revealExplanation puts the long explanation of the typed command on the
// screen. When the command declares none, the message line says so. It invents
// no text: an unknown command and a command with no explanation both leave the
// level where it is.
//
// It MUST run after the last updateCompletions call on the path. That function
// calls dismissReveal whenever one candidate or none is left. That is the state
// which brings Tab here, so a reveal written before it is erased by it.
func (m *Model) revealExplanation() {
	input, ok := m.commandCompleterInput()
	if !ok {
		// The command completer is not the completion source for this input, so
		// nothing here knows what the operator typed. Config paths are the
		// config editor's own completion and declare no long form.
		return
	}

	m.revealExplanationOf(input)
}

// selectedCandidate answers the candidate the operator has singled out. That is
// the menu selection while the menu is open, and the one match ghost text offers
// while the menu is closed. It answers false when they singled out none.
func (m Model) selectedCandidate() (Completion, bool) {
	if m.showDropdown && m.selected >= 0 && m.selected < len(m.completions) {
		return m.completions[m.selected], true
	}
	if len(m.completions) == 1 && m.ghostText != "" {
		return m.completions[0], true
	}
	return Completion{}, false
}

// revealCandidateExplanation puts the long explanation of one CANDIDATE on the
// screen. The operator has typed a prefix and highlighted a name. So the command
// to explain is the one the candidate would produce, and never the text now in
// the prompt.
func (m *Model) revealCandidateExplanation(comp Completion) {
	if input, ok := m.commandCompleterInput(); ok {
		m.revealExplanationOf(completedInput(input, comp.Text))
		return
	}

	// The command completer is not the source, so the candidate is a config
	// path. A config node declares the same two texts a command declares. The
	// YANG description is the one-line summary the menu row shows. The ze:help
	// extension is the explanation, and it is often a paragraph. The box takes
	// the explanation alone, so the row and the box never say one thing twice.
	// A node that declares no ze:help declares no explanation, and
	// revealDeclared says so.
	m.revealDeclared(subjectOf(completedInput(m.textInput.Value(), comp.Text)), comp.LongHelp)
}

// subjectOf names the command or the config path an explanation is about. A
// pipe operator explains nothing about it, so the subject is the part before
// the first one. TreeCompleter.Explain cuts the same way.
func subjectOf(input string) string {
	base, _, _ := strings.Cut(input, "|")
	return textbuf.Join(strings.Fields(base), " ")
}

// revealDeclared puts one declared text on the screen for one subject. An empty
// text is the subject declaring none, and the message row says which rather
// than leaving the operator with a key that did nothing.
func (m *Model) revealDeclared(subject, text string) {
	if subject == "" {
		return
	}
	if text == "" {
		// Silence would leave the operator unable to tell an undeclared
		// explanation from a dead key, so the message line says which it is.
		// It reads "<command>: <what>", the shape the ? hint uses.
		var tb textbuf.Buffer
		m.completionHint = tb.Str(subject).Str(": no explanation is declared").String()
		m.completionHintDim = true
		return
	}
	m.explanation = text
	m.explanationSubject = subject
}

// revealExplanationOf puts the long explanation of one command on the screen.
// The caller states which command: the text in the prompt for Tab, and the text
// the highlighted candidate would produce for ?.
func (m *Model) revealExplanationOf(input string) {
	text, _ := m.commandCompleter.Explain(input)
	m.revealDeclared(subjectOf(input), text)
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
		m.dismissReveal()
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
		m.dismissReveal()
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
	m.dismissReveal()
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
	m.dismissReveal()
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
	m.dismissReveal()
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
			Text: cmd, Description: "Set default output format", Type: completionCommand,
		})
	}
	var tb textbuf.Buffer
	cmdSpace := tb.Str(cmd).Byte(' ').String()
	if input == cmd || input == cmdSpace {
		for name := range validCLIFormats {
			completions = append(completions, Completion{
				Text: tb.Reset().Str(cmd).Byte(' ').Str(name).String(), Description: tb.Reset().Str(name).Str(" format").String(), Type: completionValue,
			})
		}
		return completions
	}
	if strings.HasPrefix(input, cmdSpace) {
		partial := input[len(cmdSpace):]
		for name := range validCLIFormats {
			if strings.HasPrefix(name, partial) {
				completions = append(completions, Completion{
					Text: tb.Reset().Str(cmd).Byte(' ').Str(name).String(), Description: tb.Reset().Str(name).Str(" format").String(), Type: completionValue,
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
