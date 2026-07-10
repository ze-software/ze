// VALIDATES: the cpuidle collector reads per-C-state residency times and emits
// per-state and derived "active" residency percentages across two collects, from
// a fake /sys/devices/system/cpu tree (root injected via the c.root seam).
// PREVENTS: a broken residency delta, the active = 100 - idle% derivation, or a
// wrong C-state dimension label.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestCPUIdleCollect(t *testing.T) {
	dir := t.TempDir()
	// C-state residency times in microseconds.
	writeProcFile(t, dir, "cpu0/cpuidle/state0/name", "C1\n")
	writeProcFile(t, dir, "cpu0/cpuidle/state0/time", "0\n")
	writeProcFile(t, dir, "cpu0/cpuidle/state1/name", "C2\n")
	writeProcFile(t, dir, "cpu0/cpuidle/state1/time", "0\n")

	c := newCPUIdleCollector(time.Second) // interval = 1e6 microseconds
	c.root = dir
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}

	// C1 +200000us (20%), C2 +100000us (10%) over 1e6us → active = 100 - 30 = 70%.
	writeProcFile(t, dir, "cpu0/cpuidle/state0/time", "200000\n")
	writeProcFile(t, dir, "cpu0/cpuidle/state1/time", "100000\n")
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const name = "netdata_cpuidle_cpu_cstate_residency_time_percentage_average"
	const chart = "cpuidle.cpu0_cpuidle"
	if got := reg.gauge(t, name, chart, "C1", "cpuidle"); got != 20 {
		t.Errorf("C1 = %v, want 20", got)
	}
	if got := reg.gauge(t, name, chart, "C2", "cpuidle"); got != 10 {
		t.Errorf("C2 = %v, want 10", got)
	}
	if got := reg.gauge(t, name, chart, "active", "cpuidle"); got != 70 {
		t.Errorf("active = %v, want 70", got)
	}
}
