package editor

import (
	"fmt"
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
)

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
	err             error
	width           int
	height          int
	quitting        bool
}

// Debounce delay for validation after keystroke.
const validationDebounce = 100 * time.Millisecond

// Message types for the editor.
type (
	executeResultMsg struct {
		output string
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
func NewModel(ed *Editor) Model {
	ti := textinput.New()
	ti.Placeholder = "type command or Tab for suggestions"
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 60

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	comp := NewCompleter(ed.Schema())
	comp.SetTree(ed.Tree())

	val := NewConfigValidator()

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
	}
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
			content := m.editor.WorkingContent()
			if content != "" {
				m.setViewportContent(content)
			}
		}
		return m, nil

	case executeResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else if msg.output != "" {
			m.setViewportContent(msg.output)
		}
		return m, nil

	case successMsg:
		m.err = nil
		return m, nil

	case errorMsg:
		m.err = msg.err
		return m, nil

	case outputMsg:
		m.setViewportContent(msg.text)
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

// applyCompletion applies a completion to the input.
func (m *Model) applyCompletion(comp Completion) {
	input := m.textInput.Value()
	words := strings.Fields(input)

	if len(words) > 0 && !strings.HasSuffix(input, " ") {
		// Replace last partial word
		words[len(words)-1] = comp.Text
		m.textInput.SetValue(strings.Join(words, " ") + " ")
	} else {
		// Append completion
		m.textInput.SetValue(input + comp.Text + " ")
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

// setViewportContent sets content in the viewport and shows it.
func (m *Model) setViewportContent(content string) {
	m.viewportContent = content
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.showViewport = true
	m.err = nil
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
	maxShow := 6
	if len(m.completions) < maxShow {
		maxShow = len(m.completions)
	}

	// Calculate scroll offset
	start := 0
	if m.selected >= maxShow {
		start = m.selected - maxShow + 1
	}
	end := start + maxShow
	if end > len(m.completions) {
		end = len(m.completions)
		start = end - maxShow
		if start < 0 {
			start = 0
		}
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
func (m Model) executeCommand(input string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.dispatchCommand(input)
		if err != nil {
			return executeResultMsg{err: err}
		}
		return executeResultMsg{output: result}
	}
}

// dispatchCommand parses and executes a command.
func (m *Model) dispatchCommand(input string) (string, error) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return "", nil
	}

	cmd := tokens[0]
	args := tokens[1:]

	switch cmd {
	case "exit", "quit":
		return "", nil // Will be handled by quit logic

	case "help", "?":
		m.showHelp = true
		return "", nil

	case "top":
		m.contextPath = nil
		m.isTemplate = false
		// Show full config at root
		content, _ := m.cmdShow(nil)
		if content != "" {
			m.setViewportContent(content)
		}
		return "", nil

	case "up":
		if len(m.contextPath) > 0 {
			m.contextPath = m.contextPath[:len(m.contextPath)-1]
		}
		if len(m.contextPath) == 0 {
			m.isTemplate = false
		}
		// Refresh display for new context
		content, _ := m.cmdShow(nil)
		if content != "" {
			m.setViewportContent(content)
		}
		return "", nil

	case "edit":
		return m.cmdEdit(args)

	case "show":
		return m.cmdShow(args)

	case "compare":
		return m.editor.Diff(), nil

	case "commit":
		return m.cmdCommit()

	case "discard":
		if err := m.editor.Discard(); err != nil {
			return "", err
		}
		m.runValidation() // Re-validate after discard
		return "Changes discarded", nil

	case "history":
		return m.cmdHistory()

	case "rollback":
		return m.cmdRollback(args)

	case "set":
		return m.cmdSet(args)

	case "delete":
		return m.cmdDelete(args)

	case "errors":
		return m.cmdErrors()

	default:
		return "", fmt.Errorf("unknown command: %s", cmd)
	}
}

// Command implementations

func (m *Model) cmdEdit(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: edit <path>")
	}

	// Check for wildcard template
	if len(args) >= 2 && args[len(args)-1] == "*" {
		m.contextPath = args[:len(args)-1]
		m.contextPath = append(m.contextPath, "*")
		m.isTemplate = true
	} else {
		m.contextPath = args
		m.isTemplate = false
	}

	// Automatically show the section we're editing
	content, _ := m.cmdShow(nil)
	if content != "" {
		m.setViewportContent(content)
	}

	return "", nil // No separate message, viewport shows the section
}

func (m *Model) cmdShow(_ []string) (string, error) {
	content := m.editor.WorkingContent()
	if content == "" {
		return "(empty configuration)", nil
	}

	// If we have a context path, try to show only that section
	if len(m.contextPath) > 0 {
		filtered := m.filterContentByContext(content)
		if filtered != "" {
			return filtered, nil
		}
	}

	return content, nil
}

// filterContentByContext extracts the inner content of the section matching the current context.
func (m *Model) filterContentByContext(content string) string {
	if len(m.contextPath) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	var result strings.Builder
	depth := 0
	inSection := false
	sectionDepth := 0

	// Build the pattern to match (e.g., "neighbor 127.0.0.1 {")
	pattern := m.contextPath[0]
	if len(m.contextPath) > 1 && m.contextPath[1] != "*" {
		pattern += " " + m.contextPath[1]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track brace depth
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		if !inSection {
			// Look for start of our section
			if strings.HasPrefix(trimmed, pattern) && strings.Contains(trimmed, "{") {
				inSection = true
				sectionDepth = depth + openBraces
				// Don't include the opening line
			}
		} else {
			// Check if we've exited our section (closing brace)
			newDepth := depth + openBraces - closeBraces
			if newDepth < sectionDepth {
				break // Don't include closing brace
			}

			// We're in our section - include this line (dedented)
			// Remove one level of indentation
			dedented := strings.TrimPrefix(line, "\t")
			dedented = strings.TrimPrefix(dedented, "    ")
			if strings.TrimSpace(dedented) != "" {
				result.WriteString(dedented)
				result.WriteString("\n")
			}
		}

		depth += openBraces - closeBraces
	}

	return result.String()
}

func (m *Model) cmdHistory() (string, error) {
	backups, err := m.editor.ListBackups()
	if err != nil {
		return "", err
	}

	if len(backups) == 0 {
		return "No backups found", nil
	}

	var b strings.Builder
	for i, backup := range backups {
		b.WriteString(fmt.Sprintf("%d. %s (%s)\n",
			i+1,
			backup.Path,
			backup.Timestamp.Format("2006-01-02")))
	}
	return b.String(), nil
}

func (m *Model) cmdRollback(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("usage: rollback <number>")
	}

	var n int
	if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil {
		return "", fmt.Errorf("invalid backup number: %s", args[0])
	}

	backups, err := m.editor.ListBackups()
	if err != nil {
		return "", err
	}

	if n < 1 || n > len(backups) {
		return "", fmt.Errorf("backup %d not found (have %d backups)", n, len(backups))
	}

	if err := m.editor.Rollback(backups[n-1].Path); err != nil {
		return "", err
	}

	return fmt.Sprintf("Rolled back to %s", backups[n-1].Path), nil
}

func (m *Model) cmdSet(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: set <path> <value>")
	}

	// Build full path with context
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// For now, just acknowledge - actual tree modification needs SetParser integration
	m.editor.MarkDirty()
	return fmt.Sprintf("Set %s", strings.Join(fullPath, " ")), nil
}

func (m *Model) cmdDelete(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: delete <path>")
	}

	// Build full path with context
	fullPath := make([]string, 0, len(m.contextPath)+len(args))
	fullPath = append(fullPath, m.contextPath...)
	fullPath = append(fullPath, args...)

	// For now, just acknowledge
	m.editor.MarkDirty()
	return fmt.Sprintf("Deleted %s", strings.Join(fullPath, " ")), nil
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
func (m *Model) cmdCommit() (string, error) {
	// Re-run validation before commit
	m.runValidation()

	// Block commit if there are errors
	if len(m.validationErrors) > 0 {
		return "", fmt.Errorf("cannot commit: %d validation error(s). Use 'errors' to see details", len(m.validationErrors))
	}

	// Save changes
	if err := m.editor.Save(); err != nil {
		return "", err
	}

	return "Configuration committed", nil
}

// cmdErrors displays validation errors.
func (m *Model) cmdErrors() (string, error) {
	if len(m.validationErrors) == 0 && len(m.validationWarnings) == 0 {
		return "No validation issues", nil
	}

	var b strings.Builder

	if len(m.validationErrors) > 0 {
		b.WriteString(fmt.Sprintf("Errors (%d):\n", len(m.validationErrors)))
		for _, e := range m.validationErrors {
			if e.Line > 0 {
				b.WriteString(fmt.Sprintf("  Line %d: %s\n", e.Line, e.Message))
			} else {
				b.WriteString(fmt.Sprintf("  %s\n", e.Message))
			}
		}
	}

	if len(m.validationWarnings) > 0 {
		if len(m.validationErrors) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("Warnings (%d):\n", len(m.validationWarnings)))
		for _, w := range m.validationWarnings {
			if w.Line > 0 {
				b.WriteString(fmt.Sprintf("  Line %d: %s\n", w.Line, w.Message))
			} else {
				b.WriteString(fmt.Sprintf("  %s\n", w.Message))
			}
		}
	}

	return b.String(), nil
}
