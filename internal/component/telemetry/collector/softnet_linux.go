// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type softNetCollector struct {
	fs       procfs.FS
	interval time.Duration

	gauge metrics.GaugeVec

	prevProcessed    uint64
	prevDropped      uint64
	prevTimeSqueezed uint64
	first            bool
}

func newSoftNetCollector(fs procfs.FS, interval time.Duration) *softNetCollector {
	return &softNetCollector{fs: fs, interval: interval, first: true}
}

func (c *softNetCollector) Name() string { return "softnet" }

func (c *softNetCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_system_softnet_stat_events_persec_average",
		"Softnet Statistics",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *softNetCollector) Collect() error {
	stats, err := c.fs.NetSoftnetStat()
	if err != nil {
		return err
	}

	var processed, dropped, squeezed uint64
	for _, s := range stats {
		processed += uint64(s.Processed)
		dropped += uint64(s.Dropped)
		squeezed += uint64(s.TimeSqueezed)
	}

	if c.first {
		c.prevProcessed = processed
		c.prevDropped = dropped
		c.prevTimeSqueezed = squeezed
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	const chart = "system.softnet_stat"
	const family = "softnet"

	c.gauge.With(chart, "processed", family).Set(float64(safeDelta(processed, c.prevProcessed)) / secs)
	c.gauge.With(chart, "dropped", family).Set(float64(safeDelta(dropped, c.prevDropped)) / secs)
	c.gauge.With(chart, "squeezed", family).Set(float64(safeDelta(squeezed, c.prevTimeSqueezed)) / secs)

	c.prevProcessed = processed
	c.prevDropped = dropped
	c.prevTimeSqueezed = squeezed

	return nil
}
