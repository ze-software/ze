package local

import (
	"context"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

// The worker under test ticks on a wall clock, so the tests drive it at a
// cadence far below any cap they set and move the responder's own clock
// instead. workerTick is that cadence: fast enough that a bounded wait covers
// many ticks, slow enough not to spin a core.
const workerTick = time.Millisecond

// waitFor polls until want returns true or the deadline passes. It reports
// whether the condition was reached, so a caller can name its own failure.
func waitFor(within time.Duration, want func() bool) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if want() {
			return true
		}
		time.Sleep(workerTick)
	}
	return want()
}

// movableClock returns a clock the test can advance without sleeping, and the
// advance function. The elapsed time is atomic because the worker goroutine
// reads the clock while the test writes it.
func movableClock() (now func() time.Time, advance func(time.Duration)) {
	base := time.Now()
	var elapsed atomic.Int64
	return func() time.Time { return base.Add(time.Duration(elapsed.Load())) },
		func(d time.Duration) { elapsed.Store(int64(d)) }
}

// countingFirewall replaces the firewall entry points with counting stubs, so a
// test can assert that an idle worker touched the kernel not at all. The counts
// are atomic because the worker goroutine increments them.
func countingFirewall() (applies *atomic.Int64, restore func()) {
	origReg := registerTables
	origApply := applyAll
	var n atomic.Int64
	registerTables = func(_ string, _ []firewall.Table) error { return nil }
	applyAll = func() error {
		n.Add(1)
		return nil
	}
	return &n, func() {
		registerTables = origReg
		applyAll = origApply
	}
}

// publishResponder makes r the responder the worker acts on, the way a config
// apply does, and detaches it when the test ends.
func publishResponder(t *testing.T, r *responder) {
	t.Helper()
	activeResponder.Store(r)
	t.Cleanup(func() { activeResponder.Store(nil) })
}

func floodVictim() ddosevent.VectorTuple {
	return ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
		Proto:     17,
		DstPort:   53,
	}
}

// TestLocalMaxDurationRemovesTheRule proves the operator's cap is enforced by a
// running worker and not merely parsed.
//
// VALIDATES: AC-6 -- a drop rule older than max-mitigation-duration is removed
// while the attack is still running and no AttackCleared has arrived.
// PREVENTS: the leaf going back to being parsed, range-checked and read by
// nothing. removeMitigation had two callers before this test, the
// suppress branch of applyMitigation and onCleared, and neither is a timer, so
// an attack that never cleared held an nftables drop for the life of the daemon.
//
// The cap is driven through the worker startMaxDurationWorker starts, never by
// calling enforceMaxDuration directly: a test that calls the enforcement
// function itself stays green when the wiring is deleted, which is the shape
// plan/journal/unwired-feature.md records as the reason the sibling defect
// survived for months.
func TestLocalMaxDurationRemovesTheRule(t *testing.T) {
	defer withNoopFirewall()()

	r := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 60}, nil)
	now, advance := movableClock()
	r.now = now
	publishResponder(t, r)

	startMaxDurationWorker(t.Context(), workerTick)

	r.onDetected(&ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    floodVictim(),
		Family:    ddosevent.FamilyUDPFlood,
	})
	if active, _ := r.status(); !active {
		t.Fatal("setup: the drop rule must be installed before the cap can lift it")
	}

	advance(59 * time.Second)
	if removed := waitFor(50*time.Millisecond, func() bool {
		active, _ := r.status()
		return !active
	}); removed {
		t.Error("the cap fired before max-mitigation-duration elapsed")
	}

	advance(61 * time.Second)
	if removed := waitFor(2*time.Second, func() bool {
		active, _ := r.status()
		return !active
	}); !removed {
		t.Error("a drop rule older than max-mitigation-duration must be removed by the worker")
	}
}

// TestLocalMaxDurationZeroMeansNoCap pins the documented escape.
//
// VALIDATES: AC-7 -- "0 = no cap", as the leaf's own description states and as
// the flowspec twin already implements.
// PREVENTS: a zero read as a deadline of zero seconds, which would remove every
// drop rule on the worker's first tick and leave the box unprotected under a
// flood (ai/rules/principles.md -- a zero is never an answer).
func TestLocalMaxDurationZeroMeansNoCap(t *testing.T) {
	defer withNoopFirewall()()

	r := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 0}, nil)
	now, advance := movableClock()
	r.now = now
	publishResponder(t, r)

	startMaxDurationWorker(t.Context(), workerTick)

	r.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := r.status(); !active {
		t.Fatal("setup: the drop rule must be installed before the cap could lift it")
	}

	advance(72 * time.Hour)
	if removed := waitFor(100*time.Millisecond, func() bool {
		active, _ := r.status()
		return !active
	}); removed {
		t.Error("0 means no cap: the drop rule must survive any elapsed time")
	}
}

// TestLocalMaxDurationClockStartsOnTheFirstInstall pins where the cap clock is
// written.
//
// VALIDATES: AC-8 -- a re-install in place keeps counting from the FIRST
// install.
// PREVENTS: the clock being written by applyMitigation. That function runs on
// both AttackDetected and AttackCharacterized and re-installs while the rule is
// already live, so a clock written there restarts on every characterization and
// an attack that re-characterizes inside its own cap never expires.
func TestLocalMaxDurationClockStartsOnTheFirstInstall(t *testing.T) {
	defer withNoopFirewall()()

	r := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 60}, nil)
	now, advance := movableClock()
	r.now = now
	publishResponder(t, r)

	startMaxDurationWorker(t.Context(), workerTick)

	r.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := r.status(); !active {
		t.Fatal("setup: the first install must be live before the refresh can be tested")
	}

	// Half way through the cap, characterization narrows the rule in place. The
	// rule stays live across this, so the cap must keep counting from the first
	// install.
	advance(30 * time.Second)
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: floodVictim(),
		Family: ddosevent.FamilyUDPFlood,
	})
	if active, _ := r.status(); !active {
		t.Fatal("setup: the characterized re-install must leave the rule live")
	}

	advance(61 * time.Second)
	if removed := waitFor(2*time.Second, func() bool {
		active, _ := r.status()
		return !active
	}); !removed {
		t.Error("a refresh restarted the cap clock: 61s after the FIRST install the rule must be gone")
	}
}

// TestLocalMaxDurationIdleWorkerRemovesNothing proves the worker costs nothing
// on a box that is not under attack.
//
// VALIDATES: AC-9 -- with no rule installed the worker removes nothing, however
// long it runs and whatever the cap is.
// PREVENTS: an unconditional withdraw on the tick, which would reconcile the
// firewall once a second forever and log a removal nobody asked for.
func TestLocalMaxDurationIdleWorkerRemovesNothing(t *testing.T) {
	applies, restore := countingFirewall()
	defer restore()

	r := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 1}, nil)
	now, advance := movableClock()
	r.now = now
	publishResponder(t, r)

	startMaxDurationWorker(t.Context(), workerTick)

	advance(72 * time.Hour)
	// Many ticks pass at workerTick, and none of them has a rule to remove.
	time.Sleep(50 * time.Millisecond)

	if active, _ := r.status(); active {
		t.Error("an idle responder must never report a live mitigation")
	}
	if n := applies.Load(); n != 0 {
		t.Errorf("the idle worker reconciled the firewall %d times; it must not touch the kernel with no rule installed", n)
	}
}

// TestLocalMaxDurationWorkerStops pins the worker's stop path
// (ai/rules/goroutine-lifecycle.md): the owner cancels the context, and the
// worker's exit is observable so a caller can wait for it.
func TestLocalMaxDurationWorkerStops(t *testing.T) {
	defer withNoopFirewall()()

	ctx, cancel := context.WithCancel(t.Context())
	exited := startMaxDurationWorker(ctx, workerTick)

	select {
	case <-exited:
		t.Fatal("the worker exited before its context was canceled")
	case <-time.After(10 * time.Millisecond):
	}

	cancel()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Error("the worker did not exit after its context was canceled")
	}
}
