// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type vmstatCollector struct {
	interval time.Duration

	pgfaults  metrics.GaugeVec
	pgio      metrics.GaugeVec
	swapio    metrics.GaugeVec
	oomKill   metrics.GaugeVec
	numa      metrics.GaugeVec
	balloon   metrics.GaugeVec
	zswapio   metrics.GaugeVec
	ksmCow    metrics.GaugeVec
	thpFaults metrics.GaugeVec
	thpCollap metrics.GaugeVec

	prev  map[string]uint64
	first bool
}

func newVMStatCollector(interval time.Duration) *vmstatCollector {
	return &vmstatCollector{interval: interval, first: true}
}

func (c *vmstatCollector) Name() string { return "vmstat" }

func (c *vmstatCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.pgfaults = reg.GaugeVec(prefix+"_mem_pgfaults_faults_persec_average", "Page Faults", labels)
	c.pgio = reg.GaugeVec(prefix+"_system_pgpgio_KiB_persec_average", "Disk Paging I/O", labels)
	c.swapio = reg.GaugeVec(prefix+"_mem_swapio_KiB_persec_average", "Swap I/O", labels)
	c.oomKill = reg.GaugeVec(prefix+"_mem_oom_kill_kills_persec_average", "OOM Kills", labels)
	c.numa = reg.GaugeVec(prefix+"_mem_numa_events_persec_average", "NUMA Events", labels)
	c.balloon = reg.GaugeVec(prefix+"_mem_balloon_KiB_persec_average", "Memory Balloon", labels)
	c.zswapio = reg.GaugeVec(prefix+"_mem_zswapio_KiB_persec_average", "Zswap I/O", labels)
	c.ksmCow = reg.GaugeVec(prefix+"_mem_ksm_cow_KiB_persec_average", "KSM CoW", labels)
	c.thpFaults = reg.GaugeVec(prefix+"_mem_thp_faults_events_persec_average", "THP Faults", labels)
	c.thpCollap = reg.GaugeVec(prefix+"_mem_thp_collapse_events_persec_average", "THP Collapse", labels)
}

func (c *vmstatCollector) Collect() error {
	cur, err := readVMStat()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	// Page faults
	c.pgfaults.With("mem.pgfaults", "minor", "mem").Set(float64(safeDelta(cur["pgfault"], c.prev["pgfault"])) / secs)
	c.pgfaults.With("mem.pgfaults", "major", "mem").Set(float64(safeDelta(cur["pgmajfault"], c.prev["pgmajfault"])) / secs)

	c.pgio.With("system.pgpgio", "in", "disk").Set(float64(safeDelta(cur["pgpgin"], c.prev["pgpgin"])) / secs)
	c.pgio.With("system.pgpgio", "out", "disk").Set(float64(safeDelta(cur["pgpgout"], c.prev["pgpgout"])) / secs)

	c.swapio.With("mem.swapio", "in", "swap").Set(float64(safeDelta(cur["pswpin"], c.prev["pswpin"])) / secs)
	c.swapio.With("mem.swapio", "out", "swap").Set(float64(safeDelta(cur["pswpout"], c.prev["pswpout"])) / secs)

	c.oomKill.With("mem.oom_kill", "kills", "OOM kills").Set(float64(safeDelta(cur["oom_kill"], c.prev["oom_kill"])) / secs)

	c.numa.With("mem.numa", "local", "numa").Set(float64(safeDelta(cur["numa_local"], c.prev["numa_local"])) / secs)
	c.numa.With("mem.numa", "foreign", "numa").Set(float64(safeDelta(cur["numa_foreign"], c.prev["numa_foreign"])) / secs)
	c.numa.With("mem.numa", "interleave", "numa").Set(float64(safeDelta(cur["numa_interleave"], c.prev["numa_interleave"])) / secs)
	c.numa.With("mem.numa", "other", "numa").Set(float64(safeDelta(cur["numa_other"], c.prev["numa_other"])) / secs)
	c.numa.With("mem.numa", "pte_updates", "numa").Set(float64(safeDelta(cur["numa_pte_updates"], c.prev["numa_pte_updates"])) / secs)
	c.numa.With("mem.numa", "hint_faults", "numa").Set(float64(safeDelta(cur["numa_hint_faults"], c.prev["numa_hint_faults"])) / secs)
	c.numa.With("mem.numa", "hint_faults_local", "numa").Set(float64(safeDelta(cur["numa_hint_faults_local"], c.prev["numa_hint_faults_local"])) / secs)
	c.numa.With("mem.numa", "pages_migrated", "numa").Set(float64(safeDelta(cur["numa_pages_migrated"], c.prev["numa_pages_migrated"])) / secs)

	c.balloon.With("mem.balloon", "inflate", "balloon").Set(float64(safeDelta(cur["balloon_inflate"], c.prev["balloon_inflate"])) / secs)
	c.balloon.With("mem.balloon", "deflate", "balloon").Set(float64(safeDelta(cur["balloon_deflate"], c.prev["balloon_deflate"])) / secs)
	c.balloon.With("mem.balloon", "migrate", "balloon").Set(float64(safeDelta(cur["balloon_migrate"], c.prev["balloon_migrate"])) / secs)

	c.zswapio.With("mem.zswapio", "in", "zswap").Set(float64(safeDelta(cur["zswpin"], c.prev["zswpin"])) / secs)
	c.zswapio.With("mem.zswapio", "out", "zswap").Set(float64(safeDelta(cur["zswpout"], c.prev["zswpout"])) / secs)

	c.ksmCow.With("mem.ksm_cow", "swapin", "ksm").Set(float64(safeDelta(cur["ksm_swpin_copy"], c.prev["ksm_swpin_copy"])) / secs)
	c.ksmCow.With("mem.ksm_cow", "write", "ksm").Set(float64(safeDelta(cur["cow_ksm"], c.prev["cow_ksm"])) / secs)

	c.thpFaults.With("mem.thp_faults", "alloc", "hugepages").Set(float64(safeDelta(cur["thp_fault_alloc"], c.prev["thp_fault_alloc"])) / secs)
	c.thpFaults.With("mem.thp_faults", "fallback", "hugepages").Set(float64(safeDelta(cur["thp_fault_fallback"], c.prev["thp_fault_fallback"])) / secs)
	c.thpFaults.With("mem.thp_faults", "fallback_charge", "hugepages").Set(float64(safeDelta(cur["thp_fault_fallback_charge"], c.prev["thp_fault_fallback_charge"])) / secs)

	c.thpCollap.With("mem.thp_collapse", "alloc", "hugepages").Set(float64(safeDelta(cur["thp_collapse_alloc"], c.prev["thp_collapse_alloc"])) / secs)
	c.thpCollap.With("mem.thp_collapse", "failed", "hugepages").Set(float64(safeDelta(cur["thp_collapse_alloc_failed"], c.prev["thp_collapse_alloc_failed"])) / secs)

	c.prev = cur
	return nil
}

func readVMStat() (map[string]uint64, error) {
	f, err := os.Open("/proc/vmstat")
	if err != nil {
		return nil, fmt.Errorf("open /proc/vmstat: %w", err)
	}
	defer func() { _ = f.Close() }()

	m := make(map[string]uint64, 64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		m[fields[0]] = v
	}
	return m, scanner.Err()
}
