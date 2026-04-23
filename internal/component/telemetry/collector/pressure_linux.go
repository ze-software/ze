// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type pressureCollector struct {
	fs procfs.FS

	cpuSome    metrics.GaugeVec
	memorySome metrics.GaugeVec
	memoryFull metrics.GaugeVec
	ioSome     metrics.GaugeVec
	ioFull     metrics.GaugeVec
}

func newPressureCollector(fs procfs.FS) *pressureCollector {
	return &pressureCollector{fs: fs}
}

func (c *pressureCollector) Name() string { return "pressure" }

func (c *pressureCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.cpuSome = reg.GaugeVec(prefix+"_system_cpu_some_pressure_percentage_average", "CPU Pressure Some", labels)
	c.memorySome = reg.GaugeVec(prefix+"_system_memory_some_pressure_percentage_average", "Memory Pressure Some", labels)
	c.memoryFull = reg.GaugeVec(prefix+"_system_memory_full_pressure_percentage_average", "Memory Pressure Full", labels)
	c.ioSome = reg.GaugeVec(prefix+"_system_io_some_pressure_percentage_average", "I/O Pressure Some", labels)
	c.ioFull = reg.GaugeVec(prefix+"_system_io_full_pressure_percentage_average", "I/O Pressure Full", labels)
}

func (c *pressureCollector) Collect() error {
	cpuPSI, err := c.fs.PSIStatsForResource("cpu")
	if err == nil && cpuPSI.Some != nil {
		c.cpuSome.With("system.cpu_some_pressure", "some 10", "cpu").Set(cpuPSI.Some.Avg10)
		c.cpuSome.With("system.cpu_some_pressure", "some 60", "cpu").Set(cpuPSI.Some.Avg60)
		c.cpuSome.With("system.cpu_some_pressure", "some 300", "cpu").Set(cpuPSI.Some.Avg300)
	}

	memPSI, err := c.fs.PSIStatsForResource("memory")
	if err == nil {
		if memPSI.Some != nil {
			c.memorySome.With("system.memory_some_pressure", "some 10", "ram").Set(memPSI.Some.Avg10)
			c.memorySome.With("system.memory_some_pressure", "some 60", "ram").Set(memPSI.Some.Avg60)
			c.memorySome.With("system.memory_some_pressure", "some 300", "ram").Set(memPSI.Some.Avg300)
		}
		if memPSI.Full != nil {
			c.memoryFull.With("system.memory_full_pressure", "full 10", "ram").Set(memPSI.Full.Avg10)
			c.memoryFull.With("system.memory_full_pressure", "full 60", "ram").Set(memPSI.Full.Avg60)
			c.memoryFull.With("system.memory_full_pressure", "full 300", "ram").Set(memPSI.Full.Avg300)
		}
	}

	ioPSI, err := c.fs.PSIStatsForResource("io")
	if err == nil {
		if ioPSI.Some != nil {
			c.ioSome.With("system.io_some_pressure", "some 10", "disk").Set(ioPSI.Some.Avg10)
			c.ioSome.With("system.io_some_pressure", "some 60", "disk").Set(ioPSI.Some.Avg60)
			c.ioSome.With("system.io_some_pressure", "some 300", "disk").Set(ioPSI.Some.Avg300)
		}
		if ioPSI.Full != nil {
			c.ioFull.With("system.io_full_pressure", "full 10", "disk").Set(ioPSI.Full.Avg10)
			c.ioFull.With("system.io_full_pressure", "full 60", "disk").Set(ioPSI.Full.Avg60)
			c.ioFull.With("system.io_full_pressure", "full 300", "disk").Set(ioPSI.Full.Avg300)
		}
	}

	return nil
}
