package flowspec

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestBuildFlowspecMatchFromVector(t *testing.T) {
	// VALIDATES: AC-1 -- surgical match from vector tuple
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
		Proto:     17,
		DstPort:   53,
	}
	m := buildMatch(v)
	if m.DstPrefix != v.DstPrefix {
		t.Errorf("DstPrefix: got %v, want %v", m.DstPrefix, v.DstPrefix)
	}
	if m.Proto != 17 {
		t.Errorf("Proto: got %d, want 17", m.Proto)
	}
	if m.DstPort != 53 {
		t.Errorf("DstPort: got %d, want 53", m.DstPort)
	}
}

func TestBuildMatchReflection(t *testing.T) {
	// VALIDATES: AC-1b -- reflection flood keys on source port
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
		Proto:     17,
		SrcPort:   53,
	}
	m := buildMatch(v)
	if m.SrcPort != 53 {
		t.Errorf("SrcPort: got %d, want 53", m.SrcPort)
	}
}

// test-relax: TestAllowlistSubtraction / TestAllowlistNoOverlap removed -- the
// per-responder allowlist (shouldAnnounce) was replaced by the detector's traffic
// policy, delivered via the event's SuppressMitigation flag + Direction gating
// (spec-ddos-direction-allowlist). Coverage moves to detect/policy_test.go and the
// responder SuppressMitigation/Direction tests.
