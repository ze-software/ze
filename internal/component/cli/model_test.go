package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Test config constants to avoid duplication.
const (
	testValidBGPConfig         = `bgp { router-id 1.2.3.4; session { asn { local 65000; } } }`
	testValidBGPConfigOneLine  = `bgp { router-id 1.2.3.4; session { asn { local 65000; } } }`
	testValidBGPConfigWithPeer = `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 90; }
  }
}`
	// testValidBGPConfigSimplePeer is for tests that don't need receive-hold-time.
	testValidBGPConfigSimplePeer = `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
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

	// Write config with validation error (receive-hold-time 1 inside peer)
	content := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 1; }
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

	// Write config with validation error (receive-hold-time 2 inside peer)
	content := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 2; }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Commit should fail — status message reports the block, config stays in viewport
	result, err := model.cmdCommit()
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "blocked")
	assert.NotNil(t, result.configView, "config should stay visible on commit failure")
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
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 90; }
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
	view := model.View().Content
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
	ed.SetWorkingContent(`bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 1; } } }`)
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
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 1; }
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
	view := model.View().Content
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
	newModel, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
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
	assert.Contains(t, m.StatusMessage(), "Pending changes", "should show pending changes warning")
}

// TestEscapeClearsInputInsteadOfQuitting verifies that Escape clears
// non-empty input rather than starting the quit confirmation flow.
//
// VALIDATES: Escape with text in the input clears it and returns to config view.
// PREVENTS: Accidentally quitting when the user just wants to discard typed input.
func TestEscapeClearsInputInsteadOfQuitting(t *testing.T) {
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

	// Type something into the input
	model.textInput.SetValue("set bgp invalid-thing")

	// Press Escape
	newModel, cmd := model.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, ok := newModel.(Model)
	require.True(t, ok)

	assert.Equal(t, "", m.InputValue(), "Escape should clear the input")
	assert.False(t, m.confirmQuit, "Escape should not trigger quit confirmation")
	assert.Equal(t, "", m.StatusMessage(), "Escape should clear status message")
	assert.Nil(t, cmd, "Escape clearing input should return no command")
}

// TestEscapeEmptyInputTriggersQuit verifies that Escape with empty input
// and config already showing starts the quit confirmation flow.
//
// VALIDATES: Escape on empty input with config view behaves as before (quit confirmation).
// PREVENTS: Breaking the existing quit shortcut.
func TestEscapeEmptyInputTriggersQuit(t *testing.T) {
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

	// Simulate config view already displayed (as after WindowSizeMsg)
	model.showConfigContent()

	// Input is empty, press Escape
	newModel, _ := model.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, ok := newModel.(Model)
	require.True(t, ok)

	assert.True(t, m.confirmQuit, "Escape on empty input should trigger quit confirmation")
}

// TestEscapeAfterErrorsReturnsToConfig verifies that Escape returns to config
// view after running a command that replaced the viewport (like "errors").
//
// VALIDATES: Escape restores config view when viewport shows command output.
// PREVENTS: Being forced to quit when you just want to dismiss command output.
func TestEscapeAfterErrorsReturnsToConfig(t *testing.T) {
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

	// Simulate command output in viewport (like "errors" does)
	model.setViewportText("1 issue(s):\n  line 3: missing required field")
	assert.False(t, model.showingConfig, "viewport should not be showing config after setViewportText")

	// Press Escape with empty input
	newModel, _ := model.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, ok := newModel.(Model)
	require.True(t, ok)

	assert.False(t, m.confirmQuit, "Escape should not trigger quit when showing command output")
	assert.True(t, m.showingConfig, "Escape should restore config view")

	// Press Escape again — now it should trigger quit
	newModel2, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	m2, ok := newModel2.(Model)
	require.True(t, ok)

	assert.True(t, m2.confirmQuit, "second Escape should trigger quit confirmation")
}

// TestCtrlCStillQuitsWithInput verifies that Ctrl+C triggers quit even when
// there is text in the input (only Escape gets the clear-input behavior).
//
// VALIDATES: Ctrl+C always triggers quit confirmation regardless of input.
// PREVENTS: Ctrl+C losing its "I want to leave" meaning.
func TestCtrlCStillQuitsWithInput(t *testing.T) {
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

	model.textInput.SetValue("set bgp something")

	newModel, _ := model.handleKeyMsg(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m, ok := newModel.(Model)
	require.True(t, ok)

	assert.True(t, m.confirmQuit, "Ctrl+C should trigger quit even with input text")
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
	model.history.Append("show")
	model.history.Append("edit bgp")

	// Press Up - should recall most recent command
	m := model.handleHistoryUp()
	updated, ok := m.(Model)
	require.True(t, ok)
	assert.Equal(t, "edit bgp", updated.InputValue(), "first Up should recall most recent")

	// Press Up again - should recall older command
	m = updated.handleHistoryUp()
	updated, ok = m.(Model)
	require.True(t, ok)
	assert.Equal(t, "show", updated.InputValue(), "second Up should recall older command")

	// Press Down - should return to more recent
	m = updated.handleHistoryDown()
	updated, ok = m.(Model)
	require.True(t, ok)
	assert.Equal(t, "edit bgp", updated.InputValue(), "Down should go to more recent")

	// Press Down again - should restore original input
	m = updated.handleHistoryDown()
	updated, ok = m.(Model)
	require.True(t, ok)
	assert.Empty(t, updated.InputValue(), "Down past end restores original input")
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

	assert.Equal(t, []string{"show"}, m.history.Entries(), "command should be saved to history")
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

	assert.Equal(t, []string{"show"}, m.history.Entries(), "duplicate consecutive commands should not be added")
}

// TestSetHistoryLoadsCurrentMode verifies SetHistory loads saved entries for the active mode.
//
// VALIDATES: SetHistory populates current mode history from store.
// PREVENTS: History empty after restart despite saved entries.
func TestSetHistoryLoadsCurrentMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	// Pre-populate store with edit history.
	storePath := filepath.Join(tmpDir, "test.zefs")
	store, err := zefs.Create(storePath)
	require.NoError(t, err)
	require.NoError(t, store.WriteFile("meta/history/testuser/edit", []byte("show\ncommit"), 0))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	m, err := NewModel(ed) // starts in ModeEdit
	require.NoError(t, err)

	m.SetHistory(NewHistory(store, "testuser"))
	store.Close() //nolint:errcheck // test cleanup

	assert.Equal(t, []string{"show", "commit"}, m.history.Entries(), "should load edit history from store")
}

// TestSetHistoryPreloadsOtherMode verifies SetHistory pre-loads the other mode's history.
//
// VALIDATES: Switching to command mode after SetHistory has saved command history.
// PREVENTS: Other mode history lost on first mode switch.
func TestSetHistoryPreloadsOtherMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	storePath := filepath.Join(tmpDir, "test.zefs")
	store, err := zefs.Create(storePath)
	require.NoError(t, err)
	require.NoError(t, store.WriteFile("meta/history/testuser/edit", []byte("show"), 0))
	require.NoError(t, store.WriteFile("meta/history/testuser/command", []byte("peer list\ndaemon status"), 0))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	m, err := NewModel(ed)
	require.NoError(t, err)

	m.SetHistory(NewHistory(store, "testuser"))
	store.Close() //nolint:errcheck // test cleanup

	// Switch to command mode.
	m.SwitchMode(ModeCommand)
	assert.Equal(t, []string{"peer list", "daemon status"}, m.history.Entries(),
		"command history should be pre-loaded from store")

	// Switch back to edit.
	m.SwitchMode(ModeEdit)
	assert.Equal(t, []string{"show"}, m.history.Entries(),
		"edit history should survive mode round-trip")
}

// TestModelHistoryPersistOnEnter verifies that Enter persists history through a real store.
//
// VALIDATES: Executed commands are saved to the store and survive reload.
// PREVENTS: History lost on restart because Save was never called.
func TestModelHistoryPersistOnEnter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	storePath := filepath.Join(tmpDir, "test.zefs")
	store, err := zefs.Create(storePath)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.width = 80
	model.height = 24
	model.SetHistory(NewHistory(store, "testuser"))

	// Execute "show"
	model.textInput.SetValue("show")
	newModel, _ := model.handleEnter()
	_, ok := newModel.(Model)
	require.True(t, ok)

	// Reload history from the same store.
	h2 := NewHistory(store, "testuser")
	loaded := h2.Load("edit")
	store.Close() //nolint:errcheck // test cleanup

	assert.Equal(t, []string{"show"}, loaded, "command should be persisted to store via Save")
}

// TestCommandModelHistoryPersistOnEnter verifies that Enter in command-only mode
// (NewCommandModel, used by ze cli) persists history through a real store.
//
// VALIDATES: AC-2: Commands in ze cli survive restart.
// PREVENTS: Command-mode history lost because Save uses wrong mode key.
func TestCommandModelHistoryPersistOnEnter(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "test.zefs")
	store, err := zefs.Create(storePath)
	require.NoError(t, err)

	model := NewCommandModel()
	model.width = 80
	model.height = 24
	model.SetHistory(NewHistory(store, "testuser"))

	// Execute "peer list" (operational command in command mode).
	// Without a commandExecutor, handleEnter falls through to history save.
	model.textInput.SetValue("peer list")
	newModel, _ := model.handleEnter()
	_, ok := newModel.(Model)
	require.True(t, ok)

	// Reload from store and verify saved under "command" key.
	h2 := NewHistory(store, "testuser")
	loaded := h2.Load("command")
	store.Close() //nolint:errcheck // test cleanup

	assert.Equal(t, []string{"peer list"}, loaded, "command-mode history should be persisted under 'command' key")
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
	assert.Contains(t, texts, "connection", "should show connection in children")
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

// --- Phase 2: Command-only mode (unified CLI) tests ---

// TestModelStartsInEditMode verifies that NewModel creates a model in ModeEdit.
//
// VALIDATES: AC-1 from spec-unified-cli: ze config edit opens in Edit mode.
// PREVENTS: Editor model accidentally starting in command mode.
func TestModelStartsInEditMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	m, err := NewModel(ed)
	require.NoError(t, err)
	assert.Equal(t, ModeEdit, m.Mode(), "editor model should start in ModeEdit")
	assert.True(t, m.hasEditor(), "editor model should have an editor")
}

// TestModelStartsInCommandMode verifies that NewCommandModel creates a model
// in ModeCommand with nil editor.
//
// VALIDATES: AC-2 from spec-unified-cli: ze cli opens in Command mode.
// PREVENTS: Command-only model accidentally starting in edit mode.
func TestModelStartsInCommandMode(t *testing.T) {
	m := NewCommandModel()
	assert.Equal(t, ModeCommand, m.Mode(), "command model should start in ModeCommand")
	assert.False(t, m.hasEditor(), "command model should have no editor")
}

// TestEditCommandsUnavailableWithoutEditor verifies that edit commands return
// errors when the model has no editor.
//
// VALIDATES: AC-3 from spec-unified-cli: edit commands unavailable without config.
// PREVENTS: Nil dereference when dispatching set/delete/commit without editor.
func TestEditCommandsUnavailableWithoutEditor(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	editCommands := []string{
		"set bgp router-id 1.2.3.4",
		"delete bgp",
		"show",
		"compare",
		"commit",
		"discard",
		"history",
		"rollback 1",
		"load file absolute replace test.conf",
		"errors",
		"top",
		"up",
	}

	for _, cmd := range editCommands {
		result, err := m.dispatchCommand(cmd)
		assert.Error(t, err, "command %q should fail without editor", cmd)
		assert.Empty(t, result.output, "command %q should produce no output", cmd)
	}
}

// TestModeEditBlockedWithoutEditor verifies that typing "edit" or an edit command
// in command-only mode returns an error and stays in ModeCommand.
//
// VALIDATES: AC-3 from spec-unified-cli: mode switch blocked without config.
// PREVENTS: Entering ModeEdit with nil editor causing nil panics in updateCompletions/View.
func TestModeEditBlockedWithoutEditor(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	// Try "edit" command — should stay in ModeCommand
	m.textInput.SetValue("edit")
	newModel, _ := m.handleEnter()
	updated, ok := newModel.(Model)
	require.True(t, ok)
	assert.Equal(t, ModeCommand, updated.Mode(), "should stay in ModeCommand after 'edit'")
	assert.NotEmpty(t, updated.statusMessage, "should show error message")

	// Try "set bgp router-id 1.2.3.4" — should stay in ModeCommand
	updated.textInput.SetValue("set bgp router-id 1.2.3.4")
	newModel, _ = updated.handleEnter()
	updated2, ok := newModel.(Model)
	require.True(t, ok)
	assert.Equal(t, ModeCommand, updated2.Mode(), "should stay in ModeCommand after 'set'")
}

// TestViewRendersWithoutEditor verifies that View() does not panic and renders
// correct header when the model has no editor.
//
// VALIDATES: BLOCKER-1 from spec review: View() m.editor.Dirty() nil guard.
// PREVENTS: Panic on every render frame in command-only mode.
func TestViewRendersWithoutEditor(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	// Should not panic
	output := m.View().Content
	assert.Contains(t, output, "Ze CLI", "header should say Ze CLI, not Ze Editor")
	assert.NotContains(t, output, "[modified]", "should not show modified indicator")
}

// TestShiftArrowLineScroll verifies that Shift+Up/Down scrolls the viewport one line.
//
// VALIDATES: AC-6 from spec-unified-cli: Shift+Arrow scrolls one line.
// PREVENTS: Regression of existing line-by-line scrolling.
func TestShiftArrowLineScroll(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	// Create config with enough content to fill viewport
	var content strings.Builder
	content.WriteString("bgp {\n  router-id 1.2.3.4\n")
	for i := range 50 {
		fmt.Fprintf(&content, "  # line %d\n", i)
	}
	content.WriteString("}\n")
	err := os.WriteFile(configPath, []byte(content.String()), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Initialize with window size to activate viewport
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, ok := newModel.(Model)
	require.True(t, ok)

	// Populate viewport with scrollable content
	m.viewport.SetContent(strings.Repeat("line\n", 100))
	m.showViewport = true
	initialOffset := m.viewport.YOffset()

	// Shift+Down should scroll exactly one line
	newModel, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	m, ok = newModel.(Model)
	require.True(t, ok)
	assert.Equal(t, initialOffset+1, m.viewport.YOffset(), "Shift+Down should scroll down one line")

	// Shift+Up should scroll back one line
	newModel, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	m, ok = newModel.(Model)
	require.True(t, ok)
	assert.Equal(t, initialOffset, m.viewport.YOffset(), "Shift+Up should scroll up one line")
}

// TestCtrlArrowPageScroll verifies that Ctrl+Up/Down scrolls the viewport one page.
//
// VALIDATES: AC-7 from spec-unified-cli: Ctrl+Arrow pages viewport.
// PREVENTS: Missing keyboard shortcut for page scrolling.
func TestCtrlArrowPageScroll(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	// Create config with enough content to fill viewport
	var content strings.Builder
	content.WriteString("bgp {\n  router-id 1.2.3.4\n")
	for i := range 50 {
		fmt.Fprintf(&content, "  # line %d\n", i)
	}
	content.WriteString("}\n")
	err := os.WriteFile(configPath, []byte(content.String()), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Initialize with window size to activate viewport
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, ok := newModel.(Model)
	require.True(t, ok)

	// Scroll down to populate some offset
	m.viewport.SetContent(strings.Repeat("line\n", 100))
	m.showViewport = true
	initialOffset := m.viewport.YOffset()

	// Ctrl+Down should page down
	newModel, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	m, ok = newModel.(Model)
	require.True(t, ok)
	assert.Greater(t, m.viewport.YOffset(), initialOffset, "Ctrl+Down should scroll down")

	// Ctrl+Up should page up
	afterDown := m.viewport.YOffset()
	newModel, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
	m, ok = newModel.(Model)
	require.True(t, ok)
	assert.Less(t, m.viewport.YOffset(), afterDown, "Ctrl+Up should scroll up")
}

// --- Phase 5: Plugin CLI tests ---

// TestPluginCommandCompleter verifies plugin SDK method completions.
//
// VALIDATES: AC-11 from spec-unified-cli: tab completion for plugin SDK methods.
// PREVENTS: Plugin CLI having no completions or wrong completions.
func TestPluginCommandCompleter(t *testing.T) {
	pc := NewPluginCompleter()

	// Empty input should return all methods
	all := pc.Complete("")
	assert.GreaterOrEqual(t, len(all), 10, "should have at least 10 plugin SDK methods")

	// Partial match
	comps := pc.Complete("up")
	require.Len(t, comps, 1)
	assert.Equal(t, "update-route", comps[0].Text)

	// Ghost text for partial input
	ghost := pc.GhostText("dec")
	assert.NotEmpty(t, ghost, "should have ghost text for partial 'dec'")

	// No ghost text for empty input
	assert.Empty(t, pc.GhostText(""), "empty input should have no ghost text")

	// After typing full method + space, show argument hint
	argComps := pc.Complete("decode-nlri ")
	require.Len(t, argComps, 1)
	assert.Equal(t, "hint", argComps[0].Type, "argument hint should be type 'hint'")
	assert.Contains(t, argComps[0].Text, "family")

	// Unknown method
	assert.Empty(t, pc.Complete("zzz"), "unknown prefix should have no completions")

	// Verify PluginCompleter satisfies CommandModeCompleter interface
	var _ CommandModeCompleter = pc
}

// TestModelDisplaysLoginWarnings verifies that login warnings appear in View() output.
//
// VALIDATES: AC-1 from spec-login-warnings: Welcome shows warning message and command.
// VALIDATES: AC-4 from spec-login-warnings: Warning includes actionable command.
// PREVENTS: Login warnings silently dropped, not rendered to operator.
func TestModelDisplaysLoginWarnings(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	m.SetLoginWarnings([]LoginWarning{
		{Message: "3 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
	})

	view := m.View().Content
	assert.Contains(t, view, "3 peer(s) have stale prefix data", "warning message should appear")
	assert.Contains(t, view, "ze update bgp peer * prefix", "actionable command should appear")
}

// TestModelNoLoginWarnings verifies that View() renders normally without login warnings.
//
// VALIDATES: AC-2 from spec-login-warnings: No warning in welcome when no stale peers.
// PREVENTS: Empty warning block rendered when warnings are nil.
func TestModelNoLoginWarnings(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	// No SetLoginWarnings called -- loginWarnings is nil
	view := m.View().Content
	assert.NotContains(t, view, "warning:", "no warning block should appear")
	assert.NotContains(t, view, "run:", "no command suggestion should appear")
}

// TestModelMultipleLoginWarnings verifies that multiple warnings all render.
//
// VALIDATES: Multiple warnings render as consecutive blocks.
// PREVENTS: Only first warning displayed, rest silently dropped.
func TestModelMultipleLoginWarnings(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	m.SetLoginWarnings([]LoginWarning{
		{Message: "3 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
		{Message: "RPKI cache expired", Command: "ze update rpki"},
	})

	view := m.View().Content
	assert.Contains(t, view, "3 peer(s) have stale prefix data")
	assert.Contains(t, view, "RPKI cache expired")
	assert.Contains(t, view, "ze update bgp peer * prefix")
	assert.Contains(t, view, "ze update rpki")
}

// TestModelDisplaysLoginWarningsWithEditor verifies warnings render in editor-capable mode.
//
// VALIDATES: AC-1 from spec-login-warnings in editor path.
// PREVENTS: Warnings only working in command-only mode but not editor mode.
func TestModelDisplaysLoginWarningsWithEditor(t *testing.T) {
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

	model.SetLoginWarnings([]LoginWarning{
		{Message: "2 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
	})

	view := model.View().Content
	assert.Contains(t, view, "2 peer(s) have stale prefix data", "warning should appear in editor mode")
	assert.Contains(t, view, "ze update bgp peer * prefix", "command should appear in editor mode")
}

// TestWarningWithEmptyCommand verifies warning renders without "run:" when Command is empty.
//
// VALIDATES: Warning with no actionable command displays message only.
// PREVENTS: Bare "run:" line displayed when Command is empty.
func TestWarningWithEmptyCommand(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	m.SetLoginWarnings([]LoginWarning{
		{Message: "software update available", Command: ""},
	})

	view := m.View().Content
	assert.Contains(t, view, "software update available", "warning message should appear")
	assert.NotContains(t, view, "run:", "no run line when Command is empty")
}

// TestWarningWithEmptyMessage verifies empty Message warnings are skipped.
//
// VALIDATES: Empty Message produces no visual artifact.
// PREVENTS: Bare "warning: " line displayed for empty Message.
func TestWarningWithEmptyMessage(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.height = 24

	m.SetLoginWarnings([]LoginWarning{
		{Message: "", Command: "ze update rpki"},
	})

	view := m.View().Content
	assert.NotContains(t, view, "warning:", "empty message should not render warning line")
	assert.NotContains(t, view, "run:", "empty message warning should not render command line")
}
