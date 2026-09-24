package flowspecfirewall

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
)

// ICMP type numbers belong to the NLRI's address family, including when no
// explicit protocol component accompanies the type.
func TestICMPTypeUsesFlowSpecFamily(t *testing.T) {
	for _, tc := range []struct {
		family   flowspec.Family
		protocol string
		match    firewall.Match
	}{
		{flowspec.IPv4FlowSpec, "icmp", firewall.MatchICMPType{Type: 8}},
		{flowspec.IPv6FlowSpec, "icmpv6", firewall.MatchICMPv6Type{Type: 8}},
	} {
		fs := flowspec.NewFlowSpec(tc.family)
		if err := fs.AddComponent(flowspec.NewFlowICMPTypeComponent(8)); err != nil {
			t.Fatal(err)
		}
		terms, err := translateFlowSpec(fs, flowAction{discard: true}, "icmp-family")
		if err != nil {
			t.Fatal(err)
		}
		want := []firewall.Match{tc.match, firewall.MatchProtocol{Protocol: tc.protocol}}
		if len(terms) != 1 || !reflect.DeepEqual(terms[0].Matches, want) {
			t.Fatalf("%s: matches = %#v, want %#v", tc.family, terms, want)
		}
	}
}

// A bitmask include is any-bit-set, whereas match requires every named bit.
// Predicates the firewall model cannot express must be refused as a whole.
func TestTCPFlagBitmaskOperators(t *testing.T) {
	for _, tc := range []struct {
		name      string
		op, value byte
		want      firewall.MatchTCPFlags
		refuse    bool
	}{
		{"include-syn", 0, 2, firewall.MatchTCPFlags{Flags: 2, Mask: 2}, false},
		{"match-syn-ack", 1, 18, firewall.MatchTCPFlags{Flags: 18, Mask: 18}, false},
		{"neither-syn-nor-ack", 2, 18, firewall.MatchTCPFlags{Flags: 0, Mask: 18}, false},
		{"not-syn", 3, 2, firewall.MatchTCPFlags{Flags: 0, Mask: 2}, false},
		{"include-syn-or-ack", 0, 18, firewall.MatchTCPFlags{}, true},
		{"not-both-syn-ack", 3, 18, firewall.MatchTCPFlags{}, true},
		{"include-empty", 0, 0, firewall.MatchTCPFlags{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs, err := flowspec.ParseFlowSpec(flowspec.IPv4FlowSpec, []byte{3, 9, 0x80 | tc.op, tc.value})
			if err != nil {
				t.Fatal(err)
			}
			terms, err := translateFlowSpec(fs, flowAction{discard: true}, tc.name)
			if tc.refuse {
				if err == nil || len(terms) != 0 {
					t.Fatalf("unrepresentable bitmask installed: terms=%#v, error=%v", terms, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := []firewall.Match{tc.want, firewall.MatchProtocol{Protocol: "tcp"}}
			if len(terms) != 1 || !reflect.DeepEqual(terms[0].Matches, want) {
				t.Fatalf("matches = %#v, want %#v", terms, want)
			}
		})
	}
}
