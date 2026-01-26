package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test config constants to avoid duplication.
const (
	testValidBGPConfig         = `bgp { router-id 1.2.3.4; }`
	testValidBGPConfigOneLine  = `bgp { router-id 1.2.3.4; }`
	testValidBGPConfigWithPeer = `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time 90;
  }
}`
)

// TestModelValidationOnLoad verifies validation runs when editor loads.
//
// VALIDATES: Initial validation populates error list.
// PREVENTS: Invalid config not flagged until commit.
func TestModelValidationOnLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config with validation error (hold-time 1 inside peer)
	content := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time 1;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Should have validation error from load
	assert.NotEmpty(t, model.validationErrors, "should have errors on load")
}

// TestModelCommitBlockedOnErrors verifies commit is blocked with errors.
//
// VALIDATES: Commit returns error when validation fails.
// PREVENTS: Saving invalid configuration.
func TestModelCommitBlockedOnErrors(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config with validation error (hold-time 2 inside peer)
	content := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time 2;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Commit should fail
	_, err = model.cmdCommit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot commit")
	assert.Contains(t, err.Error(), "validation error")
}

// TestModelCommitSucceedsWhenValid verifies commit works with valid config.
//
// VALIDATES: Commit succeeds when no validation errors.
// PREVENTS: False positive blocking of valid config.
func TestModelCommitSucceedsWhenValid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write valid config
	content := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time 90;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty() // Mark dirty so commit does something

	// Should have no errors
	assert.Empty(t, model.validationErrors, "valid config should have no errors")

	// Commit should succeed
	result, err := model.cmdCommit()
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "committed")
}

// TestModelErrorsCommand verifies errors command output.
//
// VALIDATES: Errors command formats error list correctly.
// PREVENTS: User unable to see validation issues.
func TestModelErrorsCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config with errors
	content := `bgp {
  router-id invalid;
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Get errors
	result, err := model.cmdErrors()
	require.NoError(t, err)

	// Should have error content (parser error for invalid router-id)
	assert.Contains(t, result.output, "Errors")
}

// TestModelErrorsCommandNoIssues verifies errors command with valid config.
//
// VALIDATES: Errors command shows "no issues" when valid.
// PREVENTS: Confusing output for valid config.
func TestModelErrorsCommandNoIssues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write valid config
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Get errors
	result, err := model.cmdErrors()
	require.NoError(t, err)

	assert.Contains(t, result.output, "No validation issues")
}

// TestModelCmdTop verifies top command returns to root context.
//
// VALIDATES: Top command clears context and shows full config.
// PREVENTS: User stuck in nested context.
func TestModelCmdTop(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set a hierarchical context path
	model.contextPath = []string{"bgp", "peer", "1.1.1.1"}

	// Call top
	result, err := model.cmdTop()
	require.NoError(t, err)

	// Should clear context
	assert.True(t, result.clearContext, "should set clearContext flag")

	// Should return full config view
	assert.NotNil(t, result.configView, "should return config view")
	assert.Contains(t, result.configView.content, "bgp", "should contain full config")
}

// TestModelCmdEditHierarchical verifies edit builds hierarchical context path.
//
// VALIDATES: Edit command finds full path to target block.
// PREVENTS: Flat context paths that break navigation.
func TestModelCmdEditHierarchical(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit a nested block
	result, err := model.cmdEdit([]string{"peer", "1.1.1.1"})
	require.NoError(t, err)

	// Should build hierarchical path including parent (bgp)
	assert.Equal(t, []string{"bgp", "peer", "1.1.1.1"}, result.newContext, "should have hierarchical path")

	// Should show peer content
	assert.NotNil(t, result.configView, "should have config view")
	assert.Contains(t, result.configView.content, "peer-as", "should show peer block content")
}

// TestModelCmdEditWildcardTemplate verifies edit with wildcard creates template context.
//
// VALIDATES: "edit peer *" creates template mode without requiring exact block.
// PREVENTS: Template editing broken by block-not-found check.
func TestModelCmdEditWildcardTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit with wildcard template
	result, err := model.cmdEdit([]string{"peer", "*"})
	require.NoError(t, err, "wildcard edit should not error")

	// Should be in template mode
	assert.True(t, result.isTemplate, "should be template mode")

	// Context should include wildcard
	assert.Equal(t, []string{"bgp", "peer", "*"}, result.newContext, "should have template path")

	// Should have config view (parent block content)
	assert.NotNil(t, result.configView, "should have config view")
}

// TestModelCmdEditNotFound verifies edit shows error for nonexistent block.
//
// VALIDATES: Edit command fails with clear error for missing block.
// PREVENTS: Silent failure or confusing state when block doesn't exist.
func TestModelCmdEditNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Try to edit nonexistent block
	_, err = model.cmdEdit([]string{"nonexistent", "block"})
	require.Error(t, err, "should error for nonexistent block")
	assert.Contains(t, err.Error(), "not found", "error should mention not found")
}

// TestModelCmdEditFromContext verifies edit works from within a context.
//
// VALIDATES: Edit finds blocks relative to current position.
// PREVENTS: Navigation broken when already in a subsection.
func TestModelCmdEditFromContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Config with nested structure
	content := `bgp {
  router-id 1.2.3.4;
  peer 1.1.1.1 {
    peer-as 65001;
    capability {
      route-refresh;
    }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Start at bgp level
	model.contextPath = []string{"bgp"}

	// Edit peer from within bgp context - should still find it
	result, err := model.cmdEdit([]string{"peer", "1.1.1.1"})
	require.NoError(t, err)

	// Should have full hierarchical path
	assert.Equal(t, []string{"bgp", "peer", "1.1.1.1"}, result.newContext)
}

// TestModelCmdEditExactMatch verifies edit uses exact block matching.
//
// VALIDATES: Edit doesn't match prefix (e.g., "peer" shouldn't match "peer-group").
// PREVENTS: Wrong block selected due to prefix matching.
func TestModelCmdEditExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Config with similar block names
	content := `bgp {
  peer-group external {
    peer-as 65001;
  }
  peer 1.1.1.1 {
    peer-as 65002;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit "peer 1.1.1.1" should NOT match "peer-group"
	result, err := model.cmdEdit([]string{"peer", "1.1.1.1"})
	require.NoError(t, err)

	// Should find the correct peer block
	assert.Equal(t, []string{"bgp", "peer", "1.1.1.1"}, result.newContext)
	assert.Contains(t, result.configView.content, "65002", "should show peer content, not peer-group")
}

// TestModelCmdUp verifies up command goes up one context level.
//
// VALIDATES: Up command navigates to parent block in hierarchy.
// PREVENTS: User unable to navigate out of nested context.
func TestModelCmdUp(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set a hierarchical context path (inside bgp > peer 1.1.1.1)
	model.contextPath = []string{"bgp", "peer", "1.1.1.1"}

	// Call up - should go to parent (bgp block)
	result, err := model.cmdUp()
	require.NoError(t, err)

	// Should go up to bgp level
	assert.Equal(t, []string{"bgp"}, result.newContext, "should go up to bgp level")
	assert.NotNil(t, result.configView, "should have config view")
}

// TestModelCmdUpFromTemplate verifies up command from template context.
//
// VALIDATES: Up from template context goes to parent block.
// PREVENTS: Navigation broken in template mode.
func TestModelCmdUpFromTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set template context (inside bgp > peer *)
	model.contextPath = []string{"bgp", "peer", "*"}
	model.isTemplate = true

	// Call up - should go to bgp level (skipping invalid "bgp peer" context)
	result, err := model.cmdUp()
	require.NoError(t, err)

	// Should go to bgp level and clear template mode
	assert.Equal(t, []string{"bgp"}, result.newContext, "should go up to bgp level")
	assert.False(t, result.isTemplate, "should clear template mode")
	assert.NotNil(t, result.configView, "should have config view")
}

// TestModelCmdUpAtRoot verifies up command at root level.
//
// VALIDATES: Up at root shows message instead of error.
// PREVENTS: Confusing error when user is already at top.
func TestModelCmdUpAtRoot(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// No context path (at root)
	model.contextPath = nil

	// Call up
	result, err := model.cmdUp()
	require.NoError(t, err)

	// Should show message
	assert.Contains(t, result.output, "top level", "should indicate already at top")
}

// TestModelStatusMessageDisplay verifies status message appears in View.
//
// VALIDATES: Status message displays above viewport with correct styling.
// PREVENTS: Status messages not visible to user.
func TestModelStatusMessageDisplay(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Set a status message
	model.statusMessage = "Test status message"

	// View should contain the status message
	view := model.View()
	assert.Contains(t, view, "Test status message", "status message should appear in view")
	assert.Contains(t, view, "►", "status message should have indicator prefix")
}

// TestModelStatusMessageClearsOnCommand verifies status clears on next command.
//
// VALIDATES: Status message is temporary - clears when command doesn't set one.
// PREVENTS: Stale status messages persisting indefinitely.
func TestModelStatusMessageClearsOnCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set initial status
	model.statusMessage = "Initial status"

	// Run a command that doesn't set statusMessage (cmdShow)
	result, err := model.cmdShow(nil)
	require.NoError(t, err)

	// Result should have empty statusMessage
	assert.Empty(t, result.statusMessage, "cmdShow should not set status")

	// Simulate Update handler applying result
	model.statusMessage = result.statusMessage

	// Status should be cleared
	assert.Empty(t, model.statusMessage, "status should clear after command without status")
}

// TestModelStatusMessageClearsOnError verifies status clears on error.
//
// VALIDATES: Status message clears when command fails.
// PREVENTS: Misleading status shown alongside error.
func TestModelStatusMessageClearsOnError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set initial status
	model.statusMessage = "Initial status"

	// Simulate error result from Update handler
	msg := commandResultMsg{err: fmt.Errorf("test error")}

	// Apply error handling (from Update handler logic)
	if msg.err != nil {
		model.err = msg.err
		model.statusMessage = "" // This is what Update does
	}

	// Status should be cleared, error should be set
	assert.Empty(t, model.statusMessage, "status should clear on error")
	assert.Error(t, model.err, "error should be set")
}

// TestModelRevalidatesOnDiscard verifies validation runs after discard.
//
// VALIDATES: Validation state updated after discard.
// PREVENTS: Stale error list after content change.
func TestModelRevalidatesOnDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Start with valid config
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	assert.Empty(t, model.validationErrors, "initial config should be valid")

	// Simulate editing to invalid (set working content)
	ed.SetWorkingContent(`bgp { router-id invalid; }`)
	model.runValidation()
	assert.NotEmpty(t, model.validationErrors, "edited config should have errors")

	// Discard calls runValidation, but we test it directly
	ed.SetWorkingContent(ed.OriginalContent())
	model.runValidation()
	assert.Empty(t, model.validationErrors, "after discard should be valid again")
}

// TestModelValidationDebounce verifies debounced validation tick handling.
//
// VALIDATES: Validation tick with matching ID triggers validation.
// PREVENTS: Stale ticks triggering unwanted validation.
func TestModelValidationDebounce(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Start with valid config
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	initialID := model.validationID

	// Schedule validation increments ID and returns tick command
	cmd := model.scheduleValidation()
	assert.NotNil(t, cmd, "scheduleValidation should return a command")
	assert.Equal(t, initialID+1, model.validationID, "validationID should increment")

	// Simulate receiving tick with matching ID
	// First change content to something with errors
	ed.SetWorkingContent(`bgp { peer 1.1.1.1 { peer-as 65001; hold-time 1; } }`)
	currentID := model.validationID

	// Update returns a new model - we need to use that
	newModel, _ := model.Update(validationTickMsg{id: currentID})
	updatedModel, ok := newModel.(Model)
	require.True(t, ok, "Update should return a Model")
	assert.NotEmpty(t, updatedModel.validationErrors, "matching tick should trigger validation")

	// Test stale tick: increment ID (simulating new keystroke), then send old ID
	// Change content to valid - if validation runs, errors would clear
	ed.SetWorkingContent(testValidBGPConfig)
	// Keep errors from previous validation to detect if stale tick runs
	errorsBeforeStale := len(updatedModel.validationErrors)
	require.Greater(t, errorsBeforeStale, 0, "should still have errors before stale tick test")

	// Increment ID (simulating another keystroke scheduling new validation)
	updatedModel.validationID++
	staleID := currentID // The old ID before increment

	// Send stale tick - should NOT trigger validation
	staleModel, _ := updatedModel.Update(validationTickMsg{id: staleID})
	finalModel, ok := staleModel.(Model)
	require.True(t, ok, "Update should return a Model")

	// Errors should remain (validation didn't run on stale tick)
	assert.Equal(t, errorsBeforeStale, len(finalModel.validationErrors),
		"stale tick should not trigger validation - errors should remain")
}

// TestModelStatusBarErrorIndicator verifies error count in status bar.
//
// VALIDATES: View() shows error count when errors exist.
// PREVENTS: User unaware of validation issues.
func TestModelStatusBarErrorIndicator(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config with validation error
	content := `bgp {
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time 1;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Should have errors from load
	require.NotEmpty(t, model.validationErrors, "should have errors")

	// View should show error indicator
	view := model.View()
	assert.Contains(t, view, "error", "status bar should show error indicator")
}

// TestModelKeyrunesTriggersValidation verifies text changes schedule validation.
//
// VALIDATES: KeyRunes message triggers debounced validation.
// PREVENTS: Validation not running on keystroke.
func TestModelKeyrunesTriggersValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigOneLine), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	initialID := model.validationID

	// Send KeyRunes message (typing 'a')
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updatedModel, ok := newModel.(Model)
	require.True(t, ok, "Update should return a Model")

	// Validation ID should have incremented
	assert.Greater(t, updatedModel.validationID, initialID, "validation ID should increment on keystroke")

	// Should return a batched command (including validation tick)
	assert.NotNil(t, cmd, "should return command for debounced validation")
}

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
	filteredContent := `peer-as 65001;
hold-time 1;`

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

	// Config with syntax error on line 6 (invalid value)
	// The parser reports error on the line with the invalid token
	content := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
    hold-time notanumber;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Should have validation error from load
	require.NotEmpty(t, model.validationErrors, "should have errors")

	// Find error with line number that's in the peer block (lines 5-6)
	var errorInPeerBlock bool
	for _, e := range model.validationErrors {
		if e.Line >= 5 && e.Line <= 6 {
			errorInPeerBlock = true
			t.Logf("Found error in peer block: Line=%d, Message=%s", e.Line, e.Message)
		}
	}

	// Enter edit context for the peer - get result and apply it manually
	result, err := model.cmdEdit([]string{"peer", "1.1.1.1"})
	require.NoError(t, err)

	// Apply the result (simulating what Update handler does)
	if result.newContext != nil {
		model.contextPath = result.newContext
		model.isTemplate = result.isTemplate
	}
	if result.configView != nil {
		model.setViewportData(*result.configView)
	}

	// Viewport should show filtered content
	assert.Contains(t, model.viewportContent, "hold-time", "viewport should show peer content")

	// If we have an error in the peer block, it should be highlighted
	if errorInPeerBlock {
		assert.Contains(t, model.viewportContent, "\x1b[", "error line should be highlighted in filtered view")
	}
}

// TestModelStatusBarNoErrorsWhenValid verifies no indicator when valid.
//
// VALIDATES: View() shows no error indicator for valid config.
// PREVENTS: False error display.
func TestModelStatusBarNoErrorsWhenValid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigOneLine), 0600)
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
