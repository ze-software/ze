package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestDropPlaintextPasswordEntries: the helper drops only entries whose
// final path segment starts with "plaintext-"; preserves order; returns
// a fresh slice (input is not mutated).
//
// VALIDATES: orphan-metadata cleanup after ApplyPasswordHashing.
// PREVENTS: commit metadata referencing a leaf the hash hook removed.
func TestDropPlaintextPasswordEntries(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "bgp local-as"},
		{Path: "system authentication user bob plaintext-password"},
		{Path: "system authentication user alice profile"},
	}
	original := make([]config.SessionEntry, len(in))
	copy(original, in)

	out := dropPlaintextPasswordEntries(in)

	assert.Len(t, out, 2)
	assert.Equal(t, "bgp local-as", out[0].Path)
	assert.Equal(t, "system authentication user alice profile", out[1].Path)

	// Input slice must not have been mutated (we returned a fresh allocation).
	for i, se := range in {
		assert.Equal(t, original[i].Path, se.Path,
			"input slice modified at index %d", i)
	}
}

// TestDropPlaintextPasswordEntriesEmpty: empty input returns empty output.
func TestDropPlaintextPasswordEntriesEmpty(t *testing.T) {
	out := dropPlaintextPasswordEntries(nil)
	assert.Empty(t, out)
}

// TestDropPlaintextPasswordEntriesNoMatches: no plaintext-* entries returns
// the same content (different backing array).
func TestDropPlaintextPasswordEntriesNoMatches(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "bgp local-as"},
		{Path: "system authentication user alice profile"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Len(t, out, 2)
	assert.Equal(t, in[0].Path, out[0].Path)
	assert.Equal(t, in[1].Path, out[1].Path)
}

// TestDropPlaintextPasswordEntriesAllMatch: all-plaintext input returns empty.
func TestDropPlaintextPasswordEntriesAllMatch(t *testing.T) {
	in := []config.SessionEntry{
		{Path: "system authentication user alice plaintext-password"},
		{Path: "system authentication user bob plaintext-password"},
	}
	out := dropPlaintextPasswordEntries(in)
	assert.Empty(t, out)
}

// TestCommitPathPasswordHashingUnchanged: the editor commit path is untouched
// by the load path gaining the same transform.
//
// VALIDATES: spec-netlab-integration AC-5 -- an existing config that commits a
// plaintext password through the editor still hashes it, still drops the
// ephemeral leaf, and still serializes a file carrying no plaintext.
//
// This one passes before the change as well as after, by design: AC-5 is the
// no-regression half of the spec. What it discriminates is the change that
// would break it -- the editor call site losing its ApplyPasswordHashing (it
// now takes two return values), or a second hashing implementation appearing
// on the load path and diverging from this one.
func TestCommitPathPasswordHashingUnchanged(t *testing.T) {
	seed := validBGPConfig + `
system {
	authentication {
		user lab {
			plaintext-password "labsecret"
		}
	}
}
`
	configPath := writeTestConfig(t, seed)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	written := string(data)

	assert.NotContains(t, written, "labsecret", "the committed file must never carry the plaintext")
	assert.NotContains(t, written, "plaintext-password", "the ephemeral leaf must not be serialized")

	tree, err := config.ParseTreeWithYANG(written, nil)
	require.NoError(t, err)
	lab := tree.GetContainer("system").GetContainer("authentication").GetList("user")["lab"]
	require.NotNil(t, lab)
	hash, ok := lab.Get("password")
	require.True(t, ok, "the committed file carries the canonical password leaf")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("labsecret")),
		"the committed hash validates the password the operator typed")
}

// TestCommitPathDropsEmptyPlaintextLeaf: an EMPTY ephemeral leaf is dropped too.
//
// VALIDATES: the deliberate change spec-netlab-integration made to the commit
// path. plaintext-password is ze:ephemeral, so it must never reach a serialized
// file whatever its value. Before this change hashPlaintextSibling returned early
// on an empty value without deleting the leaf, and serializeSetMetaChild writes
// every name present in the tree, so `plaintext-password ""` was committed to
// config.conf.
//
// PREVENTS: a serialized config carrying a write-only leaf. Ze re-reads that file
// at boot, and the leaf is meaningless there.
func TestCommitPathDropsEmptyPlaintextLeaf(t *testing.T) {
	seed := validBGPConfig + `
system {
	authentication {
		user lab {
			plaintext-password ""
		}
	}
}
`
	configPath := writeTestConfig(t, seed)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "plaintext-password",
		"an empty ephemeral leaf is dropped, not serialized")
}

// newBlobEditor builds a session editor backed by blob storage, mirroring the
// production web-only wiring (the only mode that exercised the commit deadlock).
// The filesystem seed is removed so reads provably come from the blob store.
func newBlobEditor(t *testing.T, seed string) (*Editor, storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(seed), 0o600))

	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, os.Remove(configPath))

	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))
	return ed, store, configPath
}

// TestCommitSessionFlushesOnBlob is the editor-layer regression for F1/F2:
// CommitSession used to self-deadlock on the zefs store mutex when it deleted
// the .edit file through e.store.Remove while holding the write guard. The
// deadlock prevented the guard from flushing, so the committed value never
// reached the blob. This test would hang (and fail via -timeout) before the
// fix that routes the .edit cleanup through the guard.
//
// This exercises the bare CommitSession() path with no reload notifier — the
// same path used by BOTH the web-only commit (no commit hook) and the appliance
// SSH session editor (session_factory.go wires no notifier, so cmdCommitSession
// takes the non-transactional CommitSession branch). The bug is not web-only.
//
// VALIDATES: blob-backed CommitSession completes and the value is persisted.
// PREVENTS: CommitSession self-deadlock + silent loss of the committed config.
func TestCommitSessionFlushesOnBlob(t *testing.T) {
	ed, store, configPath := newBlobEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	// A fresh ReadFile goes through the store's own lock; it would itself hang
	// if CommitSession had not released the guard, and would lack the value if
	// the guard never flushed.
	data, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "9.9.9.9",
		"committed router-id must be flushed to the blob store")
	assert.False(t, ed.Dirty(), "editor should be clean after commit")
}

// TestDiscardSessionPathOnBlob covers the same lock-discipline class as F1:
// DiscardSessionPath called e.store.Exists(changePath) while holding the write
// guard, and blobStorage.Exists re-takes the store's read lock, deadlocking
// against the held write lock. Filesystem-backed tests never caught it because
// filesystem Exists does not touch the AcquireLock mutex. The fix uses
// guard.Has. This test would hang before the fix.
//
// VALIDATES: blob-backed DiscardSessionPath completes without deadlock.
// PREVENTS: store access under a held guard recurring elsewhere in the editor.
func TestDiscardSessionPathOnBlob(t *testing.T) {
	ed, _, _ := newBlobEditor(t, validBGPConfig)

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	require.True(t, ed.Dirty(), "edit should mark the session dirty")

	require.NoError(t, ed.DiscardSessionPath(nil))
	assert.False(t, ed.Dirty(), "discard-all should clear the dirty flag")
}

// TestCommitStampsSchemaVersion verifies that CommitSession prepends the schema
// stamp to the written config file.
func TestCommitStampsSchemaVersion(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	session := NewEditSession("thomas", "local")
	ed.SetSession(session)

	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err)

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	content := string(data)
	assert.True(t, strings.HasPrefix(content, "# ze-schema: "),
		"committed config should start with schema stamp, got first line: %q",
		strings.SplitN(content, "\n", 2)[0])

	scanned := config.ScanStampRelease(data)
	assert.NotEmpty(t, scanned, "stamp release should be present")

	// AC-7: show config (WorkingContent) must NOT include the stamp.
	showOutput := ed.WorkingContent()
	assert.False(t, strings.HasPrefix(showOutput, "# ze-schema:"),
		"show config output should not contain schema stamp")

	// AC-9: re-commit to create a backup, then verify the backup has the stamp.
	err = ed.SetValue([]string{"bgp"}, "router-id", "8.8.8.8")
	require.NoError(t, err)
	_, err = ed.CommitSession()
	require.NoError(t, err)

	backups, err := ed.ListBackups()
	require.NoError(t, err)
	if assert.NotEmpty(t, backups, "should have at least one backup") {
		backupData, readErr := os.ReadFile(backups[0].Path)
		require.NoError(t, readErr)
		backupRelease := config.ScanStampRelease(backupData)
		assert.NotEmpty(t, backupRelease,
			"backup should contain schema stamp from committed config")
	}
}

// insertConflictConfig holds a leaf-list (bgp filter import) used by the
// ordered-insert conflict tests below.
const insertConflictConfig = `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  filter {
    import [ alpha bravo ];
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`

// newInsertConflictEditor seeds insertConflictConfig, opens a session editor,
// and records an ordered insert of "charlie" after "alpha".
func newInsertConflictEditor(t *testing.T) (*Editor, string) {
	t.Helper()
	configPath := writeTestConfig(t, insertConflictConfig)

	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	ed.SetSession(NewEditSession("thomas", "local"))

	require.NoError(t, ed.InsertLeafListValue(
		[]string{"bgp", "filter"}, "import", "charlie", config.InsertAfter, "alpha"))
	return ed, configPath
}

// TestCommitInsertRefRemovedIsConflict: an ordered insert whose before/after
// reference member was removed by a concurrent commit must surface as a stale
// conflict the user can resolve, not abort the commit with a raw apply error.
//
// VALIDATES: missing insert reference -> CommitResult.Conflicts (ConflictStale).
// PREVENTS: "apply structural ops: ... not found" failing commit without
// identifying the stale edit.
func TestCommitInsertRefRemovedIsConflict(t *testing.T) {
	ed, configPath := newInsertConflictEditor(t)

	// Concurrent commit removes the reference member alpha.
	modified := strings.Replace(insertConflictConfig,
		"import [ alpha bravo ];", "import [ bravo ];", 1)
	require.NoError(t, os.WriteFile(configPath, []byte(modified), 0o600))

	result, err := ed.CommitSession()
	require.NoError(t, err,
		"missing insert reference must be a conflict, not a commit error")
	require.NotEmpty(t, result.Conflicts, "stale conflict expected")
	assert.Equal(t, 0, result.Applied, "no changes should be applied")

	c := result.Conflicts[0]
	assert.Equal(t, ConflictStale, c.Type)
	assert.Contains(t, c.Path, "bgp filter import")
	assert.Contains(t, c.MyValue, "charlie")
	assert.Equal(t, "alpha", c.PreviousValue,
		"conflict should name the removed reference member")
}

// TestCommitCandidateInsertRefRemovedIsConflict: same protection on the
// transactional CommitSessionCandidate path.
//
// VALIDATES: candidate commit surfaces missing insert reference as a conflict.
// PREVENTS: the full-daemon commit path keeping the raw-error behavior.
func TestCommitCandidateInsertRefRemovedIsConflict(t *testing.T) {
	ed, configPath := newInsertConflictEditor(t)

	modified := strings.Replace(insertConflictConfig,
		"import [ alpha bravo ];", "import [ bravo ];", 1)
	require.NoError(t, os.WriteFile(configPath, []byte(modified), 0o600))

	result, _, err := ed.CommitSessionCandidate(time.Now())
	require.NoError(t, err,
		"missing insert reference must be a conflict, not a commit error")
	require.NotEmpty(t, result.Conflicts, "stale conflict expected")
	assert.Equal(t, ConflictStale, result.Conflicts[0].Type)
}

// TestCommitInsertMemberAlreadyPresentNoConflict: when the inserted member
// already landed in the committed config (idempotent replay or identical
// concurrent edit), commit proceeds without conflict.
//
// VALIDATES: present member short-circuits the missing-ref conflict check.
// PREVENTS: false conflicts on idempotent member inserts.
func TestCommitInsertMemberAlreadyPresentNoConflict(t *testing.T) {
	ed, configPath := newInsertConflictEditor(t)

	// Concurrent commit removed alpha but already contains charlie.
	modified := strings.Replace(insertConflictConfig,
		"import [ alpha bravo ];", "import [ charlie bravo ];", 1)
	require.NoError(t, os.WriteFile(configPath, []byte(modified), 0o600))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	assert.Empty(t, result.Conflicts,
		"member already present must not conflict")
}
