package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Get errors
	result, err := model.cmdErrors(nil)
	require.NoError(t, err)

	assert.Contains(t, result.output, "No issues")
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
      capability {
        route-refresh
      }
    }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
    connection {
      remote {
        ip 2.2.2.2
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
  peer transit2 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65002
      }
    }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
  peer peer2 {
    connection {
      remote {
        ip 2.2.2.2
      }
    }
    session {
      asn {
        remote 65002
      }
    }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
  peer a1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } }
  peer a1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } }
  peer a2 { connection { remote { ip 1.1.1.2; } } session { asn { remote 65002; } } }
  peer a2 { connection { remote { ip 1.1.1.2; } } session { asn { remote 65002; } } }
  peer a3 { connection { remote { ip 1.1.1.3; } } session { asn { remote 65003; } } }
  peer a3 { connection { remote { ip 1.1.1.3; } } session { asn { remote 65003; } } }
  peer b1 { connection { remote { ip 2.2.2.1; } } session { asn { remote 65004; } } }
  peer b1 { connection { remote { ip 2.2.2.1; } } session { asn { remote 65004; } } }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.applyResult(editResult)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizeCommand(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestTokenizeCommandBackslashLiteral verifies backslash has no special meaning.
//
// VALIDATES: Backslash is preserved as a literal character.
// PREVENTS: Backslash being treated as an escape character.
func TestTokenizeCommandBackslashLiteral(t *testing.T) {
	tests := []struct {
		input  string
		expect []string
	}{
		{`set path C:\Users`, []string{"set", "path", `C:\Users`}},
		{`set path "C:\Users\test"`, []string{"set", "path", `C:\Users\test`}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
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
		description "old value"
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.applyResult(editResult)

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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.applyResult(editResult)

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
		session { asn { remote 65001; } }
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
		session { asn { remote 65001; } }
	}
}`
	err := os.WriteFile(configPath, []byte(originalContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter the group context using full path (JUNOS-style)
	editResult, err := model.cmdEdit([]string{"bgp", "group", "my group"})
	require.NoError(t, err)
	model.applyResult(editResult)

	// Set a value inside the group block
	setResult, err := model.cmdSet([]string{"session", "asn", "remote", "65002"})
	require.NoError(t, err)

	// Verify the content was modified correctly
	assert.Contains(t, setResult.statusMessage, "set")
	content := ed.WorkingContent()
	assert.Contains(t, content, "65002")
	assert.NotContains(t, content, "65001", "old value should be replaced")
	// Verify the group block structure is preserved
	assert.Contains(t, content, `group "my group" {`)
}

// TestCommitTriggersReload verifies commit stages a candidate before reload.
//
// VALIDATES: AC-1 stages candidate, reload promotes active, then CLI reports success.
// PREVENTS: CLI commit mutating active storage before daemon verification.
func TestCommitTriggersReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test

	notified := false
	ed.SetReloadNotifier(func() error {
		notified = true
		candidate, _, ok, readErr := storage.ReadCandidateConfig(ed.store, ed.originalPath)
		require.NoError(t, readErr)
		require.True(t, ok, "candidate must be staged before reload notification")
		assert.Contains(t, string(candidate), "router-id 1.2.3.4")

		activeBefore, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, testValidBGPConfig, string(activeBefore), "active file must not change before promotion")

		require.NoError(t, storage.PromoteCandidate(ed.store, ed.originalPath))
		return nil
	})

	// Mark dirty so save will proceed
	ed.MarkDirty()

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCommit()
	require.NoError(t, err)

	assert.True(t, notified, "reload notifier should have been called")
	assert.Contains(t, result.statusMessage, "committed")
	assert.Contains(t, result.statusMessage, "reloaded")
	assert.False(t, ed.Dirty(), "editor should be clean after successful transactional commit")
}

// TestCommitReloadFailsGracefully verifies reload failure rejects the commit.
//
// VALIDATES: AC-2 reload failure leaves active unchanged and reports commit failure.
// PREVENTS: CLI commit reporting success when daemon rejected the candidate.
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCommit()
	require.NoError(t, err, "commit failure is reported as command status")

	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, testValidBGPConfig, string(data), "active file should remain unchanged")
	assert.True(t, ed.Dirty(), "editor should remain dirty after rejected commit")
	assert.Contains(t, result.statusMessage, "commit failed")
	assert.Contains(t, result.statusMessage, "connection refused")
	_, ok, pointerErr := storage.ReadPointer(ed.store, ed.originalPath, storage.PointerCandidate)
	require.NoError(t, pointerErr)
	assert.False(t, ok, "failed commit should clear local candidate pointer")
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCommit()
	require.NoError(t, err, "commit should succeed without notifier")
	assert.Contains(t, result.statusMessage, "committed")
	assert.Contains(t, result.statusMessage, "daemon not running", "standalone mode should inform daemon is not running")
	assert.NotContains(t, result.statusMessage, "reloaded", "standalone mode should not claim reloaded")
}

// VALIDATES: AC-9 -- Config commit via SSH CLI emits an audit record with actor, surface, action, and summary.
// PREVENTS: Interactive CLI config commits bypassing the unified audit trail.
func TestCLIConfigCommitAuditRecord(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test
	ed.setWorkingContent(strings.Replace(testValidBGPConfig, "router-id 1.2.3.4", "router-id 5.6.7.8", 1))

	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	model.SetAuditRecorder(recorder, audit.SSH, "alice", "192.0.2.10:2222")

	_, err = model.cmdCommit()
	require.NoError(t, err)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:2222", entries[0].RemoteAddr)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "5.6.7.8")
}

// VALIDATES: AC-10 -- Config discard via SSH CLI emits an audit record with actor, surface, action, and summary.
// PREVENTS: Interactive CLI config discards losing audit attribution.
func TestCLIConfigDiscardAuditRecord(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test
	ed.setWorkingContent(strings.Replace(testValidBGPConfig, "router-id 1.2.3.4", "router-id 5.6.7.8", 1))

	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	model.SetAuditRecorder(recorder, audit.SSH, "alice", "192.0.2.10:2222")

	_, err = model.cmdDiscard()
	require.NoError(t, err)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:2222", entries[0].RemoteAddr)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "5.6.7.8")
}

// VALIDATES: AC-10 -- Config rollback via SSH CLI emits a discard audit record with actor and backup path.
// PREVENTS: Interactive CLI rollback bypassing the unified audit trail.
func TestCLIConfigRollbackAuditRecord(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup in test
	require.NoError(t, ed.createBackup(ed.OriginalContent(), nil))

	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	model.SetAuditRecorder(recorder, audit.SSH, "alice", "192.0.2.10:2222")

	_, err = model.cmdRollback([]string{"1"})
	require.NoError(t, err)

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, "192.0.2.10:2222", entries[0].RemoteAddr)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
	assert.Contains(t, entries[0].Detail, "rollback ")
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter peer context
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer1"})
	require.NoError(t, err)
	model.applyResult(editResult)

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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.dispatchCommand("set bgp rib adj-rib-in peer * route-count 5")
	require.Error(t, err, "set on config false path should fail")
	assert.Contains(t, err.Error(), "read-only")
}

// TestSetListKeyKeywordThenChild verifies that a list entry created through the
// key-leaf keyword can immediately accept child leaf updates.
//
// VALIDATES: "set ... next hop address <ip>" followed by "set ... next hop <ip> weight 2".
// PREVENTS: keyed entry creation from dropping the next status update or child mutation.
func TestSetListKeyKeywordThenChild(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	content := `static {
  table default {
    route 0.0.0.0/0 {
      next { }
    }
  }
}`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.dispatchCommand("set static table default route 0.0.0.0/0 next hop address 10.104.1.254")
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "created hop 10.104.1.254")

	result, err = model.dispatchCommand("set static table default route 0.0.0.0/0 next hop 10.104.1.254 weight 2")
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "set")
	assert.Contains(t, ed.WorkingContent(), "weight 2")
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
	notifier := newSocketReloadNotifier("/tmp/ze-test-nonexistent-" + t.Name() + ".sock")
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
	var b textbuf.Buffer
	formatChangeEntry(&b, config.PendingChange{
		Kind:  config.PendingChangeSet,
		Path:  "bgp router-id",
		Value: "1.2.3.4",
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
	var b textbuf.Buffer
	formatChangeEntry(&b, config.PendingChange{
		Kind:     config.PendingChangeSet,
		Path:     "bgp remote as",
		Value:    "65002",
		Previous: "65001",
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
	var b textbuf.Buffer
	formatChangeEntry(&b, config.PendingChange{
		Kind:     config.PendingChangeDelete,
		Path:     "bgp timer receive-hold-time",
		Previous: "180",
	})
	line := b.String()
	assert.Contains(t, line, "  - delete bgp timer receive-hold-time")
	assert.Contains(t, line, "(was: 180)")
	assert.NotContains(t, line, "set")
}

// TestFilterOutSessionCommands verifies session-dependent command filtering.
//
// VALIDATES: who, disconnect are removed; other commands preserved.
// PREVENTS: Non-session commands accidentally filtered or session commands leaking.
func TestFilterOutSessionCommands(t *testing.T) {
	input := []Completion{
		{Text: cmdSet, Type: "command"},
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
	assert.NotContains(t, texts, cmdWho)
	assert.NotContains(t, texts, cmdDisconnect)
}

// TestCmdOptionBlameRedirectsToPipe verifies option blame redirects to pipe syntax.
//
// VALIDATES: "option blame" tells user to use "show | blame".
// PREVENTS: Stale muscle memory from old syntax silently failing.
func TestCmdOptionBlameRedirectsToPipe(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.cmdOption([]string{cmdBlame})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "show | blame")
}

// TestCmdOptionChangesReportsColumnState verifies bare "option changes" reports the
// diff-gutter column state. The pending-changes *view* moved to "show | changes";
// "changes" stays a display column toggled via "option changes enable|disable",
// consistent with author/date/source. Only "blame" (not a column) redirects to a pipe.
//
// VALIDATES: "option changes" is a column-state query, not an error/redirect.
// PREVENTS: Regression conflating the changes column with the show | changes view.
func TestCmdOptionChangesReportsColumnState(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdOption([]string{cmdChanges})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, cmdChanges, "bare 'option changes' reports column state")
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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
	err = ed2.SetValue([]string{"bgp", "session", "asn"}, "local", "65001")
	require.NoError(t, err)

	model, err := NewModel(ed2, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	model.autoSaveOnQuit()

	// .edit file should NOT exist (session mode skips auto-save)
	editPath := configPath + ".edit"
	_, statErr := os.Stat(editPath)
	assert.True(t, os.IsNotExist(statErr), ".edit should not exist in session mode")
}

// TestCmdCommitSessionReload verifies session commit stages a candidate before reload.
//
// VALIDATES: AC-1 stages session commit candidate, reload promotes it, then CLI reports success.
// PREVENTS: Session commit mutating active storage before daemon verification.
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
		candidate, _, ok, readErr := storage.ReadCandidateConfig(ed.store, ed.originalPath)
		require.NoError(t, readErr)
		require.True(t, ok, "session candidate must be staged before reload notification")
		assert.Contains(t, string(candidate), "router-id 9.9.9.9")

		activeBefore, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, testValidBGPConfigSimplePeer, string(activeBefore), "active file must not change before promotion")

		require.NoError(t, storage.PromoteCandidate(ed.store, ed.originalPath))
		return nil
	})

	session := NewEditSession("alice", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCommitSession()
	require.NoError(t, err)

	assert.True(t, notified, "reload notifier should be called")
	assert.Contains(t, result.statusMessage, "reloaded", "status should mention reloaded")
	assert.False(t, ed.Dirty(), "editor should be clean after successful transactional session commit")
}

// TestCmdCommitSessionDeleteContainerClearsDirty verifies that a session commit
// of a container deletion leaves the editor clean.
//
// VALIDATES: delete container + commit clears dirty state in session mode.
// PREVENTS: committed container deletions lingering as unsaved changes.
func TestCmdCommitSessionDeleteContainerClearsDirty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	content := `bgp {
  session {
    asn { local 65000; }
  }
  router-id 1.2.3.4;
  peer peer1 {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65001; } }
    timer { receive-hold-time 90; }
  }
}`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	ed.SetSession(NewEditSession("thomas", "local"))
	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.dispatchCommand("delete bgp peer peer1 timer")
	require.NoError(t, err)
	changeData, readErr := ed.store.ReadFile(ChangePath(configPath, "thomas"))
	require.NoError(t, readErr)
	assert.Contains(t, string(changeData), "delete-container bgp peer peer1 timer")

	result, err := model.cmdCommitSession()
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "change(s) applied")
	assert.False(t, ed.Dirty(), "status=%q working=%q original=%q", result.statusMessage, ed.WorkingContent(), ed.OriginalContent())
}

// TestCmdCommitSessionReloadFails verifies session commit rejects reload failure.
//
// VALIDATES: AC-2 leaves active unchanged and session edits pending when daemon rejects candidate.
// PREVENTS: Session commit losing edits or reporting success after reload failure.
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCommitSession()
	require.NoError(t, err, "session commit failure is reported as command status")

	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, testValidBGPConfigSimplePeer, string(data), "active file should remain unchanged")
	assert.True(t, ed.Dirty(), "editor should remain dirty after rejected session commit")
	assert.Contains(t, result.statusMessage, "commit failed")
	assert.Contains(t, result.statusMessage, "connection refused")
	_, ok, pointerErr := storage.ReadPointer(ed.store, ed.originalPath, storage.PointerCandidate)
	require.NoError(t, pointerErr)
	assert.False(t, ok, "failed session commit should clear local candidate pointer")
	assert.NotEmpty(t, ed.PendingChanges(session.ID), "session changes should remain pending after rejection")
}

// TestCmdCommitSessionRejectsExistingCandidate verifies session commits do not overwrite an in-flight candidate.
//
// VALIDATES: AC-13 rejects concurrent transactional commits from the session path.
// PREVENTS: session commits bypassing candidate existence checks and replacing another caller's candidate.
func TestCmdCommitSessionRejectsExistingCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup
	ed.SetReloadNotifier(func() error { return nil })

	_, err = storage.WriteCandidateVersion(ed.store, ed.originalPath, []byte("in-flight"), time.Date(2026, 5, 24, 10, 0, 0, 0, time.Local))
	require.NoError(t, err)

	session := NewEditSession("alice", "local")
	ed.SetSession(session)
	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.cmdCommitSession()
	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrCandidateExists))

	data, _, ok, readErr := storage.ReadCandidateConfig(ed.store, ed.originalPath)
	require.NoError(t, readErr)
	require.True(t, ok)
	assert.Equal(t, "in-flight", string(data))
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

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCommitSession()
	require.NoError(t, err)

	assert.Contains(t, result.statusMessage, "change(s) applied",
		"session commit should succeed with set-format validation")
}

// TestCmdCommitSessionValidatesPeerProcessConfig verifies session validation
// accepts a complete peer config with process binding.
//
// VALIDATES: AC-2 direct SSH CLI reject tests reach daemon transaction verify,
// not local editor validation.
// PREVENTS: Session-mode set/meta serialization dropping peer fields and
// blocking commits before transactional reload can reject a candidate.
func TestCmdCommitSessionValidatesPeerProcessConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	content := `plugin {
    external cli-reject-plugin {
        run ./cli-reject-plugin.run
        encoder json
    }
}

system {
    authentication {
        user admin {
            password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
        }
    }
}

environment {
    ssh {
        enabled true
        server main {
            ip 127.0.0.1;
            port 2200;
        }
    }
}

bgp {
    router-id 1.2.3.4
    session {
        asn {
            local 1
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
                accept false
            }
        }
        session {
            asn {
                remote 1
            }
            router-id 1.2.3.4
            family {
                ipv4/unicast { prefix { maximum 10000; } }
            }
        }
        behavior {
            group-updates disable
        }
        attach process cli-reject-plugin { }
    }
}`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	session := NewEditSession("alice", "local")
	ed.SetSession(session)
	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "2.2.2.2"))

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result := model.validator.ValidateTransition(ed.OriginalContent(), ed.WorkingContent())
	assert.Empty(t, result.Errors, "expected no validation errors for serialized working content: %s", ed.WorkingContent())
	assert.Empty(t, result.Warnings, "expected no validation warnings for serialized working content: %s", ed.WorkingContent())
}

// TestRenameListEntry verifies the rename command renames a peer.
//
// VALIDATES: rename <list> <old-key> to <new-key> changes the list key.
// PREVENTS: Peer rename losing configuration data.
func TestRenameListEntry(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Rename peer1 to peer2
	result, err := model.cmdRename([]string{"bgp", "peer", "peer1", "to", "peer2"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Renamed peer peer1 to peer2")

	// Verify content: old name gone, new name present with same config
	content := ed.WorkingContent()
	assert.NotContains(t, content, "peer peer1")
	assert.Contains(t, content, "peer peer2")
	assert.Contains(t, content, "1.1.1.1") // IP preserved
	assert.Contains(t, content, "65001")   // ASN preserved
	assert.True(t, ed.Dirty())
}

// TestRenameListEntryWithContext verifies rename works relative to context.
//
// VALIDATES: Rename uses context path for relative navigation.
// PREVENTS: Context-relative rename breaking path resolution.
func TestRenameListEntryWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter bgp context
	editResult, err := model.cmdEdit([]string{"bgp"})
	require.NoError(t, err)
	model.applyResult(editResult)

	// Rename relative to context
	result, err := model.cmdRename([]string{"peer", "peer1", "to", "london"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Renamed peer peer1 to london")

	content := ed.WorkingContent()
	assert.NotContains(t, content, "peer peer1")
	assert.Contains(t, content, "peer london")
}

// TestRenameListEntryNotFound verifies rename fails for missing key.
//
// VALIDATES: Rename returns error for non-existent source key.
// PREVENTS: Silent no-op on missing entry.
func TestRenameListEntryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.cmdRename([]string{"bgp", "peer", "nonexistent", "to", "newname"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRenameListEntryTargetExists verifies rename fails when target already exists.
//
// VALIDATES: Rename rejects duplicate target key.
// PREVENTS: Overwriting existing entry on rename.
func TestRenameListEntryTargetExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4
  session { asn { local 65000; } }
  peer alpha {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65001; } }
  }
  peer beta {
    connection { remote { ip 2.2.2.2; } }
    session { asn { remote 65002; } }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.cmdRename([]string{"bgp", "peer", "alpha", "to", "beta"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestRenameListEntryBadSyntax verifies rename rejects bad syntax.
//
// VALIDATES: Missing "to" keyword produces usage error.
// PREVENTS: Ambiguous rename commands silently misinterpreted.
func TestRenameListEntryBadSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	tests := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"missing to", []string{"bgp", "peer", "peer1", "peer2"}},
		{"too few before to", []string{"peer1", "to", "peer2"}},
		{"extra after new", []string{"bgp", "peer", "peer1", "to", "peer2", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := model.cmdRename(tt.args)
			require.Error(t, err)
		})
	}
}

// TestRenameViaDispatch verifies rename works through the command dispatcher.
//
// VALIDATES: "rename peer peer1 to peer2" dispatches correctly.
// PREVENTS: Rename command not registered in dispatch table.
func TestRenameViaDispatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.dispatchCommand("rename bgp peer peer1 to renamed-peer")
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Renamed peer peer1 to renamed-peer")
	assert.Contains(t, ed.WorkingContent(), "peer renamed-peer")
}

// TestRenameQuotedListKey verifies rename handles quoted list keys.
// Uses names containing dots so quoting is meaningful (dots could be
// confused with path separators without quotes).
//
// VALIDATES: Rename works when valid list entry names are quoted.
// PREVENTS: Quoted key parsing breaking rename path resolution.
func TestRenameQuotedListKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4
  session { asn { local 65000; } }
  peer "peer.old" {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65001; } }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Dispatch handles tokenization including quotes
	result, err := model.dispatchCommand(`rename bgp peer "peer.old" to "peer.new"`)
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Renamed peer peer.old to peer.new")

	output := ed.WorkingContent()
	assert.NotContains(t, output, `peer peer.old`)
	assert.Contains(t, output, `peer peer.new`)
	assert.Contains(t, output, "1.1.1.1") // IP preserved
}

// TestRenameKeyNamedTo verifies rename handles a key literally named "to".
//
// VALIDATES: "to" at second-to-last position is the separator, not a key.
// PREVENTS: Ambiguity when list key value is "to".
func TestRenameKeyNamedTo(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4
  session { asn { local 65000; } }
  peer to {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65001; } }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// "to" is at position len-2, so args = ["bgp", "peer", "to", "to", "newname"]
	// The second "to" is the separator, "to" before it is the old key
	result, err := model.cmdRename([]string{"bgp", "peer", "to", "to", "newname"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Renamed peer to to newname")

	output := ed.WorkingContent()
	assert.Contains(t, output, "peer newname")
}

// TestCopyListEntry verifies the copy command duplicates a peer.
//
// VALIDATES: copy <list> <src> to <dst> creates a clone with the new key.
// PREVENTS: Copy losing source entry or failing to deep-copy subtree.
func TestCopyListEntry(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdCopy([]string{"bgp", "peer", "peer1", "to", "peer2"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Copied peer peer1 to peer2")

	// Both entries should exist with the same config
	content := ed.WorkingContent()
	assert.Contains(t, content, "peer peer1")
	assert.Contains(t, content, "peer peer2")
	assert.Contains(t, content, "1.1.1.1") // IP preserved in both
	assert.True(t, ed.Dirty())
}

// TestCopyListEntryDeepCopy verifies the copy is independent of the source.
//
// VALIDATES: Modifying the copy does not affect the source.
// PREVENTS: Shallow copy causing shared state between entries.
func TestCopyListEntryDeepCopy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Copy peer1 to peer2
	result, err := model.cmdCopy([]string{"bgp", "peer", "peer1", "to", "peer2"})
	require.NoError(t, err)
	model.applyResult(result)

	// Modify peer2's IP
	editResult, err := model.cmdEdit([]string{"bgp", "peer", "peer2"})
	require.NoError(t, err)
	model.applyResult(editResult)

	_, err = model.cmdSet([]string{"connection", "remote", "ip", "2.2.2.2"})
	require.NoError(t, err)

	// peer1 should still have original IP
	content := ed.WorkingContent()
	assert.Contains(t, content, "peer peer1")
	assert.Contains(t, content, "peer peer2")
	// Count occurrences of each IP
	assert.Equal(t, 1, strings.Count(content, "1.1.1.1"), "peer1 should keep original IP")
	assert.Equal(t, 1, strings.Count(content, "2.2.2.2"), "peer2 should have new IP")
}

// TestCopyListEntryWithContext verifies copy works relative to context.
//
// VALIDATES: Copy uses context path for relative navigation.
// PREVENTS: Context-relative copy breaking path resolution.
func TestCopyListEntryWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Enter bgp context
	editResult, err := model.cmdEdit([]string{"bgp"})
	require.NoError(t, err)
	model.applyResult(editResult)

	result, err := model.cmdCopy([]string{"peer", "peer1", "to", "london"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Copied peer peer1 to london")

	content := ed.WorkingContent()
	assert.Contains(t, content, "peer peer1")
	assert.Contains(t, content, "peer london")
}

// TestCopyListEntryTargetExists verifies copy fails when target already exists.
//
// VALIDATES: Copy rejects duplicate target key.
// PREVENTS: Overwriting existing entry on copy.
func TestCopyListEntryTargetExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	content := `bgp {
  router-id 1.2.3.4
  session { asn { local 65000; } }
  peer alpha {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65001; } }
  }
  peer beta {
    connection { remote { ip 2.2.2.2; } }
    session { asn { remote 65002; } }
  }
}`
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.cmdCopy([]string{"bgp", "peer", "alpha", "to", "beta"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestCopyListEntryBadSyntax verifies copy rejects bad syntax.
//
// VALIDATES: Missing "to" keyword produces usage error.
// PREVENTS: Ambiguous copy commands silently misinterpreted.
func TestCopyListEntryBadSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	tests := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"missing to", []string{"bgp", "peer", "peer1", "peer2"}},
		{"too few args", []string{"peer1", "to", "peer2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := model.cmdCopy(tt.args)
			require.Error(t, err)
		})
	}
}

// TestInsertLeafList verifies insert command places values at correct positions.
//
// VALIDATES: AC-10 -- insert first/last/before/after on leaf-list.
// PREVENTS: Wrong insertion position in filter chains.
func TestInsertLeafList(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithFilter := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  filter {
    import [ alpha bravo ];
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

	err := os.WriteFile(configPath, []byte(configWithFilter), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Insert "charlie" after "alpha".
	result, err := model.cmdInsert([]string{"bgp", "filter", "import", "charlie", "after", "alpha"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Inserted charlie")

	content := ed.WorkingContent()
	assert.Contains(t, content, "import [ alpha charlie bravo ]")
}

// TestInsertLeafListFirst verifies insert first places value at beginning.
//
// VALIDATES: AC-10 -- insert first.
// PREVENTS: First position not prepending.
func TestInsertLeafListFirst(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithFilter := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  filter {
    import [ alpha bravo ];
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

	err := os.WriteFile(configPath, []byte(configWithFilter), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdInsert([]string{"bgp", "filter", "import", "zero", "first"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Inserted zero")

	content := ed.WorkingContent()
	assert.Contains(t, content, "import [ zero alpha bravo ]")
}

// TestInsertBadSyntax verifies insert rejects bad syntax.
//
// VALIDATES: Insert command validates arguments.
// PREVENTS: Cryptic errors from bad insert syntax.
func TestInsertBadSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Too few args.
	_, err = model.cmdInsert([]string{"filter", "import"})
	require.Error(t, err)

	// Missing position keyword.
	_, err = model.cmdInsert([]string{"filter", "import", "foo", "bar"})
	require.Error(t, err)
}

// TestInsertValidatesValueType verifies insert validates the value against
// the leaf-list's YANG type before applying, mirroring cmdSet.
//
// VALIDATES: insert rejects values that fail the leaf-list YANG type.
// PREVENTS: wrong-typed leaf-list members surfacing only at commit validation.
func TestInsertValidatesValueType(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithNameServer := testValidBGPConfigWithPeer + `
system {
	name-server [ 8.8.8.8 ]
}`

	err := os.WriteFile(configPath, []byte(configWithNameServer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// name-server is zt:ip-address: a non-IP value must be rejected
	// before it reaches the tree.
	_, err = model.cmdInsert([]string{"system", "name-server", "not-an-ip", "last"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
	assert.NotContains(t, ed.WorkingContent(), "not-an-ip")

	// A valid IP is accepted at the requested position.
	result, err := model.cmdInsert([]string{"system", "name-server", "1.1.1.1", "first"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Inserted 1.1.1.1")
	assert.Contains(t, ed.WorkingContent(), "name-server [ 1.1.1.1 8.8.8.8 ]")
}

// TestDeactivateLeafListValue verifies deactivate adds inactive: prefix.
//
// VALIDATES: AC-5 -- deactivate on leaf-list value adds inactive: prefix.
// PREVENTS: Deactivate only working on containers.
func TestDeactivateLeafListValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithFilter := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  filter {
    import [ no-self-as reject-bogons ];
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

	err := os.WriteFile(configPath, []byte(configWithFilter), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdDeactivate([]string{"bgp", "filter", "import", "no-self-as"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Deactivated no-self-as")

	// The member stays in the leaf line and the deactivation renders as an
	// `inactive: <leaf> <member>` statement, never the internal
	// "inactive:" prefix (which fails item validation on reparse).
	content := ed.WorkingContent()
	assert.Contains(t, content, "import [ no-self-as reject-bogons ]")
	assert.Contains(t, content, "inactive: import no-self-as")
	assert.NotContains(t, content, "inactive:no-self-as")
}

// TestActivateLeafListValue verifies activate removes inactive: prefix.
//
// VALIDATES: AC-6 -- activate on inactive leaf-list value removes prefix.
// PREVENTS: Activate only working on containers.
func TestActivateLeafListValue(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithFilter := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  filter {
    import [ inactive:no-self-as reject-bogons ];
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

	err := os.WriteFile(configPath, []byte(configWithFilter), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdActivate([]string{"bgp", "filter", "import", "no-self-as"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Activated no-self-as")

	content := ed.WorkingContent()
	assert.Contains(t, content, "no-self-as")
	assert.NotContains(t, content, "inactive:no-self-as")
}

// TestDeactivateLeafListPerPeer verifies deactivate works through list entry paths.
//
// VALIDATES: deactivate bgp peer X filter import Y works (list key in path).
// PREVENTS: resolveLeafListValue failing on list keys in path.
func TestDeactivateLeafListPerPeer(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithPeerFilter := `bgp {
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
    filter {
      import [ no-self-as reject-bogons ];
    }
  }
}`

	err := os.WriteFile(configPath, []byte(configWithPeerFilter), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.cmdDeactivate([]string{"bgp", "peer", "peer1", "filter", "import", "no-self-as"})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Deactivated no-self-as")

	content := ed.WorkingContent()
	assert.Contains(t, content, "inactive: import no-self-as")
	assert.NotContains(t, content, "inactive:no-self-as")
}

// TestInsertDuplicateRejected verifies insert rejects duplicate values.
//
// VALIDATES: InsertMultiValue prevents duplicates.
// PREVENTS: Same filter running twice in a chain.
func TestInsertDuplicateRejected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configWithFilter := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  filter {
    import [ alpha bravo ];
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

	err := os.WriteFile(configPath, []byte(configWithFilter), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// Inserting "alpha" again should fail.
	_, err = model.cmdInsert([]string{"bgp", "filter", "import", "alpha", "last"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestInsertNonLeafListRejected verifies insert rejects non-leaf-list targets.
//
// VALIDATES: cmdInsert validates target is a leaf-list.
// PREVENTS: Silent insert into non-leaf-list fields.
func TestInsertNonLeafListRejected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	// "router-id" is a leaf, not a leaf-list.
	_, err = model.cmdInsert([]string{"bgp", "router-id", "5.6.7.8", "last"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a leaf-list")
}

// TestCopyViaDispatch verifies copy works through the command dispatcher.
//
// VALIDATES: "copy bgp peer peer1 to peer2" dispatches correctly.
// PREVENTS: Copy command not registered in dispatch table.
func TestCopyViaDispatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	result, err := model.dispatchCommand("copy bgp peer peer1 to cloned-peer")
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "Copied peer peer1 to cloned-peer")

	content := ed.WorkingContent()
	assert.Contains(t, content, "peer peer1")
	assert.Contains(t, content, "peer cloned-peer")
}

// TestSetCLIFormat verifies `set cli format json` records the choice on the session.
//
// The choice used to be stored via env.Set, which is process-global: see
// TestSetCLIFormatIsSessionScoped for why that was wrong.
func TestSetCLIFormat(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	m := &Model{}
	ok := handleSetCLIFormat("set cli format json", m)
	assert.True(t, ok, "should handle set cli format")
	assert.Equal(t, "cli format set to json", m.statusMessage)
	assert.Equal(t, "json", m.cliFormat, "choice recorded on the session")
}

// TestSetCLIFormatIsSessionScoped verifies one operator's format choice does not
// change what every other operator sees.
//
// VALIDATES: AC-16 -- set cli format in session A leaves session B alone.
// PREVENTS: The pre-existing leak. env.Set (env.go:111-119) writes a package-global
// cache AND calls os.Setenv, so `set cli format json` over one SSH session changed
// the default output format for every other concurrent SSH and web CLI session on
// the box. The Model is per-session (session_factory.go:87), so the override
// belongs on it.
func TestSetCLIFormatIsSessionScoped(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	sessionA := &Model{}
	sessionB := &Model{}

	require.True(t, handleSetCLIFormat("set cli format json", sessionA))

	assert.Equal(t, "json", sessionA.cliFormat, "session A took the override")
	assert.Empty(t, sessionB.cliFormat, "session B must not inherit session A's choice")
	assert.NotEqual(t, "json", env.Get("ze.cli.format"),
		"set cli format must not write process-global state")
}

// TestSetCLIFormatInvalid verifies `set cli format bogus` returns an error.
func TestSetCLIFormatInvalid(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	m := &Model{}
	ok := handleSetCLIFormat("set cli format bogus", m)
	assert.True(t, ok, "should handle set cli format")
	assert.Contains(t, m.statusMessage, "invalid format")
	assert.Contains(t, m.statusMessage, "valid: json, ndjson, table, text, yaml")
}

// TestSetCLIFormatShow verifies `set cli format` (no value) shows current setting.
func TestSetCLIFormatShow(t *testing.T) {
	t.Setenv("ze.cli.format", "yaml")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	m := &Model{}
	ok := handleSetCLIFormat("set cli format", m)
	assert.True(t, ok, "should handle set cli format")
	assert.Equal(t, "cli format: yaml", m.statusMessage)
}

// TestSetCLIFormatNotMatched verifies unrelated commands are not intercepted.
func TestSetCLIFormatNotMatched(t *testing.T) {
	m := &Model{}
	assert.False(t, handleSetCLIFormat("set bgp peer something", m), "should not handle unrelated set commands")
	assert.False(t, handleSetCLIFormat("set cli formatting foo", m), "should not match prefix-only (regression: #1)")
	assert.False(t, handleSetCLIFormat("set cli formatjson", m), "should not match without space separator")
}

func TestAppendNewCompletions(t *testing.T) {
	t.Run("empty extra returns existing unchanged", func(t *testing.T) {
		existing := []Completion{{Text: "a", Description: "A"}}
		got := appendNewCompletions(existing, nil)
		assert.Equal(t, existing, got)
	})

	t.Run("non-overlapping entries appended", func(t *testing.T) {
		existing := []Completion{{Text: "a", Description: "A"}}
		extra := []Completion{{Text: "b", Description: "B"}}
		got := appendNewCompletions(existing, extra)
		assert.Len(t, got, 2)
		assert.Equal(t, "b", got[1].Text)
	})

	t.Run("duplicate text skipped", func(t *testing.T) {
		existing := []Completion{{Text: "show", Description: "Operational show"}}
		extra := []Completion{{Text: "show", Description: "Config show"}, {Text: "set", Description: "Set value"}}
		got := appendNewCompletions(existing, extra)
		assert.Len(t, got, 2)
		assert.Equal(t, "Operational show", got[0].Description, "first entry wins")
		assert.Equal(t, "set", got[1].Text)
	})

	t.Run("empty existing accepts all", func(t *testing.T) {
		extra := []Completion{{Text: "x"}, {Text: "y"}}
		got := appendNewCompletions(nil, extra)
		assert.Len(t, got, 2)
	})
}
