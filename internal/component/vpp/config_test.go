package vpp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSettings(t *testing.T) {
	// VALIDATES: AC-1 -- YANG config parsed into settings struct
	// PREVENTS: config parsing regression
	tests := []struct {
		name    string
		input   string
		check   func(*testing.T, *VPPSettings)
		wantErr bool
	}{
		{
			name:  "minimal enabled",
			input: `{"enabled":"true"}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if !s.Enabled {
					t.Error("expected enabled=true")
				}
				if s.APISocket != "/run/vpp/api.sock" {
					t.Errorf("api-socket default: got %q", s.APISocket)
				}
				if s.Memory.Buffers != 128000 {
					t.Errorf("buffers default: got %d", s.Memory.Buffers)
				}
				if s.Memory.HugepageSize != "2M" {
					t.Errorf("hugepage-size default: got %q", s.Memory.HugepageSize)
				}
				if !s.LCP.Enabled {
					t.Error("lcp.enabled default should be true")
				}
				if s.LCP.Netns != "dataplane" {
					t.Errorf("lcp.netns default: got %q", s.LCP.Netns)
				}
			},
		},
		{
			name:  "disabled",
			input: `{"enabled":"false"}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.Enabled {
					t.Error("expected enabled=false")
				}
			},
		},
		{
			name:  "custom api-socket",
			input: `{"enabled":"true","api-socket":"/tmp/vpp.sock"}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.APISocket != "/tmp/vpp.sock" {
					t.Errorf("api-socket: got %q", s.APISocket)
				}
			},
		},
		{
			name:  "cpu settings",
			input: `{"cpu":{"main-core":"0","workers":"3"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.CPU.MainCore == nil || *s.CPU.MainCore != 0 {
					t.Errorf("main-core: got %v", s.CPU.MainCore)
				}
				if s.CPU.Workers == nil || *s.CPU.Workers != 3 {
					t.Errorf("workers: got %v", s.CPU.Workers)
				}
			},
		},
		{
			name:  "cpu omitted leaves nil",
			input: `{}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.CPU.MainCore != nil {
					t.Error("main-core should be nil when omitted")
				}
				if s.CPU.Workers != nil {
					t.Error("workers should be nil when omitted")
				}
			},
		},
		{
			name:  "memory settings",
			input: `{"memory":{"main-heap":"1536M","hugepage-size":"1G","buffers":"256000"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.Memory.MainHeap != "1536M" {
					t.Errorf("main-heap: got %q", s.Memory.MainHeap)
				}
				if s.Memory.HugepageSize != "1G" {
					t.Errorf("hugepage-size: got %q", s.Memory.HugepageSize)
				}
				if s.Memory.Buffers != 256000 {
					t.Errorf("buffers: got %d", s.Memory.Buffers)
				}
			},
		},
		{
			name:    "invalid hugepage size",
			input:   `{"memory":{"hugepage-size":"4K"}}`,
			wantErr: true,
		},
		{
			name:  "dpdk interface",
			input: `{"dpdk":{"interface":{"0000:03:00.0":{"name":"xe0","rx-queues":"4","tx-queues":"4"}}}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if len(s.DPDK.Interfaces) != 1 {
					t.Fatalf("expected 1 interface, got %d", len(s.DPDK.Interfaces))
				}
				iface := s.DPDK.Interfaces[0]
				if iface.PCIAddress != "0000:03:00.0" {
					t.Errorf("pci: got %q", iface.PCIAddress)
				}
				if iface.Name != "xe0" {
					t.Errorf("name: got %q", iface.Name)
				}
				if iface.RxQueues == nil || *iface.RxQueues != 4 {
					t.Errorf("rx-queues: got %v", iface.RxQueues)
				}
				if iface.TxQueues == nil || *iface.TxQueues != 4 {
					t.Errorf("tx-queues: got %v", iface.TxQueues)
				}
			},
		},
		{
			name:  "dpdk interface without queues",
			input: `{"dpdk":{"interface":{"0000:05:00.1":{"name":"e1"}}}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if len(s.DPDK.Interfaces) != 1 {
					t.Fatalf("expected 1 interface, got %d", len(s.DPDK.Interfaces))
				}
				if s.DPDK.Interfaces[0].RxQueues != nil {
					t.Error("rx-queues should be nil when omitted")
				}
			},
		},
		{
			name:  "stats settings",
			input: `{"stats":{"segment-size":"1G","socket-path":"/tmp/stats.sock","poll-interval":"10"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.Stats.SegmentSize != "1G" {
					t.Errorf("segment-size: got %q", s.Stats.SegmentSize)
				}
				if s.Stats.SocketPath != "/tmp/stats.sock" {
					t.Errorf("socket-path: got %q", s.Stats.SocketPath)
				}
				if s.Stats.PollInterval != 10 {
					t.Errorf("poll-interval: got %d, want 10", s.Stats.PollInterval)
				}
			},
		},
		{
			name:  "stats poll-interval default",
			input: `{"stats":{}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.Stats.PollInterval != 30 {
					t.Errorf("default poll-interval: got %d, want 30", s.Stats.PollInterval)
				}
			},
		},
		{
			name:  "lcp disabled",
			input: `{"lcp":{"enabled":"false","sync":"false","auto-subint":"false","netns":"mgmt"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.LCP.Enabled {
					t.Error("lcp.enabled should be false")
				}
				if s.LCP.Sync {
					t.Error("lcp.sync should be false")
				}
				if s.LCP.AutoSubint {
					t.Error("lcp.auto-subint should be false")
				}
				if s.LCP.Netns != "mgmt" {
					t.Errorf("lcp.netns: got %q", s.LCP.Netns)
				}
			},
		},
		{
			name:    "invalid json",
			input:   `{broken`,
			wantErr: true,
		},
		{
			name:    "unknown top-level key rejected",
			input:   `{"enabled":"true","typo-key":"value"}`,
			wantErr: true,
		},
		{
			name:    "unknown cpu key rejected",
			input:   `{"cpu":{"main-core":"0","bogus":"1"}}`,
			wantErr: true,
		},
		{
			name:    "unknown lcp key rejected",
			input:   `{"lcp":{"enabled":"true","unknown":"yes"}}`,
			wantErr: true,
		},
		{
			name:    "poll-interval zero rejected",
			input:   `{"stats":{"poll-interval":"0"}}`,
			wantErr: true,
		},
		{
			name:    "poll-interval 3601 rejected",
			input:   `{"stats":{"poll-interval":"3601"}}`,
			wantErr: true,
		},
		{
			name:  "poll-interval last valid 3600",
			input: `{"stats":{"poll-interval":"3600"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.Stats.PollInterval != 3600 {
					t.Errorf("poll-interval: got %d, want 3600", s.Stats.PollInterval)
				}
			},
		},
		{
			name:  "poll-interval first valid 1",
			input: `{"stats":{"poll-interval":"1"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.Stats.PollInterval != 1 {
					t.Errorf("poll-interval: got %d, want 1", s.Stats.PollInterval)
				}
			},
		},
		{
			// VALIDATES: AC-4 -- external defaults to false when omitted
			// PREVENTS: silent behavior change for existing configs
			name:  "external default false",
			input: `{"enabled":"true"}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.External {
					t.Error("external default should be false")
				}
			},
		},
		{
			// VALIDATES: AC-1 -- external=true plumbs through ParseSettings
			// PREVENTS: external leaf silently dropped
			name:  "external true",
			input: `{"enabled":"true","external":"true"}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if !s.External {
					t.Error("expected External=true")
				}
				if !s.Enabled {
					t.Error("expected Enabled=true alongside External")
				}
			},
		},
		{
			// VALIDATES: unknown keys still rejected even with new leaf present
			// PREVENTS: typo of "external" silently ignored
			name:    "unknown key near external rejected",
			input:   `{"enabled":"true","externall":"true"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseSettings(json.RawMessage(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// VALIDATES: AC-10 -- invalid PCI address rejected
	// PREVENTS: bad config reaching VPP startup

	// validBase returns a fully valid VPPSettings for modification in test cases.
	validBase := func() VPPSettings {
		return VPPSettings{
			Enabled:   true,
			APISocket: "/run/vpp/api.sock",
			Memory:    MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
			Stats:     StatsSettings{SegmentSize: "512M", SocketPath: "/run/vpp/stats.sock"},
			LCP:       LCPSettings{Enabled: true, Netns: "dataplane"},
			DPDK: DPDKSettings{
				Interfaces: []DPDKInterface{
					{PCIAddress: "0000:03:00.0", Name: "xe0"},
				},
			},
		}
	}

	tests := []struct {
		name    string
		modify  func(*VPPSettings)
		wantErr bool
	}{
		{
			name:   "disabled passes",
			modify: func(s *VPPSettings) { s.Enabled = false },
		},
		{
			name:   "valid enabled",
			modify: func(_ *VPPSettings) {},
		},
		{
			name:    "empty api-socket",
			modify:  func(s *VPPSettings) { s.APISocket = "" },
			wantErr: true,
		},
		{
			name:    "relative api-socket",
			modify:  func(s *VPPSettings) { s.APISocket = "relative/path.sock" },
			wantErr: true,
		},
		{
			name:    "api-socket path traversal",
			modify:  func(s *VPPSettings) { s.APISocket = "/run/../etc/passwd" },
			wantErr: true,
		},
		{
			name:    "zero buffers",
			modify:  func(s *VPPSettings) { s.Memory.Buffers = 0 },
			wantErr: true,
		},
		{
			name:    "invalid main-heap format",
			modify:  func(s *VPPSettings) { s.Memory.MainHeap = "not-a-size" },
			wantErr: true,
		},
		{
			name:    "invalid stats segment-size",
			modify:  func(s *VPPSettings) { s.Stats.SegmentSize = "; rm -rf /" },
			wantErr: true,
		},
		{
			name:    "invalid stats socket-path",
			modify:  func(s *VPPSettings) { s.Stats.SocketPath = "not-absolute" },
			wantErr: true,
		},
		{
			name:    "netns with path separator",
			modify:  func(s *VPPSettings) { s.LCP.Netns = "../escape" },
			wantErr: true,
		},
		{
			name:    "empty netns",
			modify:  func(s *VPPSettings) { s.LCP.Netns = "" },
			wantErr: true,
		},
		{
			name:    "invalid pci address",
			modify:  func(s *VPPSettings) { s.DPDK.Interfaces[0].PCIAddress = "not-a-pci-addr" },
			wantErr: true,
		},
		{
			name:    "missing interface name",
			modify:  func(s *VPPSettings) { s.DPDK.Interfaces[0].Name = "" },
			wantErr: true,
		},
		{
			name:   "lcp disabled skips netns validation",
			modify: func(s *VPPSettings) { s.LCP.Enabled = false; s.LCP.Netns = "" },
		},
		{
			name:    "interface name with spaces (injection)",
			modify:  func(s *VPPSettings) { s.DPDK.Interfaces[0].Name = "xe0 ; rm -rf /" },
			wantErr: true,
		},
		{
			name:    "interface name too long",
			modify:  func(s *VPPSettings) { s.DPDK.Interfaces[0].Name = "abcdefghijklmnop" },
			wantErr: true,
		},
		{
			name:   "interface name at max length (15 chars)",
			modify: func(s *VPPSettings) { s.DPDK.Interfaces[0].Name = "abcdefghijklmno" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validBase()
			tt.modify(&s)
			err := s.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePCIAddress(t *testing.T) {
	// VALIDATES: AC-10 -- PCI address validation
	// PREVENTS: path traversal via malformed PCI address in sysfs writes
	tests := []struct {
		addr    string
		wantErr bool
	}{
		{"0000:03:00.0", false},
		{"0000:ff:1f.7", false},
		{"abcd:12:34.5", false},
		{"ABCD:12:34.5", false},
		// Invalid
		{"", true},
		{"03:00.0", true},                // missing domain
		{"0000:03:00", true},             // missing function
		{"0000:03:00.8", true},           // function > 7
		{"0000:03:00.0/../../etc", true}, // path traversal attempt
		{"not-a-pci-addr", true},
		{"0000:GG:00.0", true}, // non-hex
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			err := ValidatePCIAddress(tt.addr)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.addr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.addr, err)
			}
		})
	}
}

// TestParseConfigSection locks the plugin-server wrapping contract that
// the fib-vpp-plugin-load .ci test surfaced. ExtractConfigSubtree in
// internal/component/plugin/server/startup.go wraps every subtree in
// its path structure, so the vpp plugin receives `{"vpp": {...}}` at
// runtime. Before the fix, OnConfigure called ParseSettings(s.Data)
// directly on the wrapped JSON, which rejected the outer "vpp" key as
// unknown, returned an error, left `settings` nil, and caused
// NewVPPManager to deref-panic in OnStarted. This test guards both the
// unwrap and the "inner parse failure wraps cleanly" behaviors.
//
// VALIDATES: live plugin OnConfigure path. spec-vpp-1-lifecycle wiring.
// PREVENTS:  regression to the pre-2026-04-17 state where live ze with
//
//	a vpp { ... } config crashed the plugin on startup.
func TestParseConfigSection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string // substring; empty means expect success
		check   func(*testing.T, *VPPSettings)
	}{
		{
			name:  "wrapped minimal config",
			input: `{"vpp":{"enabled":"true"}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if !s.Enabled {
					t.Error("expected enabled=true after unwrap")
				}
				if s.APISocket != "/run/vpp/api.sock" {
					t.Errorf("api-socket default: got %q", s.APISocket)
				}
			},
		},
		{
			name:  "wrapped full config",
			input: `{"vpp":{"enabled":"true","api-socket":"/tmp/vpp.sock","cpu":{"main-core":"0","workers":"3"}}}`,
			check: func(t *testing.T, s *VPPSettings) {
				t.Helper()
				if s.APISocket != "/tmp/vpp.sock" {
					t.Errorf("api-socket: got %q", s.APISocket)
				}
				if s.CPU.MainCore == nil || *s.CPU.MainCore != 0 {
					t.Errorf("main-core: got %v", s.CPU.MainCore)
				}
			},
		},
		{
			name:    "missing vpp root",
			input:   `{"not-vpp":{}}`,
			wantErr: "missing 'vpp' root",
		},
		{
			name:    "malformed JSON",
			input:   `{not valid json`,
			wantErr: "parse wrapped config",
		},
		{
			name:    "inner parse failure propagates",
			input:   `{"vpp":{"memory":{"hugepage-size":"4K"}}}`,
			wantErr: "parse config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfigSection(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil settings")
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// (dead helpers removed; tests use strings.Contains directly)
