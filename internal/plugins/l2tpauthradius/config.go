// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS plugin config
// Related: register.go -- plugin lifecycle callbacks

package l2tpauthradius

import (
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/radius"
)

// radiusConfig holds parsed RADIUS server configuration.
type radiusConfig struct {
	Servers       []radius.Server
	Timeout       time.Duration
	Retries       int
	AcctInterval  time.Duration
	NASIdentifier string
}

// errNoRADIUSConfig is returned when the config tree has no auth.radius block.
var errNoRADIUSConfig = fmt.Errorf("%s: no auth.radius block in config", Name)

func parseConfigFromTree(tree map[string]any) (*radiusConfig, error) {
	authBlock, ok := tree["auth"].(map[string]any)
	if !ok {
		return nil, errNoRADIUSConfig
	}
	radiusBlock, ok := authBlock["radius"].(map[string]any)
	if !ok {
		return nil, errNoRADIUSConfig
	}

	cfg := &radiusConfig{
		Timeout:      3 * time.Second,
		Retries:      3,
		AcctInterval: 300 * time.Second,
	}

	if nasID, ok := radiusBlock["nas-identifier"].(string); ok {
		cfg.NASIdentifier = nasID
	}

	if timeout, ok := radiusBlock["timeout"].(float64); ok {
		v := int(timeout)
		if v < 1 || v > 30 {
			return nil, fmt.Errorf("%s: timeout must be 1-30, got %d", Name, v)
		}
		cfg.Timeout = time.Duration(v) * time.Second
	}

	if retries, ok := radiusBlock["retries"].(float64); ok {
		v := int(retries)
		if v < 1 || v > 10 {
			return nil, fmt.Errorf("%s: retries must be 1-10, got %d", Name, v)
		}
		cfg.Retries = v
	}

	if interval, ok := radiusBlock["acct-interval"].(float64); ok {
		v := int(interval)
		if v < 60 || v > 3600 {
			return nil, fmt.Errorf("%s: acct-interval must be 60-3600, got %d", Name, v)
		}
		cfg.AcctInterval = time.Duration(v) * time.Second
	}

	serverList, ok := radiusBlock["server"]
	if !ok {
		return nil, fmt.Errorf("%s: no servers configured", Name)
	}

	entries, ok := serverList.([]any)
	if !ok {
		if single, ok2 := serverList.(map[string]any); ok2 {
			entries = []any{single}
		} else {
			return nil, fmt.Errorf("%s: invalid server list type %T", Name, serverList)
		}
	}

	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		address, _ := m["address"].(string)
		if address == "" {
			return nil, fmt.Errorf("%s: server entry missing address", Name)
		}
		port := 1812
		if p, ok := m["port"].(float64); ok {
			port = int(p)
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("%s: port must be 1-65535, got %d", Name, port)
			}
		}
		sharedKey, _ := m["shared-key"].(string)
		if sharedKey == "" {
			return nil, fmt.Errorf("%s: server %s missing shared-key", Name, address)
		}
		cfg.Servers = append(cfg.Servers, radius.Server{
			Address:   fmt.Sprintf("%s:%d", address, port),
			SharedKey: []byte(sharedKey),
		})
	}

	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("%s: no valid servers configured", Name)
	}

	return cfg, nil
}
