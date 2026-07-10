// VALIDATES: the memory collector parses /proc/meminfo and emits RAM/swap/
// committed figures on the correct gauge/label tuples, converted to MiB.
// PREVENTS: a regression of the kB-treated-as-bytes bug that under-reported
// every "_MiB_average" memory gauge by a factor of 1024.

//go:build linux

package collector

import "testing"

func TestMemoryCollect(t *testing.T) {
	fs := procFixture(t, map[string]string{
		"meminfo": "MemTotal:       15666184 kB\n" +
			"MemFree:          440324 kB\n" +
			"Buffers:         1020128 kB\n" +
			"Cached:         12007640 kB\n" +
			"SwapTotal:             0 kB\n" +
			"SwapFree:              0 kB\n" +
			"Committed_AS:     530844 kB\n",
	})
	reg := newRecordingRegistry()
	c := newMemoryCollector(fs)
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// kB / 1024 = MiB. Before the fix these were divided by 1024*1024.
	const ram = "netdata_system_ram_MiB_average"
	if got, want := reg.gauge(t, ram, "system.ram", "free", "ram"), 440324.0/1024; got != want {
		t.Errorf("ram free = %v, want %v", got, want)
	}
	if got, want := reg.gauge(t, ram, "system.ram", "cached", "ram"), 12007640.0/1024; got != want {
		t.Errorf("ram cached = %v, want %v", got, want)
	}
	if got, want := reg.gauge(t, ram, "system.ram", "buffers", "ram"), 1020128.0/1024; got != want {
		t.Errorf("ram buffers = %v, want %v", got, want)
	}
	wantUsed := (15666184.0 - 440324.0 - 12007640.0 - 1020128.0) / 1024
	if got := reg.gauge(t, ram, "system.ram", "used", "ram"); got != wantUsed {
		t.Errorf("ram used = %v, want %v", got, wantUsed)
	}

	if got, want := reg.gauge(t, "netdata_mem_committed_MiB_average", "mem.committed", "Committed_AS", "mem"), 530844.0/1024; got != want {
		t.Errorf("committed = %v, want %v", got, want)
	}
}

func TestMemoryCollectMissingFile(t *testing.T) {
	fs := procFixture(t, nil)
	c := newMemoryCollector(fs)
	c.Init(newRecordingRegistry(), "netdata")
	if err := c.Collect(); err == nil {
		t.Fatal("expected error when /proc/meminfo is absent")
	}
}
