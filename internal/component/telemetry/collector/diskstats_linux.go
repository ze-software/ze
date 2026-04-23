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
	major      uint64
	minor      uint64
	name       string
	readOps    uint64
	readSect   uint64
	writeOps   uint64
	writeSect  uint64
	ioMs       uint64
	weightedMs uint64
}

type diskStatsCollector struct {
	interval time.Duration

	io      metrics.GaugeVec
	ops     metrics.GaugeVec
	iotime  metrics.GaugeVec
	busy    metrics.GaugeVec
	backlog metrics.GaugeVec
	sysIO   metrics.GaugeVec

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
	c.iotime = reg.GaugeVec(prefix+"_disk_iotime_milliseconds_persec_average", "Disk IO Time", labels)
	c.busy = reg.GaugeVec(prefix+"_disk_busy_milliseconds_average", "Disk Busy Time", labels)
	c.backlog = reg.GaugeVec(prefix+"_disk_backlog_milliseconds_average", "Disk Backlog", labels)
	c.sysIO = reg.GaugeVec(prefix+"_system_io_KiB_persec_average", "System Disk I/O", labels)
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

		c.io.With(chart, "reads", family).Set(float64(drSect) / 2 / secs)
		c.io.With(chart, "writes", family).Set(float64(dwSect) / 2 / secs)

		c.ops.With(chart, "reads", family).Set(float64(safeDelta(ce.readOps, pe.readOps)) / secs)
		c.ops.With(chart, "writes", family).Set(float64(safeDelta(ce.writeOps, pe.writeOps)) / secs)

		c.iotime.With(chart, "reads", family).Set(float64(safeDelta(ce.ioMs, pe.ioMs)) / secs)
		c.busy.With(chart, "busy", family).Set(float64(safeDelta(ce.ioMs, pe.ioMs)) / secs)
		c.backlog.With(chart, "backlog", family).Set(float64(safeDelta(ce.weightedMs, pe.weightedMs)) / secs)

		totalReadSect += drSect
		totalWriteSect += dwSect
	}

	c.sysIO.With("system.io", "in", "disk").Set(float64(totalReadSect) / 2 / secs)
	c.sysIO.With("system.io", "out", "disk").Set(float64(totalWriteSect) / 2 / secs)

	c.prev = cur
	return nil
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
		e.readSect, _ = strconv.ParseUint(fields[5], 10, 64)
		e.writeOps, _ = strconv.ParseUint(fields[7], 10, 64)
		e.writeSect, _ = strconv.ParseUint(fields[9], 10, 64)
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
		// nvme0n1 is a disk, nvme0n1p1 is a partition.
		// Partition names end with pN where N is one or more digits.
		idx := strings.LastIndex(n, "p")
		if idx < 0 {
			return true
		}
		suffix := n[idx+1:]
		if len(suffix) == 0 {
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
