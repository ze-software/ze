package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
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
	content     string      // The text content to display
	lineMapping map[int]int // Maps displayed line (1-based) to original line (1-based), nil for full content
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
	statusMessage   string // Temporary status message (clears on next command)
	err             error
	width           int
	height          int
	quitting        bool

	// Commit confirm state
	confirmTimerActive bool   // True if waiting for confirm/abort
	confirmBackupPath  string // Path to backup for rollback on timeout/abort

	// Paste mode state (for load terminal ...)
	pasteMode         bool            // True if accumulating paste input
	pasteBuffer       strings.Builder // Accumulates pasted lines
	pasteModeLocation string          // "absolute" or "relative"
	pasteModeAction   string          // "replace" or "merge"
}

// PipeFilter represents a filter in a pipe chain.
type PipeFilter struct {
	Type string // "grep", "head", "tail"
	Arg  string // Pattern or count
}

// Debounce delay for validation after keystroke.
const validationDebounce = 100 * time.Millisecond

// Command names (used in multiple switch statements).
const cmdShow = "show"

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
	statusMessage string        // Temporary status message (shown above viewport, clears on next command)
	newContext    []string      // New context path (nil = no change)
	clearContext  bool          // True to clear context to root
	isTemplate    bool          // Template mode flag (used with newContext)
	showHelp      bool          // Show help overlay
	revalidate    bool          // Trigger re-validation after command

	// Commit confirm state (must be propagated through result, not set directly on model)
	setConfirmTimer   bool   // True to set confirmTimerActive
	confirmTimerValue bool   // Value to set confirmTimerActive to
	confirmBackupPath string // Backup path for rollback (empty to clear)

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
)

// NewModel creates a new editor model.
func NewModel(ed *Editor) (Model, error) {
	ti := textinput.New()
	ti.Placeholder = "type command or Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 60

	vp := viewport.New(80, 20)
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
		validationErrors:   result.Errors,
		validationWarnings: result.Warnings,
	}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
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

		// Handle viewport scrolling (when no dropdown)
		if m.showViewport && !m.showDropdown {
			switch msg.Type { //nolint:exhaustive // only handle specific keys
			case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		switch msg.Type { //nolint:exhaustive // only handle specific keys
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

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

		default:
			// Update text input and regenerate completions
			m.textInput, cmd = m.textInput.Update(msg)
			m.updateCompletions()
			return m, tea.Batch(cmd, m.scheduleValidation())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = min(msg.Width-4, 80)
		// Resize viewport
		m.viewport.Width = min(msg.Width-4, 80)
		m.viewport.Height = max(msg.Height-10, 5)
		// Show config on first size event (startup)
		if !m.showViewport && m.viewportContent == "" {
			if m.editor.WorkingContent() != "" {
				m.showConfigContent()
			}
		}
		return m, nil

	case commandResultMsg:
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

		// Apply viewport changes
		if r.configView != nil {
			m.setViewportData(*r.configView)
		} else if r.output != "" {
			m.setViewportText(r.output)
		}

		// Status message (temporary notification)
		m.statusMessage = r.statusMessage

		// Other state
		if r.showHelp {
			m.showHelp = true
		}
		if r.revalidate {
			m.runValidation()
		}

		// Apply confirm timer state (must be propagated through result)
		if r.setConfirmTimer {
			m.confirmTimerActive = r.confirmTimerValue
			m.confirmBackupPath = r.confirmBackupPath
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
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleTab handles Tab key press.
func (m Model) handleTab() (tea.Model, tea.Cmd) {
	// Ensure completions are populated
	if len(m.completions) == 0 {
		m.updateCompletions()
	}

	if m.ghostText != "" && !m.showDropdown {
		// Accept ghost text
		m.textInput.SetValue(m.textInput.Value() + m.ghostText + " ")
		m.textInput.CursorEnd()
		m.updateCompletions()
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
		// Single completion: apply it
		m.applyCompletion(m.completions[0])
		m.updateCompletions()
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

	// Clear input
	m.textInput.SetValue("")
	m.showDropdown = false
	m.selected = -1
	m.ghostText = ""
	m.completions = nil

	// Execute command
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
		m.statusMessage = "Paste mode cancelled"
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
		if len(s) > 0 {
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
func (m *Model) updateCompletions() {
	input := m.textInput.Value()
	m.completions = m.completer.Complete(input, m.contextPath)
	m.ghostText = m.completer.GhostText(input, m.contextPath)

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

// setViewportData sets content with line mapping in the viewport.
// Applies error and warning highlighting based on validation state.
func (m *Model) setViewportData(data viewportData) {
	highlighted := highlightValidationIssues(data.content, m.validationErrors, m.validationWarnings, data.lineMapping)
	m.viewportContent = highlighted
	m.viewport.SetContent(highlighted)
	m.viewport.GotoTop()
	m.showViewport = true
	m.err = nil
}

// setViewportText sets simple text content without line mapping.
// Use for non-config content like diffs, history, or messages.
func (m *Model) setViewportText(content string) {
	m.setViewportData(viewportData{content: content, lineMapping: nil})
}

// highlightValidationIssues adds styling to lines with validation errors or warnings.
// Errors are highlighted in red, warnings in yellow.
// lineMapping maps filtered line numbers to original line numbers (used when showing filtered content).
func highlightValidationIssues(content string, errors, warnings []ConfigValidationError, lineMapping map[int]int) string {
	if len(errors) == 0 && len(warnings) == 0 {
		return content
	}

	// Build sets of line numbers (1-based, in original content)
	errorLines := make(map[int]bool)
	for _, e := range errors {
		if e.Line > 0 {
			errorLines[e.Line] = true
		}
	}

	warningLines := make(map[int]bool)
	for _, w := range warnings {
		if w.Line > 0 && !errorLines[w.Line] { // Errors take precedence over warnings
			warningLines[w.Line] = true
		}
	}

	if len(errorLines) == 0 && len(warningLines) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		filteredLineNum := i + 1 // Convert to 1-based

		// Determine the original line number to check
		var origLineNum int
		if lineMapping != nil {
			// Filtered content: map to original line number
			origLineNum = lineMapping[filteredLineNum]
		} else {
			// Full content: filtered line == original line
			origLineNum = filteredLineNum
		}

		if origLineNum > 0 {
			if errorLines[origLineNum] {
				lines[i] = errorLineStyle.Render(line)
			} else if warningLines[origLineNum] {
				lines[i] = warningLineStyle.Render(line)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Use fixed height to prevent scrolling when dropdown appears
	viewHeight := m.height
	if viewHeight < 10 {
		viewHeight = 24 // Default fallback
	}

	var lines []string

	// Header (2 lines: header + blank)
	header := "Ze Editor"
	if m.editor.Dirty() {
		header += " [modified]"
	}
	// Add validation status indicator
	var statusIndicator string
	if len(m.validationErrors) > 0 {
		statusIndicator = errorStyle.Render(fmt.Sprintf(" ⚠️ %d error(s)", len(m.validationErrors)))
	} else if len(m.validationWarnings) > 0 {
		statusIndicator = dimStyle.Render(fmt.Sprintf(" ⚡ %d warning(s)", len(m.validationWarnings)))
	}
	lines = append(lines, dimStyle.Render(header)+statusIndicator+" "+dimStyle.Render("(Tab/?: complete, Enter: execute, Esc: quit)"))
	lines = append(lines, "")

	// Status message (temporary notification from commands)
	if m.statusMessage != "" {
		lines = append(lines, promptStyle.Render("► "+m.statusMessage))
		lines = append(lines, "")
	}

	// Viewport for scrollable content (show/compare output)
	if m.showViewport && m.viewportContent != "" {
		lines = append(lines, dimStyle.Render("─── "+m.contextLabel()+" (↑↓ scroll) ───"))
		vpLines := strings.Split(m.viewport.View(), "\n")
		lines = append(lines, vpLines...)
		lines = append(lines, "")
	}

	// Calculate how many empty lines we need before the prompt
	// Reserve: prompt (1) + error (1 if present)
	// Note: dropdown overlays existing content, doesn't need reserved space
	reservedBottom := 1
	if m.err != nil {
		reservedBottom++
	}

	// Pad to push prompt toward bottom
	for len(lines) < viewHeight-reservedBottom {
		lines = append(lines, "")
	}

	// Prompt with context + input
	promptLine := m.buildPrompt() + m.renderInputWithGhost()
	lines = append(lines, promptLine)

	// Error display
	if m.err != nil {
		lines = append(lines, errorStyle.Render("Error: "+m.err.Error()))
	}

	// Pad to exact height
	for len(lines) < viewHeight {
		lines = append(lines, "")
	}

	// Truncate if too many lines
	if len(lines) > viewHeight {
		lines = lines[:viewHeight]
	}

	baseView := strings.Join(lines, "\n")

	// Overlay dropdown if showing
	if m.showDropdown && len(m.completions) > 0 {
		return m.overlayDropdown(baseView)
	}

	// Help overlay
	if m.showHelp {
		return m.renderHelpOverlay(baseView)
	}

	return baseView
}

// overlayDropdown renders the dropdown as a floating overlay on the base view.
func (m Model) overlayDropdown(base string) string {
	dropdown := m.renderDropdownBox()

	// Find the prompt line position
	baseLines := strings.Split(base, "\n")
	promptLineIdx := len(baseLines) - 1
	for promptLineIdx > 0 && strings.TrimSpace(baseLines[promptLineIdx]) == "" {
		promptLineIdx--
	}

	// Position dropdown starting on the line after prompt
	y := promptLineIdx + 1
	x := 2 // Indent slightly from left edge

	return placeOverlay(x, y, dropdown, base)
}

// placeOverlay places a foreground string over a background string at position (x, y).
func placeOverlay(x, y int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")
	fgHeight := len(fgLines)

	// Clamp y position
	if y < 0 {
		y = 0
	}
	if y+fgHeight > len(bgLines) {
		y = len(bgLines) - fgHeight
	}
	if y < 0 {
		y = 0
	}

	result := make([]string, 0, len(bgLines))
	for i, bgLine := range bgLines {
		if i < y || i >= y+fgHeight {
			result = append(result, bgLine)
			continue
		}

		// Overlay foreground line at position x
		fgLine := fgLines[i-y]
		result = append(result, overlayLine(bgLine, fgLine, x))
	}

	return strings.Join(result, "\n")
}

// overlayLine places fg on top of bg at position x, handling ANSI codes.
func overlayLine(bg, fg string, x int) string {
	bgWidth := ansi.PrintableRuneWidth(bg)
	fgWidth := ansi.PrintableRuneWidth(fg)

	// ANSI reset to prevent style bleed
	const reset = "\x1b[0m"

	// If bg is shorter than x, just pad and add fg
	if bgWidth <= x {
		return bg + reset + strings.Repeat(" ", x-bgWidth) + fg
	}

	// Need to slice bg around fg insertion point
	// Walk through bg tracking visible position vs byte position
	left := truncateAtWidth(bg, x)
	leftWidth := ansi.PrintableRuneWidth(left)

	// Pad if truncation was short
	padding := ""
	if leftWidth < x {
		padding = strings.Repeat(" ", x-leftWidth)
	}

	// Get right portion: skip x + fgWidth visible chars
	right := skipWidth(bg, x+fgWidth)

	// Add reset between parts to prevent style bleeding
	return left + reset + padding + fg + reset + right
}

// truncateAtWidth returns the prefix of s up to width visible characters.
func truncateAtWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	w := 0
	inEsc := false

	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			result.WriteRune(r)
			continue
		}
		if inEsc {
			result.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}

		if w >= width {
			break
		}
		result.WriteRune(r)
		w++
	}

	return result.String()
}

// skipWidth returns the suffix of s after skipping width visible characters.
func skipWidth(s string, width int) string {
	if width <= 0 {
		return s
	}

	w := 0
	inEsc := false
	i := 0

	for _, r := range s {
		if w >= width && !inEsc {
			return s[i:]
		}

		if r == '\x1b' {
			inEsc = true
			i += len(string(r))
			continue
		}
		if inEsc {
			i += len(string(r))
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}

		w++
		i += len(string(r))
	}

	return ""
}

// renderDropdownBox renders the dropdown with a simple format.
// Uses plain text (no ANSI) for consistent width calculations.
func (m Model) renderDropdownBox() string {
	maxShow := min(6, len(m.completions))

	// Calculate scroll offset
	start := 0
	if m.selected >= maxShow {
		start = m.selected - maxShow + 1
	}
	end := start + maxShow
	if end > len(m.completions) {
		end = len(m.completions)
		start = max(0, end-maxShow)
	}

	// Fixed inner width (between │ and │)
	const innerWidth = 48

	var lines []string

	// Top border: ╭─ Completions (15 chars) + dashes + ╮ = 52 total
	lines = append(lines, "╭─ Completions "+strings.Repeat("─", innerWidth-12)+"╮")

	for i := start; i < end; i++ {
		comp := m.completions[i]

		// Build line content
		var prefix string
		if i == m.selected {
			prefix = "> "
		} else {
			prefix = "  "
		}

		cmd := comp.Text
		if len(cmd) > 12 {
			cmd = cmd[:12]
		}

		desc := comp.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		// Format: prefix(2) + cmd(12) + padding + desc
		line := prefix + cmd
		for len(line) < 15 {
			line += " "
		}
		line += desc
		for len(line) < innerWidth {
			line += " "
		}
		if len(line) > innerWidth {
			line = line[:innerWidth]
		}

		lines = append(lines, "│ "+line+" │")
	}

	if len(m.completions) > maxShow {
		more := fmt.Sprintf("  ... %d more", len(m.completions)-maxShow)
		for len(more) < innerWidth {
			more += " "
		}
		lines = append(lines, "│ "+more+" │")
	}

	// Bottom border
	lines = append(lines, "╰"+strings.Repeat("─", innerWidth+2)+"╯")

	return strings.Join(lines, "\n")
}

// contextLabel returns a label for the current context.
func (m Model) contextLabel() string {
	if len(m.contextPath) == 0 {
		return "Configuration"
	}
	return strings.Join(m.contextPath, " ")
}

// renderHelpOverlay renders help as a floating overlay.
func (m Model) renderHelpOverlay(base string) string {
	help := `Commands:
  set <path> <value>   Set a configuration value
  delete <path>        Delete a configuration value
  edit <path>          Enter a subsection context
  edit <list> *        Edit template for all entries
  top                  Return to root context
  up                   Go up one level
  show                 Display configuration (scrollable)
  compare              Show diff vs original
  commit               Save changes with backup
  discard              Revert all changes
  history              List backup files
  rollback <N>         Restore backup N
  exit                 Exit editor

Load:
  load file absolute replace <path>    Replace entire config from file
  load file absolute merge <path>      Merge file at root
  load file relative replace <path>    Replace context subtree from file
  load file relative merge <path>      Merge file at current context
  load terminal absolute replace       Paste mode - replace entire config
  load terminal absolute merge         Paste mode - merge at root
  load terminal relative replace       Paste mode - replace context subtree
  load terminal relative merge         Paste mode - merge at context
  (Paste mode: type content, then Ctrl-D to apply)

Keys:
  Tab                  Complete / cycle suggestions
  ↑↓                   Navigate dropdown / scroll output
  Enter                Execute command / accept selection
  Esc                  Close overlay / quit

Press Esc to close this help.`

	overlay := overlayStyle.Render(help)

	// Center the overlay
	lines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Simple overlay: just show it after a few lines of base
	var result strings.Builder
	for i, line := range lines {
		if i < 3 {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")
	for _, line := range overlayLines {
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}

// buildPrompt returns the context-aware prompt string.
func (m Model) buildPrompt() string {
	if len(m.contextPath) == 0 {
		return promptStyle.Render("ze# ")
	}

	contextStr := strings.Join(m.contextPath, " ")
	if m.isTemplate {
		return promptStyle.Render("ze") +
			contextStyle.Render("["+contextStr+"]") +
			promptStyle.Render("# ")
	}
	return promptStyle.Render("ze") +
		contextStyle.Render("["+contextStr+"]") +
		promptStyle.Render("# ")
}

// renderInputWithGhost renders the text input with ghost text overlay.
func (m Model) renderInputWithGhost() string {
	// If we have ghost text and dropdown is not showing, render manually
	// to avoid textinput's width padding pushing ghost text to the right
	if m.ghostText != "" && !m.showDropdown {
		value := m.textInput.Value()
		prompt := m.textInput.Prompt // Include the "> " prompt
		// Show: prompt + typed text + cursor on first ghost char + rest of ghost text
		// Use reverse video for cursor block like textinput does
		if len(m.ghostText) == 1 {
			cursor := lipgloss.NewStyle().Reverse(true).Render(m.ghostText)
			return prompt + value + cursor
		}
		cursor := lipgloss.NewStyle().Reverse(true).Render(string(m.ghostText[0]))
		return prompt + value + cursor + ghostStyle.Render(m.ghostText[1:])
	}

	return m.textInput.View()
}

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
	case "exit", "quit":
		return commandResult{}, nil // Will be handled by quit logic

	case "help", "?":
		return commandResult{showHelp: true}, nil

	case "top":
		return m.cmdTop()

	case "up":
		return m.cmdUp()

	case "edit":
		return m.cmdEdit(args)

	case cmdShow:
		return m.cmdShow(args)

	case "compare":
		return commandResult{output: m.editor.Diff()}, nil

	case "commit":
		// Check for "commit confirm <N>"
		if len(args) >= 1 && args[0] == "confirm" {
			if len(args) < 2 {
				return commandResult{}, fmt.Errorf("usage: commit confirm <seconds>")
			}
			seconds, err := strconv.Atoi(args[1])
			if err != nil {
				return commandResult{}, fmt.Errorf("invalid seconds: %s", args[1])
			}
			return m.cmdCommitConfirm(seconds)
		}
		return m.cmdCommit()

	case "confirm":
		return m.cmdConfirm()

	case "abort":
		return m.cmdAbort()

	case "discard":
		return m.cmdDiscard()

	case "history":
		return m.cmdHistory()

	case "rollback":
		return m.cmdRollback(args)

	case "load":
		// New syntax: load <source> <location> <action> [file]
		return m.cmdLoadNew(args)

	case "set":
		return m.cmdSet(args)

	case "delete":
		return m.cmdDelete(args)

	case "errors":
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

	// Verify the path exists in the tree
	if m.editor.WalkPath(fullPath) == nil {
		return commandResult{}, fmt.Errorf("block not found: %s", strings.Join(args, " "))
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

	return commandResult{statusMessage: "Configuration committed"}, nil
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
	return m.editor.Dirty()
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
	if r.configView != nil {
		m.setViewportData(*r.configView)
	} else if r.output != "" {
		m.setViewportText(r.output)
	}
	m.statusMessage = r.statusMessage
	if r.showHelp {
		m.showHelp = true
	}
	if r.revalidate {
		m.runValidation()
	}
	if r.setConfirmTimer {
		m.confirmTimerActive = r.confirmTimerValue
		m.confirmBackupPath = r.confirmBackupPath
	}
}

// --- Phase 2: New Editor Features ---

// cmdCommitConfirm commits with auto-rollback if not confirmed within timeout.
// RFC 4271 Section 4.2 doesn't specify this, but it's a standard network CLI feature.
func (m *Model) cmdCommitConfirm(seconds int) (commandResult, error) {
	// Boundary validation: 1-3600 seconds
	if seconds < 1 {
		return commandResult{}, fmt.Errorf("timeout must be at least 1 second")
	}
	if seconds > 3600 {
		return commandResult{}, fmt.Errorf("timeout must be at most 3600 seconds (1 hour)")
	}

	// Validate before commit
	result := m.validator.Validate(m.editor.WorkingContent())
	if len(result.Errors) > 0 {
		return commandResult{}, fmt.Errorf("cannot commit: %d validation error(s). Use 'errors' to see details", len(result.Errors))
	}

	// Save changes (this creates a backup)
	if err := m.editor.Save(); err != nil {
		return commandResult{}, err
	}

	// Get the most recent backup path for potential rollback
	backups, err := m.editor.ListBackups()
	if err != nil || len(backups) == 0 {
		return commandResult{}, fmt.Errorf("commit succeeded but no backup found for rollback")
	}

	return commandResult{
		statusMessage:     fmt.Sprintf("Committed. Confirm within %d seconds or changes will be rolled back. Use 'confirm' or 'abort'.", seconds),
		setConfirmTimer:   true,
		confirmTimerValue: true,
		confirmBackupPath: backups[0].Path,
	}, nil
}

// cmdConfirm confirms a pending commit, making changes permanent.
func (m *Model) cmdConfirm() (commandResult, error) {
	if !m.confirmTimerActive {
		return commandResult{}, fmt.Errorf("no pending commit to confirm")
	}

	return commandResult{
		statusMessage:     "Configuration confirmed and saved permanently.",
		setConfirmTimer:   true,
		confirmTimerValue: false,
		confirmBackupPath: "",
	}, nil
}

// cmdAbort aborts a pending commit and rolls back to previous state.
func (m *Model) cmdAbort() (commandResult, error) {
	if !m.confirmTimerActive {
		return commandResult{}, fmt.Errorf("no pending commit to abort")
	}

	// Rollback to backup
	if m.confirmBackupPath != "" {
		if err := m.editor.Rollback(m.confirmBackupPath); err != nil {
			return commandResult{
				setConfirmTimer:   true,
				confirmTimerValue: false,
				confirmBackupPath: "",
			}, fmt.Errorf("rollback failed: %w", err)
		}
	}

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage:     "Changes rolled back to previous state.",
		configView:        &viewportData{content: content, lineMapping: nil},
		revalidate:        true,
		setConfirmTimer:   true,
		confirmTimerValue: false,
		confirmBackupPath: "",
	}, nil
}

// cmdLoad loads configuration from a file, replacing current content.
func (m *Model) cmdLoad(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: load <file>")
	}

	loadPath := m.resolveConfigPath(args[0])

	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", args[0], err)
	}

	m.editor.SetWorkingContent(string(data))
	m.editor.MarkDirty()

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: fmt.Sprintf("Configuration loaded from %s", args[0]),
		configView:    &viewportData{content: content, lineMapping: nil},
		revalidate:    true,
	}, nil
}

// cmdLoadMerge loads configuration from a file and merges with current content.
func (m *Model) cmdLoadMerge(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: load merge <file>")
	}

	loadPath := m.resolveConfigPath(args[0])

	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", args[0], err)
	}

	// Merge needs full content (not subtree)
	currentContent := m.editor.WorkingContent()
	mergeContent := string(data)

	merged := mergeConfigs(currentContent, mergeContent)

	m.editor.SetWorkingContent(merged)
	m.editor.MarkDirty()

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: fmt.Sprintf("Configuration merged from %s", args[0]),
		configView:    &viewportData{content: content, lineMapping: nil},
		revalidate:    true,
	}, nil
}

// resolveConfigPath resolves a path relative to the config file directory.
func (m *Model) resolveConfigPath(path string) string {
	if isAbsPath(path) {
		return path
	}
	configDir := getDir(m.editor.OriginalPath())
	return joinPath(configDir, path)
}

// parseLoadArgs parses the new load command syntax: load <source> <location> <action> [file]
// Returns (source, location, action, path, error).
// source: "file" or "terminal"
// location: "absolute" or "relative"
// action: "replace" or "merge"
// path: required when source="file", empty for "terminal"
func parseLoadArgs(args []string) (source, location, action, path string, err error) {
	const usage = "usage: load file|terminal absolute|relative replace|merge [path]"

	if len(args) < 1 {
		return "", "", "", "", fmt.Errorf("missing source (file|terminal). %s", usage)
	}

	source = args[0]
	if source != "file" && source != "terminal" {
		return "", "", "", "", fmt.Errorf("invalid source %q (must be file|terminal). %s", source, usage)
	}

	if len(args) < 2 {
		return "", "", "", "", fmt.Errorf("missing location (absolute|relative). %s", usage)
	}

	location = args[1]
	if location != loadLocationAbsolute && location != loadLocationRelative {
		return "", "", "", "", fmt.Errorf("invalid location %q (must be absolute|relative). %s", location, usage)
	}

	if len(args) < 3 {
		return "", "", "", "", fmt.Errorf("missing action (replace|merge). %s", usage)
	}

	action = args[2]
	if action != loadActionReplace && action != loadActionMerge {
		return "", "", "", "", fmt.Errorf("invalid action %q (must be replace|merge). %s", action, usage)
	}

	if source == "file" {
		if len(args) < 4 {
			return "", "", "", "", fmt.Errorf("missing path for 'load file'. %s", usage)
		}
		path = args[3]
	}

	return source, location, action, path, nil
}

// cmdLoadNew handles the redesigned load command syntax.
// Syntax: load <source> <location> <action> [file].
func (m *Model) cmdLoadNew(args []string) (commandResult, error) {
	source, location, action, path, err := parseLoadArgs(args)
	if err != nil {
		return commandResult{}, err
	}

	// Terminal source enters paste mode
	if source == "terminal" {
		return commandResult{
			statusMessage:     "[Paste mode - Ctrl-D to finish]",
			enterPasteMode:    true,
			pasteModeLocation: location,
			pasteModeAction:   action,
		}, nil
	}

	// File source - read and apply
	loadPath := m.resolveConfigPath(path)
	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	if location == loadLocationAbsolute {
		return m.applyLoadAbsolute(action, string(data), path)
	}
	return m.applyLoadRelative(action, string(data), path)
}

// applyLoadAbsolute applies loaded content at root level.
func (m *Model) applyLoadAbsolute(action, content, path string) (commandResult, error) {
	if action == loadActionReplace {
		m.editor.SetWorkingContent(content)
		m.editor.MarkDirty()
		return commandResult{
			statusMessage: fmt.Sprintf("Configuration loaded from %s", path),
			configView:    &viewportData{content: m.editor.ContentAtPath(m.contextPath), lineMapping: nil},
			revalidate:    true,
		}, nil
	}

	// action == "merge"
	currentContent := m.editor.WorkingContent()
	merged := mergeConfigs(currentContent, content)
	m.editor.SetWorkingContent(merged)
	m.editor.MarkDirty()
	return commandResult{
		statusMessage: fmt.Sprintf("Configuration merged from %s", path),
		configView:    &viewportData{content: m.editor.ContentAtPath(m.contextPath), lineMapping: nil},
		revalidate:    true,
	}, nil
}

// applyLoadRelative applies loaded content at current context position.
func (m *Model) applyLoadRelative(action, content, path string) (commandResult, error) {
	if len(m.contextPath) == 0 {
		// At root level, relative == absolute
		return m.applyLoadAbsolute(action, content, path)
	}

	// Apply at context position
	currentContent := m.editor.WorkingContent()
	var newContent string

	if action == loadActionReplace {
		newContent = replaceAtContext(currentContent, m.contextPath, content)
	} else {
		newContent = mergeAtContext(currentContent, m.contextPath, content)
	}

	m.editor.SetWorkingContent(newContent)
	m.editor.MarkDirty()

	verb := "loaded"
	if action == loadActionMerge {
		verb = "merged"
	}

	return commandResult{
		statusMessage: fmt.Sprintf("Configuration %s from %s at %s", verb, path, strings.Join(m.contextPath, " ")),
		configView:    &viewportData{content: m.editor.ContentAtPath(m.contextPath), lineMapping: nil},
		revalidate:    true,
	}, nil
}

// replaceAtContext replaces the content at the given context path with new content.
func replaceAtContext(fullConfig string, contextPath []string, newContent string) string {
	if len(contextPath) == 0 {
		return fullConfig // nothing to replace
	}

	lines := strings.Split(fullConfig, "\n")
	var result strings.Builder

	// Build the pattern to match (e.g., "peer 1.1.1.1" or just "bgp")
	var targetPattern string
	if len(contextPath) == 1 {
		targetPattern = contextPath[0]
	} else {
		// len >= 2: combine last two elements (e.g., "peer" + "1.1.1.1")
		targetPattern = contextPath[len(contextPath)-2] + " " + contextPath[len(contextPath)-1]
	}

	inTarget := false
	targetDepth := 0
	currentDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		if !inTarget {
			// Looking for target block
			if strings.Contains(trimmed, "{") {
				blockPart := strings.TrimSuffix(trimmed, "{")
				blockPart = strings.TrimSpace(blockPart)

				if blockPart == targetPattern {
					// Found target - write opening line and new content
					result.WriteString(line)
					result.WriteString("\n")
					inTarget = true
					targetDepth = currentDepth + openBraces

					// Write indented new content
					indent := strings.Repeat("  ", targetDepth)
					for _, newLine := range strings.Split(strings.TrimSpace(newContent), "\n") {
						result.WriteString(indent)
						result.WriteString(newLine)
						result.WriteString("\n")
					}
					currentDepth += openBraces - closeBraces
					continue
				}
			}
			result.WriteString(line)
			result.WriteString("\n")
		} else {
			// Inside target - skip old content until closing brace
			newDepth := currentDepth + openBraces - closeBraces
			if newDepth < targetDepth {
				// Found closing brace - write it
				result.WriteString(line)
				result.WriteString("\n")
				inTarget = false
			}
			// Skip old content lines
		}

		currentDepth += openBraces - closeBraces
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// mergeAtContext merges new content into the block at the given context path.
func mergeAtContext(fullConfig string, contextPath []string, newContent string) string {
	if len(contextPath) == 0 {
		return fullConfig // nothing to merge into
	}

	lines := strings.Split(fullConfig, "\n")
	var result strings.Builder

	// Build the pattern to match (e.g., "peer 1.1.1.1" or just "bgp")
	var targetPattern string
	if len(contextPath) == 1 {
		targetPattern = contextPath[0]
	} else {
		// len >= 2: combine last two elements (e.g., "peer" + "1.1.1.1")
		targetPattern = contextPath[len(contextPath)-2] + " " + contextPath[len(contextPath)-1]
	}

	inTarget := false
	targetDepth := 0
	currentDepth := 0
	contentInserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		if !inTarget {
			if strings.Contains(trimmed, "{") {
				blockPart := strings.TrimSuffix(trimmed, "{")
				blockPart = strings.TrimSpace(blockPart)

				if blockPart == targetPattern {
					inTarget = true
					targetDepth = currentDepth + openBraces
				}
			}
			result.WriteString(line)
			result.WriteString("\n")
		} else {
			newDepth := currentDepth + openBraces - closeBraces
			if newDepth < targetDepth && !contentInserted {
				// Insert merged content before closing brace
				indent := strings.Repeat("  ", targetDepth)
				for _, newLine := range strings.Split(strings.TrimSpace(newContent), "\n") {
					result.WriteString(indent)
					result.WriteString(newLine)
					result.WriteString("\n")
				}
				contentInserted = true
				inTarget = false
			}
			result.WriteString(line)
			result.WriteString("\n")
		}

		currentDepth += openBraces - closeBraces
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// cmdShowPipe executes show with pipe filters.
func (m *Model) cmdShowPipe(_ []string, filters []PipeFilter) (commandResult, error) {
	content := m.editor.ContentAtPath(m.contextPath)
	if content == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}

	// Apply pipe filters
	output := content
	for _, filter := range filters {
		var err error
		output, err = applyPipeFilter(output, filter)
		if err != nil {
			return commandResult{}, err
		}
	}

	return commandResult{output: output}, nil
}

// applyPipeFilter applies a single pipe filter to content.
// Returns error for unknown filter types.
func applyPipeFilter(content string, filter PipeFilter) (string, error) {
	lines := strings.Split(content, "\n")

	switch filter.Type {
	case "grep":
		var matched []string
		for _, line := range lines {
			if strings.Contains(line, filter.Arg) {
				matched = append(matched, line)
			}
		}
		return strings.Join(matched, "\n"), nil

	case "head":
		n := 10 // default
		if filter.Arg != "" {
			if parsed, err := parseIntArg(filter.Arg); err == nil && parsed > 0 {
				n = parsed
			}
		}
		if n > len(lines) {
			n = len(lines)
		}
		return strings.Join(lines[:n], "\n"), nil

	case "tail":
		n := 10 // default
		if filter.Arg != "" {
			if parsed, err := parseIntArg(filter.Arg); err == nil && parsed > 0 {
				n = parsed
			}
		}
		if n > len(lines) {
			n = len(lines)
		}
		return strings.Join(lines[len(lines)-n:], "\n"), nil

	case "compare":
		// Compare filter marks each line with + or - based on content
		// This is a simplified version - it just prefixes lines to indicate changes
		// A proper implementation would need the original content to compute a real diff
		var result []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result = append(result, "+ "+line)
			}
		}
		if len(result) == 0 {
			return "(no changes)", nil
		}
		return strings.Join(result, "\n"), nil
	}

	return "", fmt.Errorf("unknown pipe filter: %s", filter.Type)
}

// mergeConfigs merges two configuration strings.
// Simple strategy: use current as base, add non-duplicate blocks/keys from merge.
// Existing keys in current are preserved (merge file's duplicates are skipped).
func mergeConfigs(current, merge string) string {
	currentLines := strings.Split(current, "\n")
	mergeLines := strings.Split(merge, "\n")

	// Extract existing keys from current config at depth 1 (inside main block)
	existingKeys := make(map[string]bool)
	depth := 0
	for _, line := range currentLines {
		trimmed := strings.TrimSpace(line)
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		// At depth 1, extract keys
		if depth == 1 && trimmed != "" && trimmed != "}" {
			key := extractConfigKey(trimmed)
			if key != "" {
				existingKeys[key] = true
			}
		}

		depth += openBraces - closeBraces
	}

	// Find the closing brace of the main block in current and insert merge content before it
	result := make([]string, 0, len(currentLines)+len(mergeLines))
	depth = 0
	inserted := false
	mergeDepth := 0
	skipUntilClose := false

	for i, line := range currentLines {
		trimmed := strings.TrimSpace(line)
		depth += strings.Count(trimmed, "{")
		depth -= strings.Count(trimmed, "}")

		// If we're about to close the main block and haven't inserted yet
		if depth == 0 && strings.Contains(trimmed, "}") && !inserted {
			// Insert merge content, skipping duplicates
			for _, mergeLine := range mergeLines {
				mergeTrimmed := strings.TrimSpace(mergeLine)

				// Track depth in merge content
				mergeOpenBraces := strings.Count(mergeTrimmed, "{")
				mergeCloseBraces := strings.Count(mergeTrimmed, "}")

				// Skip top-level block markers
				if mergeTrimmed == "" || mergeTrimmed == "bgp {" || mergeTrimmed == "}" {
					mergeDepth += mergeOpenBraces - mergeCloseBraces
					continue
				}

				// If we're skipping a duplicate block, continue until it closes
				if skipUntilClose {
					mergeDepth += mergeOpenBraces - mergeCloseBraces
					if mergeDepth <= 1 {
						skipUntilClose = false
					}
					continue
				}

				// At depth 1 in merge, check if key already exists
				if mergeDepth == 1 {
					key := extractConfigKey(mergeTrimmed)
					if key != "" && existingKeys[key] {
						// Skip this key/block - it already exists in current
						if mergeOpenBraces > 0 {
							skipUntilClose = true
						}
						mergeDepth += mergeOpenBraces - mergeCloseBraces
						continue
					}
				}

				mergeDepth += mergeOpenBraces - mergeCloseBraces
				result = append(result, mergeLine)
			}
			inserted = true
		}

		result = append(result, currentLines[i])
	}

	return strings.Join(result, "\n")
}

// extractConfigKey extracts the key from a config line.
// For "router-id 1.2.3.4;" returns "router-id".
// For "peer 1.1.1.1 {" returns "peer 1.1.1.1".
// For "local-as 65000;" returns "local-as".
func extractConfigKey(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, "{")
	line = strings.TrimSuffix(line, ";")
	line = strings.TrimSpace(line)

	// Split into words
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}

	// For leaf values like "router-id 1.2.3.4", the key is "router-id"
	// For blocks like "peer 1.1.1.1", the key is "peer 1.1.1.1"
	// Heuristic: if there are 2 parts and first is a known block keyword, use both
	if len(parts) >= 2 {
		first := parts[0]
		// Known block keywords that take a key value
		blockKeywords := map[string]bool{
			"peer": true, "template": true, "plugin": true, "process": true, "group": true,
		}
		if blockKeywords[first] {
			return first + " " + parts[1]
		}
	}

	// Default: just use the first word as the key
	return parts[0]
}

// findPipeIndex returns the index of "|" in tokens, or -1 if not found.
func findPipeIndex(tokens []string) int {
	for i, t := range tokens {
		if t == "|" {
			return i
		}
	}
	return -1
}

// dispatchWithPipe handles commands with pipe filters.
func (m *Model) dispatchWithPipe(cmdTokens, pipeTokens []string) (commandResult, error) {
	if len(cmdTokens) == 0 {
		return commandResult{}, fmt.Errorf("no command before pipe")
	}

	// Parse pipe filters
	filters := parsePipeFilters(pipeTokens)

	// Only show supports piping currently
	cmd := cmdTokens[0]
	switch cmd {
	case "show":
		return m.cmdShowPipe(cmdTokens[1:], filters)
	case "errors":
		result, err := m.cmdErrors()
		if err != nil {
			return result, err
		}
		// Apply filters to errors output
		for _, f := range filters {
			result.output, err = applyPipeFilter(result.output, f)
			if err != nil {
				return commandResult{}, err
			}
		}
		return result, nil
	}

	return commandResult{}, fmt.Errorf("command '%s' does not support piping", cmd)
}

// parsePipeFilters parses pipe filter tokens into PipeFilter structs.
func parsePipeFilters(tokens []string) []PipeFilter {
	var filters []PipeFilter
	i := 0

	for i < len(tokens) {
		if tokens[i] == "|" {
			i++
			continue
		}

		filter := PipeFilter{Type: tokens[i]}
		i++

		// Get argument if present
		if i < len(tokens) && tokens[i] != "|" {
			filter.Arg = tokens[i]
			i++
		}

		filters = append(filters, filter)
	}

	return filters
}

// Helper functions wrapping standard library calls
// These use os, filepath, strconv packages.

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // Path comes from user input in editor context
}

func isAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

func getDir(path string) string {
	return filepath.Dir(path)
}

func joinPath(base, path string) string {
	return filepath.Join(base, path)
}

func parseIntArg(s string) (int, error) {
	return strconv.Atoi(s)
}
