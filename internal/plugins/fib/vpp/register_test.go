// VPP FIB register: init() side-effects. The package init() (register.go) must
// register the "fib-vpp" plugin into the shared plugin registry with its config
// root and dependencies, so the composition root discovers it.
package fibvpp

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestFibVPPRegistered(t *testing.T) {
	// VALIDATES: init() (register.go) wires "fib-vpp" into the plugin registry.
	// PREVENTS: a silently-unregistered backend that the composition root cannot find.
	reg := registry.Lookup("fib-vpp")
	if reg == nil {
		t.Fatal("fib-vpp not registered; init() did not run registry.Register")
	}
	if reg.Name != "fib-vpp" {
		t.Errorf("Name = %q, want fib-vpp", reg.Name)
	}
	if !slices.Contains(reg.ConfigRoots, "fib/vpp") {
		t.Errorf("ConfigRoots = %v, want to contain %q", reg.ConfigRoots, "fib/vpp")
	}
	for _, dep := range []string{"rib", "vpp"} {
		if !slices.Contains(reg.Dependencies, dep) {
			t.Errorf("Dependencies = %v, want to contain %q", reg.Dependencies, dep)
		}
	}
}
