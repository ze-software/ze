package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit a nested block using full path (JUNOS-style: relative to context)
	result, err := model.cmdEdit([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)

	// Should build hierarchical path
	assert.Equal(t, []string{"bgp", "peer", "1.1.1.1"}, result.newContext, "should have hierarchical path")

	// Should show config content (full serialized tree in Part 1)
	assert.NotNil(t, result.configView, "should have config view")
	assert.Contains(t, result.configView.content, "peer-as", "should contain peer block content")
}

// TestModelCmdEditWildcardTemplate verifies edit with wildcard creates template context.
//
// VALIDATES: "edit peer *" creates template mode without requiring exact block.
// PREVENTS: Template editing broken by block-not-found check.
func TestModelCmdEditWildcardTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit with wildcard template — deferred to Part 2/3
	_, err = model.cmdEdit([]string{"peer", "*"})
	require.Error(t, err, "wildcard edit should error (deferred feature)")
	assert.Contains(t, err.Error(), "not yet supported", "should mention not supported")
}

// TestModelCmdEditNotFound verifies edit shows error for nonexistent block.
//
// VALIDATES: Edit command fails with clear error for missing block.
// PREVENTS: Silent failure or confusing state when block doesn't exist.
func TestModelCmdEditNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit "bgp peer 1.1.1.1" using full path (JUNOS-style)
	result, err := model.cmdEdit([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)

	// Should find the correct peer block
	assert.Equal(t, []string{"bgp", "peer", "1.1.1.1"}, result.newContext)
	assert.Contains(t, result.configView.content, "65002", "should contain peer 1.1.1.1 content")
}

// TestModelCmdUp verifies up command goes up one context level.
//
// VALIDATES: Up command navigates to parent block in hierarchy.
// PREVENTS: User unable to navigate out of nested context.
func TestModelCmdUp(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
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

// TestSetCommandModifiesConfig verifies that "set" actually modifies the config content.
//
// VALIDATES: Set command updates working content with new value.
// PREVENTS: Set command only showing status without modifying content.
func TestSetCommandModifiesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	originalContent := `bgp {
	router-id 1.2.3.4;
	peer 1.1.1.1 {
		peer-as 65001;
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set description
	result, err := model.dispatchCommand(`set description "test peer"`)
	require.NoError(t, err)

	// Verify content was modified
	content := ed.WorkingContent()
	assert.Contains(t, content, `description "test peer"`, "description should be added")
	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.Contains(t, result.statusMessage, "Set", "status should mention Set")
}

// TestTokenizeCommandQuotedStrings verifies tokenizer handles quoted strings.
//
// VALIDATES: Quoted strings are kept together as single tokens.
// PREVENTS: Splitting "my peer" into ["my", "peer"].
func TestTokenizeCommandQuotedStrings(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "simple",
			input:  "set key value",
			expect: []string{"set", "key", "value"},
		},
		{
			name:   "quoted value",
			input:  `set description "my description"`,
			expect: []string{"set", "description", "my description"},
		},
		{
			name:   "quoted key (list entry)",
			input:  `set peer "my peer" description "test"`,
			expect: []string{"set", "peer", "my peer", "description", "test"},
		},
		{
			name:   "multiple quoted",
			input:  `edit "block name" "sub block"`,
			expect: []string{"edit", "block name", "sub block"},
		},
		{
			name:   "empty string",
			input:  "",
			expect: nil,
		},
		{
			name:   "escaped quote in value",
			input:  `set description "value with \" quote"`,
			expect: []string{"set", "description", `value with " quote`},
		},
		{
			name:   "escaped backslash",
			input:  `set path "C:\\Users\\test"`,
			expect: []string{"set", "path", `C:\Users\test`},
		},
		{
			name:   "quote only token",
			input:  `set value "\""`,
			expect: []string{"set", "value", `"`},
		},
		{
			name:   "backslash at end (not escape)",
			input:  `set path C:\Users`,
			expect: []string{"set", "path", `C:\Users`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizeCommand(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestSetCommandUpdatesExistingValue verifies set replaces existing values.
//
// VALIDATES: Existing key values are replaced, not duplicated.
// PREVENTS: Multiple entries for same key after set.
func TestSetCommandUpdatesExistingValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	originalContent := `bgp {
	router-id 1.2.3.4;
	peer 1.1.1.1 {
		peer-as 65001;
		description "old value";
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set new description (should replace existing)
	_, err = model.dispatchCommand(`set description "new value"`)
	require.NoError(t, err)

	// Verify content was updated (not duplicated)
	content := ed.WorkingContent()
	assert.Contains(t, content, `description "new value"`, "new value should be present")
	assert.NotContains(t, content, "old value", "old value should be replaced")
	// Count occurrences of "description" key - should be exactly 1
	count := strings.Count(content, "description")
	assert.Equal(t, 1, count, "should have exactly one description entry")
}

// TestSetCommandRejectsInvalidValue verifies that set rejects values
// that don't match the YANG leaf type.
//
// VALIDATES: "set hold-time abc" returns error for non-numeric value.
// PREVENTS: Invalid typed values being accepted and only caught at commit.
func TestSetCommandRejectsInvalidValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set hold-time to invalid string — should fail
	_, err = model.dispatchCommand("set hold-time abc")
	require.Error(t, err, "should reject non-numeric hold-time")
	assert.Contains(t, err.Error(), "invalid value")

	// Set hold-time to valid value — should succeed
	_, err = model.dispatchCommand("set hold-time 180")
	require.NoError(t, err, "should accept valid numeric hold-time")
}

// TestJoinTokensWithQuotes verifies quote handling in token rejoining.
//
// VALIDATES: Tokens with spaces, embedded quotes, and empty strings are properly quoted.
// PREVENTS: Malformed command strings from completion.
func TestJoinTokensWithQuotes(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		expect string
	}{
		{
			name:   "simple tokens",
			tokens: []string{"set", "key", "value"},
			expect: "set key value",
		},
		{
			name:   "token with space",
			tokens: []string{"set", "peer", "my peer"},
			expect: `set peer "my peer"`,
		},
		{
			name:   "embedded quote escaped",
			tokens: []string{"set", "description", `my "special" peer`},
			expect: `set description "my \"special\" peer"`,
		},
		{
			name:   "empty string quoted",
			tokens: []string{"set", "description", ""},
			expect: `set description ""`,
		},
		{
			name:   "multiple spaces preserved",
			tokens: []string{"set", "value", "a    b"},
			expect: `set value "a    b"`,
		},
		{
			name:   "tab in token",
			tokens: []string{"set", "value", "a\tb"},
			expect: "set value \"a\tb\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinTokensWithQuotes(tt.tokens)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestEditQuotedListKey verifies edit command works with quoted string-keyed list entries.
//
// VALIDATES: Tree navigation handles string-keyed lists (e.g., template group names).
// PREVENTS: Navigation failure for list entries with spaces in keys.
func TestEditQuotedListKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// template.group is a string-keyed list (key "name")
	originalContent := `template {
	group "my group" {
		peer-as 65001;
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit group with quoted name using full path (JUNOS-style)
	editResult, err := model.cmdEdit([]string{"template", "group", "my group"})
	require.NoError(t, err, "edit should find string-keyed list entry")

	// Verify we entered the correct context
	assert.Equal(t, []string{"template", "group", "my group"}, editResult.newContext)

	// Verify config content includes the group block (full tree in Part 1)
	assert.NotNil(t, editResult.configView)
	assert.Contains(t, editResult.configView.content, "peer-as 65001")
}

// TestSetInQuotedListEntry verifies set command works inside string-keyed list entries.
//
// VALIDATES: Full flow: edit string-keyed list entry -> set value -> config updated correctly.
// PREVENTS: Tree mutation failure when setting values in string-keyed blocks.
func TestSetInQuotedListEntry(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// template.group is a string-keyed list (key "name")
	originalContent := `template {
	group "my group" {
		peer-as 65001;
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter the group context using full path (JUNOS-style)
	editResult, err := model.cmdEdit([]string{"template", "group", "my group"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set a value inside the group block
	setResult, err := model.cmdSet([]string{"peer-as", "65002"})
	require.NoError(t, err)

	// Verify the content was modified correctly
	assert.Contains(t, setResult.statusMessage, "Set")
	content := ed.WorkingContent()
	assert.Contains(t, content, "peer-as 65002")
	assert.NotContains(t, content, "peer-as 65001", "old value should be replaced")
	// Verify the group block structure is preserved
	assert.Contains(t, content, `group "my group" {`)
}

// TestCommitTriggersReload verifies commit calls reload notifier after save.
//
// VALIDATES: After save, reload notification is attempted.
// PREVENTS: Config saved but daemon not notified of changes.
func TestCommitTriggersReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	// Track whether notifier was called
	notified := false
	ed.SetReloadNotifier(func() error {
		notified = true
		return nil
	})

	// Mark dirty so save will proceed
	ed.MarkDirty()

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdCommit()
	require.NoError(t, err)

	assert.True(t, notified, "reload notifier should have been called")
	assert.Contains(t, result.statusMessage, "committed")
	assert.Contains(t, result.statusMessage, "reloaded")
}

// TestCommitReloadFailsGracefully verifies commit succeeds even when reload fails.
//
// VALIDATES: Daemon not running → commit succeeds with warning.
// PREVENTS: Save lost because daemon notification failed.
func TestCommitReloadFailsGracefully(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	// Notifier that simulates daemon not running
	ed.SetReloadNotifier(func() error {
		return fmt.Errorf("connection refused")
	})

	ed.MarkDirty()

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdCommit()
	require.NoError(t, err, "commit should succeed even when reload fails")

	// Save should still succeed
	assert.False(t, ed.Dirty(), "editor should no longer be dirty")
	// Status should indicate reload failure
	assert.Contains(t, result.statusMessage, "committed")
	assert.Contains(t, result.statusMessage, "reload")
}

// TestCommitValidationFailsNoReload verifies no reload when validation fails.
//
// VALIDATES: YANG validation fails → no save, no reload notification.
// PREVENTS: Invalid config pushed to running daemon.
func TestCommitValidationFailsNoReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write config with errors
	content := `bgp {
  router-id invalid;
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	// Track whether notifier was called
	notified := false
	ed.SetReloadNotifier(func() error {
		notified = true
		return nil
	})

	ed.MarkDirty()

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.cmdCommit()
	require.Error(t, err, "commit should fail with validation errors")
	assert.Contains(t, err.Error(), "validation error")
	assert.False(t, notified, "reload notifier should NOT be called on validation failure")
}

// TestCommitNoNotifierStandalone verifies commit works without notifier (standalone mode).
//
// VALIDATES: Editor works in standalone mode (no daemon).
// PREVENTS: Nil pointer panic when no notifier is set.
func TestCommitNoNotifierStandalone(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	// No notifier set (standalone mode)
	ed.MarkDirty()

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdCommit()
	require.NoError(t, err, "commit should succeed without notifier")
	assert.Contains(t, result.statusMessage, "committed")
	assert.Contains(t, result.statusMessage, "daemon not running", "standalone mode should inform daemon is not running")
	assert.NotContains(t, result.statusMessage, "reloaded", "standalone mode should not claim reloaded")
}

// TestSetThroughList verifies set with full path through a list from root context.
//
// VALIDATES: spec-editor-2 AC-1: "set bgp peer 1.1.1.1 hold-time 90" from root.
// PREVENTS: Positional path splitting breaking list paths.
func TestSetThroughList(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Set hold-time through list from root — no edit context
	result, err := model.dispatchCommand("set bgp peer 1.1.1.1 hold-time 120")
	require.NoError(t, err, "set through list should succeed")
	assert.Contains(t, result.statusMessage, "Set")

	content := ed.WorkingContent()
	assert.Contains(t, content, "120", "hold-time should be updated to 120")
}

// TestSetRejectsNonLeafPath verifies set rejects paths that don't resolve to a leaf.
//
// VALIDATES: spec-editor-2 AC-4: "set bgp nonexistent value" → error.
// PREVENTS: ValidateValueAtPath silently passing non-leaf paths.
func TestSetRejectsNonLeafPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// "bgp" is a container, not a leaf — set should reject
	_, err = model.dispatchCommand("set bgp nonexistent value")
	require.Error(t, err, "set on unknown path should fail")
}

// TestSetInContextPreserved verifies set still works within an edit context.
//
// VALIDATES: spec-editor-2 AC-5: existing context-relative set still works.
// PREVENTS: Regressions in existing set behavior.
func TestSetInContextPreserved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "1.1.1.1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set within context — should still work
	result, err := model.dispatchCommand("set hold-time 120")
	require.NoError(t, err, "context-relative set should still work")
	assert.Contains(t, result.statusMessage, "Set")

	content := ed.WorkingContent()
	assert.Contains(t, content, "120", "hold-time should be updated to 120")
}

// TestSetThroughListDescription verifies set of a string value through a list from root.
//
// VALIDATES: spec-editor-2 AC-6: set description through list correctly stores value.
// PREVENTS: String values through list paths being mishandled.
func TestSetThroughListDescription(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.dispatchCommand(`set bgp peer 1.1.1.1 description "my peer"`)
	require.NoError(t, err, "set description through list should succeed")
	assert.Contains(t, result.statusMessage, "Set")

	content := ed.WorkingContent()
	assert.Contains(t, content, "my peer", "description should contain 'my peer'")
}

// TestSetRejectsConfigFalse verifies set rejects paths through config false containers.
//
// VALIDATES: spec-editor-2 AC-2: "set bgp rib ..." → error (config false).
// PREVENTS: Writing to read-only state (rib is config false in YANG).
func TestSetRejectsConfigFalse(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.dispatchCommand("set bgp rib adj-rib-in peer * route-count 5")
	require.Error(t, err, "set on config false path should fail")
	assert.Contains(t, err.Error(), "read-only")
}

// TestSetRejectsMissingListKey verifies set rejects a list path without a key.
//
// VALIDATES: spec-editor-2 AC-3: "set bgp peer hold-time 90" (missing key) → error.
// PREVENTS: Ambiguous set when list key is missing.
func TestSetRejectsMissingListKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// "set bgp peer hold-time 90" — peer is a list, "hold-time" is not a valid key value,
	// but more importantly "90" should not land in a random place.
	_, err = model.dispatchCommand("set bgp peer hold-time 90")
	require.Error(t, err, "set on list without key should fail")
}

// TestSetRejectsUnknownPath verifies set rejects a path with unknown elements.
//
// VALIDATES: spec-editor-2 AC-4: unknown path element → error.
// PREVENTS: Creating garbage tree entries for non-existent config paths.
func TestSetRejectsUnknownPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.dispatchCommand("set bgp totally-unknown-leaf value")
	require.Error(t, err, "set with unknown path should fail")
	assert.Contains(t, err.Error(), "unknown path")
}

// TestSocketReloadNotifierNoDaemon verifies socket notifier fails gracefully.
//
// VALIDATES: NewSocketReloadNotifier returns error when daemon socket doesn't exist.
// PREVENTS: Panic or hang when daemon is not running.
func TestSocketReloadNotifierNoDaemon(t *testing.T) {
	// Use a non-existent socket path
	notifier := NewSocketReloadNotifier("/tmp/ze-test-nonexistent-" + t.Name() + ".sock")
	err := notifier()
	require.Error(t, err, "should fail when daemon socket doesn't exist")
	assert.Contains(t, err.Error(), "daemon not reachable")
}
