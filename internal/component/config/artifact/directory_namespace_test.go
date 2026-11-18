// Design: docs/architecture/storage-backends.md -- backup parents preserve the live store namespace.
package artifact_test

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
)

// backupDirectoryPhysicalTree observes actual files, because an already-open
// store's key index can hide an unrelated writer's new database entries.
func backupDirectoryPhysicalTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	values := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			values[name] = nil
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		values[name] = content
		return nil
	})
	require.NoError(t, err)
	return values
}

// TestBackupDirectoryPreservesLiveStoreNamespace uses the actual installed
// source and shared-server upload path. Database and reserved canonical parents
// must refuse before creating anything; an ordinary sibling remains useful.
func TestBackupDirectoryPreservesLiveStoreNamespace(t *testing.T) {
	for _, test := range []struct {
		name, parent string
		allowed      bool
		linked       bool
	}{
		{name: "live tree", parent: storage.TreeName},
		{name: "live descendant", parent: filepath.Join(storage.TreeName, "backup-folder")},
		{name: "absent canonical name", parent: storage.BlobName},
		{name: "absent canonical descendant", parent: filepath.Join(storage.BlobName, "backup-folder")},
		{name: "linked live descendant", parent: "backup-folder", linked: true},
		{name: "sibling", parent: "backups", allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			store, err := storage.Create(directory)
			require.NoError(t, err)
			require.NoError(t, store.WriteKey("test/directory/kept", []byte("unchanged live bytes")))
			storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: store})
			t.Cleanup(func() {
				storagecli.InstallDataRPC(nil)
				require.NoError(t, store.Close())
			})
			parent := filepath.Join(directory, test.parent)
			if test.linked {
				link := filepath.Join(t.TempDir(), "live-tree")
				require.NoError(t, os.Symlink(filepath.Join(directory, storage.TreeName), link))
				parent = filepath.Join(link, test.parent)
			}
			backupDirectoryEnvironment(t, parent)
			before := backupDirectoryPhysicalTree(t, directory)
			liveBefore := backupDirectoryPhysicalTree(t, filepath.Join(directory, storage.TreeName))
			owner := artifact.New()
			fixture := newPolicyHTTP(t, owner, false)
			response := fixture.upload(t, fixture.source, "alice", "https://example.com")
			if test.allowed {
				require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
				require.Equal(t, filepath.Join(directory, test.parent), filepath.Dir(owner.FixtureState().Root))
			} else {
				require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
				require.Empty(t, owner.FixtureState().Root)
				require.Equal(t, before, backupDirectoryPhysicalTree(t, directory), "refused backup parents must create no files or folders")
			}
			require.Equal(t, liveBefore, backupDirectoryPhysicalTree(t, filepath.Join(directory, storage.TreeName)))
			kept, err := store.ReadKey("test/directory/kept")
			require.NoError(t, err)
			require.Equal(t, "unchanged live bytes", string(kept))
		})
	}
}
