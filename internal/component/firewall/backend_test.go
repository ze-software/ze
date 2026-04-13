package firewall

import (
	"errors"
	"testing"
)

// VALIDATES: AC-6 "Backend interface registered: RegisterBackend/LoadBackend/GetBackend work".
// PREVENTS: backend registration lifecycle broken.
func TestBackendRegistration(t *testing.T) {
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
func TestBackendDuplicateRejected(t *testing.T) {
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
func TestBackendUnknownFails(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	if err := LoadBackend("nonexistent"); err == nil {
		t.Fatal("LoadBackend should fail for unknown backend")
	}
}

// VALIDATES: AC-7 "ListTables, GetCounters return data".
// PREVENTS: read methods not present on interface.
func TestBackendReadMethods(t *testing.T) {
	resetBackends()
	t.Cleanup(resetBackends)

	fb := &fakeBackend{
		tables: []Table{{Name: "wan", Family: FamilyInet}},
		counters: []ChainCounters{{
			Chain: "input",
			Terms: []TermCounter{{Name: "allow-ssh", Packets: 42, Bytes: 1234}},
		}},
	}
	if err := RegisterBackend("fake", func() (Backend, error) { return fb, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	if err := LoadBackend("fake"); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}

	b := GetBackend()

	tables, err := b.ListTables()
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if len(tables) != 1 || tables[0].Name != "wan" {
		t.Errorf("ListTables = %v, want [{Name:wan}]", tables)
	}

	counters, err := b.GetCounters("wan")
	if err != nil {
		t.Fatalf("GetCounters: %v", err)
	}
	if len(counters) != 1 || counters[0].Terms[0].Packets != 42 {
		t.Errorf("GetCounters = %v, want packets=42", counters)
	}
}

func TestCloseBackend(t *testing.T) {
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

func TestBackendFactoryError(t *testing.T) {
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
	tables   []Table
	counters []ChainCounters
	closed   bool
}

func (f *fakeBackend) Apply([]Table) error                         { return nil }
func (f *fakeBackend) ListTables() ([]Table, error)                { return f.tables, nil }
func (f *fakeBackend) GetCounters(string) ([]ChainCounters, error) { return f.counters, nil }
func (f *fakeBackend) Close() error                                { f.closed = true; return nil }
