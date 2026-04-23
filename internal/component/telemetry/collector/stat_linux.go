// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type statCollector struct {
	fs       procfs.FS
	interval time.Duration

	processes  metrics.GaugeVec
	forks      metrics.GaugeVec
	ctxt       metrics.GaugeVec
	interrupts metrics.GaugeVec

	prevCtxt    uint64
	prevForks   uint64
	prevHardIRQ uint64
	first       bool
}

func newStatCollector(fs procfs.FS, interval time.Duration) *statCollector {
	return &statCollector{fs: fs, interval: interval, first: true}
}

func (c *statCollector) Name() string { return "stat" }

func (c *statCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.processes = reg.GaugeVec(prefix+"_system_processes_processes_average", "System Processes", labels)
	c.forks = reg.GaugeVec(prefix+"_system_forks_processes_persec_average", "Process Forks", labels)
	c.ctxt = reg.GaugeVec(prefix+"_system_ctxt_context_switches_persec_average", "Context Switches", labels)
	c.interrupts = reg.GaugeVec(prefix+"_system_intr_interrupts_persec_average", "CPU Interrupts", labels)
}

func (c *statCollector) Collect() error {
	stat, err := c.fs.Stat()
	if err != nil {
		return err
	}

	c.processes.With("system.processes", "running", "processes").Set(float64(stat.ProcessesRunning))
	c.processes.With("system.processes", "blocked", "processes").Set(float64(stat.ProcessesBlocked))

	if c.first {
		c.prevCtxt = stat.ContextSwitches
		c.prevForks = stat.ProcessCreated
		c.prevHardIRQ = stat.IRQTotal
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	c.forks.With("system.forks", "started", "processes").Set(float64(safeDelta(stat.ProcessCreated, c.prevForks)) / secs)
	c.ctxt.With("system.ctxt", "switches", "processes").Set(float64(safeDelta(stat.ContextSwitches, c.prevCtxt)) / secs)
	c.interrupts.With("system.intr", "interrupts", "interrupts").Set(float64(safeDelta(stat.IRQTotal, c.prevHardIRQ)) / secs)

	c.prevCtxt = stat.ContextSwitches
	c.prevForks = stat.ProcessCreated
	c.prevHardIRQ = stat.IRQTotal

	return nil
}
