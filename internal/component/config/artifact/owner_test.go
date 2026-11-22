// Design: docs/architecture/web-interface.md -- shared artifact quota and cleanup preservation.
package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	_ "github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/zefs"
)

func ownerFixture(t *testing.T) (*Owner, []byte) {
	t.Helper()
	owner := New()
	owner.parentTemp = t.TempDir()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	path := filepath.Join(t.TempDir(), "original.zefs")
	store, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		return seed.WriteFile("router.conf", []byte("set bgp router-id 1.2.3.4\n"), 0o600)
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, store.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return owner, content
}

func ownerUpload(t *testing.T, owner *Owner, user string, content []byte) (Info, error) {
	t.Helper()
	transfer, err := owner.Begin(t.Context(), user, "original.zefs", -1, false)
	if err != nil {
		return Info{}, err
	}
	for offset := 0; offset < len(content); offset += command.ContentOctetsMax {
		_, err = owner.Append(t.Context(), user, transfer.ID, int64(offset), content[offset:min(offset+command.ContentOctetsMax, len(content))])
		if err != nil {
			return Info{}, errors.Join(err, owner.Abort(context.WithoutCancel(t.Context()), user, transfer.ID))
		}
	}
	info, _, err := owner.Finish(t.Context(), user, transfer.ID)
	return info, err
}

// TestArtifactCorruptUploadLeavesNoPrivateFiles proves failed integrity admission
// removes all partial files and accounting, without exposing an artifact identity.
func TestArtifactCorruptUploadLeavesNoPrivateFiles(t *testing.T) {
	owner, _ := ownerFixture(t)
	_, err := ownerUpload(t, owner, "alice", []byte("corrupt artifact"))
	require.Error(t, err)
	assert.Empty(t, owner.items)
	assert.Empty(t, owner.uploads)
	assert.Zero(t, owner.bytes)
	entries, err := os.ReadDir(owner.root)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestArtifactRemainingBudgetBoundsUpload preserves the streaming admission
// proof: one byte less remaining than a real artifact refuses without retention.
func TestArtifactRemainingBudgetBoundsUpload(t *testing.T) {
	owner, content := ownerFixture(t)
	owner.bytes = OwnerBytesMax - int64(len(content)) + 1
	_, err := ownerUpload(t, owner, "alice", content)
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, 413, refusal.Status)
	assert.Empty(t, owner.items)
}

// TestArtifactFailedUploadCleanupSealsAdmission preserves one bounded failed
// directory, existing workspace access, and admission only after debt removal.
func TestArtifactFailedUploadCleanupSealsAdmission(t *testing.T) {
	owner, content := ownerFixture(t)
	valid, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	validDirectory := filepath.Dir(owner.items[valid.ID].path)
	blocked := ""
	owner.mu.Lock()
	owner.removeAll = func(path string) error {
		if path == validDirectory {
			return os.RemoveAll(path)
		}
		if blocked == "" {
			blocked = path
		}
		if path == blocked {
			return errors.New("injected partial-upload removal failure")
		}
		return os.RemoveAll(path)
	}
	owner.mu.Unlock()
	_, err = ownerUpload(t, owner, "alice", []byte("corrupt artifact"))
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 500, refusal.Status)
	require.Equal(t, blocked, owner.cleanupDebt)
	require.Positive(t, owner.debtBytes)
	entries, err := os.ReadDir(owner.root)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	_, err = ownerUpload(t, owner, "alice", content)
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, 503, refusal.Status)
	assert.Len(t, owner.items, 1)
	entries, err = os.ReadDir(owner.root)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	_, err = owner.Inspect(t.Context(), "alice", valid.ID)
	require.NoError(t, err)
	owner.mu.Lock()
	owner.removeAll = os.RemoveAll
	owner.mu.Unlock()
	_, err = ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	assert.Empty(t, owner.cleanupDebt)
	assert.Zero(t, owner.debtBytes)
	_, err = os.Stat(blocked)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// TestArtifactFailedSnapshotCleanupBoundsAllOwners preserves the complete
// download before cleanup fails, then fences allocation by another account.
func TestArtifactFailedSnapshotCleanupBoundsAllOwners(t *testing.T) {
	owner, content := ownerFixture(t)
	first, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	second, err := ownerUpload(t, owner, "bob", content)
	require.NoError(t, err)
	snapshot, err := owner.Snapshot(t.Context(), "alice", first.ID, "")
	require.NoError(t, err)
	_, bytes, err := owner.Read(t.Context(), "alice", snapshot.ID, 0)
	require.NoError(t, err)
	require.NotEmpty(t, bytes)
	directory := filepath.Dir(owner.sources[snapshot.ID].path)
	owner.mu.Lock()
	owner.removeAll = func(path string) error {
		if path == directory {
			return errors.New("injected snapshot cleanup failure")
		}
		return os.RemoveAll(path)
	}
	owner.mu.Unlock()
	require.Error(t, owner.Abort(t.Context(), "alice", snapshot.ID))
	require.NotEmpty(t, owner.cleanupDebt)
	require.Positive(t, owner.debtBytes)
	_, err = owner.Snapshot(t.Context(), "bob", second.ID, "")
	require.Error(t, err)
	entries, err := os.ReadDir(owner.root)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	_, err = owner.Inspect(t.Context(), "bob", second.ID)
	require.NoError(t, err)
	inspection, err := owner.Snapshot(t.Context(), "bob", second.ID, "router.conf")
	require.NoError(t, err)
	_, inspected, err := owner.Read(t.Context(), "bob", inspection.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, "set bgp router-id 1.2.3.4\n", string(inspected), "ordinary committed-source inspection must survive failed snapshot cleanup")
	require.NoError(t, owner.Abort(t.Context(), "bob", inspection.ID))
	owner.mu.Lock()
	owner.removeAll = os.RemoveAll
	owner.mu.Unlock()
	retry, err := owner.Snapshot(t.Context(), "bob", second.ID, "")
	require.NoError(t, err)
	require.NoError(t, owner.Abort(t.Context(), "bob", retry.ID))
	assert.Empty(t, owner.cleanupDebt)
	assert.Zero(t, owner.debtBytes)
}

// TestArtifactStorageFailureReachesDoctor retains the real runtime-event
// observation without recording account, label or content in the diagnostic.
func TestArtifactStorageFailureReachesDoctor(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	diagnostic.RegisterBuiltinCodes()
	owner, content := ownerFixture(t)
	owner.parentTemp = filepath.Join(t.TempDir(), "missing-parent")
	_, err := ownerUpload(t, owner, "private-account", content)
	require.Error(t, err)
	require.Contains(t, diagnostic.EventCodes(), CodeDoctorBackupWorkspaceStorage)
	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, CodeDoctorBackupWorkspaceStorage, events[0].Code)
	assert.NotContains(t, events[0].Message, "private-account")
	assert.NotContains(t, events[0].Message, "original.zefs")
	assert.NotEmpty(t, events[0].Fix)
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "recent-events" {
			continue
		}
		require.Contains(t, check.Codes, CodeDoctorBackupWorkspaceStorage)
		rows := check.Check(diagnostic.DoctorCheckContext{})
		require.Len(t, rows, 1)
		assert.Equal(t, CodeDoctorBackupWorkspaceStorage, rows[0].Code)
		assert.Equal(t, events[0].Fix, rows[0].Fix)
		assert.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
		assert.Contains(t, rows[0].Message, events[0].Message)
		assert.Contains(t, rows[0].Message, events[0].Source)
		assert.Contains(t, rows[0].Message, events[0].Subject)
		assert.Contains(t, rows[0].Message, "1 time, last at "+events[0].Updated.UTC().Format(time.RFC3339))
		assert.NotContains(t, rows[0].Message, "private-account")
		assert.NotContains(t, rows[0].Message, "original.zefs")
		return
	}
	t.Fatal("the runtime recent-events doctor check must report workspace storage failures")
}

// TestArtifactExpiryAndClose preserves unrelated files while expiring owned
// workspaces and refusing new admission after the shared owner has stopped.
func TestArtifactExpiryAndClose(t *testing.T) {
	owner, content := ownerFixture(t)
	now := time.Now()
	owner.now = func() time.Time { return now }
	info, err := ownerUpload(t, owner, "alice", content)
	require.NoError(t, err)
	sentinel := filepath.Join(owner.parentTemp, "unrelated")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o600))
	owner.mu.Lock()
	now = now.Add(Idle + time.Second)
	owner.mu.Unlock()
	_, err = owner.Inspect(t.Context(), "alice", info.ID)
	require.Error(t, err)
	assert.Empty(t, owner.items)
	assert.Zero(t, owner.bytes)
	root := owner.root
	require.NoError(t, owner.Close())
	_, err = os.Stat(root)
	require.ErrorIs(t, err, os.ErrNotExist)
	kept, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(kept))
	_, err = owner.Begin(t.Context(), "alice", "original.zefs", -1, false)
	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, 410, refusal.Status)
}

// TestArtifactCloseFailureReportsPossibleRetention preserves cleanup failure
// identity and doctor reporting while the worker and admission have stopped.
func TestArtifactCloseFailureReportsPossibleRetention(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	owner := New()
	owner.parentTemp = t.TempDir()
	require.NoError(t, owner.ensureRoot())
	root := owner.root
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	path := filepath.Join(root, "retained.zefs")
	require.NoError(t, os.WriteFile(path, []byte("private task-owned bytes"), 0o600))
	failure := errors.New("injected private directory cleanup failure")
	owner.removeAll = func(string) error { return failure }
	err := owner.Close()
	require.ErrorIs(t, err, failure)
	assert.Contains(t, err.Error(), "files may remain")
	assert.Contains(t, err.Error(), "Run ze show doctor")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "private task-owned bytes", string(content))
	_, err = owner.Begin(t.Context(), "alice", "original.zefs", -1, false)
	require.Error(t, err)
	require.ErrorIs(t, owner.Close(), failure)
	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, CodeDoctorBackupWorkspaceStorage, events[0].Code)
}
