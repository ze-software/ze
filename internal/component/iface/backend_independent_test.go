// Design: docs/features/interfaces.md -- independent backend owners
// Related: backend.go -- OpenBackend never replaces the configured owner

package iface

import (
	"errors"
	"testing"
)

type independentBackend struct {
	Backend
	closes int
}

func (b *independentBackend) Close() error {
	b.closes++
	return nil
}

// Keep an active owner with a known generation while opening a different
// backend. Both successful opens must be independent and individually closed.
func TestOpenBackendPreservesActiveOwner(t *testing.T) {
	active := installIndependentBackendFactory(t, func() (Backend, error) {
		return &independentBackend{}, nil
	})
	first, err := OpenBackend(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenBackend(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("separate opens returned the same owner")
	}
	assertIndependentBackendSelection(t, active)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	firstOwner, ok := first.(*independentBackend)
	if !ok {
		t.Fatalf("first owner has type %T, want *independentBackend", first)
	}
	secondOwner, ok := second.(*independentBackend)
	if !ok {
		t.Fatalf("second owner has type %T, want *independentBackend", second)
	}
	if firstOwner.closes != 1 || secondOwner.closes != 1 {
		t.Fatal("independent owners were not released exactly once")
	}
	assertIndependentBackendSelection(t, active)
}

// An unavailable factory must not replace, close or advance the active owner.
func TestOpenBackendFailurePreservesActiveOwner(t *testing.T) {
	failure := errors.New("independent factory refused")
	active := installIndependentBackendFactory(t, func() (Backend, error) { return nil, failure })
	owner, err := OpenBackend(t.Name())
	if owner != nil || !errors.Is(err, failure) {
		t.Fatalf("open = %v, %v", owner, err)
	}
	assertIndependentBackendSelection(t, active)
	owner, err = OpenBackend(t.Name() + "-unregistered")
	if owner != nil || err == nil {
		t.Fatalf("unknown factory = %v, %v", owner, err)
	}
	assertIndependentBackendSelection(t, active)
}

// A factory returning no owner cannot be reported as a successful acquisition.
func TestOpenBackendRejectsMissingOwner(t *testing.T) {
	active := installIndependentBackendFactory(t, func() (Backend, error) {
		return nil, nil //nolint:nilnil // This broken-factory result must be refused without replacing the active owner.
	})
	owner, err := OpenBackend(t.Name())
	if owner != nil || err == nil {
		t.Fatalf("missing owner = %v, %v", owner, err)
	}
	assertIndependentBackendSelection(t, active)
}

func installIndependentBackendFactory(t *testing.T, factory func() (Backend, error)) *independentBackend {
	t.Helper()
	active := &independentBackend{}
	backendsMu.Lock()
	previous, previousName, previousGeneration := activeBackend, activeBackendName, activeBackendGeneration
	activeBackend, activeBackendName, activeBackendGeneration = active, "independent-test-active", 73
	backendsMu.Unlock()
	t.Cleanup(func() {
		backendsMu.Lock()
		delete(backends, t.Name())
		activeBackend, activeBackendName, activeBackendGeneration = previous, previousName, previousGeneration
		backendsMu.Unlock()
	})
	if err := RegisterBackend(t.Name(), factory); err != nil {
		t.Fatal(err)
	}
	return active
}

func assertIndependentBackendSelection(t *testing.T, active *independentBackend) {
	t.Helper()
	backendsMu.Lock()
	defer backendsMu.Unlock()
	if activeBackend != active || activeBackendName != "independent-test-active" || activeBackendGeneration != 73 || active.closes != 0 {
		t.Fatalf("active owner changed: owner=%v name=%s generation=%d closes=%d", activeBackend, activeBackendName, activeBackendGeneration, active.closes)
	}
}
