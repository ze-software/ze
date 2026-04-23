// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type mdstatCollector struct {
	fs procfs.FS

	health metrics.GaugeVec
	disks  metrics.GaugeVec
	sync   metrics.GaugeVec
}

func newMDStatCollector(fs procfs.FS) *mdstatCollector {
	return &mdstatCollector{fs: fs}
}

func (c *mdstatCollector) Name() string { return "mdstat" }

func (c *mdstatCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.health = reg.GaugeVec(prefix+"_md_health_disks_average", "MD Array Health", labels)
	c.disks = reg.GaugeVec(prefix+"_md_disks_disks_average", "MD Array Disks", labels)
	c.sync = reg.GaugeVec(prefix+"_md_mismatch_cnt_unsynchronized_blocks_average", "MD Mismatch Count", labels)
}

func (c *mdstatCollector) Collect() error {
	stats, err := c.fs.MDStat()
	if err != nil {
		return err
	}

	for _, md := range stats {
		chart := "md." + md.Name
		family := md.Name

		c.disks.With(chart, "inuse", family).Set(float64(md.DisksActive))
		c.disks.With(chart, "down", family).Set(float64(md.DisksDown))

		c.health.With(chart, "total", family).Set(float64(md.DisksTotal))
		c.health.With(chart, "failed", family).Set(float64(md.DisksFailed))
		c.health.With(chart, "spare", family).Set(float64(md.DisksSpare))

		if md.BlocksToBeSynced > 0 {
			c.sync.With(chart, "unsynchronized", family).Set(float64(md.BlocksToBeSynced - md.BlocksSynced))
		} else {
			c.sync.With(chart, "unsynchronized", family).Set(0)
		}
	}

	return nil
}
