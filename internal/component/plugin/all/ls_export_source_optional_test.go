//go:build ze_bgp && ze_isis && ze_ospf

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestLSExportPullsOnlyTheIGPADomainNames resolves the BGP-LS exporter against
// its real registration in a build that carries IS-IS and OSPF.
//
// VALIDATES: spec-plugin-optional-dependency-refusal AC-13, full-build side.
// An export with no `domain`, or whose domains name only bgp-epe, pulls
// neither IGP; a domain naming isis pulls isis alone, and the startup tiers
// order isis before the exporter.
// PREVENTS: the optional edges starting every IGP in each build that exports
// BGP-LS, or the need through the `domain` list never pulling its provider.
// METHOD: config.ConfigNeedHolds over a lowered tree with the list shape
// Tree.toMap emits (list keyword, then entry key, then entry), handed to
// registry.ResolveDependencies as config.ExpandDependencies hands it.
func TestLSExportPullsOnlyTheIGPADomainNames(t *testing.T) {
	for _, name := range []string{"bgp-ls-export", "isis", "ospf"} {
		if !registry.Has(name) {
			t.Fatalf("composition root did not register %s", name)
		}
	}

	domains := func(sources map[string]string) map[string]any {
		entries := make(map[string]any, len(sources))
		for name, source := range sources {
			entries[name] = map[string]any{"source": source}
		}
		return map[string]any{"bgp-ls-export": map[string]any{"domain": entries}}
	}
	cases := []struct {
		name       string
		configTree map[string]any
		wantISIS   bool
	}{
		{name: "no domain", configTree: map[string]any{"bgp-ls-export": map[string]any{}}, wantISIS: false},
		{name: "epe domain only", configTree: domains(map[string]string{"edge": "bgp-epe"}), wantISIS: false},
		{name: "one domain names isis", configTree: domains(map[string]string{"edge": "bgp-epe", "core": "isis"}), wantISIS: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := registry.ResolveDependencies([]string{"bgp-ls-export"}, nil, config.ConfigNeedHolds(tc.configTree))
			if err != nil {
				t.Fatalf("ResolveDependencies: %v", err)
			}
			if slices.Contains(resolved, "ospf") {
				t.Fatalf("ospf pulled though no domain names it: %v", resolved)
			}
			if got := slices.Contains(resolved, "isis"); got != tc.wantISIS {
				t.Fatalf("isis pulled = %v, want %v: %v", got, tc.wantISIS, resolved)
			}
			if !tc.wantISIS {
				return
			}
			tiers, err := registry.TopologicalTiers(resolved, nil)
			if err != nil {
				t.Fatalf("TopologicalTiers: %v", err)
			}
			if tierIndex(tiers, "isis") >= tierIndex(tiers, "bgp-ls-export") {
				t.Fatalf("isis not ordered before bgp-ls-export: %v", tiers)
			}
		})
	}
}
