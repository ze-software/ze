// Design: docs/architecture/web-interface.md -- configured private backup storage.
package artifact_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/core/env"
)

func backupDirectoryEnvironment(t *testing.T, directory string) {
	t.Helper()
	// Restore inherited spellings before clearing the lookup cache again.
	t.Cleanup(env.ResetCache)
	t.Setenv("ze.backup.directory", directory)
	t.Setenv("ZE_BACKUP_DIRECTORY", directory)
	env.ResetCache()
}

func backupDirectoryConfig(t *testing.T, directory string) {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse("environment { backup { directory " + strconv.Quote(directory) + "; } }")
	require.NoError(t, err, "the configured backup directory must exist in the real always-linked YANG schema")
	values := config.ExtractEnvironment(tree)
	require.Equal(t, directory, values["backup"]["directory"])
	require.Contains(t, config.EnvironmentEntries(values), "ze.backup.directory="+directory)
	config.ApplyEnvConfig(values)
}

// TestBackupDirectoryYANGSelectsSharedArtifactRoot follows a parsed configuration
// through environment plumbing into the production shared owner. Uploaded and
// current-system backups use the selected private root; stopping that owner
// removes its children while retaining unrelated content in the parent folder.
func TestBackupDirectoryYANGSelectsSharedArtifactRoot(t *testing.T) {
	backupDirectoryEnvironment(t, "")
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o750))
	marker := filepath.Join(parent, "keep.txt")
	require.NoError(t, os.WriteFile(marker, []byte("unrelated parent content"), 0o600))
	backupDirectoryConfig(t, parent)
	owner := artifact.New()
	fixture := newPolicyHTTP(t, owner, false)
	upload := fixture.upload(t, fixture.source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	state := owner.FixtureState()
	require.Equal(t, parent, filepath.Dir(state.Root))
	info, err := os.Stat(state.Root)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	parentInfo, err := os.Stat(parent)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o750), parentInfo.Mode().Perm(), "an existing parent must keep its configured mode")
	for _, path := range state.Workspaces {
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, store.WriteKey("test/directory/private", []byte{0, 255, 128}))
	storagecli.InstallDataRPC(&storagecli.DataRPCTarget{Store: store})
	t.Cleanup(func() {
		storagecli.InstallDataRPC(nil)
		require.NoError(t, store.Close())
	})
	download := fixture.request(t, "/backups/system-download", "alice", strings.NewReader(""), "application/x-www-form-urlencoded", "https://example.com")
	require.Equal(t, http.StatusOK, download.Code, download.Body.String())
	path := filepath.Join(t.TempDir(), "system.zefs")
	require.NoError(t, os.WriteFile(path, download.Body.Bytes(), 0o600))
	require.NoError(t, storage.CheckArtifact(path))
	require.Equal(t, state.Root, owner.FixtureState().Root)
	require.Equal(t, state.Workspaces, owner.FixtureState().Workspaces)
	fixture.server.Stop()
	_, err = os.Stat(state.Root)
	require.ErrorIs(t, err, os.ErrNotExist)
	kept, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.True(t, bytes.Equal([]byte("unrelated parent content"), kept))
}

// TestBackupDirectoryOSOverrideWins preserves the established OS-over-YANG
// priority instead of accepting the leaf while creating files somewhere else.
func TestBackupDirectoryOSOverrideWins(t *testing.T) {
	parent := t.TempDir()
	backupDirectoryEnvironment(t, parent)
	backupDirectoryConfig(t, filepath.Join(t.TempDir(), "configured"))
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	root, err := owner.FixtureRoot()
	require.NoError(t, err)
	require.Equal(t, parent, filepath.Dir(root))
}

// TestBackupDirectoryExplicitRelativePathRefuses prevents an invalid explicit
// override from silently placing credential-bearing files in the default temp.
func TestBackupDirectoryExplicitRelativePathRefuses(t *testing.T) {
	backupDirectoryEnvironment(t, "relative-backups")
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	root, err := owner.FixtureRoot()
	require.Error(t, err)
	require.Empty(t, root)
	require.Empty(t, owner.FixtureState().Root)
}

// TestBackupDirectoryYANGRejectsRelativePath exercises native YANG restrictions
// through the production parser, before invalid folder text reaches the owner.
func TestBackupDirectoryYANGRejectsRelativePath(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	_, err = config.NewParser(schema).Parse(`environment { backup { directory "relative-backups"; } }`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute")
}

// TestBackupDirectoryCreatesMissingConfiguredParent preserves user-owned parent
// folders after removing the private workspace and never needs a temp fallback.
func TestBackupDirectoryCreatesMissingConfiguredParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "new-parent", "backups")
	backupDirectoryEnvironment(t, parent)
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	root, err := owner.FixtureRoot()
	require.NoError(t, err)
	require.Equal(t, parent, filepath.Dir(root))
	require.NoError(t, owner.Close())
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	require.Empty(t, entries)
}

// TestBackupDirectoryFileParentRefuses forces a real directory-creation failure
// even as root; an explicit unusable location must never fall back to OS temp.
func TestBackupDirectoryFileParentRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("keep this original file"), 0o600))
	backupDirectoryEnvironment(t, path)
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	root, err := owner.FixtureRoot()
	require.Error(t, err)
	require.Empty(t, root)
	require.Empty(t, owner.FixtureState().Root)
	kept, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "keep this original file", string(kept))
}

// TestBackupDirectoryUsesPlatformDataOverride reaches the production default
// selector with a test-owned data directory, without creating host data files.
func TestBackupDirectoryUsesPlatformDataOverride(t *testing.T) {
	backupDirectoryEnvironment(t, "")
	data := t.TempDir()
	t.Setenv("ze.data.dir", data)
	t.Setenv("ZE_DATA_DIR", data)
	env.ResetCache()
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	root, err := owner.FixtureRoot()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(data, "backups"), filepath.Dir(root))
}

// TestBackupDirectoryExistingOwnerRetainsItsRoot models a new startup setting:
// a fresh owner uses it while the previous owner's root and data stay intact.
func TestBackupDirectoryExistingOwnerRetainsItsRoot(t *testing.T) {
	first := t.TempDir()
	backupDirectoryEnvironment(t, first)
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	root, err := owner.FixtureRoot()
	require.NoError(t, err)
	marker := filepath.Join(root, "retained-workspace")
	require.NoError(t, os.WriteFile(marker, []byte("keep active work"), 0o600))
	second := t.TempDir()
	// A fresh process starts with its new environment. This does not simulate
	// applying a leaf live: existing environment policy requires a restart.
	t.Setenv("ze.backup.directory", second)
	t.Setenv("ZE_BACKUP_DIRECTORY", second)
	env.ResetCache()
	keptRoot, err := owner.FixtureRoot()
	require.NoError(t, err)
	require.Equal(t, root, keptRoot)
	newOwner := artifact.New()
	t.Cleanup(func() { require.NoError(t, newOwner.Close()) })
	newRoot, err := newOwner.FixtureRoot()
	require.NoError(t, err)
	require.Equal(t, second, filepath.Dir(newRoot))
	kept, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "keep active work", string(kept))
}
