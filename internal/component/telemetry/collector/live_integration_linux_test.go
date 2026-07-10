// VALIDATES: the real platform collectors, wired against the host's live /proc
// and /sys via registerPlatformCollectors, run a full collect cycle without a
// hard failure and emit a stable sample gauge (loadavg). This is the AC-10 live
// scrape: the integration suite runs under QEMU, so "live /proc" is the
// appliance's own /proc.
// PREVENTS: a fixture-only regression — a collector that passes on golden files
// but panics or errors against real kernel-formatted /proc/sys files.

//go:build integration && linux

package collector

import (
	"log/slog"
	"testing"
	"time"
)

func TestLiveProcScrapeSmoke(t *testing.T) {
	reg := newRecordingRegistry()
	m := NewManager(reg, "netdata", time.Millisecond, slog.Default())
	registerPlatformCollectors(m) // builds the real collectors on live /proc + /sys
	if len(m.collectors) == 0 {
		t.Skip("procfs unavailable on this host")
	}

	for _, c := range m.collectors {
		c.Init(reg, "netdata")
	}

	// Two cycles so the delta-based collectors emit their rate gauges. A subsystem
	// that is simply absent (btrfs/zfs/ipvs/wireless) legitimately no-ops or errors;
	// that is logged, not fatal. A panic here is a real regression.
	for cycle := range 2 {
		for _, c := range m.collectors {
			if err := c.Collect(); err != nil {
				t.Logf("collector %s (cycle %d): %v", c.Name(), cycle, err)
			}
		}
	}

	// loadavg exists on every Linux host: prove the live scrape reached the
	// registry with a sane value.
	got, ok := reg.gaugeOK("netdata_system_load_load_average", "system.load", "load1", "load")
	if !ok {
		t.Fatal("loadavg gauge not emitted from live /proc scrape")
	}
	if got < 0 {
		t.Errorf("live load1 = %v, want >= 0", got)
	}

	// uptime likewise: monotonic, strictly positive on a running host.
	if up, ok := reg.gaugeOK("netdata_system_uptime_seconds_average", "system.uptime", "uptime", "uptime"); !ok || up <= 0 {
		t.Errorf("live uptime gauge missing or non-positive: %v (ok=%v)", up, ok)
	}
}
