// Design: docs/guide/config-editor.md -- session rollback is a candidate edit.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestSessionRollbackStagesUntilCommit restores verified history into a real
// session and proves storage changes only when that candidate is committed.
func TestSessionRollbackStagesUntilCommit(t *testing.T) {
	ed, _, path := newLoadSessionEditor(t, validBGPConfig)
	backup := parseLoadInput(t, ed, validBGPConfig)
	backup.GetContainer("bgp").Set("router-id", "203.0.113.15")
	require.NoError(t, ed.createBackup(config.Serialize(backup, ed.schema), nil))
	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.NotEmpty(t, backups)
	committed, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, ed.Rollback(backups[0].Path))
	assert.Contains(t, ed.ContentAtPath([]string{"bgp"}), "203.0.113.15")
	unchanged, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged, "rollback changed committed bytes before review/commit")
	assert.True(t, ed.Dirty())
	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	changed, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(changed), "203.0.113.15")
}

// TestSessionRollbackRefusesMalformedBackupAtomically proves a hash-verified
// but invalid saved configuration replaces neither pending nor committed data.
func TestSessionRollbackRefusesMalformedBackupAtomically(t *testing.T) {
	ed, _, path := newLoadSessionEditor(t, validBGPConfig)
	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "198.51.100.15"))
	candidate := ed.WorkingContent()
	committed, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, ed.createBackup("bgp { not-a-setting true; }", nil))
	backups, err := ed.ListBackups()
	require.NoError(t, err)
	require.NotEmpty(t, backups)
	require.Error(t, ed.Rollback(backups[0].Path))
	assert.Equal(t, candidate, ed.WorkingContent())
	unchanged, err := ed.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged)
	assert.Contains(t, ed.ContentAtPath([]string{"bgp"}), "198.51.100.15")
}
