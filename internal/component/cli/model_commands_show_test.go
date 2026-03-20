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
func TestCmdShowDefault(t *testing.T) {
	m := testShowModel(t)

	result, err := m.cmdShow(nil)
	require.NoError(t, err)
	// Default show returns configView (for diff gutter support)
	assert.NotNil(t, result.configView, "show should return configView for tree display")
	assert.Contains(t, result.configView.content, "bgp")
}

// TestCmdShowColumnToggle verifies column enable/disable.
//
// VALIDATES: AC-2 -- show author enable writes preference.
// PREVENTS: Column toggle silently ignored or routed to wrong handler.
func TestCmdShowColumnToggle(t *testing.T) {
	m := testShowModel(t)

	// Enable author column
	result, err := m.cmdShow([]string{colAuthor, cmdEnable})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "author column enabled")
	assert.True(t, m.editor.ShowColumnEnabled(colAuthor))

	// Disable author column
	result, err = m.cmdShow([]string{colAuthor, cmdDisable})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "author column disabled")
	assert.False(t, m.editor.ShowColumnEnabled(colAuthor))

	// Query column state
	result, err = m.cmdShow([]string{colAuthor})
	require.NoError(t, err)
	assert.Contains(t, result.output, "author: disabled")
}

// TestCmdShowAllNone verifies show all / show none.
//
// VALIDATES: AC-4 -- show all enables all columns; AC-5 -- show none disables all.
// PREVENTS: Bulk toggle missing a column.
func TestCmdShowAllNone(t *testing.T) {
	m := testShowModel(t)

	// Enable all
	result, err := m.cmdShow([]string{cmdAll})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "All columns enabled")
	assert.True(t, m.editor.ShowColumnEnabled(colAuthor))
	assert.True(t, m.editor.ShowColumnEnabled(colDate))
	assert.True(t, m.editor.ShowColumnEnabled(colSource))
	assert.True(t, m.editor.ShowColumnEnabled(colChanges))

	// Disable all
	result, err = m.cmdShow([]string{cmdNone})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "All columns disabled")
	assert.False(t, m.editor.ShowColumnEnabled(colAuthor))
	assert.False(t, m.editor.ShowColumnEnabled(colDate))
	assert.False(t, m.editor.ShowColumnEnabled(colSource))
	assert.False(t, m.editor.ShowColumnEnabled(colChanges))
}

// TestCmdShowChangesEnableDisambiguation verifies "show changes enable" toggles column,
// while "show changes all" shows pending changes.
//
// VALIDATES: "changes" used as column name with enable/disable, vs subcommand with "all".
// PREVENTS: "show changes enable" interpreted as pending changes subcommand.
func TestCmdShowChangesEnableDisambiguation(t *testing.T) {
	m := testShowModel(t)

	// "show changes enable" -> column toggle
	result, err := m.cmdShow([]string{colChanges, cmdEnable})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "changes column enabled")
	assert.True(t, m.editor.ShowColumnEnabled(colChanges))

	// "show changes" without enable/disable -> requires session (subcommand)
	_, err = m.cmdShow([]string{cmdChanges})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an active editing session")
}

// TestCmdShowUnknownSubcommand verifies unknown args produce an error.
//
// VALIDATES: Unknown show subcommands are rejected.
// PREVENTS: Silent fallthrough to default display for typos.
func TestCmdShowUnknownSubcommand(t *testing.T) {
	m := testShowModel(t)

	_, err := m.cmdShow([]string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown show subcommand")
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

// TestCmdShowColumnToggleInvalidArg verifies error for invalid enable/disable arg.
//
// VALIDATES: show <column> <invalid> returns usage error.
// PREVENTS: Silent acceptance of invalid toggle values.
func TestCmdShowColumnToggleInvalidArg(t *testing.T) {
	m := testShowModel(t)

	_, err := m.cmdShow([]string{colAuthor, "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}
