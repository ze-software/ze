package ppp

import (
	"errors"
	"testing"
)

// VALIDATES: pppOps fields are settable in tests for fake injection,
//
//	matching the kernelOps pattern in internal/component/l2tp.
func TestPPPOpsFakeInjection(t *testing.T) {
	var calledFD int
	var calledMRU uint16
	ops := pppOps{
		setMRU: func(fd int, mru uint16) error {
			calledFD = fd
			calledMRU = mru
			return nil
		},
	}
	if err := ops.setMRU(42, 1456); err != nil {
		t.Fatalf("setMRU: %v", err)
	}
	if calledFD != 42 {
		t.Errorf("fd = %d, want 42", calledFD)
	}
	if calledMRU != 1456 {
		t.Errorf("mru = %d, want 1456", calledMRU)
	}
}

// VALIDATES: A fake setMRU's error propagates to the caller.
func TestPPPOpsErrorPropagates(t *testing.T) {
	want := errors.New("fake ioctl failure")
	ops := pppOps{
		setMRU: func(int, uint16) error { return want },
	}
	got := ops.setMRU(1, 1500)
	if !errors.Is(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// VALIDATES: newPPPOps wires the real implementation.
func TestNewPPPOpsBindsReal(t *testing.T) {
	ops := newPPPOps()
	if ops.setMRU == nil {
		t.Fatalf("newPPPOps did not bind setMRU")
	}
	// Cannot call realSetMRU here -- the production function does an
	// ioctl which requires a real /dev/ppp fd. The Manager (Phase 10)
	// will exercise this with a real fd in integration tests; ze's Go
	// unit test surface stops at "the wiring exists".
}
