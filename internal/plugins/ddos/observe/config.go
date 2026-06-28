// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- observability config

package observe

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
)

const (
	Name       = "ddos-observe"
	configRoot = "ddos-observe"
)

var loggerPtr atomic.Pointer[slog.Logger]

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

type Config struct {
	IncidentRingSize     int `json:"incident-ring-size"`
	StaleIncidentTimeout int `json:"stale-incident-timeout"`
}

func DefaultConfig() *Config {
	return &Config{
		IncidentRingSize:     1000,
		StaleIncidentTimeout: 3600,
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
	if v, ok := toInt(m["incident-ring-size"]); ok {
		cfg.IncidentRingSize = v
	}
	if v, ok := toInt(m["stale-incident-timeout"]); ok {
		cfg.StaleIncidentTimeout = v
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.IncidentRingSize < 1 || c.IncidentRingSize > 100000 {
		return fmt.Errorf("incident-ring-size %d out of range [1, 100000]", c.IncidentRingSize)
	}
	if c.StaleIncidentTimeout < 1 || c.StaleIncidentTimeout > 86400 {
		return fmt.Errorf("stale-incident-timeout %d out of range [1, 86400]", c.StaleIncidentTimeout)
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
