// Design: docs/architecture/web-interface.md -- accepted backup folder generations and retained roots.
package artifact

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

func directoryLiveEnvironment(t *testing.T, value string) {
	t.Helper()
	t.Cleanup(env.ResetCache)
	t.Setenv("ze.backup.directory", value)
	t.Setenv("ZE_BACKUP_DIRECTORY", value)
	env.ResetCache()
}

// TestBackupDirectoryAcceptanceOrder makes late and rejected preparations
// compete. Only accepted generations order selection; a newer rejection does
// not prevent a valid older acceptance, and stale acceptance cannot overwrite it.
func TestBackupDirectoryAcceptanceOrder(t *testing.T) {
	for _, newerAccepted := range []bool{false, true} {
		t.Run(fmt.Sprint(newerAccepted), func(t *testing.T) {
			directoryLiveEnvironment(t, "")
			owner, _ := ownerFixture(t)
			first := filepath.Join(t.TempDir(), "first")
			second := filepath.Join(t.TempDir(), "second")
			older, err := owner.PrepareDirectory(t.Context(), first, 1)
			require.NoError(t, err)
			newer, err := owner.PrepareDirectory(t.Context(), second, 2)
			require.NoError(t, err)
			require.NoError(t, newer.Complete(t.Context(), newerAccepted))
			require.NoError(t, older.Complete(t.Context(), true))
			upload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
			require.NoError(t, err)
			want := first
			if newerAccepted {
				want = second
			}
			require.Equal(t, want, filepath.Dir(owner.root))
			require.NoError(t, owner.Abort(t.Context(), "alice", upload.ID))
		})
	}
}

// TestBackupDirectoryProbeCleanupDebt fences further probes after failed
// unlink and removes that precise owned file on Close, preserving its parent.
func TestBackupDirectoryProbeCleanupDebt(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, _ := ownerFixture(t)
	parent := t.TempDir()
	owner.removeAll = func(path string) error {
		if strings.HasPrefix(filepath.Base(path), ".ze-backup-directory-check-") {
			return errors.New("injected permission-probe removal failure")
		}
		return os.RemoveAll(path)
	}
	_, err := owner.PrepareDirectory(t.Context(), parent, 1)
	require.Error(t, err)
	debt := owner.cleanupDebt
	require.Equal(t, parent, filepath.Dir(debt))
	require.Equal(t, int64(1), owner.debtBytes)
	_, err = owner.PrepareDirectory(t.Context(), parent, 2)
	require.Error(t, err)
	require.Equal(t, debt, owner.cleanupDebt)
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	require.Len(t, entries, 1, "a retry cannot leak or overwrite another probe")
	owner.removeAll = os.RemoveAll
	require.NoError(t, owner.Close())
	entries, err = os.ReadDir(parent)
	require.NoError(t, err)
	require.Empty(t, entries)
}

// TestBackupDirectoryRepeatedSwitchesBoundRoots exercises actual allocation
// between commits, so retaining one empty root per directory change goes red.
func TestBackupDirectoryRepeatedSwitchesBoundRoots(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, _ := ownerFixture(t)
	base := t.TempDir()
	for generation := uint64(1); generation <= 24; generation++ {
		parent := filepath.Join(base, fmt.Sprintf("parent-%d", generation))
		prepared, err := owner.PrepareDirectory(t.Context(), parent, generation)
		require.NoError(t, err)
		require.NoError(t, prepared.Complete(t.Context(), true))
		upload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
		require.NoError(t, err)
		require.NoError(t, owner.Abort(t.Context(), "alice", upload.ID))
		owner.mu.Lock()
		roots := len(owner.roots)
		owner.mu.Unlock()
		require.Equal(t, 1, roots)
	}
	require.NoError(t, owner.Close())
	parents, err := os.ReadDir(base)
	require.NoError(t, err)
	for _, parent := range parents {
		entries, err := os.ReadDir(filepath.Join(base, parent.Name()))
		require.NoError(t, err)
		require.Empty(t, entries, "only the user-selected parent remains after close")
	}
}

// TestBackupDirectoryCommitRespectsOSOverride keeps an actual inherited folder
// authoritative without creating the lower-priority candidate folder.
func TestBackupDirectoryCommitRespectsOSOverride(t *testing.T) {
	parent := t.TempDir()
	directoryLiveEnvironment(t, parent)
	owner, _ := ownerFixture(t)
	candidate := filepath.Join(t.TempDir(), "lower-priority")
	prepared, err := owner.PrepareDirectory(t.Context(), candidate, 1)
	require.NoError(t, err)
	require.NoError(t, prepared.Complete(t.Context(), true))
	upload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
	require.NoError(t, err)
	require.Equal(t, parent, filepath.Dir(owner.root))
	_, err = os.Stat(candidate)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, owner.Abort(t.Context(), "alice", upload.ID))
}

// TestBackupDirectoryRejectsOwnedRootParent pins both literal and linked
// descendants while retaining the old workspace and a useful sibling switch.
func TestBackupDirectoryRejectsOwnedRootParent(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, content := ownerFixture(t)
	workspace, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	oldRoot := owner.root
	link := filepath.Join(t.TempDir(), "workspace-link")
	require.NoError(t, os.Symlink(oldRoot, link))
	for _, parent := range []string{oldRoot, filepath.Join(oldRoot, "nested"), filepath.Join(link, "nested")} {
		_, err := owner.PrepareDirectory(t.Context(), parent, 1)
		require.Error(t, err)
		require.Equal(t, oldRoot, owner.root)
		kept, err := owner.Inspect(t.Context(), "alice", workspace.ID)
		require.NoError(t, err)
		require.Equal(t, workspace.ID, kept.ID)
	}
	sibling := filepath.Join(t.TempDir(), "safe-backups")
	prepared, err := owner.PrepareDirectory(t.Context(), sibling, 2)
	require.NoError(t, err)
	require.NoError(t, prepared.Complete(t.Context(), true))
	upload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
	require.NoError(t, err)
	require.Equal(t, sibling, filepath.Dir(owner.root))
	_, err = os.Stat(oldRoot)
	require.NoError(t, err, "occupied old roots must not be pruned on acceptance")
	require.NoError(t, owner.Abort(t.Context(), "alice", upload.ID))
}

// TestBackupDirectoryPreparationChecksPermissions observes the real probe
// immediately before its removal, preserving user-owned parent permissions and
// contents and leaving no probe or speculative private root after preparation.
func TestBackupDirectoryPreparationChecksPermissions(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			directoryLiveEnvironment(t, "")
			owner, _ := ownerFixture(t)
			oldParent := owner.parentTemp
			parent := filepath.Join(t.TempDir(), "backup-parent")
			mode := os.FileMode(0o700)
			marker := filepath.Join(parent, "keep.txt")
			if existing {
				mode = 0o750
				require.NoError(t, os.Mkdir(parent, mode))
				require.NoError(t, os.WriteFile(marker, []byte("keep parent contents"), 0o600))
			}
			seen := false
			owner.removeAll = func(path string) error {
				if strings.HasPrefix(filepath.Base(path), ".ze-backup-directory-check-") {
					seen = true
					info, err := os.Stat(path)
					if err != nil {
						return err
					}
					require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
					content, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					require.Equal(t, []byte{0}, content, "the permission probe must write real bytes")
				}
				return os.RemoveAll(path)
			}
			prepared, err := owner.PrepareDirectory(t.Context(), parent, 1)
			require.NoError(t, err)
			require.True(t, seen)
			info, err := os.Stat(parent)
			require.NoError(t, err)
			require.Equal(t, mode, info.Mode().Perm())
			entries, err := os.ReadDir(parent)
			require.NoError(t, err)
			if existing {
				require.Len(t, entries, 1)
				kept, err := os.ReadFile(marker)
				require.NoError(t, err)
				require.Equal(t, "keep parent contents", string(kept))
			} else {
				require.Empty(t, entries)
			}
			require.Empty(t, owner.root)
			require.NoError(t, prepared.Complete(t.Context(), false))
			require.Equal(t, oldParent, owner.parentTemp)
		})
	}
}

// TestBackupDirectorySwitchRetainsUnfinishedUpload completes real artifact
// bytes in the retired root while new allocations use the accepted parent.
func TestBackupDirectorySwitchRetainsUnfinishedUpload(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, content := ownerFixture(t)
	upload, err := owner.Begin(t.Context(), "alice", "in-progress.zefs", int64(len(content)), false)
	require.NoError(t, err)
	half := len(content) / 2
	_, err = owner.Append(t.Context(), "alice", upload.ID, 0, content[:half])
	require.NoError(t, err)
	oldRoot := owner.root
	oldPath := owner.uploads[upload.ID].path
	next := filepath.Join(t.TempDir(), "next-backups")
	prepared, err := owner.PrepareDirectory(t.Context(), next, 1)
	require.NoError(t, err)
	require.NoError(t, prepared.Complete(t.Context(), true))
	partial, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	require.Equal(t, content[:half], partial)
	_, err = owner.Append(t.Context(), "alice", upload.ID, int64(half), content[half:])
	require.NoError(t, err)
	workspace, _, err := owner.Finish(t.Context(), "alice", upload.ID)
	require.NoError(t, err)
	completed, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	require.Equal(t, content, completed)
	_, err = owner.Inspect(t.Context(), "alice", workspace.ID)
	require.NoError(t, err)
	newUpload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
	require.NoError(t, err)
	require.Equal(t, next, filepath.Dir(owner.root))
	require.NoError(t, owner.Remove(t.Context(), "alice", workspace.ID))
	require.NoError(t, owner.Abort(t.Context(), "alice", newUpload.ID))
	_, err = os.Stat(oldRoot)
	require.ErrorIs(t, err, os.ErrNotExist, "retired upload roots must be removed once no transfer or workspace uses them")
}

// TestBackupDirectorySwitchRetainsStandaloneSnapshot keeps a source-only old
// root readable until its immutable system transfer is explicitly aborted.
func TestBackupDirectorySwitchRetainsStandaloneSnapshot(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, store := systemSnapshotFixture(t)
	download, err := owner.SnapshotSystem(t.Context(), "alice", systemSnapshotProducer(t.Context(), store))
	require.NoError(t, err)
	oldRoot := owner.root
	path := owner.sources[download.ID].path
	_, before, err := owner.Read(t.Context(), "alice", download.ID, 0)
	require.NoError(t, err)
	prepared, err := owner.PrepareDirectory(t.Context(), filepath.Join(t.TempDir(), "next-backups"), 1)
	require.NoError(t, err)
	require.NoError(t, prepared.Complete(t.Context(), true))
	_, after, err := owner.Read(t.Context(), "alice", download.ID, 0)
	require.NoError(t, err)
	require.NotEmpty(t, after)
	require.Equal(t, before, after)
	retained, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, retained)
	require.NoError(t, owner.Abort(t.Context(), "alice", download.ID))
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
	upload, err := owner.Begin(t.Context(), "alice", "next.zefs", -1, false)
	require.NoError(t, err)
	_, err = os.Stat(oldRoot)
	require.ErrorIs(t, err, os.ErrNotExist, "aborted source-only roots must be pruned before new allocation")
	require.NoError(t, owner.Abort(t.Context(), "alice", upload.ID))
}

// TestBackupDirectoryRetiredCleanupFencesAllocation retains failed cleanup as
// bounded debt after acceptance, then admits useful work only after repair.
func TestBackupDirectoryRetiredCleanupFencesAllocation(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, content := ownerFixture(t)
	upload, err := owner.Begin(t.Context(), "alice", "old.zefs", -1, false)
	require.NoError(t, err)
	oldRoot := owner.root
	require.NoError(t, owner.Abort(t.Context(), "alice", upload.ID))
	next := filepath.Join(t.TempDir(), "next-backups")
	prepared, err := owner.PrepareDirectory(t.Context(), next, 1)
	require.NoError(t, err)
	owner.removeAll = func(path string) error {
		if path == oldRoot {
			return errors.New("injected retired-root cleanup failure")
		}
		return os.RemoveAll(path)
	}
	require.Error(t, prepared.Complete(t.Context(), true))
	_, err = os.Stat(oldRoot)
	require.NoError(t, err)
	_, err = owner.Begin(t.Context(), "alice", "blocked.zefs", -1, false)
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 503, refusal.Status)
	entries, err := os.ReadDir(next)
	require.NoError(t, err)
	require.Empty(t, entries, "cleanup refusal must create no new private root")
	owner.removeAll = os.RemoveAll
	workspace, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	require.Equal(t, next, filepath.Dir(owner.root))
	_, err = os.Stat(oldRoot)
	require.ErrorIs(t, err, os.ErrNotExist)
	completed, err := os.ReadFile(owner.items[workspace.ID].path)
	require.NoError(t, err)
	require.Equal(t, content, completed)
	require.NoError(t, owner.Remove(t.Context(), "alice", workspace.ID))
}

// TestBackupDirectoryDottedParentRetainsWorkspace exercises the configured
// parent path, whose spelling must not hide an occupied root during cleanup.
func TestBackupDirectoryDottedParentRetainsWorkspace(t *testing.T) {
	directoryLiveEnvironment(t, "")
	_, content := ownerFixture(t)
	owner := New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	owner.CaptureDirectory(t.TempDir() + "/./")
	workspace, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	path := owner.items[workspace.ID].path
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, before)
	prepared, err := owner.PrepareDirectory(t.Context(), filepath.Join(t.TempDir(), "next-backups"), 1)
	require.NoError(t, err)
	require.NoError(t, prepared.Complete(t.Context(), true))
	after, err := os.ReadFile(path)
	require.NoError(t, err, "acceptance must retain an occupied root regardless of configured path spelling")
	require.Equal(t, before, after)
	_, err = owner.Inspect(t.Context(), "alice", workspace.ID)
	require.NoError(t, err)
}

// TestBackupDirectoryRejectsParentTraversal prevents a linked prefix plus '..'
// from concealing a private root ancestor during candidate admission.
func TestBackupDirectoryRejectsParentTraversal(t *testing.T) {
	directoryLiveEnvironment(t, "")
	owner, content := ownerFixture(t)
	workspace, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	path := owner.items[workspace.ID].path
	child := filepath.Join(owner.root, "alias-target")
	require.NoError(t, os.Mkdir(child, 0o700))
	alias := filepath.Join(t.TempDir(), "owned-child-link")
	require.NoError(t, os.Symlink(child, alias))
	_, err = owner.PrepareDirectory(t.Context(), alias+"/../nested", 1)
	require.Error(t, err, "parent traversal must refuse before a linked prefix can conceal an owned ancestor")
	_, err = os.Stat(filepath.Join(owner.root, "nested"))
	require.ErrorIs(t, err, os.ErrNotExist)
	kept, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, kept)
	_, err = owner.Inspect(t.Context(), "alice", workspace.ID)
	require.NoError(t, err)
}
