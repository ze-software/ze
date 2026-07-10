// VALIDATES: the per-CPU softnet collector emits processed/dropped/squeezed
// rates per CPU (chart keyed by slice index) across two collects.
// PREVENTS: cross-CPU aggregation leaking into a per-CPU chart, or a broken
// per-CPU delta.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestSoftNetPerCPUCollect(t *testing.T) {
	const first = "00000064 0000000a 00000002 00000000 00000000 00000000 00000000 00000000 00000000\n" +
		"000000c8 00000014 00000004 00000000 00000000 00000000 00000000 00000000 00000000\n"
	const second = "00000096 0000000f 00000005 00000000 00000000 00000000 00000000 00000000 00000000\n" +
		"00000104 00000019 00000009 00000000 00000000 00000000 00000000 00000000 00000000\n"

	fs, dir := procDir(t, map[string]string{"net/softnet_stat": first})
	reg := newRecordingRegistry()
	c := newSoftNetPerCPUCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/softnet_stat", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const name = "netdata_cpu_softnet_stat_events_persec_average"
	// cpu0: processed 0x96-0x64 = 50, dropped 15-10 = 5, squeezed 5-2 = 3
	if got := reg.gauge(t, name, "cpu.cpu0_softnet_stat", "processed", "softnet"); got != 50 {
		t.Errorf("cpu0 processed = %v, want 50", got)
	}
	if got := reg.gauge(t, name, "cpu.cpu0_softnet_stat", "squeezed", "softnet"); got != 3 {
		t.Errorf("cpu0 squeezed = %v, want 3", got)
	}
	// cpu1: processed 0x104-0xc8 = 60, dropped 25-20 = 5, squeezed 9-4 = 5
	if got := reg.gauge(t, name, "cpu.cpu1_softnet_stat", "processed", "softnet"); got != 60 {
		t.Errorf("cpu1 processed = %v, want 60", got)
	}
	if got := reg.gauge(t, name, "cpu.cpu1_softnet_stat", "dropped", "softnet"); got != 5 {
		t.Errorf("cpu1 dropped = %v, want 5", got)
	}
}
