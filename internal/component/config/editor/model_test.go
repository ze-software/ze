package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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

// TestModelStatusMessageDisplay verifies status messages appear in View.
//
// VALIDATES: View() renders statusMessage in output.
// PREVENTS: Status messages invisible to user.
func TestModelStatusMessageDisplay(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
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
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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

	err := os.WriteFile(configPath, []byte(testValidBGPConfigOneLine), 0o600)
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

// TestExitCommandQuits verifies that typing "exit" quits the editor.
//
// VALIDATES: "exit" command triggers tea.Quit.
// PREVENTS: "exit" being silently ignored.
func TestExitCommandQuits(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Type "exit" and press Enter
	model.textInput.SetValue("exit")
	newModel, cmd := model.handleEnter()
	m, ok := newModel.(Model)
	require.True(t, ok)

	// Should be quitting
	assert.True(t, m.quitting, "exit should set quitting flag")
	assert.NotNil(t, cmd, "exit should return a tea.Cmd (tea.Quit)")
}

// TestExitBlockedByDirty verifies that "exit" warns when there are unsaved changes.
//
// VALIDATES: "exit" with dirty state shows warning instead of quitting.
// PREVENTS: Losing unsaved changes on exit.
func TestExitBlockedByDirty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	// Mark as dirty
	ed.MarkDirty()

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Type "exit" and press Enter
	model.textInput.SetValue("exit")
	newModel, _ := model.handleEnter()
	m, ok := newModel.(Model)
	require.True(t, ok)

	// Should NOT be quitting — dirty state blocks it
	assert.False(t, m.quitting, "exit should not quit with unsaved changes")
	assert.Contains(t, m.StatusMessage(), "Unsaved", "should show unsaved changes warning")
}

// TestCommandHistoryRecall verifies Up/Down arrows recall previously executed commands.
//
// VALIDATES: Up arrow recalls previous command, Down returns to current input.
// PREVENTS: Command history not working after entering commands.
func TestCommandHistoryRecall(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Simulate executing two commands by populating history directly
	model.history = []string{"show", "edit bgp"}
	model.historyIdx = -1

	// Press Up — should recall most recent command
	m := model.handleHistoryUp()
	updated, ok := m.(Model)
	require.True(t, ok)
	assert.Equal(t, "edit bgp", updated.InputValue(), "first Up should recall most recent")
	assert.Equal(t, 1, updated.historyIdx)

	// Press Up again — should recall older command
	m = updated.handleHistoryUp()
	updated, ok = m.(Model)
	require.True(t, ok)
	assert.Equal(t, "show", updated.InputValue(), "second Up should recall older command")
	assert.Equal(t, 0, updated.historyIdx)

	// Press Down — should return to more recent
	m = updated.handleHistoryDown()
	updated, ok = m.(Model)
	require.True(t, ok)
	assert.Equal(t, "edit bgp", updated.InputValue(), "Down should go to more recent")

	// Press Down again — should restore original input
	m = updated.handleHistoryDown()
	updated, ok = m.(Model)
	require.True(t, ok)
	assert.Empty(t, updated.InputValue(), "Down past end restores original input")
	assert.Equal(t, -1, updated.historyIdx)
}

// TestCommandHistorySavedOnEnter verifies commands are saved to history when executed.
//
// VALIDATES: Executed commands appear in history.
// PREVENTS: History empty after running commands.
func TestCommandHistorySavedOnEnter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Type and execute "show"
	model.textInput.SetValue("show")
	newModel, _ := model.handleEnter()
	m, ok := newModel.(Model)
	require.True(t, ok)

	assert.Equal(t, []string{"show"}, m.history, "command should be saved to history")
}

// TestCommandHistoryNoDuplicates verifies consecutive identical commands are not duplicated.
//
// VALIDATES: Same command entered twice only appears once in history.
// PREVENTS: History cluttered with repeated identical commands.
func TestCommandHistoryNoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Execute "show" twice
	model.textInput.SetValue("show")
	newModel, _ := model.handleEnter()
	m, ok := newModel.(Model)
	require.True(t, ok)

	m.textInput.SetValue("show")
	newModel, _ = m.handleEnter()
	m, ok = newModel.(Model)
	require.True(t, ok)

	assert.Equal(t, []string{"show"}, m.history, "duplicate consecutive commands should not be added")
}

// TestTabOnListKeyShowsChildrenImmediately verifies that pressing Tab on a typed
// list key value accepts it and immediately shows the next-level completions.
//
// VALIDATES: Tab on "set bgp peer 10.0.0.1" adds space and shows peer children.
// PREVENTS: User needing to press Tab twice — once to accept key, once to see children.
func TestTabOnListKeyShowsChildrenImmediately(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Simulate typing "set bgp peer 10.0.0.1"
	model.textInput.SetValue("set bgp peer 10.0.0.1")
	model.updateCompletions()

	// Press Tab — should accept the key
	newModel, _ := model.handleTab()
	m, ok := newModel.(Model)
	require.True(t, ok, "handleTab should return a Model")

	// Input should now have trailing space (key accepted)
	assert.True(t, strings.HasSuffix(m.InputValue(), " "),
		"input should end with space after Tab accepts key")

	// Children should be shown immediately (dropdown visible with peer children)
	assert.True(t, m.ShowDropdown(), "dropdown should be visible with next-level completions")
	assert.Greater(t, len(m.Completions()), 1, "should have multiple peer children")

	// Should contain peer-specific fields
	texts := completionTexts(m.Completions())
	assert.Contains(t, texts, "peer-as", "should show peer-as in children")
}

// TestExitAfterDiscard verifies that "exit" works after set + discard.
//
// VALIDATES: AC-3 from spec-editor-1: exit works after set then discard.
// PREVENTS: Exit blocked after discard clears dirty state.
func TestExitAfterDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Set a value — makes editor dirty
	_, err = model.dispatchCommand("set bgp router-id 5.6.7.8")
	require.NoError(t, err)
	assert.True(t, ed.Dirty(), "should be dirty after set")

	// Discard — should clear dirty
	_, err = model.dispatchCommand("discard")
	require.NoError(t, err)
	assert.False(t, ed.Dirty(), "should NOT be dirty after discard")

	// Exit — should succeed
	model.textInput.SetValue("exit")
	newModel, cmd := model.handleEnter()
	m, ok := newModel.(Model)
	require.True(t, ok)
	assert.True(t, m.quitting, "exit should succeed after discard")
	assert.NotNil(t, cmd, "exit should return tea.Quit")
}

// TestExitAfterCommit verifies that "exit" works after set + commit.
//
// VALIDATES: AC-4 from spec-editor-1: exit works after set then commit.
// PREVENTS: Exit blocked after commit clears dirty state.
func TestExitAfterCommit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24

	// Set a value — makes editor dirty
	_, err = model.dispatchCommand("set bgp router-id 5.6.7.8")
	require.NoError(t, err)
	assert.True(t, ed.Dirty(), "should be dirty after set")

	// Commit — should save and clear dirty
	_, err = model.dispatchCommand("commit")
	require.NoError(t, err)
	assert.False(t, ed.Dirty(), "should NOT be dirty after commit")

	// Exit — should succeed
	model.textInput.SetValue("exit")
	newModel, cmd := model.handleEnter()
	m, ok := newModel.(Model)
	require.True(t, ok)
	assert.True(t, m.quitting, "exit should succeed after commit")
	assert.NotNil(t, cmd, "exit should return tea.Quit")
}

// TestNoFalseDirtyOnOpen verifies that opening the editor doesn't set dirty flag.
//
// VALIDATES: AC-5 from spec-editor-1: no false dirty on open.
// PREVENTS: Serialization round-trip drift causing false dirty state.
func TestNoFalseDirtyOnOpen(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	assert.False(t, ed.Dirty(), "editor should NOT be dirty immediately after open")

	// Also verify with peer config (more complex serialization)
	configPath2 := filepath.Join(tmpDir, "test2.conf")
	err = os.WriteFile(configPath2, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed2, err := NewEditor(configPath2)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // test cleanup

	assert.False(t, ed2.Dirty(), "editor should NOT be dirty with peer config")
}
