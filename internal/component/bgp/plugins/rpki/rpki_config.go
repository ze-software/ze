// Design: docs/architecture/plugin/rib-storage-design.md -- RPKI config parsing
// Overview: rpki.go -- plugin entry point using parsed config
package rpki

import (
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
)

// cacheServerConfig holds parsed config for a single RTR cache server.
type cacheServerConfig struct {
	Address    string
	Port       uint16
	Preference uint8
}

// rpkiConfig holds the parsed RPKI plugin configuration.
type rpkiConfig struct {
	CacheServers      []cacheServerConfig
	ValidationTimeout uint16 // seconds, 0 = use default (30s)
}

// parseRPKIConfig extracts RPKI configuration from a BGP config JSON string.
// The JSON is delivered by the engine via OnConfigure with root="bgp".
// Returns empty config (no cache servers) when no rpki section is present.
func parseRPKIConfig(jsonStr string) (*rpkiConfig, error) {
	bgpTree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return nil, fmt.Errorf("rpki: invalid BGP config JSON")
	}

	cfg := &rpkiConfig{}

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

		cfg.CacheServers = append(cfg.CacheServers, cs)
	}

	return cfg, nil
}
