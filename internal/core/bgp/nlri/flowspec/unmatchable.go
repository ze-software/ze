// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec component combinations
// RFC: rfc/short/rfc8955.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: vocabulary.go — KeywordComponentType, the keyword each type is written with

package flowspec

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrFlowSpecUnmatchable is the refusal of a flow route no packet can match.
// CheckMatchable wraps it with the components that contradict each other and
// the change that makes the route match.
var ErrFlowSpecUnmatchable = errors.New("flowspec: the flow route matches no packet, " +
	"and RFC 8955 Section 4.2 says such a route SHOULD NOT be propagated")

// IP protocol numbers the transport components depend on.
const (
	protocolICMP   = 1  // RFC 792
	protocolTCP    = 6  // RFC 9293
	protocolUDP    = 17 // RFC 768
	protocolICMPv6 = 58 // RFC 4443
)

// transportTypes are the components that read the transport header, and so
// match only some IP protocols and only a packet that carries that header. The
// order decides which pair a refusal names: the first present one is compared
// with each later one.
var transportTypes = [...]FlowComponentType{
	FlowPort, FlowDestPort, FlowSourcePort, FlowICMPType, FlowICMPCode, FlowTCPFlags,
}

// CheckMatchable answers an error wrapping ErrFlowSpecUnmatchable when no
// packet can match the components a flow route carries, and nil when a packet
// can. It is the one statement of the rule: the FlowSpec methods ask it about
// their components, and the ExaBGP bridge and migration ask it about the words
// of a route before any FlowSpec exists.
//
// has reports whether the route carries a component of a type. matches answers
// the operator list of a component has reports present. A caller that holds the
// component types but not their operator lists passes nil for matches: the
// checks that read the values (each component's own list, the three port
// components together, the protocol and fragment lists beside a transport
// component) are then not made, so nil is the guard that limits such a caller
// to the combinations of types. Ze judges
// the route again, values included, when it builds the FlowSpec.
//
// RFC 8955 Section 4.2: "some combinations cannot match any packets (e.g.,
// "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be
// propagated by BGP."
func CheckMatchable(afi AFI, has func(FlowComponentType) bool, matches func(FlowComponentType) []FlowMatch) error {
	var required protocolSet
	var first FlowComponentType
	for _, t := range transportTypes {
		if !has(t) {
			continue
		}
		need := transportProtocols(t, afi)
		if first == 0 {
			first = t
			required = need
			continue
		}
		narrowed := required.and(need)
		if narrowed.empty() {
			// Two needs fail to meet only when one side is ICMP. transportTypes
			// puts ICMP after the ports and before the TCP flags, so the ICMP
			// side is t against a port and first against the TCP flags.
			icmp, other := need.names(), first
			switch first { //exhaustive:ignore // only an ICMP first changes sides
			case FlowICMPType, FlowICMPCode:
				icmp, other = required.names(), t
			}
			return fmt.Errorf("%w: the %s match needs %s packets and the %s match needs %s "+
				"packets, and %s has no %s; remove one of the two matches",
				ErrFlowSpecUnmatchable, first, required.names(), t, need.names(), icmp, transportField(other))
		}
		required = narrowed
	}
	if matches == nil {
		return nil
	}
	if err := checkComponentValues(afi, has, matches); err != nil {
		return err
	}
	if err := checkPortValues(has, matches); err != nil {
		return err
	}
	if first == 0 {
		return nil
	}

	if has(FlowIPProtocol) && required.and(upperLayerProtocols(afi, matches(FlowIPProtocol))).empty() {
		// RFC 8955 Section 4.2.2.3: "Defines a list of {numeric_op, value}
		// pairs that are used to match the IP protocol value octet in IP
		// packets." RFC 8956 Section 3.3 makes it the first upper-layer Next
		// Header for IPv6.
		return fmt.Errorf("%w: the %s match needs %s packets and the %s match admits none of "+
			"them; widen the %s match or remove the %s match",
			ErrFlowSpecUnmatchable, first, required.names(), FlowIPProtocol, FlowIPProtocol, first)
	}
	// Every transport component quotes the same clause, "never matches ... if
	// the packet is fragmented and this is not the first fragment" (RFC 8955
	// Sections 4.2.2.4, 4.2.2.7, 4.2.2.8, 4.2.2.9; RFC 8956 Sections 3.4, 3.5).
	if has(FlowFragment) && !fragmentAdmitted(afi, matches(FlowFragment), transportFragmentStates[:]) {
		return fmt.Errorf("%w: the %s match reads the transport header, which only an "+
			"unfragmented packet or a first fragment carries, and the %s match admits "+
			"neither; remove the %s match or the %s match",
			ErrFlowSpecUnmatchable, first, FlowFragment, FlowFragment, first)
	}
	return nil
}

// transportProtocols answers the IP protocols a packet MUST carry for the
// transport component t to match it. It is asked only about transportTypes.
func transportProtocols(t FlowComponentType, afi AFI) protocolSet {
	switch t { //exhaustive:ignore // only the transport components restrict the protocol
	case FlowPort, FlowDestPort, FlowSourcePort:
		// RFC 8955 Section 4.2.2.4: "The port component, if present, never
		// matches when the packet's IP protocol value is not 6 (TCP) or 17
		// (UDP), if the packet is fragmented and this is not the first
		// fragment, or if the system is unable to locate the transport header."
		// Sections 4.2.2.5 and 4.2.2.6 refer the destination and source port
		// to the same port definition.
		return protocolsOf(protocolTCP, protocolUDP)
	case FlowICMPType, FlowICMPCode:
		if afi == AFIIPv6 {
			// RFC 8956 Section 3.4: "The ICMPv6 type component, if present,
			// never matches when the packet's upper-layer IP protocol value is
			// not 58 (ICMPv6), if the packet is fragmented and this is not the
			// first fragment, or if the system is unable to locate the
			// transport header." Section 3.5 says the same of the ICMPv6 code.
			return protocolsOf(protocolICMPv6)
		}
		// RFC 8955 Section 4.2.2.7: "The ICMP type component, if present,
		// never matches when the packet's IP protocol value is not 1 (ICMP), if
		// the packet is fragmented and this is not the first fragment, or if
		// the system is unable to locate the transport header." Section
		// 4.2.2.8 says the same of the ICMP code.
		return protocolsOf(protocolICMP)
	case FlowTCPFlags:
		// RFC 8955 Section 4.2.2.9: "The TCP flags component, if present,
		// never matches when the packet's IP protocol value is not 6 (TCP), if
		// the packet is fragmented and this is not the first fragment, or if
		// the system is unable to locate the transport header."
		return protocolsOf(protocolTCP)
	}
	panic("BUG: transportProtocols asked about a component that does not read the transport header")
}

// transportField names what the port or TCP-flags component t reads, for a
// refusal saying ICMP carries no such field.
func transportField(t FlowComponentType) string {
	if t == FlowTCPFlags {
		return "TCP flags"
	}
	return "ports"
}

// expressionAdmits reports whether data satisfies the operator list. bitmask
// selects the bitmask_op reading of Section 4.2.1.2 over the numeric_op one of
// Section 4.2.1.1. valueMask keeps the value bits the component defines, the
// rest being ignored on decoding; a numeric list passes every bit. An empty
// list admits nothing, as an OR of no terms.
//
// RFC 8955 Section 4.2.1.1: "If unset, the result of the previous {op, value}
// pair is logically ORed with the current one. If set, the operation is a
// logical AND. In the first operator octet of a sequence, it MUST be encoded as
// unset and MUST be treated as always unset on decoding. The AND operator has
// higher priority than OR for the purposes of evaluating logical expressions."
func expressionAdmits(list []FlowMatch, data uint64, bitmask bool, valueMask uint64) bool {
	admitted := false
	group := false
	for i, m := range list {
		term := termAdmits(m.Op, m.Value&valueMask, data, bitmask)
		if i == 0 {
			group = term
			continue
		}
		if m.And {
			group = group && term
			continue
		}
		admitted = admitted || group
		group = term
	}
	return admitted || group
}

// termAdmits reports whether data satisfies one {op, value} pair.
func termAdmits(op FlowOperator, value, data uint64, bitmask bool) bool {
	if bitmask {
		// RFC 8955 Section 4.2.1.2: "If set, this is a bitwise match
		// operation defined as "(data AND value) == value"; if unset, (data
		// AND value) evaluates to TRUE if any of the bits in the value mask
		// are set in the data." and "not (NOT bit): If set, logical negation
		// of operation."
		hit := data&value != 0
		if op&FlowOpMatch != 0 {
			hit = data&value == value
		}
		return hit != (op&FlowOpNot != 0)
	}
	// RFC 8955 Section 4.2.1.1, Table 1: the lt, gt and eq bits each admit
	// their comparison, and the term is the OR of the bits set.
	if op&FlowOpLess != 0 && data < value {
		return true
	}
	if op&FlowOpGreater != 0 && data > value {
		return true
	}
	return op&FlowOpEqual != 0 && data == value
}

// protocolSet is a set of IP protocol numbers, one bit for each of the 256
// values the one-octet field can hold.
type protocolSet [4]uint64

// protocolsOf answers the set holding the protocols named.
func protocolsOf(protocols ...uint8) protocolSet {
	var s protocolSet
	for _, p := range protocols {
		s[p/64] |= 1 << (p % 64)
	}
	return s
}

func (s protocolSet) and(o protocolSet) protocolSet {
	return protocolSet{s[0] & o[0], s[1] & o[1], s[2] & o[2], s[3] & o[3]}
}

func (s protocolSet) empty() bool { return s == protocolSet{} }

func (s protocolSet) has(p int) bool { return s[p/64]&(1<<(p%64)) != 0 }

// names answers the set as the operator reads it, "TCP or UDP".
func (s protocolSet) names() string {
	var tb textbuf.Buffer
	for p := range 256 {
		if !s.has(p) {
			continue
		}
		if tb.Len() > 0 {
			tb.Str(" or ")
		}
		tb.Str(protocolName(p))
	}
	return tb.String()
}

// protocolName answers the name of a protocol a transport component needs.
func protocolName(p int) string {
	switch p {
	case protocolICMP:
		return "ICMP"
	case protocolTCP:
		return "TCP"
	case protocolUDP:
		return "UDP"
	case protocolICMPv6:
		return "ICMPv6"
	}
	panic("BUG: protocolName asked about a protocol no transport component needs")
}

// CheckMatchable answers an error wrapping ErrFlowSpecUnmatchable when no
// packet can match this FlowSpec, and nil when one can.
//
// RFC 8955 Section 4.2.
func (f *FlowSpec) CheckMatchable() error {
	return CheckMatchable(f.family.AFI, f.hasComponent, f.componentMatches)
}

// hasComponent reports whether the FlowSpec carries a component of type t.
func (f *FlowSpec) hasComponent(t FlowComponentType) bool {
	return f.componentOfType(t) != nil
}

// componentMatches answers the operator list of the component of type t, which
// CheckMatchable asks only about a numeric or bitmask type the FlowSpec holds.
func (f *FlowSpec) componentMatches(t FlowComponentType) []FlowMatch {
	numeric, ok := f.componentOfType(t).(*numericComponent)
	if !ok {
		panic("BUG: CheckMatchable asked for the operator list of a non-numeric component")
	}
	return numeric.matches
}

// CheckMatchable answers an error wrapping ErrFlowSpecUnmatchable when no
// packet can match this FlowSpec VPN route, and nil when one can.
//
// RFC 8955 Section 4.2.
func (f *FlowSpecVPN) CheckMatchable() error {
	return f.flowSpec.CheckMatchable()
}
