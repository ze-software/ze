//go:build ze_flowexport && ze_bgp

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestFlowExportEnrichmentOffDoesNotPullRIB resolves flow export against its
// real registration in a full build.
//
// VALIDATES: spec-plugin-optional-dependency-refusal AC-5 and AC-6, R-1. Flow
// export configured with enrichment off, or with no enrichment block and no
// `bgp {}`, does not pull bgp-rib; with `enrichment { bgp true; }` it pulls
// bgp-rib and the startup tiers order bgp-rib before flow-export.
// PREVENTS: a bare optional edge starting the BGP RIB in every BGP build that
// runs flow export.
// METHOD: config.ConfigNeedHolds over the lowered tree each case describes,
// handed to registry.ResolveDependencies exactly as config.ExpandDependencies
// and boot auto-load hand it.
func TestFlowExportEnrichmentOffDoesNotPullRIB(t *testing.T) {
	for _, name := range []string{"flow-export", "bgp-rib"} {
		if !registry.Has(name) {
			t.Fatalf("composition root did not register %s", name)
		}
	}

	enrichment := func(bgp string) map[string]any {
		return map[string]any{"flow-export": map[string]any{"enrichment": map[string]any{"bgp": bgp}}}
	}
	cases := []struct {
		name       string
		configTree map[string]any
		wantRIB    bool
	}{
		{name: "no enrichment block", configTree: map[string]any{"flow-export": map[string]any{}}, wantRIB: false},
		{name: "enrichment bgp false", configTree: enrichment("false"), wantRIB: false},
		{name: "enrichment bgp true", configTree: enrichment("true"), wantRIB: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := registry.ResolveDependencies([]string{"flow-export"}, nil, config.ConfigNeedHolds(tc.configTree))
			if err != nil {
				t.Fatalf("ResolveDependencies: %v", err)
			}
			if got := slices.Contains(resolved, "bgp-rib"); got != tc.wantRIB {
				t.Fatalf("bgp-rib pulled = %v, want %v: %v", got, tc.wantRIB, resolved)
			}
			if !tc.wantRIB {
				return
			}
			tiers, err := registry.TopologicalTiers(resolved, nil)
			if err != nil {
				t.Fatalf("TopologicalTiers: %v", err)
			}
			if tierIndex(tiers, "bgp-rib") >= tierIndex(tiers, "flow-export") {
				t.Fatalf("bgp-rib not ordered before flow-export: %v", tiers)
			}
		})
	}
}
