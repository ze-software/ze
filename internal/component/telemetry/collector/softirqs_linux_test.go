// VALIDATES: the softirqs collector parses /proc/softirqs, sums each softirq
// type across CPUs, and emits per-second rates across two collects.
// PREVENTS: a broken per-CPU sum or delta on the system.softirqs metric.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func softirqs(hi, timer, netTx, netRx string) string {
	return "                    CPU0       CPU1\n" +
		"          HI: " + hi + "\n" +
		"       TIMER: " + timer + "\n" +
		"      NET_TX: " + netTx + "\n" +
		"      NET_RX: " + netRx + "\n" +
		"       BLOCK:          0          0\n" +
		"    IRQ_POLL:          0          0\n" +
		"     TASKLET:          0          0\n" +
		"       SCHED:          0          0\n" +
		"     HRTIMER:          0          0\n" +
		"         RCU:          0          0\n"
}

func TestSoftIRQsCollect(t *testing.T) {
	first := softirqs("0 0", "0 0", "0 0", "0 0")
	// TIMER total 30, NET_TX total 300, NET_RX total 1200.
	second := softirqs("0 0", "10 20", "100 200", "500 700")

	fs, dir := procDir(t, map[string]string{"softirqs": first})
	reg := newRecordingRegistry()
	c := newSoftIRQsCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "softirqs", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const name = "netdata_system_softirqs_softirqs_persec_average"
	if got := reg.gauge(t, name, "system.softirqs", "TIMER", "softirqs"); got != 30 {
		t.Errorf("TIMER = %v, want 30", got)
	}
	if got := reg.gauge(t, name, "system.softirqs", "NET_TX", "softirqs"); got != 300 {
		t.Errorf("NET_TX = %v, want 300", got)
	}
	if got := reg.gauge(t, name, "system.softirqs", "NET_RX", "softirqs"); got != 1200 {
		t.Errorf("NET_RX = %v, want 1200", got)
	}
}
