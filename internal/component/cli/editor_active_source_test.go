// Design: docs/architecture/storage-backends.md -- native editor committed source authority.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

const activeEditorConfig = "bgp { router-id 1.2.3.4; session { asn { local 65000; } } }\n"

func activeEditorStore(t *testing.T) storage.Storage {
	t.Helper()
	source := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, source.Close()) })
	_, err := storage.RestoreConfig(source, "router.conf", []byte(activeEditorConfig))
	require.NoError(t, err)
	return source
}

// TestStoredEditorReadsTheActiveVersion preserves raw mirror access while
// stored editors follow the verified pointer, including absence and failure.
func TestStoredEditorReadsTheActiveVersion(t *testing.T) {
	for _, state := range []string{"pointer-only", "stale-mirror", "unresolved", "corrupt-object"} {
		t.Run(state, func(t *testing.T) {
			source := activeEditorStore(t)
			stale := []byte(strings.ReplaceAll(activeEditorConfig, "1.2.3.4", "192.0.2.199"))
			require.NoError(t, source.WriteFile("router.conf", stale, 0o600))
			switch state {
			case "pointer-only":
				require.NoError(t, source.Remove("router.conf"))
			case "unresolved":
				require.NoError(t, source.WriteKey(zefs.KeyConfigActive.Key("router.conf"), []byte("20000101-000000.000\n")))
			case "corrupt-object":
				keys, err := source.ListKeys("object/")
				require.NoError(t, err)
				require.Len(t, keys, 1)
				require.NoError(t, source.WriteKey(keys[0], []byte("different content under the same digest")))
			}
			ed, err := NewEditorWithStorage(source, "router.conf")
			if state == "unresolved" || state == "corrupt-object" {
				require.Error(t, err)
				require.Nil(t, ed)
			} else {
				require.NoError(t, err)
				require.Equal(t, activeEditorConfig, ed.OriginalContent())
				require.NoError(t, ed.Close())
			}
			raw, rawErr := source.ReadFile("router.conf")
			if state == "pointer-only" {
				require.ErrorIs(t, rawErr, os.ErrNotExist)
			} else {
				require.NoError(t, rawErr)
				require.Equal(t, stale, raw, "raw storage access must retain its original mirror contract")
			}
		})
	}
}

// TestStoredSessionGuardReadsTheActiveVersion changes the mirror after the
// editor opens, proving resume and candidate commit re-read the same authority.
func TestStoredSessionGuardReadsTheActiveVersion(t *testing.T) {
	for _, operation := range []string{"resume", "commit", "discard"} {
		t.Run(operation, func(t *testing.T) {
			source := activeEditorStore(t)
			ed, err := NewIsolatedEditorWithStorage(source, "router.conf")
			require.NoError(t, err)
			ed.SetSession(NewEditSession("alice", "local"))
			ed.SetOfflinePromotion()
			stale := strings.ReplaceAll(activeEditorConfig, "1.2.3.4", "192.0.2.199")
			require.NoError(t, source.WriteFile("router.conf", []byte(stale), 0o600))
			require.NoError(t, ed.ResumeSession())
			require.Equal(t, activeEditorConfig, ed.OriginalContent())
			if operation == "resume" {
				return
			}
			require.NoError(t, ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65001"))
			if operation == "discard" {
				require.NoError(t, ed.DiscardSessionPath(nil))
				require.Contains(t, ed.WorkingContent(), "1.2.3.4")
				require.NotContains(t, ed.WorkingContent(), "192.0.2.199")
				return
			}
			result, _, err := ed.CommitSessionCandidate(time.Now())
			require.NoError(t, err)
			require.NotZero(t, result.Applied)
			require.NoError(t, ed.NotifyReload())
			active, err := storage.ReadActiveConfig(source, "router.conf")
			require.NoError(t, err)
			require.Contains(t, string(active), "1.2.3.4")
			require.Contains(t, string(active), "65001")
			require.NotContains(t, string(active), "192.0.2.199")
		})
	}
}

// TestLooseEditorKeepsItsExplicitFileAuthority proves a stored pointer does
// not redirect an editor deliberately opened on a caller's separate file.
func TestLooseEditorKeepsItsExplicitFileAuthority(t *testing.T) {
	source := activeEditorStore(t)
	path := filepath.Join(t.TempDir(), "router.conf")
	content := strings.ReplaceAll(activeEditorConfig, "1.2.3.4", "192.0.2.50")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	ed, err := NewLooseFileEditor(source, path)
	require.NoError(t, err)
	require.Equal(t, content, ed.OriginalContent())
	require.NoError(t, ed.Close())
}

// TestStoredSessionRefusesChangedActivePointer proves a pointer or object
// failure arriving after construction cannot be hidden by a readable mirror
// or consume any saved session state during its guarded resume.
func TestStoredSessionRefusesChangedActivePointer(t *testing.T) {
	for _, state := range []string{"unresolved", "corrupt-object"} {
		t.Run(state, func(t *testing.T) {
			source := activeEditorStore(t)
			ed, err := NewIsolatedEditorWithStorage(source, "router.conf")
			require.NoError(t, err)
			ed.SetSession(NewEditSession("alice", "local"))
			if state == "unresolved" {
				require.NoError(t, source.WriteKey(zefs.KeyConfigActive.Key("router.conf"), []byte("20000101-000000.000\n")))
			} else {
				keys, err := source.ListKeys("object/")
				require.NoError(t, err)
				require.Len(t, keys, 1)
				require.NoError(t, source.WriteKey(keys[0], []byte("different content under the same digest")))
			}
			before, err := source.ListKeys("")
			require.NoError(t, err)
			require.Error(t, ed.ResumeSession())
			after, err := source.ListKeys("")
			require.NoError(t, err)
			require.Equal(t, before, after, "refused resume must not create ownership or draft records")
			require.Equal(t, activeEditorConfig, ed.OriginalContent())
		})
	}
}

// TestStoredEditorSaveRequiresTheOwningCommit proves a versioned source cannot
// report success after writing only its mirror. An explicitly installed writer
// keeps native publication and pending-candidate policy with the store owner.
func TestStoredEditorSaveRequiresTheOwningCommit(t *testing.T) {
	for _, operation := range []string{"missing owner", "explicit owner", "pending candidate"} {
		t.Run(operation, func(t *testing.T) {
			source := activeEditorStore(t)
			ed, err := NewEditorWithStorage(source, "router.conf")
			require.NoError(t, err)
			if operation != "missing owner" {
				ed.SetCommitWriter(func(expected, content []byte) error {
					require.Equal(t, []byte(activeEditorConfig), expected)
					_, err := storage.RestoreConfig(source, "router.conf", content)
					return err
				})
			}
			if operation == "pending candidate" {
				_, err := storage.WriteCandidateVersion(source, "router.conf", []byte(activeEditorConfig), time.Now())
				require.NoError(t, err)
			}
			require.NoError(t, ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65001"))
			before := storedEditorKeys(t, source)
			_, saveErr := ed.Save()
			if operation == "explicit owner" {
				require.NoError(t, saveErr)
				active, err := storage.ReadActiveConfig(source, "router.conf")
				require.NoError(t, err)
				require.Contains(t, string(active), "65001")
				return
			}
			require.Error(t, saveErr)
			if operation == "pending candidate" {
				require.ErrorIs(t, saveErr, storage.ErrCandidateExists)
			}
			require.Equal(t, before, storedEditorKeys(t, source), "a refused save must not create history or rewrite its mirror")
			require.True(t, ed.Dirty())
		})
	}
}

func storedEditorKeys(t *testing.T, source storage.Storage) map[string][]byte {
	t.Helper()
	keys, err := source.ListKeys("")
	require.NoError(t, err)
	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		result[key], err = source.ReadKey(key)
		require.NoError(t, err)
	}
	return result
}
