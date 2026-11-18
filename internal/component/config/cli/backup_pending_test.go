// Design: docs/guide/config-editor.md -- one publication owner for local and daemon artifacts.
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestBackupSetPreservesPendingCandidate proves local --backup publication
// changes committed content while retaining a different saved candidate, as
// owner-held artifact operations do. The original draft bytes remain separate.
func TestBackupSetPreservesPendingCandidate(t *testing.T) {
	path := newBackupArtifact(t)
	store, err := openBackup(path, true)
	require.NoError(t, err)
	pending := []byte(strings.ReplaceAll(backupTestConfig, "1.2.3.4", "192.0.2.55"))
	stamp, err := storage.WriteCandidateVersion(store, "router.conf", pending, time.Now())
	require.NoError(t, err)
	require.NoError(t, store.WriteFile("router.conf.draft", []byte("saved draft to preserve"), 0o600))
	require.NoError(t, store.Close())
	code, diagnostic := captureStderr(t, func() int {
		return cmdSetImpl(nil, []string{"--backup", path, "router.conf", "bgp", "router-id", "192.0.2.99"})
	})
	require.Equal(t, exitOK, code, diagnostic)
	store, err = openBackup(path, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	current, err := storage.ReadActiveConfig(store, "router.conf")
	require.NoError(t, err)
	require.Contains(t, string(current), "192.0.2.99")
	saved, savedStamp, exists, err := storage.ReadCandidateConfig(store, "router.conf")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, stamp, savedStamp)
	require.Equal(t, pending, saved)
	draft, err := store.ReadFile("router.conf.draft")
	require.NoError(t, err)
	require.Equal(t, "saved draft to preserve", string(draft))
}
