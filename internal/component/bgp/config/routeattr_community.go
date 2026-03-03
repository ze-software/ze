// Design: docs/architecture/config/syntax.md — community attribute parsing
// Overview: routeattr.go — core route attribute types

package bgpconfig

import (
	"encoding/hex"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
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
type ExtendedCommunity struct {
	Raw   string // Original string for encoding
	Bytes []byte // Wire-format bytes (8 bytes per community)
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

	// Process parts, looking ahead for two-word formats (action, mark, redirect-to-nexthop).
	for i := 0; i < len(parts); i++ {
		p := parts[i]

		// Check for two-word formats that need the next part
		if i+1 < len(parts) {
			switch p {
			case "action":
				// "action sample-terminal" or "action sample" or "action terminal"
				b := parseFlowSpecAction(parts[i+1])
				allBytes = append(allBytes, b...)
				i++ // skip next part
				continue
			case "mark":
				// "mark N" - DSCP marking
				b := parseFlowSpecMark(parts[i+1])
				allBytes = append(allBytes, b...)
				i++ // skip next part
				continue
			case "redirect-to-nexthop":
				// "redirect-to-nexthop IP" - RFC 7674 Section 3.1
				b := parseFlowSpecRedirectNextHop(parts[i+1])
				if b != nil {
					allBytes = append(allBytes, b...)
					i++ // skip next part
					continue
				}
				// If parsing failed, fall through to single-word handling
			}
		}

		// Single-word extended community
		b, err := parseOneExtCommunity(p)
		if err != nil {
			return ExtendedCommunity{}, err
		}
		allBytes = append(allBytes, b...)
	}

	return ExtendedCommunity{Raw: s, Bytes: allBytes}, nil
}

// parseOneExtCommunity parses a single extended community string to 8 bytes.
// Supports formats:
//   - Hex format: 0x0002fde800000001 (16 hex chars = 8 bytes wire format)
//   - Named format: target:ASN:NN, origin:ASN:NN
//   - FlowSpec actions: rate-limit:N, redirect-to-nexthop-draft, copy-to-nexthop, mark N
//   - Generic format: ASN:NN, IP:NN
func parseOneExtCommunity(s string) ([]byte, error) {
	// Check for hex format (0x prefix, no colons)
	if (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) && !strings.Contains(s, ":") {
		return parseExtCommunityHex(s)
	}

	// FlowSpec single-word actions (no colons).
	switch s {
	case "redirect-to-nexthop-draft":
		// Pre-IETF draft: Redirect to next-hop (type 0x08, subtype 0x00).
		return []byte{0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
	case "copy-to-nexthop":
		// RFC 5575bis: Copy and redirect to next-hop (type 0x08, subtype 0x00, value 1).
		return []byte{0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}, nil
	case "discard":
		// RFC 8955 Section 7.3: Traffic-rate 0 = discard (type 0x8006).
		return []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
	}

	// Format: [type:]value1:value2
	parts := strings.Split(s, ":")

	// FlowSpec rate-limit:N format.
	if len(parts) == 2 && parts[0] == "rate-limit" {
		return parseFlowSpecRateLimit(parts[1])
	}

	if len(parts) == 2 {
		// Generic format: ASN:NN or ASN:IP
		return parseGenericExtCommunity(parts[0], parts[1])
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

// parseGenericExtCommunity parses ASN:Value format (type 0x00, subtype from context).
// Supports formats: ASN:NN, IP:NN, ASN:IP (where IP is converted to uint32).
func parseGenericExtCommunity(asnStr, valStr string) ([]byte, error) {
	// Check if first part is an IP address (format: IP:NN)
	if ip, err := netip.ParseAddr(asnStr); err == nil && ip.Is4() {
		num, err := strconv.ParseUint(valStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid extended-community number %q", valStr)
		}
		b := ip.As4()
		return []byte{
			ecTypeTransitiveIPv4, ecSubtypeRouteTarget,
			b[0], b[1], b[2], b[3],
			byte(num >> 8), byte(num),
		}, nil
	}

	// Parse ASN part (supports "L" suffix for forced 4-byte encoding)
	asn, forced4Byte, err := parseExtCommunityASN(asnStr)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
	}

	// "L" suffix forces Type 2 (4-byte ASN, 2-byte number)
	if forced4Byte {
		num, err := strconv.ParseUint(valStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid extended-community number %q (4-byte ASN format max 65535)", valStr)
		}
		return []byte{
			ecTypeTransitive4ByteAS, ecSubtypeRouteTarget,
			byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
			byte(num >> 8), byte(num),
		}, nil
	}

	// Check if value is an IP address (format: ASN:IP, IP converted to uint32)
	if ip, err := netip.ParseAddr(valStr); err == nil && ip.Is4() {
		b := ip.As4()
		num := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		if asn <= 0xFFFF {
			// 2-byte ASN, 4-byte number (from IP)
			return []byte{
				ecTypeTransitive2ByteAS, ecSubtypeRouteTarget,
				byte(asn >> 8), byte(asn),
				byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
			}, nil
		}
		// 4-byte ASN, 2-byte number (truncate IP to 16 bits)
		return []byte{
			ecTypeTransitive4ByteAS, ecSubtypeRouteTarget,
			byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
			byte(num >> 8), byte(num),
		}, nil
	}

	// Format: ASN:NN (both numeric)
	num, err := strconv.ParseUint(valStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community number %q", valStr)
	}

	if asn <= 0xFFFF {
		// 2-byte ASN, 4-byte number
		return []byte{
			ecTypeTransitive2ByteAS, ecSubtypeRouteTarget,
			byte(asn >> 8), byte(asn),
			byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
		}, nil
	}
	// 4-byte ASN, 2-byte number
	return []byte{
		ecTypeTransitive4ByteAS, ecSubtypeRouteTarget,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(num >> 8), byte(num),
	}, nil
}

// parseRouteTargetOrOrigin parses target:ASN:NN or origin:ASN:NN format.
// Also supports target:IP:NN and target:ASN:IP formats.
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
		return nil, fmt.Errorf("4-byte ASN with IP value not supported")
	}

	num, err := strconv.ParseUint(numStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community number %q", numStr)
	}

	if asn <= 0xFFFF {
		// Type 0: 2-byte ASN, 4-byte number
		return []byte{
			ecTypeTransitive2ByteAS, subtype,
			byte(asn >> 8), byte(asn),
			byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
		}, nil
	}

	// ASN > 65535: Use Type 1 (IPv4) format if number fits in 16 bits.
	if num <= 0xFFFF {
		return []byte{
			ecTypeTransitiveIPv4, subtype,
			byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
			byte(num >> 8), byte(num),
		}, nil
	}

	// Type 2: 4-byte ASN, 2-byte number (only when number > 65535)
	return []byte{
		ecTypeTransitive4ByteAS, subtype,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(num >> 8), byte(num),
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
// Uses 4-byte representation for ASN, with Type 1 (IPv4) format preferred.
func parseRouteTargetOrOrigin4(subtype byte, asnStr, numStr string) ([]byte, error) {
	asn, _, err := parseExtCommunityASN(asnStr)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
	}
	num, err := strconv.ParseUint(numStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community number %q", numStr)
	}

	// Use Type 1 (IPv4) format: 4-byte "IP" (really ASN), 2-byte number
	return []byte{
		ecTypeTransitiveIPv4, subtype,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(num >> 8), byte(num),
	}, nil
}

// parseFlowSpecRateLimit parses rate-limit:N format for FlowSpec (RFC 5575).
// Traffic Rate extended community: type 0x80, subtype 0x06.
// Value is a 4-byte IEEE float for rate in bytes/second.
func parseFlowSpecRateLimit(rateStr string) ([]byte, error) {
	rate, err := strconv.ParseFloat(rateStr, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid rate-limit value %q: %w", rateStr, err)
	}
	// Convert to IEEE 754 single-precision float (4 bytes)
	bits := math.Float32bits(float32(rate))
	return []byte{
		0x80, 0x06, // Type: Traffic Rate
		0x00, 0x00, // AS number (informational, usually 0)
		byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits),
	}, nil
}

// parseFlowSpecAction parses FlowSpec action flags (RFC 8955 Section 7.4).
// Traffic Action extended community: type 0x80, subtype 0x07.
// Format: "sample", "terminal", "sample-terminal" (hyphen or space separated).
func parseFlowSpecAction(flagsStr string) []byte {
	var flags uint8
	// Parse flags: sample (bit 1), terminal (bit 0)
	lower := strings.ToLower(flagsStr)
	if strings.Contains(lower, "sample") {
		flags |= 0x02 // Sample bit
	}
	if strings.Contains(lower, "terminal") {
		flags |= 0x01 // Terminal bit
	}
	return []byte{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, flags}
}

// parseFlowSpecMark parses DSCP marking value (RFC 8955 Section 7.6).
// Traffic Marking extended community: type 0x80, subtype 0x09.
func parseFlowSpecMark(dscpStr string) []byte {
	dscp, _ := strconv.ParseUint(dscpStr, 10, 8)
	return []byte{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, byte(dscp)}
}

// parseFlowSpecRedirectNextHop parses redirect-to-nexthop with IPv4 (RFC 7674 Section 3.1).
// Returns nil if the IP is invalid or IPv6 (IPv6 handled separately via attribute 25).
func parseFlowSpecRedirectNextHop(ipStr string) []byte {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil || !ip.Is4() {
		return nil // Invalid or IPv6 - handled elsewhere
	}
	ipBytes := ip.As4()
	return []byte{0x01, 0x0c, ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3], 0x00, 0x00}
}

// parseFlowSpecRedirect parses redirect:ASN:NN format for FlowSpec (RFC 5575).
func parseFlowSpecRedirect(asnStr, numStr string) ([]byte, error) {
	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect ASN %q", asnStr)
	}
	num, err := strconv.ParseUint(numStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect number %q", numStr)
	}

	if asn <= 0xFFFF {
		// Type 0x80 (non-transitive), subtype 0x08: 2-byte ASN, 4-byte value
		return []byte{
			0x80, 0x08,
			byte(asn >> 8), byte(asn),
			byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
		}, nil
	}
	// Type 0x82 (non-transitive 4-byte AS), subtype 0x08: 4-byte ASN, 2-byte value
	return []byte{
		0x82, 0x08,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(num >> 8), byte(num),
	}, nil
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
