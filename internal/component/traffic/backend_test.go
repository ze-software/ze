package traffic

import (
	"context"
	"errors"
	"testing"
)

// VALIDATES: AC-6 "Backend interface registered: RegisterBackend/LoadBackend/GetBackend work".
// PREVENTS: backend registration lifecycle broken.
func TestTrafficBackendRegistration(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	called := false
	err := RegisterBackend("test", func() (Backend, error) {
		called = true
		return &fakeBackend{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}

	if err := LoadBackend("test"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	if !called {
		t.Fatal("factory not called")
	}

	b := GetBackend()
	if b == nil {
		t.Fatal("GetBackend returned nil")
	}
}

// VALIDATES: AC-6 duplicate backend name rejected.
// PREVENTS: silent overwrite of registered backends.
func TestTrafficBackendDuplicateRejected(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	factory := func() (Backend, error) { return &fakeBackend{}, nil }
	if err := RegisterBackend("dup", factory); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := RegisterBackend("dup", factory); err == nil {
		t.Fatal("duplicate register should fail")
	}
}

// VALIDATES: AC-6 unknown backend name returns error.
// PREVENTS: nil pointer from loading unregistered backend.
func TestTrafficBackendUnknownFails(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	if err := LoadBackend("nonexistent"); err == nil {
		t.Fatal("LoadBackend should fail for unknown backend")
	}
}

// VALIDATES: AC-7 "ListQdiscs returns data".
// PREVENTS: read method not present on interface.
func TestTrafficBackendReadMethods(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	fb := &fakeBackend{
		qos: InterfaceQoS{
			Interface: "eth0",
			Qdisc:     Qdisc{Type: QdiscHTB},
		},
	}
	if err := RegisterBackend("fake", func() (Backend, error) { return fb, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("fake"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}

	b := GetBackend()
	qos, err := b.ListQdiscs("eth0")
	if err != nil {
		t.Fatalf("ListQdiscs: %v", err)
	}
	if qos.Interface != "eth0" {
		t.Errorf("ListQdiscs().Interface = %q, want %q", qos.Interface, "eth0")
	}
}

func TestTrafficCloseBackend(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	fb := &fakeBackend{}
	if err := RegisterBackend("closeable", func() (Backend, error) { return fb, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("closeable"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}

	if err := CloseBackend(); err != nil {
		t.Fatalf("CloseBackend: %v", err)
	}
	if !fb.closed {
		t.Fatal("backend Close not called")
	}
	if GetBackend() != nil {
		t.Fatal("GetBackend should be nil after CloseBackend")
	}
}

func TestTrafficBackendFactoryError(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	if err := RegisterBackend("broken", func() (Backend, error) {
		return nil, errors.New("init failed")
	}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}

	if err := LoadBackend("broken"); err == nil {
		t.Fatal("LoadBackend should fail when factory returns error")
	}
}

// resetBackends clears the backend registry for test isolation.
func resetBackends() {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	backends = map[string]func() (Backend, error){}
	activeBackend = nil
}

// fakeBackend implements Backend for testing.
type fakeBackend struct {
	qos    InterfaceQoS
	closed bool
}

func (f *fakeBackend) Apply(context.Context, map[string]InterfaceQoS) error { return nil }
func (f *fakeBackend) ListQdiscs(string) (InterfaceQoS, error)              { return f.qos, nil }
func (f *fakeBackend) Close() error                                         { f.closed = true; return nil }
