package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/muesli/reflow/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHighlightValidationIssues verifies error lines get highlighted.
//
// VALIDATES: Lines with errors are marked with red styling.
// PREVENTS: User unable to see which lines have errors.
func TestHighlightValidationIssues(t *testing.T) {
	// Force color output for testing (lipgloss disables in non-TTY)
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	content := `line 1
line 2
line 3
line 4`

	errors := []ConfigValidationError{
		{Line: 2, Message: "error on line 2"},
		{Line: 4, Message: "error on line 4"},
	}

	result := highlightValidationIssues(content, errors, nil, nil, true)

	// Lines 2 and 4 should have error styling (ANSI codes)
	lines := strings.Split(result, "\n")
	require.Len(t, lines, 4)

	// Line 1 should NOT have ANSI codes
	assert.NotContains(t, lines[0], "\x1b[", "line 1 should not have ANSI codes")

	// Line 2 should have ANSI codes (error line)
	assert.Contains(t, lines[1], "\x1b[", "line 2 should have ANSI styling")
	assert.Contains(t, lines[1], "line 2", "line 2 content preserved")

	// Line 3 should NOT have ANSI codes
	assert.NotContains(t, lines[2], "\x1b[", "line 3 should not have ANSI codes")

	// Line 4 should have ANSI codes (error line)
	assert.Contains(t, lines[3], "\x1b[", "line 4 should have ANSI styling")
	assert.Contains(t, lines[3], "line 4", "line 4 content preserved")
}

// TestHighlightValidationIssuesEmpty verifies no crash with empty errors.
//
// VALIDATES: Empty error list returns content unchanged.
// PREVENTS: Nil panic or unnecessary processing.
func TestHighlightValidationIssuesEmpty(t *testing.T) {
	content := "line 1\nline 2"

	result := highlightValidationIssues(content, nil, nil, nil, true)
	assert.Equal(t, content, result, "empty errors should return unchanged content")

	result = highlightValidationIssues(content, []ConfigValidationError{}, nil, nil, true)
	assert.Equal(t, content, result, "empty errors should return unchanged content")
}

// TestHighlightValidationIssuesOutOfRange verifies out-of-range lines are ignored.
//
// VALIDATES: Error with line > content lines doesn't crash.
// PREVENTS: Index out of range panic.
func TestHighlightValidationIssuesOutOfRange(t *testing.T) {
	content := "line 1\nline 2"

	errors := []ConfigValidationError{
		{Line: 5, Message: "out of range"},
		{Line: 0, Message: "zero line"},
	}

	// Should not panic
	result := highlightValidationIssues(content, errors, nil, nil, true)
	assert.Equal(t, content, result, "out of range errors should be ignored")
}

// TestHighlightValidationIssuesWithMapping verifies line mapping works for filtered content.
//
// VALIDATES: Error lines are highlighted correctly in filtered views.
// PREVENTS: Errors missed when viewing subsection of config.
func TestHighlightValidationIssuesWithMapping(t *testing.T) {
	// Force color output for testing
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	// Filtered content (e.g., inside a peer block)
	// Original config had: line 1=bgp{, line 2=router-id, line 3=peer{, line 4=peer-as, line 5=receive-hold-time
	// Filtered shows just line 4 and 5 as lines 1 and 2
	filteredContent := `peer-as 65001
receive-hold-time 1`

	// Error is on original line 5 (receive-hold-time), which is filtered line 2
	errors := []ConfigValidationError{
		{Line: 5, Message: "invalid receive-hold-time"},
	}

	// Mapping: filtered line 1 → original line 4, filtered line 2 → original line 5
	lineMapping := map[int]int{
		1: 4,
		2: 5,
	}

	result := highlightValidationIssues(filteredContent, errors, nil, lineMapping, true)

	lines := strings.Split(result, "\n")
	require.Len(t, lines, 2)

	// Line 1 (peer-as) should NOT have ANSI codes - no error on original line 4
	assert.NotContains(t, lines[0], "\x1b[", "line 1 should not have ANSI codes")

	// Line 2 (receive-hold-time) should have ANSI codes - error on original line 5
	assert.Contains(t, lines[1], "\x1b[", "line 2 should have ANSI styling")
	assert.Contains(t, lines[1], "receive-hold-time", "line 2 content preserved")
}

// TestHighlightValidationIssuesWarnings verifies warning lines get highlighted differently.
//
// VALIDATES: Lines with warnings are marked with yellow styling.
// PREVENTS: Warnings not visible or confused with errors.
func TestHighlightValidationIssuesWarnings(t *testing.T) {
	// Force color output for testing
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	content := `line 1
line 2
line 3`

	// Error on line 2, warning on line 3
	errors := []ConfigValidationError{
		{Line: 2, Message: "error"},
	}
	warnings := []ConfigValidationError{
		{Line: 3, Message: "warning"},
	}

	result := highlightValidationIssues(content, errors, warnings, nil, true)

	lines := strings.Split(result, "\n")
	require.Len(t, lines, 3)

	// Line 1 should NOT have ANSI codes
	assert.NotContains(t, lines[0], "\x1b[", "line 1 should not have ANSI codes")

	// Line 2 should have ANSI codes (error)
	assert.Contains(t, lines[1], "\x1b[", "line 2 should have ANSI styling")

	// Line 3 should have ANSI codes (warning)
	assert.Contains(t, lines[2], "\x1b[", "line 3 should have ANSI styling")
}

// TestHighlightValidationIssuesErrorPrecedence verifies errors take precedence over warnings.
//
// VALIDATES: When same line has error and warning, error style is used.
// PREVENTS: Warning style hiding error.
func TestHighlightValidationIssuesErrorPrecedence(t *testing.T) {
	// Force color output for testing
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	content := "line with both"

	errors := []ConfigValidationError{{Line: 1, Message: "error"}}
	warnings := []ConfigValidationError{{Line: 1, Message: "warning"}}

	result := highlightValidationIssues(content, errors, warnings, nil, true)

	// Should have styling (error takes precedence)
	assert.Contains(t, result, "\x1b[", "should have ANSI styling")
	// Can't easily distinguish error vs warning style in test, but error should win
}

// TestModelContextHighlighting verifies highlighting works when viewing subsection.
//
// VALIDATES: Errors highlight correctly in filtered view (edit context).
// PREVENTS: Line mapping disconnect between validation and display.
func TestModelContextHighlighting(t *testing.T) {
	// Force color output for testing
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Config with parse error (invalid receive-hold-time value)
	// The parser rejects "notanumber" during type validation, so tree is empty.
	// This test verifies error highlighting works on the full config view (raw text fallback).
	content := `bgp {
  router-id 1.2.3.4
  local-as 65000
  peer 1.1.1.1 {
    peer-as 65001
    timer { receive-hold-time notanumber; }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Should have validation error from load (parse error with line number)
	require.NotEmpty(t, model.validationErrors, "should have errors")

	// Show full config content with error highlighting
	model.showConfigContent()

	// Viewport should show the raw config content (tree is invalid, raw text fallback)
	assert.Contains(t, model.viewportContent, "receive-hold-time", "viewport should show config content")

	// Error line should be highlighted with ANSI escape codes
	assert.Contains(t, model.viewportContent, "\x1b[", "error line should be highlighted")
}

// makeTestCompletions creates N test completions for dropdown tests.
func makeTestCompletions(n int) []Completion {
	comps := make([]Completion, n)
	for i := range n {
		comps[i] = Completion{
			Text:        fmt.Sprintf("cmd%d", i+1),
			Description: fmt.Sprintf("Command %d", i+1),
			Type:        "command",
		}
	}
	return comps
}

// TestDropdownShowsAllItemsWhenSpaceAvailable verifies all items shown when screen is large enough.
//
// VALIDATES: Dropdown shows all completions when screen has enough space.
// PREVENTS: Hardcoded 6-item limit hiding available completions.
func TestDropdownShowsAllItemsWhenSpaceAvailable(t *testing.T) {
	m := Model{
		completions:  makeTestCompletions(10),
		selected:     0,
		showDropdown: true,
	}

	dropdown := m.renderDropdownBox(20) // 20 lines available — plenty for 10 items
	assert.NotContains(t, dropdown, "more", "all items should be visible without truncation")
	// Verify all 10 items present
	for i := range 10 {
		assert.Contains(t, dropdown, fmt.Sprintf("cmd%d", i+1), "should contain item %d", i+1)
	}
}

// TestDropdownTruncatesWhenSpaceLimited verifies truncation when screen is small.
//
// VALIDATES: Dropdown truncates when insufficient screen space.
// PREVENTS: Dropdown overflowing screen bounds.
func TestDropdownTruncatesWhenSpaceLimited(t *testing.T) {
	m := Model{
		completions:  makeTestCompletions(20),
		selected:     0,
		showDropdown: true,
	}

	dropdown := m.renderDropdownBox(6) // 6 lines: 2 borders + "more" = 3 items max
	assert.Contains(t, dropdown, "more", "should show truncation indicator")
	// Count content lines (between borders)
	lines := strings.Split(dropdown, "\n")
	contentLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "│") {
			contentLines++
		}
	}
	// 6 available - 2 borders - 1 "more" = 3 item lines + 1 more line = 4 content lines
	assert.Equal(t, 4, contentLines, "should show 3 items + 1 more indicator")
}

// TestDropdownPositionedAbovePrompt verifies dropdown renders above the command line.
//
// VALIDATES: Dropdown appears above the command line, not below it.
// PREVENTS: Dropdown overlaying the typed command.
func TestDropdownPositionedAbovePrompt(t *testing.T) {
	// Build a base view with prompt near the bottom (like the real View())
	var lines []string
	lines = append(lines, "Ze Editor", "")
	// Pad to push prompt to line 22 (0-indexed) in a 24-line terminal
	for len(lines) < 23 {
		lines = append(lines, "")
	}
	lines = append(lines, "ze# show") // prompt at line 23
	base := strings.Join(lines, "\n")

	m := Model{
		completions:  makeTestCompletions(3),
		selected:     0,
		showDropdown: true,
		height:       24,
		width:        80,
	}

	result := m.overlayDropdown(base)
	resultLines := strings.Split(result, "\n")

	// Find prompt line — should still be intact
	promptIdx := -1
	for i, line := range resultLines {
		if strings.Contains(line, "ze# show") {
			promptIdx = i
			break
		}
	}
	require.NotEqual(t, -1, promptIdx, "prompt line should exist in output")

	// Find dropdown top border
	dropdownIdx := -1
	for i, line := range resultLines {
		if strings.Contains(line, "Completions") {
			dropdownIdx = i
			break
		}
	}
	require.NotEqual(t, -1, dropdownIdx, "dropdown should exist in output")

	// Dropdown must be ABOVE the prompt
	assert.Less(t, dropdownIdx, promptIdx, "dropdown should be above the prompt line")
}

// TestWarningLineDimHint verifies completionHintDim renders with dimStyle.
//
// VALIDATES: When completionHintDim is true, warningLine uses dim styling.
// PREVENTS: Dim hints rendered in bright style, confusing partial vs confirmed input.
func TestWarningLineDimHint(t *testing.T) {
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	m := Model{
		completionHint:    "partial match hint",
		completionHintDim: true,
	}

	result := m.warningLine()

	// Should use dimStyle (color 241) not hintStyle (color 73)
	expected := dimStyle.Render("partial match hint")
	assert.Equal(t, expected, result, "dim hint should use dimStyle")
	assert.Contains(t, result, "partial match hint", "hint text should be preserved")
}

// TestWarningLineInvalidHint verifies "invalid " prefix renders with warnStyle.
//
// VALIDATES: Hints starting with "invalid " use warning (orange) style.
// PREVENTS: Invalid input hints shown in normal hint color, missing user attention.
func TestWarningLineInvalidHint(t *testing.T) {
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	m := Model{
		completionHint:    "invalid receive-hold-time value",
		completionHintDim: false,
	}

	result := m.warningLine()

	expected := warnStyle.Render("invalid receive-hold-time value")
	assert.Equal(t, expected, result, "invalid hint should use warnStyle")
}

// TestWarningLinePlainHint verifies plain hints render with hintStyle.
//
// VALIDATES: Hints without "invalid " prefix and not dim use hintStyle.
// PREVENTS: Normal completion descriptions using wrong style.
func TestWarningLinePlainHint(t *testing.T) {
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	m := Model{
		completionHint:    "foo: bar",
		completionHintDim: false,
	}

	result := m.warningLine()

	expected := hintStyle.Render("foo: bar")
	assert.Equal(t, expected, result, "plain hint should use hintStyle")
}

// TestFeedbackLineWelcome verifies welcome message uses welcomeStyle.
//
// VALIDATES: Status messages starting with "welcome" render in welcome (yellow) style.
// PREVENTS: Welcome message rendered with generic success style.
func TestFeedbackLineWelcome(t *testing.T) {
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	m := Model{
		statusMessage: "welcome to ze editor",
	}

	result := m.feedbackLine()

	expected := welcomeStyle.Render("welcome to ze editor")
	assert.Equal(t, expected, result, "welcome message should use welcomeStyle")
	// Should NOT have the ">" prefix that other status messages get
	assert.NotContains(t, result, "►", "welcome should not have indicator prefix")
}

// TestFeedbackLineQuit verifies "Quit?" message uses warnStyle.
//
// VALIDATES: Status messages starting with "Quit?" render in warn (orange) style.
// PREVENTS: Quit confirmation rendered as success, misleading the user.
func TestFeedbackLineQuit(t *testing.T) {
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	m := Model{
		statusMessage: "Quit? Press Esc again to exit",
	}

	result := m.feedbackLine()

	expected := warnStyle.Render("► Quit? Press Esc again to exit")
	assert.Equal(t, expected, result, "quit message should use warnStyle with indicator")
	assert.Contains(t, result, "►", "quit should have indicator prefix")
}

// TestDropdownWidthNarrow verifies dropdown renders correctly at minimum width.
//
// VALIDATES: Dropdown renders valid box structure at narrow terminal width (50).
// PREVENTS: Dropdown breaking or panicking when terminal is narrow.
func TestDropdownWidthNarrow(t *testing.T) {
	m := Model{
		completions:  makeTestCompletions(3),
		selected:     0,
		showDropdown: true,
		width:        50,
	}

	dropdown := m.renderDropdownBox(10)
	lines := strings.Split(dropdown, "\n")

	// Should have valid box structure: top border, items, bottom border
	require.GreaterOrEqual(t, len(lines), 5, "should have at least top + 3 items + bottom")
	assert.True(t, strings.HasPrefix(lines[0], "╭"), "should start with top-left corner")
	assert.True(t, strings.HasPrefix(lines[len(lines)-1], "╰"), "should end with bottom-left corner")
	assert.Contains(t, lines[0], "Completions", "top border should contain title")

	// All content lines should have matching borders
	for i := 1; i < len(lines)-1; i++ {
		assert.True(t, strings.HasPrefix(lines[i], "│"), "content line %d should start with │", i)
		assert.True(t, strings.HasSuffix(lines[i], "│"), "content line %d should end with │", i)
	}

	// All items should be present
	for i := range 3 {
		assert.Contains(t, dropdown, fmt.Sprintf("cmd%d", i+1), "should contain item %d", i+1)
	}
}

// TestDropdownWidthWide verifies dropdown renders correctly at maximum width.
//
// VALIDATES: Dropdown renders valid box structure at wide terminal width (200) with capped inner width.
// PREVENTS: Dropdown stretching unboundedly in ultra-wide terminals.
func TestDropdownWidthWide(t *testing.T) {
	m := Model{
		completions:  makeTestCompletions(3),
		selected:     0,
		showDropdown: true,
		width:        200,
	}

	dropdown := m.renderDropdownBox(10)
	lines := strings.Split(dropdown, "\n")

	// Should have valid box structure
	require.GreaterOrEqual(t, len(lines), 5, "should have at least top + 3 items + bottom")
	assert.True(t, strings.HasPrefix(lines[0], "╭"), "should start with top-left corner")
	assert.True(t, strings.HasPrefix(lines[len(lines)-1], "╰"), "should end with bottom-left corner")

	// Inner width should be capped at 96. Content line = "│ " + inner(96) + " │" = 100 chars.
	// The top border = "╭─ Completions " + dashes + "╮" should have consistent length.
	for i := 1; i < len(lines)-1; i++ {
		lineLen := len([]rune(lines[i]))
		// "│ " (2) + inner(96) + " │" (2) = 100
		assert.Equal(t, 100, lineLen, "content line %d should be 100 chars (inner width capped at 96)", i)
	}

	// All items should still be present
	for i := range 3 {
		assert.Contains(t, dropdown, fmt.Sprintf("cmd%d", i+1), "should contain item %d", i+1)
	}
}

// TestCollectAnsiState verifies ANSI sequence collection for color restoration.
func TestCollectAnsiState(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "plain text returns empty",
			input:  "hello world",
			width:  5,
			expect: "",
		},
		{
			name:   "single color sequence",
			input:  "\x1b[32mgreen text",
			width:  5,
			expect: "\x1b[32m",
		},
		{
			name:   "reset after color",
			input:  "\x1b[32mhi\x1b[0m bye",
			width:  6,
			expect: "\x1b[32m\x1b[0m",
		},
		{
			name:   "multiple sequences",
			input:  "\x1b[1m\x1b[34mbold blue",
			width:  5,
			expect: "\x1b[1m\x1b[34m",
		},
		{
			name:   "zero width returns empty",
			input:  "\x1b[32mtext",
			width:  0,
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectAnsiState(tt.input, tt.width)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestOverlayLineRestoresColor verifies background color is restored after overlay.
func TestOverlayLineRestoresColor(t *testing.T) {
	bg := "\x1b[34mblue background text here\x1b[0m"
	fg := "OVER"
	result := overlayLine(bg, fg, 5)

	// The right portion should be preceded by the background's ANSI state
	// so terminal color is restored after the overlay ends.
	assert.Contains(t, result, "OVER\x1b[0m\x1b[34m", "should restore bg color after overlay")
}

// TestDropdownMultilineDescriptionStaysOutOfTheBox verifies a description with
// a newline in it cannot reach the menu, so it cannot split a row.
//
// The box used to carry a description column and collapsed the newline to keep
// its frame. The column is gone: the row is the name alone, and the declared
// summary is on message line 2.
func TestDropdownMultilineDescriptionStaysOutOfTheBox(t *testing.T) {
	m := Model{
		completions: []Completion{
			{Text: "firewall", Description: "Firewall tables.\nTable names are bare.", Type: "command"},
			{Text: "interface", Description: "Network interface config", Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        80,
	}

	dropdown := m.renderDropdownBox(10)
	lines := strings.Split(dropdown, "\n")

	// Every content line (between top and bottom borders) must start and end with │
	for i := 1; i < len(lines)-1; i++ {
		assert.True(t, strings.HasPrefix(lines[i], "│"), "line %d should start with │: %q", i, lines[i])
		assert.True(t, strings.HasSuffix(lines[i], "│"), "line %d should end with │: %q", i, lines[i])
	}

	// Should have exactly: top border + 2 items + bottom border = 4 lines
	assert.Equal(t, 4, len(lines), "should have 4 lines (top + 2 items + bottom)")

	assert.NotContains(t, dropdown, "Table names are bare",
		"no description reaches the menu, so its newline cannot split a row")
	assert.NotContains(t, dropdown, "Network interface config",
		"no description reaches the menu")
}

// TestModelStatusBarNoErrorsWhenValid verifies no indicator when valid.
//
// VALIDATES: View() shows no error indicator for valid config.
// PREVENTS: False error display.
func TestModelStatusBarNoErrorsWhenValid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigOneLine), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Should have no errors
	require.Empty(t, model.validationErrors, "valid config should have no errors")

	// View should not show error indicator anywhere
	view := model.View().Content
	assert.NotContains(t, view, "error(s)", "view should not show error count for valid config")
}

// TestSanitizeForDisplayCleanString verifies clean strings pass through unchanged.
//
// VALIDATES: Normal config text is not altered by sanitization.
// PREVENTS: Sanitizer corrupting valid config content.
func TestSanitizeForDisplayCleanString(t *testing.T) {
	clean := "bgp {\n  router-id 1.2.3.4\n  local-as 65000\n}"
	assert.Equal(t, clean, sanitizeForDisplay(clean))
}

// TestSanitizeForDisplayPreservesWhitespace verifies tabs and newlines are preserved.
//
// VALIDATES: Tab and newline characters survive sanitization.
// PREVENTS: Config indentation or structure destroyed.
func TestSanitizeForDisplayPreservesWhitespace(t *testing.T) {
	input := "key\tvalue\nkey2\tvalue2\r\n"
	assert.Equal(t, input, sanitizeForDisplay(input))
}

// TestSanitizeForDisplayStripsANSIEscapes verifies ANSI escape sequences are removed.
//
// VALIDATES: Embedded ANSI color codes in config values are stripped.
// PREVENTS: Raw escape codes corrupting TUI display.
func TestSanitizeForDisplayStripsANSIEscapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "SGR color code",
			input: "value \x1b[31mred\x1b[0m text",
			want:  "value red text",
		},
		{
			name:  "cursor movement",
			input: "before\x1b[2Aafter",
			want:  "beforeafter",
		},
		{
			name:  "multiple sequences",
			input: "\x1b[1m\x1b[31mbold red\x1b[0m",
			want:  "bold red",
		},
		{
			name:  "OSC sequence with BEL",
			input: "text\x1b]0;title\x07rest",
			want:  "textrest",
		},
		{
			name:  "OSC sequence with ST",
			input: "text\x1b]0;title\x1b\\rest",
			want:  "textrest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeForDisplay(tt.input))
		})
	}
}

// TestSanitizeForDisplayStripsControlChars verifies C0/C1 control characters are replaced.
//
// VALIDATES: Non-printable control characters replaced with Unicode replacement char.
// PREVENTS: Null bytes, bells, and other control chars corrupting display.
func TestSanitizeForDisplayStripsControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "null byte",
			input: "abc\x00def",
			want:  "abc\uFFFDdef",
		},
		{
			name:  "bell",
			input: "abc\x07def",
			want:  "abc\uFFFDdef",
		},
		{
			name:  "vertical tab",
			input: "abc\x0Bdef",
			want:  "abc\uFFFDdef",
		},
		{
			name:  "form feed",
			input: "abc\x0Cdef",
			want:  "abc\uFFFDdef",
		},
		{
			name:  "DEL",
			input: "abc\x7Fdef",
			want:  "abc\uFFFDdef",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeForDisplay(tt.input))
		})
	}
}

// TestSanitizeForDisplayEmptyString verifies empty input returns empty output.
//
// VALIDATES: Empty string handled without panic or error.
// PREVENTS: Nil/empty edge case crash.
func TestSanitizeForDisplayEmptyString(t *testing.T) {
	assert.Equal(t, "", sanitizeForDisplay(""))
}

// TestSanitizeForDisplayUnicode verifies normal Unicode is preserved.
//
// VALIDATES: Non-ASCII printable characters (CJK, emoji, accented) survive sanitization.
// PREVENTS: Over-aggressive stripping of valid multibyte characters.
func TestSanitizeForDisplayUnicode(t *testing.T) {
	input := "peer 192.168.1.1 # commentaire francais"
	assert.Equal(t, input, sanitizeForDisplay(input))

	// CJK and emoji
	input2 := "description 测试 🌐"
	assert.Equal(t, input2, sanitizeForDisplay(input2))
}

// TestPromptColorSaysWhichModeTheOperatorIsIn verifies the prompt carries the
// mode as color, not just as the `>` / `#` glyph.
//
// VALIDATES: promptColor returns blue in operational mode and green in config
// mode, so an operator reads the mode on the line they are typing.
// PREVENTS: the regression this replaced, where both modes rendered magenta and
// the color carried no information at all.
func TestPromptColorSaysWhichModeTheOperatorIsIn(t *testing.T) {
	operational := Model{mode: ModeOperational}
	config := Model{mode: ModeConfig}

	assert.Equal(t, promptOperationalStyle, operational.promptColor(), "operational mode is blue")
	assert.Equal(t, promptConfigStyle, config.promptColor(), "config mode is green")
	assert.NotEqual(t, operational.promptColor(), config.promptColor(),
		"the two modes must not share a color, or the color says nothing")
}

// TestPromptColorFlagsTheLastFailedCommand verifies a failed command recolors
// the prompt in either mode, and that success clears it.
//
// VALIDATES: promptColor returns the failure color whenever Model.err is set,
// in operational mode and in config mode alike, and returns the mode color once
// err is nil again.
// PREVENTS: a failure that scrolls off the message area leaving no trace, so the
// operator types the next command believing the last one worked.
func TestPromptColorFlagsTheLastFailedCommand(t *testing.T) {
	boom := errors.New("no such peer")

	for _, mode := range []EditorMode{ModeOperational, ModeConfig} {
		t.Run(mode.String(), func(t *testing.T) {
			failed := Model{mode: mode, err: boom}
			assert.Equal(t, promptStyle, failed.promptColor(), "a failed command shows the failure color")

			// Failure outranks the mode, so it must NOT be the mode color.
			ok := Model{mode: mode}
			assert.NotEqual(t, ok.promptColor(), failed.promptColor(),
				"failure must be distinguishable from the mode it happened in")

			// And it clears: err back to nil returns the mode color.
			assert.Equal(t, ok.promptColor(), Model{mode: mode, err: nil}.promptColor())
		})
	}
}

// TestBuildPromptUsesTheStateColor verifies buildPrompt renders through
// promptColor rather than a fixed style, for every prompt shape.
//
// VALIDATES: the three shapes (`ze> `, `ze# `, `ze[...]# `) each change bytes
// when the state changes, and the breadcrumb keeps the context color.
// PREVENTS: promptColor being added and then not wired into the string the
// operator actually sees, which no color unit test alone would catch.
func TestBuildPromptUsesTheStateColor(t *testing.T) {
	boom := errors.New("no such peer")

	operational := Model{mode: ModeOperational}.buildPrompt()
	operationalFailed := Model{mode: ModeOperational, err: boom}.buildPrompt()
	assert.NotEqual(t, operational, operationalFailed, "`ze> ` must change color after a failure")

	config := Model{mode: ModeConfig}.buildPrompt()
	configFailed := Model{mode: ModeConfig, err: boom}.buildPrompt()
	assert.NotEqual(t, config, configFailed, "`ze# ` must change color after a failure")
	assert.NotEqual(t, operational, config, "the two modes must render differently")

	path := []string{"neighbor", "192.0.2.1"}
	ctx := Model{mode: ModeConfig, contextPath: path}.buildPrompt()
	ctxFailed := Model{mode: ModeConfig, contextPath: path, err: boom}.buildPrompt()
	assert.NotEqual(t, ctx, ctxFailed, "`ze[...]# ` must change color after a failure")

	// The breadcrumb is WHERE you are, which a failure does not change, so its
	// own color survives in both.
	breadcrumb := contextStyle.Render("[neighbor 192.0.2.1]")
	assert.Contains(t, ctx, breadcrumb)
	assert.Contains(t, ctxFailed, breadcrumb)
}

// TestPromptEmitsTheColorBytesAnSSHClientReceives verifies the prompt carries
// the ANSI-256 escape an operator's terminal acts on, not merely a distinct
// lipgloss.Style value.
//
// VALIDATES: buildPrompt emits SGR `38;5;33` in operational mode, `38;5;82` in
// config mode, and `38;5;205` after a failure, under every color profile. Render
// does not consult lipgloss.Writer.Profile. The escape is therefore in the
// string whatever the terminal is. Bubbletea's renderer downsamples it per
// session, and wish sets tea.WithColorProfile from the SSH client's TERM.
// PREVENTS: a comparison of two Style VALUES passing while the rendered prompt
// carries no escape at all, which is what a color test that never inspects
// bytes cannot tell apart.
func TestPromptEmitsTheColorBytesAnSSHClientReceives(t *testing.T) {
	const (
		blue    = "\x1b[38;5;33m"
		green   = "\x1b[38;5;82m"
		magenta = "\x1b[38;5;205m"
	)
	boom := errors.New("no such peer")

	// The loop ASSERTS an invariant. It does not establish a precondition, and
	// to read it as setup would mislead. `Style.Render`
	// (`vendor/charm.land/lipgloss/v2/style.go`) names neither Writer nor
	// Profile. The escape is emitted whatever this global says, so every
	// iteration passes with the line deleted.
	//
	// What the loop buys is a regression guard. If Render ever starts to
	// consult the profile, the Ascii iteration goes red HERE. Without the
	// guard it would instead strip color from one class of terminal in
	// production, and say nothing. Degradation is downstream and per session,
	// in bubbletea's renderer, from the SSH client's TERM
	// (`wish/v2/bubbletea`).
	//
	// Other tests in this file set the same global and then assert on Render
	// output. Those lines are inert for the reason above.
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })
	for _, prof := range []colorprofile.Profile{
		colorprofile.Ascii, colorprofile.ANSI256, colorprofile.TrueColor,
	} {
		lipgloss.Writer.Profile = prof

		assert.Contains(t, Model{mode: ModeOperational}.buildPrompt(), blue)
		assert.Contains(t, Model{mode: ModeConfig}.buildPrompt(), green)
		assert.Contains(t, Model{mode: ModeConfig, err: boom}.buildPrompt(), magenta)
		assert.Contains(t, Model{mode: ModeOperational, err: boom}.buildPrompt(), magenta)

		// The failure color must REPLACE the mode color, never sit beside it.
		failed := Model{mode: ModeOperational, err: boom}.buildPrompt()
		assert.NotContains(t, failed, blue, "failure replaces the mode color")
	}
}

// TestMessageLinesDoNotRepeatTheIdleBanner verifies the two-line message area
// never renders the same idle banner on both of its lines.
//
// VALIDATES: an idle model -- no error, no status message, no completion or
// validation hint -- puts the banner on exactly one of the two lines.
//
// PREVENTS: the banner printed twice above the prompt.
//
// Line 1 (feedbackLine) and line 2 (warningLine) each fell through to
// idleInfoLine when they had nothing of their own to say. That is the state a
// session is in the moment it leaves configuration mode. A recording of
// `exit` showed "Ze Editor [operational]" on two consecutive rows.
func TestMessageLinesDoNotRepeatTheIdleBanner(t *testing.T) {
	m := Model{}

	line1, line2 := m.messageLines()

	if line1 != "" && line1 == line2 {
		t.Errorf("both message lines carry the same text, so the banner prints twice: %q", line1)
	}
	if line2 == "" {
		t.Errorf("the idle banner is missing from the message area: line1=%q line2=%q", line1, line2)
	}
}

// TestDropdownRowIsTheCommandNameAlone verifies the menu row carries the name
// and nothing else.
//
// VALIDATES: renderDropdownBox draws each candidate as its whole name, with no
// description column and no ellipsis on the name.
// PREVENTS: The width truncation the description column forced, which cut a
// declared summary at a column the author never chose.
func TestDropdownRowIsTheCommandNameAlone(t *testing.T) {
	const summaryLong = "advertise the local preference attribute to every configured peer in the group"
	const summaryShort = "configure a BGP neighbor and its address families"

	m := Model{
		completions: []Completion{
			{Text: "advertise-interval-milliseconds", Description: summaryLong, Type: "command"},
			{Text: "neighbor", Description: summaryShort, Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        80,
	}

	dropdown := m.renderDropdownBox(10)

	assert.Contains(t, dropdown, "advertise-interval-milliseconds",
		"the longest command name in the repository renders whole")
	assert.Contains(t, dropdown, "neighbor", "every candidate renders")

	for _, fragment := range []string{"advertise the local", "preference", "address families"} {
		assert.NotContains(t, dropdown, fragment,
			"no description text belongs in the menu: %q", fragment)
	}
	assert.NotContains(t, dropdown, "...", "no row is truncated with an ellipsis")

	// Every row is the box frame plus the name, so the frame still closes.
	lines := strings.Split(dropdown, "\n")
	for i := 1; i < len(lines)-1; i++ {
		assert.True(t, strings.HasPrefix(lines[i], "│"), "line %d should start with │: %q", i, lines[i])
		assert.True(t, strings.HasSuffix(lines[i], "│"), "line %d should end with │: %q", i, lines[i])
	}
}

// TestSelectionMovesTheSummaryOnTheMessageLine verifies the message line
// carries the selected candidate's declared summary, whole.
//
// VALIDATES: warningText derives the second message line from m.selected, so an
// arrow key moves the summary with the selection and no handler writes it.
// PREVENTS: A summary that lags the selection, and a summary cut to a column
// width the way the removed description column cut it.
func TestSelectionMovesTheSummaryOnTheMessageLine(t *testing.T) {
	const summaryFirst = "advertise the local preference attribute to every configured peer in the group"
	const summarySecond = "configure a BGP neighbor and its address families"
	const summaryThird = "hold the session down until the operator clears it"

	m := Model{
		completions: []Completion{
			{Text: "advertise", Description: summaryFirst, Type: "command"},
			{Text: "neighbor", Description: summarySecond, Type: "command"},
			{Text: "shutdown", Description: summaryThird, Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        80,
	}

	assert.Equal(t, summaryFirst, m.MessageHint(),
		"the first candidate's summary shows whole, past the width the old column allowed")

	down, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	m, ok := down.(Model)
	require.True(t, ok)
	assert.Equal(t, summarySecond, m.MessageHint(), "down moves the summary with the selection")

	down2, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	m, ok = down2.(Model)
	require.True(t, ok)
	assert.Equal(t, summaryThird, m.MessageHint(), "each move changes the summary")

	up, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	m, ok = up.(Model)
	require.True(t, ok)
	assert.Equal(t, summarySecond, m.MessageHint(), "up moves it back")
}

// TestSummaryDoesNotDisplaceAnError verifies the two message rows keep their
// own occupants.
//
// VALIDATES: An error stays on message line 1 (feedbackLine) while the selected
// candidate's summary is on line 2 (warningLine).
// PREVENTS: A menu that hides the fault an operator needs to read.
func TestSummaryDoesNotDisplaceAnError(t *testing.T) {
	const summary = "configure a BGP neighbor and its address families"

	m := Model{
		err: errors.New("peer 192.0.2.1 is not configured"),
		completions: []Completion{
			{Text: "neighbor", Description: summary, Type: "command"},
			{Text: "shutdown", Description: "hold the session down", Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        80,
	}

	line1, line2 := m.messageLines()

	assert.Contains(t, line1, "peer 192.0.2.1 is not configured", "the error keeps line 1")
	assert.NotContains(t, line1, summary, "the summary never reaches line 1")
	assert.Contains(t, line2, summary, "the summary is on line 2")
	assert.NotContains(t, line2, "peer 192.0.2.1 is not configured",
		"the two rows are different rows, so neither can hide the other")
}

// TestSummaryStripsATerminalEscape verifies the message line carries no escape
// sequence a plugin wrote.
//
// VALIDATES: warningText passes the selected candidate's declared summary
// through sanitizeForDisplay before it reaches the terminal.
// PREVENTS: A plugin's summary clearing the screen or moving the cursor when an
// operator arrows onto its command.
func TestSummaryStripsATerminalEscape(t *testing.T) {
	m := Model{
		completions: []Completion{
			{Text: "evil", Description: "clear \x1b[2J and move \x1b[1;1H the cursor", Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        80,
	}

	row := m.warningLine()

	assert.NotContains(t, row, "\x1b[2J", "an erase-display sequence never reaches the terminal")
	assert.NotContains(t, row, "\x1b[1;1H", "a cursor-position sequence never reaches the terminal")
	assert.Contains(t, row, "clear and move the cursor", "the words the author wrote survive")
}

// TestSummaryWithANewlineStaysOnOneRow verifies the second bound the message row
// keeps on text a plugin declares.
//
// VALIDATES: warningText folds every whitespace run in the selected candidate's
// summary, so the row it returns holds no newline, carriage return or tab.
// PREVENTS: A declared summary drawing a SECOND screen line. View counts the
// message area as two entries and reads the prompt row from that count, so an
// extra line puts the cursor one row above the prompt. `./le docvalid
// help-shape` reads the summaries declared in this tree, and a plugin that
// registers over the wire declares one it never sees.
func TestSummaryWithANewlineStaysOnOneRow(t *testing.T) {
	m := Model{
		completions: []Completion{
			{Text: "wordy", Description: "First line.\nSecond line.\r\n\tThird.", Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        80,
	}

	row := m.warningLine()

	assert.NotContains(t, row, "\n", "no newline reaches the message row")
	assert.NotContains(t, row, "\r", "no carriage return reaches the message row")
	assert.NotContains(t, row, "\t", "no tab reaches the message row")
	assert.Contains(t, row, "First line. Second line. Third.", "every word the author wrote survives")
	assert.Equal(t, "First line. Second line. Third.", m.MessageHint(), "the harness reads the same one row")
}

// TestHintWithANewlineStaysOnOneRow verifies the ? hint keeps that bound too.
//
// VALIDATES: warningText folds the completion hint, which handleKeyMsg builds
// from a candidate's declared text as "<name>: <summary>".
// PREVENTS: The ? key drawing a two-line message row where the summary route
// draws one, which would leave the guard on half of a pair.
func TestHintWithANewlineStaysOnOneRow(t *testing.T) {
	m := Model{
		completionHint: "wordy: First line.\nSecond line.",
		width:          80,
	}

	assert.Equal(t, "wordy: First line. Second line.", m.MessageHint(), "the hint is one row")
}

// TestDropdownClampsANameWiderThanTheBox verifies the one bound the menu keeps.
//
// VALIDATES: renderDropdownBox clamps a candidate name to the inner width, so a
// name wider than the terminal cannot break the box frame.
// PREVENTS: A ragged overlay when a name is longer than the box, which is the
// boundary case the removed description column used to absorb.
func TestDropdownClampsANameWiderThanTheBox(t *testing.T) {
	m := Model{
		completions: []Completion{
			{Text: strings.Repeat("x", 200), Description: "a name nobody declares", Type: "command"},
			{Text: "neighbor", Description: "a short one", Type: "command"},
		},
		selected:     0,
		showDropdown: true,
		width:        50, // inner width clamps to its 48 floor
	}

	dropdown := m.renderDropdownBox(10)
	lines := strings.Split(dropdown, "\n")

	require.Len(t, lines, 4, "top border + 2 items + bottom border")
	for i := 1; i < len(lines)-1; i++ {
		row := []rune(lines[i])
		// "│ " (2) + inner(48) + " │" (2) = 52
		assert.Equal(t, 52, len(row), "content line %d keeps the frame width: %q", i, lines[i])
		assert.True(t, strings.HasPrefix(lines[i], "│"), "content line %d starts with │", i)
		assert.True(t, strings.HasSuffix(lines[i], "│"), "content line %d ends with │", i)
	}
}

// TestExplanationIsBoundedByTerminalHeight covers AC-11.
//
// VALIDATES: an explanation far taller than the terminal renders inside the
// rows it was given, with an overflow indicator and an intact frame.
// PREVENTS: a declared explanation pushing the prompt off the screen or
// breaking the box the operator reads it in.
func TestExplanationIsBoundedByTerminalHeight(t *testing.T) {
	m := Model{
		explanation:        strings.Repeat("the explanation continues and continues. ", 400),
		explanationSubject: "peer list",
		width:              80,
		height:             24,
	}

	const availableHeight = 10
	box := m.renderExplanationBox(availableHeight)
	lines := strings.Split(box, "\n")

	if len(lines) > availableHeight {
		t.Errorf("box is %d lines, want at most %d", len(lines), availableHeight)
	}
	if !strings.HasPrefix(lines[0], "╭") {
		t.Errorf("first line %q does not open the frame", lines[0])
	}
	if !strings.Contains(lines[0], "peer list") {
		t.Errorf("title %q does not name the command the explanation is about", lines[0])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "╰") {
		t.Errorf("last line %q does not close the frame", lines[len(lines)-1])
	}
	if !strings.Contains(box, "more") {
		t.Error("a bounded explanation must say that text remains")
	}

	width := ansi.PrintableRuneWidth(lines[0])
	for i, line := range lines {
		if got := ansi.PrintableRuneWidth(line); got != width {
			t.Errorf("line %d is %d wide, want %d: %q", i, got, width, line)
		}
	}
}

// TestExplanationOverlayKeepsTheFrame covers AC-11 over the composed view.
//
// VALIDATES: the overlay writes the explanation above the prompt and changes no
// line count, so the terminal frame survives.
// PREVENTS: an explanation that scrolls the prompt away.
func TestExplanationOverlayKeepsTheFrame(t *testing.T) {
	lines := []string{"Ze Editor", ""}
	for len(lines) < 23 {
		lines = append(lines, "")
	}
	lines = append(lines, "ze# peer list ")
	base := strings.Join(lines, "\n")

	m := Model{
		explanation:        strings.Repeat("wrap this sentence over many rows. ", 200),
		explanationSubject: "peer list",
		width:              80,
		height:             24,
	}

	out := m.overlayExplanation(base)
	outLines := strings.Split(out, "\n")

	if len(outLines) != len(lines) {
		t.Errorf("overlay produced %d lines, want %d", len(outLines), len(lines))
	}
	promptIdx, boxIdx := -1, -1
	for i, line := range outLines {
		if strings.Contains(line, "ze# peer list") {
			promptIdx = i
		}
		if boxIdx == -1 && strings.Contains(line, "╭") {
			boxIdx = i
		}
	}
	if promptIdx == -1 {
		t.Fatal("the prompt line must survive the overlay")
	}
	if boxIdx == -1 {
		t.Fatal("the explanation box must appear in the overlay")
	}
	if boxIdx >= promptIdx {
		t.Errorf("box at line %d is not above the prompt at line %d", boxIdx, promptIdx)
	}
}

// TestExplanationStripsATerminalEscape is a Security Review row.
//
// VALIDATES: a plugin-declared explanation reaches the terminal with its C0,
// DEL, C1 and ANSI bytes stripped.
// PREVENTS: a declared explanation repainting or corrupting the operator's
// terminal.
func TestExplanationStripsATerminalEscape(t *testing.T) {
	m := Model{
		explanation:        "safe \x1b[31mred\x1b[0m and \x07 a bell",
		explanationSubject: "peer list",
		width:              80,
	}

	box := m.renderExplanationBox(10)

	if strings.Contains(box, "\x1b") {
		t.Errorf("box carries an escape byte: %q", box)
	}
	if strings.Contains(box, "\x07") {
		t.Errorf("box carries a control byte: %q", box)
	}
	if !strings.Contains(box, "safe") || !strings.Contains(box, "red") {
		t.Errorf("box lost the declared words: %q", box)
	}
}

// TestExplanationHonoursTheAuthoredLineBreaks covers the wrap contract.
//
// VALIDATES: the explanation keeps the newlines its author wrote and wraps what
// is still wider than the box.
// PREVENTS: a declared paragraph break collapsing into one run of text.
func TestExplanationHonoursTheAuthoredLineBreaks(t *testing.T) {
	m := Model{
		explanation:        "first line\nsecond line\n" + strings.Repeat("word ", 40),
		explanationSubject: "peer list",
		width:              80,
	}

	box := m.renderExplanationBox(20)
	lines := strings.Split(box, "\n")

	firstIdx, secondIdx := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "first line") {
			firstIdx = i
		}
		if strings.Contains(line, "second line") {
			secondIdx = i
		}
	}
	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("both declared lines must render: %q", box)
	}
	if secondIdx != firstIdx+1 {
		t.Errorf("the authored break put line 2 at row %d, want row %d", secondIdx, firstIdx+1)
	}

	innerWidth := ansi.PrintableRuneWidth(lines[0]) - 4
	for i, line := range lines {
		if got := ansi.PrintableRuneWidth(line) - 4; got > innerWidth {
			t.Errorf("row %d is %d wide, want at most %d", i, got, innerWidth)
		}
	}
}

// TestWrapForBoxReadsNoMoreRowsThanAsked guards the work bound.
//
// VALIDATES: the wrap stops at the row count the caller states, whatever the
// length of the declared text.
// PREVENTS: a render walking a declaration of any size once per frame.
func TestWrapForBoxReadsNoMoreRowsThanAsked(t *testing.T) {
	const linesMax = 4
	lines := wrapForBox(strings.Repeat("a sentence that keeps going. ", 5000), 40, linesMax)

	if len(lines) != linesMax {
		t.Errorf("wrap produced %d rows, want %d", len(lines), linesMax)
	}
	for i, line := range lines {
		if len([]rune(line)) > 40 {
			t.Errorf("row %d is %d runes, want at most 40", i, len([]rune(line)))
		}
	}
}
