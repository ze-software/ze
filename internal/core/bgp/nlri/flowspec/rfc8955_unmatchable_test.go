package flowspec

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// matchableCase is one flow route and whether a packet can match it.
type matchableCase struct {
	name        string
	family      Family
	components  []FlowComponent
	unmatchable bool
}

func eqMatch(v uint64) FlowMatch { return FlowMatch{Op: FlowOpEqual, Value: v} }

func protocolMatches(list ...FlowMatch) FlowComponent {
	return NewFlowNumericComponent(FlowIPProtocol, list)
}

func fragmentMatches(list ...FlowMatch) FlowComponent {
	return NewFlowFragmentMatchComponent(list)
}

// matchableCases holds every contradiction class RFC 8955 Sections 4.2.2.3 to
// 4.2.2.12 and RFC 8956 Sections 3.3 to 3.7 name, each beside the closest route
// a packet can still match, so a check that refuses too much goes red too.
var matchableCases = []matchableCase{
	// Two transport components that need disjoint protocols.
	{"icmp-type with port", ipv4FlowSpec, []FlowComponent{NewFlowICMPTypeComponent(8), NewFlowPortComponent(80)}, true},
	{"icmp-code with destination-port", ipv4FlowSpec, []FlowComponent{NewFlowICMPCodeComponent(0), NewFlowDestPortComponent(80)}, true},
	{"icmp-type with tcp-flags", ipv4FlowSpec, []FlowComponent{NewFlowICMPTypeComponent(8), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},
	{"icmp-code with tcp-flags", ipv4FlowSpec, []FlowComponent{NewFlowICMPCodeComponent(0), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},
	{"tcp-flags with source-port", ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}}), NewFlowSourcePortComponent(53)}, false},
	{"ipv6 icmp-type with port", ipv6FlowSpec, []FlowComponent{NewFlowICMPTypeComponent(128), NewFlowPortComponent(80)}, true},
	{"ipv6 icmp-code with tcp-flags", ipv6FlowSpec, []FlowComponent{NewFlowICMPCodeComponent(0), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},

	// A protocol component beside a transport component.
	{"protocol udp with destination-port", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(17)), NewFlowDestPortComponent(53)}, false},
	{"protocol icmp with destination-port", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(1)), NewFlowDestPortComponent(53)}, true},
	{"protocol gre with port", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(47)), NewFlowPortComponent(80)}, true},
	{"protocol udp with tcp-flags", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(17)), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},
	{"protocol tcp with tcp-flags", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(6)), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, false},
	{"protocol tcp with icmp-type", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(6)), NewFlowICMPTypeComponent(8)}, true},
	{"protocol icmp with icmp-code", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(1)), NewFlowICMPCodeComponent(0)}, false},
	{"ipv6 next-header icmpv6 with icmp-type", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(58)), NewFlowICMPTypeComponent(128)}, false},
	{"ipv6 next-header icmp with icmp-type", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(1)), NewFlowICMPTypeComponent(128)}, true},
	{"protocol udp with port and tcp-flags", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(17)), NewFlowPortComponent(80), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},
	{"protocol gre alone", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(47))}, false},

	// Protocol operator expressions: the route is unmatchable only when no
	// value the expression admits is a protocol the transport component needs.
	{"protocol >=6 and <=17 with port", ipv4FlowSpec, []FlowComponent{protocolMatches(
		FlowMatch{Op: FlowOpGreater | FlowOpEqual, Value: 6},
		FlowMatch{Op: FlowOpLess | FlowOpEqual, Value: 17, And: true}), NewFlowPortComponent(80)}, false},
	{"protocol >6 and <17 with port", ipv4FlowSpec, []FlowComponent{protocolMatches(
		FlowMatch{Op: FlowOpGreater, Value: 6},
		FlowMatch{Op: FlowOpLess, Value: 17, And: true}), NewFlowPortComponent(80)}, true},
	{"protocol icmp or udp with port", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(1), eqMatch(17)), NewFlowPortComponent(80)}, false},
	{"protocol !=6 with tcp-flags", ipv4FlowSpec, []FlowComponent{protocolMatches(
		FlowMatch{Op: FlowOpNotEq, Value: 6}), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},
	{"protocol !=6 with port", ipv4FlowSpec, []FlowComponent{protocolMatches(FlowMatch{Op: FlowOpNotEq, Value: 6}), NewFlowPortComponent(80)}, false},
	{"protocol >17 with port", ipv4FlowSpec, []FlowComponent{protocolMatches(FlowMatch{Op: FlowOpGreater, Value: 17}), NewFlowPortComponent(80)}, true},
	{"protocol icmp or >=6 and <=6 with tcp-flags", ipv4FlowSpec, []FlowComponent{protocolMatches(
		eqMatch(1),
		FlowMatch{Op: FlowOpGreater | FlowOpEqual, Value: 6},
		FlowMatch{Op: FlowOpLess | FlowOpEqual, Value: 6, And: true}), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, false},

	// A fragment component that admits no packet carrying a transport header.
	{"fragment is-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)}), NewFlowPortComponent(80)}, true},
	{"fragment last-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragLastFragment)}), NewFlowPortComponent(80)}, true},
	{"fragment first-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragFirstFragment)}), NewFlowPortComponent(80)}, false},
	{"fragment not is-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Op: FlowOpNot, Value: uint64(FlowFragIsFragment)}), NewFlowPortComponent(80)}, false},
	{"fragment dont-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragDontFragment)}), NewFlowPortComponent(80)}, false},
	{"fragment is-fragment with icmp-type", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)}), NewFlowICMPTypeComponent(8)}, true},
	{"fragment is-fragment with tcp-flags", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)}), NewFlowTCPFlagsMatchComponent([]FlowMatch{{Value: 0x02}})}, true},
	{"ipv6 fragment is-fragment with icmp-code", ipv6FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)}), NewFlowICMPCodeComponent(0)}, true},
	{"fragment is-fragment or first-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Value: uint64(FlowFragIsFragment)}, FlowMatch{Value: uint64(FlowFragFirstFragment)}), NewFlowPortComponent(80)}, false},
	{"fragment dont-fragment and is-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragDontFragment | FlowFragIsFragment)}), NewFlowPortComponent(80)}, true},
	{"fragment is-fragment alone", ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)})}, false},
	{"ipv6 fragment match dont-fragment with port", ipv6FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragDontFragment)}), NewFlowPortComponent(80)}, false},
	{"fragment match reserved bit and first-fragment with port", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: 0x10 | uint64(FlowFragFirstFragment)}), NewFlowPortComponent(80)}, false},

	// The port component beside destination-port and source-port: a packet
	// matches port through its destination or its source port.
	{"port 80 with destination-port 443 and source-port 443", ipv4FlowSpec, []FlowComponent{
		NewFlowPortComponent(80), NewFlowDestPortComponent(443), NewFlowSourcePortComponent(443)}, true},
	{"port 80 with destination-port 80 and source-port 443", ipv4FlowSpec, []FlowComponent{
		NewFlowPortComponent(80), NewFlowDestPortComponent(80), NewFlowSourcePortComponent(443)}, false},
	{"port 80 with destination-port 443 and source-port 80", ipv4FlowSpec, []FlowComponent{
		NewFlowPortComponent(80), NewFlowDestPortComponent(443), NewFlowSourcePortComponent(80)}, false},
	{"port 80 with destination-port 443", ipv4FlowSpec, []FlowComponent{
		NewFlowPortComponent(80), NewFlowDestPortComponent(443)}, false},
	{"port >1000 with destination-port <=1000 and source-port 1000 or 1001", ipv4FlowSpec, []FlowComponent{
		NewFlowNumericComponent(FlowPort, []FlowMatch{{Op: FlowOpGreater, Value: 1000}}),
		NewFlowNumericComponent(FlowDestPort, []FlowMatch{{Op: FlowOpLess | FlowOpEqual, Value: 1000}}),
		NewFlowNumericComponent(FlowSourcePort, []FlowMatch{eqMatch(1000), eqMatch(1001)})}, false},
	{"port >1000 with destination-port <=1000 and source-port <=1000", ipv4FlowSpec, []FlowComponent{
		NewFlowNumericComponent(FlowPort, []FlowMatch{{Op: FlowOpGreater, Value: 1000}}),
		NewFlowNumericComponent(FlowDestPort, []FlowMatch{{Op: FlowOpLess | FlowOpEqual, Value: 1000}}),
		NewFlowNumericComponent(FlowSourcePort, []FlowMatch{{Op: FlowOpLess | FlowOpEqual, Value: 1000}})}, true},

	// One component whose own operator list admits no value its field holds.
	{"destination-port >100 and <50", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowDestPort, []FlowMatch{
		{Op: FlowOpGreater, Value: 100}, {Op: FlowOpLess, Value: 50, And: true}})}, true},
	{"destination-port >100 and <500", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowDestPort, []FlowMatch{
		{Op: FlowOpGreater, Value: 100}, {Op: FlowOpLess, Value: 500, And: true}})}, false},
	{"destination-port >100 and <50 or 80", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowDestPort, []FlowMatch{
		{Op: FlowOpGreater, Value: 100}, {Op: FlowOpLess, Value: 50, And: true}, eqMatch(80)})}, false},
	{"port 80 and !=80", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowPort, []FlowMatch{
		eqMatch(80), {Op: FlowOpNotEq, Value: 80, And: true}})}, true},
	{"port >=80 and <=81 and !=80 and !=81", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowPort, []FlowMatch{
		{Op: FlowOpGreater | FlowOpEqual, Value: 80}, {Op: FlowOpLess | FlowOpEqual, Value: 81, And: true},
		{Op: FlowOpNotEq, Value: 80, And: true}, {Op: FlowOpNotEq, Value: 81, And: true}})}, true},
	{"port >=80 and <=82 and !=80 and !=81", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowPort, []FlowMatch{
		{Op: FlowOpGreater | FlowOpEqual, Value: 80}, {Op: FlowOpLess | FlowOpEqual, Value: 82, And: true},
		{Op: FlowOpNotEq, Value: 80, And: true}, {Op: FlowOpNotEq, Value: 81, And: true}})}, false},
	{"source-port >65535", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowSourcePort, []FlowMatch{{Op: FlowOpGreater, Value: 65535}})}, true},
	{"source-port >=65535", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowSourcePort, []FlowMatch{{Op: FlowOpGreater | FlowOpEqual, Value: 65535}})}, false},
	{"icmp-type >255", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowICMPType, []FlowMatch{{Op: FlowOpGreater, Value: 255}})}, true},
	{"icmp-code <1", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowICMPCode, []FlowMatch{{Op: FlowOpLess, Value: 1}})}, false},
	{"icmp-code <0", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowICMPCode, []FlowMatch{{Op: FlowOpLess, Value: 0}})}, true},
	{"packet-length >65535", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowPacketLength, []FlowMatch{{Op: FlowOpGreater, Value: 65535}})}, true},
	{"ipv6 packet-length >65535", ipv6FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowPacketLength, []FlowMatch{{Op: FlowOpGreater, Value: 65535}})}, false},
	{"dscp >63", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowDSCP, []FlowMatch{{Op: FlowOpGreater, Value: 63}})}, true},
	{"dscp 64", ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowDSCP, []FlowMatch{eqMatch(64)})}, true},
	{"dscp 63", ipv4FlowSpec, []FlowComponent{NewFlowDSCPComponent(63)}, false},
	{"ipv6 flow-label >1048575", ipv6FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowFlowLabel, []FlowMatch{{Op: FlowOpGreater, Value: 0xFFFFF}})}, true},
	{"ipv6 flow-label 1048575", ipv6FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowFlowLabel, []FlowMatch{eqMatch(0xFFFFF)})}, false},
	{"protocol >255", ipv4FlowSpec, []FlowComponent{protocolMatches(FlowMatch{Op: FlowOpGreater, Value: 255})}, true},
	{"protocol >=255", ipv4FlowSpec, []FlowComponent{protocolMatches(FlowMatch{Op: FlowOpGreater | FlowOpEqual, Value: 255})}, false},
	{"protocol false", ipv4FlowSpec, []FlowComponent{protocolMatches(FlowMatch{Value: 6})}, true},

	// An IPv6 next-header match names the first upper-layer header, never an
	// extension header (RFC 8956 Section 3.3).
	{"ipv6 next-header hop-by-hop", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(0))}, true},
	{"ipv6 next-header esp", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(50))}, true},
	{"ipv6 next-header fragment or routing", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(44), eqMatch(43))}, true},
	{"ipv6 next-header fragment or tcp", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(44), eqMatch(6))}, false},
	{"ipv6 next-header mobility", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(135))}, false},
	{"ipv4 protocol esp", ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(50))}, false},
	{"ipv6 next-header authentication with port", ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(51)), NewFlowPortComponent(80)}, true},

	// A bitmask list whose terms contradict each other.
	{"tcp-flags syn and not syn", ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{
		{Op: FlowOpMatch, Value: 0x02}, {Op: FlowOpMatch | FlowOpNot, Value: 0x02, And: true}})}, true},
	{"tcp-flags syn and not ack", ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{
		{Op: FlowOpMatch, Value: 0x02}, {Op: FlowOpMatch | FlowOpNot, Value: 0x10, And: true}})}, false},
	{"tcp-flags any of syn or ack and none of syn or ack", ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{
		{Value: 0x12}, {Op: FlowOpNot, Value: 0x12, And: true}})}, true},
	{"tcp-flags high bit and not high bit", ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{
		{Op: FlowOpMatch, Value: 0x0100}, {Op: FlowOpMatch | FlowOpNot, Value: 0x0100, And: true}})}, true},
	{"tcp-flags high bit and syn", ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{
		{Op: FlowOpMatch, Value: 0x0100}, {Op: FlowOpMatch, Value: 0x02, And: true}})}, false},
	{"fragment first-fragment and is-fragment", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragFirstFragment | FlowFragIsFragment)})}, true},
	{"fragment first-fragment or is-fragment", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Value: uint64(FlowFragFirstFragment | FlowFragIsFragment)})}, false},
	{"fragment first-fragment and last-fragment", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragFirstFragment | FlowFragLastFragment)})}, true},
	{"fragment last-fragment and not is-fragment", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragLastFragment)},
		FlowMatch{Op: FlowOpMatch | FlowOpNot, Value: uint64(FlowFragIsFragment), And: true})}, true},
	{"fragment last-fragment and is-fragment", ipv4FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragLastFragment | FlowFragIsFragment)})}, false},
	{"ipv6 fragment first-fragment and is-fragment", ipv6FlowSpec, []FlowComponent{fragmentMatches(
		FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragFirstFragment | FlowFragIsFragment)})}, true},
}

// TestCheckMatchable checks every contradiction class on a built FlowSpec, on
// the same FlowSpec read back from its wire bytes (the receive path), and on a
// FlowSpec VPN route.
//
// RFC requirement: RFC8955-4.2-3 negative -- a route whose components no packet can satisfy together (ICMP beside a port or TCP flags, a protocol component admitting no protocol a port, ICMP or TCP-flags component needs, a fragment component admitting only non-first fragments beside a transport component, a port component admitting no value the destination-port or the source-port component beside it admits) or one component whose own list admits no value its field holds (a numeric AND group that excludes every value, a port, ICMP, IPv4 packet-length, DSCP, flow-label or protocol value above the field width, an IPv6 next-header admitting only extension headers, TCP flags or fragment bitmask terms that contradict each other) answers ErrFlowSpecUnmatchable, built, read from the wire and as a VPN route (§4.2).
// RFC requirement: RFC8955-4.2-3 positive -- a route a packet can match, including a protocol expression that admits one needed protocol, a port component that admits the destination or the source port beside it, a list one OR term of which admits a value, and a fragment bit the RFC says is ignored on decoding, answers nil (§4.2).
func TestCheckMatchable(t *testing.T) {
	for _, tc := range matchableCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := NewFlowSpec(tc.family)
			vpnFamily := Family{AFI: tc.family.AFI, SAFI: SAFIFlowSpecVPN}
			vpn := NewFlowSpecVPN(vpnFamily, RouteDistinguisher{})
			for _, c := range tc.components {
				require.NoError(t, flow.AddComponent(c))
				require.NoError(t, vpn.AddComponent(c))
			}
			parsed, err := ParseFlowSpec(tc.family, flow.Bytes())
			require.NoError(t, err)
			parsedVPN, err := ParseFlowSpecVPN(vpnFamily, vpn.Bytes())
			require.NoError(t, err)

			for name, got := range map[string]error{
				"built":         flow.CheckMatchable(),
				"wire":          parsed.CheckMatchable(),
				"vpn":           vpn.CheckMatchable(),
				"vpn from wire": parsedVPN.CheckMatchable(),
			} {
				if errors.Is(got, ErrFlowSpecUnmatchable) != tc.unmatchable {
					t.Errorf("%s: CheckMatchable() = %v, want unmatchable %v", name, got, tc.unmatchable)
				}
			}
		})
	}
}

// TestCheckMatchableTypesOnly checks a caller that holds the component types
// and not their operator lists (the ExaBGP bridge and migration): the
// combinations of types are refused, and the value checks are not made.
func TestCheckMatchableTypesOnly(t *testing.T) {
	present := func(types ...FlowComponentType) func(FlowComponentType) bool {
		return func(t FlowComponentType) bool {
			return slices.Contains(types, t)
		}
	}
	err := CheckMatchable(AFIIPv4, present(FlowICMPType, FlowTCPFlags), nil)
	require.ErrorIs(t, err, ErrFlowSpecUnmatchable)
	require.ErrorContains(t, err, "the tcp-flags match needs TCP packets, and ICMP has no TCP flags")
	require.NoError(t, CheckMatchable(AFIIPv4, present(FlowIPProtocol, FlowPort), nil))
}

// TestCheckMatchableNamesTheFix checks each refusal names the contradiction and
// the change that makes the route match.
func TestCheckMatchableNamesTheFix(t *testing.T) {
	for _, tc := range []struct {
		family     Family
		components []FlowComponent
		want       string
	}{
		{ipv4FlowSpec, []FlowComponent{NewFlowPortComponent(80), NewFlowICMPTypeComponent(8)},
			"the port match needs TCP or UDP packets and the icmp-type match needs ICMP packets, and ICMP has no ports"},
		{ipv4FlowSpec, []FlowComponent{protocolMatches(eqMatch(47)), NewFlowPortComponent(80)},
			"widen the protocol match or remove the port match"},
		{ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)}), NewFlowDestPortComponent(80)},
			"remove the fragment match or the destination-port match"},
		{ipv4FlowSpec, []FlowComponent{NewFlowPortComponent(80), NewFlowDestPortComponent(443), NewFlowSourcePortComponent(443)},
			"the port match admits no port the destination-port match or the source-port match admits"},
		{ipv4FlowSpec, []FlowComponent{NewFlowNumericComponent(FlowDSCP, []FlowMatch{eqMatch(64)})},
			"the dscp match admits no value from 0 to 63"},
		{ipv6FlowSpec, []FlowComponent{protocolMatches(eqMatch(0))},
			"admits only IPv6 extension headers"},
		{ipv4FlowSpec, []FlowComponent{NewFlowTCPFlagsMatchComponent([]FlowMatch{
			{Op: FlowOpMatch, Value: 0x02}, {Op: FlowOpMatch | FlowOpNot, Value: 0x02, And: true}})},
			"the tcp-flags match admits no combination of TCP flags"},
		{ipv4FlowSpec, []FlowComponent{fragmentMatches(FlowMatch{Op: FlowOpMatch, Value: uint64(FlowFragFirstFragment | FlowFragIsFragment)})},
			"the fragment match admits no packet"},
	} {
		flow := NewFlowSpec(tc.family)
		for _, c := range tc.components {
			require.NoError(t, flow.AddComponent(c))
		}
		require.ErrorContains(t, flow.CheckMatchable(), tc.want)
	}
}
