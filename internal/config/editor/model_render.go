package editor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
)

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
