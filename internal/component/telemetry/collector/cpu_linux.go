// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"strconv"
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type cpuCollector struct {
	fs       procfs.FS
	interval time.Duration

	sysGauge metrics.GaugeVec
	cpuGauge metrics.GaugeVec
	prev     procfs.CPUStat
	prevCPU  map[int64]procfs.CPUStat
	first    bool
}

func newCPUCollector(fs procfs.FS, interval time.Duration) *cpuCollector {
	return &cpuCollector{fs: fs, interval: interval, first: true}
}

func (c *cpuCollector) Name() string { return "cpu" }

func (c *cpuCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.sysGauge = reg.GaugeVec(prefix+"_system_cpu_percentage_average", "System CPU utilization", labels)
	c.cpuGauge = reg.GaugeVec(prefix+"_cpu_cpu_percentage_average", "Per-CPU Utilization", labels)
}

func (c *cpuCollector) Collect() error {
	stat, err := c.fs.Stat()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = stat.CPUTotal
		c.prevCPU = stat.CPU
		c.first = false
		return nil
	}

	setCPUPct(c.sysGauge, "system.cpu", "cpu", stat.CPUTotal, c.prev)

	for cpuNum, cur := range stat.CPU {
		prev, ok := c.prevCPU[cpuNum]
		if !ok {
			continue
		}
		chart := "cpu.cpu" + strconv.FormatInt(cpuNum, 10)
		setCPUPct(c.cpuGauge, chart, "utilization", cur, prev)
	}

	c.prev = stat.CPUTotal
	c.prevCPU = stat.CPU
	return nil
}

func clampDelta(cur, prev float64) float64 {
	if cur < prev {
		return 0
	}
	return cur - prev
}

func setCPUPct(g metrics.GaugeVec, chart, family string, cur, prev procfs.CPUStat) {
	du := clampDelta(cur.User, prev.User)
	dn := clampDelta(cur.Nice, prev.Nice)
	ds := clampDelta(cur.System, prev.System)
	di := clampDelta(cur.Idle, prev.Idle)
	dw := clampDelta(cur.Iowait, prev.Iowait)
	dq := clampDelta(cur.IRQ, prev.IRQ)
	df := clampDelta(cur.SoftIRQ, prev.SoftIRQ)
	dt := clampDelta(cur.Steal, prev.Steal)
	dg := clampDelta(cur.Guest, prev.Guest)
	dgn := clampDelta(cur.GuestNice, prev.GuestNice)

	total := du + dn + ds + di + dw + dq + df + dt + dg + dgn
	if total == 0 {
		return
	}

	pct := func(v float64) float64 { return v / total * 100 }

	g.With(chart, "user", family).Set(pct(du))
	g.With(chart, "system", family).Set(pct(ds))
	g.With(chart, "nice", family).Set(pct(dn))
	g.With(chart, "iowait", family).Set(pct(dw))
	g.With(chart, "irq", family).Set(pct(dq))
	g.With(chart, "softirq", family).Set(pct(df))
	g.With(chart, "steal", family).Set(pct(dt))
	g.With(chart, "guest", family).Set(pct(dg))
	g.With(chart, "guest_nice", family).Set(pct(dgn))
	g.With(chart, "idle", family).Set(pct(di))
}
