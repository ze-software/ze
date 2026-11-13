// Design: docs/architecture/plugin/plugin-system.md -- owner registries share the startup seal.

package registry

import (
	"errors"
	"testing"
)

// TestOwnerRegistrationUsesRegistrySeal proves owner publication is unchanged
// when refused, and shares the existing seal rather than a second lifecycle.
func TestOwnerRegistrationUsesRegistrySeal(t *testing.T) {
	snapshot := Snapshot()
	Reset()
	t.Cleanup(func() { Restore(snapshot) })
	publication := 0
	if err := WithRegistration("test extension", "owner", func() error { publication++; return nil }); err != nil {
		t.Fatal(err)
	}
	if publication != 1 {
		t.Fatalf("publication count = %d", publication)
	}
	refused := errors.New("invalid owner declaration")
	if err := WithRegistration("test extension", "owner", func() error { return refused }); !errors.Is(err, refused) {
		t.Fatalf("declaration refusal = %v", err)
	}
	if err := WithRegistration("test extension", "owner", nil); err == nil {
		t.Fatal("nil publication was accepted")
	}
	Seal()
	if err := WithRegistration("test extension", "owner", func() error { publication++; return nil }); !errors.Is(err, ErrRegistrySealed) {
		t.Fatalf("late owner publication = %v", err)
	}
	if publication != 1 {
		t.Fatal("sealed owner registry was mutated")
	}
}
