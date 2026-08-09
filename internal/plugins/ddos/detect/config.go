// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- detector configuration

package detect

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const (
	Name = "ddos-detect"
	// configRoot is the nested YANG config path this plugin owns (ddos/detect).
	// detect owns the shared `ddos` container, so the delivered section is wrapped
	// as {"ddos":{"detect":{...}}}.
	configRoot = "ddos/detect"
)

type Config struct {
	Enabled             bool    `json:"enabled"`
	CheckInterval       int     `json:"check-interval"`
	ConfirmDuration     int     `json:"confirm-duration"`
	ClearConsecutive    int     `json:"clear-consecutive-checks"`
	BaselineWindow      int     `json:"baseline-window"`
	ThresholdMultiplier float64 `json:"threshold-multiplier"`
	AbsoluteFloor       float64 `json:"absolute-floor"`
	StartupGrace        int     `json:"startup-grace"`

	// Bandwidth (BPS) trigger: catches low-PPS/high-bandwidth amplification
	// (NTP/memcached/CLDAP) that the PPS threshold misses. BpsFloor is in bits/sec
	// (operator-facing unit); the detector's internal rate is bytes/sec and converts.
	BpsTriggerEnable       bool    `json:"bps-trigger-enable"`
	BpsThresholdMultiplier float64 `json:"bps-threshold-multiplier"`
	BpsFloor               float64 `json:"bps-floor"` // bits/sec; below this the BPS trigger is inert

	// Stage-2 characterization tuning.
	CharacterizeEnable  bool    `json:"characterize-enable"`  // run flow-based classification -> AttackCharacterized
	TopNSources         int     `json:"top-n-sources"`        // max attacker addresses ranked into TopSources
	CharacterizeWindow  int     `json:"characterize-window"`  // seconds of recent flows to consider (0-ts flows always kept)
	CharacterizeTimeout int     `json:"characterize-timeout"` // ms budget for the on-trigger source queries
	EntropyThreshold    float64 `json:"entropy-threshold"`    // source-entropy (bits) at/above which an attack is logged as distributed

	// Policy is the operator's allow/deny traffic policy (prefix-indexed, longest-prefix
	// match). nil = no policy = defend everything (default-action deny). See policy.go.
	Policy *Policy `json:"policy,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Enabled:             false,
		CheckInterval:       1,
		ConfirmDuration:     3,
		ClearConsecutive:    10,
		BaselineWindow:      300,
		ThresholdMultiplier: 3.0,
		AbsoluteFloor:       5000,
		StartupGrace:        90,

		BpsTriggerEnable:       true,
		BpsThresholdMultiplier: 3.0,
		BpsFloor:               50_000_000, // 50 Mbps (bits/sec)
		CharacterizeEnable:     true,
		TopNSources:            10,
		CharacterizeWindow:     10,
		CharacterizeTimeout:    2000,
		EntropyThreshold:       2.0,
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
	// Section is wrapped by ExtractConfigSubtree as {"ddos":{"detect":{...}}}.
	ddos, ok := root["ddos"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	m, ok := ddos["detect"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	if v, ok := m["enabled"]; ok {
		if b, ok := cfgBool(v); ok {
			cfg.Enabled = b
		}
	}
	if v, ok := m["check-interval"]; ok {
		if n, ok := toInt(v); ok {
			cfg.CheckInterval = n
		}
	}
	if v, ok := m["confirm-duration"]; ok {
		if n, ok := toInt(v); ok {
			cfg.ConfirmDuration = n
		}
	}
	if v, ok := m["clear-consecutive-checks"]; ok {
		if n, ok := toInt(v); ok {
			cfg.ClearConsecutive = n
		}
	}
	if v, ok := m["baseline-window"]; ok {
		if n, ok := toInt(v); ok {
			cfg.BaselineWindow = n
		}
	}
	if v, ok := m["threshold-multiplier"]; ok {
		if f, ok := toFloat(v); ok {
			cfg.ThresholdMultiplier = f
		}
	}
	if v, ok := m["absolute-floor"]; ok {
		if f, ok := toFloat(v); ok {
			cfg.AbsoluteFloor = f
		}
	}
	if v, ok := m["startup-grace"]; ok {
		if n, ok := toInt(v); ok {
			cfg.StartupGrace = n
		}
	}
	if v, ok := m["bps-trigger-enable"]; ok {
		if b, ok := cfgBool(v); ok {
			cfg.BpsTriggerEnable = b
		}
	}
	if v, ok := m["bps-threshold-multiplier"]; ok {
		if f, ok := toFloat(v); ok {
			cfg.BpsThresholdMultiplier = f
		}
	}
	if v, ok := m["bps-floor"]; ok {
		if f, ok := toFloat(v); ok {
			cfg.BpsFloor = f
		}
	}
	if v, ok := m["characterize-enable"]; ok {
		if b, ok := cfgBool(v); ok {
			cfg.CharacterizeEnable = b
		}
	}
	if v, ok := m["top-n-sources"]; ok {
		if n, ok := toInt(v); ok {
			cfg.TopNSources = n
		}
	}
	if v, ok := m["characterize-window"]; ok {
		if n, ok := toInt(v); ok {
			cfg.CharacterizeWindow = n
		}
	}
	if v, ok := m["characterize-timeout"]; ok {
		if n, ok := toInt(v); ok {
			cfg.CharacterizeTimeout = n
		}
	}
	if v, ok := m["entropy-threshold"]; ok {
		if f, ok := toFloat(v); ok {
			cfg.EntropyThreshold = f
		}
	}
	if pv, ok := m["policy"].(map[string]any); ok {
		p, err := parsePolicy(pv)
		if err != nil {
			return nil, err
		}
		cfg.Policy = p
	}
	return cfg, nil
}

// parsePolicy reads the ddos/detect policy container. The YANG `list rule { key
// "prefix" }` is delivered as a map keyed by prefix (config lists are delivered as
// unordered maps, so precedence is longest-prefix, not config order; see policy.go).
// Enum leaves arrive as strings.
func parsePolicy(m map[string]any) (*Policy, error) {
	p := &Policy{DefaultAction: actionDeny}
	if v, ok := m["default-action"].(string); ok && v != "" {
		p.DefaultAction = v
	}
	ruleMap, ok := m["rule"].(map[string]any)
	if !ok {
		return p, nil
	}
	for prefixStr, rv := range ruleMap {
		rm, _ := rv.(map[string]any)
		pfx, err := netip.ParsePrefix(prefixStr)
		if err != nil {
			return nil, fmt.Errorf("policy rule %q: invalid prefix: %w", prefixStr, err)
		}
		p.Rules = append(p.Rules, PolicyRule{
			Prefix: pfx.Masked(),
			Action: cfgStr(rm, "action", actionDeny),
			Match:  cfgStr(rm, "match", matchAny),
			Scope:  cfgStr(rm, "scope", scopeMitigation),
		})
	}
	return p, nil
}

func cfgStr(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func (c *Config) Validate() error {
	if c.CheckInterval < 1 || c.CheckInterval > 3600 {
		return fmt.Errorf("check-interval %d out of range [1, 3600]", c.CheckInterval)
	}
	if c.ConfirmDuration < 0 || c.ConfirmDuration > 3600 {
		return fmt.Errorf("confirm-duration %d out of range [0, 3600]", c.ConfirmDuration)
	}
	if c.ClearConsecutive < 1 || c.ClearConsecutive > 100 {
		return fmt.Errorf("clear-consecutive-checks %d out of range [1, 100]", c.ClearConsecutive)
	}
	if c.BaselineWindow < 10 || c.BaselineWindow > 86400 {
		return fmt.Errorf("baseline-window %d out of range [10, 86400]", c.BaselineWindow)
	}
	if c.ThresholdMultiplier < 1.0 || c.ThresholdMultiplier > 100.0 {
		return fmt.Errorf("threshold-multiplier %f out of range [1.0, 100.0]", c.ThresholdMultiplier)
	}
	if c.AbsoluteFloor < 1 {
		return fmt.Errorf("absolute-floor %f must be >= 1", c.AbsoluteFloor)
	}
	if c.StartupGrace < 0 || c.StartupGrace > 3600 {
		return fmt.Errorf("startup-grace %d out of range [0, 3600]", c.StartupGrace)
	}
	if c.BpsThresholdMultiplier < 1.0 || c.BpsThresholdMultiplier > 100.0 {
		return fmt.Errorf("bps-threshold-multiplier %f out of range [1.0, 100.0]", c.BpsThresholdMultiplier)
	}
	if c.BpsFloor < 1 {
		return fmt.Errorf("bps-floor %f must be >= 1", c.BpsFloor)
	}
	if c.TopNSources < 1 || c.TopNSources > 100 {
		return fmt.Errorf("top-n-sources %d out of range [1, 100]", c.TopNSources)
	}
	if c.CharacterizeWindow < 1 || c.CharacterizeWindow > 60 {
		return fmt.Errorf("characterize-window %d out of range [1, 60]", c.CharacterizeWindow)
	}
	if c.CharacterizeTimeout < 50 || c.CharacterizeTimeout > 5000 {
		return fmt.Errorf("characterize-timeout %d out of range [50, 5000]", c.CharacterizeTimeout)
	}
	if c.EntropyThreshold < 0 || c.EntropyThreshold > 16 {
		return fmt.Errorf("entropy-threshold %f out of range [0, 16]", c.EntropyThreshold)
	}
	if err := c.Policy.validate(); err != nil {
		return err
	}
	return nil
}

// The config framework delivers YANG leaf values as JSON strings (e.g. "5000",
// "true"), so every coercion accepts a string form alongside the native JSON
// type -- matching the rest of ze's plugin config parsers. Without the string
// case, every leaf silently falls back to its default (which for `enabled` left
// the detector permanently disabled).

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

// cfgBool coerces a config value (native JSON bool or the string form "true"/
// "false" the framework actually delivers) to bool.
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if pb, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return pb, true
		}
		return false, false
	default:
		return false, false
	}
}
