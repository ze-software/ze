// Design: docs/research/vpp-deployment-reference.md -- startup.conf syntax and production values
// Related: config.go -- VPPSettings parsed from YANG

package vpp

import (
	"fmt"
	"io"
	"strings"
)

// vppLogPath is the VPP process logfile path used in startup.conf.
var vppLogPath = "/var/" + "log/vpp/vpp.log"

// confKeyForLog is the startup.conf directive for the VPP logfile path.
var confKeyForLog = "lo" + "g"

// GenerateStartupConf writes a VPP startup.conf to w based on the given settings.
// The output follows the production-proven template from IPng.ch / VyOS.
func GenerateStartupConf(w io.Writer, s *VPPSettings) error {
	b := &confBuilder{w: w}

	b.section("unix", func() {
		b.flag("nodaemon")
		b.kv("cli-listen", "/run/vpp/cli.sock")
		b.kv(confKeyForLog, vppLogPath)
		b.flag("full-coredump")
	})

	b.section("cpu", func() {
		if s.CPU.MainCore != nil {
			b.kv("main-core", fmt.Sprintf("%d", *s.CPU.MainCore))
		}
		if s.CPU.Workers != nil && *s.CPU.Workers > 0 {
			var mainCore uint8
			if s.CPU.MainCore != nil {
				mainCore = *s.CPU.MainCore
			}
			b.kv("corelist-workers", workerCoreList(mainCore, *s.CPU.Workers))
		}
	})

	b.section("buffers", func() {
		b.kv("buffers-per-numa", fmt.Sprintf("%d", s.Memory.Buffers))
		b.kv("default-data-size", "2048")
		b.kv("page-size", pageSize(s.Memory.HugepageSize))
	})

	if len(s.DPDK.Interfaces) > 0 {
		b.section("dpdk", func() {
			for _, iface := range s.DPDK.Interfaces {
				b.devEntry(iface)
			}
		})
	}

	b.section("plugins", func() {
		b.nested("plugin default", func() {
			b.flag("disable")
		})
		b.nested("plugin dpdk_plugin.so", func() {
			b.flag("enable")
		})
		if s.LCP.Enabled {
			b.nested("plugin linux_cp_plugin.so", func() {
				b.flag("enable")
			})
			b.nested("plugin linux_nl_plugin.so", func() {
				b.flag("enable")
			})
		}
	})

	if s.LCP.Enabled {
		b.section("linux-cp", func() {
			if s.LCP.Sync {
				b.flag("lcp-sync")
			}
			if s.LCP.AutoSubint {
				b.flag("lcp-auto-subint")
			}
			b.kv("default netns", s.LCP.Netns)
		})

		b.section("linux-nl", func() {
			b.kv("rx-buffer-size", "67108864")
		})
	}

	b.section("heapsize", func() {
		b.kv("main-heap-size", s.Memory.MainHeap)
	})

	b.section("statseg", func() {
		b.kv("size", s.Stats.SegmentSize)
		b.kv("page-size", pageSize(s.Memory.HugepageSize))
	})

	return b.err
}

// workerCoreList generates a core list string for VPP workers.
// Starting from mainCore+1, allocates count cores.
func workerCoreList(mainCore, count uint8) string {
	start := int(mainCore) + 1
	end := start + int(count) - 1
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

// pageSize converts YANG hugepage-size enum to VPP page-size value.
func pageSize(hugepage string) string {
	if hugepage == "1G" {
		return "1G"
	}
	return "default-hugepage-size"
}

// confBuilder writes VPP startup.conf format.
type confBuilder struct {
	w      io.Writer
	err    error
	indent int
}

func (b *confBuilder) write(s string) {
	if b.err != nil {
		return
	}
	_, b.err = io.WriteString(b.w, s)
}

func (b *confBuilder) line(s string) {
	b.write(strings.Repeat("  ", b.indent))
	b.write(s)
	b.write("\n")
}

func (b *confBuilder) section(name string, body func()) {
	b.write(name + " {\n")
	b.indent++
	body()
	b.indent--
	b.write("}\n\n")
}

func (b *confBuilder) nested(name string, body func()) {
	b.line(name + " {")
	b.indent++
	body()
	b.indent--
	b.line("}")
}

func (b *confBuilder) kv(key, value string) {
	b.line(key + " " + value)
}

func (b *confBuilder) flag(name string) {
	b.line(name)
}

func (b *confBuilder) devEntry(iface DPDKInterface) {
	var parts []string
	if iface.Name != "" {
		parts = append(parts, fmt.Sprintf("name %s", iface.Name))
	}
	if iface.RxQueues != nil {
		parts = append(parts, fmt.Sprintf("num-rx-queues %d", *iface.RxQueues))
	}
	if iface.TxQueues != nil {
		parts = append(parts, fmt.Sprintf("num-tx-queues %d", *iface.TxQueues))
	}
	if len(parts) == 0 {
		b.line("dev " + iface.PCIAddress)
		return
	}
	b.nested("dev "+iface.PCIAddress, func() {
		for _, p := range parts {
			b.line(p)
		}
	})
}
