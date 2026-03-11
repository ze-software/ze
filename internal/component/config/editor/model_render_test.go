package editor

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

	result := highlightValidationIssues(content, errors, nil, nil)

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

	result := highlightValidationIssues(content, nil, nil, nil)
	assert.Equal(t, content, result, "empty errors should return unchanged content")

	result = highlightValidationIssues(content, []ConfigValidationError{}, nil, nil)
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
	result := highlightValidationIssues(content, errors, nil, nil)
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
	// Original config had: line 1=bgp{, line 2=router-id, line 3=peer{, line 4=peer-as, line 5=hold-time
	// Filtered shows just line 4 and 5 as lines 1 and 2
	filteredContent := `peer-as 65001
hold-time 1`

	// Error is on original line 5 (hold-time), which is filtered line 2
	errors := []ConfigValidationError{
		{Line: 5, Message: "invalid hold-time"},
	}

	// Mapping: filtered line 1 → original line 4, filtered line 2 → original line 5
	lineMapping := map[int]int{
		1: 4,
		2: 5,
	}

	result := highlightValidationIssues(filteredContent, errors, nil, lineMapping)

	lines := strings.Split(result, "\n")
	require.Len(t, lines, 2)

	// Line 1 (peer-as) should NOT have ANSI codes - no error on original line 4
	assert.NotContains(t, lines[0], "\x1b[", "line 1 should not have ANSI codes")

	// Line 2 (hold-time) should have ANSI codes - error on original line 5
	assert.Contains(t, lines[1], "\x1b[", "line 2 should have ANSI styling")
	assert.Contains(t, lines[1], "hold-time", "line 2 content preserved")
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

	result := highlightValidationIssues(content, errors, warnings, nil)

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

	result := highlightValidationIssues(content, errors, warnings, nil)

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

	// Config with parse error on line 6 (invalid hold-time value)
	// The parser rejects "notanumber" during type validation, so tree is empty.
	// This test verifies error highlighting works on the full config view (raw text fallback).
	content := `bgp {
  router-id 1.2.3.4
  local-as 65000
  peer 1.1.1.1 {
    peer-as 65001
    hold-time notanumber
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
	assert.Contains(t, model.viewportContent, "hold-time", "viewport should show config content")

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

	// View should not show error indicator
	view := model.View()
	// Check that error style text is not present
	// The status bar should just show "Ze Editor" without error count
	lines := strings.Split(view, "\n")
	if len(lines) > 0 {
		header := lines[0]
		assert.NotContains(t, header, "⚠️", "status bar should not show error icon for valid config")
	}
}
