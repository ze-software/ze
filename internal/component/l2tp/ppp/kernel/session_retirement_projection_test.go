// Design: docs/research/l2tpv2-ze-integration.md -- exact physical release projection

package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// A regular file exercises only the session's projection of its original
// retirement ledger. It supplies no PPP/native authority or liveness proof.
func retirementProjectionSession(unit *CreatedUnit) *session {
	return &session{closed: true, unitLease: &claimedCreatedUnit{unit: unit}}
}

// A completed writer socket cannot certify an unsuccessful original-file
// close. The retained first-close cause must keep the session owner pending.
func TestSessionNativeReleaseProjectionRetainsOriginalCloseFailure(t *testing.T) {
	unit := retirementTestUnit(t)
	owner := retirementProjectionSession(unit)
	if err := unit.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unit.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original close cause missing: %v", err)
	}
	if unit.NativeOperationsPending() {
		t.Fatal("model unexpectedly retained a native writer")
	}
	if !owner.NativeReleasePending() {
		t.Fatal("failed original close was reported as confirmed native release")
	}
}

// Actual borrow bookkeeping must retain the original until its callback
// joins, even after Close succeeds and there is no pending writer socket.
func TestSessionNativeReleaseProjectionJoinsOriginalBorrow(t *testing.T) {
	unit := retirementTestUnit(t)
	owner := retirementProjectionSession(unit)
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	var borrowErr error
	go func() {
		defer close(joined)
		borrowErr = unit.withOriginalUnitFile(func(*os.File) error {
			close(entered)
			<-release
			return nil
		})
	}()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		<-joined
		if borrowErr != nil {
			t.Error(borrowErr)
		}
	})
	<-entered
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if !owner.NativeReleasePending() {
		t.Fatal("unfinished original borrow was reported as confirmed native release")
	}
	releaseOnce.Do(func() { close(release) })
	<-joined
	if err := unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
		t.Fatal(err)
	}
	if owner.NativeReleasePending() {
		t.Fatal("joined original borrow retained completed native release")
	}
}

// The session's consumed remote-duplicate flag must not erase the actual
// first-close failure retained by that original unit's duplicate ledger.
func TestSessionNativeReleaseProjectionRetainsDuplicateCloseFailure(t *testing.T) {
	unit := retirementTestUnit(t)
	owner := retirementProjectionSession(unit)
	cause := errors.New("original duplicate did not confirm close")
	closeDuplicate := func(int) error { return cause }
	hold, err := unit.retainNativeUnit(-1, func(int) (int, error) { return 91, nil }, closeDuplicate)
	if err != nil {
		t.Fatal(err)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := hold.close(closeDuplicate); !errors.Is(err, cause) {
		t.Fatalf("duplicate close cause missing: %v", err)
	}
	if !owner.NativeReleasePending() {
		t.Fatal("failed original duplicate close was reported as confirmed native release")
	}
}

// Namespace cleanup is independent history. Actual successful unit/borrow/
// duplicate retirement must clear custody while preserving that Close cause.
func TestSessionNativeReleaseProjectionIgnoresNamespaceHistory(t *testing.T) {
	unit := retirementTestUnit(t)
	owner := retirementProjectionSession(unit)
	namespace, err := os.Create(filepath.Join(t.TempDir(), "namespace-history"))
	if err != nil {
		t.Fatal(err)
	}
	unit.namespace = &capturedSessionNamespace{file: namespace}
	if err := namespace.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unit.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("namespace history missing: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := unit.WaitNativePPPUnitRetired(ctx); err != nil {
		t.Fatalf("namespace history obscured physical retirement: %v", err)
	}
	if owner.NativeReleasePending() {
		t.Fatal("namespace-only history retained completed native release")
	}
}

type retirementProjectionWriter struct {
	NativeUnitOperations
	settles int
	closes  int
}

func (writer *retirementProjectionWriter) Settle(ctx context.Context) error {
	writer.settles++
	return ctx.Err()
}
func (writer *retirementProjectionWriter) Close() error { writer.closes++; return nil }
func (*retirementProjectionWriter) ClosePending() bool  { return false }

// The independent retained seal is a prerequisite, not a local timeout. The
// actual local settlement call begins only after that exact hold completes.
func TestSessionNativeReleaseProjectionPendingSealDefersSettlement(t *testing.T) {
	unit := retirementTestUnit(t)
	owner := retirementProjectionSession(unit)
	writer := &retirementProjectionWriter{}
	bound := make(chan struct{})
	close(bound)
	unit.nativeOperations = &nativeUnitOperationsBinding{done: bound, ownership: nativeUnitOperationsOwned{writer}}
	closeDuplicate := func(int) error { return nil }
	hold, err := unit.retainNativeUnit(-1, func(int) (int, error) { return 91, nil }, closeDuplicate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := hold.close(closeDuplicate); err != nil {
			t.Error(err)
		}
	})
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	owner.unitRetained = true
	if err := owner.closeNativeOperations(); err != nil {
		t.Fatalf("independent seal manufactured a local settlement failure: %v", err)
	}
	if writer.settles != 0 || writer.closes != 0 {
		t.Fatalf("local writer ran before retained seal completion: settle=%d close=%d", writer.settles, writer.closes)
	}
	if err := hold.close(closeDuplicate); err != nil {
		t.Fatal(err)
	}
	owner.unitRetained = false
	if err := owner.closeNativeOperations(); err != nil {
		t.Fatalf("completed seal left local settlement blocked: %v", err)
	}
	if writer.settles != 1 || writer.closes != 1 || unit.NativeOperationsPending() {
		t.Fatalf("completed seal did not settle the same writer: settle=%d close=%d pending=%v", writer.settles, writer.closes, unit.NativeOperationsPending())
	}
}
