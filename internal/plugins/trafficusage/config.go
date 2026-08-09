// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage config parsing & validation

package trafficusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Defaults mirror the upstream lan-bandwidth-exporter (interval 1s, stale 5m,
// 10240 LRU entries per map). Interval and stale-timeout are configured in
// milliseconds (integer) so sub-second polling is expressible.
const (
	defaultInterval     = time.Second
	defaultStaleTimeout = 5 * time.Minute
	defaultMaxEntries   = uint32(10240)

	minInterval = 100 * time.Millisecond
	maxInterval = time.Hour
	minStale    = time.Second
)

// Config is the parsed traffic-usage configuration. Interval is global (a single
// poll loop). TrackIP, StaleTimeout, and MaxEntries here are the global defaults;
// each interface inherits them unless it sets its own override (resolved into
// InterfaceConfig at parse time).
type Config struct {
	Enabled      bool
	Interval     time.Duration
	TrackIP      bool
	StaleTimeout time.Duration
	MaxEntries   uint32
	Interfaces   []InterfaceConfig
}

// InterfaceConfig is one accounted interface with its effective (override or
// inherited-global) settings already resolved.
type InterfaceConfig struct {
	Name         string
	TrackIP      bool
	StaleTimeout time.Duration
	MaxEntries   uint32
}

// IsEmpty reports whether the config requests no accounting: the plugin is
// disabled. A reload that removes or disables the section parses to an empty
// Config and the engine tears down any running monitor.
func (c *Config) IsEmpty() bool {
	return !c.Enabled
}

// usageSubtree unwraps the two-level section wrapping that the plugin-server
// ExtractConfigSubtree helper produces for the "traffic/usage" config root:
// {"traffic": {"usage": {...}}}. Returns nil when either level is absent.
func usageSubtree(root map[string]any) map[string]any {
	traffic, ok := root["traffic"].(map[string]any)
	if !ok {
		return nil
	}
	usage, ok := traffic["usage"].(map[string]any)
	if !ok {
		return nil
	}
	return usage
}

// ParseConfig parses the traffic-usage JSON config section. The daemon delivers
// leaves as JSON strings ("2000"); array-form and unit-test configs embed JSON
// numbers and bools. Defaults are applied for absent leaves.
func ParseConfig(data string) (*Config, error) {
	cfg := &Config{
		Interval:     defaultInterval,
		StaleTimeout: defaultStaleTimeout,
		MaxEntries:   defaultMaxEntries,
	}
	if strings.TrimSpace(data) == "" {
		return cfg, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	// Section is wrapped by ExtractConfigSubtree as {"traffic":{"usage":{...}}}.
	tu := usageSubtree(root)
	if tu == nil {
		return cfg, nil
	}

	if v, ok := cfgBool(tu["enabled"]); ok {
		cfg.Enabled = v
	}
	if v, ok := cfgBool(tu["track-ip"]); ok {
		cfg.TrackIP = v
	}
	if ms, ok := cfgUint32(tu["interval"]); ok {
		cfg.Interval = time.Duration(ms) * time.Millisecond
	}
	if ms, ok := cfgUint32(tu["stale-timeout"]); ok {
		cfg.StaleTimeout = time.Duration(ms) * time.Millisecond
	}
	if v, ok := cfgUint32(tu["max-entries"]); ok {
		cfg.MaxEntries = v
	}
	// Resolve each interface's effective settings, inheriting the globals parsed
	// above unless the interface overrides them.
	cfg.Interfaces = parseInterfaces(tu["interfaces"], InterfaceConfig{
		TrackIP:      cfg.TrackIP,
		StaleTimeout: cfg.StaleTimeout,
		MaxEntries:   cfg.MaxEntries,
	})
	return cfg, nil
}

// Validate checks the config for operability. An enabled section with no
// (enabled) interface or an out-of-range interval is rejected, as is any
// interface whose name is invalid, whose effective stale-timeout is a non-zero
// sub-second value, or whose effective max-entries is zero.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if len(c.Interfaces) == 0 {
		errs = append(errs, errors.New("at least one enabled interface is required when enabled"))
	}
	if c.Interval < minInterval || c.Interval > maxInterval {
		errs = append(errs, fmt.Errorf("interval %v out of range %v..%v", c.Interval, minInterval, maxInterval))
	}
	for i := range c.Interfaces {
		ifc := &c.Interfaces[i]
		if !validInterfaceName(ifc.Name) {
			errs = append(errs, fmt.Errorf("invalid interface name %q", ifc.Name))
		}
		if ifc.StaleTimeout != 0 && ifc.StaleTimeout < minStale {
			errs = append(errs, fmt.Errorf("interface %q: stale-timeout %v must be 0 (disabled) or >= %v", ifc.Name, ifc.StaleTimeout, minStale))
		}
		if ifc.MaxEntries == 0 {
			errs = append(errs, fmt.Errorf("interface %q: max-entries must be >= 1", ifc.Name))
		}
	}
	return errors.Join(errs...)
}

// validInterfaceName accepts a ze interface name: 1-255 chars, leading
// alphanumeric, then alphanumeric or . _ - @ -- matching the YANG pattern
// '[A-Za-z0-9][A-Za-z0-9._@-]*'. The name is a logical ze name (resolved to the
// OS device via iface.Resolve), so it is not bound to the kernel's IFNAMSIZ; the
// resolved OS device name is bounded by the kernel at attach time. This is a
// defense-in-depth check; the YANG pattern is the primary gate.
func validInterfaceName(name string) bool {
	if name == "" || len(name) > 255 {
		return false
	}
	for i, r := range name {
		alnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		switch {
		case alnum:
		case i > 0 && (r == '.' || r == '_' || r == '-' || r == '@'):
		default:
			return false
		}
	}
	return true
}

// parseInterfaces reads the `interfaces { interface <name> { enabled; track-ip;
// stale-timeout; max-entries } }` keyed list (the OSPF/ISIS interface shape) and
// returns the resolved config for each enabled interface. def carries the global
// defaults each interface inherits unless it overrides them. The daemon delivers
// a YANG keyed list as a map of name -> body; array form is accepted for tests.
func parseInterfaces(raw any, def InterfaceConfig) []InterfaceConfig {
	container, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	switch entries := container["interface"].(type) {
	case map[string]any:
		out := make([]InterfaceConfig, 0, len(entries))
		for name, body := range entries {
			if ifc, ok := resolveInterface(name, body, def); ok {
				out = append(out, ifc)
			}
		}
		return out
	case []any:
		out := make([]InterfaceConfig, 0, len(entries))
		for _, e := range entries {
			body, _ := e.(map[string]any)
			name, _ := body["name"].(string)
			if ifc, ok := resolveInterface(name, body, def); ok {
				out = append(out, ifc)
			}
		}
		return out
	default:
		return nil
	}
}

// resolveInterface builds one InterfaceConfig from its list entry, inheriting
// def for any setting the entry does not override. It returns ok=false for an
// empty name or an entry with enabled=false (the `enabled` leaf defaults true,
// so an entry with no body is accounted on with the global defaults).
func resolveInterface(name string, body any, def InterfaceConfig) (InterfaceConfig, bool) {
	if name == "" {
		return InterfaceConfig{}, false
	}
	ifc := InterfaceConfig{Name: name, TrackIP: def.TrackIP, StaleTimeout: def.StaleTimeout, MaxEntries: def.MaxEntries}
	m, ok := body.(map[string]any)
	if !ok {
		return ifc, true
	}
	if en, ok := cfgBool(m["enabled"]); ok && !en {
		return InterfaceConfig{}, false
	}
	if v, ok := cfgBool(m["track-ip"]); ok {
		ifc.TrackIP = v
	}
	if ms, ok := cfgUint32(m["stale-timeout"]); ok {
		ifc.StaleTimeout = time.Duration(ms) * time.Millisecond
	}
	if v, ok := cfgUint32(m["max-entries"]); ok {
		ifc.MaxEntries = v
	}
	return ifc, true
}

// cfgUint32 coerces a config value (string or JSON number) to uint32.
func cfgUint32(v any) (uint32, bool) {
	switch n := v.(type) {
	case float64:
		return uint32(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return uint32(i), true
		}
	case string:
		if i, err := strconv.ParseUint(strings.TrimSpace(n), 10, 32); err == nil {
			return uint32(i), true
		}
	}
	return 0, false
}

// cfgBool coerces a config value (string or JSON bool) to bool.
func cfgBool(v any) (bool, bool) {
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
