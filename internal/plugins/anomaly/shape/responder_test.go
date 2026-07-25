// VALIDATES: the pinned Responder State Machine -- AC-1 shadow installs nothing,
// AC-3/AC-4 armed install + timed auto-revert (fake clock), AC-5 Cleared early
// revert, AC-6 blast-radius cap refuses, AC-7 kill-switch reverts all + forces
// shadow, AC-8 allowlist never installs, AC-9 Stop withdraws all, AC-12 Ongoing
// re-arm + stale-timer guard.
// PREVENTS: a live rule in shadow mode, a rule that outlives its TTL, a self-
// lockout on a protected source, over-arming past the cap, and a superseded timer
// double-withdrawing.

package shape

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/anomalyevent"
)

type fakeTimer struct {
	fn      func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	was := t.stopped
	t.stopped = true
	return !was
}

// testResponder wires a responder with mocked firewall install/withdraw and an
// injectable clock so timers fire on demand.
type testResponder struct {
	r         *responder
	installed []firewall.Table
	applies   int
	timers    []*fakeTimer
}

func newTestResponder(t *testing.T, cfg *Config) *testResponder {
	t.Helper()
	tr := &testResponder{}
	registerTables = func(_ string, tables []firewall.Table) { tr.installed = tables }
	applyAll = func() error { tr.applies++; return nil }
	t.Cleanup(func() {
		registerTables = firewall.RegisterTables
		applyAll = firewall.ApplyAll
	})
	tr.r = newResponder(cfg)
	tr.r.afterFunc = func(_ time.Duration, f func()) stopper {
		ft := &fakeTimer{fn: f}
		tr.timers = append(tr.timers, ft)
		return ft
	}
	return tr
}

func (tr *testResponder) termCount() int {
	n := 0
	for _, tb := range tr.installed {
		n += len(tb.Chains[0].Terms)
	}
	return n
}

func det(p string) *anomalyevent.AnomalyDetected {
	return &anomalyevent.AnomalyDetected{Entity: netip.MustParsePrefix(p)}
}

func armedCfg() *Config {
	c := DefaultConfig()
	c.Mode = ModeArmed
	c.BlastRadiusCap = 2
	return c
}

func TestShapeShadowNoInstall(t *testing.T) {
	tr := newTestResponder(t, DefaultConfig()) // shadow
	tr.r.onDetected(det("198.51.100.5/32"))
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("shadow mode installed something: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}
}

func TestArmedInstallAndAutoRevert(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(det("198.51.100.5/32"))
	if tr.termCount() != 1 || tr.r.armedCount != 1 {
		t.Fatalf("armed install: terms=%d armed=%d, want 1/1", tr.termCount(), tr.r.armedCount)
	}
	if len(tr.timers) != 1 {
		t.Fatalf("timers = %d, want 1 (auto-revert armed)", len(tr.timers))
	}
	// Fire the auto-revert timer: the rule withdraws even with no Cleared event.
	tr.timers[len(tr.timers)-1].fn()
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("after auto-revert: terms=%d armed=%d, want 0/0", tr.termCount(), tr.r.armedCount)
	}
}

func TestClearedEarlyRevert(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(det("198.51.100.5/32"))
	tr.r.onCleared(&anomalyevent.AnomalyCleared{Entity: netip.MustParsePrefix("198.51.100.5/32")})
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("after cleared: terms=%d armed=%d, want 0/0", tr.termCount(), tr.r.armedCount)
	}
}

func TestBlastRadiusCap(t *testing.T) {
	tr := newTestResponder(t, armedCfg()) // cap 2
	tr.r.onDetected(det("198.51.100.1/32"))
	tr.r.onDetected(det("198.51.100.2/32"))
	tr.r.onDetected(det("198.51.100.3/32")) // over cap -> refused
	if tr.r.armedCount != 2 {
		t.Errorf("armedCount = %d, want 2 (cap enforced)", tr.r.armedCount)
	}
	if tr.termCount() != 2 {
		t.Errorf("terms = %d, want 2", tr.termCount())
	}
}

func TestKillSwitchRevertsAll(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(det("198.51.100.1/32"))
	tr.r.onDetected(det("198.51.100.2/32"))
	tr.r.killSwitch()
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("after kill-switch: terms=%d armed=%d, want 0/0", tr.termCount(), tr.r.armedCount)
	}
	// Forced to shadow: a new incident installs nothing.
	tr.r.onDetected(det("198.51.100.9/32"))
	if tr.termCount() != 0 {
		t.Errorf("kill-switch did not force shadow: terms=%d", tr.termCount())
	}
}

func TestAllowlistNeverInstalls(t *testing.T) {
	cfg := armedCfg()
	cfg.Allowlist = []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	tr := newTestResponder(t, cfg)
	tr.r.onDetected(det("198.51.100.5/32")) // protected
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("allowlisted source armed: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}
}

func TestStopWithdrawsAll(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(det("198.51.100.1/32"))
	tr.r.Stop()
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("after Stop: terms=%d armed=%d, want 0/0", tr.termCount(), tr.r.armedCount)
	}
}

func TestOngoingExtendsTTLAndStaleTimerNoop(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	e := "198.51.100.5/32"
	tr.r.onDetected(det(e))
	stale := tr.timers[0] // the original timer
	tr.r.onOngoing(&anomalyevent.AnomalyOngoing{Entity: netip.MustParsePrefix(e)})
	if len(tr.timers) != 2 {
		t.Fatalf("Ongoing did not re-arm the timer: timers=%d", len(tr.timers))
	}
	if tr.r.armedCount != 1 || tr.termCount() != 1 {
		t.Errorf("Ongoing changed armed state: armed=%d terms=%d, want 1/1", tr.r.armedCount, tr.termCount())
	}
	// The superseded (stale) timer firing must be a no-op (generation guard).
	stale.fn()
	if tr.termCount() != 1 || tr.r.armedCount != 1 {
		t.Errorf("stale timer withdrew a live rule: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}
}

// TestStaleTimerAcrossReArmNoop proves the generation guard survives record
// RECREATION: a timer from a first arming must not withdraw a freshly re-armed
// rule after a clear. A per-record generation (reset on the new record) would
// collide here; the responder-level monotonic generation does not.
func TestStaleTimerAcrossReArmNoop(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	e := "198.51.100.5/32"
	pfx := netip.MustParsePrefix(e)

	tr.r.onDetected(det(e)) // first arming -> timer0
	firstTimer := tr.timers[0]

	tr.r.onCleared(&anomalyevent.AnomalyCleared{Entity: pfx}) // withdraw the first record
	tr.r.onDetected(det(e))                                   // re-arm -> a NEW record + timer1
	if tr.termCount() != 1 || tr.r.armedCount != 1 {
		t.Fatalf("re-arm failed: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}

	// The first arming's (now stale) timer firing must NOT withdraw the fresh rule.
	firstTimer.fn()
	if tr.termCount() != 1 || tr.r.armedCount != 1 {
		t.Errorf("stale timer from the first arming withdrew the re-armed rule: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}
}

type fakeCounter struct{ n int }

func (c *fakeCounter) Inc()          { c.n++ }
func (c *fakeCounter) Add(v float64) { c.n += int(v) }

type fakeGauge struct{}

func (fakeGauge) Set(float64) {}
func (fakeGauge) Inc()        {}
func (fakeGauge) Dec()        {}
func (fakeGauge) Add(float64) {}

// TestRevertedCounterCountsClearAndAutoRevert proves ze_anomaly_shape_reverted_total
// counts BOTH withdrawal paths -- the early clear (onCleared) and the timed
// auto-revert -- matching its help text (a prior cut counted only auto-revert).
func TestRevertedCounterCountsClearAndAutoRevert(t *testing.T) {
	rev := &fakeCounter{}
	metricsPtr.Store(&shapeMetrics{
		armed: fakeGauge{}, reverted: rev, armRefused: &fakeCounter{}, killswitch: &fakeCounter{},
	})
	t.Cleanup(func() { metricsPtr.Store(nil) })

	tr := newTestResponder(t, armedCfg())

	// Early clear -> counts one.
	tr.r.onDetected(det("198.51.100.1/32"))
	tr.r.onCleared(&anomalyevent.AnomalyCleared{Entity: netip.MustParsePrefix("198.51.100.1/32")})
	if rev.n != 1 {
		t.Fatalf("reverted after clear = %d, want 1", rev.n)
	}

	// Timed auto-revert -> counts one more.
	tr.r.onDetected(det("198.51.100.2/32"))
	tr.timers[len(tr.timers)-1].fn()
	if rev.n != 2 {
		t.Errorf("reverted after auto-revert = %d, want 2", rev.n)
	}
}
