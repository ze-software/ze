package flowspec

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
)

func withNoopAnnounce() func() {
	origA := announceFunc
	origW := withdrawFunc
	announceFunc = func(_ flowspecMatch, _ string) error { return nil }
	withdrawFunc = func(_ flowspecMatch) error { return nil }
	return func() {
		announceFunc = origA
		withdrawFunc = origW
	}
}

func TestIgnoresDetectorClearWhileMitigating(t *testing.T) {
	// VALIDATES: AC-12 -- AttackCleared does not withdraw while mitigating
	defer withNoopAnnounce()()
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600})
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if !r.active {
		t.Fatal("should be active after detect")
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
	r := newResponder(&Config{ResponseLevel: "alert", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600})
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if r.active {
		t.Error("alert mode should not activate mitigation")
	}
}

func TestEnforceModeAnnounces(t *testing.T) {
	defer withNoopAnnounce()()
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600})
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17, DstPort: 53},
		Family: ddosevent.FamilyUDPFlood,
	})
	if !r.active {
		t.Error("enforce mode should activate mitigation")
	}
}

func TestProbeTickWithdraws(t *testing.T) {
	// VALIDATES: AC-5 -- probe-driven withdraw after sub-rate window
	defer withNoopAnnounce()()
	r := newResponder(&Config{ResponseLevel: "enforce", HoldDown: 2, ProbeInterval: 3, ProbeWindow: 2, ProbeRate: 1000000, BackoffCap: 3600})
	r.onDetected(&ddosevent.AttackDetected{
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

func TestAllowlistedTargetNotAnnounced(t *testing.T) {
	defer withNoopAnnounce()()
	r := newResponder(&Config{
		ResponseLevel: "enforce",
		Allowlist:     []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
		HoldDown:      300, ProbeInterval: 60, ProbeWindow: 10, ProbeRate: 1000000, BackoffCap: 3600,
	})
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if r.active {
		t.Error("allowlisted target should not be mitigated")
	}
}
