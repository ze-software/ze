// IS-IS's registration reaches forwarding through a declaration, never a
// writer's name.
package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestISISRegistrationNeedsDataPlane proves IS-IS reaches forwarding through a
// declaration rather than a writer's name: it declares NeedsDataPlane, so the
// engine loads the writer of the configured data plane (fib-vpp on a VPP
// deployment) for its routes.
//
// VALIDATES: AC-16 -- IS-IS declares NeedsDataPlane, and no Dependencies entry
// is a registration that declares ProgramsFIB. This package links no FIB
// writer, so the cross-registry half is TestNoRegistrationDependsOnAFIBWriter
// (internal/component/plugin/all).
// METHOD: the registration IS-IS's init() handed the registry.
func TestISISRegistrationNeedsDataPlane(t *testing.T) {
	reg := registry.Lookup(componentName)
	if reg == nil {
		t.Fatal("isis is not registered")
	}
	if !reg.NeedsDataPlane {
		t.Fatal("isis does not declare NeedsDataPlane, so no writer is loaded for its routes")
	}
	for _, dep := range reg.Dependencies {
		if dep == "fib-kernel" || dep == "fib-vpp" {
			t.Errorf("isis Dependencies names the FIB writer %q; the data plane decides the writer", dep)
		}
		if other := registry.Lookup(dep); other != nil && other.ProgramsFIB {
			t.Errorf("isis Dependencies names %q, which programs a FIB", dep)
		}
	}
}
