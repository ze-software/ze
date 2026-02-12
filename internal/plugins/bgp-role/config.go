package bgp_role

import (
	"encoding/json"
	"fmt"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// peerRoleConfig holds per-peer role configuration.
type peerRoleConfig struct {
	role   string // role name: provider, rs, rs-client, customer, peer
	strict bool   // require peer to send Role capability
}

// extractPeerRoleConfigs parses BGP config JSON and returns per-peer role configs.
func extractPeerRoleConfigs(jsonStr string) map[string]*peerRoleConfig {
	var bgpConfig map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &bgpConfig); err != nil {
		logger().Warn("invalid JSON in bgp config", "err", err)
		return nil
	}

	// The config tree is wrapped: {"bgp": {"peer": {...}}}
	bgpSubtree, ok := bgpConfig["bgp"].(map[string]any)
	if !ok {
		bgpSubtree = bgpConfig
	}

	peersMap, ok := bgpSubtree["peer"].(map[string]any)
	if !ok {
		logger().Debug("no peer config in bgp tree")
		return nil
	}

	configs := make(map[string]*peerRoleConfig)

	for peerAddr, peerData := range peersMap {
		peerMap, ok := peerData.(map[string]any)
		if !ok {
			continue
		}

		capMap, ok := peerMap["capability"].(map[string]any)
		if !ok {
			continue
		}

		roleData, ok := capMap["role"].(map[string]any)
		if !ok {
			continue
		}

		roleName, ok := roleData["role"].(string)
		if !ok || roleName == "" {
			continue
		}

		// Validate role name
		if _, valid := roleValues[roleName]; !valid {
			logger().Warn("unknown role name", "peer", peerAddr, "role", roleName)
			continue
		}

		cfg := &peerRoleConfig{role: roleName}

		// Extract strict mode (default false)
		if strict, ok := roleData["role-strict"].(bool); ok {
			cfg.strict = strict
		}

		configs[peerAddr] = cfg
		logger().Debug("role config", "peer", peerAddr, "role", roleName, "strict", cfg.strict)
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
