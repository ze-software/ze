// Design: docs/architecture/config/syntax.md — community attribute parsing
// RFC: rfc/short/rfc8955.md — FlowSpec traffic filtering actions (Section 7)
// Overview: routeattr.go — core route attribute types
// Related: ../../../core/bgp/attribute/flowspec_encode.go — the shared action encoders

package bgpconfig

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

var err4ByteAsnWithIpValue = errors.New("4-byte ASN with IP value not supported")

const (
	flowSpecRateLimitName        = "rate-limit"
	flowSpecRateLimitPacketsName = "rate-limit-packets"
	flowSpecPacketsUnit          = "packets"
	flowSpecBytesUnit            = "bytes"
)

// Community represents standard BGP communities (RFC 1997).
// Each community is 4 bytes: high 16 bits = ASN, low 16 bits = value.
type Community struct {
	Raw    string   // Original string (e.g., "30740:30740")
	Values []uint32 // Parsed values (each is ASN<<16 | value)
}

// ParseCommunity parses community string(s) to wire format values.
// Formats: ASN:Value, list in brackets [ASN:Value ASN:Value], well-known names.
func ParseCommunity(s string) (Community, error) {
	if s == "" {
		return Community{}, nil
	}

	// Remove brackets if present: [30740:0 30740:30740] -> 30740:0 30740:30740
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		s = strings.TrimSpace(s)
	}

	parts := strings.Fields(s)
	values := make([]uint32, 0, len(parts))

	for _, p := range parts {
		v, err := parseOneCommunity(p)
		if err != nil {
			return Community{}, err
		}
		values = append(values, v)
	}

	return Community{Raw: s, Values: values}, nil
}

// parseOneCommunity parses a single community string to uint32.
// Delegates to attribute.ParseCommunity for shared logic.
func parseOneCommunity(s string) (uint32, error) {
	return attribute.ParseCommunity(s)
}

// LargeCommunity represents large BGP communities (RFC 8092).
// Each community is 12 bytes: GlobalAdmin(4) + LocalData1(4) + LocalData2(4).
type LargeCommunity struct {
	Raw    string      // Original string
	Values [][3]uint32 // Parsed values (each is [GA, LD1, LD2])
}

// ParseLargeCommunity parses large community string(s).
// Format: GA:LD1:LD2, list in brackets [GA:LD1:LD2 GA:LD1:LD2].
// Duplicates are removed (per RFC 8092).
func ParseLargeCommunity(s string) (LargeCommunity, error) {
	if s == "" {
		return LargeCommunity{}, nil
	}

	// Remove brackets if present
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		s = strings.TrimSpace(s)
	}

	parts := strings.Fields(s)
	values := make([][3]uint32, 0, len(parts))
	seen := make(map[[3]uint32]bool)

	for _, p := range parts {
		v, err := parseOneLargeCommunity(p)
		if err != nil {
			return LargeCommunity{}, err
		}
		// Deduplicate
		if !seen[v] {
			seen[v] = true
			values = append(values, v)
		}
	}

	return LargeCommunity{Raw: s, Values: values}, nil
}

// parseOneLargeCommunity parses a single large community to [3]uint32.
// Delegates to attribute.ParseLargeCommunity for shared logic.
func parseOneLargeCommunity(s string) ([3]uint32, error) {
	lc, err := attribute.ParseLargeCommunity(s)
	if err != nil {
		return [3]uint32{}, err
	}
	return [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2}, nil
}

// ExtendedCommunity represents one or more extended communities (RFC 4360).
// Formats: target:ASN:NN, origin:ASN:NN, N:IP:NN, ASN:IP (type-0 generic).
//
// One string can produce communities of two widths, so it produces two byte
// slices. Every RFC 4360 community is 8 octets and rides EXTENDED_COMMUNITIES
// (attribute 16). An IPv6-address-specific community is 20 octets and rides
// IPV6_EXTENDED_COMMUNITIES (attribute 25, RFC 5701 Section 2), because a
// 16-octet global administrator does not fit the 8-octet form.
type ExtendedCommunity struct {
	Raw       string // Original string for encoding
	Bytes     []byte // Wire-format bytes (8 bytes per community), attribute 16
	IPv6Bytes []byte // Wire-format bytes (20 bytes per community), attribute 25
}

// Extended community types and subtypes (RFC 4360, RFC 7153).
const (
	// Type high byte (transitive = 0x00, non-transitive = 0x40).
	ecTypeTransitive2ByteAS = 0x00 // 2-byte AS, transitive
	ecTypeTransitiveIPv4    = 0x01 // IPv4 address, transitive
	ecTypeTransitive4ByteAS = 0x02 // 4-byte AS, transitive

	// Subtypes.
	ecSubtypeRouteTarget = 0x02 // Route Target (RFC 4360)
	ecSubtypeRouteOrigin = 0x03 // Route Origin (RFC 4360)
)

// ParseExtendedCommunity parses extended community string(s).
// Formats: target:ASN:NN, origin:ASN:NN, ASN:IP (generic type-0).
func ParseExtendedCommunity(s string) (ExtendedCommunity, error) {
	if s == "" {
		return ExtendedCommunity{}, nil
	}

	// Strip brackets if present: "[ target:X:Y origin:A:B ]" -> "target:X:Y origin:A:B"
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	parts := strings.Fields(s)
	var allBytes []byte
	var allIPv6Bytes []byte

	// Process parts, looking ahead for two-word formats (action, mark, redirect-to-nexthop).
	for i := 0; i < len(parts); i++ {
		p := parts[i]

		// Check for two-word formats that need the next part
		if i+1 < len(parts) {
			switch p {
			case "action":
				// "action sample-terminal" or "action sample" or "action terminal".
				ec, err := attribute.FlowSpecTrafficAction(parts[i+1])
				if err != nil {
					return ExtendedCommunity{}, err
				}
				allBytes = append(allBytes, ec[:]...)
				i++ // skip next part
				continue
			case "mark":
				// "mark N" - DSCP marking
				ec, err := parseFlowSpecMark(parts[i+1])
				if err != nil {
					return ExtendedCommunity{}, err
				}
				allBytes = append(allBytes, ec[:]...)
				i++ // skip next part
				continue
			case flowSpecRedirectNextHop:
				// "redirect-to-nexthop IP". This is the ONE place that decides
				// which form the string names, because the two forms land in
				// different attributes: an IPv4 next hop is an 8-octet
				// IPv4-address-specific community, and an IPv6 next hop is a
				// 20-octet IPv6-address-specific one (RFC 5701 Section 2).
				// Deciding it twice is what left the IPv6 encoder unreachable:
				// this parser refused the string before the second decision ran.
				ec8, ec20, err := parseFlowSpecRedirectNextHop(parts[i+1])
				if err != nil {
					return ExtendedCommunity{}, err
				}
				allBytes = append(allBytes, ec8...)
				allIPv6Bytes = append(allIPv6Bytes, ec20...)
				i++ // skip next part
				continue
			}
		}

		// Single-word extended community
		b, err := parseOneExtCommunity(p)
		if err != nil {
			return ExtendedCommunity{}, err
		}
		allBytes = append(allBytes, b...)
	}

	return ExtendedCommunity{Raw: s, Bytes: allBytes, IPv6Bytes: allIPv6Bytes}, nil
}

// parseOneExtCommunity parses a single extended community string to 8 bytes.
// Supports formats:
//   - Hex format: 0x0002fde800000001 (16 hex chars = 8 bytes wire format)
//   - Named format: target:ASN:NN, origin:ASN:NN
//   - FlowSpec actions: rate-limit:N, rate-limit:N:packets, rate-limit-packets:N, redirect-to-nexthop-draft, copy-to-nexthop, mark N
//   - Generic format: ASN:NN, IP:NN
func parseOneExtCommunity(s string) ([]byte, error) {
	// Check for hex format (0x prefix, no colons)
	if (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) && !strings.Contains(s, ":") {
		return parseExtCommunityHex(s)
	}

	// FlowSpec single-word actions (no colons). The table lives in the attribute
	// package because the `update text` API parser needs the SAME vocabulary
	// (route/route_community.go parseExtendedCommunity); it used to have its own,
	// shorter one, so `copy-to-nexthop` worked here and failed there.
	if ec, ok := attribute.FlowSpecActionKeyword(s); ok {
		return ec[:], nil
	}

	// Format: [type:]value1:value2
	parts := strings.Split(s, ":")

	// FlowSpec rate-limit:N, rate-limit:N:packets, and legacy rate-limit-packets:N formats.
	if len(parts) == 2 && parts[0] == flowSpecRateLimitPacketsName {
		return parseFlowSpecRateLimitWithSubtype(parts[1], attribute.FlowSpecRatePackets)
	}
	if len(parts) == 2 && parts[0] == flowSpecRateLimitName {
		return parseFlowSpecRateLimit(parts[1])
	}
	if len(parts) == 3 && parts[0] == flowSpecRateLimitName {
		switch parts[2] {
		case flowSpecPacketsUnit:
			return parseFlowSpecRateLimitWithSubtype(parts[1], attribute.FlowSpecRatePackets)
		case flowSpecBytesUnit:
			return parseFlowSpecRateLimitWithSubtype(parts[1], attribute.FlowSpecRateBytes)
		default:
			return nil, fmt.Errorf("unknown rate-limit unit %q", parts[2])
		}
	}

	if len(parts) == 2 {
		// Generic format: ASN:NN, IP:NN or ASN:IP. The subtype an unqualified
		// pair carries is route target, which is what "target:" spells out.
		return parseRouteTargetOrOrigin(ecSubtypeRouteTarget, parts[0], parts[1])
	}

	if len(parts) == 3 {
		// Named format: target:ASN:NN or origin:ASN:NN
		switch parts[0] {
		case "target":
			return parseRouteTargetOrOrigin(ecSubtypeRouteTarget, parts[1], parts[2])
		case "origin":
			return parseRouteTargetOrOrigin(ecSubtypeRouteOrigin, parts[1], parts[2])
		case "target4":
			// Explicit 4-byte AS route target
			return parseRouteTargetOrOrigin4(ecSubtypeRouteTarget, parts[1], parts[2])
		case "origin4":
			// Explicit 4-byte AS route origin
			return parseRouteTargetOrOrigin4(ecSubtypeRouteOrigin, parts[1], parts[2])
		case "redirect":
			// FlowSpec redirect (RFC 5575): type 0x80, subtype 0x08
			return parseFlowSpecRedirect(parts[1], parts[2])
		case "mup":
			// MUP extended community: mup:ASN:NN (draft-mpmz-bess-mup-safi)
			// Uses type 0x0C (Generic Transitive Experimental Use) with subtype 0x00
			return parseMUPExtCommunity(parts[1], parts[2])
		}
		return nil, fmt.Errorf("unknown extended-community type %q", parts[0])
	}

	if len(parts) == 5 && parts[0] == "l2info" {
		return parseL2InfoExtCommunity(parts[1], parts[2], parts[3], parts[4])
	}

	return nil, fmt.Errorf("invalid extended-community %q: expected format like target:ASN:NN", s)
}

// parseL2InfoExtCommunity parses Layer 2 Info Extended Community (RFC 4761).
// Format: l2info:encaps:control:mtu:preference
// Wire format: 0x800A | encaps(1) | control(1) | mtu(2) | preference(2).
func parseL2InfoExtCommunity(encapsStr, controlStr, mtuStr, prefStr string) ([]byte, error) {
	encaps, err := strconv.ParseUint(encapsStr, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("invalid l2info encaps %q", encapsStr)
	}
	control, err := strconv.ParseUint(controlStr, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("invalid l2info control %q", controlStr)
	}
	mtu, err := strconv.ParseUint(mtuStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid l2info mtu %q", mtuStr)
	}
	preference, err := strconv.ParseUint(prefStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid l2info preference %q", prefStr)
	}
	return []byte{
		0x80, 0x0A, // Type: Layer 2 Info
		byte(encaps), byte(control),
		byte(mtu >> 8), byte(mtu),
		byte(preference >> 8), byte(preference),
	}, nil
}

// parseExtCommunityHex parses hex format extended community (e.g., "0x0002fde800000001").
// The hex string represents the raw 8-byte wire format.
// RFC 4360: Extended communities are 8 bytes (type + subtype + 6 bytes value).
func parseExtCommunityHex(s string) ([]byte, error) {
	// Strip 0x/0X prefix
	hexStr := strings.TrimPrefix(s, "0x")
	hexStr = strings.TrimPrefix(hexStr, "0X")

	// Must be exactly 16 hex chars (8 bytes for extended community)
	if len(hexStr) != 16 {
		return nil, fmt.Errorf("invalid extended-community %q: hex format must be 16 chars (8 bytes)", s)
	}

	// Decode hex to bytes
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community %q: %w", s, err)
	}

	return raw, nil
}

// fourByteASExtCommunity builds the four-octet AS specific extended community
// of RFC 5668 Section 2: a 4-octet global administrator that holds the AS
// number, and a 2-octet local administrator that holds numStr.
//
// The local administrator is two octets wide, and no form in RFC 4360 or RFC
// 5668 carries a four-byte AS beside a four-octet number, so a number above
// 65535 is refused here rather than truncated onto the wire.
//
// Every route that carries a four-byte AS is written here, so the limit is
// stated once. RFC 4360 Section 3.2 gives type 0x01 an IPv4 unicast address in
// its global administrator, which is why an AS number never goes in that form.
func fourByteASExtCommunity(subtype byte, asn uint64, numStr string) ([]byte, error) {
	num, err := strconv.ParseUint(numStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community number %q (4-byte ASN format max 65535)", numStr)
	}
	return []byte{
		ecTypeTransitive4ByteAS, subtype,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(num >> 8), byte(num),
	}, nil
}

// parseRouteTargetOrOrigin parses target:ASN:NN or origin:ASN:NN format.
// Also supports target:IP:NN and target:ASN:IP, and the unqualified ASN:NN,
// IP:NN and ASN:IP pairs, which carry the route target subtype.
//
// The administrator widths decide the type. RFC 4360 Section 3.1 gives type
// 0x00 a 2-octet AS and a 4-octet number; Section 3.2 gives type 0x01 an IPv4
// unicast address and a 2-octet number; RFC 5668 Section 2 gives type 0x02 a
// 4-octet AS and a 2-octet number. Nothing carries a 4-octet AS beside a
// 4-octet number, so that pair is refused.
func parseRouteTargetOrOrigin(subtype byte, asnStr, numStr string) ([]byte, error) {
	// Check if ASN part is an IP address (format: IP:NN)
	if ip, err := netip.ParseAddr(asnStr); err == nil && ip.Is4() {
		// Type 1: IPv4 address
		num, err := strconv.ParseUint(numStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid extended-community number %q", numStr)
		}
		b := ip.As4()
		return []byte{
			ecTypeTransitiveIPv4, subtype,
			b[0], b[1], b[2], b[3],
			byte(num >> 8), byte(num),
		}, nil
	}

	asn, forced4Byte, err := parseExtCommunityASN(asnStr)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
	}

	// "L" suffix forces Type 2 (4-byte ASN, 2-byte number)
	if forced4Byte {
		return fourByteASExtCommunity(subtype, asn, numStr)
	}

	// Check if number part is an IP address (format: ASN:IP -> convert IP to uint32)
	if ip, err := netip.ParseAddr(numStr); err == nil && ip.Is4() {
		b := ip.As4()
		num := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		if asn <= 0xFFFF {
			// Type 0: 2-byte ASN, 4-byte number (from IP)
			return []byte{
				ecTypeTransitive2ByteAS, subtype,
				byte(asn >> 8), byte(asn),
				byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
			}, nil
		}
		// 4-byte ASN not valid with 4-byte IP
		return nil, err4ByteAsnWithIpValue
	}

	if asn > 0xFFFF {
		// RFC 5668 Section 2: type 0x02 carries the AS number in a 4-octet
		// global administrator. Type 0x01 cannot: RFC 4360 Section 3.2 puts
		// "an IPv4 unicast address assigned by one of the Internet
		// registries" in its global administrator, so a peer reads an AS
		// number written there as a dotted quad.
		return fourByteASExtCommunity(subtype, asn, numStr)
	}

	// RFC 4360 Section 3.1: type 0x00, 2-byte ASN, 4-byte number.
	num, err := strconv.ParseUint(numStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community number %q", numStr)
	}
	return []byte{
		ecTypeTransitive2ByteAS, subtype,
		byte(asn >> 8), byte(asn),
		byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
	}, nil
}

// parseMUPExtCommunity parses mup:ASN:NN format.
// MUP Extended Community uses type 0x0C (Generic Transitive Experimental Use).
// Wire format: type(1) subtype(1) global-admin(2) local-admin(4)
// For mup:10:10 -> 0x0C 0x00 0x000A 0x0000000A.
func parseMUPExtCommunity(asnStr, numStr string) ([]byte, error) {
	asn, err := strconv.ParseUint(asnStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid mup extended-community ASN %q (must be 16-bit)", asnStr)
	}
	num, err := strconv.ParseUint(numStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid mup extended-community number %q", numStr)
	}

	// Type 0x0C: Generic Transitive Experimental Use
	// Subtype 0x00: MUP Extended Community
	// Format: 2-byte global-admin (ASN) + 4-byte local-admin (number)
	return []byte{
		0x0C, 0x00, // Type + Subtype
		byte(asn >> 8), byte(asn),
		byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
	}, nil
}

// parseRouteTargetOrOrigin4 parses target4:ASN:NN or origin4:ASN:NN format.
// It always writes the four-octet AS specific form of RFC 5668 Section 2, type
// 0x02, whatever the width of the AS number: that is what the "4" in the
// keyword asks for.
//
// RFC 5668 Section 3 tells an operator with a two-octet AS number to prefer the
// type 0x00 form, and "target:" gives that form. "target4:" is the operator
// asking for the four-octet form on purpose.
func parseRouteTargetOrOrigin4(subtype byte, asnStr, numStr string) ([]byte, error) {
	asn, _, err := parseExtCommunityASN(asnStr)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
	}
	return fourByteASExtCommunity(subtype, asn, numStr)
}

// parseFlowSpecRateLimit parses rate-limit:N and rate-limit:N:packets formats for FlowSpec.
// RFC 8955 Section 7.1: Traffic Rate extended community: type 0x80, subtype 0x06.
// RFC 8955 Section 7.2: Traffic Rate Packets extended community: type 0x80, subtype 0x0c.
// Value is a 4-byte IEEE float for rate in bytes/second or packets/second.
func parseFlowSpecRateLimit(rateStr string) ([]byte, error) {
	rate, unit, hasUnit := strings.Cut(rateStr, ":")
	if hasUnit {
		switch unit {
		case flowSpecPacketsUnit:
			return parseFlowSpecRateLimitWithSubtype(rate, attribute.FlowSpecRatePackets)
		case flowSpecBytesUnit:
			return parseFlowSpecRateLimitWithSubtype(rate, attribute.FlowSpecRateBytes)
		default:
			return nil, fmt.Errorf("unknown rate-limit unit %q", unit)
		}
	}
	return parseFlowSpecRateLimitWithSubtype(rate, attribute.FlowSpecRateBytes)
}

// parseFlowSpecRateLimitWithSubtype builds one traffic-rate community. The
// 2-octet id stays zero: a config rate-limit names no AS, and RFC 8955
// Section 7.1 calls the field "purely informational".
func parseFlowSpecRateLimitWithSubtype(rateStr string, unit attribute.FlowSpecRateUnit) ([]byte, error) {
	rate, err := strconv.ParseFloat(rateStr, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: %w", flowSpecRateLimitName, rateStr, err)
	}

	ec, err := attribute.FlowSpecTrafficRate(unit, 0, rate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", flowSpecRateLimitName, err)
	}
	return ec[:], nil
}

// parseFlowSpecMark parses the DSCP of a `mark N` action into the traffic-marking
// extended community (RFC 8955 Section 7.5).
//
// The parse error is returned rather than discarded. It used to be dropped, and
// strconv answers MaxUint64 on a range error, so `mark 300` wrote 0xff into the
// value and `mark abc` wrote 0x00, remarking every matching packet to
// best-effort. The DSCP bound itself lives in the shared encoder, which every
// FlowSpec caller now reaches.
func parseFlowSpecMark(dscpStr string) (attribute.ExtendedCommunity, error) {
	dscp, err := strconv.ParseUint(dscpStr, 10, 8)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid DSCP %q in mark: %w", dscpStr, err)
	}
	return attribute.FlowSpecTrafficMarking(dscp)
}

// parseFlowSpecRedirectNextHop parses the address of a `redirect-to-nexthop IP`
// action into the community its address family needs.
//
// It returns the 8-octet community for an IPv4 address and the 20-octet one for
// an IPv6 address, and exactly one of the two is ever non-empty. They travel in
// different attributes: 16 for the first, 25 for the second (RFC 5701).
func parseFlowSpecRedirectNextHop(ipStr string) (ipv4Bytes, ipv6Bytes []byte, err error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid %s address %q: %w", flowSpecRedirectNextHop, ipStr, err)
	}

	if ip.Is4() {
		ec, err := attribute.FlowSpecRedirectToIPv4(ip)
		if err != nil {
			return nil, nil, err
		}
		return ec[:], nil, nil
	}

	ec, err := attribute.FlowSpecRedirectToIPv6(ip)
	if err != nil {
		return nil, nil, err
	}
	return nil, ec[:], nil
}

// parseFlowSpecRedirect parses redirect:ADMIN:VALUE for FlowSpec
// (RFC 8955 Section 7.4). ADMIN is an AS number or an IPv4 address, and it
// selects which of the section's three encodings the community takes.
func parseFlowSpecRedirect(adminStr, numStr string) ([]byte, error) {
	ec, err := attribute.FlowSpecRedirect(adminStr, numStr)
	if err != nil {
		return nil, err
	}
	return ec[:], nil
}

// parseExtCommunityASN parses an ASN string that may have an "L" suffix forcing 4-byte encoding.
// Returns the parsed ASN value and whether 4-byte encoding was explicitly requested.
// The "L" suffix forces Type 2 (4-byte AS, RFC 5668) wire format regardless of ASN value.
func parseExtCommunityASN(s string) (uint64, bool, error) {
	forced := false
	if strings.HasSuffix(s, "L") || strings.HasSuffix(s, "l") {
		s = s[:len(s)-1]
		forced = true
	}
	asn, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false, err
	}
	return asn, forced, nil
}

func (ec ExtendedCommunity) String() string {
	return ec.Raw
}

// Values returns individual community values.
func (ec ExtendedCommunity) Values() []string {
	if ec.Raw == "" {
		return nil
	}
	return strings.Fields(ec.Raw)
}
