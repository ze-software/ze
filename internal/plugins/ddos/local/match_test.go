package local

import (
	"net/netip"
	"slices"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
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

func TestAllowlistSubtraction(t *testing.T) {
	// VALIDATES: AC-2 -- allowlisted prefix produces no term
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
		Proto:     17,
		DstPort:   53,
	}
	allowlist := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
	ok := shouldMitigate(v, allowlist)
	if ok {
		t.Error("should not mitigate an allowlisted target")
	}
}

func TestAllowlistNoOverlap(t *testing.T) {
	v := ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("192.168.1.1/32"),
		Proto:     6,
		DstPort:   80,
	}
	allowlist := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	ok := shouldMitigate(v, allowlist)
	if !ok {
		t.Error("should mitigate a non-allowlisted target")
	}
}
