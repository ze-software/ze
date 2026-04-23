// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type entropyCollector struct {
	fs    procfs.FS
	gauge metrics.GaugeVec
}

func newEntropyCollector(fs procfs.FS) *entropyCollector {
	return &entropyCollector{fs: fs}
}

func (c *entropyCollector) Name() string { return "entropy" }

func (c *entropyCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_system_entropy_entropy_average",
		"Available Entropy",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *entropyCollector) Collect() error {
	r, err := c.fs.KernelRandom()
	if err != nil {
		return err
	}
	if r.EntropyAvaliable != nil {
		c.gauge.With("system.entropy", "entropy", "entropy").Set(float64(*r.EntropyAvaliable))
	}
	return nil
}
