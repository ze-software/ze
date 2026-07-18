// Design: docs/architecture/plugin/rib-storage-design.md -- RPKI config parsing
// Overview: rpki.go -- plugin entry point using parsed config
package rpki

import (
	"errors"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
)

var errRpkiInvalidBgpConfigJson = errors.New("rpki: invalid BGP config JSON")

// cacheServerConfig holds parsed config for a single RTR cache server.
type cacheServerConfig struct {
	Address       string
	Port          uint16
	Preference    uint8
	SourceAddress string
}

// ASPA policy actions.
const (
	ASPAPolicyReject  uint8 = 0
	ASPAPolicyLogOnly uint8 = 1
	ASPAPolicyAccept  uint8 = 2
)

// aspaActionFromString converts a config string to a policy action constant.
func aspaActionFromString(s string) (uint8, bool) {
	switch s {
	case "reject":
		return ASPAPolicyReject, true
	case "log-only":
		return ASPAPolicyLogOnly, true
	case "accept":
		return ASPAPolicyAccept, true
	default:
		return 0, false
	}
}

// rpkiConfig holds the parsed RPKI plugin configuration.
type rpkiConfig struct {
	CacheServers      []cacheServerConfig
	ValidationTimeout uint16 // seconds, 0 = use default (30s)
	ASPAValidation    bool   // enable/disable ASPA path verification (default false)
	ASPAInvalidAction uint8  // action for ASPA Invalid routes (default: log-only)
	ASPAUnknownAction uint8  // action for ASPA Unknown routes (default: accept)
	// OriginInvalidAction is the RFC 6811 Section 3 operator-configurable action for the Invalid
	// origin-validation state (default: reject). It is what makes the exclusion of Invalid routes
	// an explicitly configured policy choice (RFC 6811 Section 2) rather than an unconditional
	// side effect: only OriginInvalidAction == ASPAPolicyReject excludes the route.
	OriginInvalidAction uint8
	// OriginNotFoundAction is the action for the NotFound origin-validation state (default: accept).
	OriginNotFoundAction uint8
}

// parseRPKIConfig extracts RPKI configuration from a BGP config JSON string.
// The JSON is delivered by the engine via OnConfigure with root="bgp".
// Returns empty config (no cache servers) when no rpki section is present.
func parseRPKIConfig(jsonStr string) (*rpkiConfig, error) {
	bgpTree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return nil, errRpkiInvalidBgpConfigJson
	}

	cfg := &rpkiConfig{
		ASPAValidation:       false,
		ASPAInvalidAction:    ASPAPolicyLogOnly,
		ASPAUnknownAction:    ASPAPolicyAccept,
		OriginInvalidAction:  ASPAPolicyReject, // RFC 6811: default reject Invalid (matches YANG default)
		OriginNotFoundAction: ASPAPolicyAccept, // default accept NotFound (matches YANG default)
	}

	rpkiMap, ok := bgpTree["rpki"].(map[string]any)
	if !ok {
		return cfg, nil // No RPKI config section -- empty config
	}

	// Parse validation-timeout
	if vtStr, ok := rpkiMap["validation-timeout"].(string); ok {
		vt, err := strconv.ParseUint(vtStr, 10, 16)
		if err == nil {
			cfg.ValidationTimeout = uint16(vt) //nolint:gosec // range checked by ParseUint
		}
	}

	// Parse origin-validation policy from rpki/policy container. RFC 6811 Section 3: the action
	// for each validation state is operator-configurable. The YANG defaults are
	// invalid-action=reject and not-found-action=accept.
	if policyMap, ok := rpkiMap["policy"].(map[string]any); ok {
		if actionStr, ok := policyMap["invalid-action"].(string); ok {
			if action, valid := aspaActionFromString(actionStr); valid {
				cfg.OriginInvalidAction = action
			}
		}
		if actionStr, ok := policyMap["not-found-action"].(string); ok {
			if action, valid := aspaActionFromString(actionStr); valid {
				cfg.OriginNotFoundAction = action
			}
		}
	}

	// Parse ASPA settings from rpki/aspa container.
	if aspaMap, ok := rpkiMap["aspa"].(map[string]any); ok {
		if valStr, ok := aspaMap["validation"].(string); ok {
			cfg.ASPAValidation = valStr == "true" || valStr == "1"
		}
		if policyMap, ok := aspaMap["policy"].(map[string]any); ok {
			if actionStr, ok := policyMap["invalid-action"].(string); ok {
				if action, valid := aspaActionFromString(actionStr); valid {
					cfg.ASPAInvalidAction = action
				}
			}
			if actionStr, ok := policyMap["unknown-action"].(string); ok {
				if action, valid := aspaActionFromString(actionStr); valid {
					cfg.ASPAUnknownAction = action
				}
			}
		}
	}

	// Parse cache-server list (YANG list keyed by address)
	csMap, ok := rpkiMap["cache-server"].(map[string]any)
	if !ok {
		return cfg, nil // RPKI section exists but no cache servers
	}

	for addr, serverRaw := range csMap {
		serverMap, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		cs := cacheServerConfig{
			Address:    addr,
			Port:       323, // RTR default port
			Preference: 100, // YANG default
		}

		if portStr, ok := serverMap["port"].(string); ok {
			p, err := strconv.ParseUint(portStr, 10, 16)
			if err == nil {
				cs.Port = uint16(p) //nolint:gosec // range checked by ParseUint
			}
		}

		if prefStr, ok := serverMap["preference"].(string); ok {
			p, err := strconv.ParseUint(prefStr, 10, 8)
			if err == nil {
				cs.Preference = uint8(p) //nolint:gosec // range checked by ParseUint
			}
		}

		if sa, ok := serverMap["source-address"].(string); ok && sa != "" {
			cs.SourceAddress = sa
		}

		cfg.CacheServers = append(cfg.CacheServers, cs)
	}

	return cfg, nil
}
