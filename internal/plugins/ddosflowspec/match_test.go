package ddosflowspec

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
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

func TestAllowlistSubtraction(t *testing.T) {
	// VALIDATES: AC-2 -- allowlisted prefix not announced
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
	}
	allowlist := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
	if shouldAnnounce(v, allowlist) {
		t.Error("should not announce for allowlisted target")
	}
}

func TestAllowlistNoOverlap(t *testing.T) {
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("192.168.1.1/32"),
	}
	allowlist := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if !shouldAnnounce(v, allowlist) {
		t.Error("should announce for non-allowlisted target")
	}
}
