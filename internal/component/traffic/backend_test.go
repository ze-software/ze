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

// TestReadQdiscsUsesActiveBackendElseDefaultReader verifies ReadQdiscs and
// ReadAllQdiscs prefer the active backend, fall back to the default read-only
// reader when no backend is loaded, and return ErrNoBackendAvailable otherwise.
//
// VALIDATES: AC-3 (read-only fallback when unconfigured, active backend
// precedence when configured, clean sentinel error when neither exists).
// PREVENTS: broken readback on unshaped Linux routers or bypassing an active
// VPP/kernel backend.
func TestReadQdiscsUsesActiveBackendElseDefaultReader(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	if err := RegisterReader("", &fakeReader{}); err == nil {
		t.Fatal("RegisterReader accepted empty name")
	}
	if err := RegisterReader("nil-reader", nil); err == nil {
		t.Fatal("RegisterReader accepted nil reader")
	}

	if _, err := ReadQdiscs("eth0"); !errors.Is(err, ErrNoBackendAvailable) {
		t.Fatalf("ReadQdiscs without backend/reader err = %v, want ErrNoBackendAvailable", err)
	}
	if _, err := ReadAllQdiscs(); !errors.Is(err, ErrNoBackendAvailable) {
		t.Fatalf("ReadAllQdiscs without backend/reader err = %v, want ErrNoBackendAvailable", err)
	}

	const fallbackIface = "eth-fallback"
	const activeIface = "eth-active"
	const fallbackKind = "fq_codel"

	if defaultBackendName != "" {
		fr := &fakeReader{
			single: InterfaceQoS{Interface: fallbackIface, Qdisc: Qdisc{Kind: fallbackKind}},
			all:    []InterfaceQoS{{Interface: fallbackIface, Qdisc: Qdisc{Kind: fallbackKind}}},
		}
		if err := RegisterReader(defaultBackendName, fr); err != nil {
			t.Fatalf("RegisterReader(%q): %v", defaultBackendName, err)
		}
		if err := RegisterReader(defaultBackendName, fr); err == nil {
			t.Fatal("RegisterReader accepted duplicate name")
		}

		got, err := ReadQdiscs(fallbackIface)
		if err != nil {
			t.Fatalf("ReadQdiscs via default reader: %v", err)
		}
		if got.Interface != fallbackIface {
			t.Fatalf("ReadQdiscs via default reader Interface = %q, want %q", got.Interface, fallbackIface)
		}
		if got.Qdisc.KindName() != fallbackKind {
			t.Fatalf("ReadQdiscs via default reader KindName = %q, want %q", got.Qdisc.KindName(), fallbackKind)
		}

		all, err := ReadAllQdiscs()
		if err != nil {
			t.Fatalf("ReadAllQdiscs via default reader: %v", err)
		}
		if len(all) != 1 {
			t.Fatalf("ReadAllQdiscs via default reader len = %d, want 1", len(all))
		}
		if all[0].Interface != fallbackIface {
			t.Fatalf("ReadAllQdiscs via default reader[0].Interface = %q, want %q", all[0].Interface, fallbackIface)
		}
	}

	// Active backend without QdiscReader: ReadQdiscs uses active backend;
	// ReadAllQdiscs returns a clear non-ErrNoBackendAvailable error.
	plainActive := &fakeBackend{qos: InterfaceQoS{Interface: activeIface, Qdisc: Qdisc{Type: QdiscHTB}}}
	if err := RegisterBackend("plain", func() (Backend, error) { return plainActive, nil }); err != nil {
		t.Fatalf("RegisterBackend(plain): %v", err)
	}
	if err := LoadBackend("plain"); err != nil {
		t.Fatalf("LoadBackend(plain): %v", err)
	}

	got, err := ReadQdiscs(activeIface)
	if err != nil {
		t.Fatalf("ReadQdiscs via active plain backend: %v", err)
	}
	if got.Interface != activeIface {
		t.Fatalf("ReadQdiscs via active plain backend Interface = %q, want %q", got.Interface, activeIface)
	}
	if got.Qdisc.Type != QdiscHTB {
		t.Fatalf("ReadQdiscs via active plain backend Type = %v, want %v", got.Qdisc.Type, QdiscHTB)
	}
	_, bulkErr := ReadAllQdiscs()
	if bulkErr == nil {
		t.Fatal("ReadAllQdiscs via non-reader backend returned nil error, want error")
	}
	if errors.Is(bulkErr, ErrNoBackendAvailable) {
		t.Fatalf("ReadAllQdiscs via non-reader backend err = %v, must not be ErrNoBackendAvailable", bulkErr)
	}

	// Active backend implementing QdiscReader: both ReadQdiscs and ReadAllQdiscs
	// prefer the active backend over the default reader.
	bulkActive := &fakeBulkBackend{
		qos: InterfaceQoS{Interface: activeIface, Qdisc: Qdisc{Type: QdiscTBF}},
		all: []InterfaceQoS{{Interface: activeIface, Qdisc: Qdisc{Type: QdiscTBF}}},
	}
	if err := RegisterBackend("bulk", func() (Backend, error) { return bulkActive, nil }); err != nil {
		t.Fatalf("RegisterBackend(bulk): %v", err)
	}
	if err := LoadBackend("bulk"); err != nil {
		t.Fatalf("LoadBackend(bulk): %v", err)
	}

	all, err := ReadAllQdiscs()
	if err != nil {
		t.Fatalf("ReadAllQdiscs via active bulk backend: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ReadAllQdiscs via active bulk backend len = %d, want 1", len(all))
	}
	if all[0].Interface != activeIface {
		t.Fatalf("ReadAllQdiscs via active bulk backend[0].Interface = %q, want %q", all[0].Interface, activeIface)
	}

	// Closing active backend restores fallback to default reader.
	if err := CloseBackend(); err != nil {
		t.Fatalf("CloseBackend: %v", err)
	}
	if defaultBackendName != "" {
		allAfterClose, err := ReadAllQdiscs()
		if err != nil {
			t.Fatalf("ReadAllQdiscs after CloseBackend: %v", err)
		}
		if len(allAfterClose) != 1 {
			t.Fatalf("ReadAllQdiscs after CloseBackend len = %d, want 1", len(allAfterClose))
		}
		if allAfterClose[0].Interface != fallbackIface {
			t.Fatalf("ReadAllQdiscs after CloseBackend[0].Interface = %q, want %q", allAfterClose[0].Interface, fallbackIface)
		}
	}
}

// resetBackends clears the backend and reader registries for test isolation.
func resetBackends() {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	backends = map[string]func() (Backend, error){}
	readers = map[string]QdiscReader{}
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

// fakeBulkBackend implements Backend and QdiscReader for testing.
type fakeBulkBackend struct {
	qos InterfaceQoS
	all []InterfaceQoS
}

func (f *fakeBulkBackend) Apply(context.Context, map[string]InterfaceQoS) error { return nil }
func (f *fakeBulkBackend) ListQdiscs(string) (InterfaceQoS, error)              { return f.qos, nil }
func (f *fakeBulkBackend) ListAllQdiscs() ([]InterfaceQoS, error)               { return f.all, nil }
func (f *fakeBulkBackend) Close() error                                         { return nil }

// fakeReader implements QdiscReader for testing.
type fakeReader struct {
	single InterfaceQoS
	all    []InterfaceQoS
}

func (f *fakeReader) ListQdiscs(string) (InterfaceQoS, error) { return f.single, nil }
func (f *fakeReader) ListAllQdiscs() ([]InterfaceQoS, error)  { return f.all, nil }
