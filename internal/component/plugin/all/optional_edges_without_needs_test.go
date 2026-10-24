//go:build ze_bgp

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestOptionalEdgesWithoutNeedsPullAsBefore resolves the optional edges that
// declare no ConfigNeed against their real registrations.
//
// VALIDATES: spec-plugin-optional-dependency-refusal AC-17 and A-3. With a
// config that sets nothing, route server still pulls bgp-adj-rib-in, graceful
// restart still pulls bgp-rs, and static still pulls interface, each ordered
// before its owner: the conditional pull changes nothing for an edge no need
// names.
// PREVENTS: the conditional pull dropping an unconditional optional provider.
func TestOptionalEdgesWithoutNeedsPullAsBefore(t *testing.T) {
	emptyConfig := config.ConfigNeedHolds(map[string]any{})
	for _, edge := range []struct{ owner, provider string }{
		{owner: "bgp-rs", provider: "bgp-adj-rib-in"},
		{owner: "bgp-gr", provider: "bgp-rs"},
		{owner: "static", provider: "interface"},
	} {
		t.Run(edge.owner, func(t *testing.T) {
			reg := registry.Lookup(edge.owner)
			if reg == nil {
				t.Fatalf("composition root did not register %s", edge.owner)
			}
			if len(reg.ConfigNeeds) != 0 {
				t.Fatalf("%s now declares config needs %v; this test covers edges without one", edge.owner, reg.ConfigNeeds)
			}
			resolved, err := registry.ResolveDependencies([]string{edge.owner}, nil, emptyConfig)
			if err != nil {
				t.Fatalf("ResolveDependencies: %v", err)
			}
			if !slices.Contains(resolved, edge.provider) {
				t.Fatalf("%s no longer pulls %s: %v", edge.owner, edge.provider, resolved)
			}
			tiers, err := registry.TopologicalTiers(resolved, nil)
			if err != nil {
				t.Fatalf("TopologicalTiers: %v", err)
			}
			if tierIndex(tiers, edge.provider) >= tierIndex(tiers, edge.owner) {
				t.Fatalf("%s not ordered before %s: %v", edge.provider, edge.owner, tiers)
			}
		})
	}
}

// tierIndex answers the startup tier holding name, failing the lookup with -1
// so a missing name can never compare as ordered.
func tierIndex(tiers [][]string, name string) int {
	for index, tier := range tiers {
		if slices.Contains(tier, name) {
			return index
		}
	}
	return -1
}
