// Design: docs/architecture/anomaly/anomaly-1-detect.md -- behavioral anomaly detector configuration

package detect

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	Name = "anomaly-detect"
	// configRoot is the nested YANG config path this plugin owns (anomaly/detect).
	// The plugin owns the shared `anomaly` container, so the delivered section is
	// wrapped as {"anomaly":{"detect":{...}}}. This must match the container path
	// in yang/ze-anomaly-detect-conf.yang and the ConfigRoots/WantsConfig entries.
	configRoot = "anomaly/detect"
)

type Config struct {
	Enabled                bool    `json:"enabled"`
	DeviationThreshold     float64 `json:"deviation-threshold"`       // sigma at/above which a feature fires
	MinFeaturesToCorrelate int     `json:"min-features-to-correlate"` // weak-signal correlation gate
	MinCohortSize          int     `json:"min-cohort-size"`           // min cohort members before rarity is scored
	CorroborationWeight    float64 `json:"corroboration-weight"`      // discount on corroborating features [0,1]
	ConfirmDuration        int     `json:"confirm-duration"`          // consecutive above-threshold ticks to confirm
	ClearConsecutive       int     `json:"clear-consecutive"`         // consecutive below-threshold ticks to clear
	BaselineWindow         int     `json:"baseline-window"`           // baseline horizon in ticks (EWMA alpha derived)
	CohortPrefixLenV4      int     `json:"cohort-prefix-len-v4"`      // source-prefix bucket for v4 cohorts
	CohortPrefixLenV6      int     `json:"cohort-prefix-len-v6"`      // source-prefix bucket for v6 cohorts
}

func DefaultConfig() *Config {
	return &Config{
		Enabled:                false,
		DeviationThreshold:     3.0,
		MinFeaturesToCorrelate: 2,
		MinCohortSize:          4,
		CorroborationWeight:    0.5,
		ConfirmDuration:        3,
		ClearConsecutive:       10,
		BaselineWindow:         300,
		CohortPrefixLenV4:      24,
		CohortPrefixLenV6:      48,
	}
}

// detectSubtree unwraps the two-level section wrapping that the plugin-server
// ExtractConfigSubtree helper produces for the "anomaly/detect" config root:
// {"anomaly": {"detect": {...}}}. Returns nil when either level is absent.
func detectSubtree(root map[string]any) map[string]any {
	anomaly, ok := root["anomaly"].(map[string]any)
	if !ok {
		return nil
	}
	detect, ok := anomaly["detect"].(map[string]any)
	if !ok {
		return nil
	}
	return detect
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
	// Section is wrapped by ExtractConfigSubtree as {"anomaly":{"detect":{...}}}.
	m := detectSubtree(root)
	if m == nil {
		return cfg, nil
	}

	if b, ok := cfgBool(m["enabled"]); ok {
		cfg.Enabled = b
	}
	if n, ok := toFloat(m["deviation-threshold"]); ok {
		cfg.DeviationThreshold = n
	}
	if n, ok := toInt(m["min-features-to-correlate"]); ok {
		cfg.MinFeaturesToCorrelate = n
	}
	if n, ok := toInt(m["min-cohort-size"]); ok {
		cfg.MinCohortSize = n
	}
	if n, ok := toFloat(m["corroboration-weight"]); ok {
		cfg.CorroborationWeight = n
	}
	if n, ok := toInt(m["confirm-duration"]); ok {
		cfg.ConfirmDuration = n
	}
	if n, ok := toInt(m["clear-consecutive"]); ok {
		cfg.ClearConsecutive = n
	}
	if n, ok := toInt(m["baseline-window"]); ok {
		cfg.BaselineWindow = n
	}
	if n, ok := toInt(m["cohort-prefix-len-v4"]); ok {
		cfg.CohortPrefixLenV4 = n
	}
	if n, ok := toInt(m["cohort-prefix-len-v6"]); ok {
		cfg.CohortPrefixLenV6 = n
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.DeviationThreshold < 1.0 || c.DeviationThreshold > 100.0 {
		return fmt.Errorf("deviation-threshold %g out of range [1.0, 100.0]", c.DeviationThreshold)
	}
	if c.MinFeaturesToCorrelate < 1 || c.MinFeaturesToCorrelate > 6 {
		return fmt.Errorf("min-features-to-correlate %d out of range [1, 6]", c.MinFeaturesToCorrelate)
	}
	if c.MinCohortSize < 2 || c.MinCohortSize > 1024 {
		return fmt.Errorf("min-cohort-size %d out of range [2, 1024]", c.MinCohortSize)
	}
	if c.CorroborationWeight < 0.0 || c.CorroborationWeight > 1.0 {
		return fmt.Errorf("corroboration-weight %g out of range [0.0, 1.0]", c.CorroborationWeight)
	}
	if c.ConfirmDuration < 1 || c.ConfirmDuration > 3600 {
		return fmt.Errorf("confirm-duration %d out of range [1, 3600]", c.ConfirmDuration)
	}
	if c.ClearConsecutive < 1 || c.ClearConsecutive > 100 {
		return fmt.Errorf("clear-consecutive %d out of range [1, 100]", c.ClearConsecutive)
	}
	if c.BaselineWindow < 10 || c.BaselineWindow > 86400 {
		return fmt.Errorf("baseline-window %d out of range [10, 86400]", c.BaselineWindow)
	}
	if c.CohortPrefixLenV4 < 8 || c.CohortPrefixLenV4 > 32 {
		return fmt.Errorf("cohort-prefix-len-v4 %d out of range [8, 32]", c.CohortPrefixLenV4)
	}
	if c.CohortPrefixLenV6 < 16 || c.CohortPrefixLenV6 > 64 {
		return fmt.Errorf("cohort-prefix-len-v6 %d out of range [16, 64]", c.CohortPrefixLenV6)
	}
	return nil
}

// The config framework delivers YANG leaf values as JSON strings (e.g. "300",
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
