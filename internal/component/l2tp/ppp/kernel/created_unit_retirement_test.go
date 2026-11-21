// Design: docs/research/l2tpv2-ze-integration.md -- exact unit retirement completion

package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A regular owned file models only completion bookkeeping. It does not grant
// PPP creation, native attachment, a counter reader or kernel retirement proof.
func retirementTestUnit(t *testing.T) *CreatedUnit {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "original-unit"))
	if err != nil {
		t.Fatal(err)
	}
	unit := &CreatedUnit{file: file}
	unit.self = unit
	t.Cleanup(func() {
		unit.mu.Lock()
		closed := unit.closed
		unit.mu.Unlock()
		if !closed {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	return unit
}

// Cancellation must leave the original open; successful Close then supplies
// the retained result even when the caller's context is already canceled.
func TestCreatedUnitRetirementWaitDoesNotInitiateClose(t *testing.T) {
	unit := retirementTestUnit(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := unit.WaitNativePPPUnitRetired(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("passive wait: %v", err)
	}
	if _, err := unit.file.Stat(); err != nil {
		t.Fatalf("wait closed the original: %v", err)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := unit.WaitNativePPPUnitRetired(ctx); err != nil {
			t.Fatalf("completed native-file outcome: %v", err)
		}
		if err := unit.Close(); err != nil {
			t.Fatalf("repeated Close changed the result: %v", err)
		}
	}
}

// Nil, zero and copied owners must not mint a terminal native receipt. The
// copied private fields deliberately preserve the original self pointer.
func TestCreatedUnitRetirementRejectsUnmintedOwners(t *testing.T) {
	unit := retirementTestUnit(t)
	copy := &CreatedUnit{self: unit, file: unit.file}
	for _, owner := range []*CreatedUnit{nil, {}, copy} {
		if err := owner.WaitNativePPPUnitRetired(t.Context()); err == nil {
			t.Fatal("unminted owner supplied retirement")
		}
	}
	if err := unit.WaitNativePPPUnitRetired(nil); err == nil { //nolint:staticcheck // SA1012: the nil Context is the refused input this test proves.
		t.Fatal("nil context supplied a wait")
	}
	if err := copy.WithDescriptor(func(int) error { return nil }); err == nil {
		t.Fatal("copied open owner lent its original descriptor")
	}
	if _, err := unit.file.Stat(); err != nil {
		t.Fatalf("invalid observer harmed original: %v", err)
	}
}

// Copy after completion while both mutexes are idle. The copied terminal
// result must still grant no authority over the original unit's retirement.
func TestCreatedUnitRetirementRejectsCompletedOwnerCopy(t *testing.T) {
	unit := retirementTestUnit(t)
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	copy := *unit //nolint:govet // Deliberately copy the complete idle owner to prove the self-identity boundary.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := copy.WaitNativePPPUnitRetired(ctx); err == nil {
		t.Fatal("copied completed owner supplied retirement authority")
	}
	if err := copy.WithDescriptor(func(int) error { return nil }); err == nil {
		t.Fatal("copied completed owner lent the original descriptor")
	}
	if err := unit.WaitNativePPPUnitRetired(ctx); err != nil {
		t.Fatalf("copy harmed original completion: %v", err)
	}
}

// A failed original-file close is retained as failure, never converted into
// successful retirement, and no repeated observer retries the native close.
func TestCreatedUnitRetirementRetainsOriginalCloseFailure(t *testing.T) {
	unit := retirementTestUnit(t)
	if err := unit.file.Close(); err != nil {
		t.Fatal(err)
	}
	cause := unit.Close()
	if !errors.Is(cause, os.ErrClosed) {
		t.Fatalf("original close cause: %v", cause)
	}
	for range 2 {
		if err := unit.WaitNativePPPUnitRetired(t.Context()); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("retirement lost close cause: %v", err)
		}
		if err := unit.Close(); !errors.Is(err, cause) {
			t.Fatalf("Close retried or lost cause: %v", err)
		}
	}
}

// The namespace file is a separate cleanup obligation. Its failure remains in
// Close while the successful original unit-file close can settle native writes.
func TestCreatedUnitRetirementSeparatesNamespaceCloseCause(t *testing.T) {
	unit := retirementTestUnit(t)
	file, err := os.Create(filepath.Join(t.TempDir(), "creator-namespace"))
	if err != nil {
		t.Fatal(err)
	}
	unit.namespace = &capturedSessionNamespace{file: file}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unit.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("namespace cause missing: %v", err)
	}
	if err := unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
		t.Fatalf("namespace failure obscured confirmed unit-file close: %v", err)
	}
}

// Library retention has one original duplicate. A failed close or panic must
// remain a failed outcome after original closure, without another close call.
func TestCreatedUnitRetirementRetainsDuplicateCloseFailure(t *testing.T) {
	for _, panicClose := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panicClose], func(t *testing.T) {
			unit := retirementTestUnit(t)
			cause := errors.New("retained original duplicate close cause")
			calls := 0
			closeDuplicate := func(int) error {
				calls++
				if panicClose {
					panic(cause)
				}
				return cause
			}
			hold, err := unit.retainNativeUnit(-1, func(int) (int, error) { return 91, nil }, closeDuplicate)
			if err != nil {
				t.Fatal(err)
			}
			if err := unit.Close(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := hold.close(closeDuplicate); !errors.Is(err, cause) {
					t.Fatalf("duplicate close cause: %v", err)
				}
			}
			if calls != 1 {
				t.Fatalf("duplicate close retried %d times", calls)
			}
			if err := unit.WaitNativePPPUnitRetired(t.Context()); !errors.Is(err, cause) {
				t.Fatalf("retirement hid uncertain duplicate: %v", err)
			}
		})
	}
}

// A private duplicate port can return a partial descriptor with an error.
// The actual retention producer must close that exact returned owner once,
// retain the operation cause, and leave the original file available to Close.
func TestCreatedUnitRetentionPartialResultClosesExactDuplicate(t *testing.T) {
	unit := retirementTestUnit(t)
	cause := errors.New("partial duplicate result")
	var closed []int
	closeDuplicate := func(fd int) error { closed = append(closed, fd); return nil }
	hold, err := unit.retainNativeUnit(-1, func(int) (int, error) { return 92, cause }, closeDuplicate)
	if !errors.Is(err, cause) {
		t.Fatalf("partial result lost original cause: %v", err)
	}
	if hold == nil {
		t.Fatal("partial result lost its tracked owner")
	}
	if len(closed) != 1 {
		t.Fatalf("partial close count: %v", closed)
	}
	if closed[0] != 92 {
		t.Fatalf("partial close selected a different descriptor: %v", closed)
	}
	if _, err := unit.file.Stat(); err != nil {
		t.Fatalf("partial result closed original file: %v", err)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
		t.Fatalf("confirmed partial cleanup prevented retirement: %v", err)
	}
	if err := hold.close(closeDuplicate); err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 {
		t.Fatalf("completed partial cleanup retried: %v", closed)
	}
}

// A failed native fcntl with no new descriptor leaves no retention. A later
// successful binding must remain possible on the SAME still-open original.
func TestCreatedUnitRetentionNoDescriptorFailurePermitsRetry(t *testing.T) {
	unit := retirementTestUnit(t)
	cause := errors.New("duplicate refused before allocation")
	calls := 0
	closeDuplicate := func(fd int) error {
		calls++
		if fd != 93 {
			t.Errorf("closed descriptor %d, want 93", fd)
		}
		return nil
	}
	if _, err := unit.retainNativeUnit(-1, func(int) (int, error) { return -1, cause }, closeDuplicate); !errors.Is(err, cause) {
		t.Fatalf("missing original refusal: %v", err)
	}
	if calls != 0 {
		t.Fatalf("unallocated refusal closed a descriptor: %d", calls)
	}
	hold, err := unit.retainNativeUnit(-1, func(int) (int, error) { return 93, nil }, closeDuplicate)
	if err != nil {
		t.Fatalf("clean refusal poisoned later original retention: %v", err)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := unit.WaitNativePPPUnitRetired(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("later live retention was forgotten: %v", err)
	}
	if err := hold.close(closeDuplicate); err != nil {
		t.Fatal(err)
	}
	if err := unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("successful later duplicate close count=%d", calls)
	}
}
