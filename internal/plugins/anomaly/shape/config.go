// Design: docs/architecture/anomaly/anomaly-2-shape.md -- shadow-first anomaly responder config

package shape

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/configvalue"
)

const (
	Name = "anomaly-shape"
	// configRoot is the nested YANG config path this plugin owns (anomaly/shape).
	// The plugin augments the shared `anomaly` container with `shape`, so the
	// delivered section is wrapped as {"anomaly":{"shape":{...}}}. This must match
	// the augment target in yang/ze-anomaly-shape-conf.yang and the
	// ConfigRoots/WantsConfig entries.
	configRoot = "anomaly/shape"

	// ModeShadow logs the would-be action and installs nothing (the default).
	ModeShadow = "shadow"
	// ModeArmed installs live per-entity firewall actions.
	ModeArmed = "armed"

	// ActionLimit rate-limits an armed source (the surgical default).
	ActionLimit = "limit"
	// ActionDrop drops an armed source (the fallback).
	ActionDrop = "drop"
)

type Config struct {
	Mode           string         `json:"mode"`             // shadow (default) | armed
	Action         string         `json:"action"`           // limit (default) | drop
	LimitRate      uint64         `json:"limit-rate"`       // rate for the Limit action
	LimitUnit      string         `json:"limit-unit"`       // second | minute | hour | day
	LimitBurst     uint32         `json:"limit-burst"`      // burst allowance
	AutoRevertTTL  int            `json:"auto-revert-ttl"`  // seconds; safety ceiling from last signal
	BlastRadiusCap int            `json:"blast-radius-cap"` // max concurrently-armed live actions
	KillSwitch     bool           `json:"kill-switch"`      // revert all + force shadow
	Allowlist      []netip.Prefix `json:"allowlist"`        // protected sources never armed
}

func DefaultConfig() *Config {
	return &Config{
		Mode:           ModeShadow,
		Action:         ActionLimit,
		LimitRate:      1000,
		LimitUnit:      "second",
		LimitBurst:     0,
		AutoRevertTTL:  300,
		BlastRadiusCap: 16,
		KillSwitch:     false,
	}
}

// shapeSubtree unwraps the two-level section wrapping that the plugin-server
// ExtractConfigSubtree helper produces for the "anomaly/shape" config root:
// {"anomaly": {"shape": {...}}}. Returns nil when either level is absent.
func shapeSubtree(root map[string]any) map[string]any {
	anomaly, ok := root["anomaly"].(map[string]any)
	if !ok {
		return nil
	}
	shape, ok := anomaly["shape"].(map[string]any)
	if !ok {
		return nil
	}
	return shape
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
	// Section is wrapped by ExtractConfigSubtree as {"anomaly":{"shape":{...}}}.
	m := shapeSubtree(root)
	if m == nil {
		return cfg, nil
	}
	if v, ok := m["mode"].(string); ok {
		cfg.Mode = v
	}
	if v, ok := m["action"].(string); ok {
		cfg.Action = v
	}
	if n, ok := toUint64(m["limit-rate"]); ok {
		cfg.LimitRate = n
	}
	if v, ok := m["limit-unit"].(string); ok {
		cfg.LimitUnit = v
	}
	if n, ok := toUint64(m["limit-burst"]); ok {
		cfg.LimitBurst = uint32(n)
	}
	if n, ok := toInt(m["auto-revert-ttl"]); ok {
		cfg.AutoRevertTTL = n
	}
	if n, ok := toInt(m["blast-radius-cap"]); ok {
		cfg.BlastRadiusCap = n
	}
	if b, ok := cfgBool(m["kill-switch"]); ok {
		cfg.KillSwitch = b
	}
	// configvalue.LeafList, not a []any assertion: Tree.ToMap collapses a
	// one-member leaf-list to a bare string, so the assertion dropped the whole
	// allowlist whenever the operator named exactly one protected prefix, and
	// the responder then armed against it.
	for _, s := range configvalue.LeafList(m["allowlist"]) {
		if p, err := netip.ParsePrefix(s); err == nil {
			cfg.Allowlist = append(cfg.Allowlist, p)
		}
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	switch c.Mode {
	case ModeShadow, ModeArmed:
	default:
		return fmt.Errorf("mode %q must be shadow or armed", c.Mode)
	}
	switch c.Action {
	case ActionLimit, ActionDrop:
	default:
		return fmt.Errorf("action %q must be limit or drop", c.Action)
	}
	switch c.LimitUnit {
	case "second", "minute", "hour", "day":
	default:
		return fmt.Errorf("limit-unit %q must be second, minute, hour, or day", c.LimitUnit)
	}
	if c.Action == ActionLimit && c.LimitRate < 1 {
		return fmt.Errorf("limit-rate must be >= 1 for the limit action")
	}
	if c.AutoRevertTTL < 5 || c.AutoRevertTTL > 3600 {
		return fmt.Errorf("auto-revert-ttl %d out of range [5, 3600]", c.AutoRevertTTL)
	}
	if c.BlastRadiusCap < 1 || c.BlastRadiusCap > 1024 {
		return fmt.Errorf("blast-radius-cap %d out of range [1, 1024]", c.BlastRadiusCap)
	}
	return nil
}

// The config framework delivers YANG leaf values as JSON strings (e.g. "16",
// "true"), so every scalar coercion accepts a string form alongside the native
// JSON type -- matching the rest of ze's plugin config parsers. Without the
// string case, every leaf silently falls back to its default (which for
// `kill-switch` would ignore an operator's emergency revert request).

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

func toUint64(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case string:
		if u, err := strconv.ParseUint(strings.TrimSpace(n), 10, 64); err == nil {
			return u, true
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
