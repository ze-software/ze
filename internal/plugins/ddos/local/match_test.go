package local

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestBuildTermFromVector(t *testing.T) {
	// VALIDATES: AC-1 -- surgical term from the event vector tuple
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
		Proto:     17,
		DstPort:   53,
	}
	term := buildDropTerm("attack-1", v)
	if term.Name != "attack-1" {
		t.Errorf("Name: got %q, want attack-1", term.Name)
	}
	if !slices.ContainsFunc(term.Matches, func(m firewall.Match) bool {
		if da, ok := m.(firewall.MatchDestinationAddress); ok {
			return da.Prefix == netip.MustParsePrefix("10.0.0.1/32")
		}
		return false
	}) {
		t.Error("missing MatchDestinationAddress for 10.0.0.1/32")
	}
	if !slices.ContainsFunc(term.Matches, func(m firewall.Match) bool {
		if mp, ok := m.(firewall.MatchProtocol); ok {
			return mp.Protocol == "udp"
		}
		return false
	}) {
		t.Error("missing MatchProtocol udp")
	}
	if !slices.ContainsFunc(term.Actions, func(a firewall.Action) bool {
		_, ok := a.(firewall.Drop)
		return ok
	}) {
		t.Error("missing Drop action")
	}
}

func TestLocalTCPFlagsMatch(t *testing.T) {
	// VALIDATES: AC-9 -- a vector carrying TCP flags (SYN) produces a
	// MatchTCPFlags term so the drop matches only SYN packets.
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
		Proto:     6,
		TCPFlags:  0x02, // SYN
	}
	term := buildDropTerm("syn-drop", v)
	if !slices.ContainsFunc(term.Matches, func(m firewall.Match) bool {
		tf, ok := m.(firewall.MatchTCPFlags)
		return ok && tf.Flags == firewall.TCPFlagSYN && tf.Mask == firewall.TCPFlagSYN
	}) {
		t.Error("missing MatchTCPFlags{SYN} for a SYN-flood vector")
	}

	// A vector with no flags must not emit a MatchTCPFlags term.
	noFlags := buildDropTerm("udp-drop", ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17,
	})
	if slices.ContainsFunc(noFlags.Matches, func(m firewall.Match) bool {
		_, ok := m.(firewall.MatchTCPFlags)
		return ok
	}) {
		t.Error("must not emit MatchTCPFlags when TCPFlags is zero")
	}
}

// TestAllowlistSubtraction / TestAllowlistNoOverlap removed -- the
// per-responder allowlist (shouldMitigate) was replaced by the detector's traffic
// policy, which decides exempt-vs-mitigate and delivers it via the event's
// SuppressMitigation flag (spec-ddos-direction-allowlist). Coverage moves to the
// detector policy_test.go and the responder SuppressMitigation tests.
