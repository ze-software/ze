// Design: docs/research/vpp-deployment-reference.md -- VPP startup.conf values and NIC driver matrix
// Detail: startupconf.go -- startup.conf generation from VPPSettings
// Detail: dpdk.go -- DPDK NIC driver binding using DPDKInterface
//
// Package vpp manages VPP's full process lifecycle as a self-contained system.
// The Manager type owns startup, health monitoring, crash recovery, and clean
// shutdown. See vpp.go for the lifecycle loop.

package vpp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// VPPSettings holds parsed VPP configuration from the YANG config tree.
type VPPSettings struct {
	Enabled   bool
	External  bool // true: ze connects via GoVPP but does not exec/supervise the VPP binary
	APISocket string
	CPU       CPUSettings
	Memory    MemorySettings
	DPDK      DPDKSettings
	Stats     StatsSettings
	LCP       LCPSettings
}

// CPUSettings holds VPP CPU pinning settings.
type CPUSettings struct {
	MainCore *uint8 // nil = auto
	Workers  *uint8 // nil = auto
}

// MemorySettings holds VPP memory and buffer settings.
type MemorySettings struct {
	MainHeap     string // e.g. "1G", "1536M"
	HugepageSize string // "2M" or "1G"
	Buffers      uint32
}

// DPDKSettings holds DPDK NIC configuration.
type DPDKSettings struct {
	Interfaces []DPDKInterface
}

// DPDKInterface represents a single DPDK-managed NIC.
type DPDKInterface struct {
	PCIAddress string
	Name       string
	RxQueues   *uint8 // nil = VPP default
	TxQueues   *uint8 // nil = VPP default
}

// StatsSettings holds VPP stats segment settings.
type StatsSettings struct {
	SegmentSize  string
	SocketPath   string
	PollInterval uint16 // seconds, 1-3600, default 30
}

// LCPSettings holds Linux Control Plane plugin settings.
type LCPSettings struct {
	Enabled    bool
	Sync       bool
	AutoSubint bool
	Netns      string
}

// yangTrue is the string representation of boolean true in YANG config JSON.
const yangTrue = "true"

// pciAddressRE validates PCI bus addresses in DDDD:DD:DD.D format.
var pciAddressRE = regexp.MustCompile(`^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]$`)

// sizeRE validates VPP size values like "512M", "1G", "1536M".
var sizeRE = regexp.MustCompile(`^\d+[MmGg]$`)

// validateSize checks that a VPP size string matches the expected format.
func validateSize(field, value string) error {
	if !sizeRE.MatchString(value) {
		return fmt.Errorf("vpp %s: invalid size %q (expected e.g. 512M, 1G, 1536M)", field, value)
	}
	return nil
}

// validateSocketPath checks that a path looks like a Unix socket path.
func validateSocketPath(field, path string) error {
	if path == "" {
		return fmt.Errorf("vpp %s: must not be empty", field)
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("vpp %s: must be absolute path, got %q", field, path)
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("vpp %s: must not contain '..', got %q", field, path)
	}
	if len(path) > 108 {
		return fmt.Errorf("vpp %s: path too long (%d > 108 chars, Unix socket limit)", field, len(path))
	}
	return nil
}

// validateNetns checks that a network namespace name is reasonable.
func validateNetns(name string) error {
	if name == "" {
		return fmt.Errorf("vpp lcp: netns must not be empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("vpp lcp: netns must not contain path separators, got %q", name)
	}
	if len(name) > 255 {
		return fmt.Errorf("vpp lcp: netns too long (%d > 255 chars)", len(name))
	}
	return nil
}

// ifaceNameRE validates VPP interface short names (alphanumeric + hyphen/underscore).
var ifaceNameRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,14}$`)

// validateIfaceName checks that an interface name is safe for startup.conf.
func validateIfaceName(name string) error {
	if !ifaceNameRE.MatchString(name) {
		return fmt.Errorf("vpp: interface name %q invalid (alphanumeric, 1-15 chars, starts with letter)", name)
	}
	return nil
}

// unknownKeys returns an error if raw contains any key not in known.
func unknownKeys(context string, raw map[string]json.RawMessage, known []string) error {
	set := make(map[string]bool, len(known))
	for _, k := range known {
		set[k] = true
	}
	for k := range raw {
		if !set[k] {
			return fmt.Errorf("vpp %s: unknown key %q", context, k)
		}
	}
	return nil
}

// ValidatePCIAddress checks that addr matches the PCI bus address format.
func ValidatePCIAddress(addr string) error {
	if !pciAddressRE.MatchString(addr) {
		return fmt.Errorf("invalid PCI address %q: expected DDDD:DD:DD.D format (e.g. 0000:03:00.0)", addr)
	}
	return nil
}

// ParseConfigSection parses a wrapped VPP config section delivered by the
// plugin-server `ExtractConfigSubtree` helper. That helper wraps every
// subtree in its path structure, so a section for the "vpp" root arrives
// as `{"vpp": {...}}` rather than the bare `{...}` that ParseSettings
// operates on. This function unwraps the "vpp" root and delegates to
// ParseSettings.
//
// Use this from plugin OnConfigure callbacks. Use ParseSettings directly
// from tests or callers that already hold the inner subtree.
func ParseConfigSection(data string) (*VPPSettings, error) {
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &wrapped); err != nil {
		return nil, fmt.Errorf("vpp: parse wrapped config: %w", err)
	}
	inner, ok := wrapped["vpp"]
	if !ok {
		return nil, fmt.Errorf("vpp: config section missing 'vpp' root")
	}
	parsed, err := ParseSettings(inner)
	if err != nil {
		return nil, fmt.Errorf("vpp: parse config: %w", err)
	}
	return parsed, nil
}

// ParseSettings extracts VPP configuration from a YANG config JSON section.
// The section is the "vpp" subtree from the config tree.
func ParseSettings(section json.RawMessage) (*VPPSettings, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(section, &raw); err != nil {
		return nil, fmt.Errorf("vpp config: %w", err)
	}

	if err := unknownKeys("config", raw, []string{
		"enabled", "external", "api-socket", "cpu", "memory", "dpdk", "stats", "lcp",
	}); err != nil {
		return nil, err
	}

	cfg := &VPPSettings{
		APISocket: "/run/vpp/api.sock",
		Memory: MemorySettings{
			MainHeap:     "1G",
			HugepageSize: "2M",
			Buffers:      128000,
		},
		Stats: StatsSettings{
			SegmentSize:  "512M",
			SocketPath:   "/run/vpp/stats.sock",
			PollInterval: 30,
		},
		LCP: LCPSettings{
			Enabled:    true,
			Sync:       true,
			AutoSubint: true,
			Netns:      "dataplane",
		},
	}

	if v, ok := raw["enabled"]; ok {
		cfg.Enabled = strings.Trim(string(v), `"`) == yangTrue
	}
	if v, ok := raw["external"]; ok {
		cfg.External = strings.Trim(string(v), `"`) == yangTrue
	}
	if v, ok := raw["api-socket"]; ok {
		cfg.APISocket = strings.Trim(string(v), `"`)
	}
	if v, ok := raw["cpu"]; ok {
		if err := parseCPU(v, &cfg.CPU); err != nil {
			return nil, err
		}
	}
	if v, ok := raw["memory"]; ok {
		if err := parseMemory(v, &cfg.Memory); err != nil {
			return nil, err
		}
	}
	if v, ok := raw["dpdk"]; ok {
		if err := parseDPDK(v, &cfg.DPDK); err != nil {
			return nil, err
		}
	}
	if v, ok := raw["stats"]; ok {
		if err := parseStats(v, &cfg.Stats); err != nil {
			return nil, err
		}
	}
	if v, ok := raw["lcp"]; ok {
		if err := parseLCP(v, &cfg.LCP); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// Validate checks the settings for semantic errors beyond YANG schema validation.
func (s *VPPSettings) Validate() error {
	if !s.Enabled {
		return nil
	}

	if err := validateSocketPath("api-socket", s.APISocket); err != nil {
		return err
	}

	if err := validateSize("memory main-heap", s.Memory.MainHeap); err != nil {
		return err
	}
	if s.Memory.Buffers == 0 {
		return fmt.Errorf("vpp: memory buffers must be > 0")
	}

	if err := validateSize("stats segment-size", s.Stats.SegmentSize); err != nil {
		return err
	}
	if err := validateSocketPath("stats socket-path", s.Stats.SocketPath); err != nil {
		return err
	}

	if s.LCP.Enabled {
		if err := validateNetns(s.LCP.Netns); err != nil {
			return err
		}
	}

	for i, iface := range s.DPDK.Interfaces {
		if err := ValidatePCIAddress(iface.PCIAddress); err != nil {
			return fmt.Errorf("vpp: dpdk interface %d: %w", i, err)
		}
		if err := validateIfaceName(iface.Name); err != nil {
			return fmt.Errorf("vpp: dpdk interface %d (%s): %w", i, iface.PCIAddress, err)
		}
	}

	return nil
}

func parseCPU(data json.RawMessage, cpu *CPUSettings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("vpp cpu: %w", err)
	}
	if err := unknownKeys("cpu", raw, []string{"main-core", "workers"}); err != nil {
		return err
	}
	if v, ok := raw["main-core"]; ok {
		n, err := parseUint8(v)
		if err != nil {
			return fmt.Errorf("vpp cpu main-core: %w", err)
		}
		cpu.MainCore = &n
	}
	if v, ok := raw["workers"]; ok {
		n, err := parseUint8(v)
		if err != nil {
			return fmt.Errorf("vpp cpu workers: %w", err)
		}
		cpu.Workers = &n
	}
	return nil
}

func parseMemory(data json.RawMessage, mem *MemorySettings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("vpp memory: %w", err)
	}
	if err := unknownKeys("memory", raw, []string{"main-heap", "hugepage-size", "buffers"}); err != nil {
		return err
	}
	if v, ok := raw["main-heap"]; ok {
		mem.MainHeap = strings.Trim(string(v), `"`)
	}
	if v, ok := raw["hugepage-size"]; ok {
		s := strings.Trim(string(v), `"`)
		if s != "2M" && s != "1G" {
			return fmt.Errorf("vpp memory hugepage-size: must be 2M or 1G, got %q", s)
		}
		mem.HugepageSize = s
	}
	if v, ok := raw["buffers"]; ok {
		n, err := parseUint32(v)
		if err != nil {
			return fmt.Errorf("vpp memory buffers: %w", err)
		}
		mem.Buffers = n
	}
	return nil
}

func parseDPDK(data json.RawMessage, dpdk *DPDKSettings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("vpp dpdk: %w", err)
	}
	ifaceData, ok := raw["interface"]
	if !ok {
		return nil
	}

	// YANG list is a JSON object keyed by the list key (pci-address).
	var ifaceMap map[string]json.RawMessage
	if err := json.Unmarshal(ifaceData, &ifaceMap); err != nil {
		return fmt.Errorf("vpp dpdk interface: %w", err)
	}
	// Sort PCI addresses for deterministic interface ordering in startup.conf.
	pciAddrs := make([]string, 0, len(ifaceMap))
	for pci := range ifaceMap {
		pciAddrs = append(pciAddrs, pci)
	}
	sort.Strings(pciAddrs)
	for _, pci := range pciAddrs {
		entry := ifaceMap[pci]
		iface := DPDKInterface{PCIAddress: pci}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(entry, &fields); err != nil {
			return fmt.Errorf("vpp dpdk interface %s: %w", pci, err)
		}
		if v, ok := fields["name"]; ok {
			iface.Name = strings.Trim(string(v), `"`)
		}
		if v, ok := fields["rx-queues"]; ok {
			n, err := parseUint8(v)
			if err != nil {
				return fmt.Errorf("vpp dpdk interface %s rx-queues: %w", pci, err)
			}
			iface.RxQueues = &n
		}
		if v, ok := fields["tx-queues"]; ok {
			n, err := parseUint8(v)
			if err != nil {
				return fmt.Errorf("vpp dpdk interface %s tx-queues: %w", pci, err)
			}
			iface.TxQueues = &n
		}
		dpdk.Interfaces = append(dpdk.Interfaces, iface)
	}
	return nil
}

func parseStats(data json.RawMessage, stats *StatsSettings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("vpp stats: %w", err)
	}
	if err := unknownKeys("stats", raw, []string{"segment-size", "socket-path", "poll-interval"}); err != nil {
		return err
	}
	if v, ok := raw["segment-size"]; ok {
		stats.SegmentSize = strings.Trim(string(v), `"`)
	}
	if v, ok := raw["socket-path"]; ok {
		stats.SocketPath = strings.Trim(string(v), `"`)
	}
	if v, ok := raw["poll-interval"]; ok {
		n, err := parseUint16(v)
		if err != nil {
			return fmt.Errorf("vpp stats poll-interval: %w", err)
		}
		if n < 1 || n > 3600 {
			return fmt.Errorf("vpp stats poll-interval: must be 1..3600, got %d", n)
		}
		stats.PollInterval = n
	}
	return nil
}

func parseLCP(data json.RawMessage, lcp *LCPSettings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("vpp lcp: %w", err)
	}
	if err := unknownKeys("lcp", raw, []string{"enabled", "sync", "auto-subint", "netns"}); err != nil {
		return err
	}
	if v, ok := raw["enabled"]; ok {
		lcp.Enabled = strings.Trim(string(v), `"`) == yangTrue
	}
	if v, ok := raw["sync"]; ok {
		lcp.Sync = strings.Trim(string(v), `"`) == yangTrue
	}
	if v, ok := raw["auto-subint"]; ok {
		lcp.AutoSubint = strings.Trim(string(v), `"`) == yangTrue
	}
	if v, ok := raw["netns"]; ok {
		lcp.Netns = strings.Trim(string(v), `"`)
	}
	return nil
}

func parseUint16(data json.RawMessage) (uint16, error) {
	s := strings.Trim(string(data), `"`)
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("expected uint16: %w", err)
	}
	return uint16(n), nil
}

func parseUint8(data json.RawMessage) (uint8, error) {
	s := strings.Trim(string(data), `"`)
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("expected uint8: %w", err)
	}
	return uint8(n), nil
}

func parseUint32(data json.RawMessage) (uint32, error) {
	s := strings.Trim(string(data), `"`)
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("expected uint32: %w", err)
	}
	return uint32(n), nil
}
