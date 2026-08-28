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
	"errors"
	"fmt"
	"regexp"

	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/naming"
)

var (
	errVppConfigSectionMissingVppRoot = errors.New("vpp: config section missing 'vpp' root")
	errVppMemoryBuffersMustBe0        = errors.New("vpp: memory buffers must be > 0")
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
	Plugins   PluginSettings
}

// PluginSettings holds optional VPP plugin enablement toggles. startup.conf
// disables plugins by default (plugin default { disable }); each toggle here
// emits an explicit `plugin <name>.so { enable }`.
type PluginSettings struct {
	Wireguard bool
}

// CPUSettings holds VPP CPU pinning settings.
type CPUSettings struct {
	MainCore *uint8 // nil = auto
	Workers  *uint8 // nil = auto
	// PollSleepMicroseconds is VPP's fixed sleep between main-loop polls
	// (emitted as unix { poll-sleep-usec N }). nil = unset (VPP default: no
	// sleep, busy-poll at 100% CPU). An explicit 0 is emitted (VPP treats 0 as
	// "do not sleep", matching the default, but the operator asked for it).
	PollSleepMicroseconds *uint32
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
//
// An EMPTY name is VALID and is the one value that leaves the LCP TAPs in the
// namespace ze runs in. lcp_set_default_ns NULLs VPP's global default namespace
// for it and closes the fd (third_party/vpp-linux-cp/src/lcp.c), and
// lcp_itf_pair_create then resolves an empty per-pair netns to that NULL and
// creates the TAP without entering any namespace
// (third_party/vpp-linux-cp/src/lcp_interface.c). Every other value is a
// namespace NAME under /var/run/netns/, which ze's BGP listener cannot bind in.
//
// Rejecting it made two guards contradict each other: checkVPPLCPNetns
// (internal/plugins/iface/vpp/doctor.go) tells the operator to leave the leaf
// empty, and this function then refused the commit, so the advice ze prints could
// not be followed. The leaf being OMITTED still means "dataplane": ParseConfig
// above seeds that, matching the YANG default.
// The accepted set is what VPP can carry through, decided once rather than as a
// special case per character. lcp_cli.c parses the leaf with
// unformat(line_input, "netns %s", &ns), and VPP's unformat_string ends a %s at
// a space, tab, newline or carriage return unless the input is brace-delimited,
// which this call site does not use. A name with a space is therefore TRUNCATED
// at the space rather than refused: `netns my ns` reaches VPP as `my`, and
// lcp.c then builds /var/run/netns/my, which does not exist, so LCP pair
// creation fails at apply on a config ze accepted. Every other non-printable
// character reaches the same path as an unusable filename.
func validateNetns(name string) error {
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("vpp lcp: netns must not contain path separators, got %q", name)
	}
	for _, r := range name {
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return fmt.Errorf("vpp lcp: netns must be printable with no spaces, got %q", name)
		}
	}
	if len(name) > 255 {
		return fmt.Errorf("vpp lcp: netns too long (%d > 255 chars)", len(name))
	}
	return nil
}

func validateIfaceName(name string) error {
	return naming.ValidateNodeName("vpp interface", name, 15)
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

// validatePCIAddress checks that addr matches the PCI bus address format.
func validatePCIAddress(addr string) error {
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
	inner, ok := wrapped[componentVPP]
	if !ok {
		return nil, errVppConfigSectionMissingVppRoot
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
		"enabled", "external", "api-socket", "cpu", "memory", "dpdk", "stats", "lcp", "plugins",
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
	if v, ok := raw["plugins"]; ok {
		if err := parsePlugins(v, &cfg.Plugins); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func parsePlugins(data json.RawMessage, plugins *PluginSettings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("vpp plugins: %w", err)
	}
	if err := unknownKeys("plugins", raw, []string{"wireguard"}); err != nil {
		return err
	}
	if v, ok := raw["wireguard"]; ok {
		plugins.Wireguard = strings.Trim(string(v), `"`) == yangTrue
	}
	return nil
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
		return errVppMemoryBuffersMustBe0
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
		if err := validatePCIAddress(iface.PCIAddress); err != nil {
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
	if err := unknownKeys("cpu", raw, []string{"main-core", "workers", "poll-sleep"}); err != nil {
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
	if v, ok := raw["poll-sleep"]; ok {
		usec, err := parsePollSleepMs(strings.Trim(string(v), `"`))
		if err != nil {
			return fmt.Errorf("vpp cpu poll-sleep: %w", err)
		}
		cpu.PollSleepMicroseconds = &usec
	}
	return nil
}

// parsePollSleepMs parses a whole-millisecond poll-sleep value ("10ms"; ms is
// the only accepted unit) into microseconds for the VPP unix { poll-sleep-usec }
// directive. Range 0ms..100ms.
func parsePollSleepMs(s string) (uint32, error) {
	num, ok := strings.CutSuffix(strings.ToLower(strings.TrimSpace(s)), "ms")
	if !ok {
		return 0, fmt.Errorf("must be a whole-millisecond value like 10ms, got %q", s)
	}
	ms, err := strconv.ParseUint(strings.TrimSpace(num), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("not a millisecond number: %q", s)
	}
	if ms > 100 {
		return 0, fmt.Errorf("must be 0ms..100ms, got %q", s)
	}
	return uint32(ms) * 1000, nil
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
