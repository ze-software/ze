package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// testShowModel creates a Model with a valid BGP config for show command tests.
func testShowModel(t *testing.T) *Model {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	return &model
}

// TestCmdShowDefault verifies bare "show" returns hierarchical tree.
//
// VALIDATES: AC-1 -- show with no columns enabled displays bare hierarchical tree.
// PREVENTS: show returning set+meta format when session is active.
// TestCmdOptionNoArgs verifies option with no arguments returns usage error.
//
// VALIDATES: option requires at least one argument.
// PREVENTS: Silent no-op when user types bare "option".
func TestCmdOptionNoArgs(t *testing.T) {
	m := testShowModel(t)

	_, err := m.cmdOption(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")

	_, err = m.cmdOption([]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}

// TestCmdOptionUnknownName verifies option with unknown name returns error.
//
// VALIDATES: Unknown option names are rejected.
// PREVENTS: Silent acceptance of invalid option names.
func TestCmdOptionUnknownName(t *testing.T) {
	m := testShowModel(t)

	_, err := m.cmdOption([]string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown option")
}

// TestCmdShowRejectsOldSubcommands verifies show rejects moved subcommands with guidance.
//
// VALIDATES: Old "show blame" syntax produces helpful error.
// PREVENTS: Silent wrong output when user uses old syntax.
func TestCmdShowRejectsOldSubcommands(t *testing.T) {
	m := testShowModel(t)

	for _, old := range [][]string{
		{cmdBlame},
		{cmdChanges},
		{colAuthor, cmdEnable},
		{cmdAll},
		{cmdNone},
	} {
		_, err := m.cmdShow(old)
		require.Error(t, err, "show %v should error", old)
		assert.Contains(t, err.Error(), "option", "error should mention 'option'")
	}
}

func TestCmdShowDefault(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShow(nil)
	require.NoError(t, err)
	// Default show returns configView (for diff gutter support)
	assert.NotNil(t, result.configView, "show should return configView for tree display")
	assert.Contains(t, result.configView.content, "bgp")
}

// TestCmdOptionColumnToggle verifies column enable/disable via option command.
//
// VALIDATES: AC-2 -- option author enable writes preference.
// PREVENTS: Column toggle silently ignored or routed to wrong handler.
func TestCmdOptionColumnToggle(t *testing.T) {
	m := testShowModel(t)

	// Enable author column
	result, err := m.cmdOption([]string{colAuthor, cmdEnable})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "author column enabled")
	assert.True(t, m.editor.ShowColumnEnabled(colAuthor))

	// Disable author column
	result, err = m.cmdOption([]string{colAuthor, cmdDisable})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "author column disabled")
	assert.False(t, m.editor.ShowColumnEnabled(colAuthor))

	// Query column state
	result, err = m.cmdOption([]string{colAuthor})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "author: disabled")
}

// TestCmdOptionAllNone verifies option all / option none.
//
// VALIDATES: AC-4 -- option all enables all columns; AC-5 -- option none disables all.
// PREVENTS: Bulk toggle missing a column.
func TestCmdOptionAllNone(t *testing.T) {
	m := testShowModel(t)

	// Enable all
	result, err := m.cmdOption([]string{cmdAll})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "All columns enabled")
	assert.True(t, m.editor.ShowColumnEnabled(colAuthor))
	assert.True(t, m.editor.ShowColumnEnabled(colDate))
	assert.True(t, m.editor.ShowColumnEnabled(colSource))
	assert.True(t, m.editor.ShowColumnEnabled(colChanges))

	// Disable all
	result, err = m.cmdOption([]string{cmdNone})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "All columns disabled")
	assert.False(t, m.editor.ShowColumnEnabled(colAuthor))
	assert.False(t, m.editor.ShowColumnEnabled(colDate))
	assert.False(t, m.editor.ShowColumnEnabled(colSource))
	assert.False(t, m.editor.ShowColumnEnabled(colChanges))
}

// TestCmdOptionChangesEnableDisambiguation verifies "option changes enable" toggles column,
// while "option changes" without enable/disable shows pending changes.
//
// VALIDATES: "changes" used as column name with enable/disable, vs view mode without.
// PREVENTS: "option changes enable" interpreted as pending changes subcommand.
func TestCmdOptionChangesEnableDisambiguation(t *testing.T) {
	m := testShowModel(t)

	// "option changes enable" -> column toggle
	result, err := m.cmdOption([]string{colChanges, cmdEnable})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "changes column enabled")
	assert.True(t, m.editor.ShowColumnEnabled(colChanges))

	// "option changes" without enable/disable -> requires session (view mode)
	_, err = m.cmdOption([]string{cmdChanges})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an active editing session")
}

// TestCmdShowWithArgs verifies show ignores args and displays tree.
//
// VALIDATES: show with args still displays default tree (args reserved for future path filtering).
// PREVENTS: Error on "show peer" or other schema path args.
func TestCmdShowWithArgs(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShow([]string{"peer"})
	require.NoError(t, err)
	assert.NotNil(t, result.configView, "show with args should display tree")
}

// TestCmdShowDisplayNoColumns verifies bare display uses standard serializers.
//
// VALIDATES: cmdShowDisplay with no columns enabled returns configView (tree).
// PREVENTS: Annotated serializer called when no columns enabled.
func TestCmdShowDisplayNoColumns(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShowDisplay(fmtTree, "")
	require.NoError(t, err)
	assert.NotNil(t, result.configView, "tree format returns configView")
	assert.Contains(t, result.configView.content, "bgp")
}

// TestCmdShowDisplayConfigFormat verifies | format config produces set commands.
//
// VALIDATES: AC-6 -- show | format config displays flat set-command format.
// PREVENTS: Format pipe ignored or tree format returned.
func TestCmdShowDisplayConfigFormat(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShowDisplay(fmtConfig, "")
	require.NoError(t, err)
	assert.Contains(t, result.output, "set bgp")
}

// TestCmdShowDisplayCompare verifies compare mode returns configView with original.
//
// VALIDATES: AC-8 -- show | compare committed shows diff markers.
// PREVENTS: Compare mode losing diff baseline (original content).
func TestCmdShowDisplayCompare(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShowDisplay(fmtTree, srcConfirmed)
	require.NoError(t, err)
	require.NotNil(t, result.configView, "compare mode returns configView")
	assert.True(t, result.configView.hasOriginal, "compare mode sets hasOriginal")
	assert.NotEmpty(t, result.configView.originalContent, "compare mode provides original content")
}

// TestCmdShowPipeFormatConfig verifies format pipe through cmdShowPipe.
//
// VALIDATES: AC-6 -- show | format config via pipe path.
// PREVENTS: Format pipe not recognized by cmdShowPipe.
func TestCmdShowPipeFormatConfig(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdFormat, Arg: fmtConfig}})
	require.NoError(t, err)
	assert.Contains(t, result.output, "set bgp")
}

// TestCmdShowPipeFormatInvalid verifies unknown format names produce error.
//
// VALIDATES: Security -- format names validated against fixed set.
// PREVENTS: Arbitrary format injection.
func TestCmdShowPipeFormatInvalid(t *testing.T) {
	m := testShowModel(t)

	_, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdFormat, Arg: "invalid"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

// TestCmdShowPipeComparePreservesConfigView verifies configView survives pipe with no text filters.
//
// VALIDATES: show | compare returns configView with hasOriginal intact.
// PREVENTS: configView lost during pipe processing.
func TestCmdShowPipeComparePreservesConfigView(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: ""}})
	require.NoError(t, err)
	require.NotNil(t, result.configView, "compare pipe should return configView")
	assert.True(t, result.configView.hasOriginal, "configView should have hasOriginal for diff gutter")
}

// TestEditorShowColumnPreferences verifies ShowColumnEnabled and SetShowColumn.
//
// VALIDATES: Column preferences round-trip correctly.
// PREVENTS: Unknown column names accepted, nil map panic.
func TestEditorShowColumnPreferences(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	// Initially all disabled
	assert.False(t, ed.ShowColumnEnabled(colAuthor))
	assert.False(t, ed.ShowColumnEnabled(colDate))

	// Enable author
	ed.SetShowColumn(colAuthor, true)
	assert.True(t, ed.ShowColumnEnabled(colAuthor))
	assert.False(t, ed.ShowColumnEnabled(colDate))

	// Unknown column name rejected
	ed.SetShowColumn("invalid", true)
	assert.False(t, ed.ShowColumnEnabled("invalid"))
}

// TestEditorSavedDraftContent verifies SavedDraftContent returns draft or empty.
//
// VALIDATES: SavedDraftContent returns content when draft exists, empty otherwise.
// PREVENTS: Panic on missing draft file.
func TestEditorSavedDraftContent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	// No draft exists -- empty string
	assert.Empty(t, ed.SavedDraftContent())

	// Create a draft file
	draftPath := configPath + ".draft"
	err = os.WriteFile(draftPath, []byte("draft content here"), 0o600)
	require.NoError(t, err)

	assert.Equal(t, "draft content here", ed.SavedDraftContent())
}

// TestWalkMetaPath verifies metadata tree navigation in parallel with config tree.
//
// VALIDATES: walkMetaPath resolves container paths and returns correct sub-MetaTree.
// PREVENTS: Metadata gutter showing wrong or empty data at sub-paths.
func TestWalkMetaPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // test cleanup

	// Set up a session with metadata
	session := NewEditSession("testuser", "local")
	require.NotNil(t, session)
	ed.SetSession(session)

	// Make a change to create metadata
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	t.Run("empty path returns empty meta", func(t *testing.T) {
		result := ed.walkMetaPath(ed.meta, nil)
		assert.NotNil(t, result)
	})

	t.Run("nil meta returns empty meta", func(t *testing.T) {
		result := ed.walkMetaPath(nil, []string{"bgp"})
		assert.NotNil(t, result)
	})

	t.Run("container path resolves", func(t *testing.T) {
		result := ed.walkMetaPath(ed.meta, []string{"bgp"})
		assert.NotNil(t, result)
		// Should have the router-id entry from our change
		if entries := result.AllEntries(); len(entries) > 0 {
			_, hasRouterID := entries["router-id"]
			assert.True(t, hasRouterID, "bgp sub-meta should contain router-id entry")
		}
	})

	t.Run("nonexistent path returns empty meta", func(t *testing.T) {
		result := ed.walkMetaPath(ed.meta, []string{"nonexistent"})
		assert.NotNil(t, result)
		assert.Empty(t, result.AllEntries())
	})
}

// TestResolveMetaListKey verifies positional index resolution in MetaTree.
//
// VALIDATES: #N positional syntax resolves to correct list entry metadata.
// PREVENTS: Panic on invalid indexes, wrong key resolution.
func TestResolveMetaListKey(t *testing.T) {
	t.Run("non-positional key returns nil", func(t *testing.T) {
		tree := config.NewTree()
		meta := config.NewMetaTree()
		assert.Nil(t, resolveMetaListKey(tree, "peer", "peer1", meta))
	})

	t.Run("nil tree returns nil", func(t *testing.T) {
		meta := config.NewMetaTree()
		assert.Nil(t, resolveMetaListKey(nil, "peer", "#1", meta))
	})

	t.Run("invalid index returns nil", func(t *testing.T) {
		tree := config.NewTree()
		meta := config.NewMetaTree()
		assert.Nil(t, resolveMetaListKey(tree, "peer", "#0", meta))
		assert.Nil(t, resolveMetaListKey(tree, "peer", "#abc", meta))
		assert.Nil(t, resolveMetaListKey(tree, "peer", "#-1", meta))
	})

	t.Run("out of range returns nil", func(t *testing.T) {
		tree := config.NewTree()
		entry := config.NewTree()
		tree.AddListEntry("peer", "peer1", entry)
		meta := config.NewMetaTree()
		assert.Nil(t, resolveMetaListKey(tree, "peer", "#99", meta))
	})
}

// TestCmdShowDisplayWithColumns verifies annotated output when columns are enabled.
//
// VALIDATES: cmdShowDisplay with columns enabled returns annotated content.
// PREVENTS: Annotated view path skipped when columns are on.
func TestCmdShowDisplayWithColumns(t *testing.T) {
	m := testShowModel(t)

	// Enable author column
	m.editor.SetShowColumn(colAuthor, true)

	result, err := m.cmdShowDisplay(fmtTree, "")
	require.NoError(t, err)
	// Should return output (not configView) with annotated content
	assert.NotEmpty(t, result.output, "annotated display should produce output")
	assert.Contains(t, result.output, "bgp")
}

// TestCmdOptionColumnToggleInvalidArg verifies error for invalid enable/disable arg.
//
// VALIDATES: option <column> <invalid> returns usage error.
// PREVENTS: Silent acceptance of invalid toggle values.
func TestCmdOptionColumnToggleInvalidArg(t *testing.T) {
	m := testShowModel(t)

	_, err := m.cmdOption([]string{colAuthor, "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}

// TestParsePipeFiltersRollback verifies "compare rollback N" is parsed as a single filter with combined arg.
//
// VALIDATES: AC-10a -- pipe parser keeps "rollback N" together.
// PREVENTS: "rollback" and "N" split into separate filters.
func TestParsePipeFiltersRollback(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   []PipeFilter
	}{
		{
			name:   "compare rollback 1",
			tokens: []string{"|", "compare", "rollback", "1"},
			want:   []PipeFilter{{Type: cmdCompare, Arg: "rollback 1"}},
		},
		{
			name:   "compare committed unchanged",
			tokens: []string{"|", "compare", "committed"},
			want:   []PipeFilter{{Type: cmdCompare, Arg: "committed"}},
		},
		{
			name:   "compare rollback 2 then format config",
			tokens: []string{"|", "compare", "rollback", "2", "|", "format", "config"},
			want: []PipeFilter{
				{Type: cmdCompare, Arg: "rollback 2"},
				{Type: cmdFormat, Arg: fmtConfig},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePipeFilters(tt.tokens)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCmdShowPipeCompareRollback verifies compare rollback N uses backup content as baseline.
//
// VALIDATES: AC-10a -- show | compare rollback 1 shows diff markers against rollback 1.
// PREVENTS: "rollback" treated as unknown target, N orphaned by pipe parser.
func TestCmdShowPipeCompareRollback(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	// Create a backup so rollback 1 exists.
	err = ed.createBackup(ed.OriginalContent(), nil)
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)
	m := &model

	// compare rollback 1 should return configView with original from backup.
	result, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: "rollback 1"}})
	require.NoError(t, err)
	require.NotNil(t, result.configView, "rollback compare returns configView")
	assert.True(t, result.configView.hasOriginal, "rollback compare sets hasOriginal")
	assert.NotEmpty(t, result.configView.originalContent, "rollback compare provides backup content")
}

// TestCmdShowPipeCompareRollbackInvalid verifies rollback N rejects invalid N.
//
// VALIDATES: AC-10a boundary -- rollback 0 and rollback beyond count produce error.
// PREVENTS: Index out of range panic, silent fallback to committed.
func TestCmdShowPipeCompareRollbackInvalid(t *testing.T) {
	m := testShowModel(t)

	// No backups exist: rollback 1 should error.
	_, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: "rollback 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup")

	// rollback 0 should error (1-based indexing).
	_, err = m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: "rollback 0"}})
	require.Error(t, err)

	// rollback abc should error.
	_, err = m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: "rollback abc"}})
	require.Error(t, err)
}

// TestCmdShowPipeCompareRollbackStackFormat verifies pipes stack: rollback + format.
//
// VALIDATES: AC-10b -- show | compare rollback 2 | format config stacks both pipes.
// PREVENTS: Format pipe lost when compare rollback is present.
func TestCmdShowPipeCompareRollbackStackFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	// Create two backups.
	err = ed.createBackup(ed.OriginalContent(), nil)
	require.NoError(t, err)
	err = ed.createBackup(ed.OriginalContent(), nil)
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)
	m := &model

	// compare rollback 2 + format config: should produce set-format content with diff baseline.
	result, err := m.cmdShowPipe(nil, []PipeFilter{
		{Type: cmdCompare, Arg: "rollback 2"},
		{Type: cmdFormat, Arg: fmtConfig},
	})
	require.NoError(t, err)
	// With format config + compare: content is rendered as set commands,
	// and configView carries the original for diff.
	require.NotNil(t, result.configView, "stacked pipes return configView")
	assert.True(t, result.configView.hasOriginal, "stacked pipes set hasOriginal")
}

// TestCmdShowConfirmed verifies "show confirmed" displays the committed config.
//
// VALIDATES: AC-20 -- show confirmed displays the committed (original) config.
// PREVENTS: show confirmed showing working config or erroring.
func TestCmdShowConfirmed(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	m := &model

	// Make a change so working differs from committed.
	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)
	require.True(t, ed.Dirty())

	// "show confirmed" should display the original (committed) config, not the working one.
	result, err := m.cmdShow([]string{srcConfirmed})
	require.NoError(t, err)
	// The committed config has the original router-id, not 9.9.9.9.
	var content string
	if result.configView != nil {
		content = result.configView.content
	} else {
		content = result.output
	}
	assert.NotContains(t, content, "9.9.9.9", "confirmed should not show working change")
	assert.Contains(t, content, "bgp", "confirmed should show config content")
}

// TestCmdShowSaved verifies "show saved" displays the saved draft file.
//
// VALIDATES: AC-21 -- show saved displays the saved draft content.
// PREVENTS: show saved showing working config or erroring when draft exists.
func TestCmdShowSaved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	// Create a draft file with different content.
	draftContent := "set bgp router-id 8.8.8.8\nset bgp local as 65000\n"
	draftPath := configPath + ".draft"
	err = os.WriteFile(draftPath, []byte(draftContent), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	model, err := NewModel(ed)
	require.NoError(t, err)
	m := &model

	// "show saved" should display the saved draft content.
	result, err := m.cmdShow([]string{srcSaved})
	require.NoError(t, err)
	var content string
	if result.configView != nil {
		content = result.configView.content
	} else {
		content = result.output
	}
	assert.Contains(t, content, "8.8.8.8", "saved should show draft content")
}

// TestCmdShowSavedNoDraft verifies "show saved" with no draft file returns helpful message.
//
// VALIDATES: AC-21 boundary -- no draft file produces informative error.
// PREVENTS: Empty viewport or panic when draft doesn't exist.
func TestCmdShowSavedNoDraft(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShow([]string{srcSaved})
	require.NoError(t, err)
	assert.Contains(t, result.output, "no saved draft", "should indicate missing draft")
}

// TestCmdShowCompareUsername verifies "show | compare <username>" shows diff for user's changes.
//
// VALIDATES: AC-23 -- show | compare <username> shows diff markers for the specified user.
// PREVENTS: Unknown username silently accepted without diff.
func TestCmdShowCompareUsername(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	err := os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600)
	require.NoError(t, err)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup

	// Start a session and make changes so metadata exists.
	session := NewEditSession("alice", "local")
	ed.SetSession(session)
	err = ed.SetValue([]string{"bgp"}, "router-id", "7.7.7.7")
	require.NoError(t, err)

	model, err := NewModel(ed)
	require.NoError(t, err)
	m := &model

	// "show | compare alice" should return configView with diff baseline.
	result, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: "alice"}})
	require.NoError(t, err)
	require.NotNil(t, result.configView, "compare user returns configView")
	assert.True(t, result.configView.hasOriginal, "compare user sets hasOriginal")
	// The baseline should be the config without alice's changes.
	assert.NotContains(t, result.configView.originalContent, "7.7.7.7",
		"baseline should not contain alice's change")
}
