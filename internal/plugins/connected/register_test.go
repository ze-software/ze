// Design: docs/architecture/core-design.md -- connected plugin registration
// Related: register.go -- plugin registration

package connected

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestConnectedDeclaresOptionalInterfaceDependency verifies that the connected
// plugin declares "interface" as an unconditional optional dependency.
//
// VALIDATES: connected declares "interface" in OptionalDependencies (with no
// ConfigNeeds) and not in Dependencies, so loading connected pulls the
// interface provider and orders it before connected even when no `interface {}`
// stanza is configured, while a build without the interface plugin still runs
// connected.
// PREVENTS: a `connected {}` + `bgp {}` configuration without `interface {}` or
// `static {}` loading no interface monitor, leaving connected subnets out of
// Loc-RIB and direct BGP next hops unresolvable.
func TestConnectedDeclaresOptionalInterfaceDependency(t *testing.T) {
	reg := registry.Lookup(pluginName)
	if reg == nil {
		t.Fatal("connected plugin not registered")
	}
	if !slices.Contains(reg.OptionalDependencies, "interface") {
		t.Errorf("optional dependencies = %v, want to include \"interface\"", reg.OptionalDependencies)
	}
	if slices.Contains(reg.Dependencies, "interface") {
		t.Error("\"interface\" is a hard dependency; it must be optional so builds without the interface plugin still load connected")
	}
	if len(reg.ConfigNeeds) != 0 {
		t.Errorf("config needs = %v, want none so an empty config pulls \"interface\" whenever connected loads", reg.ConfigNeeds)
	}
}
