// Design: plan/spec-cp-survival-5-detect-1-detector.md -- detector configuration

package detect

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Name       = "ddos-detect"
	configRoot = "ddos-detect"
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

	if v, ok := m["enabled"]; ok {
		if b, ok := v.(bool); ok {
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
	return cfg, nil
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
