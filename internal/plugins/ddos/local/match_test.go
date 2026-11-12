package local

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestBuildDropTermCoversEveryCanonicalProtocol walks the canonical protocol
// table rather than a sample.
//
// VALIDATES: a vector carrying any protocol the firewall backends can enforce
// produces a MatchProtocol for it.
// PREVENTS: this package holding its own protocol table again. The private copy
// knew four names, so a mitigation for an SCTP, ESP, AH, OSPF, VRRP or GRE flood
// silently dropped its protocol condition and programmed a term wider than the
// attack -- a blackhole for every other protocol reaching the victim prefix.
func TestBuildDropTermCoversEveryCanonicalProtocol(t *testing.T) {
	for _, name := range firewall.ProtocolNames() {
		num, ok := firewall.ProtocolNumber(name)
		require.True(t, ok)
		term := buildDropTerm("attack", ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("192.0.2.0/24"),
			Proto:     num,
		})
		assert.Contains(t, term.Matches, firewall.MatchProtocol{Protocol: name},
			"protocol %d (%s) must narrow the mitigation term", num, name)
	}
}

// TestBuildDropTermSkipsUnnamedProtocol pins the deliberate behavior of this
// producer, which differs from the FlowSpec bridge on purpose.
//
// VALIDATES: a protocol with no canonical name contributes no match, leaving
// the other fields of the vector to narrow the term.
// PREVENTS: a MatchProtocol carrying digits, which no backend can lower and
// which would abort the whole firewall reconcile for every owner.
func TestBuildDropTermSkipsUnnamedProtocol(t *testing.T) {
	term := buildDropTerm("attack", ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("192.0.2.0/24"),
		Proto:     253,
		DstPort:   53,
	})
	for _, m := range term.Matches {
		_, isProto := m.(firewall.MatchProtocol)
		assert.False(t, isProto, "an unnamed protocol number must contribute no protocol match")
	}
	assert.Contains(t, term.Matches, firewall.MatchDestinationPort{
		Ranges: []firewall.PortRange{{Lo: 53, Hi: 53}},
	})
}

// TestBuildDropTermPutsTheProtocolFirst pins the order of the term's matches.
// Goal: a mitigation for a vector whose protocol has a name opens with that
// protocol, ahead of the victim address, so the IKE ESP form fast path's
// analyzer reads the lowered rule as one bare ESP never matches
// (espFormExcludesESP, internal/component/ike/dataplane); a vector whose
// protocol has no name opens with the address, and that drop can match ESP to
// the victim. Method: build the term for a UDP, a TCP SYN and an unnamed
// protocol vector and compare the whole match list.
//
// VALIDATES: buildDropTerm's documented order, protocol then address then ports
// then flags.
// PREVENTS: an armed mitigation keeping the ESP form fast path off for every
// Child SA on the box.
// MUTATION: append the destination address ahead of the protocol in
// buildDropTerm; the udp and syn rows go red.
func TestBuildDropTermPutsTheProtocolFirst(t *testing.T) {
	victim := netip.MustParsePrefix("192.0.2.1/32")
	tests := []struct {
		name   string
		vector ddosevent.VectorTuple
		want   []firewall.Match
	}{
		{
			name:   "udp",
			vector: ddosevent.VectorTuple{DstPrefix: victim, Proto: 17, DstPort: 53, SrcPort: 123},
			want: []firewall.Match{
				firewall.MatchProtocol{Protocol: "udp"},
				firewall.MatchDestinationAddress{Prefix: victim},
				firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 53, Hi: 53}}},
				firewall.MatchSourcePort{Ranges: []firewall.PortRange{{Lo: 123, Hi: 123}}},
			},
		},
		{
			name:   "syn",
			vector: ddosevent.VectorTuple{DstPrefix: victim, Proto: 6, TCPFlags: 0x02},
			want: []firewall.Match{
				firewall.MatchProtocol{Protocol: "tcp"},
				firewall.MatchDestinationAddress{Prefix: victim},
				firewall.MatchTCPFlags{Flags: firewall.TCPFlags(0x02), Mask: firewall.TCPFlags(0x02)},
			},
		},
		{
			name:   "unnamed protocol",
			vector: ddosevent.VectorTuple{DstPrefix: victim, Proto: 253, DstPort: 53},
			want: []firewall.Match{
				firewall.MatchDestinationAddress{Prefix: victim},
				firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 53, Hi: 53}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := buildDropTerm("attack", tt.vector)
			assert.Equal(t, tt.want, term.Matches)
		})
	}
}
