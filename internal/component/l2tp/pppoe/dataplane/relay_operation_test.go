// Design: docs/architecture/l2tp/followup-l2tp-call.md -- descriptor operation lifetime
// Related: session.go -- openedSession.WithDescriptor, openedSession.CloseTransport

package dataplane

import (
	"errors"
	"testing"
)

// TestRelayOperationExcludesTransportClose blocks inside the actual descriptor
// operation, proves it owns the transport lock, then joins both lifecycle calls.
// Closing afterward must prevent a second operation from receiving any descriptor.
func TestRelayOperationExcludesTransportClose(t *testing.T) {
	frames, handle, _ := testSessionResources()
	transport := &testRelayTransport{descriptor: 37}
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	original, ok := opened.(*openedSession)
	if !ok {
		t.Fatalf("constructor returned %T, want *openedSession", opened)
	}
	entered := make(chan int, 1)
	release := make(chan struct{})
	operationDone := make(chan error, 1)
	operationErr := errors.New("bridge operation failed")
	go func() {
		operationDone <- opened.WithDescriptor(func(descriptor int) error {
			entered <- descriptor
			<-release
			return operationErr
		})
	}()
	select {
	case descriptor := <-entered:
		if descriptor != 37 {
			t.Errorf("descriptor = %d, want 37", descriptor)
		}
	case err := <-operationDone:
		t.Fatalf("operation returned without lending its descriptor: %v", err)
	}
	if original.transportMu.TryLock() {
		original.transportMu.Unlock()
		t.Error("descriptor operation did not exclude transport close")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- opened.CloseTransport() }()
	close(release)
	if err := <-operationDone; !errors.Is(err, operationErr) {
		t.Errorf("operation error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Error(err)
	}
	if transport.closes != 1 {
		t.Errorf("transport closes = %d, want 1", transport.closes)
	}
	// The original numeric slot can now name another resource. No operation
	// may receive it after ownership ended, even if the owner reports it again.
	called := false
	if err := opened.WithDescriptor(func(int) error { called = true; return nil }); err == nil {
		t.Error("descriptor operation succeeded after transport close")
	}
	if called {
		t.Error("operation received a descriptor after transport close")
	}
	if transport.borrows != 1 {
		t.Errorf("owner borrows = %d, want 1", transport.borrows)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestRelayOperationRefusesUnavailableOwner supplies each unavailable transport
// state and proves the operation never runs or loses the owner's refusal.
func TestRelayOperationRefusesUnavailableOwner(t *testing.T) {
	ownerErr := errors.New("relay owner failed")
	for _, test := range []struct {
		name       string
		descriptor int
		failure    error
	}{
		{"owner failure", 37, ownerErr}, {"invalid descriptor", -1, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			frames, handle, _ := testSessionResources()
			transport := &testRelayTransport{descriptor: test.descriptor, descriptorErr: test.failure}
			opened, err := NewOpenedSession(frames, handle, transport)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			err = opened.WithDescriptor(func(int) error { called = true; return nil })
			if err == nil {
				t.Fatal("unavailable owner allowed a descriptor operation")
			}
			if called {
				t.Fatal("unavailable owner invoked the operation")
			}
			if test.failure != nil {
				if !errors.Is(err, test.failure) {
					t.Errorf("owner refusal lost: %v", err)
				}
			}
			if err := opened.WithDescriptor(nil); err == nil {
				t.Error("nil operation was accepted")
			}
			closeTestSession(t, opened)
		})
	}
	t.Run("unsupported transport", func(t *testing.T) {
		frames, handle, transport := testSessionResources()
		opened, err := NewOpenedSession(frames, handle, transport)
		if err != nil {
			t.Fatal(err)
		}
		called := false
		if err := opened.WithDescriptor(func(int) error { called = true; return nil }); err == nil {
			t.Fatal("unsupported owner allowed an operation")
		}
		if called {
			t.Fatal("unsupported owner invoked the operation")
		}
		closeTestSession(t, opened)
	})
}
