// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- Flow export config parsing

package flowexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// Config values reach this parser two ways: the live daemon delivers config
// tree leaves as JSON strings ("6343"), while array-form and unit-test configs
// embed JSON numbers (6343) and bools (true). The cfg* helpers accept both so a
// value parses identically on either path -- the original float64/bool-only
// assertions silently dropped every daemon-delivered numeric field (port stayed
// at its default, rate became 0), which is why counter datagrams never arrived.

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

// cfgInt coerces a config value (string or JSON number) to int.
func cfgInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
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

// Config is the parsed flow-export configuration.
type Config struct {
	Collectors []CollectorConfig `json:"collectors"`
	Sampling   []SamplingConfig  `json:"sampling"`
	Conntrack  ConntrackConfig   `json:"conntrack"`
	Enrichment EnrichmentConfig  `json:"enrichment"`
}

// IsEmpty reports whether the config requests no flow-export activity:
// no collectors, no sampling interfaces, and conntrack export disabled.
// A reload that removes the flow-export section parses to an empty Config;
// the configure path tears the exporter down rather than building one.
func (c *Config) IsEmpty() bool {
	return len(c.Collectors) == 0 && len(c.Sampling) == 0 && !c.Conntrack.Enabled
}

// SamplingConfig describes packet sampling on one interface (spec 2).
// Sampled packets are exported as sFlow v5 flow samples.
type SamplingConfig struct {
	Interface string `json:"interface"`
	Rate      uint32 `json:"rate"`       // 1-in-N sampling rate
	TruncSize uint32 `json:"trunc-size"` // header bytes captured per sample
	Group     uint32 `json:"group"`      // psample group ID
}

// ConntrackConfig controls per-flow record export from conntrack (spec 2).
// Per-flow records are exported via NetFlow v9 and IPFIX collectors.
type ConntrackConfig struct {
	Enabled       bool `json:"enabled"`
	ActiveTimeout int  `json:"active-timeout"`   // seconds between conntrack dumps
	RecentRing    int  `json:"recent-flow-ring"` // recent-flow ring capacity (records) for `show flow recent`
}

// EnrichmentConfig controls BGP RIB enrichment of flow records (spec 2).
type EnrichmentConfig struct {
	BGP bool `json:"bgp"`
}

// CollectorConfig describes a single flow export collector endpoint.
type CollectorConfig struct {
	Name              string `json:"name"`
	Address           string `json:"address"`
	Port              int    `json:"port"`
	SourceAddress     string `json:"source-address"`
	Protocol          string `json:"protocol"`
	PollingInterval   int    `json:"polling-interval"`
	TemplateRefresh   int    `json:"template-refresh"`
	SubAgentID        uint32 `json:"sub-agent-id"`
	ObservationDomain uint32 `json:"observation-domain"`
	AgentAddress      string `json:"agent-address"`
}

// ParseConfig parses the flow-export JSON config section.
// YANG list with key "name" arrives as a keyed map:
// {"flow-export":{"collector":{"c1":{...},"c2":{...}}}}
// Also handles array form for test convenience.
func ParseConfig(data string) (*Config, error) {
	if strings.TrimSpace(data) == "" {
		return &Config{}, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("flow-export config: unmarshal: %w", err)
	}

	feMap, ok := root["flow-export"].(map[string]any)
	if !ok {
		return &Config{}, nil
	}

	cfg := &Config{}

	switch v := feMap["collector"].(type) {
	case map[string]any:
		// YANG keyed-map: values are sub-maps (collector configs).
		// Single-collector shorthand: values are scalars (address string, port number).
		isKeyedMap := false
		for _, val := range v {
			if _, ok := val.(map[string]any); ok {
				isKeyedMap = true
				break
			}
		}
		if isKeyedMap {
			for name, entry := range v {
				em, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				cfg.Collectors = append(cfg.Collectors, parseCollectorMap(name, em))
			}
		} else {
			cfg.Collectors = append(cfg.Collectors, parseCollectorMap("", v))
		}
	case []any:
		for _, raw := range v {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			cfg.Collectors = append(cfg.Collectors, parseCollectorMap("", m))
		}
	}

	cfg.Sampling = parseSampling(feMap["sampling"])
	cfg.Conntrack = parseConntrack(feMap["conntrack"])
	cfg.Enrichment = parseEnrichment(feMap["enrichment"])

	return cfg, nil
}

// parseSampling parses the sampling container. The interface list arrives
// as a YANG keyed map: {"sampling":{"interface":{"eth0":{...}}}}.
func parseSampling(raw any) []SamplingConfig {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	var out []SamplingConfig
	switch ifs := m["interface"].(type) {
	case map[string]any:
		for name, entry := range ifs {
			em, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, parseSamplingEntry(name, em))
		}
	case []any:
		for _, raw := range ifs {
			em, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, parseSamplingEntry("", em))
		}
	}
	return out
}

func parseSamplingEntry(name string, m map[string]any) SamplingConfig {
	s := SamplingConfig{
		Interface: name,
		TruncSize: 128,
		Group:     1,
	}
	if v, ok := m["interface"].(string); ok {
		s.Interface = v
	}
	if v, ok := m["name"].(string); ok && s.Interface == "" {
		s.Interface = v
	}
	if v, ok := cfgUint32(m["rate"]); ok {
		s.Rate = v
	}
	if v, ok := cfgUint32(m["trunc-size"]); ok {
		s.TruncSize = v
	}
	if v, ok := cfgUint32(m["group"]); ok {
		s.Group = v
	}
	return s
}

func parseConntrack(raw any) ConntrackConfig {
	c := ConntrackConfig{ActiveTimeout: 60, RecentRing: 4096}
	m, ok := raw.(map[string]any)
	if !ok {
		return c
	}
	if v, ok := cfgBool(m["enabled"]); ok {
		c.Enabled = v
	}
	if v, ok := cfgInt(m["active-timeout"]); ok {
		c.ActiveTimeout = v
	}
	if v, ok := cfgInt(m["recent-flow-ring"]); ok {
		c.RecentRing = v
	}
	return c
}

func parseEnrichment(raw any) EnrichmentConfig {
	var e EnrichmentConfig
	m, ok := raw.(map[string]any)
	if !ok {
		return e
	}
	if v, ok := cfgBool(m["bgp"]); ok {
		e.BGP = v
	}
	return e
}

func parseCollectorMap(name string, m map[string]any) CollectorConfig {
	c := CollectorConfig{
		Name:            name,
		PollingInterval: 20,
		TemplateRefresh: 600,
		Port:            6343,
	}
	if v, ok := m["name"].(string); ok {
		c.Name = v
	}
	if v, ok := m["address"].(string); ok {
		c.Address = v
	}
	if v, ok := m["source-address"].(string); ok {
		c.SourceAddress = v
	}
	if v, ok := cfgInt(m["port"]); ok {
		c.Port = v
	}
	if v, ok := m["protocol"].(string); ok {
		c.Protocol = v
	}
	if v, ok := cfgInt(m["polling-interval"]); ok {
		c.PollingInterval = v
	}
	if v, ok := cfgInt(m["template-refresh"]); ok {
		c.TemplateRefresh = v
	}
	if v, ok := cfgUint32(m["sub-agent-id"]); ok {
		c.SubAgentID = v
	}
	if v, ok := cfgUint32(m["observation-domain"]); ok {
		c.ObservationDomain = v
	}
	if v, ok := m["agent-address"].(string); ok {
		c.AgentAddress = v
	}
	return c
}

// Validate checks all collector and sampling configs for correctness.
func (c *Config) Validate() error {
	var errs []error
	for i := range c.Collectors {
		if err := c.Collectors[i].validate(); err != nil {
			errs = append(errs, fmt.Errorf("collector %q: %w", c.Collectors[i].Name, err))
		}
	}
	for i := range c.Sampling {
		if err := c.Sampling[i].validate(); err != nil {
			errs = append(errs, fmt.Errorf("sampling %q: %w", c.Sampling[i].Interface, err))
		}
	}
	if c.Conntrack.Enabled {
		if c.Conntrack.ActiveTimeout < 1 || c.Conntrack.ActiveTimeout > 3600 {
			errs = append(errs, fmt.Errorf("conntrack active-timeout %d out of range 1-3600", c.Conntrack.ActiveTimeout))
		}
		if c.Conntrack.RecentRing < 64 || c.Conntrack.RecentRing > 65536 {
			errs = append(errs, fmt.Errorf("conntrack recent-flow-ring %d out of range 64-65536", c.Conntrack.RecentRing))
		}
	}
	return errors.Join(errs...)
}

func (s *SamplingConfig) validate() error {
	var errs []error
	if s.Interface == "" {
		errs = append(errs, errors.New("interface is required"))
	}
	// spec-flow-export-2: sampling rate 1-1000000 (1-in-N).
	if s.Rate < 1 || s.Rate > 1000000 {
		errs = append(errs, fmt.Errorf("rate %d out of range 1-1000000", s.Rate))
	}
	// spec-flow-export-2: trunc-size 64-1500 bytes.
	if s.TruncSize < 64 || s.TruncSize > 1500 {
		errs = append(errs, fmt.Errorf("trunc-size %d out of range 64-1500", s.TruncSize))
	}
	// spec-flow-export-2: psample group 1-2147483647.
	if s.Group < 1 || s.Group > 2147483647 {
		errs = append(errs, fmt.Errorf("group %d out of range 1-2147483647", s.Group))
	}
	return errors.Join(errs...)
}

func (c *CollectorConfig) validate() error {
	var errs []error

	if c.Address == "" {
		errs = append(errs, errors.New("address is required"))
	} else if _, err := netip.ParseAddr(c.Address); err != nil {
		errs = append(errs, fmt.Errorf("address %q: %w", c.Address, err))
	}

	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port %d out of range 1-65535", c.Port))
	}

	switch c.Protocol {
	case "sflow", "netflow9", "ipfix":
	case "":
		errs = append(errs, errors.New("protocol is required"))
	default:
		errs = append(errs, fmt.Errorf("unknown protocol %q (sflow, netflow9, ipfix)", c.Protocol))
	}

	if c.PollingInterval < 1 || c.PollingInterval > 3600 {
		errs = append(errs, fmt.Errorf("polling-interval %d out of range 1-3600", c.PollingInterval))
	}

	if c.TemplateRefresh < 1 || c.TemplateRefresh > 86400 {
		errs = append(errs, fmt.Errorf("template-refresh %d out of range 1-86400", c.TemplateRefresh))
	}

	// agent-address is optional, but if set it must be a valid IP: the sFlow
	// encoder otherwise silently falls back to 0.0.0.0, breaking collector
	// correlation with no operator-visible error.
	if c.AgentAddress != "" {
		if _, err := netip.ParseAddr(c.AgentAddress); err != nil {
			errs = append(errs, fmt.Errorf("agent-address %q: %w", c.AgentAddress, err))
		}
	}

	return errors.Join(errs...)
}
