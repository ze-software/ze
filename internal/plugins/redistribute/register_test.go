package redistribute

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestOrchestratorResolvesWithoutBGP proves that loading the orchestrator does
// not load BGP.
//
// VALIDATES: the `redistribute` root auto-loads redistribute-orchestrator, and
// resolving its dependencies for a config with no `bgp` block selects no `bgp`
// plugin, so `redistribute { destination isis { import static } }` boots on a
// router that runs no BGP.
// PREVENTS: the boot failure "plugin bgp failed during startup at stage
// Config: ... missing required bgp { } block", which a hard `bgp` dependency
// of the orchestrator produced.
//
// Method: ask the registry for the set boot would start for the orchestrator
// alone, with a needs predicate under which no config need holds (the config
// names no BGP destination), and require `bgp` absent from both the hard
// dependencies and the resolved set.
func TestOrchestratorResolvesWithoutBGP(t *testing.T) {
	reg := registry.Lookup(Name)
	if reg == nil {
		t.Fatalf("%s is not registered", Name)
	}
	if slices.Contains(reg.Dependencies, "bgp") {
		t.Fatalf("%s declares a hard dependency on bgp: %v", Name, reg.Dependencies)
	}
	noNeedHolds := func(string, registry.ConfigNeed) bool { return false }
	names, err := registry.ResolveDependencies([]string{Name}, nil, noNeedHolds)
	if err != nil {
		t.Fatalf("resolve %s: %v", Name, err)
	}
	if slices.Contains(names, "bgp") {
		t.Fatalf("resolving %s selects bgp: %v", Name, names)
	}
}
