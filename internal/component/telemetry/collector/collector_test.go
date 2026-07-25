// VALIDATES: Manager schedules collectors correctly -- initial forced collect,
// per-tick collection, per-collector enable/disable, and per-collector interval
// gating against the global tick.
// PREVENTS: regressions in collection scheduling; flaky load-dependent timing
// (cycles are driven by a fake clock, not wall-clock sleeps).
package collector

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/test/sim"
)

type fakeCollector struct {
	name     string
	initCnt  int          // written only during Start (one goroutine), read after Stop
	collectN atomic.Int64 // written by the loop goroutine each tick; polled by tick()
}

func (f *fakeCollector) Name() string                      { return f.name }
func (f *fakeCollector) Init(_ metrics.Registry, _ string) { f.initCnt++ }
func (f *fakeCollector) Collect() error                    { f.collectN.Add(1); return nil }

// newFakeClockManager builds a Manager wired to a fake clock so tests can drive
// collection cycles deterministically. Returns the manager and the fake clock.
func newFakeClockManager(reg metrics.Registry, prefix string, interval time.Duration) (*Manager, *sim.FakeClock) {
	m := NewManager(reg, prefix, interval, nil)
	fc := sim.NewFakeClock(time.Unix(0, 0))
	m.setClock(fc)
	return m, fc
}

// tick advances the fake clock by the loop's fixed 1s tick period, fires the
// registered ticker, and waits until collector c records one more Collect():
// the observable effect of the cycle the loop goroutine just processed. The
// cycle is observed through the test's own collector, so the Manager carries no
// test-only signal. The 1ms poll is a backoff, not a fixed "hope it ran" delay.
func tick(t *testing.T, fc *sim.FakeClock, c *fakeCollector) {
	t.Helper()
	before := c.collectN.Load()
	fc.Add(time.Second)
	fc.FireTickers()
	deadline := time.Now().Add(5 * time.Second)
	for c.collectN.Load() == before {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for collection cycle")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestManagerStartStop(t *testing.T) {
	reg := metrics.NopRegistry{}
	m, fc := newFakeClockManager(reg, "test", time.Second)

	fakec := &fakeCollector{name: "fake"}
	m.Register(fakec)
	m.Start() // Start performs the initial forced collection (collectN == 1).

	// One tick at the 1s interval triggers a second collection (collectN == 2).
	tick(t, fc, fakec)
	m.Stop()

	if fakec.initCnt != 1 {
		t.Fatalf("Init called %d times, want 1", fakec.initCnt)
	}
	if fakec.collectN.Load() < 2 {
		t.Fatalf("Collect called %d times, want >= 2", fakec.collectN.Load())
	}
}

func TestManagerDefaultPrefix(t *testing.T) {
	m := NewManager(metrics.NopRegistry{}, "", 0, nil)
	if m.prefix != "netdata" {
		t.Fatalf("default prefix = %q, want netdata", m.prefix)
	}
}

func TestManagerDefaultInterval(t *testing.T) {
	m := NewManager(metrics.NopRegistry{}, "x", 0, nil)
	if m.interval != time.Second {
		t.Fatalf("default interval = %v, want 1s", m.interval)
	}
}

func TestManagerDisableCollector(t *testing.T) {
	reg := metrics.NopRegistry{}
	m, fc := newFakeClockManager(reg, "test", time.Second)

	enabled := &fakeCollector{name: "enabled"}
	disabled := &fakeCollector{name: "disabled"}
	m.Register(enabled)
	m.Register(disabled)

	m.SetOverrides(map[string]CollectorOverride{
		"disabled": {Enabled: false},
	})
	m.Start()

	tick(t, fc, enabled)
	m.Stop()

	if enabled.initCnt != 1 {
		t.Fatalf("enabled collector Init called %d times, want 1", enabled.initCnt)
	}
	if enabled.collectN.Load() < 1 {
		t.Fatalf("enabled collector Collect called %d times, want >= 1", enabled.collectN.Load())
	}
	if disabled.initCnt != 0 {
		t.Fatalf("disabled collector Init called %d times, want 0", disabled.initCnt)
	}
	if disabled.collectN.Load() != 0 {
		t.Fatalf("disabled collector Collect called %d times, want 0", disabled.collectN.Load())
	}
}

func TestManagerPerCollectorInterval(t *testing.T) {
	reg := metrics.NopRegistry{}
	m, fc := newFakeClockManager(reg, "test", time.Second)

	fast := &fakeCollector{name: "fast"}
	slow := &fakeCollector{name: "slow"}
	m.Register(fast)
	m.Register(slow)

	m.SetOverrides(map[string]CollectorOverride{
		"slow": {Enabled: true, Interval: 3 * time.Second},
	})
	m.Start() // initial forced collection: fast == 1, slow == 1.

	// Advance 2s in two 1s ticks. fast (1s interval) collects on each tick;
	// slow (3s interval) does not reach its interval and stays at the initial 1.
	tick(t, fc, fast)
	tick(t, fc, fast)
	m.Stop()

	if fast.collectN.Load() < 2 {
		t.Fatalf("fast collector Collect called %d times, want >= 2", fast.collectN.Load())
	}
	if slow.collectN.Load() != 1 {
		t.Fatalf("slow collector Collect called %d times, want 1 (initial only, 3s interval not reached)", slow.collectN.Load())
	}
}
