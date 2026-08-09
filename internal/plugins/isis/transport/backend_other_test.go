//go:build !linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- non-Linux backend stub test

package transport

import "testing"

func TestStubBackendOpenFails(t *testing.T) {
	// VALIDATES: on non-Linux the backend loads but OpenCircuit fails cleanly so
	// the component still works for config/unit tests without a privileged socket.
	be := NewBackend()
	if be == nil {
		t.Fatal("NewBackend returned nil")
	}
	if _, err := be.OpenCircuit("eth0"); err == nil {
		t.Fatal("expected OpenCircuit to fail on non-Linux, got nil error")
	}
}

func TestStubBackendThroughTransport(t *testing.T) {
	// VALIDATES: HandleLinkUp surfaces the backend open error; no circuit opens.
	tr := New(NewBackend())
	tr.EnableInterface("eth0", Level2)
	if err := tr.HandleLinkUp("eth0"); err == nil {
		t.Fatal("expected HandleLinkUp to fail on the stub backend")
	}
	if tr.CircuitOpen("eth0") {
		t.Fatal("circuit should not be open after a failed stub open")
	}
}
