// VALIDATES: spec-ospf-ext-11 -- OSPF LDP-IGP synchronization state machine (RFC 5443
// §2 cost-out + hold-down + restore, §3 stuck alert, §4 IP-cost-only), the LDP event
// subscription, and the config -> machine wiring.
// PREVENTS: restoring cost before the hold-down (re-introducing the black hole),
// overwriting the stored configured cost with LSInfinity, a hardcoded hold-down
// default, and a leaked event subscription after shutdown.
package ospf

import (
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
)

// --- deterministic timer + bus fakes -------------------------------------------------

type fakeLDPTimer struct{ stopped bool }

func (t *fakeLDPTimer) Stop() bool {
	was := t.stopped
	t.stopped = true
	return !was
}

// fakeTimers records every armed timer so a test can fire it deterministically instead
// of sleeping.
type fakeTimers struct {
	mu    sync.Mutex
	armed []armedTimer
}

type armedTimer struct {
	d  time.Duration
	f  func()
	tm *fakeLDPTimer
}

func (c *fakeTimers) afterFunc(d time.Duration, f func()) ldpSyncTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	tm := &fakeLDPTimer{}
	c.armed = append(c.armed, armedTimer{d: d, f: f, tm: tm})
	return tm
}

// lastArmed returns the most recently armed (not-yet-stopped) timer.
func (c *fakeTimers) lastArmed(t *testing.T) armedTimer {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.armed) - 1; i >= 0; i-- {
		if !c.armed[i].tm.stopped {
			return c.armed[i]
		}
	}
	t.Fatalf("no armed timer")
	return armedTimer{}
}

// fireLast fires the most recently armed live timer's callback.
func (c *fakeTimers) fireLast(t *testing.T) {
	c.lastArmed(t).f()
}

type fakeBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newFakeBus() *fakeBus { return &fakeBus{subs: map[string][]func(any){}} }

func (b *fakeBus) Emit(ns, et string, p any) (int, error) {
	b.mu.Lock()
	src := b.subs[ns+"/"+et]
	hs := make([]func(any), len(src))
	copy(hs, src)
	b.mu.Unlock()
	for _, h := range hs {
		if h != nil {
			h(p)
		}
	}
	return 0, nil
}

func (b *fakeBus) Subscribe(ns, et string, h func(any)) func() {
	key := ns + "/" + et
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], h)
	idx := len(b.subs[key]) - 1
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		if idx < len(b.subs[key]) {
			b.subs[key][idx] = nil
		}
		b.mu.Unlock()
	}
}

// emitSession publishes an LDP session event on the fake bus, failing on error.
func emitSession(t *testing.T, bus *fakeBus, eventType, iface string) {
	t.Helper()
	if _, err := bus.Emit(ldpSyncNamespace, eventType, map[string]any{"interface": iface, "session-state": "operational"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
}

// newTestSyncManager builds a manager with deterministic timers.
func newTestSyncManager() (*ldpSyncManager, *fakeTimers) {
	m := newLDPSyncManager(func() {}, slogutil.DiscardLogger())
	ft := &fakeTimers{}
	m.afterFunc = ft.afterFunc
	return m, ft
}

func p2pMachine(m *ldpSyncManager, name string, holddown time.Duration, cost uint16) {
	m.reconcileTo(map[string]ldpSyncConfig{name: {HoldDown: holddown, Cost: cost, NetworkType: networkPointToPoint}})
}

// --- tests ---------------------------------------------------------------------------

func TestLDPSyncConfigCreatesMachine(t *testing.T) {
	m, _ := newTestSyncManager()
	p2pMachine(m, "eth0", 10*time.Second, 10)

	state, managed := m.stateFor("eth0")
	if !managed {
		t.Fatal("eth0 should be LDP-sync managed after reconcileTo")
	}
	if state != ldpSyncNotSynchronized {
		t.Fatalf("initial state = %s, want not-synchronized (RFC 5443 §2 a link that just came up)", ldpSyncStateName(state))
	}
	if got := effectiveP2PCost(state, managed, 10); got != uint16(ospflsdb.LSInfinity) {
		t.Fatalf("effective cost = %d, want LSInfinity while not synchronized", got)
	}
}

// RFC requirement: RFC5443-2-1 positive -- while LDP is not fully operational
// (not-synchronized or hold-down) the P2P link metric is forced to LSInfinity regardless
// of the configured cost, so transit traffic avoids the link.
func TestLDPSyncForcesMaxMetric(t *testing.T) {
	// AC-1 / A-1: while not synchronized the P2P effective metric is forced to
	// LSInfinity regardless of the configured cost.
	for _, st := range []int{ldpSyncNotSynchronized, ldpSyncHoldDown} {
		if got := effectiveP2PCost(st, true, 42); got != uint16(ospflsdb.LSInfinity) {
			t.Fatalf("state %s: effective cost = %d, want LSInfinity", ldpSyncStateName(st), got)
		}
	}
}

// RFC requirement: RFC5443-2-2 positive -- the OSPF max-cost value the mechanism
// substitutes is exactly LSInfinity, the 16-bit 0xFFFF.
func TestLDPSyncMaxMetricValue(t *testing.T) {
	// A-2: the substituted value is exactly LSInfinity 0xFFFF (RFC 5443 §2).
	if uint16(ospflsdb.LSInfinity) != 0xFFFF {
		t.Fatalf("LSInfinity = %#x, want 0xFFFF", uint16(ospflsdb.LSInfinity))
	}
}

// RFC requirement: RFC5443-2-5 positive -- an LDP SessionUp alone drives the link only to
// Hold-Down (metric still LSInfinity); sync is not declared until the hold-down estimate
// that all label bindings are exchanged completes.
func TestLDPSyncSubscribesSessionEvents(t *testing.T) {
	// AC-2 / A-3: a published SessionUp drives the machine to Hold-Down.
	m, _ := newTestSyncManager()
	p2pMachine(m, "eth1", 10*time.Second, 10)
	bus := newFakeBus()
	m.subscribe(bus)

	emitSession(t, bus, ldpSyncEventSessionUp, "eth1")

	state, _ := m.stateFor("eth1")
	if state != ldpSyncHoldDown {
		t.Fatalf("after SessionUp state = %s, want hold-down", ldpSyncStateName(state))
	}
	if got := effectiveP2PCost(state, true, 10); got != uint16(ospflsdb.LSInfinity) {
		t.Fatalf("hold-down cost = %d, want LSInfinity", got)
	}
}

// RFC requirement: RFC5443-2-1 negative -- once LDP is fully operational (hold-down expired
// -> Synchronized) the link metric returns to the configured cost, not LSInfinity.
// RFC requirement: RFC5443-2-5 negative -- Synchronized is declared only after the hold-down
// timer expires (the all-bindings-exchanged estimate is met); session-up alone leaves the
// link at LSInfinity.
func TestLDPSyncRestoresAfterHoldDown(t *testing.T) {
	// AC-3 / A-5 / R-1: cost is restored only on hold-down expiry, not on session-up.
	m, ft := newTestSyncManager()
	p2pMachine(m, "eth0", 30*time.Second, 100)
	m.onSessionUp("eth0")

	state, _ := m.stateFor("eth0")
	if state != ldpSyncHoldDown {
		t.Fatalf("state after session-up = %s, want hold-down", ldpSyncStateName(state))
	}
	if got := effectiveP2PCost(state, true, 100); got != uint16(ospflsdb.LSInfinity) {
		t.Fatalf("cost restored on session-up (%d); must wait for hold-down (R-1)", got)
	}

	ft.fireLast(t) // hold-down expiry

	state, _ = m.stateFor("eth0")
	if state != ldpSyncSynchronized {
		t.Fatalf("state after hold-down = %s, want synchronized", ldpSyncStateName(state))
	}
	if got := effectiveP2PCost(state, true, 100); got != 100 {
		t.Fatalf("restored cost = %d, want configured 100", got)
	}
}

func TestLDPSyncRestoresConfiguredCost(t *testing.T) {
	// AC-5 / A-9 / R-2: after sync the metric equals the configured cost, never
	// LSInfinity, and the stored configured cost is never overwritten.
	const cost = uint16(250)
	m, ft := newTestSyncManager()
	p2pMachine(m, "eth0", time.Second, cost)
	m.onSessionUp("eth0")
	ft.fireLast(t)

	state, managed := m.stateFor("eth0")
	if !managed || state != ldpSyncSynchronized {
		t.Fatalf("state = %s managed=%v, want synchronized", ldpSyncStateName(state), managed)
	}
	if got := effectiveP2PCost(state, managed, cost); got != cost {
		t.Fatalf("effective cost = %d, want %d (not 1, not LSInfinity)", got, cost)
	}
	m.mu.Lock()
	stored := m.machines["eth0"].cost
	m.mu.Unlock()
	if stored != cost {
		t.Fatalf("stored cost = %d, must remain the configured %d (R-2)", stored, cost)
	}
}

func TestLDPSyncSessionDownForcesMaxMetric(t *testing.T) {
	// AC-4: SessionDown returns a synchronized interface to not-synchronized and
	// re-forces LSInfinity.
	m, ft := newTestSyncManager()
	p2pMachine(m, "eth0", time.Second, 10)
	m.onSessionUp("eth0")
	ft.fireLast(t)
	if st, _ := m.stateFor("eth0"); st != ldpSyncSynchronized {
		t.Fatalf("precondition: want synchronized, got %s", ldpSyncStateName(st))
	}

	m.onSessionDown("eth0")

	state, _ := m.stateFor("eth0")
	if state != ldpSyncNotSynchronized {
		t.Fatalf("after session-down state = %s, want not-synchronized", ldpSyncStateName(state))
	}
	if got := effectiveP2PCost(state, true, 10); got != uint16(ospflsdb.LSInfinity) {
		t.Fatalf("cost after session-down = %d, want LSInfinity", got)
	}
}

func TestLDPSyncHoldDownConfigurable(t *testing.T) {
	// AC-9: the hold-down timer runs for the configured duration; no hardcoded default.
	const hd = 17 * time.Second
	m, ft := newTestSyncManager()
	p2pMachine(m, "eth0", hd, 10)
	m.onSessionUp("eth0")

	if got := ft.lastArmed(t).d; got != hd {
		t.Fatalf("hold-down timer armed for %s, want configured %s (no hardcoded default)", got, hd)
	}
}

func TestLDPSyncResetsOnInterfaceDown(t *testing.T) {
	// A-8: interface-down resets the machine; the next bring-up starts not-synchronized.
	m, ft := newTestSyncManager()
	p2pMachine(m, "eth0", time.Second, 10)
	m.onSessionUp("eth0")
	ft.fireLast(t)
	if st, _ := m.stateFor("eth0"); st != ldpSyncSynchronized {
		t.Fatalf("precondition synchronized, got %s", ldpSyncStateName(st))
	}

	m.reset("eth0")

	state, managed := m.stateFor("eth0")
	if !managed || state != ldpSyncNotSynchronized {
		t.Fatalf("after reset state = %s managed=%v, want not-synchronized", ldpSyncStateName(state), managed)
	}
}

// RFC requirement: RFC5443-2-2 negative -- an interface not under LDP-sync is never assigned
// the 0xFFFF max value; its effective metric stays the configured cost, so the LSInfinity
// substitution is confined to the mechanism.
func TestLDPSyncDisabledIsNoOp(t *testing.T) {
	// AC-10 / A-10: an interface without ldp-sync is unmanaged; LDP events are ignored
	// and the effective cost is the configured cost.
	m, _ := newTestSyncManager()
	bus := newFakeBus()
	m.subscribe(bus)

	emitSession(t, bus, ldpSyncEventSessionUp, "eth9")

	state, managed := m.stateFor("eth9")
	if managed {
		t.Fatal("eth9 has no ldp-sync config; must be unmanaged")
	}
	if got := effectiveP2PCost(state, managed, 7); got != 7 {
		t.Fatalf("unmanaged effective cost = %d, want configured 7 (byte-for-byte today)", got)
	}
}

func TestLDPSyncUnsubscribesOnShutdown(t *testing.T) {
	// AC-14 / R-7: after stop(), a published event no longer reaches the machine.
	m, _ := newTestSyncManager()
	p2pMachine(m, "eth0", 10*time.Second, 10)
	bus := newFakeBus()
	m.subscribe(bus)
	m.stop()

	emitSession(t, bus, ldpSyncEventSessionUp, "eth0")

	if st, _ := m.stateFor("eth0"); st != ldpSyncNotSynchronized {
		t.Fatalf("state changed to %s after unsubscribe; handler leaked (R-7)", ldpSyncStateName(st))
	}
}

func TestLDPSyncStuckRaisesAlert(t *testing.T) {
	// AC-12 / R-6: a persistent not-synchronized after having been synchronized raises
	// the RFC 5443 §3 alert and records a stuck indicator.
	m, ft := newTestSyncManager()
	var alerted []string
	m.alert = func(iface string) { alerted = append(alerted, iface) }
	p2pMachine(m, "eth0", time.Second, 10)
	m.onSessionUp("eth0")
	ft.fireLast(t) // -> synchronized
	m.onSessionDown("eth0")

	ft.fireLast(t) // stuck timer expiry

	if len(alerted) != 1 || alerted[0] != "eth0" {
		t.Fatalf("alert = %v, want one alert for eth0 (RFC 5443 §3)", alerted)
	}
	found := false
	for _, e := range m.snapshot() {
		if e.Interface == "eth0" {
			found = true
			if !e.Stuck {
				t.Fatalf("snapshot stuck=false, want true after persistent cost-out")
			}
		}
	}
	if !found {
		t.Fatal("eth0 missing from snapshot")
	}
}

// RFC requirement: RFC6138-4-1 positive -- a cut-edge broadcast interface's transit link is NOT
// withheld by LDP's operational state: ldpSyncWithholdTransit returns false for cutEdge=true even
// when the interface is not yet synchronized, so the link is advertised immediately.
// RFC requirement: RFC6138-4-1 negative -- the withhold is confined to non-cut-edges: a
// not-synchronized non-cut-edge broadcast transit link IS withheld, so the cut-edge exemption is
// meaningful rather than a blanket no-withhold.
// RFC requirement: RFC5443-4-1 positive -- the only cost the mechanism raises is the 16-bit
// IP link metric (effectiveP2PCost -> LSInfinity); ze originates no TE LSA, so no TE metric
// is ever touched.
// RFC requirement: RFC5443-4-1 negative -- the LDP-sync withhold is a pure IP-plane decision
// confined to broadcast transit links: a cut-edge is never withheld, so the mechanism never
// manufactures a TE-cost change or a CSPF/TE reroute.
// RFC requirement: RFC5443-3-1 positive -- the broadcast cost-out is a whole-segment decision:
// ldpSyncWithholdTransit keys on the segment's cut-edge status with no per-neighbor input, and
// a cut-edge segment is advertised as a whole.
// RFC requirement: RFC5443-3-1 negative -- a not-synchronized non-cut-edge broadcast segment
// withholds its transit link in its entirety (never per individual peer), so the cost-out
// granularity is the whole link, not the peer.
func TestLDPSyncTECostUntouched(t *testing.T) {
	// AC-15: the mechanism raises only the IP link cost (RFC 5443 §4), never a TE cost.
	// Ze originates no TE LSA, so the guarantee holds trivially: the only override is
	// on the 16-bit IP link metric (effectiveP2PCost) / the transit-link withhold.
	if got := effectiveP2PCost(ldpSyncNotSynchronized, true, 10); got != uint16(ospflsdb.LSInfinity) {
		t.Fatalf("IP metric override = %d, want LSInfinity", got)
	}
	// The withhold flag is a broadcast-only, IP-plane decision; a cut-edge is never
	// withheld (RFC 6138 §4) and P2P never withholds.
	if ldpSyncWithholdTransit(ldpSyncNotSynchronized, true, true) {
		t.Fatal("cut-edge must never be withheld (RFC 6138 §4 MUST NOT-delay)")
	}
	if !ldpSyncWithholdTransit(ldpSyncNotSynchronized, true, false) {
		t.Fatal("non-cut-edge not-synchronized broadcast must withhold the transit link")
	}
}
