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
	"strconv"
)

// Extended community type/subtype pairs rendered by name, read as the two type
// octets in one big-endian uint16.
const (
	extCommRouteTarget        = 0x0002 // RFC 4360 Section 4: Route Target, two-octet AS specific
	extCommRouteOrigin        = 0x0003 // RFC 4360 Section 5: Route Origin, two-octet AS specific
	extCommTrafficRateBytes   = 0x8006 // RFC 8955 Section 7.1: traffic-rate-bytes
	extCommTrafficAction      = 0x8007 // RFC 8955 Section 7.3: traffic-action
	extCommRedirect           = 0x8008 // RFC 8955 Section 7.4: rt-redirect, two-octet AS specific
	extCommTrafficMarking     = 0x8009 // RFC 8955 Section 7.5: traffic-marking
	extCommTrafficRatePackets = 0x800c // RFC 8955 Section 7.2: traffic-rate-packets
)

// RFC 8955 Section 7.5: the DSCP is carried in "the 6 least significant bits of
// the Extended Community value", and every bit above it is reserved.
const extCommDSCPMask = 0x3f

// AppendDecoded appends the extended community's named form to buf and returns
// the extended buffer. It allocates nothing.
//
// The names are the ones Ze's own parsers accept on input
// (route/route_community.go parseExtendedCommunity,
// config/routeattr_community.go parseOneExtCommunity), so a community written
// as "target:65000:1" is read back as "target:65000:1".
//
// A type this function does not name renders as "0x<type><subtype>:<hex>", so
// the octets stay readable instead of being dropped.
func (e ExtendedCommunity) AppendDecoded(buf []byte) []byte {
	switch binary.BigEndian.Uint16(e[0:2]) {
	case extCommRouteTarget:
		return appendExtCommASSpecific(buf, "target:", e)
	case extCommRouteOrigin:
		return appendExtCommASSpecific(buf, "origin:", e)
	case extCommRedirect:
		return appendExtCommASSpecific(buf, "redirect:", e)
	case extCommTrafficRateBytes:
		return appendExtCommTrafficRate(buf, e, "")
	case extCommTrafficRatePackets:
		return appendExtCommTrafficRate(buf, e, "packets")
	case extCommTrafficAction:
		// RFC 8955 Section 7.3: the unused bits of the Traffic Action Field
		// "MUST be set to 0 on encoding and MUST be ignored during decoding",
		// so the name carries the whole rendering and no value octet is read.
		return append(buf, "traffic-action"...)
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

// appendExtCommASSpecific appends "<name><2-octet AS>:<4-octet local
// administrator>", the shape RFC 4360 Section 3.1 gives the transitive
// two-octet AS specific extended community and RFC 8955 Section 7.4 reuses for
// rt-redirect.
func appendExtCommASSpecific(buf []byte, name string, e ExtendedCommunity) []byte {
	buf = append(buf, name...)
	buf = strconv.AppendUint(buf, uint64(binary.BigEndian.Uint16(e[2:4])), 10)
	buf = append(buf, ':')
	return strconv.AppendUint(buf, uint64(binary.BigEndian.Uint32(e[4:8])), 10)
}

// appendExtCommTrafficRate appends "rate-limit:<rate>", and ":<unit>" when the
// sub-type names one.
//
// RFC 8955 Sections 7.1 and 7.2: the rate is a 4-octet IEEE 754 float in the
// last four octets, and "On decoding, negative values MUST be treated as zero
// (i.e., discard all traffic)". A NaN carries no rate either, so it discards.
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
