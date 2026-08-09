// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- Flowtriq reporter config

package flowtriq

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
)

const (
	Name = "ddos-flowtriq"
	// configRoot is the nested YANG config path (ddos/flowtriq); the plugin
	// augments the shared `ddos` container, so the section is wrapped as
	// {"ddos":{"flowtriq":{...}}}.
	configRoot = "ddos/flowtriq"
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
	Enabled  bool   `json:"enabled"`
	APIKey   string `json:"api-key"` //nolint:gosec // config value, not hardcoded
	NodeUUID string `json:"node-uuid"`
	APIBase  string `json:"api-base"`
}

func DefaultConfig() *Config {
	return &Config{
		APIBase: "https://flowtriq.com/api/v1",
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
	// Section is wrapped by ExtractConfigSubtree as {"ddos":{"flowtriq":{...}}}.
	ddos, ok := root["ddos"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	m, ok := ddos["flowtriq"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	if v, ok := cfgBool(m["enabled"]); ok {
		cfg.Enabled = v
	}
	if v, ok := m["api-key"].(string); ok {
		cfg.APIKey = v
	}
	if v, ok := m["node-uuid"].(string); ok {
		cfg.NodeUUID = v
	}
	if v, ok := m["api-base"].(string); ok {
		cfg.APIBase = v
	}
	return cfg, nil
}

// cfgBool coerces a config value to bool. The config framework delivers YANG leaf
// values as JSON strings ("true"/"false"), so the native JSON bool and the string
// form are both accepted -- without the string case `enabled` silently stayed false
// and the reporter never ran.
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

func (c *Config) Validate() error {
	if c.Enabled {
		if c.APIKey == "" {
			return fmt.Errorf("api-key is required when enabled")
		}
		if c.NodeUUID == "" {
			return fmt.Errorf("node-uuid is required when enabled")
		}
		if c.APIBase == "" {
			return fmt.Errorf("api-base must not be empty")
		}
	}
	return nil
}
