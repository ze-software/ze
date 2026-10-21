// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec component combinations
// RFC: rfc/short/rfc8955.md
// Overview: unmatchable.go — CheckMatchable, the one statement of the rule
// Related: types.go — the component types and operator bits

package flowspec

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// valueComponentTypes are the numeric components whose operator list is judged
// against the values the field it reads can hold. FlowIPProtocol is judged by
// upperLayerProtocols instead, because on IPv6 some values the field holds
// never match.
var valueComponentTypes = [...]FlowComponentType{
	FlowPort, FlowDestPort, FlowSourcePort, FlowICMPType, FlowICMPCode,
	FlowPacketLength, FlowDSCP, FlowFlowLabel,
}

// ipv6ExtensionHeaders are the Next Header values RFC 8200 Section 4 names as
// extension headers. RFC 8200 Section 4: "A full implementation of IPv6
// includes implementation of the following extension headers: Hop-by-Hop
// Options, Fragment, Destination Options, Routing, Authentication,
// Encapsulating Security Payload". Their protocol numbers are 0, 44, 60, 43, 51
// and 50.
var ipv6ExtensionHeaders = protocolsOf(0, 43, 44, 50, 51, 60)

// Fragment states a packet can be in, as the fragment bitmask reads them. An
// unfragmented packet sets none of IsF, FF and LF.
var (
	// transportFragmentStates are the packets that carry their transport
	// header: an unfragmented packet and a first fragment.
	transportFragmentStates = [...]uint64{0, uint64(FlowFragFirstFragment)}
	// packetFragmentStates are every packet: unfragmented, a first fragment, a
	// middle fragment and a last fragment.
	packetFragmentStates = [...]uint64{
		0,
		uint64(FlowFragFirstFragment),
		uint64(FlowFragIsFragment),
		uint64(FlowFragIsFragment | FlowFragLastFragment),
	}
)

// checkComponentValues answers an error wrapping ErrFlowSpecUnmatchable when
// one component's own operator list admits no value a packet can carry.
//
// RFC 8955 Section 4.2: "A specific packet is considered to match the Flow
// Specification when it matches the intersection (AND) of all the components
// present in the Flow Specification." A component no packet matches therefore
// leaves the whole route matching none.
func checkComponentValues(afi AFI, has func(FlowComponentType) bool, matches func(FlowComponentType) []FlowMatch) error {
	if has(FlowIPProtocol) {
		if err := checkUpperLayerValues(afi, matches(FlowIPProtocol)); err != nil {
			return err
		}
	}
	for _, t := range valueComponentTypes {
		if !has(t) {
			continue
		}
		valueMax := componentValueMax(t, afi)
		var scratch [8]valueRange
		if len(admittedRanges(scratch[:0], matches(t), valueMax)) == 0 {
			return fmt.Errorf("%w: the %s match admits no value from 0 to %d, the values its "+
				"field can hold; widen the %s match or remove it",
				ErrFlowSpecUnmatchable, t, valueMax, t)
		}
	}
	if has(FlowTCPFlags) && !tcpFlagsAdmitted(matches(FlowTCPFlags)) {
		return fmt.Errorf("%w: the %s match admits no combination of TCP flags, because its "+
			"terms contradict each other; widen the %s match or remove it",
			ErrFlowSpecUnmatchable, FlowTCPFlags, FlowTCPFlags)
	}
	if has(FlowFragment) && !fragmentAdmitted(afi, matches(FlowFragment), packetFragmentStates[:]) {
		return fmt.Errorf("%w: the %s match admits no packet: an unfragmented packet sets none "+
			"of is-fragment, first-fragment and last-fragment, a first fragment sets "+
			"first-fragment alone, and a later fragment sets is-fragment, with last-fragment "+
			"on the last one; widen the %s match or remove it",
			ErrFlowSpecUnmatchable, FlowFragment, FlowFragment)
	}
	return nil
}

// checkUpperLayerValues answers an error wrapping ErrFlowSpecUnmatchable when
// the protocol operator list admits no protocol a packet can carry.
func checkUpperLayerValues(afi AFI, list []FlowMatch) error {
	if protocolsAdmitted(list).empty() {
		return fmt.Errorf("%w: the %s match admits no value from 0 to 255, the values its "+
			"field can hold; widen the %s match or remove it",
			ErrFlowSpecUnmatchable, FlowIPProtocol, FlowIPProtocol)
	}
	if upperLayerProtocols(afi, list).empty() {
		// RFC 8956 Section 3.3: "This contains a list of {numeric_op, value}
		// pairs that are used to match the first Next Header value octet in
		// IPv6 packets that is not an extension header".
		return fmt.Errorf("%w: the %s match admits only IPv6 extension headers, and RFC 8956 "+
			"Section 3.3 matches the first Next Header that is not one; match the upper-layer "+
			"protocol instead, or remove the %s match",
			ErrFlowSpecUnmatchable, FlowIPProtocol, FlowIPProtocol)
	}
	return nil
}

// upperLayerProtocols answers the protocols a packet can carry that the
// protocol operator list admits: on IPv6, the extension headers are never the
// value the component reads.
func upperLayerProtocols(afi AFI, list []FlowMatch) protocolSet {
	admitted := protocolsAdmitted(list)
	if afi == AFIIPv6 {
		return admitted.andNot(ipv6ExtensionHeaders)
	}
	return admitted
}

// protocolsAdmitted answers the protocol numbers, of the 256 the one-octet
// field holds, that the protocol operator list admits.
//
// RFC 8955 Section 4.2.2.3: "Defines a list of {numeric_op, value} pairs that
// are used to match the IP protocol value octet in IP packets."
func protocolsAdmitted(list []FlowMatch) protocolSet {
	var admitted protocolSet
	for p := range 256 {
		if expressionAdmits(list, uint64(p), false, math.MaxUint64) {
			admitted[p/64] |= 1 << (p % 64)
		}
	}
	return admitted
}

// componentValueMax answers the largest value the packet field a numeric
// component reads can hold. It is asked only about valueComponentTypes.
func componentValueMax(t FlowComponentType, afi AFI) uint64 {
	switch t { //exhaustive:ignore // only the components judged by value have a field width here
	case FlowPort, FlowDestPort, FlowSourcePort:
		// RFC 8955 Section 4.2.2.4: "Defines a list of {numeric_op, value}
		// pairs that match source OR destination TCP/UDP ports". A TCP or
		// UDP port is 16 bits (RFC 9293 Section 3.1, RFC 768).
		return math.MaxUint16
	case FlowICMPType, FlowICMPCode:
		// The ICMP and ICMPv6 type and code are one octet each (RFC 792,
		// RFC 4443 Section 2.1).
		return math.MaxUint8
	case FlowPacketLength:
		// RFC 8955 Section 4.2.2.10: "Defines a list of {numeric_op, value}
		// pairs used to match on the total IP packet length (excluding Layer 2
		// but including IP header)." The IPv4 Total Length is 16 bits (RFC 791
		// Section 3.1). An IPv6 jumbogram carries a 32-bit Payload Length (RFC
		// 2675) beside the header, so no bound is applied to IPv6.
		if afi == AFIIPv6 {
			return math.MaxUint64
		}
		return math.MaxUint16
	case FlowDSCP:
		// RFC 8955 Section 4.2.2.11: "Defines a list of {numeric_op, value}
		// pairs used to match the 6-bit DSCP field" and "The six least
		// significant bits contain the DSCP value." The field holds 0 to 63,
		// so a value above 63 names a DSCP no packet carries.
		return 0x3F
	case FlowFlowLabel:
		// RFC 8956 Section 3.7: "This contains a list of {numeric_op, value}
		// pairs that are used to match the 20-bit Flow Label IPv6 header field".
		return 0xFFFFF
	}
	panic("BUG: componentValueMax asked about a component that is not judged by value")
}

// checkPortValues answers an error wrapping ErrFlowSpecUnmatchable when the
// port, destination-port and source-port components are all present and no
// packet's pair of ports satisfies the three together.
//
// RFC 8955 Section 4.2.2.4: "This component matches if either the destination
// port OR the source port of an IP packet matches the value." A packet with
// ports (d, s) matches the three when port admits d or s, destination-port
// admits d and source-port admits s. checkComponentValues has already proved
// each list admits a value alone, so the three meet exactly when port and
// destination-port admit a common value, or port and source-port do.
func checkPortValues(has func(FlowComponentType) bool, matches func(FlowComponentType) []FlowMatch) error {
	if !has(FlowPort) {
		return nil
	}
	if !has(FlowDestPort) {
		return nil
	}
	if !has(FlowSourcePort) {
		return nil
	}
	var portScratch, targetScratch, sourceScratch [8]valueRange
	port := admittedRanges(portScratch[:0], matches(FlowPort), math.MaxUint16)
	if rangesMeet(port, admittedRanges(targetScratch[:0], matches(FlowDestPort), math.MaxUint16)) {
		return nil
	}
	if rangesMeet(port, admittedRanges(sourceScratch[:0], matches(FlowSourcePort), math.MaxUint16)) {
		return nil
	}
	return fmt.Errorf("%w: the %s match admits no port the %s match or the %s match admits, "+
		"and a packet matches the %s match only through its destination or its source port; "+
		"widen the %s match or remove it",
		ErrFlowSpecUnmatchable, FlowPort, FlowDestPort, FlowSourcePort, FlowPort, FlowPort)
}

// valueRange is the closed interval of values from lo to hi.
type valueRange struct{ lo, hi uint64 }

// admittedRanges appends to dst the values from 0 to valueMax the numeric
// operator list admits, as ranges sorted by lo that neither overlap nor touch.
// The cost is O(n log n) in the terms, so a list a peer sends cannot make the
// check quadratic.
//
// RFC 8955 Section 4.2.1.1: "If unset, the result of the previous {op, value}
// pair is logically ORed with the current one. If set, the operation is a
// logical AND. In the first operator octet of a sequence, it MUST be encoded as
// unset and MUST be treated as always unset on decoding. The AND operator has
// higher priority than OR for the purposes of evaluating logical expressions."
// Each run of AND-joined terms is one group, and the list admits the union of
// what its groups admit.
func admittedRanges(dst []valueRange, list []FlowMatch, valueMax uint64) []valueRange {
	start := len(dst)
	var excludedScratch [8]uint64
	for begin := 0; begin < len(list); {
		end := begin + 1
		for end < len(list) && list[end].And {
			end++
		}
		dst = appendGroupRanges(dst, excludedScratch[:0], list[begin:end], valueMax)
		begin = end
	}
	return mergeRanges(dst, start)
}

// appendGroupRanges appends to dst the values from 0 to valueMax that every
// term of one AND group admits. A group admits one interval with some single
// values removed: lt, gt and eq bound the interval, and lt with gt (not equal)
// removes its value.
func appendGroupRanges(dst []valueRange, excluded []uint64, group []FlowMatch, valueMax uint64) []valueRange {
	lo, hi := uint64(0), valueMax
	for _, m := range group {
		// RFC 8955 Section 4.2.1.1, Table 1: the lt, gt and eq bits each admit
		// their comparison, and the term is the OR of the bits set.
		less := m.Op&FlowOpLess != 0
		greater := m.Op&FlowOpGreater != 0
		equal := m.Op&FlowOpEqual != 0
		if less && greater {
			if !equal {
				excluded = append(excluded, m.Value)
			}
			continue
		}
		termLo, termHi, ok := termRange(m.Value, less, greater, equal)
		if !ok {
			return dst
		}
		lo, hi = max(lo, termLo), min(hi, termHi)
		if lo > hi {
			return dst
		}
	}
	slices.Sort(excluded)
	next := lo
	for _, value := range excluded {
		if value < next {
			continue
		}
		if value > hi {
			break
		}
		if value > next {
			dst = append(dst, valueRange{next, value - 1})
		}
		if value == hi {
			return dst
		}
		next = value + 1
	}
	return append(dst, valueRange{next, hi})
}

// termRange answers the interval one term admits when it has at most one of lt
// and gt, and false when it admits no value.
func termRange(value uint64, less, greater, equal bool) (lo, hi uint64, ok bool) {
	switch {
	case less:
		if equal {
			return 0, value, true
		}
		if value == 0 {
			return 0, 0, false
		}
		return 0, value - 1, true
	case greater:
		if equal {
			return value, math.MaxUint64, true
		}
		if value == math.MaxUint64 {
			return 0, 0, false
		}
		return value + 1, math.MaxUint64, true
	case equal:
		return value, value, true
	}
	// RFC 8955 Section 4.2.1.1, Table 1: lt, gt and eq all clear is "false
	// (independent of the value)".
	return 0, 0, false
}

// mergeRanges sorts dst[start:] by lo and joins the ranges that overlap or
// touch, answering dst cut to the merged ranges.
func mergeRanges(dst []valueRange, start int) []valueRange {
	ranges := dst[start:]
	if len(ranges) == 0 {
		return dst
	}
	slices.SortFunc(ranges, func(a, b valueRange) int { return cmp.Compare(a.lo, b.lo) })
	merged := 0
	for _, r := range ranges[1:] {
		last := &ranges[merged]
		if last.hi == math.MaxUint64 {
			break
		}
		if r.lo <= last.hi+1 {
			last.hi = max(last.hi, r.hi)
			continue
		}
		merged++
		ranges[merged] = r
	}
	return dst[:start+merged+1]
}

// rangesMeet reports whether two sets of sorted, disjoint ranges share a value.
func rangesMeet(a, b []valueRange) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].hi < b[j].lo {
			i++
			continue
		}
		if b[j].hi < a[i].lo {
			j++
			continue
		}
		return true
	}
	return false
}

// tcpFlagsAdmitted reports whether some value of the 16 TCP header bits the
// component reads satisfies the bitmask operator list. The terms can demand
// bits set and bits clear in combinations no shortcut decides, so the list is
// evaluated over every value, 64 at a time: the six low bits vary across the
// bits of one word and the ten high bits across 1024 words. The cost is 1024
// word operations for each term, whatever the terms are.
//
// RFC 8955 Section 4.2.2.9: "Defines a list of {bitmask_op, bitmask} pairs used
// to match TCP control bits (see also Section 3.1 of [RFC0793])."
func tcpFlagsAdmitted(list []FlowMatch) bool {
	for high := range uint64(1 << 10) {
		if expressionWord(list, high) != 0 {
			return true
		}
	}
	return false
}

// expressionWord answers, as one bit for each of the 64 values of the six low
// bits, which values with the ten high bits high satisfy the bitmask operator
// list. It applies the Section 4.2.1.1 AND-over-OR order expressionAdmits does.
func expressionWord(list []FlowMatch, high uint64) uint64 {
	var admitted, group uint64
	for i, m := range list {
		term := termWord(m, high)
		if i == 0 {
			group = term
			continue
		}
		if m.And {
			group &= term
			continue
		}
		admitted |= group
		group = term
	}
	return admitted | group
}

// termWord answers the 64 values of the six low bits, with the ten high bits
// high, that satisfy one {bitmask_op, bitmask} pair.
//
// RFC 8955 Section 4.2.1.2: "If set, this is a bitwise match operation defined
// as "(data AND value) == value"; if unset, (data AND value) evaluates to TRUE
// if any of the bits in the value mask are set in the data." and "not (NOT
// bit): If set, logical negation of operation."
func termWord(m FlowMatch, high uint64) uint64 {
	valueHigh, valueLow := m.Value>>6, m.Value&0x3F
	var hit uint64
	if m.Op&FlowOpMatch != 0 {
		if high&valueHigh == valueHigh {
			hit = lowAllSet[valueLow]
		}
	} else {
		hit = lowAnySet[valueLow]
		if high&valueHigh != 0 {
			hit = math.MaxUint64
		}
	}
	if m.Op&FlowOpNot != 0 {
		return ^hit
	}
	return hit
}

// lowAllSet[v] has bit l set when l&v == v, and lowAnySet[v] has bit l set when
// l&v != 0, for the 64 values l of six bits.
var lowAllSet, lowAnySet = lowBitmaskWords()

func lowBitmaskWords() (allSet, anySet [64]uint64) {
	for value := range uint64(64) {
		for low := range uint64(64) {
			if low&value == value {
				allSet[value] |= 1 << low
			}
			if low&value != 0 {
				anySet[value] |= 1 << low
			}
		}
	}
	return allSet, anySet
}

// fragmentAdmitted reports whether the fragment operator list admits a packet
// in one of states, with or without DF on IPv4.
//
// RFC 8955 Section 4.2.2.12 defines four bits and says of the other four: "0:
// MUST be set to 0 on NLRI encoding and MUST be ignored during decoding". RFC
// 8956 Section 3.6 makes the DF position one of those bits for IPv6, so an IPv6
// route's DF bit is ignored rather than read as a packet that never sets it.
func fragmentAdmitted(afi AFI, list []FlowMatch, states []uint64) bool {
	defined := uint64(FlowFragDontFragment | FlowFragIsFragment | FlowFragFirstFragment | FlowFragLastFragment)
	if afi == AFIIPv6 {
		defined &^= uint64(FlowFragDontFragment)
	}
	for _, state := range states {
		if expressionAdmits(list, state, true, defined) {
			return true
		}
		if afi != AFIIPv6 && expressionAdmits(list, state|uint64(FlowFragDontFragment), true, defined) {
			return true
		}
	}
	return false
}

func (s protocolSet) andNot(o protocolSet) protocolSet {
	return protocolSet{s[0] &^ o[0], s[1] &^ o[1], s[2] &^ o[2], s[3] &^ o[3]}
}
