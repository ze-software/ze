// VALIDATES: the cpufreq collector reads scaling_cur_freq (kHz→MHz) and emits
// per-core throttle-count rates across two collects, from a fake
// /sys/devices/system/cpu tree (root injected via the c.root seam).
// PREVENTS: a wrong kHz-to-MHz conversion or a broken throttle-count delta.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestCPUFreqCollect(t *testing.T) {
	dir := t.TempDir()
	writeProcFile(t, dir, "cpu0/cpufreq/scaling_cur_freq", "2400000\n") // kHz
	writeProcFile(t, dir, "cpu0/thermal_throttle/core_throttle_count", "5\n")
	writeProcFile(t, dir, "cpu0/thermal_throttle/package_throttle_count", "1\n")

	c := newCPUFreqCollector(time.Second)
	c.root = dir
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	// 2400000 kHz / 1000 = 2400 MHz (set on every collect).
	if got := reg.gauge(t, "netdata_cpufreq_cpufreq_MHz_average", "cpufreq.cpufreq", "cpu0", "cpufreq"); got != 2400 {
		t.Errorf("freq = %v, want 2400", got)
	}

	writeProcFile(t, dir, "cpu0/thermal_throttle/core_throttle_count", "8\n")
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}
	// delta 8-5 = 3 per second.
	if got := reg.gauge(t, "netdata_cpu_core_throttling_events_persec_average", "cpu.core_throttling", "cpu0", "throttling"); got != 3 {
		t.Errorf("core throttle = %v, want 3", got)
	}
}
