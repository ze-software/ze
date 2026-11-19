// Design: docs/research/l2tpv2-ze-integration.md -- exact binding custody models

//go:build linux

package kernel

import (
	"context"
	"errors"
	"net/netip"
	"os"

	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type nativeOperationsModel struct {
	sessionTestMutationOperations
	mu      sync.Mutex
	settle  func(context.Context) error
	close   func() error
	pending bool
}

func (model *nativeOperationsModel) Settle(ctx context.Context) error {
	if model.settle != nil {
		return model.settle(ctx)
	}
	return nil
}
func (model *nativeOperationsModel) Close() error {
	if model.close != nil {
		return model.close()
	}
	return nil
}
func (model *nativeOperationsModel) ClosePending() bool {
	model.mu.Lock()
	defer model.mu.Unlock()
	return model.pending
}
func (model *nativeOperationsModel) setPending(pending bool) {
	model.mu.Lock()
	model.pending = pending
	model.mu.Unlock()
}

func TestCreatedNativeBindingKeepsExactOriginalOwner(t *testing.T) {
	unit, _, _ := capturedModelOwners(t)
	backend := unit.backend
	writer := &nativeOperationsModel{}
	calls := 0
	operations, err := unit.BindNativeOperations(context.Background(), backend, func(original *CreatedUnit, index int, number int32) (NativeUnitOperations, error) {
		calls++
		if original != unit || index != 73 || number != 7 {
			t.Errorf("changed original binding: %p %d %d", original, index, number)
		}
		return writer, nil
	})
	if err != nil || operations != writer || calls != 1 {
		t.Fatalf("bind: %v %T calls %d", err, operations, calls)
	}
	if unit.backend != backend {
		t.Fatal("binding replaced original creation/counter backend")
	}
	if _, err := unit.BindNativeOperations(context.Background(), backend, func(*CreatedUnit, int, int32) (NativeUnitOperations, error) { calls++; return writer, nil }); err == nil || calls != 1 {
		t.Fatalf("replayed binding invoked producer: %v calls %d", err, calls)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unit.CloseNativeOperations(context.Background()); err != nil {
		t.Fatal(err)
	}
	if unit.NativeOperationsPending() {
		t.Fatal("terminal socket remains pending")
	}
}

func TestCreatedNativeBindingPartialWriterKeepsCustodyAndCauses(t *testing.T) {
	unit, _, _ := capturedModelOwners(t)
	bindCause := errors.New("partial original socket bind")
	closeCause := errors.New("terminal original socket close history")
	writer := &nativeOperationsModel{close: func() error { return closeCause }}
	operations, err := unit.BindNativeOperations(context.Background(), unit.backend, func(*CreatedUnit, int, int32) (NativeUnitOperations, error) { return writer, bindCause })
	if operations != writer || !errors.Is(err, bindCause) || !unit.NativeOperationsPending() {
		t.Fatalf("partial writer lost: %T %v pending %v", operations, err, unit.NativeOperationsPending())
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		err := unit.CloseNativeOperations(context.Background())
		if !errors.Is(err, bindCause) || !errors.Is(err, closeCause) {
			t.Fatalf("terminal causes lost: %v", err)
		}
		if unit.NativeOperationsPending() {
			t.Fatal("historical close error confused with pending socket")
		}
	}
}

func TestCreatedNativeCloseDoesNotKeepUnitMutexAcrossSettlement(t *testing.T) {
	unit, _, _ := capturedModelOwners(t)
	entered, release := make(chan struct{}), make(chan struct{})
	writer := &nativeOperationsModel{settle: func(ctx context.Context) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	if _, err := unit.BindNativeOperations(context.Background(), unit.backend, func(*CreatedUnit, int, int32) (NativeUnitOperations, error) { return writer, nil }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- unit.CloseNativeOperations(ctx) }()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- unit.Close() }()
	joined := false
	select {
	case err := <-closed:
		joined = true
		if err != nil {
			t.Error(err)
		}
	case <-ctx.Done():
		t.Error("physical original Close blocked behind writer settlement")
	}
	close(release)
	if err := <-done; err != nil {
		t.Error(err)
	}
	if !joined {
		<-closed
	}
}

func TestCreatedNativeCloseUsesCurrentCustodyAfterExactSettlement(t *testing.T) {
	unit, _, _ := capturedModelOwners(t)
	pending := errors.New("model pending ACK")
	writer := &nativeOperationsModel{pending: true, settle: func(context.Context) error { return pending }, close: func() error { return pending }}
	if _, err := unit.BindNativeOperations(context.Background(), unit.backend, func(*CreatedUnit, int, int32) (NativeUnitOperations, error) { return writer, nil }); err != nil {
		t.Fatal(err)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := unit.CloseNativeOperations(context.Background()); !errors.Is(err, pending) || !unit.NativeOperationsPending() {
		t.Fatalf("unresolved writer lost: %v", err)
	}
	writer.close = func() error { writer.setPending(false); return nil }
	if err := unit.CloseNativeOperations(context.Background()); err != nil {
		t.Fatalf("transient pending marker became terminal history: %v", err)
	}
	if unit.NativeOperationsPending() {
		t.Fatal("exact settled writer retained pending custody")
	}
}

func TestCreatedNativeBindingRefusesCopiedOrClosedOwner(t *testing.T) {
	unit, _, _ := capturedModelOwners(t)
	copied := &CreatedUnit{self: unit}
	calls := 0
	bind := func(*CreatedUnit, int, int32) (NativeUnitOperations, error) {
		calls++
		return &nativeOperationsModel{}, nil
	}
	if _, err := copied.BindNativeOperations(context.Background(), unit.backend, bind); err == nil {
		t.Fatal("copied unit bound native writer")
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := unit.BindNativeOperations(context.Background(), unit.backend, bind); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed original bound writer: %v", err)
	}
	if calls != 0 {
		t.Fatalf("invalid owner invoked native producer %d times", calls)
	}
}

func TestCreatedNativeBindingPanicPublishesJoinedFailure(t *testing.T) {
	unit, _, _ := capturedModelOwners(t)
	cause := errors.New("native binder panicked")
	if _, err := unit.BindNativeOperations(context.Background(), unit.backend, func(*CreatedUnit, int, int32) (NativeUnitOperations, error) { panic(cause) }); !errors.Is(err, cause) {
		t.Fatalf("joined producer panic lost: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := unit.CloseNativeOperations(ctx); !errors.Is(err, cause) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("binding completion was not published: %v", err)
	}
}

type nativeOperationsSealModel struct{ calls int }

func (seal *nativeOperationsSealModel) NativeBirthTag() string   { return "explicit-logical-seal" }
func (seal *nativeOperationsSealModel) SealNativeRelease() error { seal.calls++; return nil }

// A regular-file lifecycle model proves ordering only. The fake descriptor 91
// below is never wrapped or passed to a native close; it is the retention port's
// recorder token. Existing genuine duplicate and native writer tests remain.
func TestCreatedNativeSessionReleasesTrackedSealHoldBeforeLocalSettlement(t *testing.T) {
	unit, _, cfg := capturedModelOwners(t)
	ops := productionSessionOps()
	ops.stampBirth = func(ppp.SessionIdentity, string) error { return nil }
	ops.duplicateFD = func(int) (int, error) { return 91, nil }
	duplicateClosed := false
	ops.closeFD = func(fd int) error {
		if fd != 91 {
			t.Errorf("changed private retained token: %d", fd)
		}
		duplicateClosed = true
		return nil
	}
	owner := capturedModelSession(t, cfg, ops)
	pending := errors.New("local pending ACK before exact retirement")
	writer := &nativeOperationsModel{pending: true}
	writer.sessionTestMutationOperations = sessionTestMutationOperations{backend: unit.backend, identity: unit.identity, ops: &ops}
	writer.settle = func(context.Context) error {
		if !duplicateClosed {
			t.Error("local writer became a prerequisite of physical duplicate close")
		}
		return pending
	}
	writer.close = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := unit.WaitNativePPPUnitRetired(ctx); err != nil {
			return err
		}
		writer.setPending(false)
		return nil
	}
	unit.mu.Lock()
	unit.nativeOperations.ownership = nativeUnitOperationsOwned{operations: writer}
	unit.mu.Unlock()
	owner.mutations = writer
	seal := &nativeOperationsSealModel{}
	if err := owner.BindNativeReleaseSeal(seal); err != nil {
		t.Fatal(err)
	}
	local := netip.MustParsePrefix("192.0.2.1/32")
	owner.addresses = map[netip.Prefix]sessionAddress{local: {peer: netip.MustParsePrefix("192.0.2.2/32"), state: sessionAddressRetiring}}
	if err := owner.Close(); err != nil {
		t.Fatalf("exact physical retirement failed to settle local writer: %v", err)
	}
	if !duplicateClosed || seal.calls != 1 {
		t.Fatalf("physical seal closure missing: %v %d", duplicateClosed, seal.calls)
	}
	if owner.NativeReleasePending() || unit.NativeOperationsPending() || len(owner.addresses) != 0 {
		t.Fatal("settled original kept unit-scoped pending custody")
	}
}
