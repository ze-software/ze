// Design: docs/architecture/core-design.md -- BGP role plugin
// RFC: rfc/short/rfc9234.md
// Overview: role.go -- role plugin entry point

package role

import (
	"fmt"
	"math"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// peerRoleConfig holds per-peer role configuration.
// The "import" keyword declares the local role and enables RFC 9234 ingress rules.
// The "export" keyword controls which destination peer roles may receive routes.
type peerRoleConfig struct {
	role   string   // role name from "import" keyword: provider, rs, rs-client, customer, peer
	strict bool     // require peer to send Role capability
	export []string // raw export tokens from config (e.g., ["default", "unknown"])
}

// RFC 9234 Section 5: default egress rules per local role.
// Maps local role to the set of destination peer roles that may receive routes.
var exportDefaults = map[string][]string{
	roleProvider: {roleCustomer, roleRSClient},
	roleCustomer: {roleProvider, roleRS, rolePeer},
	roleRS:       {roleRSClient},
	roleRSClient: {roleRS, roleProvider},
	rolePeer:     {roleCustomer, roleRSClient},
}

// resolveExport expands export tokens into the final set of allowed destination roles.
// "default" is expanded to the RFC 9234 Section 5 defaults for the local role.
// "unknown" is kept as-is (means: also send to peers with no role configured).
// Explicit role names are kept as-is.
func resolveExport(localRole string, exportTokens []string) []string {
	if len(exportTokens) == 0 {
		return nil
	}

	var result []string
	seen := make(map[string]bool)

	for _, token := range exportTokens {
		if token == "default" {
			for _, r := range exportDefaults[localRole] {
				if !seen[r] {
					seen[r] = true
					result = append(result, r)
				}
			}
		} else if !seen[token] {
			seen[token] = true
			result = append(result, token)
		}
	}

	return result
}

// parseRoleContainer extracts a peerRoleConfig from a role container map.
// The container has {"import": "<role-type>", "strict": true/false, "export": ...}.
// RFC 9234 Phase 2: "import" replaces the Phase 1 "name" keyword.
func parseRoleContainer(roleMap map[string]any) *peerRoleConfig {
	// RFC 9234 Phase 2: use "import" keyword (replaces "name").
	roleName, ok := roleMap["import"].(string)
	if !ok || roleName == "" {
		return nil
	}
	if _, valid := roleValues[roleName]; !valid {
		return nil
	}
	cfg := &peerRoleConfig{role: roleName}
	cfg.strict = parseBool(roleMap["strict"])
	cfg.export = parseExportTokens(roleMap["export"])
	return cfg
}

// parseExportTokens parses the "export" config value.
// Accepts a single string ("default") or an array of strings (["default", "unknown"]).
func parseExportTokens(v any) []string {
	if v == nil {
		return nil
	}
	// Single string token.
	if s, ok := v.(string); ok && s != "" {
		if !validExportTokens[s] {
			logger().Warn("unrecognized export token", "token", s)
		}
		return []string{s}
	}
	// Array of tokens (JSON unmarshal gives []interface{}).
	if arr, ok := v.([]any); ok {
		var tokens []string
		for _, item := range arr {
			s, ok := item.(string)
			if !ok || s == "" {
				logger().Warn("non-string export token ignored", "value", item)
				continue
			}
			if !validExportTokens[s] {
				logger().Warn("unrecognized export token", "token", s)
			}
			tokens = append(tokens, s)
		}
		if len(tokens) > 0 {
			return tokens
		}
	}
	return nil
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

// validExportTokens is the set of recognized export tokens.
var validExportTokens = map[string]bool{
	"default": true, roleUnknown: true,
	roleProvider: true, roleCustomer: true, rolePeer: true,
	roleRS: true, roleRSClient: true,
}

// extractRemoteIP extracts the remote IP address from a peer or group config map.
// Returns empty string if not found. Peer config takes precedence over group.
func extractRemoteIP(peerMap, groupMap map[string]any) string {
	for _, m := range []map[string]any{peerMap, groupMap} {
		if m == nil {
			continue
		}
		if remote, ok := m["remote"].(map[string]any); ok {
			if ip, ok := remote["ip"].(string); ok && ip != "" {
				return ip
			}
		}
	}
	return ""
}

// extractPeerRoleConfigs parses BGP config JSON and returns per-peer role configs
// and a name-to-IP mapping for resolving peer names to addresses.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
// Configs are keyed by IP address (from remote.ip) for filter lookups.
// Falls back to the config key (peer name) if no remote.ip is found.
func extractPeerRoleConfigs(jsonStr string) (map[string]*peerRoleConfig, map[string]string) {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("invalid JSON in bgp config")
		return nil, nil
	}

	configs := make(map[string]*peerRoleConfig)
	nameToIP := make(map[string]string)

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
		} else if peerMap != nil {
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

		// Key by IP address for filter lookups. Fall back to peer name.
		ip := extractRemoteIP(peerMap, groupMap)
		key := peerAddr
		if ip != "" {
			key = ip
			nameToIP[peerAddr] = ip
		}

		configs[key] = useCfg
		logger().Debug("role config", "peer", peerAddr, "ip", key, "role", useCfg.role, "strict", useCfg.strict)
	})

	if len(configs) == 0 {
		return nil, nil
	}

	return configs, nameToIP
}

// extractLocalASN extracts the local-as value from the BGP config JSON.
// Returns 0 if not found or not a valid number.
func extractLocalASN(jsonStr string) uint32 {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		logger().Warn("extractLocalASN: failed to parse BGP subtree")
		return 0
	}
	switch v := bgpSubtree["local-as"].(type) {
	case float64:
		if v < 0 || v > math.MaxUint32 {
			logger().Warn("extractLocalASN: local-as out of uint32 range", "value", v)
			return 0
		}
		return uint32(v)
	case int:
		if v < 0 || v > math.MaxUint32 {
			logger().Warn("extractLocalASN: local-as out of uint32 range", "value", v)
			return 0
		}
		return uint32(v)
	}
	return 0
}

// extractRoleCapabilities parses BGP config JSON and returns per-peer Role capabilities.
// RFC 9234 Section 4.1: Role capability code is 9.
func extractRoleCapabilities(jsonStr string) []sdk.CapabilityDecl {
	configs, _ := extractPeerRoleConfigs(jsonStr)
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
