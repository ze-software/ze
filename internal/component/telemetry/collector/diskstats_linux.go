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

type diskEntry struct {
	major       uint64
	minor       uint64
	name        string
	readOps     uint64
	readMerged  uint64
	readSect    uint64
	readMs      uint64
	writeOps    uint64
	writeMerged uint64
	writeSect   uint64
	writeMs     uint64
	inFlight    uint64
	ioMs        uint64
	weightedMs  uint64
}

type diskStatsCollector struct {
	interval time.Duration

	io      metrics.GaugeVec
	ops     metrics.GaugeVec
	mops    metrics.GaugeVec
	iotime  metrics.GaugeVec
	busy    metrics.GaugeVec
	backlog metrics.GaugeVec
	sysIO   metrics.GaugeVec
	await   metrics.GaugeVec
	svctm   metrics.GaugeVec
	avgsz   metrics.GaugeVec
	qops    metrics.GaugeVec

	prev  map[string]diskEntry
	first bool
}

func newDiskStatsCollector(interval time.Duration) *diskStatsCollector {
	return &diskStatsCollector{interval: interval, first: true}
}

func (c *diskStatsCollector) Name() string { return "diskstats" }

func (c *diskStatsCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.io = reg.GaugeVec(prefix+"_disk_io_KiB_persec_average", "Disk I/O Bandwidth", labels)
	c.ops = reg.GaugeVec(prefix+"_disk_ops_operations_persec_average", "Disk Operations", labels)
	c.mops = reg.GaugeVec(prefix+"_disk_mops_merged_operations_persec_average", "Disk Merged Operations", labels)
	c.iotime = reg.GaugeVec(prefix+"_disk_iotime_milliseconds_persec_average", "Disk IO Time", labels)
	c.busy = reg.GaugeVec(prefix+"_disk_busy_milliseconds_average", "Disk Busy Time", labels)
	c.backlog = reg.GaugeVec(prefix+"_disk_backlog_milliseconds_average", "Disk Backlog", labels)
	c.sysIO = reg.GaugeVec(prefix+"_system_io_KiB_persec_average", "System Disk I/O", labels)
	c.await = reg.GaugeVec(prefix+"_disk_await_milliseconds_operation_average", "Disk Await", labels)
	c.svctm = reg.GaugeVec(prefix+"_disk_svctm_milliseconds_operation_average", "Disk Service Time", labels)
	c.avgsz = reg.GaugeVec(prefix+"_disk_avgsz_KiB_operation_average", "Disk Average Request Size", labels)
	c.qops = reg.GaugeVec(prefix+"_disk_qops_operations_average", "Disk Queued Operations", labels)
}

func (c *diskStatsCollector) Collect() error {
	entries, err := readDiskStats()
	if err != nil {
		return err
	}

	cur := make(map[string]diskEntry, len(entries))
	for _, e := range entries {
		if !isPhysicalDisk(e) {
			continue
		}
		cur[e.name] = e
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	var totalReadSect, totalWriteSect uint64
	for name, ce := range cur {
		pe, ok := c.prev[name]
		if !ok {
			continue
		}
		chart := "disk." + name
		family := name

		drSect := safeDelta(ce.readSect, pe.readSect)
		dwSect := safeDelta(ce.writeSect, pe.writeSect)
		drOps := safeDelta(ce.readOps, pe.readOps)
		dwOps := safeDelta(ce.writeOps, pe.writeOps)
		drMs := safeDelta(ce.readMs, pe.readMs)
		dwMs := safeDelta(ce.writeMs, pe.writeMs)
		dIoMs := safeDelta(ce.ioMs, pe.ioMs)

		c.io.With(chart, "reads", family).Set(float64(drSect) / 2 / secs)
		c.io.With(chart, "writes", family).Set(float64(dwSect) / 2 / secs)

		c.ops.With(chart, "reads", family).Set(float64(drOps) / secs)
		c.ops.With(chart, "writes", family).Set(float64(dwOps) / secs)

		c.mops.With(chart, "reads", family).Set(float64(safeDelta(ce.readMerged, pe.readMerged)) / secs)
		c.mops.With(chart, "writes", family).Set(float64(safeDelta(ce.writeMerged, pe.writeMerged)) / secs)

		c.iotime.With(chart, "reads", family).Set(float64(drMs) / secs)
		c.iotime.With(chart, "writes", family).Set(float64(dwMs) / secs)

		c.busy.With(chart, "busy", family).Set(float64(dIoMs) / secs)
		c.backlog.With(chart, "backlog", family).Set(float64(safeDelta(ce.weightedMs, pe.weightedMs)) / secs)

		c.qops.With(chart, "operations", family).Set(float64(ce.inFlight))

		totalOps := drOps + dwOps
		if totalOps > 0 {
			c.await.With(chart, "reads", family).Set(safeDiv(float64(drMs), float64(drOps)))
			c.await.With(chart, "writes", family).Set(safeDiv(float64(dwMs), float64(dwOps)))
			c.svctm.With(chart, "svctm", family).Set(float64(dIoMs) / float64(totalOps))
			c.avgsz.With(chart, "reads", family).Set(safeDiv(float64(drSect)/2, float64(drOps)))
			c.avgsz.With(chart, "writes", family).Set(safeDiv(float64(dwSect)/2, float64(dwOps)))
		} else {
			c.await.With(chart, "reads", family).Set(0)
			c.await.With(chart, "writes", family).Set(0)
			c.svctm.With(chart, "svctm", family).Set(0)
			c.avgsz.With(chart, "reads", family).Set(0)
			c.avgsz.With(chart, "writes", family).Set(0)
		}

		totalReadSect += drSect
		totalWriteSect += dwSect
	}

	c.sysIO.With("system.io", "in", "disk").Set(float64(totalReadSect) / 2 / secs)
	c.sysIO.With("system.io", "out", "disk").Set(float64(totalWriteSect) / 2 / secs)

	c.prev = cur
	return nil
}

func safeDiv(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return num / den
}

func readDiskStats() ([]diskEntry, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, fmt.Errorf("open /proc/diskstats: %w", err)
	}
	defer func() { _ = f.Close() }()

	var entries []diskEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		e := diskEntry{name: fields[2]}
		e.major, _ = strconv.ParseUint(fields[0], 10, 64)
		e.minor, _ = strconv.ParseUint(fields[1], 10, 64)
		e.readOps, _ = strconv.ParseUint(fields[3], 10, 64)
		e.readMerged, _ = strconv.ParseUint(fields[4], 10, 64)
		e.readSect, _ = strconv.ParseUint(fields[5], 10, 64)
		e.readMs, _ = strconv.ParseUint(fields[6], 10, 64)
		e.writeOps, _ = strconv.ParseUint(fields[7], 10, 64)
		e.writeMerged, _ = strconv.ParseUint(fields[8], 10, 64)
		e.writeSect, _ = strconv.ParseUint(fields[9], 10, 64)
		e.writeMs, _ = strconv.ParseUint(fields[10], 10, 64)
		e.inFlight, _ = strconv.ParseUint(fields[11], 10, 64)
		e.ioMs, _ = strconv.ParseUint(fields[12], 10, 64)
		e.weightedMs, _ = strconv.ParseUint(fields[13], 10, 64)
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// isPhysicalDisk filters for whole physical disks (sd*, nvme*n*, vd*, xvd*),
// excluding partitions and virtual devices.
func isPhysicalDisk(e diskEntry) bool {
	n := e.name
	if strings.HasPrefix(n, "sd") || strings.HasPrefix(n, "vd") || strings.HasPrefix(n, "xvd") {
		for _, c := range n[2:] {
			if c >= '0' && c <= '9' {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(n, "nvme") {
		idx := strings.LastIndex(n, "p")
		if idx < 0 {
			return true
		}
		suffix := n[idx+1:]
		if suffix == "" {
			return true
		}
		for _, c := range suffix {
			if c < '0' || c > '9' {
				return true
			}
		}
		return false
	}
	return false
}
