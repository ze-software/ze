package collector

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type fakeCollector struct {
	name     string
	initCnt  int
	collectN int
}

func (f *fakeCollector) Name() string                      { return f.name }
func (f *fakeCollector) Init(_ metrics.Registry, _ string) { f.initCnt++ }
func (f *fakeCollector) Collect() error                    { f.collectN++; return nil }

func TestManagerStartStop(t *testing.T) {
	reg := metrics.NopRegistry{}
	m := NewManager(reg, "test", time.Second, nil)

	fc := &fakeCollector{name: "fake"}
	m.Register(fc)
	m.Start()

	time.Sleep(1200 * time.Millisecond)
	m.Stop()

	if fc.initCnt != 1 {
		t.Fatalf("Init called %d times, want 1", fc.initCnt)
	}
	if fc.collectN < 2 {
		t.Fatalf("Collect called %d times, want >= 2", fc.collectN)
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
	m := NewManager(reg, "test", time.Second, nil)

	enabled := &fakeCollector{name: "enabled"}
	disabled := &fakeCollector{name: "disabled"}
	m.Register(enabled)
	m.Register(disabled)

	m.SetOverrides(map[string]CollectorOverride{
		"disabled": {Enabled: false},
	})
	m.Start()

	time.Sleep(1200 * time.Millisecond)
	m.Stop()

	if enabled.initCnt != 1 {
		t.Fatalf("enabled collector Init called %d times, want 1", enabled.initCnt)
	}
	if enabled.collectN < 1 {
		t.Fatalf("enabled collector Collect called %d times, want >= 1", enabled.collectN)
	}
	if disabled.initCnt != 0 {
		t.Fatalf("disabled collector Init called %d times, want 0", disabled.initCnt)
	}
	if disabled.collectN != 0 {
		t.Fatalf("disabled collector Collect called %d times, want 0", disabled.collectN)
	}
}

func TestManagerPerCollectorInterval(t *testing.T) {
	reg := metrics.NopRegistry{}
	m := NewManager(reg, "test", time.Second, nil)

	fast := &fakeCollector{name: "fast"}
	slow := &fakeCollector{name: "slow"}
	m.Register(fast)
	m.Register(slow)

	m.SetOverrides(map[string]CollectorOverride{
		"slow": {Enabled: true, Interval: 3 * time.Second},
	})
	m.Start()

	time.Sleep(2500 * time.Millisecond)
	m.Stop()

	if fast.collectN < 2 {
		t.Fatalf("fast collector Collect called %d times, want >= 2", fast.collectN)
	}
	if slow.collectN != 1 {
		t.Fatalf("slow collector Collect called %d times, want 1 (initial only, 3s interval not reached)", slow.collectN)
	}
}
