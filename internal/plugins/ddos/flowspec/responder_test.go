package flowspec

import (
	"net/netip"
	"testing"

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

// test-relax: TestAllowlistedTargetNotAnnounced reworked into
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
