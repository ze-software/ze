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
	m := NewManager(reg, "test", 50*time.Millisecond, nil)

	fc := &fakeCollector{name: "fake"}
	m.Register(fc)
	m.Start()

	time.Sleep(120 * time.Millisecond)
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
