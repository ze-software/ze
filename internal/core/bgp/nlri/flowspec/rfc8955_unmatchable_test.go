package flowspec

import (
	"errors"
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

// matchableCases holds every contradiction class RFC 8955 Sections 4.2.2.4 to
// 4.2.2.9 and RFC 8956 Sections 3.4 and 3.5 name, each beside the closest route
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
}

// TestCheckMatchable checks every contradiction class on a built FlowSpec, on
// the same FlowSpec read back from its wire bytes (the receive path), and on a
// FlowSpec VPN route.
//
// RFC requirement: RFC8955-4.2-3 negative -- a route whose components no packet can satisfy together (ICMP beside a port or TCP flags, a protocol component admitting no protocol a port, ICMP or TCP-flags component needs, a fragment component admitting only non-first fragments beside a transport component) answers ErrFlowSpecUnmatchable, built, read from the wire and as a VPN route (§4.2).
// RFC requirement: RFC8955-4.2-3 positive -- a route a packet can match, including a protocol expression that admits one needed protocol, answers nil (§4.2).
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
			for _, p := range types {
				if p == t {
					return true
				}
			}
			return false
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
		components []FlowComponent
		want       string
	}{
		{[]FlowComponent{NewFlowPortComponent(80), NewFlowICMPTypeComponent(8)},
			"the port match needs TCP or UDP packets and the icmp-type match needs ICMP packets, and ICMP has no ports"},
		{[]FlowComponent{protocolMatches(eqMatch(47)), NewFlowPortComponent(80)},
			"widen the protocol match or remove the port match"},
		{[]FlowComponent{fragmentMatches(FlowMatch{Value: uint64(FlowFragIsFragment)}), NewFlowDestPortComponent(80)},
			"remove the fragment match or the destination-port match"},
	} {
		flow := NewFlowSpec(ipv4FlowSpec)
		for _, c := range tc.components {
			require.NoError(t, flow.AddComponent(c))
		}
		require.ErrorContains(t, flow.CheckMatchable(), tc.want)
	}
}
