// VALIDATES: the stat collector parses /proc/stat, emitting running/blocked
// process counts as gauges and forks/context-switches/interrupts as per-second
// rates across two collects.
// PREVENTS: mis-parsed process state or a broken delta/rate calculation on the
// system.processes/forks/ctxt/intr metrics.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestStatCollect(t *testing.T) {
	const first = "cpu  301854 612 111922 8979004 3552 2 3944 0 0 0\n" +
		"intr 100 0 0\n" +
		"ctxt 5000\n" +
		"btime 1418183276\n" +
		"processes 200\n" +
		"procs_running 2\n" +
		"procs_blocked 1\n"
	const second = "cpu  301854 612 111922 8979004 3552 2 3944 0 0 0\n" +
		"intr 150 0 0\n" +
		"ctxt 5500\n" +
		"btime 1418183276\n" +
		"processes 260\n" +
		"procs_running 3\n" +
		"procs_blocked 0\n"

	fs, dir := procDir(t, map[string]string{"stat": first})
	reg := newRecordingRegistry()
	c := newStatCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	// Process gauges are absolute and set on every collect.
	if got := reg.gauge(t, "netdata_system_processes_processes_average", "system.processes", "running", "processes"); got != 2 {
		t.Errorf("running = %v, want 2", got)
	}

	writeProcFile(t, dir, "stat", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	if got := reg.gauge(t, "netdata_system_processes_processes_average", "system.processes", "running", "processes"); got != 3 {
		t.Errorf("running = %v, want 3", got)
	}
	if got := reg.gauge(t, "netdata_system_forks_processes_persec_average", "system.forks", "started", "processes"); got != 60 {
		t.Errorf("forks = %v, want 60", got)
	}
	if got := reg.gauge(t, "netdata_system_ctxt_context_switches_persec_average", "system.ctxt", "switches", "processes"); got != 500 {
		t.Errorf("ctxt = %v, want 500", got)
	}
	if got := reg.gauge(t, "netdata_system_intr_interrupts_persec_average", "system.intr", "interrupts", "interrupts"); got != 50 {
		t.Errorf("interrupts = %v, want 50", got)
	}
}
