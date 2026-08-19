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
		return appendExtCommAS2Specific(buf, "origin:", e)
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
