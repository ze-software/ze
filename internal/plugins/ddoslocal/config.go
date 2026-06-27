// Design: plan/spec-cp-survival-5-detect-2-local-responder.md -- local responder config

package ddoslocal

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
)

const (
	Name       = "ddos-local"
	configRoot = "ddos-local"
)

type Config struct {
	ResponseLevel         string         `json:"response-level"`
	MaxMitigationDuration int            `json:"max-mitigation-duration"`
	Allowlist             []netip.Prefix `json:"allowlist"`
}

func DefaultConfig() *Config {
	return &Config{
		ResponseLevel:         "alert",
		MaxMitigationDuration: 3600,
	}
}

func ParseConfig(data string) (*Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(data) == "" {
		return cfg, nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	m, ok := root[configRoot].(map[string]any)
	if !ok {
		return cfg, nil
	}
	if v, ok := m["response-level"].(string); ok {
		cfg.ResponseLevel = v
	}
	if v, ok := m["max-mitigation-duration"]; ok {
		if n, ok := toInt(v); ok {
			cfg.MaxMitigationDuration = n
		}
	}
	if v, ok := m["allowlist"].([]any); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				if p, err := netip.ParsePrefix(s); err == nil {
					cfg.Allowlist = append(cfg.Allowlist, p)
				}
			}
		}
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	switch c.ResponseLevel {
	case "alert", "enforce":
	default:
		return fmt.Errorf("response-level %q must be alert or enforce", c.ResponseLevel)
	}
	if c.MaxMitigationDuration < 0 || c.MaxMitigationDuration > 86400 {
		return fmt.Errorf("max-mitigation-duration %d out of range [0, 86400]", c.MaxMitigationDuration)
	}
	return nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
