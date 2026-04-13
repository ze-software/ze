package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// Execute commit confirm with 60 second timeout
	result, err := model.cmdCommitConfirmed(60, false)
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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// 0 seconds should fail
	_, err = model.cmdCommitConfirmed(0, false)
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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// 3601 seconds should fail
	_, err = model.cmdCommitConfirmed(3601, false)
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

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// Start commit confirm
	commitResult, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err)
	model.ApplyResult(commitResult)
	require.True(t, model.ConfirmTimerActive(), "timer should be active")

	// Confirm
	result, err := model.cmdConfirm()
	require.NoError(t, err)
	model.ApplyResult(result)

	// Timer should be canceled
	assert.False(t, model.ConfirmTimerActive(), "timer should be canceled after confirm")
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
  router-id 1.2.3.4
  session { asn { local 65000; } }
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
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
	commitResult, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err)
	model.ApplyResult(commitResult)

	// Abort - should rollback
	result, err := model.cmdAbort()
	require.NoError(t, err)
	model.ApplyResult(result)

	// Timer should be canceled
	assert.False(t, model.ConfirmTimerActive(), "timer should be canceled after abort")
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
	loadContent := `bgp { router-id 5.6.7.8; session { asn { local 65000; } } }`

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(loadContent), 0o600)
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
	assert.Contains(t, ed.WorkingContent(), "session", "content should have session")
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
  router-id 1.2.3.4
  session { asn { local 65000; } }
}`
	mergeContent := `bgp {
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

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(mergePath, []byte(mergeContent), 0o600)
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
	assert.Contains(t, ed.WorkingContent(), "local", "original local should be preserved")

	// New content should be added
	assert.Contains(t, ed.WorkingContent(), "peer peer1", "new peer should be added")
	assert.Contains(t, ed.WorkingContent(), "remote", "remote should be added")

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
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
	err := os.MkdirAll(subDir, 0o750)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "test.conf")
	loadPath := filepath.Join(tmpDir, "load.conf")

	err = os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(`bgp { router-id 9.9.9.9; }`), 0o600)
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

// =============================================================================
// Phase 3: Load Command Redesign
// New syntax: load <source> <location> <action> [file]
// =============================================================================

// TestParseLoadArgsValid verifies parseLoadArgs handles all valid keyword combinations.
//
// VALIDATES: All 8 valid keyword combinations are parsed correctly.
// PREVENTS: Valid load commands rejected.
func TestParseLoadArgsValid(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantSource   string
		wantLocation string
		wantAction   string
		wantPath     string
	}{
		{
			name:         "file_absolute_replace",
			args:         []string{"file", "absolute", "replace", "/path/to/config.conf"},
			wantSource:   "file",
			wantLocation: "absolute",
			wantAction:   "replace",
			wantPath:     "/path/to/config.conf",
		},
		{
			name:         "file_absolute_merge",
			args:         []string{"file", "absolute", "merge", "/path/to/config.conf"},
			wantSource:   "file",
			wantLocation: "absolute",
			wantAction:   "merge",
			wantPath:     "/path/to/config.conf",
		},
		{
			name:         "file_relative_replace",
			args:         []string{"file", "relative", "replace", "other.conf"},
			wantSource:   "file",
			wantLocation: "relative",
			wantAction:   "replace",
			wantPath:     "other.conf",
		},
		{
			name:         "file_relative_merge",
			args:         []string{"file", "relative", "merge", "snippet.conf"},
			wantSource:   "file",
			wantLocation: "relative",
			wantAction:   "merge",
			wantPath:     "snippet.conf",
		},
		{
			name:         "terminal_absolute_replace",
			args:         []string{"terminal", "absolute", "replace"},
			wantSource:   "terminal",
			wantLocation: "absolute",
			wantAction:   "replace",
			wantPath:     "",
		},
		{
			name:         "terminal_absolute_merge",
			args:         []string{"terminal", "absolute", "merge"},
			wantSource:   "terminal",
			wantLocation: "absolute",
			wantAction:   "merge",
			wantPath:     "",
		},
		{
			name:         "terminal_relative_replace",
			args:         []string{"terminal", "relative", "replace"},
			wantSource:   "terminal",
			wantLocation: "relative",
			wantAction:   "replace",
			wantPath:     "",
		},
		{
			name:         "terminal_relative_merge",
			args:         []string{"terminal", "relative", "merge"},
			wantSource:   "terminal",
			wantLocation: "relative",
			wantAction:   "merge",
			wantPath:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, location, action, path, err := parseLoadArgs(tt.args)
			require.NoError(t, err, "valid args should not error")
			assert.Equal(t, tt.wantSource, source, "source mismatch")
			assert.Equal(t, tt.wantLocation, location, "location mismatch")
			assert.Equal(t, tt.wantAction, action, "action mismatch")
			assert.Equal(t, tt.wantPath, path, "path mismatch")
		})
	}
}

// TestParseLoadArgsErrors verifies parseLoadArgs rejects invalid inputs.
//
// VALIDATES: Missing/invalid keywords produce clear errors.
// PREVENTS: Silent failures or cryptic error messages.
func TestParseLoadArgsErrors(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErrText string
	}{
		{
			name:        "empty_args",
			args:        []string{},
			wantErrText: "source",
		},
		{
			name:        "missing_location_and_action",
			args:        []string{"file"},
			wantErrText: "location",
		},
		{
			name:        "missing_action",
			args:        []string{"file", "absolute"},
			wantErrText: "action",
		},
		{
			name:        "file_missing_path",
			args:        []string{"file", "absolute", "replace"},
			wantErrText: "path",
		},
		{
			name:        "invalid_source",
			args:        []string{"stdin", "absolute", "replace"},
			wantErrText: "source",
		},
		{
			name:        "invalid_location",
			args:        []string{"file", "root", "replace", "test.conf"},
			wantErrText: "location",
		},
		{
			name:        "invalid_action",
			args:        []string{"file", "absolute", "overwrite", "test.conf"},
			wantErrText: "action",
		},
		{
			name:        "old_syntax_direct_file",
			args:        []string{"test.conf"},
			wantErrText: "source",
		},
		{
			name:        "old_syntax_merge_file",
			args:        []string{"merge", "test.conf"},
			wantErrText: "source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := parseLoadArgs(tt.args)
			require.Error(t, err, "invalid args should error")
			assert.Contains(t, err.Error(), tt.wantErrText, "error should mention %s", tt.wantErrText)
		})
	}
}

// TestLoadFileAbsoluteReplace verifies "load file absolute replace <path>" replaces entire config.
//
// VALIDATES: Content is completely replaced with file contents.
// PREVENTS: Old content remaining after replace.
func TestLoadFileAbsoluteReplace(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	loadPath := filepath.Join(tmpDir, "load.conf")

	originalContent := `bgp { router-id 1.2.3.4; }`
	loadContent := `bgp { router-id 5.6.7.8; session { asn { local 65000; } } }`

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(loadContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Execute new load command
	result, err := model.dispatchCommand("load file absolute replace " + loadPath)
	require.NoError(t, err)

	// Content should be fully replaced
	assert.Contains(t, ed.WorkingContent(), "5.6.7.8", "should have new router-id")
	assert.Contains(t, ed.WorkingContent(), "session", "should have session")
	assert.NotContains(t, ed.WorkingContent(), "1.2.3.4", "old router-id should be gone")
	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.Contains(t, result.statusMessage, "loaded", "status should mention loaded")
}

// TestLoadFileAbsoluteMerge verifies "load file absolute merge <path>" merges at root.
//
// VALIDATES: Existing config preserved, new content added.
// PREVENTS: Merge overwriting existing values.
func TestLoadFileAbsoluteMerge(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	mergePath := filepath.Join(tmpDir, "merge.conf")

	originalContent := `bgp {
  router-id 1.2.3.4
  session { asn { local 65000; } }
}`
	mergeContent := `bgp {
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

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(mergePath, []byte(mergeContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Execute merge
	result, err := model.dispatchCommand("load file absolute merge " + mergePath)
	require.NoError(t, err)

	// Original preserved
	assert.Contains(t, ed.WorkingContent(), "router-id", "original router-id preserved")
	assert.Contains(t, ed.WorkingContent(), "local", "original local-as preserved")

	// New content added
	assert.Contains(t, ed.WorkingContent(), "peer peer1", "peer added")
	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.Contains(t, result.statusMessage, "merged", "status should mention merged")
}

// TestLoadFileRelativeReplace verifies "load file relative replace <path>" replaces context subtree.
//
// VALIDATES: Only content at current context is replaced.
// PREVENTS: Entire config being replaced when in context.
func TestLoadFileRelativeReplace(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	loadPath := filepath.Join(tmpDir, "peer.conf")

	originalContent := `bgp {
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
	// Content to replace the peer block with
	loadContent := `connection { remote { ip 1.1.1.1; } } session { asn { remote 65002; } }
description "new peer"`

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(loadContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Load relative replace - should replace peer block content only
	result, err := model.dispatchCommand("load file relative replace " + loadPath)
	require.NoError(t, err)

	// Root config should be preserved
	assert.Contains(t, ed.WorkingContent(), "router-id", "root router-id preserved")
	assert.Contains(t, ed.WorkingContent(), "session", "root session preserved")

	// Peer content should be replaced
	assert.Contains(t, ed.WorkingContent(), "65002", "new remote asn present")
	assert.Contains(t, ed.WorkingContent(), "description", "new description present")
	assert.NotContains(t, ed.WorkingContent(), "receive-hold-time", "old receive-hold-time removed")
	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.True(t, result.revalidate, "should trigger revalidation")
}

// TestLoadFileRelativeMerge verifies "load file relative merge <path>" merges at context.
//
// VALIDATES: Content merged at current context position.
// PREVENTS: Content merged at root instead of context.
func TestLoadFileRelativeMerge(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	mergePath := filepath.Join(tmpDir, "extra.conf")

	originalContent := `bgp {
  router-id 1.2.3.4
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
	mergeContent := `description "merged peer"
timer { receive-hold-time 180; }`

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(mergePath, []byte(mergeContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Load relative merge
	result, err := model.dispatchCommand("load file relative merge " + mergePath)
	require.NoError(t, err)

	// Original peer content preserved
	assert.Contains(t, ed.WorkingContent(), "65001", "original remote as preserved")

	// Merged content added
	assert.Contains(t, ed.WorkingContent(), "description", "description merged")
	assert.Contains(t, ed.WorkingContent(), "receive-hold-time", "receive-hold-time merged")
	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.True(t, result.revalidate, "should trigger revalidation")
}

// TestLoadOldSyntaxRejected verifies old "load <file>" syntax is rejected.
//
// VALIDATES: Old syntax produces clear error with new syntax hint.
// PREVENTS: Silent behavior change or confusing error.
func TestLoadOldSyntaxRejected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Try old "load <file>" syntax
	_, err = model.dispatchCommand("load test.conf")
	require.Error(t, err, "old syntax should be rejected")
	assert.Contains(t, err.Error(), "load file", "error should hint at new syntax")
}

// TestLoadOldMergeSyntaxRejected verifies old "load merge <file>" syntax is rejected.
//
// VALIDATES: Old merge syntax produces clear error.
// PREVENTS: Partial old syntax working.
func TestLoadOldMergeSyntaxRejected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Try old "load merge <file>" syntax
	_, err = model.dispatchCommand("load merge test.conf")
	require.Error(t, err, "old merge syntax should be rejected")
	assert.Contains(t, err.Error(), "source", "error should mention invalid source")
}

// TestLoadTerminalEntersPasteMode verifies "load terminal ..." enters paste mode.
//
// VALIDATES: Terminal source triggers paste mode state.
// PREVENTS: Terminal load trying to read nonexistent file.
func TestLoadTerminalEntersPasteMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Execute terminal load command
	result, err := model.dispatchCommand("load terminal absolute replace")
	require.NoError(t, err)

	// Should enter paste mode (signaled via result)
	assert.True(t, result.enterPasteMode, "should enter paste mode")
	assert.Equal(t, "absolute", result.pasteModeLocation, "should remember location")
	assert.Equal(t, "replace", result.pasteModeAction, "should remember action")
	assert.Contains(t, result.statusMessage, "Paste", "status should mention paste mode")
}

// TestLoadFileRelativeReplaceSingleContext verifies relative load works with single-element context.
//
// VALIDATES: Single-element contextPath (e.g., ["bgp"]) doesn't panic.
// PREVENTS: Index out of bounds when contextPath has only 1 element.
func TestLoadFileRelativeReplaceSingleContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	loadPath := filepath.Join(tmpDir, "bgp-content.conf")

	originalContent := `bgp {
  router-id 1.2.3.4
  session { asn { local 65000; } }
}`
	// Content to replace the bgp block with
	loadContent := `router-id 5.6.7.8
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
}`

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(loadPath, []byte(loadContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter bgp context (single-element contextPath = ["bgp"])
	editResult, err := model.cmdEdit([]string{"bgp"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Verify we have single-element context
	assert.Equal(t, []string{"bgp"}, model.ContextPath(), "should have single-element context")

	// Load relative replace - this previously panicked with index out of bounds
	result, err := model.dispatchCommand("load file relative replace " + loadPath)
	require.NoError(t, err, "should not panic with single-element context")

	// Verify NEW content is present
	assert.Contains(t, ed.WorkingContent(), "5.6.7.8", "new router-id present")
	assert.Contains(t, ed.WorkingContent(), "peer peer1", "new peer present")

	// Verify OLD content was removed (critical - proves replace worked, not append)
	assert.NotContains(t, ed.WorkingContent(), "1.2.3.4", "old router-id should be gone")
	assert.NotContains(t, ed.WorkingContent(), "local", "old local should be gone")

	// Verify STRUCTURE: bgp block is preserved (content inside braces)
	content := ed.WorkingContent()
	assert.True(t, strings.HasPrefix(strings.TrimSpace(content), "bgp {"), "should start with bgp block")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(content), "}"), "should end with closing brace")
	newRouterPos := strings.Index(content, "5.6.7.8")
	lastBracePos := strings.LastIndex(content, "}")
	assert.True(t, newRouterPos < lastBracePos, "new content should be inside bgp block")

	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.True(t, result.revalidate, "should trigger revalidation")
}

// TestLoadFileRelativeMergeSingleContext verifies relative merge works with single-element context.
//
// VALIDATES: Single-element contextPath (e.g., ["bgp"]) doesn't panic in mergeAtContext.
// PREVENTS: Index out of bounds when contextPath has only 1 element.
func TestLoadFileRelativeMergeSingleContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	mergePath := filepath.Join(tmpDir, "extra.conf")

	originalContent := `bgp {
  router-id 1.2.3.4
}`
	// Content to merge into the bgp block
	mergeContent := `session { asn { local 65000; } }
description "merged content"`

	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(mergePath, []byte(mergeContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter bgp context (single-element contextPath = ["bgp"])
	editResult, err := model.cmdEdit([]string{"bgp"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Verify we have single-element context
	assert.Equal(t, []string{"bgp"}, model.ContextPath(), "should have single-element context")

	// Load relative merge - exercises mergeAtContext with single-element path
	result, err := model.dispatchCommand("load file relative merge " + mergePath)
	require.NoError(t, err, "should not panic with single-element context")

	// Verify ORIGINAL content preserved
	assert.Contains(t, ed.WorkingContent(), "router-id 1.2.3.4", "original router-id preserved")

	// Verify NEW content merged
	assert.Contains(t, ed.WorkingContent(), "session", "merged session present")
	assert.Contains(t, ed.WorkingContent(), "merged content", "merged description present")

	// Verify STRUCTURE: merged content is INSIDE the bgp block (before final brace)
	content := ed.WorkingContent()
	sessionPos := strings.Index(content, "session")
	lastBracePos := strings.LastIndex(content, "}")
	assert.True(t, sessionPos < lastBracePos, "merged content should be inside bgp block, not after it")

	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.True(t, result.revalidate, "should trigger revalidation")
}

// =============================================================================
// Reload notification tests for commit-confirm/confirm/abort
// =============================================================================

// TestCommitConfirmTriggersReload verifies commit confirm notifies daemon.
//
// VALIDATES: cmdCommitConfirmed calls reload notifier after save.
// PREVENTS: Daemon running old config during confirm window.
func TestCommitConfirmTriggersReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	notified := false
	ed.SetReloadNotifier(func() error {
		notified = true
		return nil
	})

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	result, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err)
	model.ApplyResult(result)

	assert.True(t, notified, "reload notifier should be called during commit-confirm")
	assert.True(t, model.ConfirmTimerActive(), "timer should be active")
}

// TestCommitConfirmReloadFailsGracefully verifies commit confirm proceeds on reload failure.
//
// VALIDATES: Daemon unreachable → commit confirm still succeeds with warning.
// PREVENTS: Commit confirm blocked by daemon not running.
func TestCommitConfirmReloadFailsGracefully(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	ed.SetReloadNotifier(func() error {
		return fmt.Errorf("connection refused")
	})

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	result, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err, "commit confirm should not fail on reload error")
	model.ApplyResult(result)

	assert.Contains(t, result.statusMessage, "reload errors", "status should warn about reload failure")
	assert.True(t, model.ConfirmTimerActive(), "timer should still be active")
}

// TestConfirmTriggersReload verifies confirm command notifies daemon.
//
// VALIDATES: "confirm" after "commit confirm" calls reload notifier.
// PREVENTS: Daemon not refreshed after confirm.
func TestConfirmTriggersReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	reloadCount := 0
	ed.SetReloadNotifier(func() error {
		reloadCount++
		return nil
	})

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// Start commit confirm
	commitResult, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err)
	model.ApplyResult(commitResult)
	assert.Equal(t, 1, reloadCount, "first reload during commit-confirm")

	// Confirm
	confirmResult, err := model.cmdConfirm()
	require.NoError(t, err)
	model.ApplyResult(confirmResult)

	assert.Equal(t, 2, reloadCount, "second reload during confirm")
	assert.False(t, model.ConfirmTimerActive(), "timer should be canceled")
	assert.Contains(t, confirmResult.statusMessage, "confirmed", "status should mention confirmed")
}

// TestAbortTriggersReload verifies abort command notifies daemon after rollback.
//
// VALIDATES: "abort" after "commit confirm" rolls back and reloads.
// PREVENTS: Daemon running new config after abort.
func TestAbortTriggersReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	reloadCount := 0
	ed.SetReloadNotifier(func() error {
		reloadCount++
		return nil
	})

	model, err := NewModel(ed)
	require.NoError(t, err)
	model.editor.MarkDirty()

	// Start commit confirm
	commitResult, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err)
	model.ApplyResult(commitResult)
	assert.Equal(t, 1, reloadCount, "first reload during commit-confirm")

	// Abort - should rollback and reload
	abortResult, err := model.cmdAbort()
	require.NoError(t, err)
	model.ApplyResult(abortResult)

	assert.Equal(t, 2, reloadCount, "second reload during abort")
	assert.False(t, model.ConfirmTimerActive(), "timer should be canceled")
	assert.Contains(t, abortResult.statusMessage, "rolled back", "status should mention rollback")
}

// TestCommitConfirmedSessionRouting verifies commit confirmed routes through
// CommitSession when a session is active.
//
// VALIDATES: cmdCommitConfirmed uses CommitSession() in session mode.
// PREVENTS: Session commit bypassing CommitSession, writing hierarchical format.
func TestCommitConfirmedSessionRouting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set up session.
	session := NewEditSession("testuser", "local")
	ed.SetSession(session)

	// Make a change (creates draft + meta via write-through).
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	// Commit confirmed should succeed using CommitSession path.
	result, err := model.cmdCommitConfirmed(60, false)
	require.NoError(t, err)

	assert.Contains(t, result.statusMessage, "Confirm within",
		"should show confirm message")
	assert.True(t, result.setConfirmTimer, "should set confirm timer")

	// Verify file was written in set format (CommitSession writes set+meta).
	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	configContent := string(data)
	assert.Contains(t, configContent, "set bgp router-id",
		"config should be in set format from CommitSession")
	assert.NotContains(t, configContent, "{",
		"config should not contain hierarchical braces")
}
