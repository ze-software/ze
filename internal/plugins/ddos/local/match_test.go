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
