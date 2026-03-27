package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHighlightValidationIssues verifies error lines get highlighted.
//
// VALIDATES: Lines with errors are marked with red styling.
// PREVENTS: User unable to see which lines have errors.
func TestHighlightValidationIssues(t *testing.T) {
	// Force color output for testing (lipgloss disables in non-TTY)
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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

	model, err := NewModel(ed)
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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

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

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Should have no errors
	require.Empty(t, model.validationErrors, "valid config should have no errors")

	// View should not show error indicator anywhere
	view := model.View()
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
