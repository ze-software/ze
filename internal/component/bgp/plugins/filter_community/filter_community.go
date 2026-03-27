// Design: docs/architecture/core-design.md — community filter plugin
// Detail: config.go — config parsing for community definitions and filter rules
// Detail: filter.go — ingress filter (direct payload mutation)
// Detail: egress.go — egress filter (ModAccumulator ops)
// Detail: handler.go — AttrModHandlers for progressive build

// Package filter_community implements the bgp-filter-community plugin.
// It allows operators to tag and strip BGP communities on ingress and egress
// using named community definitions and cumulative filter rules.
package filter_community

import (
	"context"
	"fmt"
	"net"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var logger = slogutil.LazyLogger("filter-community")

// state holds the plugin's runtime state, populated via OnConfigure callback.
// Protected by mu for concurrent access from filter closures.
var (
	mu          sync.RWMutex
	definitions communityDefs
	peerConfigs map[string]filterConfig // keyed by peer name
)

// RunFilterCommunity runs the community filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunFilterCommunity(conn net.Conn) int {
	p := sdk.NewWithConn("filter-community", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return fmt.Errorf("filter-community: invalid bgp config JSON")
			}
			if err := configureCommunityFilter(bgpCfg); err != nil {
				return fmt.Errorf("filter-community: %w", err)
			}
		}
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	}); err != nil {
		logger().Error("filter-community plugin failed", "error", err)
		return 1
	}

	return 0
}

// configureCommunityFilter parses community definitions and per-peer filter configs
// from the raw BGP config subtree, accumulating filter tag/strip lists across
// bgp-level, group-level, and peer-level (same inheritance as ResolveBGPTree).
func configureCommunityFilter(bgpCfg map[string]any) error {
	defs, err := parseCommunityDefinitions(bgpCfg)
	if err != nil {
		return fmt.Errorf("community definitions: %w", err)
	}

	// BGP-level filter config (applies to all peers as base).
	bgpFilter := parseFilterConfig(bgpCfg)

	// Parse per-peer filter configs, accumulating bgp + group + peer levels.
	configs := make(map[string]filterConfig)
	configjson.ForEachPeer(bgpCfg, func(peerName string, peerMap, groupMap map[string]any) {
		// Layer 1: BGP-level defaults.
		fc := bgpFilter

		// Layer 2: Group-level (if peer is in a group).
		if groupMap != nil {
			groupFilter := parseFilterConfig(groupMap)
			fc = mergeFilterConfigs(fc, groupFilter)
		}

		// Layer 3: Peer-level (highest precedence).
		if peerMap != nil {
			peerFilter := parseFilterConfig(peerMap)
			fc = mergeFilterConfigs(fc, peerFilter)
		}

		if len(fc.ingressTag) == 0 && len(fc.ingressStrip) == 0 &&
			len(fc.egressTag) == 0 && len(fc.egressStrip) == 0 {
			return
		}

		configs[peerName] = fc
	})

	// Validate all referenced community names exist.
	for peerName, fc := range configs {
		for _, refs := range [][]string{fc.ingressTag, fc.ingressStrip, fc.egressTag, fc.egressStrip} {
			if err := validateCommunityRefs(defs, refs); err != nil {
				return fmt.Errorf("peer %s: %w", peerName, err)
			}
		}
	}

	mu.Lock()
	definitions = defs
	peerConfigs = configs
	mu.Unlock()

	logger().Debug("configured",
		"definitions", len(defs),
		"peers-with-filters", len(configs),
	)

	return nil
}

// ingressFilter is the registered IngressFilterFunc.
// Looks up the source peer's filter config and applies strip then tag.
func ingressFilter(src registry.PeerFilterInfo, payload []byte, _ map[string]any) (bool, []byte) {
	mu.RLock()
	defs := definitions
	fc, hasCfg := peerConfigs[src.Name]
	mu.RUnlock()

	if !hasCfg {
		return true, nil
	}

	modified := applyIngressFilter(payload, defs, fc)
	return true, modified
}

// egressFilter is the registered EgressFilterFunc.
// Looks up the destination peer's filter config and accumulates ops.
func egressFilter(_, dest registry.PeerFilterInfo, _ []byte, _ map[string]any, mods *registry.ModAccumulator) bool {
	mu.RLock()
	defs := definitions
	fc, hasCfg := peerConfigs[dest.Name]
	mu.RUnlock()

	if !hasCfg {
		return true
	}

	applyEgressFilter(defs, fc, mods)
	return true // Community filter never suppresses routes.
}
