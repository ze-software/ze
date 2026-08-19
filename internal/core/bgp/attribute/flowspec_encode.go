// Design: docs/architecture/wire/attributes.md -- FlowSpec traffic-action extended communities
// RFC: rfc/short/rfc8955.md -- traffic filtering actions (Section 7)
// Related: flowspec_action.go -- the colon-less action keywords, the other half of this vocabulary
// Related: extcomm_decoded.go -- the decode half, rendering these values back to their names

package attribute

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
)

// The FlowSpec Traffic Filtering Action extended communities of RFC 8955
// Section 7, built here and nowhere else.
//
// Three encoders used to hold their own copy of this vocabulary -- the config
// parser, the `update text` API parser, and the FlowSpec NLRI plugin -- and they
// disagreed. The config parser discarded the DSCP range error and wrote the
// operator's overflow into the Section 7.5 reserved bits; the plugin dropped
// `mark 0` and `rate-limit 0` because it read a zero value as "no action asked
// for"; the API parser refused a 4-octet AS in a redirect the config parser
// accepted; and none of the three implemented the type 0x81 redirect Section 7.4
// names. Every one of those is one copy being wrong where another was right,
// which is what a second copy of a vocabulary buys.
//
// So the vocabulary lives here, beside `flowSpecActionKeywords`, which was moved
// for the same reason and says so in its own comment.
const (
	// extCommTypeTransitiveFlowSpec is the high-order type octet every
	// Traffic Filtering Action shares: transitive, experimental use
	// (RFC 8955 Section 7, "All Traffic Filtering Actions are specified as
	// transitive BGP Extended Communities").
	extCommTypeTransitiveFlowSpec = 0x80

	extCommSubtypeTrafficAction  = 0x07 // RFC 8955 Section 7.3
	extCommSubtypeRedirect       = 0x08 // RFC 8955 Section 7.4
	extCommSubtypeTrafficMarking = 0x09 // RFC 8955 Section 7.5
)

// FlowSpecRateUnit names the traffic-rate sub-type, which is what decides
// whether the float that follows counts bytes per second or packets per second.
//
// The zero value names no sub-type, and FlowSpecTrafficRate refuses it, so a
// caller that forgets to say which rate it means is refused rather than handed
// one of them.
type FlowSpecRateUnit byte

const (
	// FlowSpecRateBytes is traffic-rate-bytes, RFC 8955 Section 7.1.
	FlowSpecRateBytes FlowSpecRateUnit = 0x06
	// FlowSpecRatePackets is traffic-rate-packets, RFC 8955 Section 7.2.
	FlowSpecRatePackets FlowSpecRateUnit = 0x0c
)

// flowSpecTrafficActionFlags maps each accepted action word to the bits it sets
// in the final octet of the Traffic Action Field.
//
// RFC 8955 Section 7.3 defines two bits and reserves the rest. The words are the
// ones the decoder prints (extcomm_decoded.go appendExtCommTrafficAction), so a
// community reads back in the spelling an operator may write.
var flowSpecTrafficActionFlags = map[string]byte{
	"terminal": 0x01, // T, bit 47
	"sample":   0x02, // S, bit 46
	// The decoder prints "none" when neither bit is set. Accepting it here keeps
	// a rendered community re-configurable; it sets no bit, which is what it says.
	"none": 0x00,
}

var (
	errFlowSpecRateUnit    = errors.New("flowspec traffic-rate needs a unit: bytes or packets")
	errFlowSpecRateNegSign = errors.New("flowspec traffic-rate must not be negative")
	errFlowSpecRateFinite  = errors.New("flowspec traffic-rate must be a finite number")
)

// FlowSpecTrafficRate returns the traffic-rate-bytes (sub-type 0x06) or
// traffic-rate-packets (sub-type 0x0c) extended community.
//
// Wire format, RFC 8955 Sections 7.1 and 7.2:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  0x80  | 0x06/0x0c |          2-octet id           | rate ...  |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| ... rate (IEEE 754 single, octets 4..7)                        |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octets 2 and 3 carry the 2-octet id, which Section 7.1 calls "purely
// informational". Octets 4 to 7 carry the rate.
//
// Two values are refused. RFC 8955 Section 7.1 states "On encoding, the
// traffic-rate MUST NOT be negative", and Section 7.2 repeats it for
// traffic-rate-packets. A non-finite rate is refused as well: the RFC gives an
// operator a rate or the zero that means discard, a NaN is neither, and Ze's own
// decoder reads a NaN back as zero (extcomm_decoded.go
// appendExtCommTrafficRate). Encoding one would turn a rate limit into a
// blackhole in silence, so it is refused at the point the operator wrote it.
func FlowSpecTrafficRate(unit FlowSpecRateUnit, asn uint16, rate float64) (ExtendedCommunity, error) {
	if unit != FlowSpecRateBytes && unit != FlowSpecRatePackets {
		return ExtendedCommunity{}, errFlowSpecRateUnit
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) {
		return ExtendedCommunity{}, fmt.Errorf("%w: %v", errFlowSpecRateFinite, rate)
	}
	// RFC 8955 Section 7.1: "On encoding, the traffic-rate MUST NOT be negative."
	// RFC 8955 Section 7.2: "On encoding, the traffic-rate-packets MUST NOT be negative."
	if rate < 0 {
		return ExtendedCommunity{}, fmt.Errorf("%w: %v", errFlowSpecRateNegSign, rate)
	}

	bits := math.Float32bits(float32(rate))
	return ExtendedCommunity{
		extCommTypeTransitiveFlowSpec, byte(unit),
		byte(asn >> 8), byte(asn),
		byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits),
	}, nil
}

// FlowSpecTrafficAction returns the traffic-action extended community for a
// hyphen-joined list of action words: "sample", "terminal", "sample-terminal",
// or "none".
//
// Wire format, RFC 8955 Section 7.3:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  0x80  |   0x07    |     Traffic Action Field (octets 2..6)    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Tr. Action Field (cont., octet 7)                        |S|T| |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Only bits 46 (S) and 47 (T) are defined. RFC 8955 Section 7.3: the other bits
// "MUST be set to 0 on encoding and MUST be ignored during decoding", so octets
// 2 to 6 are written as literal zero and octet 7 carries nothing else.
//
// A word outside the table is an error. Accepting one used to return the
// community with both bits clear, which is a traffic-action the operator never
// asked for and which a typo in `terminal` produced in silence.
func FlowSpecTrafficAction(words string) (ExtendedCommunity, error) {
	var flags byte
	for word := range strings.SplitSeq(strings.ToLower(words), "-") {
		bit, ok := flowSpecTrafficActionFlags[word]
		if !ok {
			return ExtendedCommunity{}, fmt.Errorf(
				"unknown flowspec traffic-action %q: expected one of %v, joined with '-'",
				word, flowSpecTrafficActionWords())
		}
		flags |= bit
	}

	// RFC 8955 Section 7.3: every bit but S and T "MUST be set to 0 on encoding".
	return ExtendedCommunity{
		extCommTypeTransitiveFlowSpec, extCommSubtypeTrafficAction,
		0x00, 0x00, 0x00, 0x00, 0x00, flags,
	}, nil
}

// flowSpecTrafficActionWords returns the accepted action words, sorted, so the
// error above names what IS accepted rather than only what was not. Derived from
// the table, so a word cannot be added without the diagnostic naming it
// (ai/rules/evidence.md).
func flowSpecTrafficActionWords() []string {
	words := make([]string, 0, len(flowSpecTrafficActionFlags))
	for word := range flowSpecTrafficActionFlags {
		words = append(words, word)
	}
	sortStrings(words)
	return words
}

// FlowSpecDSCPMax is the largest DSCP a traffic-marking community can carry.
// RFC 8955 Section 7.5 puts the DSCP in "the 6 least significant bits" of the
// value, and RFC 2474 Section 3 gives the field the same six bits.
//
// Exported so a parser can refuse an out-of-range DSCP where the operator wrote
// it, and the encoder can refuse it again on the way to the wire, without either
// of them holding its own copy of the number.
const FlowSpecDSCPMax = 63

// FlowSpecTrafficMarking returns the traffic-marking extended community for a
// DSCP value.
//
// Wire format, RFC 8955 Section 7.5:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  0x80  |   0x09    |   reserved    |   reserved    | reserved  |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   reserved    | r.|    DSCP   |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// A DSCP above 63 is refused rather than truncated. Its two high bits land in
// the "r." field of the figure, and RFC 8955 Section 7.5 states "reserved (r):
// MUST be set to 0 on encoding and MUST be ignored during decoding". Truncating
// instead would install a marking the operator did not ask for: `mark 200` is
// DSCP 8 with both reserved bits set.
//
// The argument is a uint64 so the bound lives here rather than in whichever
// bit size each caller passed to strconv.
func FlowSpecTrafficMarking(dscp uint64) (ExtendedCommunity, error) {
	if dscp > FlowSpecDSCPMax {
		return ExtendedCommunity{}, fmt.Errorf("flowspec DSCP must be 0-%d, got %d", FlowSpecDSCPMax, dscp)
	}

	// RFC 8955 Section 7.5: "reserved (r): MUST be set to 0 on encoding".
	return ExtendedCommunity{
		extCommTypeTransitiveFlowSpec, extCommSubtypeTrafficMarking,
		0x00, 0x00, 0x00, 0x00, 0x00, byte(dscp),
	}, nil
}

// FlowSpecRedirect returns the rt-redirect extended community for a
// route-target written as a global administrator and a local administrator.
//
// RFC 8955 Section 7.4 gives the action three encodings and names the source of
// each: "It uses the same encoding as the Route Target Extended Community in
// Sections 3.1 (type 0x80: 2-octet AS, 4-octet value), 3.2 (type 0x81: 4-octet
// IPv4 address, 2-octet value), and 4 of [RFC4360] and Section 2 of [RFC5668]
// (type 0x82: 4-octet AS, 2-octet value) with the high-order octet of the Type
// field 0x80, 0x81, 0x82 respectively and the low-order octet of the Type field
// (Sub-Type) always 0x08."
//
// So the administrator decides the type, and the type decides how much the local
// value can hold. A value too large for the form its administrator selects is
// refused rather than truncated, because a truncated route-target names a
// different VRF.
func FlowSpecRedirect(admin, value string) (ExtendedCommunity, error) {
	if addr, err := netip.ParseAddr(admin); err == nil {
		return flowSpecRedirectIPv4(addr, value)
	}

	asn, err := strconv.ParseUint(admin, 10, 32)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf(
			"invalid redirect administrator %q: expected an AS number or an IPv4 address", admin)
	}
	if asn > 0xFFFF {
		return flowSpecRedirect4ByteAS(uint32(asn), value)
	}
	return flowSpecRedirect2ByteAS(uint16(asn), value)
}

// flowSpecRedirect2ByteAS builds the type 0x80 form: RFC 4360 Section 3.1, a
// 2-octet AS with a 4-octet local administrator.
func flowSpecRedirect2ByteAS(asn uint16, value string) (ExtendedCommunity, error) {
	local, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("invalid redirect value %q (2-octet AS form holds 0-4294967295)", value)
	}
	return ExtendedCommunity{
		extCommTypeTransitiveFlowSpec, extCommSubtypeRedirect,
		byte(asn >> 8), byte(asn),
		byte(local >> 24), byte(local >> 16), byte(local >> 8), byte(local),
	}, nil
}

// flowSpecRedirectIPv4 builds the type 0x81 form: RFC 4360 Section 3.2, a
// 4-octet IPv4 address with a 2-octet local administrator.
//
// An IPv6 address has no rt-redirect encoding in RFC 8955 Section 7.4, whose
// three types are all 8 octets. RFC 8956 Section 6.1 gives IPv6 its own 20-octet
// community, which is a different action and a different attribute.
func flowSpecRedirectIPv4(addr netip.Addr, value string) (ExtendedCommunity, error) {
	if !addr.Is4() {
		return ExtendedCommunity{}, fmt.Errorf("redirect administrator %s is not an IPv4 address", addr)
	}
	local, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("invalid redirect value %q (IPv4 form holds 0-65535)", value)
	}
	ip := addr.As4()
	return ExtendedCommunity{
		0x81, extCommSubtypeRedirect,
		ip[0], ip[1], ip[2], ip[3],
		byte(local >> 8), byte(local),
	}, nil
}

// flowSpecRedirect4ByteAS builds the type 0x82 form: RFC 5668 Section 2, a
// 4-octet AS with a 2-octet local administrator.
func flowSpecRedirect4ByteAS(asn uint32, value string) (ExtendedCommunity, error) {
	local, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("invalid redirect value %q (4-octet AS form holds 0-65535)", value)
	}
	return ExtendedCommunity{
		0x82, extCommSubtypeRedirect,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(local >> 8), byte(local),
	}, nil
}

// flowSpecSubtypeRedirectToIP is the sub-type of the redirect-to-IP action, in
// both its IPv4-address-specific and its IPv6-address-specific form.
//
// The action is not RFC 8955 Section 7.4's rt-redirect, which names a VRF by
// route-target. It sends the matched traffic to one address, and it comes from
// draft-ietf-idr-flowspec-redirect-ip, where the 2-octet local administrator is
// a flags field whose low bit is the copy semantic. Ze writes that field zero:
// redirect, do not copy.
const flowSpecSubtypeRedirectToIP = 0x0c

// FlowSpecRedirectToIPv4 returns the redirect-to-IP extended community for an
// IPv4 next hop: an IPv4-address-specific community (type 0x01) carrying the
// address and a zero flags field.
func FlowSpecRedirectToIPv4(addr netip.Addr) (ExtendedCommunity, error) {
	if !addr.Is4() {
		return ExtendedCommunity{}, fmt.Errorf("redirect-to-nexthop %s is not an IPv4 address", addr)
	}
	ip := addr.As4()
	return ExtendedCommunity{
		0x01, flowSpecSubtypeRedirectToIP,
		ip[0], ip[1], ip[2], ip[3],
		0x00, 0x00,
	}, nil
}

// FlowSpecRedirectToIPv6 returns the redirect-to-IP community for an IPv6 next
// hop.
//
// It is 20 octets, not 8, so it rides the IPV6_EXTENDED_COMMUNITIES attribute
// (code 25) rather than EXTENDED_COMMUNITIES. RFC 5701 Section 2 gives the
// layout: a first octet of 0x00 for a transitive sub-type, the sub-type, a
// 16-octet global administrator holding the address, and a 2-octet local
// administrator, which here is the same flags field FlowSpecRedirectToIPv4
// writes zero.
func FlowSpecRedirectToIPv6(addr netip.Addr) (IPv6ExtendedCommunity, error) {
	var ec IPv6ExtendedCommunity
	if !addr.Is6() || addr.Is4In6() {
		return ec, fmt.Errorf("redirect-to-nexthop %s is not an IPv6 address", addr)
	}
	ip := addr.As16()
	// RFC 5701 Section 2: "The first high-order octet indicates whether a
	// particular sub-type of this community is transitive across Autonomous
	// Systems (ASes) (0x00), or not (0x40)."
	ec[0] = 0x00
	ec[1] = flowSpecSubtypeRedirectToIP
	copy(ec[2:18], ip[:])
	ec[18] = 0x00
	ec[19] = 0x00
	return ec, nil
}
