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

// clockBaseOffset separates the movable clock from the wall clock.
//
// A test that asserts a CARRIED timestamp has to be able to tell the instant the
// rule went in from the instant something re-stamped it. With a base of
// time.Now() a stamp taken from the REAL clock is one microsecond from a stamp
// taken from this clock at elapsed zero, so a responder that re-stamped on
// adoption computed the same age as one that carried the instant across, and the
// assertion held against both. An hour of separation is longer than any elapsed
// time these tests move, so the two readings can never be confused.
const clockBaseOffset = time.Hour

// movableClock returns a clock the test can advance without sleeping, and the
// advance function. advance sets the elapsed time ABSOLUTELY, so each call
// states a distance from the clock's base rather than from the call before it.
// The elapsed time is atomic because the worker goroutine reads the clock while
// the test writes it.
func movableClock() (now func() time.Time, advance func(time.Duration)) {
	base := time.Now().Add(-clockBaseOffset)
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

// runWorker starts the cap worker for one test and returns the stop that WAITS
// for it (ai/rules/goroutine-lifecycle.md). The wait is what keeps a tick in
// flight from reading the firewall stubs after the test has swapped them back:
// a stop that only cancels leaves the worker running through the restore.
//
// Defer the returned stop AFTER the firewall stub's own restore, so LIFO runs it
// BEFORE it.
func runWorker(t *testing.T) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	exited := startMaxDurationWorker(ctx, workerTick)
	return func() {
		cancel()
		<-exited
	}
}

// withdrawingFirewall replaces the firewall entry points and reports whether the
// ddos-local table has been withdrawn, meaning registered with no tables at all.
//
// A test that asserts a REMOVAL reads this rather than r.status(): a responder
// that never knew about the rule also reports no mitigation, so status() alone
// would pass against the very defect such a test exists to catch.
func withdrawingFirewall() (withdrawn func() bool, restore func()) {
	origReg := registerTables
	origApply := applyAll
	var gone atomic.Bool
	registerTables = func(_ string, tables []firewall.Table) error {
		gone.Store(len(tables) == 0)
		return nil
	}
	applyAll = func() error { return nil }
	return gone.Load, func() {
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

	defer runWorker(t)()

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

	defer runWorker(t)()

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

	defer runWorker(t)()

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

	defer runWorker(t)()

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

// TestLocalMitigationSurvivesAConfigApply proves an operator's unrelated commit
// cannot orphan a live drop rule.
//
// VALIDATES: AC-6 across a config apply -- the cap still removes a rule that
// went in before the apply, and it counts from the FIRST install.
// PREVENTS: the responder swap in OnConfigApply losing the mitigation. The
// firewall registry is keyed by table name, so the kernel keeps the rule after
// the responder that installed it is dropped. A fresh idle responder returns on
// !active in enforceMaxDuration and in onCleared alike, so the drop is bounded
// by neither the cap nor a clear, while show ddos local reports no mitigation.
// One ordinary commit on any ddos local leaf reaches that state.
//
// The swap is driven through replaceResponder, the one function OnConfigApply
// uses, so deleting the carry turns this test red rather than leaving it green
// over a helper nothing calls.
func TestLocalMitigationSurvivesAConfigApply(t *testing.T) {
	defer withNoopFirewall()()

	now, advance := movableClock()
	first := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 60}, nil)
	first.now = now
	publishResponder(t, first)

	defer runWorker(t)()

	first.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := first.status(); !active {
		t.Fatal("setup: the drop rule must be installed before an apply can orphan it")
	}

	// Ten seconds into a sixty-second cap the operator commits an unrelated ddos
	// local change. This call is the whole of what OnConfigApply does with the
	// responder.
	advance(10 * time.Second)
	second := replaceResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 60}, nil, first, true)
	if active, target := second.status(); !active || target.DstPrefix != floodVictim().DstPrefix {
		t.Errorf("a config apply left the drop rule in the kernel and the new responder reporting no mitigation: active=%v target=%v", active, target.DstPrefix)
	}

	// Sixty-one seconds after the FIRST install, not after the apply: the rule is
	// the same rule, so its age is the same age.
	advance(61 * time.Second)
	if removed := waitFor(2*time.Second, func() bool {
		active, _ := second.status()
		return !active
	}); !removed {
		t.Error("the cap never fired after a config apply: the drop rule outlived max-mitigation-duration with the flood still running")
	}
}

// TestLocalClearSurvivesAConfigApply proves the other removal path is armed too.
//
// VALIDATES: AC-6's premise -- a drop rule is removed when the attack clears,
// including when the clear arrives after a config apply replaced the responder.
// PREVENTS: onCleared returning on !active over a rule the previous responder
// installed, which leaves the kernel dropping the victim's traffic after the
// attack is over.
func TestLocalClearSurvivesAConfigApply(t *testing.T) {
	withdrawn, restore := withdrawingFirewall()
	defer restore()

	first := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 0}, nil)
	publishResponder(t, first)

	first.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := first.status(); !active {
		t.Fatal("setup: the drop rule must be installed before the clear can lift it")
	}
	if withdrawn() {
		t.Fatal("setup: the install must leave the ddos-local table registered")
	}

	second := replaceResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 0}, nil, first, true)
	second.onCleared(&ddosevent.AttackCleared{})

	// The kernel is what this asserts, not the snapshot: an orphaning responder
	// publishes active=false having withdrawn nothing, so a status() assertion
	// would pass over a drop rule the box is still enforcing.
	if !withdrawn() {
		t.Error("an AttackCleared after a config apply left the drop rule installed in the kernel")
	}
	if active, _ := second.status(); active {
		t.Error("the responder still reports a live mitigation after the clear")
	}
}

// TestLocalRemovingTheSectionRemovesTheDrop proves an operator who deletes the
// `ddos local` config block gets the drop rule out of the kernel.
//
// VALIDATES: AC-6's premise -- a drop rule ddos local installs is always
// reachable by something that can remove it.
// PREVENTS: the removal carrying the rule into a responder that is about to be
// discarded. A removed config root has the plugin STOPPED as soon as the reload
// transaction commits (Server.collectProcessesForRemovedConfigPaths,
// internal/component/plugin/server/startup_autoload.go), and the plugin is
// in-process, so the firewall registry it wrote to is the daemon's own and
// outlives the engine. The nftables drop would then stay in the kernel with no
// responder to see it and no cap worker left to remove it, for the life of the
// daemon.
//
// The incoming config is `enforce` on purpose. A removed section parses to
// DefaultConfig, whose response-level is `alert`, so the alert withdraw below
// would hide this arm and the removal would be handled by accident rather than
// on purpose (docs/contributing/ze-go-style.md -- a zero value is never an
// answer).
func TestLocalRemovingTheSectionRemovesTheDrop(t *testing.T) {
	withdrawn, restore := withdrawingFirewall()
	defer restore()

	first := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 0}, nil)
	publishResponder(t, first)

	first.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := first.status(); !active {
		t.Fatal("setup: the drop rule must be installed before the removal can lift it")
	}
	if withdrawn() {
		t.Fatal("setup: the install must leave the ddos-local table registered")
	}

	second := replaceResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 0}, nil, first, false)

	// The kernel is what this asserts, not the snapshot: a responder that never
	// knew about the rule also reports no mitigation, so a status() assertion
	// would pass over the very orphan this test exists to catch.
	if !withdrawn() {
		t.Error("removing the ddos local section left the drop rule installed in the kernel, with the plugin about to stop")
	}
	if active, _ := second.status(); active {
		t.Error("the responder still reports a live mitigation after the section was removed")
	}
}

// TestLocalLeavingEnforceRemovesTheDrop proves an operator can stop an on-host
// drop by committing `response-level alert`.
//
// VALIDATES: AC-6's premise across a config apply that turns mitigation off.
// PREVENTS: adoptMitigation carrying the rule unconditionally. `alert` is the
// documented "detect and report, do not block" mode (applyMitigation), and
// neither enforceMaxDuration nor onCleared reads response-level, so a carried
// rule keeps the victim blackholed until the cap expires -- up to
// max-mitigation-duration, 3600 by default -- while `show ddos local` reports it
// as active. Un-blackholing a victim is exactly what an operator switches to
// alert for.
func TestLocalLeavingEnforceRemovesTheDrop(t *testing.T) {
	withdrawn, restore := withdrawingFirewall()
	defer restore()

	first := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 0}, nil)
	publishResponder(t, first)

	first.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := first.status(); !active {
		t.Fatal("setup: the drop rule must be installed before alert mode can lift it")
	}
	if withdrawn() {
		t.Fatal("setup: the install must leave the ddos-local table registered")
	}

	second := replaceResponder(&Config{ResponseLevel: "alert", MaxMitigationDuration: 0}, nil, first, true)

	if !withdrawn() {
		t.Error("a commit of response-level alert left the drop rule installed in the kernel")
	}
	if active, _ := second.status(); active {
		t.Error("the alert-mode responder still reports a live mitigation")
	}
}

// TestLocalAdoptionTakesOwnershipFromTheOldResponder proves one rule has one
// owner across a config apply.
//
// VALIDATES: AC-6 across a config apply -- the responder that reports a live
// mitigation is the one that can remove it.
// PREVENTS: the replaced responder still acting on the rule the new one adopted.
// The cap worker reads activeResponder, so a tick that loaded the OLD pointer
// before the swap runs on the old responder afterwards, and an AttackCleared
// dispatched before unsubscribe does the same. Either would remove the rule from
// the kernel while the new responder reports it as active, so `show ddos local`
// would name a drop the box is not enforcing.
func TestLocalAdoptionTakesOwnershipFromTheOldResponder(t *testing.T) {
	withdrawn, restore := withdrawingFirewall()
	defer restore()

	now, advance := movableClock()
	first := newResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 60}, nil)
	first.now = now
	publishResponder(t, first)

	first.onDetected(&ddosevent.AttackDetected{Target: floodVictim(), Family: ddosevent.FamilyUDPFlood})
	if active, _ := first.status(); !active {
		t.Fatal("setup: the drop rule must be installed before the swap can move it")
	}

	second := replaceResponder(&Config{ResponseLevel: responseEnforce, MaxMitigationDuration: 60}, nil, first, true)

	// A cap tick that loaded the old pointer before the swap, running after it.
	advance(61 * time.Second)
	first.enforceMaxDuration()
	if withdrawn() {
		t.Error("a cap tick on the replaced responder removed the rule the new responder now owns")
	}

	// A clear dispatched before unsubscribe, delivered after the swap.
	first.onCleared(&ddosevent.AttackCleared{})
	if withdrawn() {
		t.Error("a clear still in flight on the replaced responder removed the rule the new responder now owns")
	}

	if active, _ := second.status(); !active {
		t.Error("the new responder must still report the mitigation it owns")
	}
}
