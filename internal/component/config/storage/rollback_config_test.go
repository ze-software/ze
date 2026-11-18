// Design: docs/architecture/storage-backends.md -- shared indexed config rollback.

package storage

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

func readRollbackRevisionUnderTest(t *testing.T, source Storage, name string, revision int) (VersionInfo, []byte, error) {
	t.Helper()
	reader, found := source.(interface {
		ReadRollbackRevision(string, int) (VersionInfo, []byte, error)
	})
	require.True(t, found, "the store exposes no guarded rollback revision reader")
	return reader.ReadRollbackRevision(name, revision)
}

func rollbackConfigUnderTest(t *testing.T, source Storage, revision int) (VersionInfo, string, error) {
	t.Helper()
	rollback, found := source.(interface {
		RollbackConfig(string, int) (VersionInfo, string, error)
	})
	require.True(t, found, "the store exposes no versioned configuration rollback")
	return rollback.RollbackConfig("router.conf", revision)
}

func seedConfigRollback(t *testing.T, source Storage) {
	t.Helper()
	require.NoError(t, source.WriteFile("router.conf", []byte("active third"), 0o600))
	_, _, err := EnsureActiveVersion(source, "router.conf", []byte("active third"), historyStampC)
	require.NoError(t, err)
	require.NoError(t, source.WriteVersion("router.conf", []byte("first revision"), historyStampA))
	require.NoError(t, source.WriteVersion("router.conf", []byte("second revision"), historyStampB))
	require.NoError(t, source.WriteFile("other.conf", []byte("unrelated config"), 0o600))
}

// TestConfigRollbackRevisionUsesExistingHistoryOrder proves the guarded
// selector reads the same one-based order already returned to Editor.ListBackups
// by ListVersions, verifies the bytes and changes no stored key.
func TestConfigRollbackRevisionUsesExistingHistoryOrder(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			source := backend.newStore(t, t.TempDir())
			seedConfigRollback(t, source)
			versions, err := source.ListVersions("router.conf")
			require.NoError(t, err)
			require.Len(t, versions, 3)
			require.Equal(t, FormatVersionStamp(historyStampB), versions[1].Stamp)
			before := configRenameSnapshot(t, source)
			selected, data, err := readRollbackRevisionUnderTest(t, source, "router.conf", 2)
			require.NoError(t, err)
			require.Equal(t, versions[1], selected)
			require.Equal(t, "second revision", string(data))
			require.Equal(t, before, configRenameSnapshot(t, source))
		})
	}
}

// TestConfigRollbackPublishesANewActiveVersion proves rollback changes the
// authoritative pointer as well as the mirror, retains every old revision and
// makes the previous active available as rollback on every storage encoding.
func TestConfigRollbackPublishesANewActiveVersion(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			source := backend.newStore(t, t.TempDir())
			seedConfigRollback(t, source)
			selected, stamp, err := rollbackConfigUnderTest(t, source, 2)
			require.NoError(t, err)
			require.Equal(t, FormatVersionStamp(historyStampB), selected.Stamp)
			require.NotEqual(t, selected.Stamp, stamp)
			active, err := ReadActiveConfig(source, "router.conf")
			require.NoError(t, err)
			require.Equal(t, "second revision", string(active))
			mirror, err := source.ReadFile("router.conf")
			require.NoError(t, err)
			require.Equal(t, active, mirror)
			activePointer, err := source.ReadKey(zefs.KeyConfigActive.Key("router.conf"))
			require.NoError(t, err)
			require.Equal(t, stamp+"\n", string(activePointer))
			previous, err := source.ReadKey(zefs.KeyConfigRollback.Key("router.conf"))
			require.NoError(t, err)
			require.Equal(t, FormatVersionStamp(historyStampC)+"\n", string(previous))
			versions, err := source.ListVersions("router.conf")
			require.NoError(t, err)
			require.Len(t, versions, 4)
			for _, version := range versions {
				_, err := source.ReadVersion("router.conf", version.Stamp)
				require.NoError(t, err)
			}
			unrelated, err := source.ReadFile("other.conf")
			require.NoError(t, err)
			require.Equal(t, "unrelated config", string(unrelated))
			_, _, candidate, err := ReadCandidateConfig(source, "router.conf")
			require.NoError(t, err)
			require.False(t, candidate)
		})
	}
}

// TestConfigRollbackRefusesInvalidIndices proves neither zero nor an absent
// revision can fall through to a different version or mutate saved state.
func TestConfigRollbackRefusesInvalidIndices(t *testing.T) {
	for _, revision := range []int{-1, 0, 4} {
		source := newMemoryStorage(t, t.TempDir())
		seedConfigRollback(t, source)
		before := configRenameSnapshot(t, source)
		_, _, err := rollbackConfigUnderTest(t, source, revision)
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.Equal(t, before, configRenameSnapshot(t, source))
	}
}

// TestConfigRollbackPreservesAStagedCandidate proves an indexed rollback
// cannot overwrite another operation's pending configuration.
func TestConfigRollbackPreservesAStagedCandidate(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRollback(t, source)
	_, err := WriteCandidateVersion(source, "router.conf", []byte("someone else's staged config"), historyStampC.AddDate(0, 0, 1))
	require.NoError(t, err)
	before := configRenameSnapshot(t, source)
	_, _, err = rollbackConfigUnderTest(t, source, 2)
	require.ErrorIs(t, err, ErrCandidateExists)
	require.Equal(t, before, configRenameSnapshot(t, source))
}

// TestConfigRollbackRefusesCorruptHistory proves selection follows the dated
// entry into its content object and verifies its hash before publication.
func TestConfigRollbackRefusesCorruptHistory(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRollback(t, source)
	require.NoError(t, source.WriteKey(objectKey(contentDigest([]byte("second revision"))), []byte("wrong object bytes")))
	before := configRenameSnapshot(t, source)
	_, _, err := rollbackConfigUnderTest(t, source, 2)
	require.ErrorIs(t, err, ErrHistoryObject)
	require.Equal(t, before, configRenameSnapshot(t, source))
}

// TestConfigRollbackRefusesARecordedConfirmWindow proves an owner-level call
// cannot bypass a persisted window that would revert its write on next boot.
func TestConfigRollbackRefusesARecordedConfirmWindow(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRollback(t, source)
	require.NoError(t, source.WriteKey(zefs.KeyConfigConfirmPending.Key("router.conf"), []byte("pending window record")))
	before := configRenameSnapshot(t, source)
	_, _, err := rollbackConfigUnderTest(t, source, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "confirmed")
	require.Equal(t, before, configRenameSnapshot(t, source))
}

// TestConfigRollbackPreservesAnUnfinishedFileCommit proves a recorded file
// publication cannot later replay over a rollback accepted under its old source.
func TestConfigRollbackPreservesAnUnfinishedFileCommit(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRollback(t, source)
	require.NoError(t, source.WriteKey(zefs.KeyConfigFileCommit.Key("router.conf"), []byte("pending file commit")))
	before := configRenameSnapshot(t, source)
	_, _, err := rollbackConfigUnderTest(t, source, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file commit")
	require.Equal(t, before, configRenameSnapshot(t, source))
}
