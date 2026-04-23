// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type softIRQsCollector struct {
	fs       procfs.FS
	interval time.Duration

	gauge metrics.GaugeVec

	prev  procfs.Softirqs
	first bool
}

func newSoftIRQsCollector(fs procfs.FS, interval time.Duration) *softIRQsCollector {
	return &softIRQsCollector{fs: fs, interval: interval, first: true}
}

func (c *softIRQsCollector) Name() string { return "softirqs" }

func (c *softIRQsCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_system_softirqs_softirqs_persec_average",
		"System Soft Interrupts",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *softIRQsCollector) Collect() error {
	cur, err := c.fs.Softirqs()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	const chart = "system.softirqs"
	const family = "softirqs"

	setSIRQ := func(name string, curSlice, prevSlice []uint64) {
		var curTotal, prevTotal uint64
		for _, v := range curSlice {
			curTotal += v
		}
		for _, v := range prevSlice {
			prevTotal += v
		}
		c.gauge.With(chart, name, family).Set(float64(safeDelta(curTotal, prevTotal)) / secs)
	}

	setSIRQ("HI", cur.Hi, c.prev.Hi)
	setSIRQ("TIMER", cur.Timer, c.prev.Timer)
	setSIRQ("NET_TX", cur.NetTx, c.prev.NetTx)
	setSIRQ("NET_RX", cur.NetRx, c.prev.NetRx)
	setSIRQ("BLOCK", cur.Block, c.prev.Block)
	setSIRQ("IRQ_POLL", cur.IRQPoll, c.prev.IRQPoll)
	setSIRQ("TASKLET", cur.Tasklet, c.prev.Tasklet)
	setSIRQ("SCHED", cur.Sched, c.prev.Sched)
	setSIRQ("HRTIMER", cur.HRTimer, c.prev.HRTimer)
	setSIRQ("RCU", cur.RCU, c.prev.RCU)

	c.prev = cur
	return nil
}
