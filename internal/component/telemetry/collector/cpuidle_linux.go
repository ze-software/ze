// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type cpuIdleCollector struct {
	interval time.Duration
	root     string // seam: defaults to /sys/devices/system/cpu, overridable in tests

	gauge metrics.GaugeVec

	prev  map[string]uint64
	first bool
}

func newCPUIdleCollector(interval time.Duration) *cpuIdleCollector {
	return &cpuIdleCollector{interval: interval, root: "/sys/devices/system/cpu", first: true}
}

func (c *cpuIdleCollector) Name() string { return "cpuidle" }

func (c *cpuIdleCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_cpuidle_cpu_cstate_residency_time_percentage_average",
		"CPU C-State Residency",
		chartLabels(),
	)
}

func (c *cpuIdleCollector) Collect() error {
	cur := readCPUIdleStats(c.root)
	if len(cur) == 0 {
		return nil
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	usecs := c.interval.Microseconds()
	if usecs == 0 {
		c.prev = cur
		return nil
	}

	cpus := listCPUs(c.root)
	for _, cpu := range cpus {
		chart := textbuf.StrIntStr("cpuidle.cpu", int64(cpu), "_cpuidle")
		family := "cpuidle"

		prefix := textbuf.StrIntStr("cpu", int64(cpu), ":")
		var totalIdleDelta uint64
		for k, v := range cur {
			if strings.HasPrefix(k, prefix) {
				totalIdleDelta += safeDelta(v, c.prev[k])
			}
		}

		activePct := 100.0 - float64(totalIdleDelta)/float64(usecs)*100
		if activePct < 0 {
			activePct = 0
		}
		c.gauge.With(chart, "active", family).Set(activePct)

		for k, v := range cur {
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			dim := k[strings.Index(k, ":")+1:]
			delta := safeDelta(v, c.prev[k])
			pct := float64(delta) / float64(usecs) * 100
			c.gauge.With(chart, dim, family).Set(pct)
		}
	}

	c.prev = cur
	return nil
}

func readCPUIdleStats(root string) map[string]uint64 {
	cpus := listCPUs(root)
	result := make(map[string]uint64, len(cpus)*8)
	for _, cpu := range cpus {
		base := filepath.Join(root, textbuf.StrIntStr("cpu", int64(cpu), "/cpuidle"))
		states, err := filepath.Glob(filepath.Join(base, "state*"))
		if err != nil || len(states) == 0 {
			continue
		}
		for _, st := range states {
			name := readCPUIdleField(filepath.Join(st, "name"))
			if name == "" {
				name = filepath.Base(st)
			}
			timeUs := readCPUIdleUint64(filepath.Join(st, "time"))
			key := textbuf.StrIntStr("cpu", int64(cpu), ":") + name
			result[key] = timeUs
		}
	}
	return result
}

func readCPUIdleField(path string) string {
	b, err := os.ReadFile(path) //nolint:gosec // sysfs path
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readCPUIdleUint64(path string) uint64 {
	b, err := os.ReadFile(path) //nolint:gosec // sysfs path
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v
}
