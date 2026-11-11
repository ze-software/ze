// OSPF's registration reaches forwarding through a declaration, never a
// writer's name.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestOSPFRegistrationNeedsDataPlane proves OSPF reaches forwarding through a
// declaration rather than a writer's name: it declares NeedsDataPlane, so the
// engine loads the writer of the configured data plane (fib-vpp on a VPP
// deployment) for its routes and its SR label entries.
//
// VALIDATES: AC-16 -- OSPF declares NeedsDataPlane, and no Dependencies entry
// is a registration that declares ProgramsFIB. This package links no FIB
// writer, so the cross-registry half is TestNoRegistrationDependsOnAFIBWriter
// (internal/component/plugin/all).
// METHOD: the registration OSPF's init() handed the registry.
func TestOSPFRegistrationNeedsDataPlane(t *testing.T) {
	reg := registry.Lookup(Namespace)
	if reg == nil {
		t.Fatal("ospf is not registered")
	}
	if !reg.NeedsDataPlane {
		t.Fatal("ospf does not declare NeedsDataPlane, so no writer is loaded for its routes and SR labels")
	}
	for _, dep := range reg.Dependencies {
		if dep == "fib-kernel" || dep == "fib-vpp" {
			t.Errorf("ospf Dependencies names the FIB writer %q; the data plane decides the writer", dep)
		}
		if other := registry.Lookup(dep); other != nil && other.ProgramsFIB {
			t.Errorf("ospf Dependencies names %q, which programs a FIB", dep)
		}
	}
}
