// Design: docs/architecture/core-design.md -- BGP role plugin
// RFC: rfc/short/rfc9234.md
// Overview: role.go -- role plugin entry point

package role

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// dynamicPeerIP is the group-level placeholder that ze-bgp-conf.yang allows in
// connection > remote > ip to mean "accept sessions from any address in the
// group's range". It is never a peer address: reactor/config.go:79-81 rejects
// it at peer level, and dynamically created peers carry their real address.
const dynamicPeerIP = "dynamic"

// peerRoleConfig holds per-peer role configuration.
// The "import" keyword declares the local role and enables RFC 9234 ingress rules.
// The "export" keyword controls which destination peer roles may receive routes.
type peerRoleConfig struct {
	role           string   // role name from "import" keyword: provider, rs, rs-client, customer, peer
	strict         bool     // require peer to send Role capability
	export         []string // raw export tokens from config (e.g., ["default", "unknown"])
	resolvedExport []string // pre-computed expanded export set (resolved at config time, avoids hot-path allocation)
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
	cfg.resolvedExport = resolveExport(cfg.role, cfg.export)
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

// parseBool handles both JSON boolean (true) and the config tree's string
// ("true") form -- the framework delivers YANG leaf values as strings.
func parseBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
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

		// Key by IP address, because that is the only key the runtime readers
		// use: all three getFilterConfig callers pass PeerFilterInfo.Address
		// (otc.go, OTCIngressFilter and both OTCEgressFilter lookups).
		// configjson.PeerRemoteIP reads connection>remote>ip (the delivered shape); role's old
		// local reader used the stale flat remote/ip path and silently returned "" on real config.
		ip := configjson.PeerRemoteIP(peerMap, groupMap)

		// "dynamic" is a group-level placeholder, never a peer address. It is
		// non-empty, so it used to sail past the empty check and be stored under
		// the literal key "dynamic" -- a key no reader can ever produce, since
		// every reader passes PeerFilterInfo.Address.String(). Reject it rather
		// than manufacture an unreachable entry, matching bgp-rpki
		// (plugins/rpki/rpki_config.go:283-287).
		if ip == dynamicPeerIP {
			logger().Warn("role config ignored: peer has no usable remote ip",
				"peer", peerAddr, "remote-ip", ip,
				"effect", "no Role capability is advertised and no RFC 9234 OTC gate runs for this peer",
				"fix", "set connection > remote > ip to a literal address on the peer, not the group placeholder")
			return
		}

		key := ip
		if key == "" {
			// No connection > remote > ip anywhere. The delivered map key is the
			// peer NAME, and operators very commonly name a peer by its own
			// address -- in which case the name IS the key every reader uses, so
			// the fallback resolves correctly and must be kept.
			//
			// When the name is not an address, it cannot be: the readers only
			// ever look up PeerFilterInfo.Address.String(). Storing it anyway
			// produced config nothing could find, and a nil cfg sends the
			// RFC 9234 Section 5 gates down their permissive branch -- the
			// zero-value trap of ai/rules/fail-closed-guards.md, where a miss is
			// indistinguishable from "this peer has no role configured". Such a
			// peer also never establishes (reactor/config.go:76-78 fails the
			// empty remote IP with ErrIncompleteConfig and :516-521 skips it),
			// so the role config is inert either way; the defect being fixed is
			// that it was inert SILENTLY.
			if _, err := netip.ParseAddr(peerAddr); err != nil {
				logger().Warn("role config ignored: peer has no remote ip and its name is not an address",
					"peer", peerAddr,
					"effect", "no Role capability is advertised and no RFC 9234 OTC gate runs for this peer",
					"fix", "set connection > remote > ip on the peer or its group")
				return
			}
			key = peerAddr
		} else {
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
			Encoding: sdk.CapEncodingHex,
			Payload:  fmt.Sprintf("%02x", value),
			Peers:    []string{peerAddr},
		})
		logger().Debug("role capability", "peer", peerAddr, "role", cfg.role, "value", value)
	}

	return caps
}
