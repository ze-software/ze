// Design: plan/spec-healthcheck-0-umbrella.md -- healthcheck config parsing
package healthcheck

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ProbeConfig holds parsed configuration for a single healthcheck probe.
// Note: struct equality is used for config change detection. Slice fields
// (IPs, hooks) compare by length+content, so reordering triggers reconfigure.
type ProbeConfig struct {
	Name           string
	Command        string
	Group          string
	Interval       uint32
	FastInterval   uint32
	Timeout        uint32
	Rise           uint32
	Fall           uint32
	WithdrawOnDown bool
	Disable        bool
	Debounce       bool
	UpMetric       uint32
	DownMetric     uint32
	DisabledMetric uint32

	// IP management (internal mode only).
	IPInterface string
	IPDynamic   bool
	IPs         []string // CIDRs

	// Hooks (shell commands, 30s timeout each).
	OnUp       []string
	OnDown     []string
	OnDisabled []string
	OnChange   []string
}

// equal compares two ProbeConfigs including slice fields.
func (c ProbeConfig) equal(other ProbeConfig) bool {
	return reflect.DeepEqual(c, other)
}

// parseConfig extracts healthcheck probe definitions from a BGP config JSON tree.
// The JSON has the structure: {"bgp": {"healthcheck": {"probe": {"name1": {...}, ...}}}}.
func parseConfig(jsonData string) ([]ProbeConfig, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	bgpTree, ok := getMap(tree, "bgp")
	if !ok {
		return nil, nil
	}

	hcTree, ok := getMap(bgpTree, "healthcheck")
	if !ok {
		return nil, nil
	}

	probeMap, ok := getMap(hcTree, "probe")
	if !ok {
		return nil, nil
	}

	var probes []ProbeConfig
	for name, data := range probeMap {
		m, ok := data.(map[string]any)
		if !ok {
			continue
		}
		cfg := ProbeConfig{
			Name:           name,
			Command:        getString(m, "command"),
			Group:          getString(m, "group"),
			Interval:       getUint32(m, "interval", 5),
			FastInterval:   getUint32(m, "fast-interval", 1),
			Timeout:        getUint32(m, "timeout", 5),
			Rise:           getUint32(m, "rise", 3),
			Fall:           getUint32(m, "fall", 3),
			WithdrawOnDown: getBool(m, "withdraw-on-down"),
			Disable:        getBool(m, "disable"),
			Debounce:       getBool(m, "debounce"),
			UpMetric:       getUint32(m, "up-metric", 100),
			DownMetric:     getUint32(m, "down-metric", 1000),
			DisabledMetric: getUint32(m, "disabled-metric", 500),
			OnUp:           getStringList(m, "on-up"),
			OnDown:         getStringList(m, "on-down"),
			OnDisabled:     getStringList(m, "on-disabled"),
			OnChange:       getStringList(m, "on-change"),
		}

		// Parse ip-setup container.
		if ipSetup, ok := getMap(m, "ip-setup"); ok {
			cfg.IPInterface = getString(ipSetup, "interface")
			cfg.IPDynamic = getBool(ipSetup, "dynamic")
			cfg.IPs = getStringList(ipSetup, "ip")
		}
		if cfg.Command == "" {
			return nil, fmt.Errorf("probe %q: command is required", name)
		}
		if cfg.Group == "" {
			return nil, fmt.Errorf("probe %q: group is required", name)
		}
		if cfg.Group == "med" {
			return nil, fmt.Errorf("probe %q: 'med' is not allowed as a group name (ambiguous with watchdog med argument)", name)
		}
		probes = append(probes, cfg)
	}

	// Check group uniqueness.
	groups := make(map[string]string, len(probes))
	for _, p := range probes {
		if other, exists := groups[p.Group]; exists {
			return nil, fmt.Errorf("duplicate group %q: probes %q and %q", p.Group, other, p.Name)
		}
		groups[p.Group] = p.Name
	}

	return probes, nil
}

func getMap(m map[string]any, key string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	sub, ok := v.(map[string]any)
	return sub, ok
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func getUint32(m map[string]any, key string, defaultVal uint32) uint32 {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return uint32(n)
	case string:
		var val uint32
		if _, err := fmt.Sscanf(n, "%d", &val); err == nil {
			return val
		}
	}
	return defaultVal
}

func getBool(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
}

func getStringList(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch list := v.(type) {
	case []any:
		result := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		if list == "" {
			return nil
		}
		return []string{list}
	}
	return nil
}
