// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type btrfsCollector struct {
	disk     metrics.GaugeVec
	data     metrics.GaugeVec
	metadata metrics.GaugeVec
	system   metrics.GaugeVec
}

func newBtrfsCollector() *btrfsCollector {
	return &btrfsCollector{}
}

func (c *btrfsCollector) Name() string { return "btrfs" }

func (c *btrfsCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.disk = reg.GaugeVec(prefix+"_btrfs_disk_MiB_average", "Btrfs Disk Allocation", labels)
	c.data = reg.GaugeVec(prefix+"_btrfs_data_MiB_average", "Btrfs Data Allocation", labels)
	c.metadata = reg.GaugeVec(prefix+"_btrfs_metadata_MiB_average", "Btrfs Metadata Allocation", labels)
	c.system = reg.GaugeVec(prefix+"_btrfs_system_MiB_average", "Btrfs System Allocation", labels)
}

func (c *btrfsCollector) Collect() error {
	uuids, err := filepath.Glob("/sys/fs/btrfs/*-*-*-*-*")
	if err != nil {
		return err
	}
	if len(uuids) == 0 {
		return nil
	}

	for _, base := range uuids {
		uuid := filepath.Base(base)
		label := readBtrfsLabel(base)
		family := label
		if family == "" {
			family = uuid[:8]
		}

		var diskTotal, diskUsed uint64
		for _, sub := range []string{"data", "metadata", "system"} {
			diskTotal += readSysUint64(filepath.Join(base, "allocation", sub, "disk_total"))
			diskUsed += readSysUint64(filepath.Join(base, "allocation", sub, "disk_used"))
		}
		if diskTotal > 0 {
			c.disk.With("btrfs."+family, "unallocated", family).Set(float64(diskTotal-diskUsed) / mibDivisor)
			c.disk.With("btrfs."+family, "used", family).Set(float64(diskUsed) / mibDivisor)
		}

		for _, typ := range []struct {
			name  string
			gauge metrics.GaugeVec
		}{
			{"data", c.data},
			{"metadata", c.metadata},
			{"system", c.system},
		} {
			dir := filepath.Join(base, "allocation", typ.name)
			total := readSysUint64(filepath.Join(dir, "bytes_used"))
			limit := readSysUint64(filepath.Join(dir, "disk_total"))
			if limit > 0 {
				chart := "btrfs_" + typ.name + "." + family
				typ.gauge.With(chart, "used", family).Set(float64(total) / mibDivisor)
				typ.gauge.With(chart, "free", family).Set(float64(limit-total) / mibDivisor)
			}
		}
	}

	return nil
}

func readBtrfsLabel(base string) string {
	b, err := os.ReadFile(filepath.Join(base, "label")) //nolint:gosec // sysfs path
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
