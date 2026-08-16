package flowspec

import (
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// fakeDispatcher records the commands the responder would send to the engine.
type fakeDispatcher struct {
	cmds []string
	err  error
}

func (f *fakeDispatcher) Dispatch(cmd string) error {
	f.cmds = append(f.cmds, cmd)
	return f.err
}

// blockingDispatcher holds the caller inside Dispatch until released. It stands
// in for the production sdkDispatcher (register.go), whose Dispatch is
// p.UpdateRoute: an RPC round trip to the BGP engine.
type blockingDispatcher struct {
	entered chan struct{}
	release chan struct{}
}

func (d *blockingDispatcher) Dispatch(string) error {
	close(d.entered)
	<-d.release
	return nil
}

// TestFlowspecStatusDuringSlowDispatch proves the show surface reads the
// announcement state without waiting on a BGP-engine round trip.
//
// VALIDATES: status() takes no lock -- announce (and withdraw) hold r.mu across
// dispatcher.Dispatch, and show.go handleShowDdosFlowspec reaches status()
// through the plugin dispatch-command path.
// PREVENTS: `show ddos flowspec` stalling for a full UpdateRoute RPC round trip,
// unbounded while announce/withdraw churn under a flood. Same defect and same
// fix shape as D-3 (ddos/local) and D-4 (anomaly/shape) of
// plan/spec-fixit-firewall-concurrency-deadlock.md; this is the fourth instance.
func TestFlowspecStatusDuringSlowDispatch(t *testing.T) {
	disp := &blockingDispatcher{entered: make(chan struct{}), release: make(chan struct{})}
	r := newResponder(&Config{
		ResponseLevel: responseEnforce, BlackholeFallback: true,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)

	var wg sync.WaitGroup
	wg.Go(func() {
		r.onDetected(&ddosevent.AttackDetected{
			Interface: "xe0",
			Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 17},
			Family:    ddosevent.FamilyUDPFlood,
			Severity:  ddosevent.SeverityCritical,
			Direction: ddosevent.DirectionRemote,
		})
	})
	<-disp.entered // the announce is in flight and r.mu is held

	done := make(chan struct{})
	go func() {
		r.status()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		close(disp.release)
		wg.Wait()
		t.Fatal("status() blocked behind the in-flight announce: show ddos flowspec is hostage to BGP-engine RPC latency")
	}

	close(disp.release)
	wg.Wait()

	// The snapshot the show handler reads must still track the announcement.
	if active, target, probing := r.status(); !active || target.Proto != 17 || !probing {
		t.Fatalf("status() after a successful announce = (%v, %+v, probing=%v), want an active rule carrying the announced vector with the leak-probe running",
			active, target, probing)
	}
}

// TestFlowspecWithdrawRepublishesStatus covers the WITHDRAW half of the snapshot
// contract. Announcing is proven by TestFlowspecStatusDuringSlowDispatch;
// withdrawing was proven nowhere, so deleting publishStatus from withdraw left
// every test in the package green.
//
// VALIDATES: withdraw republishes, so the lock-free snapshot stops claiming an
// upstream rule once the leak-probe has withdrawn it.
// PREVENTS: a permanently stale, fail-open report -- `show ddos flowspec`
// naming an announced FlowSpec rule (and a running leak-probe) after the
// withdraw reached the BGP engine, with no later event able to correct it.
func TestFlowspecWithdrawRepublishesStatus(t *testing.T) {
	r := newResponder(&Config{
		ResponseLevel: responseEnforce,
		HoldDown:      2, ProbeInterval: 3, ProbeWindow: 2, ProbeRate: 1000000, BackoffCap: 3600,
	}, &fakeDispatcher{})
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 17},
	})
	if active, _, probing := r.status(); !active || !probing {
		t.Fatalf("announce did not publish (active=%v probing=%v): the test proves nothing about the withdraw", active, probing)
	}

	for range 10 { // drive the leak-probe past hold-down into a withdraw
		r.probeTick(0)
	}
	if r.active {
		t.Fatal("the leak-probe did not withdraw: the test never reaches the path it covers")
	}
	if active, target, probing := r.status(); active || probing {
		t.Fatalf("status() = (active=%v, %+v, probing=%v) after the withdraw: show ddos flowspec reports an upstream rule the BGP engine no longer holds",
			active, target, probing)
	}
}

func TestIgnoresDetectorClearWhileMitigating(t *testing.T) {
	// VALIDATES: AC-12 -- AttackCleared does not withdraw while mitigating
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600}, &fakeDispatcher{})
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if !r.active {
		t.Fatal("should be active after characterize")
	}
	r.onCleared(&ddosevent.AttackCleared{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32")},
	})
	if !r.active {
		t.Error("should still be active after detector clear (leak-probe decides)")
	}
}

func TestAlertModeDoesNotAnnounce(t *testing.T) {
	// VALIDATES: AC-6 -- alert mode logs, does not announce
	disp := &fakeDispatcher{}
	r := newResponder(&Config{ResponseLevel: "alert", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600}, disp)
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if r.active {
		t.Error("alert mode should not activate mitigation")
	}
	if len(disp.cmds) != 0 {
		t.Errorf("alert mode must not dispatch, got %v", disp.cmds)
	}
}

// VALIDATES: AC-11/AC-12 -- confidence-min gates the characterized announce path;
// the default of 0 leaves behavior unchanged (announces regardless of confidence).
func TestFlowspecConfidenceGate(t *testing.T) {
	victim := netip.MustParsePrefix("10.0.0.1/32")
	cfg := func(min int) *Config {
		return &Config{ResponseLevel: "enforce", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600, ConfidenceMin: min}
	}
	ev := func(conf int) *ddosevent.AttackCharacterized {
		return &ddosevent.AttackCharacterized{Target: ddosevent.VectorTuple{DstPrefix: victim}, Family: ddosevent.FamilyUDPFlood, Confidence: conf}
	}

	// Below min: no announce.
	r := newResponder(cfg(80), &fakeDispatcher{})
	r.onCharacterized(ev(50))
	if r.active {
		t.Error("confidence 50 below min 80 must not announce")
	}
	// At/above min: announce.
	r2 := newResponder(cfg(80), &fakeDispatcher{})
	r2.onCharacterized(ev(90))
	if !r2.active {
		t.Error("confidence 90 >= min 80 must announce")
	}
	// Default 0: announces regardless of a low confidence.
	r3 := newResponder(cfg(0), &fakeDispatcher{})
	r3.onCharacterized(ev(1))
	if !r3.active {
		t.Error("confidence-min 0 must not gate (behavior unchanged)")
	}
}

func TestEnforceModeAnnouncesOnCharacterized(t *testing.T) {
	// VALIDATES: AC-7 -- flowspec announces the precise rule from AttackCharacterized
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600}, &fakeDispatcher{})
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17, DstPort: 53},
		Family: ddosevent.FamilyUDPFlood,
	})
	if !r.active {
		t.Error("enforce mode should announce on characterized")
	}
}

func TestFlowspecWaitsForCharacterized(t *testing.T) {
	// VALIDATES: AC-8 -- a fast AttackDetected does NOT announce upstream when
	// the blackhole-fallback policy is off; flowspec waits for characterization.
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600}, &fakeDispatcher{})
	r.onDetected(&ddosevent.AttackDetected{
		Target:   ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32")},
		Severity: ddosevent.SeverityCritical, // even critical waits without the policy
	})
	if r.active {
		t.Error("flowspec must not announce on AttackDetected without blackhole-fallback")
	}
}

func TestBlackholeFallbackOnCritical(t *testing.T) {
	// VALIDATES: AC-14 -- with blackhole-fallback enabled, a critical fast signal
	// engages an immediate discard; a non-critical one still waits.
	newR := func() *responder {
		return newResponder(&Config{
			ResponseLevel: "enforce", BlackholeFallback: true,
			HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
		}, &fakeDispatcher{})
	}

	crit := newR()
	crit.onDetected(&ddosevent.AttackDetected{
		Target:   ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32")},
		Severity: ddosevent.SeverityCritical,
	})
	if !crit.active {
		t.Error("critical severity with blackhole-fallback should engage immediately")
	}

	high := newR()
	high.onDetected(&ddosevent.AttackDetected{
		Target:   ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32")},
		Severity: ddosevent.SeverityHigh, // below critical -> still waits
	})
	if high.active {
		t.Error("non-critical severity must not engage the blackhole fallback")
	}
}

func TestProbeTickWithdraws(t *testing.T) {
	// VALIDATES: AC-5 -- probe-driven withdraw after sub-rate window
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 2, ProbeInterval: 3, ProbeWindow: 2, ProbeRate: 1000000, BackoffCap: 3600}, &fakeDispatcher{})
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if !r.active {
		t.Fatal("should be active")
	}
	// Advance past hold-down + to probe window
	for range 10 {
		r.probeTick(0)
	}
	if r.active {
		t.Error("should be inactive after probe clears")
	}
}

// TestAllowlistedTargetNotAnnounced reworked into
// TestSuppressMitigationNotAnnounced -- the allowlist moved to the detector policy;
// the responder now honors the event's SuppressMitigation flag instead of a local list.
func TestSuppressMitigationNotAnnounced(t *testing.T) {
	// VALIDATES: a policy-exempted attack (event SuppressMitigation) is not announced.
	r := newResponder(&Config{
		ResponseLevel: "enforce",
		HoldDown:      300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, &fakeDispatcher{})
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target:             ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
		Direction:          ddosevent.DirectionRemote,
		SuppressMitigation: true,
	})
	if r.active {
		t.Error("policy-exempted (SuppressMitigation) target should not be mitigated")
	}
}

// TestFlowspecSkipsLocalVictim validates that flowspec leaves a local (box-owned)
// victim to on-host mitigation and does not announce upstream for it.
func TestFlowspecSkipsLocalVictim(t *testing.T) {
	r := newResponder(&Config{
		ResponseLevel: "enforce",
		HoldDown:      300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, &fakeDispatcher{})
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
		Direction: ddosevent.DirectionLocal,
	})
	if r.active {
		t.Error("flowspec must skip a local victim (on-host mitigation handles it)")
	}
}

func TestResponderAnnounceEmitsUpdateText(t *testing.T) {
	// VALIDATES: AC-1 -- enforce + characterized emits the update-text add command
	// with the traffic-rate ext-community and the characterized components.
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", Action: "rate-limit", RateLimitBytes: 9600,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 6, DstPort: 80},
	})
	if len(disp.cmds) != 1 {
		t.Fatalf("expected 1 dispatched command, got %d: %v", len(disp.cmds), disp.cmds)
	}
	want := "update text extended-community [rate-limit:9600] nhop self nlri ipv4/flow add destination 192.0.2.0/24 protocol =6 destination-port =80"
	if disp.cmds[0] != want {
		t.Errorf("announce command mismatch:\n got %q\nwant %q", disp.cmds[0], want)
	}
}

func TestResponderWithdrawEmitsDel(t *testing.T) {
	// VALIDATES: AC-3 -- leak-probe clear emits a matching del re-rendered from
	// the stored match, with no ext-community (flowspec key is the NLRI).
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", Action: "rate-limit", RateLimitBytes: 9600,
		HoldDown: 2, ProbeInterval: 3, ProbeWindow: 2, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 6, DstPort: 80},
	})
	for range 10 {
		r.probeTick(0)
	}
	if r.active {
		t.Fatal("should be withdrawn after probe clears")
	}
	last := disp.cmds[len(disp.cmds)-1]
	want := "update text nlri ipv4/flow del destination 192.0.2.0/24 protocol =6 destination-port =80"
	if last != want {
		t.Errorf("withdraw command mismatch:\n got %q\nwant %q", last, want)
	}
}

func TestBuildFlowspecUpdateText(t *testing.T) {
	// VALIDATES: renderer grammar (spec "Responder Renderer Grammar") across
	// discard/rate-limit-0 equivalence, v6, tcp-flags, and del.
	tests := []struct {
		name   string
		match  flowspecMatch
		action string
		rate   uint64
		mode   string
		want   string
	}{
		{
			"rate-limit v4 full", flowspecMatch{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 6, DstPort: 80, SrcPort: 1024, TCPFlags: 0x12}, "rate-limit", 9600, "add",
			"update text extended-community [rate-limit:9600] nhop self nlri ipv4/flow add destination 192.0.2.0/24 protocol =6 destination-port =80 source-port =1024 tcp-flags syn&ack",
		},
		{
			"discard v4 dst-only", flowspecMatch{DstPrefix: netip.MustParsePrefix("203.0.113.5/32")}, "discard", 0, "add",
			"update text extended-community [rate-limit:0] nhop self nlri ipv4/flow add destination 203.0.113.5/32",
		},
		{
			"rate-limit 0 equals discard", flowspecMatch{DstPrefix: netip.MustParsePrefix("203.0.113.5/32")}, "rate-limit", 0, "add",
			"update text extended-community [rate-limit:0] nhop self nlri ipv4/flow add destination 203.0.113.5/32",
		},
		{
			"v6 with protocol", flowspecMatch{DstPrefix: netip.MustParsePrefix("2001:db8::/32"), Proto: 17}, "rate-limit", 1000, "add",
			"update text extended-community [rate-limit:1000] nhop self nlri ipv6/flow add destination 2001:db8::/32 protocol =17",
		},
		{
			"del omits ext-community", flowspecMatch{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 6, DstPort: 80}, "", 0, "del",
			"update text nlri ipv4/flow del destination 192.0.2.0/24 protocol =6 destination-port =80",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderFlowspecCommand(tc.match, tc.action, tc.rate, tc.mode)
			if got != tc.want {
				t.Errorf("\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestSuppressMitigationWithdrawsBlackholeFallback pins the withdraw the
// flowspec responder owed and did not make.
//
// VALIDATES: a blackhole fallback installed by the AttackDetected fast path is
// withdrawn when the later characterized decision exempts the attack from
// mitigation. The characterized decision is authoritative.
// PREVENTS: an upstream blackhole surviving a policy exemption indefinitely. The
// two responders disagreed: local.applyMitigation has always withdrawn here
// ("If the fast path already installed a drop, withdraw it"), flowspec returned
// and left the announce standing, so an allowlisted destination stayed
// blackholed upstream while the operator's policy said not to touch it.
func TestSuppressMitigationWithdrawsBlackholeFallback(t *testing.T) {
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", BlackholeFallback: true,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)

	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17}

	// Fast path: a critical detection installs the blackhole fallback.
	r.onDetected(&ddosevent.AttackDetected{
		Target:    target,
		Direction: ddosevent.DirectionRemote,
		Severity:  ddosevent.SeverityCritical,
	})
	if !r.active {
		t.Fatal("setup: the blackhole fallback must be installed before the exemption arrives")
	}

	// Characterization then exempts the attack from the mitigation action.
	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target:             target,
		Direction:          ddosevent.DirectionRemote,
		SuppressMitigation: true,
	})

	if r.active {
		t.Error("an exempted attack must not keep its blackhole fallback announced")
	}
	var withdrew bool
	for _, c := range disp.cmds {
		if strings.Contains(c, "del") {
			withdrew = true
		}
	}
	if !withdrew {
		t.Errorf("the exemption must dispatch a withdraw; dispatched: %v", disp.cmds)
	}
}

// TestLocalVictimWithdrawsBlackholeFallback covers the second instance of the
// same leak, which a targeted patch to the SuppressMitigation branch alone would
// have left standing.
//
// Detect classifies direction from the raw target prefix; characterization
// re-classifies from the NARROWED victim (detect/characterize.go). A /24 that
// looked remote can narrow to a box-owned /32, so direction flips Remote to
// Local between the two events, after the blackhole fallback is already out.
//
// VALIDATES: an upstream announce is withdrawn once the victim turns out to be
// local, because on-host mitigation owns it from then on.
// PREVENTS: a flowspec rule surviving upstream for a destination this box is
// mitigating itself. The detector states the contract: "the characterized event
// is authoritative -- responders withdraw any drop the fast AttackDetected
// installed".
func TestLocalVictimWithdrawsBlackholeFallback(t *testing.T) {
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", BlackholeFallback: true,
		HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)

	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17}

	r.onDetected(&ddosevent.AttackDetected{
		Target:    target,
		Direction: ddosevent.DirectionRemote,
		Severity:  ddosevent.SeverityCritical,
	})
	if !r.active {
		t.Fatal("setup: the blackhole fallback must be installed before the reclassification")
	}

	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target:    target,
		Direction: ddosevent.DirectionLocal,
	})

	if r.active {
		t.Error("a locally-mitigated victim must not keep an upstream announce")
	}
	var withdrew bool
	for _, c := range disp.cmds {
		if strings.Contains(c, "del") {
			withdrew = true
		}
	}
	if !withdrew {
		t.Errorf("the reclassification must dispatch a withdraw; dispatched: %v", disp.cmds)
	}
}

// TestOngoingDrivesTheLeakProbe wires the probe to its input.
//
// VALIDATES: an AttackOngoing sample advances the probe, and a sample below
// probe-rate eventually withdraws the announce.
// PREVENTS: the state this replaces. onCleared ignores the detector's clear
// "while mitigating (leak-probe decides)", and the probe had no production
// driver at all, so nothing withdrew a flowspec announce once it was out. The
// rule stayed on the wire until an operator removed it.
func TestOngoingDrivesTheLeakProbe(t *testing.T) {
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", Action: "rate-limit", RateLimitBytes: 9600,
		HoldDown: 1, ProbeInterval: 1, ProbeWindow: 1, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)

	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17}
	r.onCharacterized(&ddosevent.AttackCharacterized{Target: target, Direction: ddosevent.DirectionRemote})
	if !r.active {
		t.Fatal("setup: the announce must be out before the probe can lift it")
	}

	// Samples below probe-rate: the attack has stopped behind the filter.
	for i := 0; i < 8 && r.active; i++ {
		r.onOngoing(&ddosevent.AttackOngoing{Target: target, CurrentBps: 0})
	}

	if r.active {
		t.Error("a quiet probe window must withdraw the announce")
	}
}

// TestOngoingAtProbeRateKeepsTheAnnounce is the discriminator for the test
// above: the probe must not lift while traffic is still saturating.
//
// VALIDATES: a sample at or above probe-rate re-tightens instead of clearing.
// PREVENTS: driving the probe with a fabricated zero. Feeding it a constant 0
// would pass the previous test with no traffic source wired at all, and would
// lift mitigation during an active attack.
func TestOngoingAtProbeRateKeepsTheAnnounce(t *testing.T) {
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", Action: "rate-limit", RateLimitBytes: 9600,
		HoldDown: 1, ProbeInterval: 1, ProbeWindow: 1, ProbeRate: 1000, BackoffCap: 3600,
	}, disp)

	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17}
	r.onCharacterized(&ddosevent.AttackCharacterized{Target: target, Direction: ddosevent.DirectionRemote})

	for range 20 {
		r.onOngoing(&ddosevent.AttackOngoing{Target: target, CurrentBps: 5000})
	}

	if !r.active {
		t.Error("the announce must survive while traffic still saturates probe-rate")
	}
}

// TestMaxMitigationDurationWithdraws enforces the cap the YANG has always
// promised.
//
// VALIDATES: an announce older than max-mitigation-duration is withdrawn.
// PREVENTS: the documented default lying. Both ddos plugins parse, default to
// 3600 and range-validate this leaf, and neither ever read it, so every operator
// was promised a one-hour cap that did not exist.
func TestMaxMitigationDurationWithdraws(t *testing.T) {
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", Action: "rate-limit", RateLimitBytes: 9600,
		MaxMitigationDuration: 60,
		HoldDown:              300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)

	base := time.Now()
	r.now = func() time.Time { return base }

	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17}
	r.onCharacterized(&ddosevent.AttackCharacterized{Target: target, Direction: ddosevent.DirectionRemote})
	if !r.active {
		t.Fatal("setup: the announce must be out before the cap can lift it")
	}

	r.now = func() time.Time { return base.Add(59 * time.Second) }
	r.enforceMaxDuration()
	if !r.active {
		t.Error("the cap must not fire before max-mitigation-duration elapses")
	}

	r.now = func() time.Time { return base.Add(61 * time.Second) }
	r.enforceMaxDuration()
	if r.active {
		t.Error("an announce older than max-mitigation-duration must be withdrawn")
	}
}

// TestMaxMitigationDurationZeroIsNoCap pins the documented escape.
//
// VALIDATES: "0 = no cap", as both YANG descriptions state.
// PREVENTS: a zero being read as "expire immediately", which would withdraw
// every announce on the first tick.
func TestMaxMitigationDurationZeroIsNoCap(t *testing.T) {
	disp := &fakeDispatcher{}
	r := newResponder(&Config{
		ResponseLevel: "enforce", Action: "rate-limit", RateLimitBytes: 9600,
		MaxMitigationDuration: 0,
		HoldDown:              300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	}, disp)

	base := time.Now()
	r.now = func() time.Time { return base }

	target := ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17}
	r.onCharacterized(&ddosevent.AttackCharacterized{Target: target, Direction: ddosevent.DirectionRemote})

	r.now = func() time.Time { return base.Add(72 * time.Hour) }
	r.enforceMaxDuration()

	if !r.active {
		t.Error("0 means no cap: the announce must survive any elapsed time")
	}
}
