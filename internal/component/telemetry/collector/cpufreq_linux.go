// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type cpuFreqCollector struct {
	interval time.Duration

	freq     metrics.GaugeVec
	throttle metrics.GaugeVec
	prevCore map[int]uint64
	prevPkg  map[int]uint64
	first    bool
}

func newCPUFreqCollector(interval time.Duration) *cpuFreqCollector {
	return &cpuFreqCollector{interval: interval, first: true}
}

func (c *cpuFreqCollector) Name() string { return "cpufreq" }

func (c *cpuFreqCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.freq = reg.GaugeVec(prefix+"_cpufreq_cpufreq_MHz_average", "CPU Frequency", labels)
	c.throttle = reg.GaugeVec(prefix+"_cpu_core_throttling_events_persec_average", "CPU Core Throttling", labels)
}

func (c *cpuFreqCollector) Collect() error {
	cpus := listCPUs()

	curCore := make(map[int]uint64, len(cpus))
	curPkg := make(map[int]uint64, len(cpus))

	for _, cpu := range cpus {
		base := fmt.Sprintf("/sys/devices/system/cpu/cpu%d", cpu)

		// Current frequency in kHz -> MHz
		if khz := readSysInt(filepath.Join(base, "cpufreq", "scaling_cur_freq")); khz > 0 {
			c.freq.With("cpufreq.cpufreq", fmt.Sprintf("cpu%d", cpu), "cpufreq").Set(float64(khz) / 1000)
		}

		curCore[cpu] = readSysUint64(filepath.Join(base, "thermal_throttle", "core_throttle_count"))
		curPkg[cpu] = readSysUint64(filepath.Join(base, "thermal_throttle", "package_throttle_count"))
	}

	if c.first {
		c.prevCore = curCore
		c.prevPkg = curPkg
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	for _, cpu := range cpus {
		dim := fmt.Sprintf("cpu%d", cpu)
		if prev, ok := c.prevCore[cpu]; ok {
			c.throttle.With("cpu.core_throttling", dim, "throttling").Set(float64(safeDelta(curCore[cpu], prev)) / secs)
		}
		if prev, ok := c.prevPkg[cpu]; ok {
			c.throttle.With("cpu.package_throttling", dim, "throttling").Set(float64(safeDelta(curPkg[cpu], prev)) / secs)
		}
	}

	c.prevCore = curCore
	c.prevPkg = curPkg
	return nil
}

func listCPUs() []int {
	entries, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*")
	if err != nil {
		return nil
	}
	cpus := make([]int, 0, len(entries))
	for _, e := range entries {
		name := filepath.Base(e)
		n, err := strconv.Atoi(name[3:])
		if err != nil {
			continue
		}
		cpus = append(cpus, n)
	}
	return cpus
}

func readSysInt(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return v
}

func readSysUint64(path string) uint64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v
}
