// Design: docs/architecture/core-design.md -- BGP role plugin
// RFC: rfc/short/rfc9234.md

package role

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// peerRoleConfig holds per-peer role configuration.
type peerRoleConfig struct {
	role   string // role name: provider, rs, rs-client, customer, peer
	strict bool   // require peer to send Role capability
}

// parseRoleContainer extracts a peerRoleConfig from a role container map.
// The container has {"name": "<role-type>", "strict": true/false}.
func parseRoleContainer(roleMap map[string]any) *peerRoleConfig {
	roleName, ok := roleMap["name"].(string)
	if !ok || roleName == "" {
		return nil
	}
	if _, valid := roleValues[roleName]; !valid {
		return nil
	}
	cfg := &peerRoleConfig{role: roleName}
	cfg.strict = parseBool(roleMap["strict"])
	return cfg
}

// parseBool handles both JSON boolean (true) and config tree string ("true").
func parseBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s == "true"
	}
	return false
}

// parseRoleFromMap extracts a peerRoleConfig from a peer or group config map.
// Role is augmented directly on the peer/group node (not inside capability container).
func parseRoleFromMap(m map[string]any) *peerRoleConfig {
	if m == nil {
		return nil
	}
	roleMap, ok := m["role"].(map[string]any)
	if !ok {
		return nil
	}
	return parseRoleContainer(roleMap)
}

// extractPeerRoleConfigs parses BGP config JSON and returns per-peer role configs.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
// Role is a container with name (role-type enum) and strict (boolean) fields.
func extractPeerRoleConfigs(jsonStr string) map[string]*peerRoleConfig {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("invalid JSON in bgp config")
		return nil
	}

	configs := make(map[string]*peerRoleConfig)

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any) {
		// Check per-peer role config first.
		peerCfg := parseRoleFromMap(peerMap)

		// Check group-level role config (fallback).
		var groupCfg *peerRoleConfig
		if groupMap != nil {
			groupCfg = parseRoleFromMap(groupMap)
		}

		// Per-peer wins over group.
		useCfg := groupCfg
		if peerCfg != nil {
			useCfg = peerCfg
		} else if peerCfg == nil && peerMap != nil {
			// Peer has a config map but no role -- check if invalid role was present.
			if roleMap, hasRole := peerMap["role"].(map[string]any); hasRole {
				if parseRoleContainer(roleMap) == nil {
					logger().Warn("invalid role config", "peer", peerAddr)
				}
			}
		}

		if useCfg == nil {
			return
		}

		configs[peerAddr] = useCfg
		logger().Debug("role config", "peer", peerAddr, "role", useCfg.role, "strict", useCfg.strict)
	})

	if len(configs) == 0 {
		return nil
	}

	return configs
}

// extractRoleCapabilities parses BGP config JSON and returns per-peer Role capabilities.
// RFC 9234 Section 4.1: Role capability code is 9.
func extractRoleCapabilities(jsonStr string) []sdk.CapabilityDecl {
	configs := extractPeerRoleConfigs(jsonStr)
	if len(configs) == 0 {
		return nil
	}

	var caps []sdk.CapabilityDecl
	for peerAddr, cfg := range configs {
		value, ok := roleNameToValue(cfg.role)
		if !ok {
			continue
		}

		// RFC 9234 Section 4.1: capability value is single byte
		caps = append(caps, sdk.CapabilityDecl{
			Code:     roleCapCode,
			Encoding: "hex",
			Payload:  fmt.Sprintf("%02x", value),
			Peers:    []string{peerAddr},
		})
		logger().Debug("role capability", "peer", peerAddr, "role", cfg.role, "value", value)
	}

	return caps
}
