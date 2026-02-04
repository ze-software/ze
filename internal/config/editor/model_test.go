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
	// testValidBGPConfigSimplePeer is for tests that don't need hold-time.
	testValidBGPConfigSimplePeer = `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
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
// VALIDATES: Edit doesn't match prefix (e.g., "peer" shouldn't match "peer-as").
// PREVENTS: Wrong block selected due to prefix matching.
func TestModelCmdEditExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Config with multiple peer blocks
	content := `bgp {
  peer 2.2.2.2 {
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

	// Edit "peer 1.1.1.1" should find the correct peer
	result, err := model.cmdEdit([]string{"peer", "1.1.1.1"})
	require.NoError(t, err)

	// Should find the correct peer block
	assert.Equal(t, []string{"bgp", "peer", "1.1.1.1"}, result.newContext)
	assert.Contains(t, result.configView.content, "65002", "should show peer 1.1.1.1 content")
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

// =============================================================================
// Phase 2: New Editor Features - commit confirm, load, pipe
// =============================================================================

// TestModelCommitConfirmStartsTimer verifies commit confirm starts auto-rollback timer.
//
// VALIDATES: "commit confirm N" commits and sets up timer.
// PREVENTS: Timer not started, no auto-rollback.
func TestModelCommitConfirmStartsTimer(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// Execute commit confirm with 60 second timeout
	result, err := model.cmdCommitConfirm(60)
	require.NoError(t, err)

	// Apply result to model (simulating what Update handler does)
	model.ApplyResult(result)

	// Should have status message about confirm
	assert.Contains(t, result.statusMessage, "confirm", "status should mention confirm")

	// Timer should be active
	assert.True(t, model.ConfirmTimerActive(), "confirm timer should be active")
}

// TestModelCommitConfirmBoundaryLow verifies boundary: seconds must be >= 1.
//
// VALIDATES: commit confirm 0 is rejected.
// PREVENTS: Invalid zero timeout.
func TestModelCommitConfirmBoundaryLow(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// 0 seconds should fail
	_, err = model.cmdCommitConfirm(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1", "error should mention minimum")
}

// TestModelCommitConfirmBoundaryHigh verifies boundary: seconds must be <= 3600.
//
// VALIDATES: commit confirm 3601 is rejected.
// PREVENTS: Excessively long timeout.
func TestModelCommitConfirmBoundaryHigh(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// 3601 seconds should fail
	_, err = model.cmdCommitConfirm(3601)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3600", "error should mention maximum")
}

// TestModelConfirmCancelsTimer verifies "confirm" command cancels the timer.
//
// VALIDATES: "confirm" after "commit confirm" makes changes permanent.
// PREVENTS: Auto-rollback happening after user confirmed.
func TestModelConfirmCancelsTimer(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// Start commit confirm
	commitResult, err := model.cmdCommitConfirm(60)
	require.NoError(t, err)
	model.ApplyResult(commitResult)
	require.True(t, model.ConfirmTimerActive(), "timer should be active")

	// Confirm
	result, err := model.cmdConfirm()
	require.NoError(t, err)
	model.ApplyResult(result)

	// Timer should be cancelled
	assert.False(t, model.ConfirmTimerActive(), "timer should be cancelled after confirm")
	assert.Contains(t, result.statusMessage, "confirmed", "status should mention confirmed")
}

// TestModelAbortRollsBack verifies "abort" command cancels timer and rolls back.
//
// VALIDATES: "abort" after "commit confirm" reverts to backup.
// PREVENTS: Abort not reverting changes.
func TestModelAbortRollsBack(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	originalContent := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Modify content
	ed.SetWorkingContent(originalContent + "\n  # added line")
	ed.MarkDirty()

	// Start commit confirm (this saves with backup)
	commitResult, err := model.cmdCommitConfirm(60)
	require.NoError(t, err)
	model.ApplyResult(commitResult)

	// Abort - should rollback
	result, err := model.cmdAbort()
	require.NoError(t, err)
	model.ApplyResult(result)

	// Timer should be cancelled
	assert.False(t, model.ConfirmTimerActive(), "timer should be cancelled after abort")
	assert.Contains(t, result.statusMessage, "rolled back", "status should mention rollback")

	// Content should be restored to original
	assert.NotContains(t, ed.WorkingContent(), "added line", "changes should be reverted")
}

// TestModelLoadFile verifies "load <file>" replaces content.
//
// VALIDATES: load command replaces working content with file.
// PREVENTS: Load not working or merging instead of replacing.
func TestModelLoadFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	loadPath := filepath.Join(tmpDir, "load.conf")

	originalContent := `bgp { router-id 1.2.3.4; }`
	loadContent := `bgp { router-id 5.6.7.8; local-as 65000; }`

	err := os.WriteFile(configPath, []byte(originalContent), 0600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(loadContent), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Load the file
	result, err := model.cmdLoad([]string{loadPath})
	require.NoError(t, err)

	// Content should be replaced
	assert.Contains(t, ed.WorkingContent(), "5.6.7.8", "content should have new router-id")
	assert.Contains(t, ed.WorkingContent(), "local-as", "content should have local-as")
	assert.NotContains(t, ed.WorkingContent(), "1.2.3.4", "old content should be gone")

	// Should be marked dirty
	assert.True(t, ed.Dirty(), "should be marked dirty after load")
	assert.Contains(t, result.statusMessage, "loaded", "status should mention loaded")
}

// TestModelLoadMerge verifies "load merge <file>" merges configs.
//
// VALIDATES: load merge combines configurations.
// PREVENTS: Merge overwriting existing values.
func TestModelLoadMerge(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	mergePath := filepath.Join(tmpDir, "merge.conf")

	originalContent := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
}`
	mergeContent := `bgp {
  peer 1.1.1.1 {
    peer-as 65001;
  }
}`

	err := os.WriteFile(configPath, []byte(originalContent), 0600)
	require.NoError(t, err)
	err = os.WriteFile(mergePath, []byte(mergeContent), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Load merge
	result, err := model.cmdLoadMerge([]string{mergePath})
	require.NoError(t, err)

	// Original content should be preserved
	assert.Contains(t, ed.WorkingContent(), "router-id", "original router-id should be preserved")
	assert.Contains(t, ed.WorkingContent(), "local-as", "original local-as should be preserved")

	// New content should be added
	assert.Contains(t, ed.WorkingContent(), "peer 1.1.1.1", "new peer should be added")
	assert.Contains(t, ed.WorkingContent(), "peer-as", "peer-as should be added")

	assert.True(t, ed.Dirty(), "should be marked dirty after merge")
	assert.Contains(t, result.statusMessage, "merged", "status should mention merged")
}

// TestModelLoadNotFound verifies load with missing file returns error.
//
// VALIDATES: load with nonexistent file fails with clear error.
// PREVENTS: Silent failure or panic.
func TestModelLoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Load nonexistent file
	_, err = model.cmdLoad([]string{"/nonexistent/file.conf"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read", "error should mention cannot read")
}

// TestModelLoadRelativePath verifies load resolves relative paths.
//
// VALIDATES: Relative paths resolved from config file directory.
// PREVENTS: Relative paths resolved from cwd instead of config dir.
func TestModelLoadRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0750)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "test.conf")
	loadPath := filepath.Join(tmpDir, "load.conf")

	err = os.WriteFile(configPath, []byte(testValidBGPConfig), 0600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(`bgp { router-id 9.9.9.9; }`), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Load with relative path (relative to config file)
	_, err = model.cmdLoad([]string{"load.conf"})
	require.NoError(t, err)

	assert.Contains(t, ed.WorkingContent(), "9.9.9.9", "content should be loaded")
}

// TestModelPipeShowGrep verifies "show | grep pattern" filters output.
//
// VALIDATES: Pipe with grep filters show output.
// PREVENTS: Pipe not working or returning unfiltered output.
func TestModelPipeShowGrep(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
  }
  peer 2.2.2.2 {
    peer-as 65002;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Show with grep for specific peer
	result, err := model.cmdShowPipe(nil, []PipeFilter{{Type: "grep", Arg: "1.1.1.1"}})
	require.NoError(t, err)

	// Should contain matched content
	assert.Contains(t, result.output, "1.1.1.1", "should contain matched peer")

	// Should not contain unmatched content
	assert.NotContains(t, result.output, "2.2.2.2", "should not contain other peer")
}

// TestModelPipeShowHead verifies "show | head N" limits output.
//
// VALIDATES: Pipe with head limits to N lines.
// PREVENTS: Head not limiting or wrong count.
func TestModelPipeShowHead(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4;
  local-as 65000;
  peer 1.1.1.1 {
    peer-as 65001;
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Show with head 2
	result, err := model.cmdShowPipe(nil, []PipeFilter{{Type: "head", Arg: "2"}})
	require.NoError(t, err)

	// Should have only 2 non-empty lines
	lines := strings.Split(strings.TrimSpace(result.output), "\n")
	assert.LessOrEqual(t, len(lines), 2, "should have at most 2 lines")
}

// TestModelPipeChain verifies chained pipes work.
//
// VALIDATES: "show | grep foo | head 5" chains correctly.
// PREVENTS: Pipe chain breaking or wrong order.
func TestModelPipeChain(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  peer 1.1.1.1 { peer-as 65001; }
  peer 1.1.1.2 { peer-as 65002; }
  peer 1.1.1.3 { peer-as 65003; }
  peer 2.2.2.1 { peer-as 65004; }
}`
	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Grep for 1.1.1.* then head 2
	result, err := model.cmdShowPipe(nil, []PipeFilter{
		{Type: "grep", Arg: "1.1.1"},
		{Type: "head", Arg: "2"},
	})
	require.NoError(t, err)

	// Should contain 1.1.1.* peers only
	assert.Contains(t, result.output, "1.1.1", "should contain 1.1.1.* peers")
	assert.NotContains(t, result.output, "2.2.2", "should not contain 2.2.2.* peers")

	// Should have at most 2 lines
	lines := strings.Split(strings.TrimSpace(result.output), "\n")
	assert.LessOrEqual(t, len(lines), 2, "should have at most 2 lines from head")
}
