// VALIDATES: the cpu collector parses /proc/stat CPU lines and emits per-mode
// utilization percentages (system-wide and per-CPU) from the jiffy deltas across
// two collects.
// PREVENTS: a broken delta or percentage normalization on the system.cpu /
// cpu.cpuN utilization metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestCPUCollect(t *testing.T) {
	// fields: user nice system idle iowait irq softirq steal guest guest_nice
	const first = "cpu  0 0 0 0 0 0 0 0 0 0\n" +
		"cpu0 0 0 0 0 0 0 0 0 0 0\n"
	// aggregate delta: user 25, system 25, idle 50 → 25/25/50 %.
	// cpu0 delta: user 10, system 10, idle 80 → 10/10/80 %.
	const second = "cpu  25 0 25 50 0 0 0 0 0 0\n" +
		"cpu0 10 0 10 80 0 0 0 0 0 0\n"

	fs, dir := procDir(t, map[string]string{"stat": first})
	reg := newRecordingRegistry()
	c := newCPUCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "stat", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const sys = "netdata_system_cpu_percentage_average"
	if got := reg.gauge(t, sys, "system.cpu", "user", "cpu"); got != 25 {
		t.Errorf("system user%% = %v, want 25", got)
	}
	if got := reg.gauge(t, sys, "system.cpu", "system", "cpu"); got != 25 {
		t.Errorf("system system%% = %v, want 25", got)
	}
	if got := reg.gauge(t, sys, "system.cpu", "idle", "cpu"); got != 50 {
		t.Errorf("system idle%% = %v, want 50", got)
	}

	const per = "netdata_cpu_cpu_percentage_average"
	if got := reg.gauge(t, per, "cpu.cpu0", "idle", "utilization"); got != 80 {
		t.Errorf("cpu0 idle%% = %v, want 80", got)
	}
	if got := reg.gauge(t, per, "cpu.cpu0", "user", "utilization"); got != 10 {
		t.Errorf("cpu0 user%% = %v, want 10", got)
	}
}
