// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- local responder config

package local

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	Name = "ddos-local"
	// responseEnforce is the response-level that actually mitigates (vs "alert").
	responseEnforce = "enforce"
	// configRoot is the nested YANG config path (ddos/local); the plugin augments
	// the shared `ddos` container, so the section is wrapped as
	// {"ddos":{"local":{...}}}.
	configRoot = "ddos/local"
)

type Config struct {
	ResponseLevel         string `json:"response-level"`
	MaxMitigationDuration int    `json:"max-mitigation-duration"`
	// ForwardMitigation, when true, lets the responder drop a REMOTE (transit) victim's
	// traffic on the netfilter FORWARD hook, protecting a downstream host. Default false:
	// the responder guards only LOCAL (box-owned) victims on INPUT and leaves remote
	// victims to the flowspec upstream announce. The exempt-vs-mitigate decision comes
	// from the detector's traffic policy via the event's SuppressMitigation flag; this
	// leaf only governs whether local also acts on the forwarding plane.
	ForwardMitigation bool `json:"forward-mitigation"`
	// ConfidenceMin (0-100) gates the characterized mitigation path: an
	// AttackCharacterized whose confidence is below this is not mitigated. 0 (default)
	// disables the gate -- behavior identical to before confidence existed.
	ConfidenceMin int `json:"confidence-min"`
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
	// Section is wrapped by ExtractConfigSubtree as {"ddos":{"local":{...}}}.
	ddos, ok := root["ddos"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	m, ok := ddos["local"].(map[string]any)
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
	if v, ok := m["forward-mitigation"]; ok {
		if b, ok := toBool(v); ok {
			cfg.ForwardMitigation = b
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
	case "alert", responseEnforce:
	default:
		return fmt.Errorf("response-level %q must be alert or enforce", c.ResponseLevel)
	}
	if c.MaxMitigationDuration < 0 || c.MaxMitigationDuration > 86400 {
		return fmt.Errorf("max-mitigation-duration %d out of range [0, 86400]", c.MaxMitigationDuration)
	}
	if c.ConfidenceMin < 0 || c.ConfidenceMin > 100 {
		return fmt.Errorf("confidence-min %d out of range [0, 100]", c.ConfidenceMin)
	}
	return nil
}

// The config framework delivers YANG leaf values as JSON strings (e.g. "3600"),
// so toInt accepts a string form alongside the native JSON number -- matching the
// rest of ze's plugin config parsers. Without the string case, every leaf silently
// falls back to its default.
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

// toBool coerces a config value (native JSON bool or the framework's string form) to
// bool, matching the other ddos config parsers.
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
