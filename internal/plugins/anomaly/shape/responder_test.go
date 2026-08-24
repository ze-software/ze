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
	"sync"
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
	registerTables = func(_ string, tables []firewall.Table) error { tr.installed = tables; return nil }
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

// TestResponderIgnoresNonSourceEntity proves an armed responder acts on SENDERS
// only. Every term it builds matches a source address, so acting on a destination
// incident would rate-limit the victim of the traffic, and a port incident carries
// no address to act on at all.
//
// VALIDATES: child-5 AC-8 and R-5 -- the source-only guard on all three handlers.
// PREVENTS: an attacker weaponizing the detector by flooding a legitimate server
// until Ze throttles it, and a Cleared on a dest incident withdrawing a live rule
// installed for the SAME prefix as a source.
func TestResponderIgnoresNonSourceEntity(t *testing.T) {
	victim := netip.MustParsePrefix("203.0.113.5/32")

	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(&anomalyevent.AnomalyDetected{
		EntityKind: anomalyevent.EntityKindDest, Entity: victim,
	})
	tr.r.onDetected(&anomalyevent.AnomalyDetected{
		EntityKind: anomalyevent.EntityKindPort, Port: 31337, Proto: 17,
	})
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("non-source incident armed something: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}

	// The control: the same call with the source kind DOES arm, so the assertions
	// above are about the kind and not about a responder that never acts.
	tr.r.onDetected(det("198.51.100.5/32"))
	if tr.termCount() != 1 || tr.r.armedCount != 1 {
		t.Fatalf("source incident did not arm: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}

	// A destination incident on the ARMED prefix must not touch the live rule: the
	// same address can be an anomalous sender and an anomalous receiver at once.
	armed := netip.MustParsePrefix("198.51.100.5/32")
	timersBefore := len(tr.timers)
	tr.r.onOngoing(&anomalyevent.AnomalyOngoing{EntityKind: anomalyevent.EntityKindDest, Entity: armed})
	if len(tr.timers) != timersBefore {
		t.Errorf("dest ongoing re-armed the timer: %d timers, want %d", len(tr.timers), timersBefore)
	}
	tr.r.onCleared(&anomalyevent.AnomalyCleared{EntityKind: anomalyevent.EntityKindDest, Entity: armed})
	if tr.termCount() != 1 || tr.r.armedCount != 1 {
		t.Errorf("dest cleared withdrew the source rule: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
	}

	// The source-kind Cleared still withdraws, so the guard did not disable clearing.
	tr.r.onCleared(&anomalyevent.AnomalyCleared{Entity: armed})
	if tr.termCount() != 0 || tr.r.armedCount != 0 {
		t.Errorf("source cleared did not withdraw: terms=%d armed=%d", tr.termCount(), tr.r.armedCount)
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

// TestShapeStatusDuringSlowApply is the anomaly-shape counterpart of
// TestResponderStatusDuringSlowApply.
// VALIDATES: spec-fixit-firewall-concurrency-deadlock D-4 -- show.go
// handleShowAnomalyShape calls statusSnapshot(), which used to take the same mu
// that onDetected/withdraw/revertAll hold across applyAll.
// PREVENTS: `show anomaly-shape` blocking for a whole netlink round trip.
func TestShapeStatusDuringSlowApply(t *testing.T) {
	tr := newTestResponder(t, armedCfg())

	entered := make(chan struct{})
	release := make(chan struct{})
	applyAll = func() error {
		close(entered)
		<-release
		return nil
	}

	var wg sync.WaitGroup
	wg.Go(func() { tr.r.onDetected(det("198.51.100.5/32")) })
	<-entered // the reconcile is in flight and mu is held

	done := make(chan struct{})
	go func() {
		tr.r.statusSnapshot()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		close(release)
		wg.Wait()
		t.Fatal("statusSnapshot() blocked behind the in-flight reconcile: show anomaly-shape is hostage to kernel latency")
	}

	close(release)
	wg.Wait()

	st := tr.r.statusSnapshot()
	if len(st.ArmedList) != 1 || st.ArmedList[0] != "198.51.100.5/32" {
		t.Fatalf("statusSnapshot() after arming = %+v, want the armed source listed", st)
	}
}

// TestKillSwitchRepublishesStatus covers the kill-switch half of the snapshot
// contract. Arming is proven by TestShapeStatusDuringSlowApply; the kill-switch
// was proven nowhere, so deleting its publishStatus left every test in the
// package green -- gauge's republish inside revertAll runs BEFORE killed is set,
// so it publishes the OLD flag and the extra call is what makes the new one
// visible.
//
// VALIDATES: killSwitch republishes AFTER setting killed, so the lock-free
// snapshot carries the flag that stopped the responder acting.
// PREVENTS: a permanently stale, fail-open report -- `show anomaly-shape`
// answering "kill-switch: false" after the kill-switch fired, telling an
// operator the responder is still armed to act when it has been forced to
// shadow.
func TestKillSwitchRepublishesStatus(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(det("198.51.100.1/32"))
	if st := tr.r.statusSnapshot(); st.Killed || len(st.ArmedList) != 1 {
		t.Fatalf("before the kill-switch statusSnapshot() = %+v, want one armed source and killed=false", st)
	}

	tr.r.killSwitch()

	st := tr.r.statusSnapshot()
	if !st.Killed {
		t.Fatalf("statusSnapshot() = %+v after the kill-switch fired: show anomaly-shape reports kill-switch: false while the responder is forced to shadow", st)
	}
	if len(st.ArmedList) != 0 {
		t.Errorf("statusSnapshot() = %+v after the kill-switch, want no armed source", st)
	}
}

// TestStatusSnapshotDoesNotAliasPublished pins that the returned view owns its
// ArmedList. shapeStatus is the one published snapshot type carrying a slice, so
// returning *s by value hands every concurrent reader the same backing array
// while the published pointer still references it.
//
// VALIDATES: statusSnapshot copies the list, so "never mutated after Store"
// holds for readers too, not only for the writer.
// PREVENTS: a reader (a future show formatter that sorts or truncates in place)
// silently rewriting what every other reader and the published snapshot report.
func TestStatusSnapshotDoesNotAliasPublished(t *testing.T) {
	tr := newTestResponder(t, armedCfg())
	tr.r.onDetected(det("198.51.100.1/32"))

	st := tr.r.statusSnapshot()
	if len(st.ArmedList) != 1 {
		t.Fatalf("statusSnapshot() = %+v, want one armed source", st)
	}
	st.ArmedList[0] = "0.0.0.0/0"

	if again := tr.r.statusSnapshot(); again.ArmedList[0] != "198.51.100.1/32" {
		t.Fatalf("a reader's write reached the published snapshot: statusSnapshot() now reports %+v", again)
	}
}
