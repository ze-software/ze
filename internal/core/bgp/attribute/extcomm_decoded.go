// Design: docs/architecture/wire/attributes.md -- path attribute encoding
// RFC: rfc/short/rfc4360.md -- Route Target and Route Origin extended communities
// RFC: rfc/short/rfc8955.md -- FlowSpec traffic filtering actions (Section 7)
// Related: flowspec_action.go -- the encode table for colon-less FlowSpec keywords
// Related: text_append.go -- AppendText, the raw 8-octet hex filter-text form
//
// The decode half of the extended community vocabulary. flowspec_action.go
// holds the encode half, and the two must spell one community the same way: a
// receiver that reads "0002fde800000001" where the sender writes
// "target:65000:1" cannot act on what it was sent. The FlowSpec firewall
// bridge matches "rate-limit:0", "rate-limit:<n>" and "mark:<n>", and
// `ze bgp decode` prints the same words, so one renderer serves both.
//
// AppendText in text_append.go is a DIFFERENT rendering and stays as it is: it
// is the filter-text contract ("extended-community <hex>"), which every filter
// plugin parses.

package attribute

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"net/netip"
	"strconv"
)

// Extended community type/subtype pairs rendered by name, read as the two type
// octets in one big-endian uint16.
//
// Route Target, Route Origin and rt-redirect each come in three administrator
// forms, which the HIGH type octet selects: 0x00 two-octet AS (RFC 4360 Section
// 3.1), 0x01 IPv4 address (RFC 4360 Section 3.2), 0x02 four-octet AS (RFC 5668
// Section 2). RFC 4360 Sections 4 and 5 name all three for Route Target and
// Route Origin, and RFC 8955 Section 7.4 names 0x80, 0x81 and 0x82 for the
// FlowSpec redirect siblings.
const (
	extCommRouteTargetAS2     = 0x0002 // RFC 4360 Section 4: Route Target, two-octet AS specific
	extCommRouteTargetIPv4    = 0x0102 // RFC 4360 Section 4: Route Target, IPv4 address specific
	extCommRouteTargetAS4     = 0x0202 // RFC 4360 Section 4: Route Target, four-octet AS specific
	extCommRouteOriginAS2     = 0x0003 // RFC 4360 Section 5: Route Origin, two-octet AS specific
	extCommRouteOriginIPv4    = 0x0103 // RFC 4360 Section 5: Route Origin, IPv4 address specific
	extCommRouteOriginAS4     = 0x0203 // RFC 4360 Section 5: Route Origin, four-octet AS specific
	extCommTrafficRateBytes   = 0x8006 // RFC 8955 Section 7.1: traffic-rate-bytes
	extCommTrafficAction      = 0x8007 // RFC 8955 Section 7.3: traffic-action
	extCommRedirectAS2        = 0x8008 // RFC 8955 Section 7.4: rt-redirect, two-octet AS specific
	extCommRedirectIPv4       = 0x8108 // RFC 8955 Section 7.4: rt-redirect, IPv4 address specific
	extCommRedirectAS4        = 0x8208 // RFC 8955 Section 7.4: rt-redirect, four-octet AS specific
	extCommTrafficMarking     = 0x8009 // RFC 8955 Section 7.5: traffic-marking
	extCommTrafficRatePackets = 0x800c // RFC 8955 Section 7.2: traffic-rate-packets
	extCommLayer2Info         = 0x800a // RFC 4761 Section 3.2.4: Layer2 Info, VPLS pseudowire control
	extCommRedirectToIPv4     = 0x010c // draft-ietf-idr-flowspec-redirect-ip: redirect to an IPv4 next hop
	extCommMUPDirectSegment   = 0x0c00 // draft-ietf-bess-mup-safi Section 3.2: MUP, Direct-Type Segment Identifier
)

// RFC 8955 Section 7.3 Figure 5: the two defined bits of the 6-octet Traffic
// Action Field are the last two, so both sit in the final octet.
const (
	extCommTrafficActionTerminal = 0x01 // T, bit 47
	extCommTrafficActionSample   = 0x02 // S, bit 46
)

// RFC 8955 Section 7.5: the DSCP is carried in "the 6 least significant bits of
// the Extended Community value", and every bit above it is reserved.
const extCommDSCPMask = 0x3f

// AppendDecoded appends the extended community's named form to buf and returns
// the extended buffer. It allocates nothing.
//
// The Route Target, Route Origin and rt-redirect names are the ones Ze's own
// parsers accept on input (route/route_community.go parseExtendedCommunity,
// config/routeattr_community.go parseOneExtCommunity), so a community written
// as "target:65000:1" is read back as "target:65000:1", and one written as
// "target:8.8.8.8:8000" is read back as "target:8.8.8.8:8000".
//
// The traffic-action form carries the flag words the config parser reads
// ("sample", "terminal", "sample-terminal"), behind the "traffic-action:"
// keyword rather than behind the "action " keyword config writes, so it is
// read by a human rather than fed back to that parser.
//
// A type this function does not name renders as "0x<type><subtype>:<hex>", so
// the octets stay readable instead of being dropped.
func (e ExtendedCommunity) AppendDecoded(buf []byte) []byte {
	switch binary.BigEndian.Uint16(e[0:2]) {
	case extCommRouteTargetAS2:
		return appendExtCommAS2Specific(buf, "target:", e)
	case extCommRouteTargetIPv4:
		return appendExtCommIPv4Specific(buf, "target:", e)
	case extCommRouteTargetAS4:
		return appendExtCommAS4Specific(buf, "target:", e)
	case extCommRouteOriginAS2:
		return appendExtCommOriginAS2(buf, e)
	case extCommRouteOriginIPv4:
		return appendExtCommIPv4Specific(buf, "origin:", e)
	case extCommRouteOriginAS4:
		return appendExtCommAS4Specific(buf, "origin:", e)
	case extCommRedirectAS2:
		return appendExtCommAS2Specific(buf, "redirect:", e)
	case extCommRedirectIPv4:
		return appendExtCommIPv4Specific(buf, "redirect:", e)
	case extCommRedirectAS4:
		return appendExtCommAS4Specific(buf, "redirect:", e)
	case extCommTrafficRateBytes:
		return appendExtCommTrafficRate(buf, e, "")
	case extCommTrafficRatePackets:
		return appendExtCommTrafficRate(buf, e, "packets")
	case extCommTrafficAction:
		return appendExtCommTrafficAction(buf, e)
	case extCommMUPDirectSegment:
		// The two halves are a Direct-Type Segment Identifier, not an AS and a
		// local administrator, but they split the six value octets exactly
		// where RFC 4360 Section 3.1 splits them, so one writer serves both.
		return appendExtCommAS2Specific(buf, "mup:", e)
	case extCommRedirectToIPv4:
		return appendExtCommRedirectToIPv4(buf, e)
	case extCommLayer2Info:
		return appendExtCommLayer2Info(buf, e)
	case extCommTrafficMarking:
		// RFC 8955 Section 7.5: "reserved (r): MUST be set to 0 on encoding and
		// MUST be ignored during decoding". Masking to the low 6 bits is what
		// ignores them. Reading the whole octet yields a value above 63, which
		// is no DSCP, and the firewall then drops the marking action.
		buf = append(buf, "mark:"...)
		return strconv.AppendUint(buf, uint64(e[7]&extCommDSCPMask), 10)
	}
	buf = append(buf, "0x"...)
	buf = hex.AppendEncode(buf, e[0:2])
	buf = append(buf, ':')
	return hex.AppendEncode(buf, e[2:8])
}

// String returns the extended community's named form.
//
// For a human and for a parser that accepts this vocabulary. The raw hex form
// stays available as AppendText, which serves the filter-text contract.
func (e ExtendedCommunity) String() string {
	// Longest rendering is "rate-limit:18446744073709551615:packets", 39 bytes.
	var buf [48]byte
	return string(e.AppendDecoded(buf[:0]))
}

// appendExtCommAS2Specific appends "<name><2-octet AS>:<4-octet local
// administrator>", the shape RFC 4360 Section 3.1 gives the two-octet AS
// specific extended community and RFC 8955 Section 7.4 reuses for rt-redirect
// type 0x80.
func appendExtCommAS2Specific(buf []byte, name string, e ExtendedCommunity) []byte {
	buf = append(buf, name...)
	buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[2:4])), 10)
	buf = append(buf, ':')
	return strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(e[4:8])), 10)
}

// The MUP arm above reuses appendExtCommAS2Specific. What it writes there is
// `mup:<2-octet segment id>:<4-octet segment id>`, the spelling Ze's config
// parser reads (parseMUPExtCommunity, component/bgp/config/routeattr_community.go)
// and the one ExaBGP prints (MUPExtendedCommunity.__repr__, its mup.py).
// Rendering the octets left a MUP segment identifier an operator configured
// coming back as "0x0c00:000a0000000a", which that parser refuses.

// appendExtCommOriginAS2 appends "origin:<2-octet AS>:<IPv4 local
// administrator>".
//
// The two-octet AS specific Route Origin is the one place the 4-octet local
// administrator is written as an address rather than a number, and it is Ze's
// own input grammar that says so: parseOriginExtCommunity
// (component/bgp/route/route_community.go) REFUSES "origin:100:1000" and reads
// "origin:100:0.0.3.232" into these same four octets. Rendering the number
// broke that round trip in one direction only, so a community an operator
// configured came back in a spelling Ze itself will not accept.
//
// RFC 4360 Section 3.1 calls the field "a number from a numbering space which
// is administered by the organization identified by the Global Administrator
// sub-field", which constrains the wire and not the spelling. ExaBGP writes
// the address form too (OriginASNIP.__repr__, in its origin.py), so one
// community reads the same on both projects.
//
// Route Target keeps the number: parseTargetExtCommunity writes a decimal into
// the same four octets, and so does ExaBGP.
func appendExtCommOriginAS2(buf []byte, e ExtendedCommunity) []byte {
	buf = append(buf, "origin:"...)
	buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[2:4])), 10)
	buf = append(buf, ':')
	return netip.AddrFrom4([4]byte{e[4], e[5], e[6], e[7]}).AppendTo(buf)
}

// appendExtCommLayer2Info appends "l2info:<encaps>:<control>:<mtu>:<reserved>",
// the four fields RFC 4761 Section 3.2.4 Figure 3 gives the community.
//
// Ze's config parser has read this spelling since VPLS arrived
// (parseL2InfoExtCommunity, component/bgp/config/routeattr_community.go) and
// nothing rendered it, so a VPLS route an operator configured came back as
// "0x800a:130005dc006f" -- the octets, with no reader able to act on them
// without Figure 3 to hand.
//
// The fourth field is "Reserved (2 octets)" in the RFC and carries the
// preference in draft-ietf-l2vpn-vpls-multihoming, which is why Ze's parser
// names it preference and why it is rendered rather than dropped: MUST be set
// to zero when sending is a rule about the SENDER, and a receiver that hides a
// non-zero value cannot tell an operator why a peer's route differs.
func appendExtCommLayer2Info(buf []byte, e ExtendedCommunity) []byte {
	buf = append(buf, "l2info:"...)
	buf = strconv.AppendUint(buf, uint64(e[2]), 10)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(e[3]), 10)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[4:6])), 10)
	buf = append(buf, ':')
	return strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[6:8])), 10)
}

// appendExtCommRedirectToIPv4 appends "redirect-to-nexthop <IPv4>", or
// "copy-to-nexthop <IPv4>" when the copy bit is set.
//
// draft-ietf-idr-flowspec-redirect-ip gives the community an IPv4 global
// administrator holding the next hop and a 2-octet local administrator whose
// low bit says whether the matched traffic is COPIED to that address or
// REDIRECTED to it. The two are different instructions to a forwarding plane,
// so the bit is read rather than assumed: FlowSpecRedirectToIPv4
// (flowspec_encode.go) writes it zero, and a peer is free to set it.
//
// The words are Ze's own input grammar, which reads `redirect-to-nexthop
// 1.2.3.4` as two whitespace-separated parts (parseExtendedCommunitySpec,
// component/bgp/config/routeattr_community.go). Until this arm existed the
// community came back as "0x010c:010203040000", which that parser refuses, so
// a FlowSpec redirect an operator configured could not be read back.
func appendExtCommRedirectToIPv4(buf []byte, e ExtendedCommunity) []byte {
	if binary.BigEndian.Uint16(e[6:8])&0x01 != 0 {
		buf = append(buf, "copy-to-nexthop "...)
	} else {
		buf = append(buf, "redirect-to-nexthop "...)
	}
	return netip.AddrFrom4([4]byte{e[2], e[3], e[4], e[5]}).AppendTo(buf)
}

// appendExtCommAS4Specific appends "<name><4-octet AS>:<2-octet local
// administrator>", the shape RFC 5668 Section 2 gives the four-octet AS
// specific extended community, which RFC 4360 Sections 4 and 5 name as type
// 0x02 and RFC 8955 Section 7.4 reuses for rt-redirect type 0x82.
//
// The administrator is four octets wide here and two octets wide in
// appendExtCommAS2Specific, so the 6-octet value field splits in the other
// place. A four-byte-ASN deployment carries this form and nothing else.
func appendExtCommAS4Specific(buf []byte, name string, e ExtendedCommunity) []byte {
	buf = append(buf, name...)
	buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(e[2:6])), 10)
	buf = append(buf, ':')
	return strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[6:8])), 10)
}

// appendExtCommIPv4Specific appends "<name><IPv4 global administrator>:<2-octet
// local administrator>", the shape RFC 4360 Section 3.2 gives the IPv4 address
// specific extended community, which RFC 4360 Sections 4 and 5 name as type
// 0x01 and RFC 8955 Section 7.4 reuses for rt-redirect type 0x81.
//
// The administrator is an address rather than a number, so it renders as a
// dotted quad: that is the form the operator configured and the form Ze's own
// config parser reads back (config/routeattr_community.go
// parseRouteTargetOrOrigin).
func appendExtCommIPv4Specific(buf []byte, name string, e ExtendedCommunity) []byte {
	buf = append(buf, name...)
	buf = netip.AddrFrom4([4]byte{e[2], e[3], e[4], e[5]}).AppendTo(buf)
	buf = append(buf, ':')
	return strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[6:8])), 10)
}

// appendExtCommTrafficAction appends "traffic-action:" and the names of the
// action bits RFC 8955 Section 7.3 defines: S (Sample, bit 46) and T (Terminal
// Action, bit 47). With neither bit set it appends "traffic-action:none", so
// the value the peer sent is always spelled out and no reader has to ask
// whether a bare name hid a set bit.
//
// The flag words are the ones the config parser reads on input, "sample",
// "terminal" and the hyphen-joined "sample-terminal" for both
// (config/routeattr_community.go parseFlowSpecAction, written in config as
// `extended-community [action sample-terminal]`), so one community is spelled
// one way on the way in and on the way out.
//
// "terminal" is the RFC's own field name and it reads backwards: T SET tells
// the filtering engine to go on and evaluate later Flow Specifications, and T
// CLEAR stops evaluation at this one. The name follows the RFC rather than the
// behavior, because an operator comparing this output against another
// implementation's is comparing field names.
//
// Every other bit of the 6-octet Traffic Action Field is unused: Section 7.3
// says they "MUST be set to 0 on encoding and MUST be ignored during decoding",
// so no octet but the last is read, and only two bits of that one.
func appendExtCommTrafficAction(buf []byte, e ExtendedCommunity) []byte {
	buf = append(buf, "traffic-action:"...)
	switch e[7] & (extCommTrafficActionSample | extCommTrafficActionTerminal) {
	case extCommTrafficActionSample | extCommTrafficActionTerminal:
		return append(buf, "sample-terminal"...)
	case extCommTrafficActionSample:
		return append(buf, "sample"...)
	case extCommTrafficActionTerminal:
		return append(buf, "terminal"...)
	}
	return append(buf, "none"...)
}

// appendExtCommTrafficRate appends "rate-limit:<rate>", and ":<unit>" when the
// sub-type names one.
//
// RFC 8955 Sections 7.1 and 7.2: the rate is a 4-octet IEEE 754 float in the
// last four octets, and "On decoding, negative values MUST be treated as zero
// (i.e., discard all traffic)". A NaN carries no rate either, so it discards.
//
// A fractional rate rounds DOWN, because the integer conversion below truncates.
// The RFC states no rounding rule, so this states one: a rate under one unit per
// second renders "rate-limit:0", which the FlowSpec firewall bridge reads as
// discard-all. Rounding up instead would let a peer that asked for almost no
// traffic get one whole unit per second, and the peer that means "discard" has
// an exact encoding for it (rate 0), so nothing needs the fractional value to
// survive. The direction is load-bearing rather than incidental: this renderer
// feeds the firewall's input on the receive path.
func appendExtCommTrafficRate(buf []byte, e ExtendedCommunity, unit string) []byte {
	rate := float64(math.Float32frombits(binary.BigEndian.Uint32(e[4:8])))
	if rate < 0 || math.IsNaN(rate) {
		rate = 0
	}

	// A float32 reaches 3.4e38, far above the uint64 range, and Go leaves an
	// out-of-range float-to-integer conversion undefined: amd64 answers 1<<63
	// and arm64 answers MaxUint64 for the same wire bytes. Saturate, so one
	// UPDATE reads the same on every architecture Ze ships on.
	rateWhole := uint64(math.MaxUint64)
	if rate < math.MaxUint64 {
		rateWhole = uint64(rate)
	}

	buf = append(buf, "rate-limit:"...)
	buf = strconv.AppendUint(buf, rateWhole, 10)
	if unit == "" {
		return buf
	}
	buf = append(buf, ':')
	return append(buf, unit...)
}

// AppendDecoded appends the IPv6 address specific extended community's named
// form to buf and returns the extended buffer. It allocates nothing.
//
// RFC 5701 Section 2 puts a 16-octet IPv6 global administrator where RFC 4360
// Section 3.1 puts a two-octet AS, so every field offset the 8-octet vocabulary
// reads names something else here and the two cannot share a renderer.
//
// One sub-type is named, and it is the one Ze itself produces:
// draft-ietf-idr-flowspec-redirect-ip's redirect-to-IP, written by
// FlowSpecRedirectToIPv6 (flowspec_encode.go) from the `redirect-to-nexthop
// <IPv6>` an operator configures. Rendering it as hex meant a FlowSpec redirect
// to an IPv6 next hop was the one action Ze could accept and could not read
// back.
//
// Anything else keeps its octets as "0x<transitivity><sub-type>:<hex>", the
// same shape the 8-octet renderer falls back to.
func (e IPv6ExtendedCommunity) AppendDecoded(buf []byte) []byte {
	if e[1] == flowSpecSubtypeRedirectToIP {
		if binary.BigEndian.Uint16(e[18:20])&0x01 != 0 {
			buf = append(buf, "copy-to-nexthop "...)
		} else {
			buf = append(buf, "redirect-to-nexthop "...)
		}
		return netip.AddrFrom16([16]byte(e[2:18])).AppendTo(buf)
	}
	buf = append(buf, "0x"...)
	buf = hex.AppendEncode(buf, e[0:2])
	buf = append(buf, ':')
	return hex.AppendEncode(buf, e[2:20])
}

// String returns the IPv6 extended community's named form.
func (e IPv6ExtendedCommunity) String() string {
	// Longest rendering is "redirect-to-nexthop " plus a 45-byte address.
	var buf [72]byte
	return string(e.AppendDecoded(buf[:0]))
}
