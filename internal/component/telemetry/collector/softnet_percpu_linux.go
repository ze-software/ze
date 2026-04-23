// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"strconv"
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type softNetPerCPUCollector struct {
	fs       procfs.FS
	interval time.Duration

	gauge metrics.GaugeVec

	prev  []procfs.SoftnetStat
	first bool
}

func newSoftNetPerCPUCollector(fs procfs.FS, interval time.Duration) *softNetPerCPUCollector {
	return &softNetPerCPUCollector{fs: fs, interval: interval, first: true}
}

func (c *softNetPerCPUCollector) Name() string { return "softnet_percpu" }

func (c *softNetPerCPUCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_cpu_softnet_stat_events_persec_average",
		"Per-CPU Softnet Statistics",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *softNetPerCPUCollector) Collect() error {
	stats, err := c.fs.NetSoftnetStat()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = stats
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	for i, s := range stats {
		if i >= len(c.prev) {
			break
		}
		p := c.prev[i]
		chart := "cpu.cpu" + strconv.Itoa(i) + "_softnet_stat"
		family := "softnet"

		c.gauge.With(chart, "processed", family).Set(float64(safeDelta(uint64(s.Processed), uint64(p.Processed))) / secs)
		c.gauge.With(chart, "dropped", family).Set(float64(safeDelta(uint64(s.Dropped), uint64(p.Dropped))) / secs)
		c.gauge.With(chart, "squeezed", family).Set(float64(safeDelta(uint64(s.TimeSqueezed), uint64(p.TimeSqueezed))) / secs)
	}

	c.prev = stats
	return nil
}
