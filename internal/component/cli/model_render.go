// Design: docs/architecture/config/yang-config-design.md — config editor
// Related: model_mode.go — mode-aware prompt rendering
// Related: diff.go — line-based LCS diff for gutter annotation
// Related: diff_tree.go — tree-aware diff using YANG schema

package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
)

// setViewportData sets content with line mapping in the viewport.
// When originalContent is provided and differs from content, a diff gutter
// is prepended to each line showing change markers: ' ' unchanged, '+' added,
// '-' removed, '*' modified. The line mapping is adjusted so validation
// highlighting still finds the correct lines.
//
// When a YANG schema is available and we're at root context, uses tree-aware diff
// that respects container boundaries (solving LCS brace-misalignment). Falls back
// to line-based LCS diff for subtrees or when schema is unavailable.
func (m *Model) setViewportData(data viewportData) {
	content := data.content
	lineMapping := data.lineMapping

	// Apply diff gutter when original was explicitly provided, content differs,
	// and the changes column is enabled. The changes column controls all change indicators
	// (both diff gutter markers and annotated column markers).
	changesEnabled := data.forceChanges || !m.hasEditor() || m.editor.DiffGutterEnabled()
	if changesEnabled && data.hasOriginal && data.originalContent != data.content {
		if m.hasEditor() && m.editor.schema != nil && len(m.contextPath) == 0 {
			content, lineMapping = annotateContentWithTreeDiff(data.originalContent, data.content, m.editor.schema)
		} else {
			content, lineMapping = annotateContentWithGutter(data.originalContent, data.content)
		}
	}

	highlighted := highlightValidationIssues(content, m.validationErrors, m.validationWarnings, lineMapping, m.showHints)
	m.viewportContent = highlighted
	m.viewport.SetContent(highlighted)
	m.viewport.GotoTop()
	m.showViewport = true
	m.err = nil
}

// configViewAtPath builds a viewportData for the config at the given path,
// including original content for diff gutter annotation.
func (m *Model) configViewAtPath(path []string) *viewportData {
	content := m.editor.ContentAtPath(path)
	original := m.editor.OriginalContentAtPath(path)
	return &viewportData{
		content:         content,
		originalContent: original,
		hasOriginal:     true,
	}
}

// setViewportText sets simple text content without line mapping.
// Use for non-config content like diffs, history, or messages.
// Skips validation highlighting since this is not config content.
func (m *Model) setViewportText(content string) {
	m.viewportContent = content
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.showViewport = true
	m.err = nil
}

// highlightValidationIssues adds styling to lines with validation errors or warnings.
// Errors are highlighted in red with inline message, warnings in yellow with inline message.
// lineMapping maps filtered line numbers to original line numbers (used when showing filtered content).
func highlightValidationIssues(content string, errors, warnings []ConfigValidationError, lineMapping map[int]int, showHints bool) string {
	if len(errors) == 0 && len(warnings) == 0 {
		return content
	}

	// Build maps: line number → short diagnostic message (1-based, in original content).
	// Errors take precedence over warnings on the same line.
	errorMsgs := make(map[int]string)
	for _, e := range errors {
		if e.Line > 0 {
			errorMsgs[e.Line] = shortDiagnostic(e.Message)
		}
	}

	warningMsgs := make(map[int]string)
	for _, w := range warnings {
		if w.Line > 0 && errorMsgs[w.Line] == "" {
			warningMsgs[w.Line] = shortDiagnostic(w.Message)
		}
	}

	if len(errorMsgs) == 0 && len(warningMsgs) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		filteredLineNum := i + 1 // Convert to 1-based

		// Determine the original line number to check
		var origLineNum int
		if lineMapping != nil {
			origLineNum = lineMapping[filteredLineNum]
		} else {
			origLineNum = filteredLineNum
		}

		if origLineNum > 0 {
			if msg, ok := errorMsgs[origLineNum]; ok {
				styled := errorLineStyle.Render(line)
				if showHints {
					styled += dimStyle.Render("  ← " + msg)
				}
				lines[i] = styled
			} else if msg, ok := warningMsgs[origLineNum]; ok {
				styled := warningLineStyle.Render(line)
				if showHints {
					styled += dimStyle.Render("  ← " + msg)
				}
				lines[i] = styled
			}
		}
	}

	return strings.Join(lines, "\n")
}

// shortDiagnostic extracts a concise message for inline display.
// e.g. `peer 1.1.1.1: missing required field "remote as"` → `missing: remote as`
// e.g. `hold-time must be 0 or >= 3` → kept as-is.
func shortDiagnostic(msg string) string {
	// Strip "peer X.X.X.X: " prefix if present
	if idx := strings.Index(msg, ": "); idx >= 0 {
		msg = msg[idx+2:]
	}
	// Shorten "missing required field "X"" → "missing: X"
	if strings.HasPrefix(msg, "missing required field") {
		if start := strings.IndexByte(msg, '"'); start >= 0 {
			if end := strings.IndexByte(msg[start+1:], '"'); end >= 0 {
				return "missing: " + msg[start+1:start+1+end]
			}
		}
	}
	return msg
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

	// Layout: viewport at top (filling available space), message area + prompt at bottom.
	// 3 = message area (2 lines) + prompt
	const bottomRows = 3
	var lines []string

	// Viewport for scrollable content — fills all space above the bottom rows
	if m.showViewport && m.viewportContent != "" {
		m.viewport.Height = max(viewHeight-bottomRows, 5)
		vpLines := strings.Split(m.viewport.View(), "\n")
		lines = append(lines, vpLines...)
	}

	// Pad between viewport and bottom area
	for len(lines) < viewHeight-bottomRows {
		lines = append(lines, "")
	}

	// Message area (2 lines) + prompt — always at the bottom
	msg1, msg2 := m.messageLines()
	lines = append(lines, msg1, msg2, m.buildPrompt()+m.renderInputWithGhost())

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

// messageLines returns the two lines for the message area above the prompt.
// Priority: error (red) > status message > idle info (header/tips).
func (m Model) messageLines() (string, string) {
	// Error takes top priority — show in red across both lines if needed
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error()), ""
	}

	// Status message from last command (e.g., "Configuration committed")
	if m.statusMessage != "" {
		if strings.HasPrefix(m.statusMessage, "welcome") {
			return m.idleInfoLine(), welcomeStyle.Render(m.statusMessage)
		}
		return successStyle.Render("► " + m.statusMessage), m.idleInfoLine()
	}

	// Idle: show header info + validation status
	return m.idleInfoLine(), m.validationHintLine()
}

// idleInfoLine returns the default info line shown when there's no error or status.
func (m Model) idleInfoLine() string {
	var info string
	if m.editor != nil {
		info = "Ze Editor [" + m.mode.String() + "]"
		if m.editor.Dirty() {
			info += " [modified]"
		}
	} else {
		info = "Ze CLI [" + m.mode.String() + "]"
	}

	// Validation indicator
	if len(m.validationErrors) > 0 {
		info += errorStyle.Render(fmt.Sprintf(" %d error(s)", len(m.validationErrors)))
	} else if len(m.validationWarnings) > 0 {
		info += dimStyle.Render(fmt.Sprintf(" %d warning(s)", len(m.validationWarnings)))
	}

	info += dimStyle.Render("  (Tab/?: complete, Enter: execute, Esc: quit)")
	return dimStyle.Render(info)
}

// validationHintLine returns a brief summary of validation issues when idle.
// Helps the user understand why lines are highlighted (red=error, yellow=warning).
func (m Model) validationHintLine() string {
	if len(m.validationErrors) == 0 && len(m.validationWarnings) == 0 {
		return ""
	}
	hint := "  red=error, yellow=missing field — 'errors' for details, 'show' for config"
	return dimStyle.Render(hint)
}

// overlayDropdown renders the dropdown as a floating overlay on the base view.
// The dropdown is positioned above the prompt line to avoid covering the typed command.
func (m Model) overlayDropdown(base string) string {
	// Find the prompt line position
	baseLines := strings.Split(base, "\n")
	promptLineIdx := len(baseLines) - 1
	for promptLineIdx > 0 && strings.TrimSpace(baseLines[promptLineIdx]) == "" {
		promptLineIdx--
	}

	// Available space above the prompt (lines 0..promptLineIdx-1)
	availableAbove := max(promptLineIdx, 3)

	dropdown := m.renderDropdownBox(availableAbove)

	// Position dropdown above the prompt line
	dropdownHeight := strings.Count(dropdown, "\n") + 1
	y := max(promptLineIdx-dropdownHeight, 0)
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
// availableHeight is the number of screen lines available for the entire dropdown
// (including borders). Uses plain text (no ANSI) for consistent width calculations.
func (m Model) renderDropdownBox(availableHeight int) string {
	// Compute max visible items from available height:
	// 2 lines for borders (top + bottom), 1 line for "more" indicator if truncated.
	maxItems := availableHeight - 2
	if len(m.completions) > maxItems && maxItems > 1 {
		maxItems-- // Reserve line for "... N more" indicator
	}
	if maxItems < 1 {
		maxItems = 1
	}
	maxShow := min(maxItems, len(m.completions))

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

// renderHelpOverlay renders help as a floating overlay.
// Shows full editor help when an editor is attached, or command-only help otherwise.
func (m Model) renderHelpOverlay(base string) string {
	var help string
	if m.hasEditor() {
		help = `Commands:
  set <path> <value>   Set a configuration value
  delete <path>        Delete a configuration value
  edit <path>          Enter a subsection context
  edit <list> *        Edit template for all entries
  top                  Return to root context
  up                   Go up one level
  show                 Display configuration (scrollable)
  show <col> enable    Enable display column (author/date/source/changes)
  show <col> disable   Disable display column
  show all / none      Enable/disable all columns
  show | format config Display as set commands
  show | compare       Diff against committed config
  compare              Show diff vs original
  commit               Save changes with backup
  commit confirmed <N> Save with auto-revert after N seconds
  confirm              Make pending commit permanent
  abort                Cancel pending commit and roll back
  discard              Revert all changes
  history              List backup files
  rollback <N>         Restore backup N
  exit                 Exit editor

Modes:
  run                  Switch to operational command mode
  run <cmd>            Switch to command mode and execute <cmd>
  (config commands in command mode auto-switch back to edit mode)

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
	} else {
		help = `Commands:
  Type operational commands (e.g., peer list, daemon status)
  Use pipe operators to format output:
    <cmd> | table          Render as table (default)
    <cmd> | json           Pretty-print JSON
    <cmd> | json compact   Single-line JSON
    <cmd> | match <pat>    Filter lines matching pattern
    <cmd> | count          Count output lines
  exit                     Exit CLI

Keys:
  Tab                  Complete / cycle suggestions
  ↑↓                   Navigate dropdown / browse history
  Shift+↑↓             Scroll viewport one line
  Ctrl+↑↓ / PgUp/PgDn Scroll viewport one page
  Enter                Execute command / accept selection
  Esc                  Close overlay / quit

Press Esc to close this help.`
	}

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
	if m.mode == ModeCommand {
		return promptStyle.Render("ze> ")
	}

	if len(m.contextPath) == 0 {
		return promptStyle.Render("ze# ")
	}

	contextStr := strings.Join(m.contextPath, " ")
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
