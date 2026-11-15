//go:build ze_bgp

// Design: docs/architecture/testing/interop.md -- passive readiness uses the optional BGP resolver.
// Related: ze_ready_nobgp.go -- refusal when that resolver is compiled out.

package bgp

import (
	"fmt"
	"slices"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/config"
)

// zePassiveListenPorts resolves peer and dynamic-group inheritance through the
// same optional BGP resolver the daemon uses, then requires each passive port.
func zePassiveListenPorts(tree *config.Tree, name string) ([]string, error) {
	if tree.GetContainer("bgp") == nil {
		return nil, nil
	}
	resolved, err := bgpconfig.ResolveBGPTree(tree)
	if err != nil {
		return nil, fmt.Errorf("resolve %s for the Ze readiness probe: %w", name, err)
	}
	var ports []string
	if peers, ok := resolved["peer"].(map[string]any); ok {
		for peerName, entry := range peers {
			peer, isBlock := entry.(map[string]any)
			if !isBlock {
				return nil, fmt.Errorf("resolve %s for the Ze readiness probe: peer %s is not a block", name, peerName)
			}
			remote, hasRemote := config.BGPRemote(peer)
			if !hasRemote {
				continue
			}
			if remote["connect"] == "false" {
				ports = append(ports, zeListenPort(peer))
			}
		}
	}
	if groups, ok := resolved["dynamic-groups"].([]bgpconfig.DynamicGroupTemplate); ok {
		for index := range groups {
			ports = append(ports, zeListenPort(groups[index].Template))
		}
	}
	slices.Sort(ports)
	return slices.Compact(ports), nil
}
