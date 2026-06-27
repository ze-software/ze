// Design: plan/spec-cp-survival-5-detect-3-flowspec-responder.md -- flowspec responder config

package ddosflowspec

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
)

const (
	Name       = "ddos-flowspec"
	configRoot = "ddos-flowspec"
)

type Config struct {
	ResponseLevel         string         `json:"response-level"`
	Action                string         `json:"action"`
	HoldDown              int            `json:"hold-down"`
	ProbeInterval         int            `json:"probe-interval"`
	ProbeWindow           int            `json:"probe-window"`
	ProbeRate             float64        `json:"probe-rate"`
	AnnounceRateLimit     int            `json:"announce-rate-limit"`
	MaxMitigationDuration int            `json:"max-mitigation-duration"`
	BackoffCap            int            `json:"backoff-cap"`
	Allowlist             []netip.Prefix `json:"allowlist"`
}

func DefaultConfig() *Config {
	return &Config{
		ResponseLevel:         "alert",
		Action:                "rate-limit",
		HoldDown:              300,
		ProbeInterval:         60,
		ProbeWindow:           10,
		ProbeRate:             1000000,
		AnnounceRateLimit:     10,
		MaxMitigationDuration: 3600,
		BackoffCap:            3600,
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
	if v, ok := m["action"].(string); ok {
		cfg.Action = v
	}
	if v, ok := m["hold-down"]; ok {
		if n, ok := toInt(v); ok {
			cfg.HoldDown = n
		}
	}
	if v, ok := m["probe-interval"]; ok {
		if n, ok := toInt(v); ok {
			cfg.ProbeInterval = n
		}
	}
	if v, ok := m["probe-window"]; ok {
		if n, ok := toInt(v); ok {
			cfg.ProbeWindow = n
		}
	}
	if v, ok := m["probe-rate"]; ok {
		if f, ok := toFloat(v); ok {
			cfg.ProbeRate = f
		}
	}
	if v, ok := m["announce-rate-limit"]; ok {
		if n, ok := toInt(v); ok {
			cfg.AnnounceRateLimit = n
		}
	}
	if v, ok := m["max-mitigation-duration"]; ok {
		if n, ok := toInt(v); ok {
			cfg.MaxMitigationDuration = n
		}
	}
	if v, ok := m["backoff-cap"]; ok {
		if n, ok := toInt(v); ok {
			cfg.BackoffCap = n
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
	switch c.Action {
	case "rate-limit", "discard":
	default:
		return fmt.Errorf("action %q must be rate-limit or discard", c.Action)
	}
	if c.HoldDown < 1 || c.HoldDown > 86400 {
		return fmt.Errorf("hold-down %d out of range [1, 86400]", c.HoldDown)
	}
	if c.ProbeInterval < 1 || c.ProbeInterval > 3600 {
		return fmt.Errorf("probe-interval %d out of range [1, 3600]", c.ProbeInterval)
	}
	if c.ProbeWindow < 1 || c.ProbeWindow > 300 {
		return fmt.Errorf("probe-window %d out of range [1, 300]", c.ProbeWindow)
	}
	if c.ProbeRate < 1 {
		return fmt.Errorf("probe-rate %f must be >= 1", c.ProbeRate)
	}
	if c.AnnounceRateLimit < 1 || c.AnnounceRateLimit > 600 {
		return fmt.Errorf("announce-rate-limit %d out of range [1, 600]", c.AnnounceRateLimit)
	}
	if c.MaxMitigationDuration < 0 || c.MaxMitigationDuration > 604800 {
		return fmt.Errorf("max-mitigation-duration %d out of range [0, 604800]", c.MaxMitigationDuration)
	}
	if c.BackoffCap < c.HoldDown || c.BackoffCap > 604800 {
		return fmt.Errorf("backoff-cap %d out of range [%d, 604800]", c.BackoffCap, c.HoldDown)
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

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
