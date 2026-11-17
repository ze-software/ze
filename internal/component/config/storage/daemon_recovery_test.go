// Design: docs/architecture/storage-backends.md -- full restore after owner close.
package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/zefs"
)

func daemonRecoveryFixture(t *testing.T) (Storage, string, string) {
	t.Helper()
	dir := t.TempDir()
	live, err := Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, live.Close()) })
	require.NoError(t, live.WriteKey("meta/test/current", []byte("original live bytes")))
	path := filepath.Join(t.TempDir(), "recovery.zefs")
	artifact, err := CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.WriteKey("meta/test/restored", []byte{0, 255, 1, 128}))
	require.NoError(t, artifact.WriteKey("meta/config/router.conf/active", []byte("opaque pointer retained")))
	require.NoError(t, artifact.Close())
	return BindConfigSource(live, "router.conf", ConfigSourceStored), dir, path
}

// TestDaemonRecoveryAppliesOnlyAfterOwnerClose proves accepted immutable bytes
// replace the whole store only after its actual writer closes. Method: stage,
// change the caller's file, verify the live lock remains held, close and apply.
func TestDaemonRecoveryAppliesOnlyAfterOwnerClose(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	require.Same(t, source, owner.Borrow())
	admission := RecoveryFor(owner.Borrow())
	require.NotNil(t, admission)
	receipt, err := admission.StagePath(context.Background(), path)
	require.NoError(t, err)
	require.Positive(t, receipt.Bytes)
	require.Len(t, receipt.Digest, 64)
	old, err := source.ReadKey("meta/test/current")
	require.NoError(t, err)
	require.Equal(t, "original live bytes", string(old))
	_, err = RestoreBlob(path, dir)
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, os.WriteFile(path, []byte("caller replaced its original"), 0o600))
	closed, err := owner.Close()
	require.NoError(t, err)
	require.NoError(t, closed.Apply())
	restored, err := Open(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, restored.Close()) }()
	data, err := restored.ReadKey("meta/test/restored")
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 1, 128}, data)
	pointer, err := restored.ReadKey("meta/config/router.conf/active")
	require.NoError(t, err)
	require.Equal(t, "opaque pointer retained", string(pointer))
	_, err = restored.ReadKey("meta/test/current")
	require.ErrorIs(t, err, os.ErrNotExist)
	retired, err := filepath.Glob(filepath.Join(dir, TreeName+replacedInfix+"*"))
	require.NoError(t, err)
	require.Len(t, retired, 1)
}

// TestDaemonRecoveryRefusesCanceledAndDuplicateAdmission proves a request that
// ended before acceptance has no pending work, while one accepted source cannot
// be replaced by a second request. Method: cancel, accept, retry, then discard.
func TestDaemonRecoveryRefusesCanceledAndDuplicateAdmission(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	admission := RecoveryFor(owner.Borrow())
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = admission.StagePath(canceled, path)
	require.ErrorIs(t, err, context.Canceled)
	_, err = admission.StagePath(context.Background(), path)
	require.NoError(t, err)
	_, err = admission.StagePath(context.Background(), path)
	require.Error(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	_, err = admission.StagePath(context.Background(), path)
	require.Error(t, err)
	require.NoError(t, closed.Discard())
	reopened, err := Open(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, reopened.Close()) }()
	data, err := reopened.ReadKey("meta/test/current")
	require.NoError(t, err)
	require.Equal(t, "original live bytes", string(data))
}

// TestDaemonRecoveryRefusesInvalidLifecycleValues proves a zero lifecycle value
// cannot close, stage or restore a store. Method: call each public boundary and
// require a normal error rather than a panic or a successful no-op.
func TestDaemonRecoveryRefusesInvalidLifecycleValues(t *testing.T) {
	_, err := NewDaemonRecovery(nil)
	require.Error(t, err)
	var owner DaemonRecovery
	_, err = owner.Close()
	require.Error(t, err)
	var admission RecoveryAdmission
	_, err = admission.StagePath(context.Background(), "/unused")
	require.Error(t, err)
	var closed ClosedRecovery
	require.Error(t, closed.Apply())
	require.Error(t, closed.Discard())
	require.False(t, errors.Is(err, context.Canceled))
}

// TestDaemonRecoveryRetainsTheAcceptedDirectory proves a directory replacement
// cannot redirect an accepted restore into a foreign store. Method: admit on
// the live handle, move its parent, create another store at its former name,
// then close and apply through the retained destination directory.
func TestDaemonRecoveryRetainsTheAcceptedDirectory(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	_, err = RecoveryFor(owner.Borrow()).StagePath(context.Background(), path)
	require.NoError(t, err)
	held := filepath.Join(t.TempDir(), "held")
	require.NoError(t, os.Rename(dir, held))
	require.NoError(t, os.Mkdir(dir, 0o700))
	foreign, err := Create(dir)
	require.NoError(t, err)
	require.NoError(t, foreign.WriteKey("meta/test/foreign", []byte("leave this store unchanged")))
	require.NoError(t, foreign.Close())
	closed, err := owner.Close()
	require.NoError(t, err)
	require.NoError(t, closed.Apply())
	foreign, err = Open(dir)
	require.NoError(t, err)
	foreignBytes, err := foreign.ReadKey("meta/test/foreign")
	require.NoError(t, err)
	require.Equal(t, "leave this store unchanged", string(foreignBytes))
	_, err = foreign.ReadKey("meta/test/restored")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, foreign.Close())
	restored, err := Open(held)
	require.NoError(t, err)
	data, err := restored.ReadKey("meta/test/restored")
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 1, 128}, data)
	require.NoError(t, restored.Close())
}

// TestDaemonRecoveryRefusesAnOwnedSourceArtifact proves staging observes the
// source artifact's existing ownership lock, so it cannot copy a blob while
// another editor owns its publication lifetime.
func TestDaemonRecoveryRefusesAnOwnedSourceArtifact(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	defer func() {
		closed, err := owner.Close()
		require.NoError(t, err)
		require.NoError(t, closed.Discard())
	}()
	writer, err := OpenBlob(path, true)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()
	_, err = RecoveryFor(owner.Borrow()).StagePath(context.Background(), path)
	require.ErrorIs(t, err, ErrBusy)
	stages, err := filepath.Glob(filepath.Join(dir, recoveryStagePrefix+"*"))
	require.NoError(t, err)
	require.Empty(t, stages)
}

// TestDaemonRecoveryRefusesAnotherRetainedStage proves retained recovery bytes
// bound later admission across daemon lifetimes, rather than accumulating a new
// full artifact after each failed or interrupted attempt.
func TestDaemonRecoveryRefusesAnotherRetainedStage(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	retained := filepath.Join(dir, recoveryStagePrefix+"unfinished")
	require.NoError(t, os.Mkdir(retained, 0o700))
	marker := filepath.Join(retained, "retained-source")
	require.NoError(t, os.WriteFile(marker, []byte("preserve unresolved bytes"), 0o600))
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	defer func() {
		closed, err := owner.Close()
		require.NoError(t, err)
		require.NoError(t, closed.Discard())
	}()
	_, err = RecoveryFor(owner.Borrow()).StagePath(context.Background(), path)
	require.Error(t, err)
	require.Contains(t, err.Error(), retained)
	stages, err := filepath.Glob(filepath.Join(dir, recoveryStagePrefix+"*"))
	require.NoError(t, err)
	require.Equal(t, []string{retained}, stages)
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "preserve unresolved bytes", string(data))
}

// TestDaemonRecoveryCloseFailureKeepsItsSource proves an original handle that
// cannot close never yields an Apply capability and retains the verified bytes.
// Method: close one actual owned tree descriptor before the owner closes.
func TestDaemonRecoveryCloseFailureKeepsItsSource(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	_, err = RecoveryFor(source).StagePath(context.Background(), path)
	require.NoError(t, err)
	owned, found := ownedStore(source)
	require.True(t, found)
	require.NoError(t, owned.tree.root.Close())
	closed, err := owner.Close()
	require.Nil(t, closed)
	require.ErrorIs(t, err, os.ErrClosed)
	require.Contains(t, err.Error(), "ze data restore")
	stages, globErr := filepath.Glob(filepath.Join(dir, recoveryStagePrefix+"*"))
	require.NoError(t, globErr)
	require.Len(t, stages, 1)
	require.NoError(t, CheckArtifact(filepath.Join(stages[0], recoveryArtifactName)))
	reopened, openErr := Open(dir)
	require.NoError(t, openErr)
	defer func() { require.NoError(t, reopened.Close()) }()
	current, readErr := reopened.ReadKey("meta/test/current")
	require.NoError(t, readErr)
	require.Equal(t, "original live bytes", string(current))
}

// TestDaemonRecoveryApplyFailureKeepsItsSource proves a new writer winning
// the target lock refuses replacement and leaves the accepted source intact.
func TestDaemonRecoveryApplyFailureKeepsItsSource(t *testing.T) {
	source, dir, path := daemonRecoveryFixture(t)
	owner, err := NewDaemonRecovery(source)
	require.NoError(t, err)
	_, err = RecoveryFor(source).StagePath(context.Background(), path)
	require.NoError(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	competing, err := Open(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, competing.Close()) }()
	err = closed.Apply()
	require.ErrorIs(t, err, ErrBusy)
	require.Contains(t, err.Error(), "ze data restore")
	stage, err := PendingRecoveryStage(competing)
	require.NoError(t, err)
	require.NotEmpty(t, stage)
	require.NoError(t, CheckArtifact(filepath.Join(stage, recoveryArtifactName)))
	current, err := competing.ReadKey("meta/test/current")
	require.NoError(t, err)
	require.Equal(t, "original live bytes", string(current))
}
