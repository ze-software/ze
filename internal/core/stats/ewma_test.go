// VALIDATES: AC-5 EWMA -- first sample seeds exactly, subsequent samples blend by
// alpha, Ready() flips on first Add, and an out-of-range alpha defaults sanely.
// PREVENTS: an unseeded average biasing toward zero, and an alpha of 0 freezing
// the average.

package stats

import (
	"math"
	"testing"
)

func TestEWMA(t *testing.T) {
	e := NewEWMA(0.5)
	if e.Ready() {
		t.Error("EWMA should not be ready before first Add")
	}
	if v := e.Value(); v != 0 {
		t.Errorf("EWMA value before Add = %v, want 0", v)
	}
	e.Add(10)
	if !e.Ready() {
		t.Error("EWMA should be ready after first Add")
	}
	if v := e.Value(); v != 10 {
		t.Errorf("EWMA first value = %v, want 10 (seeded exactly)", v)
	}
	e.Add(20)
	// 0.5*20 + 0.5*10 = 15
	if v := e.Value(); math.Abs(v-15) > 1e-9 {
		t.Errorf("EWMA blended = %v, want 15", v)
	}
	// out-of-range alpha defaults to 0.5, not a frozen 0
	def := NewEWMA(0)
	def.Add(4)
	def.Add(8)
	if math.Abs(def.Value()-6) > 1e-9 {
		t.Errorf("EWMA(alpha=0 -> default 0.5) = %v, want 6", def.Value())
	}
}
