// Design: docs/architecture/plugin/plugin-system.md -- selected provider ordering
// Related: register.go -- BGP starts its listeners during Configure

package plugin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestBGPStartupPlannerWaitsForSelectedInterfaces drives the actual registration
// through the startup tier planner. Every Configure in a tier may run before
// any other one completes, so the selected provider must finish an earlier
// tier. A BGP-only selection must still start without activating that provider.
func TestBGPStartupPlannerWaitsForSelectedInterfaces(t *testing.T) {
	require.NotNil(t, registry.Lookup(pluginNameBGP))
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprintf("interface-selected=%v", selected), func(t *testing.T) {
			names := []string{pluginNameBGP}
			if selected {
				names = append(names, "interface")
			}
			tiers, err := registry.TopologicalTiers(names, nil)
			require.NoError(t, err)
			providerReady := false
			bgpStarts := 0
			for _, tier := range tiers {
				for _, name := range tier {
					if name != pluginNameBGP {
						continue
					}
					if selected {
						require.True(t, providerReady, "BGP Configure can bind only after the selected interface provider has completed")
					}
					bgpStarts++
				}
				// Publication occurs only when the whole tier has completed.
				for _, name := range tier {
					if name == "interface" {
						providerReady = true
					}
				}
			}
			assert.Equal(t, 1, bgpStarts)
			assert.Equal(t, selected, providerReady, "an absent order-only target is not loaded")
		})
	}
}
