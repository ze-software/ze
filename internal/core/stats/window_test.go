// VALIDATES: AC-5/AC-6 the shared Window primitive -- delta and cumulative bytes
// become a per-second rate over dt, reset each Tick, the idle counter advances on
// quiet ticks, and history is bounded to histCap (the behavior trafficstat's
// aggregator is rebuilt onto without change).
// PREVENTS: the accumulate-forever rate regression, a cumulative counter reset
// producing a negative contribution, and unbounded per-key history growth.

package stats

import "testing"

func TestWindowRateAndReset(t *testing.T) {
	// delta path: two 1000-byte deltas over 1s -> 2000 bps, then reset to 0.
	w := NewWindow(3)
	w.AddDelta(1000)
	w.AddDelta(1000)
	if bps := w.Tick(1.0); bps != 2000 {
		t.Fatalf("delta rate = %v, want 2000", bps)
	}
	if bps := w.Tick(1.0); bps != 0 {
		t.Fatalf("rate after idle tick = %v, want 0 (reset)", bps)
	}
	if w.Idle() != 1 {
		t.Errorf("idle = %d, want 1 after one zero tick", w.Idle())
	}

	// cumulative path: first sample primes, second contributes the diff.
	c := NewWindow(0)
	c.AddCumulative(1000)
	c.AddCumulative(3000)
	if bps := c.Tick(1.0); bps != 2000 {
		t.Fatalf("cumulative rate = %v, want 2000", bps)
	}
	// counter reset (value drops) contributes nothing, not a negative rate.
	c.AddCumulative(500)
	if bps := c.Tick(1.0); bps != 0 {
		t.Fatalf("rate after counter reset = %v, want 0", bps)
	}

	// dt scaling: 2000 bytes over 2s -> 1000 bps.
	d := NewWindow(0)
	d.AddDelta(2000)
	if bps := d.Tick(2.0); bps != 1000 {
		t.Fatalf("rate over dt=2 = %v, want 1000", bps)
	}
}

func TestWindowHistoryBounded(t *testing.T) {
	w := NewWindow(3)
	for i := range 10 {
		w.AddDelta(float64((i + 1) * 100))
		w.Tick(1.0)
	}
	h := w.History()
	if len(h) != 3 {
		t.Fatalf("history len = %d, want capped at 3", len(h))
	}
	// newest last: last three rates were 800, 900, 1000.
	if h[2] != 1000 {
		t.Errorf("newest history = %v, want 1000", h[2])
	}

	// histCap 0 -> no history retained.
	z := NewWindow(0)
	z.AddDelta(500)
	z.Tick(1.0)
	if z.History() != nil {
		t.Errorf("histCap=0 History = %v, want nil", z.History())
	}
}

func TestWindowIdleCounter(t *testing.T) {
	w := NewWindow(0)
	w.AddDelta(100)
	w.Tick(1.0)
	if w.Idle() != 0 {
		t.Errorf("idle after traffic = %d, want 0", w.Idle())
	}
	for i := 1; i <= 5; i++ {
		w.Tick(1.0)
		if w.Idle() != i {
			t.Errorf("idle = %d, want %d", w.Idle(), i)
		}
	}
	// traffic resets idle.
	w.AddDelta(100)
	w.Tick(1.0)
	if w.Idle() != 0 {
		t.Errorf("idle after resumed traffic = %d, want 0", w.Idle())
	}
}
