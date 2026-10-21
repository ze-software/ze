// Design: docs/architecture/plugin/rib-storage-design.md — FlowSpec validation on receive
// RFC: rfc/short/rfc8955.md — component combinations (Section 4.2)

package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated checks the receive
// gate refuses the combinations whose contradiction is in the values, or in a
// pair other than ICMP and a port: each such rule is retained but never
// eligible, publishes no install event and yields no candidate, while the
// matchable control stays eligible.
// RFC 8955 Section 4.2: "some combinations cannot match any packets (e.g.,
// "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be
// propagated by BGP."
//
// RFC requirement: RFC8955-4.2-3 positive -- a received rule matching protocol 17 and a port, port 80 with destination port 80 and source port 443, destination port above 100 and below 500, DSCP 46, or first-fragment or is-fragment stays eligible for the forward rails, publishes its install event and yields a candidate.
// RFC requirement: RFC8955-4.2-3 negative -- a received rule matching protocol 47 and a port, protocol 17 and TCP flags, an ICMP type and TCP flags, only non-first fragments and a port, port 80 with destination and source port 443, destination port above 100 and below 50, DSCP above 63, first-fragment and is-fragment together, or TCP flags SYN and not SYN is retained but ineligible for the forward rails, publishes no install event and yields no candidate.
func TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	sequence := flowValidationPath(2, 65001)
	drop := []byte{0x80, 6, 0, 0, 0, 0, 0, 0}
	// Components in ascending type order: destination 10.1/16 (1), protocol
	// (3), port == 80 (4), ICMP type == 8 (7), TCP flags SYN (9), fragment
	// is-fragment (12).
	destination := []byte{1, 16, 10, 1}
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	port := []byte{4, 0x81, 80}
	tcpSYN := []byte{9, 0x80, 0x02}
	for _, tc := range []struct {
		name       string
		components []byte
		want       bool
	}{
		{name: "protocol udp and port match", components: cat(destination, []byte{3, 0x81, 17}, port), want: true},
		{name: "protocol gre and port match nothing", components: cat(destination, []byte{3, 0x81, 47}, port)},
		{name: "protocol udp and TCP flags match nothing", components: cat(destination, []byte{3, 0x81, 17}, tcpSYN)},
		{name: "ICMP type and TCP flags match nothing", components: cat(destination, []byte{7, 0x81, 8}, tcpSYN)},
		{name: "non-first fragments and port match nothing", components: cat(destination, port, []byte{12, 0x80, 0x02})},
		// port == 80, destination-port and source-port == 443 or 80 (2-octet
		// values, operator 0x91).
		{name: "port 80 with destination and source port 443 match nothing", components: cat(destination, port, []byte{5, 0x91, 0x01, 0xBB}, []byte{6, 0x91, 0x01, 0xBB})},
		{name: "port 80 with destination port 80 and source port 443 match", components: cat(destination, port, []byte{5, 0x81, 80}, []byte{6, 0x91, 0x01, 0xBB}), want: true},
		// destination-port > 100 AND < 50 (operators 0x02, then 0xC4).
		{name: "destination port above 100 and below 50 matches nothing", components: cat(destination, []byte{5, 0x02, 100, 0xC4, 50})},
		{name: "destination port above 100 and below 500 matches", components: cat(destination, []byte{5, 0x02, 100, 0xD4, 0x01, 0xF4}), want: true},
		// DSCP > 63 (operator 0x82) against DSCP == 46.
		{name: "DSCP above 63 matches nothing", components: cat(destination, []byte{11, 0x82, 63})},
		{name: "DSCP 46 matches", components: cat(destination, []byte{11, 0x81, 46}), want: true},
		// Fragment match-all of first-fragment and is-fragment (operator
		// 0x81) against match-any of the two (operator 0x80).
		{name: "first fragment that is a later fragment matches nothing", components: cat(destination, []byte{12, 0x81, 0x06})},
		{name: "first fragment or later fragment matches", components: cat(destination, []byte{12, 0x80, 0x06}), want: true},
		// TCP flags SYN AND NOT SYN, match-all (operators 0x01, then 0xC3).
		{name: "TCP flags SYN and not SYN match nothing", components: cat(destination, []byte{9, 0x01, 0x02, 0xC3, 0x02})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, bus := flowValidationFixture(t)
			flowValidationReceive(t, r, peer, 65001, 1, family.IPv4Unicast, []byte{8, 10}, flowValidationAttrs(sequence, netip.Addr{}, 0, nil), false)
			raw := append([]byte{byte(len(tc.components))}, tc.components...)
			flowValidationReceive(t, r, peer, 65001, 2, flowspecFamily, raw, flowValidationAttrs(sequence, netip.Addr{}, 100, drop), false)
			key := ribevents.ValidationRoute{Peer: peer, Family: flowspecFamily, NLRI: string(raw)}
			if got := ribevents.RouteEligible(key, 2); got != tc.want {
				t.Fatalf("eligibility = %v, want %v", got, tc.want)
			}
			if !ribevents.RoutePresent(key) {
				t.Fatal("the received rule was not retained in the Adj-RIB-In")
			}
			if got := len(flowValidationEvents(bus)) == 1; got != tc.want {
				t.Fatalf("install event present = %v, want %v", got, tc.want)
			}
			if got := len(gatherCandidatesHeld(t, r, flowspecFamily, raw, false)) != 0; got != tc.want {
				t.Fatalf("candidate availability = %v, want %v", got, tc.want)
			}
		})
	}
}
