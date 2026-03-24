package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
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
  router-id invalid
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Get errors
	result, err := model.cmdErrors(nil)
	require.NoError(t, err)

	// Should have error content (parser error for invalid router-id)
	assert.Contains(t, result.output, "issue(s)")
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
	result, err := model.cmdErrors(nil)
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
	result, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)

	// Should build hierarchical path
	assert.Equal(t, []string{"bgp", "peer", "peer1"}, result.newContext, "should have hierarchical path")

	// Should show config content (full serialized tree in Part 1)
	assert.NotNil(t, result.configView, "should have config view")
	assert.Contains(t, result.configView.content, "remote", "should contain peer block content")
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
  router-id 1.2.3.4
  peer peer1 {
    remote { ip 1.1.1.1; as 65001; }
    capability {
      route-refresh
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
	result, err := model.cmdEdit([]string{"peer", "peer1"})
	require.NoError(t, err)

	// Should have full hierarchical path
	assert.Equal(t, []string{"bgp", "peer", "peer1"}, result.newContext)
}

// TestModelCmdEditExactMatch verifies edit uses exact block matching.
//
// VALIDATES: Edit doesn't match prefix (e.g., "peer" shouldn't match "remote").
// PREVENTS: Wrong block selected due to prefix matching.
func TestModelCmdEditExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Config with multiple peer blocks
	content := `bgp {
  peer transit1 {
    remote { ip 2.2.2.2; as 65001; }
  }
  peer transit2 {
    remote { ip 1.1.1.1; as 65002; }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed)
	require.NoError(t, err)

	// Edit "peer transit2" using full path (JUNOS-style)
	result, err := model.cmdEdit([]string{"bgp", "peer", "transit2"})
	require.NoError(t, err)

	// Should find the correct peer block
	assert.Equal(t, []string{"bgp", "peer", "transit2"}, result.newContext)
	assert.Contains(t, result.configView.content, "65002", "should contain peer transit2 content")
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

	// Call up - should go to bgp level (skipping invalid "peer" context)
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
  router-id 1.2.3.4
  local { as 65000; }
  peer peer1 {
    remote { ip 1.1.1.1; as 65001; }
  }
  peer peer2 {
    remote { ip 2.2.2.2; as 65002; }
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
	result, err := model.cmdShowPipe(nil, []PipeFilter{{Type: "match", Arg: "peer1"}})
	require.NoError(t, err)

	// Should contain matched content
	assert.Contains(t, result.output, "peer1", "should contain matched peer")

	// Should not contain unmatched content
	assert.NotContains(t, result.output, "peer2", "should not contain other peer")
}

// TestModelPipeShowHead verifies "show | head N" limits output.
//
// VALIDATES: Pipe with head limits to N lines.
// PREVENTS: Head not limiting or wrong count.
func TestModelPipeShowHead(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4
  local { as 65000; }
  peer peer1 {
    remote { ip 1.1.1.1; as 65001; }
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
  peer a1 { remote { ip 1.1.1.1; as 65001; } }
  peer a2 { remote { ip 1.1.1.2; as 65002; } }
  peer a3 { remote { ip 1.1.1.3; as 65003; } }
  peer b1 { remote { ip 2.2.2.1; as 65004; } }
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
		{Type: "match", Arg: "1.1.1"},
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
	router-id 1.2.3.4
	peer peer1 {
		remote { ip 1.1.1.1; as 65001; }
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
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set description
	result, err := model.dispatchCommand(`set description "test peer"`)
	require.NoError(t, err)

	// Verify content was modified
	content := ed.WorkingContent()
	assert.Contains(t, content, `description "test peer"`, "description should be added")
	assert.True(t, ed.Dirty(), "should be marked dirty")
	assert.Contains(t, result.statusMessage, "set", "status should mention set")
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
	router-id 1.2.3.4
	peer peer1 {
		remote { ip 1.1.1.1; as 65001; }
		description "old value"
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
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
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
// VALIDATES: "set timer receive-hold-time abc" returns error for non-numeric value.
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
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set receive-hold-time to invalid string — should fail
	_, err = model.dispatchCommand("set timer receive-hold-time abc")
	require.Error(t, err, "should reject non-numeric receive-hold-time")
	assert.Contains(t, err.Error(), "invalid value")

	// Set receive-hold-time to valid value — should succeed
	_, err = model.dispatchCommand("set timer receive-hold-time 180")
	require.NoError(t, err, "should accept valid numeric receive-hold-time")
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
// VALIDATES: Tree navigation handles string-keyed lists (e.g., bgp group names).
// PREVENTS: Navigation failure for list entries with spaces in keys.
func TestEditQuotedListKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// bgp.group is a string-keyed list (key "name")
	originalContent := `bgp {
	group "my group" {
		remote { as 65001; }
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
	editResult, err := model.cmdEdit([]string{"bgp", "group", "my group"})
	require.NoError(t, err, "edit should find string-keyed list entry")

	// Verify we entered the correct context
	assert.Equal(t, []string{"bgp", "group", "my group"}, editResult.newContext)

	// Verify config content includes the group block (full tree in Part 1)
	assert.NotNil(t, editResult.configView)
	assert.Contains(t, editResult.configView.content, "65001")
}

// TestSetInQuotedListEntry verifies set command works inside string-keyed list entries.
//
// VALIDATES: Full flow: edit string-keyed list entry -> set value -> config updated correctly.
// PREVENTS: Tree mutation failure when setting values in string-keyed blocks.
func TestSetInQuotedListEntry(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// bgp.group is a string-keyed list (key "name")
	originalContent := `bgp {
	group "my group" {
		remote { as 65001; }
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
	editResult, err := model.cmdEdit([]string{"bgp", "group", "my group"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set a value inside the group block
	setResult, err := model.cmdSet([]string{"remote", "as", "65002"})
	require.NoError(t, err)

	// Verify the content was modified correctly
	assert.Contains(t, setResult.statusMessage, "set")
	content := ed.WorkingContent()
	assert.Contains(t, content, "65002")
	assert.NotContains(t, content, "65001", "old value should be replaced")
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
  router-id invalid
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

	result, err := model.cmdCommit()
	require.NoError(t, err, "commit returns nil error, issues in output")
	assert.Contains(t, result.statusMessage, "blocked")
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
// VALIDATES: spec-editor-2 AC-1: "set bgp peer 1.1.1.1 timer receive-hold-time 90" from root.
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

	// Set receive-hold-time through list from root — no edit context
	result, err := model.dispatchCommand("set bgp peer peer1 timer receive-hold-time 120")
	require.NoError(t, err, "set through list should succeed")
	assert.Contains(t, result.statusMessage, "set")

	content := ed.WorkingContent()
	assert.Contains(t, content, "120", "receive-hold-time should be updated to 120")
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
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.ApplyResult(editResult)

	// Set within context — should still work
	result, err := model.dispatchCommand("set timer receive-hold-time 120")
	require.NoError(t, err, "context-relative set should still work")
	assert.Contains(t, result.statusMessage, "set")

	content := ed.WorkingContent()
	assert.Contains(t, content, "120", "receive-hold-time should be updated to 120")
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

	result, err := model.dispatchCommand(`set bgp peer peer1 description "my peer"`)
	require.NoError(t, err, "set description through list should succeed")
	assert.Contains(t, result.statusMessage, "set")

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
// VALIDATES: spec-editor-2 AC-3: "set bgp peer timer receive-hold-time 90" (missing key) → error.
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

	// "set bgp peer timer receive-hold-time 90" — peer is a list, "timer" is not a valid key value,
	// but more importantly "90" should not land in a random place.
	_, err = model.dispatchCommand("set bgp peer timer receive-hold-time 90")
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

// TestWhoWithSession verifies "who" command works with active session.
//
// VALIDATES: who lists active sessions when session is active.
// PREVENTS: Session guard false-positive blocking who when session exists.
func TestWhoWithSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	ed.SetSession(NewEditSession("testuser", "local"))

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.dispatchCommand("who")
	require.NoError(t, err, "who should succeed with active session")
	// No pending changes, so output says "No active sessions." (no changes tracked yet)
	assert.NotEmpty(t, result.output)
}

// TestFilterOutSessionCommandsEmpty verifies filter handles empty input.
//
// VALIDATES: filterOutSessionCommands does not panic on empty slice.
// PREVENTS: Index out of bounds on empty completion list.
func TestFilterOutSessionCommandsEmpty(t *testing.T) {
	filtered := filterOutSessionCommands(nil)
	assert.Empty(t, filtered)

	filtered = filterOutSessionCommands([]Completion{})
	assert.Empty(t, filtered)
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

// --- Phase 5: Display Views, Session Management, and Commands ---

// TestFormatChangeEntryNew verifies change entry formatting for new values.
//
// VALIDATES: New entries use '+' marker and "(new)" annotation.
// PREVENTS: Wrong marker or missing annotation for new entries.
func TestFormatChangeEntryNew(t *testing.T) {
	var b strings.Builder
	formatChangeEntry(&b, config.SessionEntry{
		Path:  "bgp router-id",
		Entry: config.MetaEntry{Value: "1.2.3.4"},
	})
	line := b.String()
	assert.Contains(t, line, "  + set bgp router-id 1.2.3.4")
	assert.Contains(t, line, "(new)")
}

// TestFormatChangeEntryModified verifies change entry formatting for modified values.
//
// VALIDATES: Modified entries use '*' marker and "(was: ...)" annotation.
// PREVENTS: Wrong marker or missing previous value for modified entries.
func TestFormatChangeEntryModified(t *testing.T) {
	var b strings.Builder
	formatChangeEntry(&b, config.SessionEntry{
		Path:  "bgp remote as",
		Entry: config.MetaEntry{Value: "65002", Previous: "65001"},
	})
	line := b.String()
	assert.Contains(t, line, "  * set bgp remote as 65002")
	assert.Contains(t, line, "(was: 65001)")
}

// TestFormatChangeEntryDelete verifies change entry formatting for deleted values.
//
// VALIDATES: Delete entries use '-' marker, "delete" command, and "(was: ...)" annotation.
// PREVENTS: Delete rendered as set with empty value.
func TestFormatChangeEntryDelete(t *testing.T) {
	var b strings.Builder
	formatChangeEntry(&b, config.SessionEntry{
		Path:  "bgp timer receive-hold-time",
		Entry: config.MetaEntry{Value: "", Previous: "180"},
	})
	line := b.String()
	assert.Contains(t, line, "  - delete bgp timer receive-hold-time")
	assert.Contains(t, line, "(was: 180)")
	assert.NotContains(t, line, "set")
}

// TestFilterOutSessionCommands verifies session-dependent command filtering.
//
// VALIDATES: who, disconnect, blame, changes are removed; other commands preserved.
// PREVENTS: Non-session commands accidentally filtered or session commands leaking.
func TestFilterOutSessionCommands(t *testing.T) {
	input := []Completion{
		{Text: cmdSet, Type: "command"},
		{Text: cmdBlame, Type: "keyword"},
		{Text: cmdChanges, Type: "keyword"},
		{Text: cmdWho, Type: "command"},
		{Text: cmdDisconnect, Type: "command"},
		{Text: cmdExit, Type: "command"},
		{Text: cmdShow, Type: "command"},
	}
	result := filterOutSessionCommands(input)

	texts := make([]string, len(result))
	for i, c := range result {
		texts[i] = c.Text
	}
	assert.Contains(t, texts, cmdSet)
	assert.Contains(t, texts, cmdExit)
	assert.Contains(t, texts, cmdShow)
	assert.NotContains(t, texts, cmdBlame)
	assert.NotContains(t, texts, cmdChanges)
	assert.NotContains(t, texts, cmdWho)
	assert.NotContains(t, texts, cmdDisconnect)
}

// TestCmdOptionBlameRequiresSession verifies option blame errors without session.
//
// VALIDATES: "option blame" returns error when no editing session is active.
// PREVENTS: Nil pointer or empty output when blame called without session.
func TestCmdOptionBlameRequiresSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// No session set -- option blame should error
	_, err = model.cmdOption([]string{cmdBlame})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an active editing session")
}

// TestCmdOptionChangesRequiresSession verifies option changes errors without session.
//
// VALIDATES: "option changes" returns error when no editing session is active.
// PREVENTS: Empty or misleading output when changes called without session.
func TestCmdOptionChangesRequiresSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.cmdOption([]string{cmdChanges})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an active editing session")
}

// TestCmdShowFormatConfigWithoutSession verifies show | format config works without session.
//
// VALIDATES: set-format display is available without an editing session via format pipe.
// PREVENTS: format config incorrectly gated behind session check.
func TestCmdShowFormatConfigWithoutSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	// No session -- show | format config should work
	result, err := model.cmdShowDisplay(fmtConfig, "")
	require.NoError(t, err)
	assert.Contains(t, result.output, "set ")
}

// TestCmdWhoRequiresSession verifies who command errors without session.
//
// VALIDATES: "who" returns error when no editing session is active.
// PREVENTS: Confusing output when who called outside session context.
func TestCmdWhoRequiresSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.dispatchCommand("who")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an active editing session")
}

// TestCmdDisconnectRequiresSession verifies disconnect errors without session.
//
// VALIDATES: "disconnect" returns error when no editing session is active.
// PREVENTS: Disconnect operating on global state without session context.
func TestCmdDisconnectRequiresSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.dispatchCommand("disconnect some-session")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an active editing session")
}

// TestCmdDisconnectOwnSession verifies disconnect rejects own session.
//
// VALIDATES: Cannot disconnect own session (must use 'discard all' instead).
// PREVENTS: User accidentally disconnecting themselves.
func TestCmdDisconnectOwnSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.cmdDisconnectSession([]string{session.ID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot disconnect own session")
}

// TestCmdDisconnectNoArgs verifies disconnect errors without session ID argument.
//
// VALIDATES: "disconnect" without args returns usage error.
// PREVENTS: Ambiguous disconnect without target.
func TestCmdDisconnectNoArgs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.cmdDisconnectSession(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}

// TestCmdSaveSessionMode verifies save in session mode calls SaveDraft.
//
// VALIDATES: "save" in session mode applies change file to draft (AC-24).
// PREVENTS: Redundant .edit snapshot when write-through already persists change file.
func TestCmdSaveSessionMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	// Make a change so SaveDraft has something to save.
	err = ed.SetValue([]string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdSave()
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Changes saved to draft",
		"save in session mode should call SaveDraft and report success")

	// Draft file should now exist (SaveDraft created it).
	draftPath := DraftPath(configPath)
	_, statErr := os.Stat(draftPath)
	assert.False(t, os.IsNotExist(statErr), "draft should exist after save in session mode")

	// Change file should be gone (SaveDraft consumed it).
	changePath := ChangePath(configPath, "alice")
	_, statErr = os.Stat(changePath)
	assert.True(t, os.IsNotExist(statErr), "change file should be deleted after save")
}

// TestCmdWhoOutputFormat verifies who command output format.
//
// VALIDATES: Who output includes current session marker, change counts, pluralization.
// PREVENTS: Malformed session listing with wrong markers or grammar.
func TestCmdWhoOutputFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	// Make a change so there's something to report
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdWho()
	require.NoError(t, err)

	assert.Contains(t, result.output, "Active editing sessions:")
	assert.Contains(t, result.output, "* "+session.ID, "current session should be marked with *")
	assert.Contains(t, result.output, "1 pending change\n", "singular 'change' for count of 1")
}

// TestCmdShowChangesNoChanges verifies show changes with empty session.
//
// VALIDATES: "show changes" with no pending changes returns informative message.
// PREVENTS: Empty or confusing output when no changes exist.
func TestCmdShowChangesNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdShowChanges(nil)
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "No pending changes")
}

// TestCmdShowChangesAllGrouping verifies show changes all groups by session.
//
// VALIDATES: "show changes all" groups changes by session with headers (AC-18).
// PREVENTS: Changes from different sessions mixed without grouping.
func TestCmdShowChangesAllGrouping(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	// Session 1 makes a change
	ed1, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed1.Close() //nolint:errcheck,gosec // test cleanup

	session1 := NewEditSession("alice", "ssh")
	ed1.SetSession(session1)
	err = ed1.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	// Session 2 makes a different change
	ed2, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed2.Close() //nolint:errcheck,gosec // test cleanup

	session2 := NewEditSession("bob", "local")
	ed2.SetSession(session2)
	err = ed2.SetValue([]string{"bgp", "local"}, "as", "65001")
	require.NoError(t, err)

	model, err := NewModel(ed2)
	require.NoError(t, err)

	result, err := model.cmdShowChangesAll()
	require.NoError(t, err)

	// Should have summary with session count
	assert.Contains(t, result.statusMessage, "2 pending changes across 2 sessions")
	// Should have tree content in configView
	assert.NotNil(t, result.configView, "should include tree view")
}

// TestCmdCommitConfirmedRejectedInSession verifies commit confirmed is rejected in session mode.
//
// VALIDATES: "commit confirmed N" in session mode returns explicit error (AC-37).
// PREVENTS: Silent misrouting of commit confirmed through session commit path.
func TestCmdCommitConfirmedRejectedInSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	model, err := NewModel(ed)
	require.NoError(t, err)

	_, err = model.dispatchCommand("commit confirmed 30")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet supported in session mode")
}

// TestHasPendingChangesSessionAware verifies pending changes detection uses session.
//
// VALIDATES: hasPendingChanges() checks session entries when session is active.
// PREVENTS: Exit prompt using dirty flag instead of session entries in session mode.
func TestHasPendingChangesSessionAware(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	model, err := NewModel(ed)
	require.NoError(t, err)

	// No changes yet
	assert.False(t, model.hasPendingChanges(), "no pending changes before set")

	// Make a change through write-through
	err = ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.1")
	require.NoError(t, err)

	assert.True(t, model.hasPendingChanges(), "should detect pending session changes")
}

// TestAutoSaveOnQuitSkipsSession verifies auto-save skips in session mode.
//
// VALIDATES: autoSaveOnQuit() does not write .edit when session is active.
// PREVENTS: Redundant .edit snapshot alongside write-through .draft.
func TestAutoSaveOnQuitSkipsSession(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	model, err := NewModel(ed)
	require.NoError(t, err)

	model.autoSaveOnQuit()

	// .edit file should NOT exist (session mode skips auto-save)
	editPath := configPath + ".edit"
	_, statErr := os.Stat(editPath)
	assert.True(t, os.IsNotExist(statErr), ".edit should not exist in session mode")
}

// TestCmdCommitSessionReload verifies session commit triggers reload notifier.
//
// VALIDATES: cmdCommitSession (model_commands.go:630-635) reload path.
// PREVENTS: Daemon not refreshed after session commit.
func TestCmdCommitSessionReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	notified := false
	ed.SetReloadNotifier(func() error {
		notified = true
		return nil
	})

	session := NewEditSession("alice", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdCommitSession()
	require.NoError(t, err)

	assert.True(t, notified, "reload notifier should be called")
	assert.Contains(t, result.statusMessage, "reloaded", "status should mention reloaded")
}

// TestCmdCommitSessionReloadFails verifies session commit handles reload failure.
//
// VALIDATES: cmdCommitSession (model_commands.go:631-632) reload error path.
// PREVENTS: Session commit failing when daemon is unreachable.
func TestCmdCommitSessionReloadFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	ed.SetReloadNotifier(func() error {
		return fmt.Errorf("connection refused")
	})

	session := NewEditSession("alice", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdCommitSession()
	require.NoError(t, err, "session commit should not fail on reload error")

	assert.Contains(t, result.statusMessage, "reload failed", "status should warn about reload failure")
	assert.Contains(t, result.statusMessage, "change(s) applied", "status should show changes applied")
}

// TestCmdCommitSessionValidatesSetFormat verifies session commit validates set-format content.
//
// VALIDATES: cmdCommitSession (model_commands.go:581) validates WorkingContent in set format.
// PREVENTS: Validator rejecting set-format content from session mode.
func TestCmdCommitSessionValidatesSetFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)

	// WorkingContent should be set+meta format now that session is active.
	content := ed.WorkingContent()
	format := config.DetectFormat(content)
	assert.NotEqual(t, config.FormatHierarchical, format,
		"WorkingContent should return set format when session active")

	// Make a valid change and commit: should succeed (validator handles set format).
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)

	result, err := model.cmdCommitSession()
	require.NoError(t, err)

	assert.Contains(t, result.statusMessage, "change(s) applied",
		"session commit should succeed with set-format validation")
}
