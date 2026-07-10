// VALIDATES: the softnet collector sums per-CPU /proc/net/softnet_stat columns
// and emits processed/dropped/squeezed as per-second rates across two collects.
// PREVENTS: a broken hex parse, wrong column selection, or a broken delta on the
// system.softnet_stat metric.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestSoftNetCollect(t *testing.T) {
	// 9 columns minimum; only cols 0/1/2 (processed/dropped/squeezed) are summed.
	const first = "00000064 0000000a 00000002 00000000 00000000 00000000 00000000 00000000 00000000\n" +
		"000000c8 00000014 00000004 00000000 00000000 00000000 00000000 00000000 00000000\n"
	const second = "00000096 0000000f 00000005 00000000 00000000 00000000 00000000 00000000 00000000\n" +
		"00000104 00000019 00000009 00000000 00000000 00000000 00000000 00000000 00000000\n"

	fs, dir := procDir(t, map[string]string{"net/softnet_stat": first})
	reg := newRecordingRegistry()
	c := newSoftNetCollector(fs, time.Second)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "net/softnet_stat", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const name = "netdata_system_softnet_stat_events_persec_average"
	// processed: (0x96+0x104) - (0x64+0xc8) = 410 - 300 = 110
	if got := reg.gauge(t, name, "system.softnet_stat", "processed", "softnet"); got != 110 {
		t.Errorf("processed = %v, want 110", got)
	}
	// dropped: (15+25) - (10+20) = 10
	if got := reg.gauge(t, name, "system.softnet_stat", "dropped", "softnet"); got != 10 {
		t.Errorf("dropped = %v, want 10", got)
	}
	// squeezed: (5+9) - (2+4) = 8
	if got := reg.gauge(t, name, "system.softnet_stat", "squeezed", "softnet"); got != 8 {
		t.Errorf("squeezed = %v, want 8", got)
	}
}
