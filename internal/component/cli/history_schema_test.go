// Design: docs/guide/config-editor.md -- recall resolves the owning editor's schema.
package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

func historyOwnerSchema() *config.Schema {
	secret := config.Leaf(config.TypeString)
	secret.Sensitive = true
	schema := config.NewSchema()
	schema.Define("history-owned", config.Container(
		config.Field("ordinary", config.Leaf(config.TypeString)),
		config.Field("material", secret),
	))
	return schema
}

// TestHistoryWithMaskRequiresProducer proves a missing producer creates no
// history owner that could later expose unmasked recall.
func TestHistoryWithMaskRequiresProducer(t *testing.T) {
	history, err := NewHistoryWithMask(nil, "owner", nil)
	require.Error(t, err)
	assert.Nil(t, history)
}

// TestHistoryWithOwnerMaskReopensSafeValues proves a supplied schema governs
// both new publication and legacy recall without another persistence format.
func TestHistoryWithOwnerMaskReopensSafeValues(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	ed := &Editor{schema: historyOwnerSchema()}
	history, err := NewHistoryWithMask(store, "owner", ed.DisplayCommandAtPath)
	require.NoError(t, err)
	t.Cleanup(history.Close)
	_, err = history.Record(ModeConfig, []string{"history-owned"}, "set ordinary public-value")
	require.NoError(t, err)
	_, err = history.Record(ModeConfig, []string{"history-owned"}, "set material private-value")
	require.NoError(t, err)
	history.Close()
	stored, err := store.ReadFile("meta/history/owner/config")
	require.NoError(t, err)
	assert.Contains(t, string(stored), "top set history-owned ordinary public-value")
	assert.NotContains(t, string(stored), "private-value")
	require.NoError(t, store.WriteFile("meta/history/owner/config", append(stored, []byte("\nset history-owned material legacy-secret")...), 0))
	fresh, err := NewHistoryWithMask(store, "owner", ed.DisplayCommandAtPath)
	require.NoError(t, err)
	t.Cleanup(fresh.Close)
	entries, err := fresh.Snapshot(ModeConfig)
	require.NoError(t, err)
	assert.Contains(t, entries, "top set history-owned ordinary public-value")
	assert.NotContains(t, strings.Join(entries, "\n"), "legacy-secret")
}

// TestModelInitialHistoryUsesOwnerSchema proves even memory-only recall binds
// the editor before its first Record, preserving useful ordinary assignments.
func TestModelInitialHistoryUsesOwnerSchema(t *testing.T) {
	model := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, model.editor.Close()) })
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	model.editor.schema = historyOwnerSchema()
	model.recordHistory("set history-owned ordinary public-value")
	model.recordHistory("set history-owned material private-value")
	assert.Equal(t, []string{
		"set history-owned ordinary public-value",
		"set history-owned material <redacted>",
	}, model.history.Entries())
}

// TestModelSetHistoryBindsOwnerBeforeLoad proves setup masks legacy data with
// the editor's schema before either mode becomes available to the model.
func TestModelSetHistoryBindsOwnerBeforeLoad(t *testing.T) {
	model := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, model.editor.Close()) })
	store := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	model.editor.schema = historyOwnerSchema()
	const legacy = "set history-owned ordinary public-value\nset history-owned material legacy-secret"
	for _, mode := range []EditorMode{ModeConfig, ModeOperational} {
		require.NoError(t, store.WriteFile("meta/history/owner/"+mode.String(), []byte(legacy), 0))
	}
	model.SetHistory(NewHistory(store, "owner"))
	for _, mode := range []EditorMode{ModeConfig, ModeOperational} {
		entries, err := model.history.Snapshot(mode)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"set history-owned ordinary public-value",
			"set history-owned material <redacted>",
		}, entries)
	}
}
