package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// VALIDATES: static's edge to bfd orders only. Selecting static alone does not
// select bfd, and static's startup resolves, in a build with the bfd plugin
// and in one without it.
//
// PREVENTS: the ordering edge turning into a dependency, which would load BFD
// for every static config or refuse static in a build with no BFD plugin.
func TestStaticDoesNotRequireBFD(t *testing.T) {
	// A config that sets nothing: static declares no ConfigNeed.
	names, err := registry.ResolveDependencies([]string{"static"}, nil, config.ConfigNeedHolds(map[string]any{}))
	if err != nil {
		t.Fatalf("ResolveDependencies: %v", err)
	}
	if slices.Contains(names, "bfd") {
		t.Fatalf("selecting static selected bfd: %v", names)
	}
	if _, err := registry.TopologicalTiers(names, nil); err != nil {
		t.Fatalf("TopologicalTiers: %v", err)
	}
}
