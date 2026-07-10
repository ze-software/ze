// VALIDATES: the uptime collector parses the first field of /proc/uptime and
// emits it on the system.uptime gauge (path injected via the c.path seam).
// PREVENTS: a mis-parsed uptime (e.g. reading idle instead of uptime) reaching
// the metrics surface.

//go:build linux

package collector

import "testing"

func TestUptimeCollect(t *testing.T) {
	_, path := tmpFile(t, "uptime", "12345.67 98765.43\n")
	c := newUptimeCollector()
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if got := reg.gauge(t, "netdata_system_uptime_seconds_average", "system.uptime", "uptime", "uptime"); got != 12345.67 {
		t.Errorf("uptime = %v, want 12345.67", got)
	}
}

func TestUptimeCollectMissing(t *testing.T) {
	c := newUptimeCollector()
	c.path = "/does/not/exist/uptime"
	c.Init(newRecordingRegistry(), "netdata")
	if err := c.Collect(); err == nil {
		t.Fatal("expected error when /proc/uptime is absent")
	}
}
