//go:build ze_bfd

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// tierOf answers the startup tier that holds name, or -1.
func tierOf(tiers [][]string, name string) int {
	for i, tier := range tiers {
		if slices.Contains(tier, name) {
			return i
		}
	}
	return -1
}

// VALIDATES: with both selected, static starts in a later tier than bfd, so
// static's OnStarted finds the BFD service bfd publishes in its own OnStarted.
//
// PREVENTS: static and bfd landing in one tier, static's one GetService at
// OnStarted answering nil, and every static next-hop running without BFD
// ("static: BFD service not available, running without BFD").
func TestStaticStartsAfterBFD(t *testing.T) {
	names, err := registry.ResolveDependencies([]string{"static", "bfd"}, nil, config.ConfigNeedHolds(map[string]any{}))
	if err != nil {
		t.Fatalf("ResolveDependencies: %v", err)
	}
	tiers, err := registry.TopologicalTiers(names, nil)
	if err != nil {
		t.Fatalf("TopologicalTiers: %v", err)
	}
	bfdTier, staticTier := tierOf(tiers, "bfd"), tierOf(tiers, "static")
	if bfdTier < 0 {
		t.Fatalf("bfd in no tier: %v", tiers)
	}
	if staticTier <= bfdTier {
		t.Fatalf("static tier %d, bfd tier %d: static must start after bfd (%v)", staticTier, bfdTier, tiers)
	}
}
