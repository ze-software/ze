// Design: docs/architecture/config/syntax.md — system identity config extraction

package system

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/host"
)

// SystemConfig holds system-wide identity configuration.
// Extracted from the system {} block in config.
type SystemConfig struct {
	Host   string
	Domain string

	// Static DNS name servers (from system { name-server [...] }).
	NameServers []string

	// DNS resolver tuning (from system { dns {} }).
	DNSTimeout     uint16
	DNSCacheSize   uint32
	DNSCacheTTL    uint32
	ResolvConfPath string

	// PeeringDB API settings for prefix data lookups.
	PeeringDBURL    string
	PeeringDBMargin uint8

	// Hardware tuning (from system { tuning {} }).
	Tuning TuningSystemConfig

	// Console devices (from system { console { device ... } }).
	ConsoleDevices []ConsoleDeviceEntry

	// Update check (from system { update-check {} }).
	UpdateCheckURL      string
	UpdateCheckInterval uint32

	// Connection tracking (from system { conntrack {} }).
	Conntrack ConntrackConfig
}

// TuningSystemConfig holds hardware tuning settings from config.
type TuningSystemConfig struct {
	CPUGovernor  string
	IRQAffinity  []IRQAffinityEntry
	EthtoolRings []EthtoolRingEntry
}

// IRQAffinityEntry maps a NIC to its desired CPU affinity.
type IRQAffinityEntry struct {
	Interface string
	CPUs      string
}

// EthtoolRingEntry holds per-interface ring buffer config.
type EthtoolRingEntry struct {
	Interface string
	RingRx    int
	RingTx    int
}

// ToHostTuningConfig converts config-extracted tuning into the host
// package's TuningConfig for ApplyTuning.
func (tc TuningSystemConfig) ToHostTuningConfig() host.TuningConfig {
	cfg := host.TuningConfig{CPUGovernor: tc.CPUGovernor}
	for _, irq := range tc.IRQAffinity {
		cfg.IRQAffinity = append(cfg.IRQAffinity, host.IRQAffinityConfig{
			Interface: irq.Interface,
			CPUs:      irq.CPUs,
		})
	}
	for _, eth := range tc.EthtoolRings {
		cfg.Ethtool = append(cfg.Ethtool, host.EthtoolConfig{
			Interface: eth.Interface,
			RingRx:    eth.RingRx,
			RingTx:    eth.RingTx,
		})
	}
	return cfg
}

// ExtractTuningFromMap extracts tuning config from a map[string]any
// tree (used by the reload path which has no *config.Tree).
func ExtractTuningFromMap(tree map[string]any) TuningSystemConfig {
	var tc TuningSystemConfig
	sys, _ := tree["system"].(map[string]any)
	if sys == nil {
		return tc
	}
	tuning, _ := sys["tuning"].(map[string]any)
	if tuning == nil {
		return tc
	}
	if cpu, _ := tuning["cpu"].(map[string]any); cpu != nil {
		if gov, _ := cpu["governor"].(string); gov != "" {
			tc.CPUGovernor = gov
		}
	}
	if irqList, _ := tuning["irq-affinity"].(map[string]any); irqList != nil {
		for iface, entry := range irqList {
			if m, _ := entry.(map[string]any); m != nil {
				if cpus, _ := m["cpus"].(string); cpus != "" {
					tc.IRQAffinity = append(tc.IRQAffinity, IRQAffinityEntry{
						Interface: iface,
						CPUs:      cpus,
					})
				}
			}
		}
	}
	if ethList, _ := tuning["ethtool"].(map[string]any); ethList != nil {
		for iface, entry := range ethList {
			if m, _ := entry.(map[string]any); m != nil {
				e := EthtoolRingEntry{Interface: iface}
				if ring, _ := m["ring"].(map[string]any); ring != nil {
					if rx, _ := ring["rx"].(string); rx != "" {
						var n int
						if _, err := fmt.Sscanf(rx, "%d", &n); err == nil && n >= 1 && n <= 65535 {
							e.RingRx = n
						}
					}
					if tx, _ := ring["tx"].(string); tx != "" {
						var n int
						if _, err := fmt.Sscanf(tx, "%d", &n); err == nil && n >= 1 && n <= 65535 {
							e.RingTx = n
						}
					}
				}
				if e.RingRx > 0 || e.RingTx > 0 {
					tc.EthtoolRings = append(tc.EthtoolRings, e)
				}
			}
		}
	}
	return tc
}

// ExpandEnvValue resolves $ENV_VAR references in config values.
// If the value starts with $, the remainder is treated as an OS environment
// variable name. If the env var is empty or unset, the literal string is returned.
// Non-$ values are returned as-is.
func ExpandEnvValue(s string) string {
	if s == "" || s[0] != '$' {
		return s
	}

	envName := s[1:]
	if v := os.Getenv(envName); v != "" {
		return v
	}

	return s
}

// ExtractSystemConfig extracts system identity config from a parsed Tree.
// Reads system.host and system.domain, applying $ENV expansion.
// Returns defaults (host="unknown", domain="") if the system block is absent.
func ExtractSystemConfig(tree *config.Tree) SystemConfig {
	sc := SystemConfig{
		Host:            "unknown",
		DNSTimeout:      5,
		DNSCacheSize:    10000,
		DNSCacheTTL:     86400,
		ResolvConfPath:  "/tmp/resolv.conf",
		PeeringDBURL:    "https://www.peeringdb.com",
		PeeringDBMargin: 10,
	}

	sys := tree.GetContainer("system")
	if sys == nil {
		return sc
	}

	if host, ok := sys.Get("host"); ok {
		sc.Host = ExpandEnvValue(host)
	}

	if domain, ok := sys.Get("domain"); ok {
		sc.Domain = ExpandEnvValue(domain)
	}

	if servers := sys.GetSlice("name-server"); len(servers) > 0 {
		sc.NameServers = servers
	}

	if dns := sys.GetContainer("dns"); dns != nil {
		if v, ok := dns.Get("resolv-conf-path"); ok {
			sc.ResolvConfPath = sanitizeResolvConfPath(v)
		}
		if v, ok := dns.Get("timeout"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 1 && n <= 60 {
				sc.DNSTimeout = uint16(n) //nolint:gosec // Bounded by range check above
			}
		}
		if v, ok := dns.Get("cache-size"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 && n <= 1000000 {
				sc.DNSCacheSize = uint32(n) //nolint:gosec // Bounded by range check above
			}
		}
		if v, ok := dns.Get("cache-ttl"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 && n <= 604800 {
				sc.DNSCacheTTL = uint32(n) //nolint:gosec // Bounded by range check above
			}
		}
	}

	sc.Tuning = extractTuning(sys)
	sc.ConsoleDevices = extractConsole(sys)
	sc.Conntrack = extractConntrack(sys)

	if uc := sys.GetContainer("update-check"); uc != nil {
		if url, ok := uc.Get("url"); ok {
			sc.UpdateCheckURL = url
		}
		sc.UpdateCheckInterval = 86400
		if v, ok := uc.Get("interval"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 60 && n <= 604800 {
				sc.UpdateCheckInterval = uint32(n) //nolint:gosec // Bounded by range check above
			}
		}
	}

	pdb := sys.GetContainer("peeringdb")
	if pdb == nil {
		return sc
	}

	if url, ok := pdb.Get("url"); ok {
		sc.PeeringDBURL = url
	}

	if margin, ok := pdb.Get("margin"); ok {
		var v int
		if _, err := fmt.Sscanf(margin, "%d", &v); err == nil && v >= 0 && v <= 100 {
			sc.PeeringDBMargin = uint8(v) //nolint:gosec // Bounded by range check above
		}
	}

	return sc
}

func extractTuning(sys *config.Tree) TuningSystemConfig {
	var tc TuningSystemConfig

	tuning := sys.GetContainer("tuning")
	if tuning == nil {
		return tc
	}

	if cpu := tuning.GetContainer("cpu"); cpu != nil {
		if gov, ok := cpu.Get("governor"); ok {
			tc.CPUGovernor = gov
		}
	}

	for key, irq := range tuning.GetList("irq-affinity") {
		cpus, _ := irq.Get("cpus")
		if key != "" && cpus != "" {
			tc.IRQAffinity = append(tc.IRQAffinity, IRQAffinityEntry{
				Interface: key,
				CPUs:      cpus,
			})
		}
	}

	for key, eth := range tuning.GetList("ethtool") {
		if key == "" {
			continue
		}
		entry := EthtoolRingEntry{Interface: key}
		if ring := eth.GetContainer("ring"); ring != nil {
			if rx, ok := ring.Get("rx"); ok {
				var n int
				if _, err := fmt.Sscanf(rx, "%d", &n); err == nil && n >= 1 && n <= 65535 {
					entry.RingRx = n
				}
			}
			if tx, ok := ring.Get("tx"); ok {
				var n int
				if _, err := fmt.Sscanf(tx, "%d", &n); err == nil && n >= 1 && n <= 65535 {
					entry.RingTx = n
				}
			}
		}
		if entry.RingRx > 0 || entry.RingTx > 0 {
			tc.EthtoolRings = append(tc.EthtoolRings, entry)
		}
	}

	return tc
}

// sanitizeResolvConfPath validates and cleans a resolv-conf-path value.
// Rejects relative paths and path traversal; returns empty string (disabling
// resolv.conf writing) for invalid input.
func sanitizeResolvConfPath(v string) string {
	if v == "" {
		return ""
	}
	if !filepath.IsAbs(v) {
		return ""
	}
	if filepath.Clean(v) != v {
		return ""
	}
	return v
}
