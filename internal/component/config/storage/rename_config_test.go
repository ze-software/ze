// Design: docs/architecture/storage-backends.md -- complete stored-config rename.

package storage

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

// renameConfigUnderTest discovers the owner operation without needing a new
// symbol to compile. Its absence is the behavioral RED before implementation.
func renameConfigUnderTest(t *testing.T, source Storage, oldName, newName string) error {
	t.Helper()
	rename, found := source.(interface{ RenameConfig(string, string) error })
	require.True(t, found, "the storage owner exposes no complete configuration rename")
	return rename.RenameConfig(oldName, newName)
}

func configRenameSnapshot(t *testing.T, source Storage) map[string][]byte {
	t.Helper()
	keys, err := source.ListKeys("")
	require.NoError(t, err)
	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		data, err := source.ReadKey(key)
		require.NoError(t, err)
		result[key] = data
	}
	return result
}

func seedConfigRename(t *testing.T, source Storage) {
	t.Helper()
	// EnsureActiveVersion adopts source bytes into pointer history; the
	// file/active mirror is a separate publication performed by its caller.
	require.NoError(t, source.WriteFile("router.conf", []byte("active"), 0o600))
	stamp, _, err := EnsureActiveVersion(source, "router.conf", []byte("active"), historyStampA)
	require.NoError(t, err)
	_, err = WriteCandidateVersion(source, "router.conf", []byte("candidate"), historyStampB)
	require.NoError(t, err)
	for key, data := range map[string]string{
		zefs.KeyConfigRollback.Key("router.conf"): stamp,
		zefs.KeyConfigRecovery.Key("router.conf"): stamp,
		"meta/config/router.conf/audit/retained":  "past config event",
		"file/active/router.conf.draft":           "saved draft",
		"file/active/router.conf.change.alice":    "alice changes",
		"file/draft/router.conf":                  "draft namespace",
		"file/candidate/router.conf":              "candidate mirror",
		"file/template/router.conf":               "template",
		"file/active/router.conf.backup":          "another config",
		"meta/history/alice/config":               "global command history",
		"meta/audit/retained":                     "global audit record",
	} {
		require.NoError(t, source.WriteKey(key, []byte(data)))
	}
}

// TestConfigRenamePreservesTheWholeConfiguration proves every encoding moves
// pointers, dated entries, mirrors and saved editor work together, while shared
// objects and unrelated history retain exactly their previous bytes.
func TestConfigRenamePreservesTheWholeConfiguration(t *testing.T) {
	for _, backend := range pointerTestStores() {
		t.Run(backend.name, func(t *testing.T) {
			source := backend.newStore(t, t.TempDir())
			seedConfigRename(t, source)
			before := configRenameSnapshot(t, source)
			mirrorMeta, err := source.Stat("router.conf")
			require.NoError(t, err)
			require.NoError(t, renameConfigUnderTest(t, source, "router.conf", "renamed.conf"))
			want := make(map[string][]byte, len(before))
			for key, data := range before {
				switch key {
				case "file/active/router.conf", "file/active/router.conf.draft", "file/active/router.conf.change.alice",
					"file/draft/router.conf", "file/candidate/router.conf", "file/template/router.conf",
					"file/20260926-100000.000/router.conf", "file/20260926-110000.000/router.conf",
					"meta/config/router.conf/active", "meta/config/router.conf/candidate", "meta/config/router.conf/rollback",
					"meta/config/router.conf/recovery", "meta/config/router.conf/audit/retained":
					want[strings.Replace(key, "router.conf", "renamed.conf", 1)] = data
				default:
					want[key] = data
				}
			}
			require.Equal(t, want, configRenameSnapshot(t, source))
			active, err := ReadActiveConfig(source, "renamed.conf")
			require.NoError(t, err)
			require.Equal(t, "active", string(active))
			candidate, _, present, err := ReadCandidateConfig(source, "renamed.conf")
			require.NoError(t, err)
			require.True(t, present)
			require.Equal(t, "candidate", string(candidate))
			versions, err := source.ListVersions("renamed.conf")
			require.NoError(t, err)
			require.Len(t, versions, 2)
			for _, version := range versions {
				_, err := source.ReadVersion("renamed.conf", version.Stamp)
				require.NoError(t, err)
			}
			gotMeta, err := source.Stat("renamed.conf")
			require.NoError(t, err)
			require.Equal(t, mirrorMeta, gotMeta)
			findings, err := CheckHistory(source)
			require.NoError(t, err)
			require.Empty(t, findings, "rename left dangling history or orphan objects")
		})
	}
}

// TestConfigRenameRefusesDestinationOwnedState proves a missing target mirror
// does not permit overwriting its pointers, history or saved editor work.
func TestConfigRenameRefusesDestinationOwnedState(t *testing.T) {
	for _, collision := range []string{
		"file/active/renamed.conf", "file/active/renamed.conf.draft", "file/active/renamed.conf.change.alice",
		"meta/config/renamed.conf/active", "file/20260926-100000.000/renamed.conf",
	} {
		t.Run(collision, func(t *testing.T) {
			source := newMemoryStorage(t, t.TempDir())
			seedConfigRename(t, source)
			require.NoError(t, source.WriteKey(collision, []byte("must survive")))
			before := configRenameSnapshot(t, source)
			require.ErrorIs(t, renameConfigUnderTest(t, source, "router.conf", "renamed.conf"), fs.ErrExist)
			require.Equal(t, before, configRenameSnapshot(t, source))
		})
	}
}

// TestConfigRenamePreservesIndependentlyNamedEditorLikeConfigs proves suffixes
// cannot take a real versioned sibling config into somebody else's rename.
func TestConfigRenamePreservesIndependentlyNamedEditorLikeConfigs(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRename(t, source)
	_, _, err := EnsureActiveVersion(source, "router.conf.draft", []byte("independent config"), historyStampC)
	require.NoError(t, err)
	require.NoError(t, renameConfigUnderTest(t, source, "router.conf", "renamed.conf"))
	data, err := ReadActiveConfig(source, "router.conf.draft")
	require.NoError(t, err)
	require.Equal(t, "independent config", string(data))
	_, err = source.ReadFile("renamed.conf.draft")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// TestConfigRenamePreservesAnIndependentPendingEditorLikeConfig proves a
// staged identity is independent even before it has an active pointer.
func TestConfigRenamePreservesAnIndependentPendingEditorLikeConfig(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRename(t, source)
	stamp, err := WriteCandidateVersion(source, "router.conf.draft", []byte("independent pending config"), historyStampC)
	require.NoError(t, err)
	require.NoError(t, renameConfigUnderTest(t, source, "router.conf", "renamed.conf"))
	data, gotStamp, present, err := ReadCandidateConfig(source, "router.conf.draft")
	require.NoError(t, err)
	require.True(t, present)
	require.Equal(t, stamp, gotStamp)
	require.Equal(t, "independent pending config", string(data))
	_, err = source.ReadFile("renamed.conf.draft")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

var errConfigRenameInjected = errors.New("injected config rename publication failure")

// configRenameFault injects one post-publication failure under the owner guard.
// It is used serially, with no concurrent writer; rollback can then run normally.
type configRenameFault struct {
	keyEncoding
	operation string
	key       string
	armed     bool
}

func (f *configRenameFault) WriteFile(key string, data []byte, mode fs.FileMode) error {
	if err := f.keyEncoding.WriteFile(key, data, mode); err != nil {
		return err
	}
	if f.armed {
		if f.operation == "write" {
			if f.key == key {
				f.armed = false
				return errConfigRenameInjected
			}
		}
	}
	return nil
}

func (f *configRenameFault) Remove(key string) error {
	if err := f.keyEncoding.Remove(key); err != nil {
		return err
	}
	if f.armed {
		if f.operation == "remove" {
			if f.key == key {
				f.armed = false
				return errConfigRenameInjected
			}
		}
	}
	return nil
}

// TestConfigRenameRestoresTheSourceAfterPublicationFailure proves failures
// after a destination write or source unlink restore all original key bytes.
func TestConfigRenameRestoresTheSourceAfterPublicationFailure(t *testing.T) {
	for _, operation := range []string{"write", "remove"} {
		t.Run(operation, func(t *testing.T) {
			source := newMemoryStorage(t, t.TempDir())
			seedConfigRename(t, source)
			before := configRenameSnapshot(t, source)
			owned, ok := ownedStore(source)
			require.True(t, ok)
			key := "file/active/renamed.conf.draft"
			if operation == "remove" {
				key = "file/active/router.conf.draft"
			}
			owned.keys = &configRenameFault{keyEncoding: owned.keys, operation: operation, key: key, armed: true}
			require.ErrorIs(t, renameConfigUnderTest(t, source, "router.conf", "renamed.conf"), errConfigRenameInjected)
			require.Equal(t, before, configRenameSnapshot(t, source))
		})
	}
}

// TestConfigRenameRefusesUnfinishedIdentityTransactions proves a pending file
// commit or confirmed window cannot move under an intent's embedded old name.
func TestConfigRenameRefusesUnfinishedIdentityTransactions(t *testing.T) {
	for _, key := range []string{zefs.KeyConfigFileCommit.Key("router.conf"), zefs.KeyConfigConfirmPending.Key("router.conf")} {
		t.Run(filepath.Base(key), func(t *testing.T) {
			source := newMemoryStorage(t, t.TempDir())
			seedConfigRename(t, source)
			require.NoError(t, source.WriteKey(key, []byte("uncompleted transaction")))
			before := configRenameSnapshot(t, source)
			require.Error(t, renameConfigUnderTest(t, source, "router.conf", "renamed.conf"))
			require.Equal(t, before, configRenameSnapshot(t, source))
		})
	}
}

var errConfigRenameRestore = errors.New("injected source restoration failure")

// configRenameRollbackFault fails after one source removal, then before that
// source key can be restored. It is used under one guard with no other writer.
type configRenameRollbackFault struct {
	keyEncoding
	removed       bool
	restoreFailed bool
}

func (f *configRenameRollbackFault) Remove(key string) error {
	if err := f.keyEncoding.Remove(key); err != nil {
		return err
	}
	if key == "file/active/router.conf.draft" {
		if !f.removed {
			f.removed = true
			return errConfigRenameInjected
		}
	}
	return nil
}

func (f *configRenameRollbackFault) WriteFile(key string, data []byte, mode fs.FileMode) error {
	if f.removed {
		if key == "file/active/router.conf.draft" {
			if !f.restoreFailed {
				f.restoreFailed = true
				return errConfigRenameRestore
			}
		}
	}
	return f.keyEncoding.WriteFile(key, data, mode)
}

// TestConfigRenameKeepsTheCompleteDestinationWhenRestorationFails proves a
// failed compensation never destroys the only complete copy of saved work.
func TestConfigRenameKeepsTheCompleteDestinationWhenRestorationFails(t *testing.T) {
	source := newMemoryStorage(t, t.TempDir())
	seedConfigRename(t, source)
	before := configRenameSnapshot(t, source)
	owned, ok := ownedStore(source)
	require.True(t, ok)
	owned.keys = &configRenameRollbackFault{keyEncoding: owned.keys}
	err := renameConfigUnderTest(t, source, "router.conf", "renamed.conf")
	require.ErrorIs(t, err, errConfigRenameInjected)
	require.ErrorIs(t, err, errConfigRenameRestore)
	for _, key := range []string{
		"file/active/router.conf", "file/active/router.conf.draft", "file/active/router.conf.change.alice",
		"file/draft/router.conf", "file/candidate/router.conf", "file/template/router.conf",
		"file/20260926-100000.000/router.conf", "file/20260926-110000.000/router.conf",
		"meta/config/router.conf/active", "meta/config/router.conf/candidate", "meta/config/router.conf/rollback",
		"meta/config/router.conf/recovery", "meta/config/router.conf/audit/retained",
	} {
		retained, err := source.ReadKey(strings.Replace(key, "router.conf", "renamed.conf", 1))
		require.NoError(t, err, key)
		require.Equal(t, before[key], retained, "the only complete destination lost %s", key)
	}
	active, err := ReadActiveConfig(source, "renamed.conf")
	require.NoError(t, err)
	require.Equal(t, "active", string(active))
}
