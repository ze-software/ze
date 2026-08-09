// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- flowspec responder config

package flowspec

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	Name = "ddos-flowspec"
	// configRoot is the nested YANG config path (ddos/flowspec); the plugin
	// augments the shared `ddos` container, so the section is wrapped as
	// {"ddos":{"flowspec":{...}}}.
	configRoot = "ddos/flowspec"

	// responseEnforce is the response-level that actually announces (vs "alert").
	responseEnforce = "enforce"

	// FlowSpec traffic actions (the `action` leaf enum values).
	actionRateLimit = "rate-limit"
	actionDiscard   = "discard"
)

type Config struct {
	ResponseLevel         string  `json:"response-level"`
	Action                string  `json:"action"`
	RateLimitBytes        uint64  `json:"rate-limit-bytes"`
	rateLimitBytesSet     bool    // whether rate-limit-bytes was present in config (0 is a valid explicit value)
	HoldDown              int     `json:"hold-down"`
	ProbeInterval         int     `json:"probe-interval"`
	ProbeWindow           int     `json:"probe-window"`
	ProbeRate             float64 `json:"probe-rate"`
	AnnounceRateLimit     int     `json:"announce-rate-limit"`
	MaxMitigationDuration int     `json:"max-mitigation-duration"`
	BackoffCap            int     `json:"backoff-cap"`
	BlackholeFallback     bool    `json:"blackhole-fallback"`
	// ConfidenceMin (0-100) gates the characterized announce path: an
	// AttackCharacterized whose confidence is below this is not announced upstream.
	// 0 (default) disables the gate. The blackhole-fallback fast path is never gated.
	ConfidenceMin int `json:"confidence-min"`
}

func DefaultConfig() *Config {
	return &Config{
		ResponseLevel: "alert",
		// Action has no default: the operator must choose rate-limit or discard
		// (YANG `mandatory`). A bare/absent block runs in alert mode and never
		// announces, so Action stays empty and unused.
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
	// Section is wrapped by ExtractConfigSubtree as {"ddos":{"flowspec":{...}}}.
	ddos, ok := root["ddos"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	m, ok := ddos["flowspec"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	if v, ok := m["response-level"].(string); ok {
		cfg.ResponseLevel = v
	}
	if v, ok := m["action"].(string); ok {
		cfg.Action = v
	}
	if v, ok := m["rate-limit-bytes"]; ok {
		cfg.rateLimitBytesSet = true
		if n, ok := toInt(v); ok && n >= 0 {
			cfg.RateLimitBytes = uint64(n)
		}
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
	if v, ok := m["blackhole-fallback"]; ok {
		if b, ok := toBool(v); ok {
			cfg.BlackholeFallback = b
		}
	}
	if v, ok := m["confidence-min"]; ok {
		if n, ok := toInt(v); ok {
			cfg.ConfidenceMin = n
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
	case actionRateLimit, actionDiscard:
	default:
		return fmt.Errorf("action %q must be rate-limit or discard", c.Action)
	}
	// action rate-limit announces an RFC 8955 traffic-rate; the operator must
	// state the byte rate explicitly (no fabricated default). A zero rate is a
	// valid choice (== discard); an ABSENT rate is a config error.
	if c.Action == actionRateLimit && !c.rateLimitBytesSet {
		return fmt.Errorf("action rate-limit requires rate-limit-bytes (use action discard, or rate-limit-bytes 0, for a drop)")
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
	if c.ConfidenceMin < 0 || c.ConfidenceMin > 100 {
		return fmt.Errorf("confidence-min %d out of range [0, 100]", c.ConfidenceMin)
	}
	return nil
}

// The config framework delivers YANG leaf values as JSON strings ("50000",
// "true"), so each coercion accepts a string form alongside the native JSON
// type; without it every numeric leaf silently reverted to its default.

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
		return 0, false
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
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// toBool coerces a config value (bool or the daemon's string form) to bool.
func toBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if pb, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return pb, true
		}
	}
	return false, false
}
