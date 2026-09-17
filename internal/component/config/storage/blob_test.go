package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenBlobRefusesCorruptionWithoutSelfHealing(t *testing.T) {
	for _, content := range [][]byte{nil, []byte("not a blob")} {
		dir := t.TempDir()
		path := filepath.Join(dir, "database.zefs")
		require.NoError(t, os.WriteFile(path, content, 0o600))
		for _, writable := range []bool{false, true} {
			s, err := OpenBlob(path, writable)
			if s != nil {
				require.NoError(t, s.Close())
			}
			require.Error(t, err)
			got, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, string(content), string(got))
			aside, globErr := filepath.Glob(path + ".replaced-*")
			require.NoError(t, globErr)
			assert.Empty(t, aside, "open must not rename corrupt artifacts")
		}
	}
}

func TestBlobArtifactPersistenceAndBasenameMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.zefs")
	s, err := CreateBlob(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	require.NoError(t, s.WriteFile("/etc/ze/router.conf", []byte("persisted"), 0))
	require.NoError(t, s.Close())
	reader, err := OpenBlob(path, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	got, err := reader.ReadFile("/another/folder/router.conf")
	require.NoError(t, err)
	assert.Equal(t, "persisted", string(got))
	keys, err := reader.List("/etc/ze")
	require.NoError(t, err)
	assert.Equal(t, []string{"file/active/router.conf"}, keys)
	_, err = CreateBlob(path)
	require.Error(t, err, "explicit create must not overwrite an existing artifact")
}

func TestCreateBlobDoesNotImportLooseFiles(t *testing.T) {
	dir := t.TempDir()
	loose := filepath.Join(dir, "router.conf")
	require.NoError(t, os.WriteFile(loose, []byte("loose config"), 0o600))
	s := newBlobStorageAt(t, dir)
	assert.False(t, s.Exists("router.conf"))
	keys, err := s.ListKeys("")
	require.NoError(t, err)
	assert.Empty(t, keys)
	got, err := os.ReadFile(loose)
	require.NoError(t, err)
	assert.Equal(t, "loose config", string(got))
}
