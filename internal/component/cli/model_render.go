// Design: docs/architecture/config/yang-config-design.md — config editor
// Overview: model.go — Model definition, Update loop, key dispatch
// Related: model_mode.go — mode-aware prompt rendering
// Related: diff.go — line-based LCS diff for gutter annotation
// Related: diff_tree.go — tree-aware diff using YANG schema

package cli

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/muesli/reflow/ansi"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sanitizeForDisplay strips or escapes non-printable characters and ANSI escape
// sequences from config content before terminal rendering. This prevents raw
// escape codes in config values from corrupting the TUI display.
//
// Preserved: tab (0x09), newline (0x0A), carriage return (0x0D).
// Stripped: C0 controls (0x00-0x08, 0x0B-0x0C, 0x0E-0x1F), DEL (0x7F),
// C1 controls (0x80-0x9F), and ANSI escape sequences (\x1b[...X, \x1b]...ST).
func sanitizeForDisplay(s string) string {
	// Fast path: if no suspicious bytes, return as-is.
	hasSuspicious := false
	for i := range len(s) {
		b := s[i]
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			hasSuspicious = true
			break
		}
		if b == 0x7F || b == 0x1b {
			hasSuspicious = true
			break
		}
		// C1 control range: 0x80-0x9F (first byte of UTF-8 encoding)
		if b >= 0x80 && b <= 0x9F {
			hasSuspicious = true
			break
		}
	}
	if !hasSuspicious {
		return s
	}

	var b textbuf.Buffer
	b.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Strip ANSI escape sequences: ESC [ ... final_byte or ESC ] ... ST
		if r == 0x1b && i+1 < len(runes) {
			next := runes[i+1]
			if next == '[' {
				// CSI sequence: ESC [ (params) final_byte (0x40-0x7E)
				i += 2 // skip ESC [
				for i < len(runes) && (runes[i] < 0x40 || runes[i] > 0x7E) {
					i++
				}
				// skip the final byte
				continue
			}
			if next == ']' {
				// OSC sequence: ESC ] ... (ST = ESC \ or BEL)
				i += 2 // skip ESC ]
				for i < len(runes) {
					if runes[i] == 0x07 { // BEL terminates OSC
						break
					}
					if runes[i] == 0x1b && i+1 < len(runes) && runes[i+1] == '\\' {
						i++ // skip the backslash too
						break
					}
					i++
				}
				continue
			}
			// Other ESC sequences (e.g., ESC ( B): skip ESC + next char
			i++
			continue
		}

		// Allow tab, newline, carriage return
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}

		// Strip C0 controls (0x00-0x1F), DEL (0x7F), C1 controls (0x80-0x9F)
		if r < 0x20 || r == 0x7F || (r >= 0x80 && r <= 0x9F) {
			b.WriteRune(unicode.ReplacementChar)
			continue
		}

		b.WriteRune(r)
	}

	return b.String()
}

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
	content := sanitizeForDisplay(data.content)
	data.originalContent = sanitizeForDisplay(data.originalContent)
	lineMapping := data.lineMapping

	// Apply diff gutter when original was explicitly provided, content differs,
	// and the changes column is enabled. The changes column controls all change indicators
	// (both diff gutter markers and annotated column markers).
	changesEnabled := data.forceChanges || !m.hasEditor() || m.editor.diffGutterEnabled()
	if changesEnabled && data.hasOriginal && data.originalContent != data.content {
		if m.hasEditor() && m.editor.schema != nil && len(m.contextPath) == 0 {
			content, lineMapping = annotateContentWithTreeDiff(data.originalContent, data.content, m.editor.schema)
		} else {
			content, lineMapping = annotateContentWithGutter(data.originalContent, data.content)
		}
	}

	// Validation line numbers index the validated content (runValidation validates
	// ContentAtPath(nil)). A view that renders a different string cannot position
	// them, so it opts out rather than styling an unrelated line.
	errs, warns := m.validationErrors, m.validationWarnings
	if data.noValidationHighlight {
		errs, warns = nil, nil
	}

	highlighted := highlightValidationIssues(content, errs, warns, lineMapping, m.showHints)
	if changesEnabled && data.hasOriginal {
		highlighted += secretChangeLines(data.secretChanges)
	}
	m.viewportContent = highlighted
	m.viewport.SetContent(highlighted)
	m.viewport.GotoTop()
	m.showViewport = true
	m.showingConfig = true
	m.err = nil
}

// secretChangeLines names each secret leaf whose value moved, one per line, and
// publishes neither value. It is appended after the highlighting, so it adds no
// display line the validation line mapping has to account for.
//
// Each line comes from secretChangeLine (editor_mask.go), which the commit diff
// also writes: one property, one wording, on every surface.
func secretChangeLines(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	var b textbuf.Buffer
	for _, leafPath := range paths {
		b.Byte('\n').Str(secretChangeLine(leafPath))
	}
	return b.String()
}

// configViewAtPath builds a viewportData for the config at the given path,
// including original content for diff gutter annotation.
func (m *Model) configViewAtPath(path []string) *viewportData {
	// Display-masked: every secret leaf is hidden in the config view. Both sides
	// are masked so the diff gutter compares masked-against-masked. Validation
	// still runs on the unmasked ContentAtPath elsewhere; masking is
	// line-preserving so validation highlight line numbers still align.
	//
	// A rotated secret is invisible to that comparison, which is why the changed
	// paths travel beside the text rather than inside it.
	content := m.editor.DisplayContentAtPath(path)
	original := m.editor.DisplayOriginalContentAtPath(path)
	return &viewportData{
		content:         content,
		originalContent: original,
		hasOriginal:     true,
		secretChanges:   m.editor.changedSecretsAt(path),
	}
}

// setViewportText sets simple text content without line mapping.
// Use for non-config content like diffs, history, or messages.
// Skips validation highlighting since this is not config content.
func (m *Model) setViewportText(content string) {
	content = sanitizeForDisplay(content)
	m.viewportContent = content
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.showViewport = true
	m.showingConfig = false
	m.err = nil
}

// setViewportTextBottom sets content bottom-aligned in the viewport.
// Prepends blank lines so content hugs the bottom when shorter than the viewport.
func (m *Model) setViewportTextBottom(content string) {
	content = sanitizeForDisplay(content)

	h := m.height
	if h < 10 {
		h = 24
	}
	vpHeight := max(h-3, 5)
	contentLines := strings.Count(content, "\n") + 1
	if pad := vpHeight - contentLines; pad > 0 {
		content = strings.Repeat("\n", pad) + content
	}

	m.viewportContent = content
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
	m.showViewport = true
	m.showingConfig = false
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
			var tb textbuf.Buffer
			if msg, ok := errorMsgs[origLineNum]; ok {
				styled := errorLineStyle.Render(line)
				if showHints {
					lines[i] = tb.Str(styled).Str(dimStyle.Render(tb.Reset().Str("  ← ").Str(msg).String())).String()
				} else {
					lines[i] = styled
				}
			} else if msg, ok := warningMsgs[origLineNum]; ok {
				styled := warningLineStyle.Render(line)
				if showHints {
					lines[i] = tb.Str(styled).Str(dimStyle.Render(tb.Reset().Str("  ← ").Str(msg).String())).String()
				} else {
					lines[i] = styled
				}
			}
		}
	}

	return textbuf.Join(lines, "\n")
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
				var tb textbuf.Buffer
				return tb.Str("missing: ").Str(msg[start+1 : start+1+end]).String()
			}
		}
	}
	return msg
}

// altView creates a tea.View with AltScreen enabled and optional hardware cursor.
func altView(s string, cursor *tea.Cursor) tea.View {
	v := tea.NewView(s)
	v.AltScreen = true
	v.Cursor = cursor
	return v
}

// paddedAltView creates an alt-screen view with 1 row top padding and 1 col
// left padding so full-screen content aligns with the viewport border position.
func paddedAltView(s string) tea.View {
	lines := strings.Split(s, "\n")
	var b textbuf.Buffer
	b.Reset(len(s) + len(lines) + 2)
	b.Byte('\n')
	for i, line := range lines {
		b.Byte(' ')
		b.Str(line)
		if i < len(lines)-1 {
			b.Byte('\n')
		}
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	// Full-screen monitor session (any streaming command with RenderFunc).
	if m.monitorSession != nil && m.monitorSession.RenderFunc != nil {
		return paddedAltView(m.monitorSession.RenderFunc(m.width, m.height))
	}

	// Active live view (dashboard / ping / traceroute) renders its own full
	// screen. A "" result means the view is in a scrollback ("| log") mode and
	// falls through to the normal viewport render below.
	if m.activeView != nil {
		if content := m.activeView.render(&m); content != "" {
			// The view states its own subject and nothing else.
			//
			// A fault it reports is rendered HERE, in the one error zone
			// every surface shares. A reader then looks in the same place
			// whatever command they ran
			// (docs/architecture/cli/error-surface.md).
			if problem := m.activeView.problem(&m); problem != "" {
				var tb textbuf.Buffer
				content = tb.Str(content).Byte('\n').Str(errorStyle.Render(problem)).String()
			}
			return paddedAltView(content)
		}
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
	if m.showViewport {
		m.viewport.SetHeight(max(viewHeight-bottomRows, 5))
		vpLines := strings.Split(m.viewport.View(), "\n")
		lines = append(lines, vpLines...)
	}

	// Login warnings (rendered above message area, consuming padding space)
	var warningLines []string
	for _, w := range m.loginWarnings {
		if w.Message == "" {
			continue
		}
		warningLines = append(warningLines, warningLineStyle.Render("warning: "+w.Message))
		if w.Command != "" {
			warningLines = append(warningLines, warningLineStyle.Render("  run: "+w.Command))
		}
	}

	// Pad between viewport and bottom area
	bottomUsed := bottomRows + len(warningLines)
	for len(lines) < viewHeight-bottomUsed {
		lines = append(lines, "")
	}

	// Warning lines above message area
	lines = append(lines, warningLines...)

	// Message area (2 lines) + prompt — always at the bottom
	msg1, msg2 := m.messageLines()
	lines = append(lines, msg1, msg2, m.buildPrompt()+m.renderInputWithGhost())
	promptLine := len(lines) - 1

	// Truncate if too many lines (login warnings can push total past viewHeight)
	if len(lines) > viewHeight {
		lines = lines[:viewHeight]
	}

	baseView := textbuf.Join(lines, "\n")

	// Overlay dropdown if showing
	if m.showDropdown && len(m.completions) > 0 {
		return altView(m.overlayDropdown(baseView), nil)
	}

	// The explanation region: what Tab reveals once the candidate list is
	// exhausted (model_help_level.go).
	if m.revealLevel() == revealExplanation {
		return altView(m.overlayExplanation(baseView), nil)
	}

	// Help overlay
	if m.showHelp {
		return altView(m.renderHelpOverlay(baseView), nil)
	}

	// Hardware cursor on the prompt line so the terminal handles blink
	// natively without re-rendering (which breaks SSH text selection).
	// Hide cursor if the prompt was truncated (login warnings on small terminal).
	var cursor *tea.Cursor
	if c := m.textInput.Cursor(); c != nil && promptLine < len(lines) {
		promptWidth := ansi.PrintableRuneWidth(m.buildPrompt())
		cur := tea.NewCursor(c.X+promptWidth, promptLine)
		cur.Blink = c.Blink
		cur.Shape = c.Shape
		cur.Color = c.Color
		cursor = cur
	}

	return altView(baseView, cursor)
}

// messageLines returns the two lines for the message area above the prompt.
//
// Line 1 (top): command feedback — the welcome banner, a command result, or the
// error. An error owns this row, so nothing on line 2 can hide one.
// Line 2 (bottom): help, in this order — the ? hint, the selected candidate's
// summary while the menu is open, the validation hint, the idle banner.
func (m Model) messageLines() (string, string) {
	// Line 1: command feedback
	line1 := m.feedbackLine()

	// Line 2: help and hints
	line2 := m.warningLine()

	return line1, line2
}

// feedbackLine returns the top message line: command results and status.
func (m Model) feedbackLine() string {
	if m.err != nil {
		var tb textbuf.Buffer
		return errorStyle.Render(tb.Str("Error: ").Err(m.err).String())
	}
	if m.statusMessage != "" {
		if strings.HasPrefix(m.statusMessage, "welcome") {
			return welcomeStyle.Render(m.statusMessage)
		}
		style := successStyle
		switch {
		case strings.HasPrefix(m.statusMessage, "commit blocked"):
			style = errorStyle
		case strings.HasPrefix(m.statusMessage, "Quit?"),
			strings.HasPrefix(m.statusMessage, "Pending changes"):
			style = warnStyle
		}
		var tb textbuf.Buffer
		return style.Render(tb.Str("► ").Str(m.statusMessage).String())
	}
	// Idle line 1 is BLANK. The idle banner is help, so it belongs to line 2.
	//
	// This used to fall through to idleInfoLine as well. With no error, no
	// status message and no hint, both lines then rendered the same string.
	// That is the state a session is in the moment it leaves configuration
	// mode, so the banner was printed twice above the prompt.
	return ""
}

// warningLine returns the bottom message line: warnings, help, and hints.
func (m Model) warningLine() string {
	text, style := m.warningText()
	return style.Render(text)
}

// warningText returns the text of the bottom message line and the style that
// renders it. warningLine renders the pair, and MessageHint reads the text, so
// a test asserts on the words an operator reads rather than on escapes.
//
// The last two rows build their own styled string from several spans, so they
// answer styleAsIs: their text is already final.
func (m Model) warningText() (string, lipgloss.Style) {
	// Completion hint from ? or validation warning/error. The ? hint carries a
	// command's declared summary, which a plugin can write, so it is sanitized
	// here at the render boundary.
	if m.completionHint != "" {
		style := hintStyle
		if m.completionHintDim {
			style = dimStyle
		} else if strings.HasPrefix(m.completionHint, "invalid ") {
			style = warnStyle
		}
		return sanitizeForDisplay(m.completionHint), style
	}
	// The selected candidate's declared summary, while the menu is open. The
	// menu row is the name alone, so this row says what the name does.
	//
	// The text is READ from the selection. No key handler writes it, so an
	// arrow key moves it by construction. It renders whole. The summary is a
	// declared sentence, and no width here cuts one.
	//
	// muted is the role a description wears (docs/architecture/cli/color-system.md).
	if m.showDropdown && m.selected >= 0 && m.selected < len(m.completions) {
		// A plugin declares this text, so it is sanitized before it reaches the
		// terminal. `./le docvalid help-shape` refuses a newline in a declared
		// summary, so the row stays one line.
		return sanitizeForDisplay(m.completions[m.selected].Description), dimStyle
	}
	// Validation hints
	if hint := m.validationHintLine(); hint != "" {
		return hint, styleAsIs
	}
	return m.idleInfoLine(), styleAsIs
}

// MessageHint returns the text of the second message line with every style
// stripped, which is what an operator reads there. The `.et` harness asserts on
// this rather than on the rendered row, so a color change never breaks a test
// about words.
func (m Model) MessageHint() string {
	text, _ := m.warningText()
	return sanitizeForDisplay(text)
}

// idleInfoLine returns the default info line shown when there's no error or status.
func (m Model) idleInfoLine() string {
	var tb textbuf.Buffer
	if m.editor != nil {
		tb.Str("Ze Editor [").Str(m.mode.String()).Byte(']')
		if m.editor.Dirty() {
			tb.Str(" [modified]")
		}
	} else {
		tb.Str("Ze CLI [").Str(m.mode.String()).Byte(']')
	}
	info := tb.String()

	// Validation indicator
	if len(m.validationErrors) > 0 {
		info += errorStyle.Render(textbuf.StrIntStr(" ", int64(len(m.validationErrors)), " error(s)"))
	} else if len(m.validationWarnings) > 0 {
		info += dimStyle.Render(textbuf.StrIntStr(" ", int64(len(m.validationWarnings)), " warning(s)"))
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
	hint := "  red=error, yellow=missing field — 'show | errors' for details, 'show' for config"
	return dimStyle.Render(hint)
}

// overlayIndent is the column an overlay box starts at, so the box is clear of
// the left edge.
const overlayIndent = 2

// overlayDropdown renders the dropdown as a floating overlay on the base view.
// The dropdown is positioned above the prompt line to avoid covering the typed command.
func (m Model) overlayDropdown(base string) string {
	promptLine, availableAbove := promptAnchor(base)
	return placeAbovePrompt(base, m.renderDropdownBox(availableAbove), promptLine)
}

// overlayExplanation renders the explanation region as a floating overlay on the
// base view, in the place and with the bounds the dropdown uses. The two are
// never on the screen together: the explanation is revealed only once the
// candidate list is empty (model_help_level.go).
func (m Model) overlayExplanation(base string) string {
	promptLine, availableAbove := promptAnchor(base)
	return placeAbovePrompt(base, m.renderExplanationBox(availableAbove), promptLine)
}

// promptAnchor finds the prompt row of a rendered base view, and the rows above
// it an overlay box can use.
func promptAnchor(base string) (promptLine, availableAbove int) {
	lines := strings.Split(base, "\n")
	promptLine = len(lines) - 1
	for promptLine > 0 && strings.TrimSpace(lines[promptLine]) == "" {
		promptLine--
	}
	return promptLine, max(promptLine, 3)
}

// placeAbovePrompt puts a rendered box over the base view. The box sits above
// the prompt row and clear of the two message rows under it, so the typed
// command stays readable.
func placeAbovePrompt(base, box string, promptLine int) string {
	const messageRows = 2
	height := strings.Count(box, "\n") + 1
	return placeOverlay(overlayIndent, max(promptLine-height-messageRows, 0), box, base)
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

	return textbuf.Join(result, "\n")
}

// overlayLine places fg on top of bg at position x, handling ANSI codes.
func overlayLine(bg, fg string, x int) string {
	bgWidth := ansi.PrintableRuneWidth(bg)
	fgWidth := ansi.PrintableRuneWidth(fg)

	// ANSI reset to prevent style bleed
	const reset = "\x1b[0m"

	var tb textbuf.Buffer
	// If bg is shorter than x, just pad and add fg
	if bgWidth <= x {
		return tb.Str(bg).Str(reset).Repeat(" ", x-bgWidth).Str(fg).String()
	}

	// Need to slice bg around fg insertion point
	// Walk through bg tracking visible position vs byte position
	left := truncateAtWidth(bg, x)
	leftWidth := ansi.PrintableRuneWidth(left)

	// Pad if truncation was short
	pad := max(x-leftWidth, 0)

	// Get right portion: skip x + fgWidth visible chars
	right := skipWidth(bg, x+fgWidth)

	// Collect ANSI sequences from bg up to the right-portion cut point,
	// so we can restore the background's active styling after the overlay.
	bgRestore := collectAnsiState(bg, x+fgWidth)

	return tb.Str(left).Str(reset).Repeat(" ", pad).Str(fg).Str(reset).Str(bgRestore).Str(right).String()
}

// collectAnsiState walks s up to width visible characters and returns
// all ANSI escape sequences encountered, concatenated. Replaying these
// sequences restores the terminal styling that was active at that point.
func collectAnsiState(s string, width int) string {
	var seqs textbuf.Buffer
	var seq textbuf.Buffer
	w := 0
	inEsc := false

	for _, r := range s {
		if w >= width && !inEsc {
			break
		}

		if r == '\x1b' {
			inEsc = true
			seq.Reset()
			seq.WriteRune(r)
			continue
		}
		if inEsc {
			seq.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
				seqs.Str(seq.String())
			}
			continue
		}

		w++
	}

	return seqs.String()
}

// truncateAtWidth returns the prefix of s up to width visible characters.
func truncateAtWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	var result textbuf.Buffer
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

	innerWidth := m.overlayInnerWidth()

	var lines []string
	lines = append(lines, boxTopBorder("Completions", innerWidth))

	for i := start; i < end; i++ {
		// The row is the command name alone. The selected candidate's summary
		// is on message line 2, whole (warningText). The menu spends none of
		// its width on prose, so it cuts none.
		//
		// A name wider than the box would break the frame, so boxContentRow
		// clamps it there. That clamp bounds the frame. It does not shorten
		// prose. The longest command name in the repository is 31 runes,
		// against an inner width of 48 or more, so no name reaches it today.
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}

		var lb textbuf.Buffer
		lines = append(lines, boxContentRow(lb.Str(prefix).Str(m.completions[i].Text).String(), innerWidth))
	}

	if len(m.completions) > maxShow {
		var mb textbuf.Buffer
		mb.Str("  ... ").Int(int64(len(m.completions) - maxShow)).Str(" more")
		lines = append(lines, boxContentRow(mb.String(), innerWidth))
	}

	lines = append(lines, boxBottomBorder(innerWidth))

	return textbuf.Join(lines, "\n")
}

// renderExplanationBox draws the region that holds the long explanation of the
// command the operator typed. availableHeight is the number of screen rows the
// whole box can use, borders included, exactly as renderDropdownBox is bounded.
//
// A plugin declares this text, so two bounds are applied here, at the render
// boundary. sanitizeForDisplay strips its C0, DEL, C1 and ANSI bytes. The wrap
// reads no more rows than the box can draw. Neither bound depends on the 4096
// bytes the declaration gate allows, so a longer declaration that reaches this
// surface by another route is bounded too.
func (m Model) renderExplanationBox(availableHeight int) string {
	innerWidth := m.overlayInnerWidth()

	// 2 rows go to the borders. One more goes to the indicator that says text
	// remains, and only when there is a row to spare for it.
	rows := max(availableHeight-2, 1)
	wrapped := wrapForBox(sanitizeForDisplay(m.explanation), innerWidth, rows+1)

	shown := len(wrapped)
	overflow := shown > rows
	indicator := overflow && rows > 1
	if overflow {
		shown = rows
		if indicator {
			shown = rows - 1
		}
	}

	lines := make([]string, 0, shown+3)
	lines = append(lines, boxTopBorder(m.explanationSubject, innerWidth))
	for _, line := range wrapped[:shown] {
		lines = append(lines, boxContentRow(line, innerWidth))
	}
	if indicator {
		lines = append(lines, boxContentRow("  ... more", innerWidth))
	}
	lines = append(lines, boxBottomBorder(innerWidth))

	return textbuf.Join(lines, "\n")
}

// overlayInnerWidth is the width inside an overlay box, between the space each
// side wall carries. The box is indented 2 columns, its two borders take 4, and
// 4 columns of margin keep it off the right edge of the terminal. It is clamped
// to [48, 96]: 48 is the width the dropdown was fixed at, and 96 caps it on an
// ultra-wide terminal.
func (m Model) overlayInnerWidth() int {
	return min(max(m.width-10, 48), 96)
}

// boxTopBorder draws the top of an overlay box, with the title reading out of
// the left corner: ╭─ title ────╮. A title wider than the box is clamped, so the
// frame closes whatever it says.
func boxTopBorder(title string, innerWidth int) string {
	// The row between the corners is innerWidth+2 wide, and the dash and the
	// two spaces around the title take 3 of it.
	name := []rune(title)
	if len(name) > innerWidth-2 {
		name = name[:max(innerWidth-2, 0)]
	}

	var tb textbuf.Buffer
	return tb.Str("╭─ ").Str(string(name)).Byte(' ').Repeat("─", innerWidth-len(name)-1).Str("╮").String()
}

// boxContentRow draws one row inside an overlay box. The text is clamped to the
// inner width, so a long line cannot break the frame. It is then padded, so
// every row of the box is one width.
func boxContentRow(text string, innerWidth int) string {
	row := []rune(text)
	if len(row) > innerWidth {
		row = row[:innerWidth]
	}

	var tb textbuf.Buffer
	return tb.Str("│ ").Str(string(row)).Repeat(" ", innerWidth-len(row)).Str(" │").String()
}

// boxBottomBorder draws the bottom of an overlay box.
func boxBottomBorder(innerWidth int) string {
	var tb textbuf.Buffer
	return tb.Str("╰").Repeat("─", innerWidth+2).Str("╯").String()
}

// wrapForBox splits text into rows of at most width runes, and reads no further
// once linesMax rows are ready. The caller states that bound, so the work here
// is bounded by the screen rather than by the length of the text.
//
// It keeps the line breaks the author wrote, and breaks a longer line at its
// last space, or mid-word when one word fills the row. A tab becomes a space and
// a carriage return is dropped: both move the cursor inside a drawn row.
func wrapForBox(text string, width, linesMax int) []string {
	if width < 1 {
		width = 1
	}

	lines := make([]string, 0, linesMax)
	var line []rune
	breakAt := -1 // Rune index in line just after its last space, -1 when none.

	for _, r := range text {
		if len(lines) >= linesMax {
			return lines
		}
		if r == '\r' {
			continue
		}
		if r == '\t' {
			r = ' '
		}
		if r == '\n' {
			lines = append(lines, string(line))
			line, breakAt = line[:0], -1
			continue
		}

		line = append(line, r)
		if r == ' ' {
			breakAt = len(line)
		}
		if len(line) <= width {
			continue
		}

		cut := breakAt
		if cut < 1 {
			cut = width
		}
		lines = append(lines, strings.TrimRight(string(line[:cut]), " "))
		line = append(line[:0], line[cut:]...)
		breakAt = -1
	}

	if len(line) > 0 && len(lines) < linesMax {
		lines = append(lines, string(line))
	}
	return lines
}

// renderHelpOverlay renders help as a floating overlay.
// Shows full editor help when an editor is attached, or command-only help otherwise.
func (m Model) renderHelpOverlay(base string) string {
	var help string
	if m.hasEditor() {
		help = `Commands:
  set <path> <value>   Set a configuration value
  delete <path>        Delete a configuration value
  rename <p> <old> to <new>  Rename a list entry
  copy <p> <src> to <dst>    Copy a list entry
  edit <path>          Enter a subsection context
  edit <list> *        Edit template for all entries
  top                  Return to root context
  up                   Go up one level
  show                 Display configuration (scrollable)
  show confirmed       Display committed (on-disk) config
  show saved           Display saved draft file
  show | blame         Annotate with authorship
  show | changes [all] Pending changes (session or all)
  show | compare       Diff against committed config
  show | compare saved Diff against saved draft
  show | compare rollback N  Diff against backup N
  show | compare <user>      Diff against user's changes
  show | errors        Validation issues
  show | format config Display as set commands
  show | history       List rollback revisions
  option <col> enable  Enable display column (author/date/source/changes)
  option <col> disable Disable display column
  option all / none    Enable/disable all columns
  option errors hints  Toggle inline diagnostic hints
  option errors hide   Hide error annotations
  commit               Save changes with backup
  commit force         Save despite warnings (errors still block)
  commit confirmed <N> Save with auto-revert after N seconds
  commit force confirmed <N> Force + auto-revert
  confirm              Make pending commit permanent
  confirm abort        Cancel pending commit and roll back
  discard              Revert all changes
  rollback <N>         Restore backup N
  exit                 Return to operational mode

Modes:
  run <cmd>            Execute operational command (stays in config mode)
  configure            Enter config mode (from operational mode)
  (config commands in operational mode auto-switch to config mode)

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
	var result textbuf.Buffer
	for i, line := range lines {
		if i < 3 {
			result.Str(line).Byte('\n')
		}
	}
	result.Byte('\n')
	for _, line := range overlayLines {
		result.Str(line).Byte('\n')
	}

	return result.String()
}

// promptColor returns the style the prompt wears, which is this session's
// state. Blue is operational mode, green is configuration mode, and magenta is
// a command that failed. Failure outranks the mode, because the operator needs
// to see the failure whichever mode they are in. It clears as soon as a command
// succeeds (Model.err, cleared in Model.Update).
func (m Model) promptColor() lipgloss.Style {
	if m.err != nil {
		return promptStyle
	}
	if m.mode == ModeOperational {
		return promptOperationalStyle
	}
	return promptConfigStyle
}

// buildPrompt returns the context-aware prompt string.
func (m Model) buildPrompt() string {
	style := m.promptColor()

	if m.mode == ModeOperational {
		return style.Render("ze> ")
	}

	if len(m.contextPath) == 0 {
		return style.Render("ze# ")
	}

	// The breadcrumb keeps the context color in every state. It says WHERE the
	// operator is, which a failed command does not change.
	contextStr := textbuf.Join(m.contextPath, " ")
	var tb textbuf.Buffer
	return tb.Str(style.Render("ze")).Str(contextStyle.Render(tb.Reset().Byte('[').Str(contextStr).Byte(']').String())).Str(style.Render("# ")).String()
}

// renderInputWithGhost renders the text input.
// Ghost text is rendered by the textinput's native suggestion feature,
// so this is a simple pass-through.
func (m Model) renderInputWithGhost() string {
	return m.textInput.View()
}
