package vpp

import (
	"bytes"
	"strings"
	"testing"
)

func uint8Ptr(v uint8) *uint8 { return &v }

func TestGenerateStartupConf(t *testing.T) {
	// VALIDATES: AC-1 -- startup.conf generated with correct sections
	// PREVENTS: startup.conf generation regression
	s := &VPPSettings{
		Enabled:   true,
		APISocket: "/run/vpp/api.sock",
		CPU: CPUSettings{
			MainCore: uint8Ptr(0),
			Workers:  uint8Ptr(3),
		},
		Memory: MemorySettings{
			MainHeap:     "1536M",
			HugepageSize: "2M",
			Buffers:      128000,
		},
		DPDK: DPDKSettings{
			Interfaces: []DPDKInterface{
				{PCIAddress: "0000:03:00.0", Name: "xe0", RxQueues: uint8Ptr(4), TxQueues: uint8Ptr(4)},
				{PCIAddress: "0000:03:00.1", Name: "xe1"},
			},
		},
		Stats: StatsSettings{
			SegmentSize: "1G",
			SocketPath:  "/run/vpp/stats.sock",
		},
		LCP: LCPSettings{
			Enabled:    true,
			Sync:       true,
			AutoSubint: true,
			Netns:      "dataplane",
		},
	}

	var buf bytes.Buffer
	if err := GenerateStartupConf(&buf, s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	checks := []struct {
		name     string
		contains string
	}{
		{"unix section", "unix {"},
		{"nodaemon", "nodaemon"},
		{"cli-listen", "cli-listen /run/vpp/cli.sock"},
		{"full-coredump", "full-coredump"},
		{"cpu section", "cpu {"},
		{"main-core", "main-core 0"},
		{"corelist-workers", "corelist-workers 1-3"},
		{"buffers section", "buffers {"},
		{"buffers-per-numa", "buffers-per-numa 128000"},
		{"default-data-size", "default-data-size 2048"},
		{"dpdk section", "dpdk {"},
		{"dev xe0", "dev 0000:03:00.0 {"},
		{"name xe0", "name xe0"},
		{"rx-queues", "num-rx-queues 4"},
		{"tx-queues", "num-tx-queues 4"},
		{"dev xe1", "dev 0000:03:00.1 {"},
		{"name xe1", "name xe1"},
		{"plugins section", "plugins {"},
		{"plugin default disable", "plugin default {"},
		{"dpdk enable", "plugin dpdk_plugin.so {"},
		{"linux_cp enable", "plugin linux_cp_plugin.so {"},
		{"linux_nl enable", "plugin linux_nl_plugin.so {"},
		{"linux-cp section", "linux-cp {"},
		{"lcp-sync", "lcp-sync"},
		{"lcp-auto-subint", "lcp-auto-subint"},
		{"default netns", "default netns dataplane"},
		{"linux-nl section", "linux-nl {"},
		{"rx-buffer-size", "rx-buffer-size 67108864"},
		{"heapsize section", "heapsize {"},
		{"main-heap-size", "main-heap-size 1536M"},
		{"statseg section", "statseg {"},
		{"statseg size", "size 1G"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(out, c.contains) {
				t.Errorf("output missing %q:\n%s", c.contains, out)
			}
		})
	}
}

func TestStartupConfCPU(t *testing.T) {
	// VALIDATES: AC-1 -- cpu section varies with config
	// PREVENTS: missing cpu section when omitted
	tests := []struct {
		name     string
		cpu      CPUSettings
		contains []string
		absent   []string
	}{
		{
			name:     "both set",
			cpu:      CPUSettings{MainCore: uint8Ptr(0), Workers: uint8Ptr(3)},
			contains: []string{"main-core 0", "corelist-workers 1-3"},
		},
		{
			name:   "omitted",
			cpu:    CPUSettings{},
			absent: []string{"main-core", "corelist-workers"},
		},
		{
			name:     "main only",
			cpu:      CPUSettings{MainCore: uint8Ptr(2)},
			contains: []string{"main-core 2"},
			absent:   []string{"corelist-workers"},
		},
		{
			name:     "workers only (main-core nil defaults to 0)",
			cpu:      CPUSettings{Workers: uint8Ptr(3)},
			contains: []string{"corelist-workers 1-3"},
			absent:   []string{"main-core"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := defaultTestSettings()
			s.CPU = tt.cpu
			out := generateToString(t, s)

			for _, c := range tt.contains {
				if !strings.Contains(out, c) {
					t.Errorf("missing %q in output", c)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(out, a) {
					t.Errorf("unexpected %q in output", a)
				}
			}
		})
	}
}

func TestStartupConfDPDK(t *testing.T) {
	// VALIDATES: AC-1 -- dpdk section with interfaces
	// PREVENTS: missing dpdk section when no interfaces
	t.Run("no interfaces", func(t *testing.T) {
		s := defaultTestSettings()
		s.DPDK.Interfaces = nil
		out := generateToString(t, s)
		if strings.Contains(out, "dpdk {") {
			t.Error("dpdk section should be absent when no interfaces")
		}
	})

	t.Run("interface without queues", func(t *testing.T) {
		s := defaultTestSettings()
		s.DPDK.Interfaces = []DPDKInterface{
			{PCIAddress: "0000:05:00.0", Name: "e0"},
		}
		out := generateToString(t, s)
		if !strings.Contains(out, "dev 0000:05:00.0 {") {
			t.Error("missing dev entry")
		}
		if !strings.Contains(out, "name e0") {
			t.Error("missing name")
		}
		if strings.Contains(out, "num-rx-queues") {
			t.Error("unexpected rx-queues when not configured")
		}
	})
}

func TestStartupConfLCPEnabled(t *testing.T) {
	// VALIDATES: AC-8 -- LCP enabled includes linux-cp and linux-nl sections
	s := defaultTestSettings()
	s.LCP.Enabled = true
	out := generateToString(t, s)

	for _, want := range []string{"linux-cp {", "linux-nl {", "linux_cp_plugin.so", "linux_nl_plugin.so"} {
		if !strings.Contains(out, want) {
			t.Errorf("LCP enabled but missing %q", want)
		}
	}
}

func TestStartupConfLCPDisabled(t *testing.T) {
	// VALIDATES: AC-9 -- LCP disabled omits linux-cp and linux-nl sections
	s := defaultTestSettings()
	s.LCP.Enabled = false
	out := generateToString(t, s)

	for _, absent := range []string{"linux-cp {", "linux-nl {", "linux_cp_plugin.so", "linux_nl_plugin.so"} {
		if strings.Contains(out, absent) {
			t.Errorf("LCP disabled but found %q", absent)
		}
	}
}

func TestStartupConfBuffers(t *testing.T) {
	// VALIDATES: AC-1 -- buffer count and hugepage size
	s := defaultTestSettings()
	s.Memory.Buffers = 256000
	s.Memory.HugepageSize = "1G"
	out := generateToString(t, s)

	if !strings.Contains(out, "buffers-per-numa 256000") {
		t.Error("wrong buffer count")
	}
	if !strings.Contains(out, "page-size 1G") {
		t.Error("1G hugepage should produce page-size 1G")
	}
}

func TestWorkerCoreList(t *testing.T) {
	tests := []struct {
		main  uint8
		count uint8
		want  string
	}{
		{0, 1, "1"},
		{0, 3, "1-3"},
		{2, 2, "3-4"},
		{0, 7, "1-7"},
	}
	for _, tt := range tests {
		got := workerCoreList(tt.main, tt.count)
		if got != tt.want {
			t.Errorf("workerCoreList(%d, %d) = %q, want %q", tt.main, tt.count, got, tt.want)
		}
	}
}

func defaultTestSettings() *VPPSettings {
	return &VPPSettings{
		Enabled:   true,
		APISocket: "/run/vpp/api.sock",
		Memory: MemorySettings{
			MainHeap:     "1G",
			HugepageSize: "2M",
			Buffers:      128000,
		},
		Stats: StatsSettings{
			SegmentSize: "512M",
			SocketPath:  "/run/vpp/stats.sock",
		},
		LCP: LCPSettings{
			Enabled:    true,
			Sync:       true,
			AutoSubint: true,
			Netns:      "dataplane",
		},
	}
}

func generateToString(t *testing.T, s *VPPSettings) string {
	t.Helper()
	var buf bytes.Buffer
	if err := GenerateStartupConf(&buf, s); err != nil {
		t.Fatalf("GenerateStartupConf: %v", err)
	}
	return buf.String()
}
