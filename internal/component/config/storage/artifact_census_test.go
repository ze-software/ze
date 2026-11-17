// Design: docs/architecture/storage-backends.md -- artifact configuration census.

package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestArtifactConfigNamesExcludesEditorRecords writes the editor's actual
// draft and account-change paths beside committed and mirror-only configs.
// The census must keep every configuration while leaving editor records alone.
// MUTATION: counting every active mirror offers saved editor work as a config.
func TestArtifactConfigNamesExcludesEditorRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.zefs")
	artifact, err := storage.CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	_, err = storage.RestoreConfig(artifact, "router.conf", []byte("committed"))
	require.NoError(t, err)
	draftPath := cli.DraftPath("router.conf")
	changePath := cli.ChangePath("router.conf", "alice")
	require.NoError(t, artifact.WriteFile(draftPath, []byte("saved draft"), 0o600))
	require.NoError(t, artifact.WriteFile(changePath, []byte("pending account intent"), 0o600))
	for _, name := range []string{"seed.cfg", "startup", "archive.draft", "release.change.alice"} {
		require.NoError(t, artifact.WriteFile(name, []byte(name), 0o600))
	}
	before := artifactCensusKeys(t, artifact)
	names, err := storage.ArtifactConfigNames(artifact)
	require.NoError(t, err)
	assert.Equal(t, []string{"archive.draft", "release.change.alice", "router.conf", "seed.cfg", "startup"}, names)
	assert.Equal(t, before, artifactCensusKeys(t, artifact), "listing must preserve every raw key")
}

// TestArtifactConfigNamesKeepsExplicitPointer proves a committed configuration's
// pointer is authoritative even when its name resembles another config's draft.
func TestArtifactConfigNamesKeepsExplicitPointer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.zefs")
	artifact, err := storage.CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, artifact.Close()) })
	for _, name := range []string{"router.conf", cli.DraftPath("router.conf"), cli.ChangePath("router.conf", "alice")} {
		_, err := storage.RestoreConfig(artifact, name, []byte(name))
		require.NoError(t, err)
	}
	names, err := storage.ArtifactConfigNames(artifact)
	require.NoError(t, err)
	assert.Equal(t, []string{"router.conf", "router.conf.change.alice", "router.conf.draft"}, names)
}

// TestReadRestoreSourceKeepsExplicitLegacyMirror proves that filtering the
// browser picker does not remove a name the existing restore API can select.
func TestReadRestoreSourceKeepsExplicitLegacyMirror(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.zefs")
	artifact, err := storage.CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.WriteFile("router.conf", []byte("base config"), 0o600))
	name := cli.DraftPath("router.conf")
	require.NoError(t, artifact.WriteFile(name, []byte("legacy explicit config"), 0o600))
	require.NoError(t, artifact.Close())
	selected, err := storage.ReadRestoreSource(path, name, "router.conf")
	require.NoError(t, err)
	assert.Equal(t, name, selected.Name)
	assert.Equal(t, []byte("legacy explicit config"), selected.Data)
}

// artifactCensusKeys observes the underlying namespace without name mapping.
func artifactCensusKeys(t *testing.T, source storage.Storage) map[string]string {
	t.Helper()
	keys, err := source.ListKeys("")
	require.NoError(t, err)
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		data, err := source.ReadKey(key)
		require.NoError(t, err)
		values[key] = string(data)
	}
	return values
}
