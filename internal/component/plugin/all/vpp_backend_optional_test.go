//go:build ze_vpp

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestVPPBackendNeedPullsVPPOnlyWhenSelected resolves the three VPP backend
// owners against their real registrations in a build that carries ze_vpp.
//
// VALIDATES: spec-plugin-optional-dependency-refusal AC-7, AC-8, AC-9, full
// build side. traffic, interface and firewall pull the vpp component only
// when their backend leaf carries vpp. With vpp selected on its own account
// (a `vpp {}` root) and the default backend, each owner is still ordered
// after vpp, which keeps the order firewall's former StartAfter gave.
// PREVENTS: traffic pulling vpp for every tc config, as its bare optional
// edge did, and a backend need that never pulls vpp for a config selecting it.
// METHOD: config.ConfigNeedHolds over the lowered tree each case describes,
// handed to registry.ResolveDependencies exactly as config.ExpandDependencies
// and boot auto-load hand it.
func TestVPPBackendNeedPullsVPPOnlyWhenSelected(t *testing.T) {
	if !registry.Has("vpp") {
		t.Fatal("composition root did not register vpp")
	}

	owners := []struct {
		owner      string
		containers []string
		defaultsTo string
	}{
		{owner: "traffic", containers: []string{"traffic", "control"}, defaultsTo: "tc"},
		{owner: "interface", containers: []string{"interface"}, defaultsTo: "netlink"},
		{owner: "firewall", containers: []string{"firewall"}, defaultsTo: "nft"},
	}
	for _, o := range owners {
		t.Run(o.owner, func(t *testing.T) {
			if !registry.Has(o.owner) {
				t.Fatalf("composition root did not register %s", o.owner)
			}
			for backend, wantVPP := range map[string]bool{o.defaultsTo: false, "vpp": true} {
				needHolds := config.ConfigNeedHolds(backendTree(backend, o.containers))
				resolved, err := registry.ResolveDependencies([]string{o.owner}, nil, needHolds)
				if err != nil {
					t.Fatalf("backend %s: ResolveDependencies: %v", backend, err)
				}
				if got := slices.Contains(resolved, "vpp"); got != wantVPP {
					t.Fatalf("backend %s: vpp pulled = %v, want %v: %v", backend, got, wantVPP, resolved)
				}
			}

			// The default backend with vpp selected by its own root: the
			// edge orders whether or not the need holds.
			tiers, err := registry.TopologicalTiers([]string{o.owner, "vpp"}, nil)
			if err != nil {
				t.Fatalf("TopologicalTiers: %v", err)
			}
			if vppTierIndex(t, tiers, "vpp") >= vppTierIndex(t, tiers, o.owner) {
				t.Fatalf("vpp not ordered before %s: %v", o.owner, tiers)
			}
		})
	}
}

// backendTree answers the lowered form of a backend leaf set to backend under
// containers: {containers[0]: {containers[1]: {"backend": backend}}}.
func backendTree(backend string, containers []string) map[string]any {
	node := map[string]any{"backend": backend}
	for _, container := range slices.Backward(containers) {
		node = map[string]any{container: node}
	}
	return node
}

// vppTierIndex answers the startup tier holding name, failing t when no tier
// holds it.
func vppTierIndex(t *testing.T, tiers [][]string, name string) int {
	t.Helper()
	for i, tier := range tiers {
		if slices.Contains(tier, name) {
			return i
		}
	}
	t.Fatalf("%s is in no tier: %v", name, tiers)
	return -1
}
