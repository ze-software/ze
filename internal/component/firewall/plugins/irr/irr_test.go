package irr

// The firewall backends the tests program through: one that records what it
// was asked to program, one that panics. The tests of the reader that drives
// them are in stored_test.go.

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// recordingBackend is a firewall backend that keeps what it was asked to
// program. A pass applies the tables it built, so a test that does not
// install one reaches the OS default backend and the host's real ruleset.
type recordingBackend struct {
	mu      sync.Mutex
	applied [][]firewall.Table
}

func (b *recordingBackend) Apply(desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.applied = append(b.applied, desired)
	return nil
}

func (b *recordingBackend) ListTables() ([]firewall.Table, error) { return nil, nil }

func (b *recordingBackend) GetCounters(string) ([]firewall.ChainCounters, error) { return nil, nil }

func (b *recordingBackend) Close() error { return nil }

// last returns the tables of the most recent apply, and whether one happened.
func (b *recordingBackend) last() ([]firewall.Table, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.applied) == 0 {
		return nil, false
	}
	return b.applied[len(b.applied)-1], true
}

// setsByName indexes an applied table's sets so a test can assert on one by
// name. The order the plugin emits them in is not the point. What matters is
// which sets are declared, and whether the right one carries elements.
func setsByName(tbl firewall.Table) map[string]firewall.Set {
	byName := make(map[string]firewall.Set, len(tbl.Sets))
	for _, s := range tbl.Sets {
		byName[s.Name] = s
	}
	return byName
}

// useRecordingBackend makes this test's applies land in memory instead of the
// kernel. The backend name is the test's own, because a registration is global
// and refusing a duplicate is how the registry protects it, and the cleanup
// unregisters it, so a second pass under -count registers it again.
func useRecordingBackend(t *testing.T) *recordingBackend {
	t.Helper()
	b := &recordingBackend{}
	name := "recording-" + t.Name()
	if err := firewall.RegisterBackend(name, func() (firewall.Backend, error) { return b, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	// Registered first, so it runs after the CloseBackend cleanup below.
	t.Cleanup(func() {
		if err := firewall.UnregisterBackendForTest(name); err != nil {
			t.Errorf("UnregisterBackendForTest: %v", err)
		}
	})
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables("firewall-irr", nil)
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("CloseBackend: %v", err)
		}
	})
	return b
}

// panickingBackend panics instead of returning an error. The reader programs
// what it read through the backend, so this puts a panic on the pass's path,
// standing in for a defect in a set builder or a backend.
type panickingBackend struct{}

func (panickingBackend) Apply([]firewall.Table) error { panic("backend panic under apply") }

func (panickingBackend) ListTables() ([]firewall.Table, error) { return nil, nil }

func (panickingBackend) GetCounters(string) ([]firewall.ChainCounters, error) { return nil, nil }

func (panickingBackend) Close() error { return nil }

// usePanickingBackend installs it for this test alone, under the test's own
// name because a backend registration is global, and unregisters it after.
func usePanickingBackend(t *testing.T) {
	t.Helper()
	name := "panicking-" + t.Name()
	if err := firewall.RegisterBackend(name, func() (firewall.Backend, error) { return panickingBackend{}, nil }); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	// Registered first, so it runs after the CloseBackend cleanup below.
	t.Cleanup(func() {
		if err := firewall.UnregisterBackendForTest(name); err != nil {
			t.Errorf("UnregisterBackendForTest: %v", err)
		}
	})
	if err := firewall.LoadBackend(name); err != nil {
		t.Fatalf("LoadBackend: %v", err)
	}
	t.Cleanup(func() {
		_ = firewall.RegisterTables("firewall-irr", nil)
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("CloseBackend: %v", err)
		}
	})
}
