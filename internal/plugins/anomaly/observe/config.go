// Design: docs/architecture/anomaly/anomaly-3-observe.md -- incident lifecycle store configuration
//
// Two leaves, both bounds: how many incidents the ring keeps, and how long an
// incident may stay open before the sweep finalizes it. There is no enabled leaf,
// because the store costs an empty slice when the detector emits nothing.

package observe

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
)

const (
	// Name is the plugin-registry name; it also names the process the plugin
	// manager starts.
	Name = "anomaly-observe"
	// configRoot is the nested YANG config path. The plugin augments the shared
	// `anomaly` container, so its section arrives wrapped as
	// {"anomaly":{"observe":{...}}}.
	configRoot = "anomaly/observe"
)

// loggerPtr is an atomic pointer rather than a plain variable because tests run
// several in-process plugin instances at once (ai/patterns/plugin.md).
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

// Config holds the observe subtree. Both fields are counts in their own unit:
// incidents for the ring, seconds for the timeout.
type Config struct {
	IncidentRingSize     int `json:"incident-ring-size"`
	StaleIncidentTimeout int `json:"stale-incident-timeout"`
}

// DefaultConfig returns the YANG defaults, which are the values the plugin runs
// with when the operator writes an empty `observe {}` block.
func DefaultConfig() *Config {
	return &Config{
		IncidentRingSize:     1000,
		StaleIncidentTimeout: 3600,
	}
}

// ParseConfig reads the wrapped section the config framework delivers. An empty
// payload is not an error: it means the operator declared the container and set no
// leaf, so every leaf keeps its default.
func ParseConfig(data string) (*Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(data) == "" {
		return cfg, nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	// ExtractConfigSubtree wraps the section as {"anomaly":{"observe":{...}}},
	// because this plugin augments the `anomaly` parent that anomaly-detect owns.
	anomaly, ok := root["anomaly"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	section, ok := anomaly["observe"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	if v, ok := toInt(section["incident-ring-size"]); ok {
		cfg.IncidentRingSize = v
	}
	if v, ok := toInt(section["stale-incident-timeout"]); ok {
		cfg.StaleIncidentTimeout = v
	}
	return cfg, nil
}

// Validate enforces the same ranges the YANG leaves declare, so a section that
// reaches the plugin by any other path is still bounded.
func (c *Config) Validate() error {
	if c.IncidentRingSize < 1 || c.IncidentRingSize > 100000 {
		return fmt.Errorf("incident-ring-size %d out of range [1, 100000]", c.IncidentRingSize)
	}
	if c.StaleIncidentTimeout < 1 || c.StaleIncidentTimeout > 86400 {
		return fmt.Errorf("stale-incident-timeout %d out of range [1, 86400]", c.StaleIncidentTimeout)
	}
	return nil
}

// toInt accepts the string form alongside the native JSON number, because the
// config framework delivers YANG leaf values as JSON strings (for example "1000").
// Without the string case every leaf silently falls back to its default.
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
