// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type loadAvgCollector struct {
	fs    procfs.FS
	gauge metrics.GaugeVec
}

func newLoadAvgCollector(fs procfs.FS) *loadAvgCollector {
	return &loadAvgCollector{fs: fs}
}

func (c *loadAvgCollector) Name() string { return "loadavg" }

func (c *loadAvgCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_system_load_load_average",
		"System Load Average",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *loadAvgCollector) Collect() error {
	la, err := c.fs.LoadAvg()
	if err != nil {
		return err
	}
	const chart = "system.load"
	const family = "load"
	c.gauge.With(chart, "load1", family).Set(la.Load1)
	c.gauge.With(chart, "load5", family).Set(la.Load5)
	c.gauge.With(chart, "load15", family).Set(la.Load15)
	return nil
}
