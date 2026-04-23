// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type memoryCollector struct {
	fs procfs.FS

	ram       metrics.GaugeVec
	swap      metrics.GaugeVec
	available metrics.GaugeVec
	committed metrics.GaugeVec
	kernel    metrics.GaugeVec
	slab      metrics.GaugeVec
	hugepages metrics.GaugeVec
	writeback metrics.GaugeVec
}

func newMemoryCollector(fs procfs.FS) *memoryCollector {
	return &memoryCollector{fs: fs}
}

func (c *memoryCollector) Name() string { return "memory" }

func (c *memoryCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.ram = reg.GaugeVec(prefix+"_system_ram_MiB_average", "System RAM", labels)
	c.swap = reg.GaugeVec(prefix+"_system_swap_MiB_average", "System Swap", labels)
	c.available = reg.GaugeVec(prefix+"_mem_available_MiB_average", "Available RAM", labels)
	c.committed = reg.GaugeVec(prefix+"_mem_committed_MiB_average", "Committed Memory", labels)
	c.kernel = reg.GaugeVec(prefix+"_mem_kernel_MiB_average", "Kernel Memory", labels)
	c.slab = reg.GaugeVec(prefix+"_mem_slab_MiB_average", "Slab Memory", labels)
	c.hugepages = reg.GaugeVec(prefix+"_mem_thp_MiB_average", "Transparent HugePages", labels)
	c.writeback = reg.GaugeVec(prefix+"_mem_writeback_MiB_average", "Writeback Memory", labels)
}

const mibDiv = 1024 * 1024

func toMiB(bytes *uint64) float64 {
	if bytes == nil {
		return 0
	}
	return float64(*bytes) / mibDiv
}

func (c *memoryCollector) Collect() error {
	m, err := c.fs.Meminfo()
	if err != nil {
		return err
	}

	// system.ram: free, used, cached, buffers
	total := toMiB(m.MemTotal)
	free := toMiB(m.MemFree)
	cached := toMiB(m.Cached)
	buffers := toMiB(m.Buffers)
	used := total - free - cached - buffers

	c.ram.With("system.ram", "free", "ram").Set(free)
	c.ram.With("system.ram", "used", "ram").Set(used)
	c.ram.With("system.ram", "cached", "ram").Set(cached)
	c.ram.With("system.ram", "buffers", "ram").Set(buffers)

	// system.swap: free, used
	swapTotal := toMiB(m.SwapTotal)
	swapFree := toMiB(m.SwapFree)
	c.swap.With("system.swap", "free", "swap").Set(swapFree)
	c.swap.With("system.swap", "used", "swap").Set(swapTotal - swapFree)

	// mem.available
	c.available.With("mem.available", "avail", "mem").Set(toMiB(m.MemAvailable))

	// mem.committed: Committed_AS
	c.committed.With("mem.committed", "Committed_AS", "mem").Set(toMiB(m.CommittedAS))

	// mem.kernel: Slab, VmallocUsed, PageTables, KernelStack
	c.kernel.With("mem.kernel", "Slab", "mem").Set(toMiB(m.Slab))
	c.kernel.With("mem.kernel", "VmallocUsed", "mem").Set(toMiB(m.VmallocUsed))
	c.kernel.With("mem.kernel", "PageTables", "mem").Set(toMiB(m.PageTables))
	c.kernel.With("mem.kernel", "KernelStack", "mem").Set(toMiB(m.KernelStack))

	// mem.slab: reclaimable, unreclaimable
	c.slab.With("mem.slab", "reclaimable", "mem").Set(toMiB(m.SReclaimable))
	c.slab.With("mem.slab", "unreclaimable", "mem").Set(toMiB(m.SUnreclaim))

	// mem.transparent_hugepages
	c.hugepages.With("mem.thp", "anonymous", "mem").Set(toMiB(m.AnonHugePages))
	c.hugepages.With("mem.thp", "shmem", "mem").Set(toMiB(m.ShmemHugePages))

	// mem.writeback: Dirty, Writeback, NFS_Unstable, Bounce
	c.writeback.With("mem.writeback", "Dirty", "mem").Set(toMiB(m.Dirty))
	c.writeback.With("mem.writeback", "Writeback", "mem").Set(toMiB(m.Writeback))
	c.writeback.With("mem.writeback", "NfsUnstable", "mem").Set(toMiB(m.NFSUnstable))
	c.writeback.With("mem.writeback", "Bounce", "mem").Set(toMiB(m.Bounce))

	return nil
}
