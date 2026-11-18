package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestCommitSessionOnAVersionedConfigKeepsEarlierEdits: a config that has a
// recorded active version (an import or a restore gave it one, as a hub's
// client-<name>.conf has) keeps every committed edit across editor sessions.
//
// PREVENTS: the commit writing the plain active file and leaving the active
// version pointer on the older version. The next session read the config
// through that pointer, opened the text from before the last edit, and its
// own commit dropped that edit with no conflict reported.
//
// Method: give the config an active version, commit one edit, then open a
// second session. It MUST show the first edit, and after it commits a second
// edit both MUST be in what the store answers as the active config and in the
// plain file a hub serves.
func TestCommitSessionOnAVersionedConfigKeepsEarlierEdits(t *testing.T) {
	first, store, configPath := newBlobEditor(t, validBGPConfig)
	require.NoError(t, first.Close())

	_, err := storage.RestoreConfig(store, configPath, []byte(validBGPConfig))
	require.NoError(t, err)

	edit := func(path []string, leaf, value string) {
		t.Helper()
		ed, openErr := NewEditorWithStorage(store, configPath)
		require.NoError(t, openErr)
		defer ed.Close() //nolint:errcheck // test cleanup
		ed.SetSession(NewEditSession("thomas", "local"))
		require.NoError(t, ed.SetValue(path, leaf, value))
		result, commitErr := ed.CommitSession()
		require.NoError(t, commitErr)
		require.Empty(t, result.Conflicts)
	}

	edit([]string{"bgp"}, "router-id", "9.9.9.9")

	second, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	assert.Contains(t, second.WorkingContent(), "9.9.9.9", "the next session opens the config without the committed edit")
	require.NoError(t, second.Close())

	edit([]string{"bgp", "session", "asn"}, "local", "65099")

	active, err := storage.ReadActiveConfig(store, configPath)
	require.NoError(t, err)
	assert.Contains(t, string(active), "9.9.9.9", "the first edit is gone from the active config")
	assert.Contains(t, string(active), "65099", "the second edit is not in the active config")

	served, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, string(active), string(served), "the plain active file and the active version disagree")
}
