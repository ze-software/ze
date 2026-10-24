//go:build ze_dhcpserver && ze_pxe

package all

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// provisioningServers are the plugins that bind a socket on each configured
// listen-interface at their Stage 2 configure.
var provisioningServers = []string{"dhcpserver", "tftpserver", "imageserver"}

// startupTierOf answers the startup tier that holds name, or -1.
func startupTierOf(tiers [][]string, name string) int {
	for i, tier := range tiers {
		if slices.Contains(tier, name) {
			return i
		}
	}
	return -1
}

// VALIDATES: with the interface plugin selected, each provisioning server
// starts in a later tier than interface, so the VLAN, bridge or dummy device
// ze's own `interface {}` config creates in interface's Stage 2 configure
// exists before the server binds its listen-interface.
//
// PREVENTS: the server sharing tier 0 with, or starting before, interface,
// logging `listen failed` at boot and never serving that interface until the
// next reload.
func TestProvisioningServersStartAfterInterface(t *testing.T) {
	for _, server := range provisioningServers {
		t.Run(server, func(t *testing.T) {
			names, err := registry.ResolveDependencies([]string{server, "interface"}, nil, config.ConfigNeedHolds(map[string]any{}))
			if err != nil {
				t.Fatalf("ResolveDependencies: %v", err)
			}
			tiers, err := registry.TopologicalTiers(names, nil)
			if err != nil {
				t.Fatalf("TopologicalTiers: %v", err)
			}
			ifaceTier, serverTier := startupTierOf(tiers, "interface"), startupTierOf(tiers, server)
			if ifaceTier < 0 {
				t.Fatalf("interface in no tier: %v", tiers)
			}
			if serverTier <= ifaceTier {
				t.Fatalf("%s tier %d, interface tier %d: %s must start after interface (%v)",
					server, serverTier, ifaceTier, server, tiers)
			}
		})
	}
}

// VALIDATES: the edge to interface orders only. Selecting a provisioning
// server alone does not select interface, and its startup still resolves.
//
// PREVENTS: the ordering edge turning into a dependency, which would load the
// interface plugin for every config that runs a DHCP, TFTP or image server
// and holds no `interface` section.
func TestProvisioningServersDoNotRequireInterface(t *testing.T) {
	for _, server := range provisioningServers {
		t.Run(server, func(t *testing.T) {
			names, err := registry.ResolveDependencies([]string{server}, nil, config.ConfigNeedHolds(map[string]any{}))
			if err != nil {
				t.Fatalf("ResolveDependencies: %v", err)
			}
			if slices.Contains(names, "interface") {
				t.Fatalf("selecting %s selected interface: %v", server, names)
			}
			tiers, err := registry.TopologicalTiers(names, nil)
			if err != nil {
				t.Fatalf("TopologicalTiers: %v", err)
			}
			if startupTierOf(tiers, server) < 0 {
				t.Fatalf("%s in no tier: %v", server, tiers)
			}
		})
	}
}
