// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type memoryCollector struct {
	fs procfs.FS

	ram        metrics.GaugeVec
	swap       metrics.GaugeVec
	available  metrics.GaugeVec
	committed  metrics.GaugeVec
	kernel     metrics.GaugeVec
	slab       metrics.GaugeVec
	thp        metrics.GaugeVec
	writeback  metrics.GaugeVec
	hugepages  metrics.GaugeVec
	reclaiming metrics.GaugeVec
	swapCached metrics.GaugeVec
	cma        metrics.GaugeVec
	directmaps metrics.GaugeVec
	hwcorrupt  metrics.GaugeVec
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
	c.thp = reg.GaugeVec(prefix+"_mem_thp_MiB_average", "Transparent HugePages", labels)
	c.writeback = reg.GaugeVec(prefix+"_mem_writeback_MiB_average", "Writeback Memory", labels)
	c.hugepages = reg.GaugeVec(prefix+"_mem_hugepages_MiB_average", "HugePages", labels)
	c.reclaiming = reg.GaugeVec(prefix+"_mem_reclaiming_MiB_average", "Reclaiming Memory", labels)
	c.swapCached = reg.GaugeVec(prefix+"_mem_swap_cached_MiB_average", "Swap Cached", labels)
	c.cma = reg.GaugeVec(prefix+"_mem_cma_MiB_average", "CMA Memory", labels)
	c.directmaps = reg.GaugeVec(prefix+"_mem_directmaps_MiB_average", "Direct Maps", labels)
	c.hwcorrupt = reg.GaugeVec(prefix+"_mem_hwcorrupt_MiB_average", "Hardware Corrupted", labels)
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

	total := toMiB(m.MemTotal)
	free := toMiB(m.MemFree)
	cached := toMiB(m.Cached)
	buffers := toMiB(m.Buffers)
	used := total - free - cached - buffers

	c.ram.With("system.ram", "free", "ram").Set(free)
	c.ram.With("system.ram", "used", "ram").Set(used)
	c.ram.With("system.ram", "cached", "ram").Set(cached)
	c.ram.With("system.ram", "buffers", "ram").Set(buffers)

	swapTotal := toMiB(m.SwapTotal)
	swapFree := toMiB(m.SwapFree)
	c.swap.With("system.swap", "free", "swap").Set(swapFree)
	c.swap.With("system.swap", "used", "swap").Set(swapTotal - swapFree)

	c.available.With("mem.available", "avail", "mem").Set(toMiB(m.MemAvailable))
	c.committed.With("mem.committed", "Committed_AS", "mem").Set(toMiB(m.CommittedAS))

	c.kernel.With("mem.kernel", "Slab", "kernel").Set(toMiB(m.Slab))
	c.kernel.With("mem.kernel", "VmallocUsed", "kernel").Set(toMiB(m.VmallocUsed))
	c.kernel.With("mem.kernel", "PageTables", "kernel").Set(toMiB(m.PageTables))
	c.kernel.With("mem.kernel", "KernelStack", "kernel").Set(toMiB(m.KernelStack))
	c.kernel.With("mem.kernel", "Percpu", "kernel").Set(toMiB(m.Percpu))
	c.kernel.With("mem.kernel", "KReclaimable", "kernel").Set(toMiB(m.SReclaimable))

	c.slab.With("mem.slab", "reclaimable", "slab").Set(toMiB(m.SReclaimable))
	c.slab.With("mem.slab", "unreclaimable", "slab").Set(toMiB(m.SUnreclaim))

	c.thp.With("mem.thp", "anonymous", "hugepages").Set(toMiB(m.AnonHugePages))
	c.thp.With("mem.thp", "shmem", "hugepages").Set(toMiB(m.ShmemHugePages))

	c.writeback.With("mem.writeback", "Dirty", "writeback").Set(toMiB(m.Dirty))
	c.writeback.With("mem.writeback", "Writeback", "writeback").Set(toMiB(m.Writeback))
	c.writeback.With("mem.writeback", "NfsUnstable", "writeback").Set(toMiB(m.NFSUnstable))
	c.writeback.With("mem.writeback", "Bounce", "writeback").Set(toMiB(m.Bounce))
	c.writeback.With("mem.writeback", "FuseWriteback", "writeback").Set(toMiB(m.WritebackTmp))

	// Static hugepages
	if m.HugePagesTotal != nil && m.HugePagesFree != nil && m.HugePagesRsvd != nil && m.HugePagesSurp != nil && *m.HugePagesTotal > 0 {
		pageSize := toMiB(m.Hugepagesize)
		c.hugepages.With("mem.hugepages", "free", "hugepages").Set(float64(*m.HugePagesFree) * pageSize)
		c.hugepages.With("mem.hugepages", "used", "hugepages").Set(float64(*m.HugePagesTotal-*m.HugePagesFree-*m.HugePagesRsvd) * pageSize)
		c.hugepages.With("mem.hugepages", "reserved", "hugepages").Set(float64(*m.HugePagesRsvd) * pageSize)
		c.hugepages.With("mem.hugepages", "surplus", "hugepages").Set(float64(*m.HugePagesSurp) * pageSize)
	}

	c.reclaiming.With("mem.reclaiming", "active", "reclaiming").Set(toMiB(m.Active))
	c.reclaiming.With("mem.reclaiming", "inactive", "reclaiming").Set(toMiB(m.Inactive))
	c.reclaiming.With("mem.reclaiming", "active_anon", "reclaiming").Set(toMiB(m.ActiveAnon))
	c.reclaiming.With("mem.reclaiming", "inactive_anon", "reclaiming").Set(toMiB(m.InactiveAnon))
	c.reclaiming.With("mem.reclaiming", "active_file", "reclaiming").Set(toMiB(m.ActiveFile))
	c.reclaiming.With("mem.reclaiming", "inactive_file", "reclaiming").Set(toMiB(m.InactiveFile))
	c.reclaiming.With("mem.reclaiming", "unevictable", "reclaiming").Set(toMiB(m.Unevictable))
	c.reclaiming.With("mem.reclaiming", "mlocked", "reclaiming").Set(toMiB(m.Mlocked))

	c.swapCached.With("mem.swap_cached", "cached", "swap").Set(toMiB(m.SwapCached))

	c.cma.With("mem.cma", "used", "cma").Set(toMiB(m.CmaTotal) - toMiB(m.CmaFree))
	c.cma.With("mem.cma", "free", "cma").Set(toMiB(m.CmaFree))

	c.directmaps.With("mem.directmaps", "4k", "overview").Set(toMiB(m.DirectMap4k))
	c.directmaps.With("mem.directmaps", "2m", "overview").Set(toMiB(m.DirectMap2M))
	c.directmaps.With("mem.directmaps", "1g", "overview").Set(toMiB(m.DirectMap1G))

	c.hwcorrupt.With("mem.hwcorrupt", "HardwareCorrupted", "ecc").Set(toMiB(m.HardwareCorrupted))

	return nil
}
