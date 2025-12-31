package config

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/exa-networks/zebgp/pkg/parse"
)

// Origin represents the ORIGIN path attribute.
// RFC 4271: 0=IGP, 1=EGP, 2=INCOMPLETE.
type Origin uint8

const (
	OriginIGP        Origin = 0
	OriginEGP        Origin = 1
	OriginIncomplete Origin = 2
)

// ParseOrigin parses an origin string (igp, egp, incomplete).
func ParseOrigin(s string) (Origin, error) {
	switch strings.ToLower(s) {
	case "", "igp":
		return OriginIGP, nil
	case "egp":
		return OriginEGP, nil
	case "incomplete":
		return OriginIncomplete, nil
	default:
		return 0, fmt.Errorf("invalid origin %q: valid values are igp, egp, incomplete", s)
	}
}

func (o Origin) String() string {
	switch o {
	case OriginIGP:
		return "igp"
	case OriginEGP:
		return "egp"
	case OriginIncomplete:
		return "incomplete"
	default:
		return fmt.Sprintf("unknown(%d)", o)
	}
}

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
// Delegates to parse.Community for shared logic.
func parseOneCommunity(s string) (uint32, error) {
	return parse.Community(s)
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
// Delegates to parse.LargeCommunity for shared logic.
func parseOneLargeCommunity(s string) ([3]uint32, error) {
	return parse.LargeCommunity(s)
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

	for _, p := range parts {
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
//   - Generic format: ASN:NN, IP:NN
func parseOneExtCommunity(s string) ([]byte, error) {
	// Check for hex format (0x prefix, no colons) - ExaBGP compatible
	if (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) && !strings.Contains(s, ":") {
		return parseExtCommunityHex(s)
	}

	// Format: [type:]value1:value2
	parts := strings.Split(s, ":")

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
		default:
			return nil, fmt.Errorf("unknown extended-community type %q", parts[0])
		}
	}

	if len(parts) == 5 && parts[0] == "l2info" {
		// Layer 2 Info Extended Community (RFC 4761): l2info:encaps:control:mtu:preference
		// Wire format: 0x800A | encaps(1) | control(1) | mtu(2) | preference(2)
		encaps, err := strconv.ParseUint(parts[1], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid l2info encaps %q", parts[1])
		}
		control, err := strconv.ParseUint(parts[2], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid l2info control %q", parts[2])
		}
		mtu, err := strconv.ParseUint(parts[3], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid l2info mtu %q", parts[3])
		}
		preference, err := strconv.ParseUint(parts[4], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid l2info preference %q", parts[4])
		}
		return []byte{
			0x80, 0x0A, // Type: Layer 2 Info
			byte(encaps), byte(control),
			byte(mtu >> 8), byte(mtu),
			byte(preference >> 8), byte(preference),
		}, nil
	}

	return nil, fmt.Errorf("invalid extended-community %q: expected format like target:ASN:NN", s)
}

// parseExtCommunityHex parses hex format extended community (e.g., "0x0002fde800000001").
// This is ExaBGP-compatible: the hex string represents the raw 8-byte wire format.
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

	// Parse ASN part
	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
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

	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
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
	// This matches exabgp behavior which converts large ASNs to IP representation.
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
// For mup:10:10 -> 0x0C 0x00 0x000A 0x0000000A
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
	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community ASN %q", asnStr)
	}
	num, err := strconv.ParseUint(numStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid extended-community number %q", numStr)
	}

	// Use Type 1 (IPv4) format: 4-byte "IP" (really ASN), 2-byte number
	// This matches exabgp behavior which prefers IPv4 format for 4-byte representations
	return []byte{
		ecTypeTransitiveIPv4, subtype,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(num >> 8), byte(num),
	}, nil
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

// PathID represents an ADD-PATH path identifier.
// Valid range: 0-4294967295 (0 means not set).
type PathID uint32

// ParsePathID parses a path-information value.
// Can be a uint32 or an IPv4 address (converted to uint32).
func ParsePathID(s string) (PathID, error) {
	if s == "" {
		return 0, nil
	}
	// Try as integer first
	if n, err := strconv.ParseUint(s, 10, 32); err == nil {
		return PathID(n), nil
	}
	// Try as IPv4 address (ExaBGP format)
	if ip, err := netip.ParseAddr(s); err == nil && ip.Is4() {
		b := ip.As4()
		return PathID(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])), nil
	}
	return 0, fmt.Errorf("invalid path-information %q: expected integer or IPv4 address", s)
}

func (p PathID) String() string {
	if p == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(p), 10)
}

// MPLSLabel represents an MPLS label stack entry.
// Valid range: 0-1048575 (20 bits).
const (
	MPLSLabelMin = 0
	MPLSLabelMax = 1048575 // 2^20 - 1
)

type MPLSLabel uint32

// ParseMPLSLabel parses an MPLS label value.
func ParseMPLSLabel(s string) (MPLSLabel, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid label %q: %w", s, err)
	}
	if n > MPLSLabelMax {
		return 0, fmt.Errorf("invalid label %d: must be 0-%d", n, MPLSLabelMax)
	}
	return MPLSLabel(n), nil
}

func (l MPLSLabel) String() string {
	if l == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(l), 10)
}

// RouteDistinguisher represents an RD (RFC 4364).
// Formats: Type0 (ASN2:NN4), Type1 (IP:NN2), Type2 (ASN4:NN2).
type RouteDistinguisher struct {
	Raw   string  // Original string
	Bytes [8]byte // Wire-format (2-byte type + 6-byte value)
}

// RD types.
const (
	rdType0 = 0 // 2-byte ASN : 4-byte assigned
	rdType1 = 1 // 4-byte IP : 2-byte assigned
	rdType2 = 2 // 4-byte ASN : 2-byte assigned
)

// ParseRouteDistinguisher parses an RD string to wire format.
func ParseRouteDistinguisher(s string) (RouteDistinguisher, error) {
	if s == "" {
		return RouteDistinguisher{}, nil
	}

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return RouteDistinguisher{}, fmt.Errorf("invalid rd %q: expected format ASN:NN or IP:NN", s)
	}

	var rd RouteDistinguisher
	rd.Raw = s

	// Check if first part is an IP address
	if ip, err := netip.ParseAddr(parts[0]); err == nil && ip.Is4() {
		// Type 1: IP:NN
		num, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return RouteDistinguisher{}, fmt.Errorf("invalid rd number %q", parts[1])
		}
		b := ip.As4()
		rd.Bytes[0], rd.Bytes[1] = 0, rdType1
		copy(rd.Bytes[2:6], b[:])
		rd.Bytes[6], rd.Bytes[7] = byte(num>>8), byte(num)
		return rd, nil
	}

	// Parse as ASN:NN
	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return RouteDistinguisher{}, fmt.Errorf("invalid rd ASN %q", parts[0])
	}
	num, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return RouteDistinguisher{}, fmt.Errorf("invalid rd number %q", parts[1])
	}

	if asn <= 0xFFFF {
		// Type 0: 2-byte ASN, 4-byte number
		rd.Bytes[0], rd.Bytes[1] = 0, rdType0
		rd.Bytes[2], rd.Bytes[3] = byte(asn>>8), byte(asn)
		rd.Bytes[4] = byte(num >> 24)
		rd.Bytes[5] = byte(num >> 16)
		rd.Bytes[6] = byte(num >> 8)
		rd.Bytes[7] = byte(num)
	} else {
		// Type 2: 4-byte ASN, 2-byte number
		rd.Bytes[0], rd.Bytes[1] = 0, rdType2
		rd.Bytes[2] = byte(asn >> 24)
		rd.Bytes[3] = byte(asn >> 16)
		rd.Bytes[4] = byte(asn >> 8)
		rd.Bytes[5] = byte(asn)
		rd.Bytes[6], rd.Bytes[7] = byte(num>>8), byte(num)
	}

	return rd, nil
}

func (rd RouteDistinguisher) String() string {
	return rd.Raw
}

// IsZero returns true if the RD is empty/unset.
func (rd RouteDistinguisher) IsZero() bool {
	return rd.Raw == ""
}

// ASPath represents the AS_PATH attribute (RFC 4271).
type ASPath struct {
	Raw    string   // Original string (e.g., "[ 57821 6939 ]")
	Values []uint32 // Parsed AS numbers
}

// ParseASPath parses an AS_PATH from ExaBGP format: "[ ASN1 ASN2 ... ]".
func ParseASPath(s string) (ASPath, error) {
	asp := ASPath{Raw: s}
	if s == "" {
		return asp, nil
	}

	// Remove brackets and trim whitespace
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	if s == "" {
		return asp, nil
	}

	// Split by whitespace
	parts := strings.Fields(s)
	asp.Values = make([]uint32, 0, len(parts))

	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return asp, fmt.Errorf("invalid AS number %q: %w", p, err)
		}
		asp.Values = append(asp.Values, uint32(n))
	}

	return asp, nil
}

// IsZero returns true if AS_PATH is empty/unset.
func (asp ASPath) IsZero() bool {
	return len(asp.Values) == 0
}

// Aggregator represents the AGGREGATOR attribute (RFC 4271).
// Format: ( ASN:IP ) e.g., ( 18144:219.118.225.189 ).
type Aggregator struct {
	Raw   string  // Original string
	ASN   uint32  // Aggregator AS
	IP    [4]byte // Aggregator IP (IPv4 only per RFC)
	Valid bool    // Whether aggregator is set
}

// ParseAggregator parses an AGGREGATOR from ExaBGP format: "( ASN:IP )".
func ParseAggregator(s string) (Aggregator, error) {
	agg := Aggregator{Raw: s}
	if s == "" {
		return agg, nil
	}

	// Remove parentheses and trim whitespace
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	s = strings.TrimSpace(s)

	if s == "" {
		return agg, nil
	}

	// Split by colon - format is ASN:IP
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return agg, fmt.Errorf("invalid aggregator format %q: expected ASN:IP", s)
	}

	// Parse ASN
	asn, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
	if err != nil {
		return agg, fmt.Errorf("invalid aggregator ASN %q: %w", parts[0], err)
	}
	agg.ASN = uint32(asn)

	// Parse IP
	ip, err := netip.ParseAddr(strings.TrimSpace(parts[1]))
	if err != nil {
		return agg, fmt.Errorf("invalid aggregator IP %q: %w", parts[1], err)
	}
	if !ip.Is4() {
		return agg, fmt.Errorf("aggregator IP must be IPv4: %s", ip)
	}
	agg.IP = ip.As4()
	agg.Valid = true

	return agg, nil
}

// IsZero returns true if aggregator is not set.
func (agg Aggregator) IsZero() bool {
	return !agg.Valid
}

// RawAttribute represents a raw BGP path attribute from config.
// Format: [ code flags value_hex ].
type RawAttribute struct {
	Code  uint8
	Flags uint8
	Value []byte
}

// ParseRawAttribute parses attribute syntax: "0xNN 0xNN 0xHEXVALUE".
func ParseRawAttribute(s string) (RawAttribute, error) {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return RawAttribute{}, fmt.Errorf("raw attribute needs at least code and flags")
	}

	// Parse attribute type code
	code, err := parseHexByte(parts[0])
	if err != nil {
		return RawAttribute{}, fmt.Errorf("invalid attribute code %q: %w", parts[0], err)
	}

	// Parse flags
	flags, err := parseHexByte(parts[1])
	if err != nil {
		return RawAttribute{}, fmt.Errorf("invalid attribute flags %q: %w", parts[1], err)
	}

	// Parse value (optional, may be empty)
	var value []byte
	if len(parts) > 2 {
		value, err = parseHexBytes(parts[2])
		if err != nil {
			return RawAttribute{}, fmt.Errorf("invalid attribute value %q: %w", parts[2], err)
		}
	}

	return RawAttribute{
		Code:  code,
		Flags: flags,
		Value: value,
	}, nil
}

// parseHexByte parses "0xNN" format to a byte.
func parseHexByte(s string) (uint8, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	v, err := strconv.ParseUint(s, 16, 8)
	return uint8(v), err
}

// parseHexBytes parses "0xHEXHEXHEX..." format to bytes.
func parseHexBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return hex.DecodeString(s)
}

// PrefixSID represents BGP Prefix-SID (RFC 8669).
// Stores the wire-format TLV bytes for attribute type 40.
type PrefixSID struct {
	Bytes []byte // Wire-format TLV bytes (without attribute header)
}

// ParsePrefixSID parses a prefix-sid string.
// Formats:
//   - Simple: "777" → Label Index 777
//   - With SRGB: "300, [( 800000,4096) ,( 1000000,5000)]"
//
// RFC 8669 TLV format:
//   - Type (1 byte) + Length (2 bytes) + Value (variable)
//
// Label Index TLV (Type 1):
//   - Reserved (3 bytes) + Flags (1 byte) + Label-Index (3 bytes)
func ParsePrefixSID(s string) (PrefixSID, error) {
	if s == "" {
		return PrefixSID{}, nil
	}

	// Clean up the input
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")
	s = strings.TrimSpace(s)

	// Check for SRGB list (contains parentheses)
	if strings.Contains(s, "(") {
		return parsePrefixSIDWithSRGB(s)
	}

	// Simple label index
	idx, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return PrefixSID{}, fmt.Errorf("invalid prefix-sid label index %q: %w", s, err)
	}

	// RFC 8669: Label-Index is 24 bits (max 16777215)
	if idx > 0xFFFFFF {
		return PrefixSID{}, fmt.Errorf("prefix-sid label index %d exceeds 24-bit maximum (16777215)", idx)
	}

	// Build TLV for Label Index (Type 1)
	// RFC 8669 Section 4.1: Type(1) + Length(2) + Reserved(3) + Flags(1) + LabelIndex(3) = 10 bytes
	tlv := []byte{
		1,               // Type: Label Index
		0,               // Length high byte
		7,               // Length low byte (7 bytes value)
		0,               // Reserved byte 1
		0,               // Reserved byte 2
		0,               // Reserved byte 3
		0,               // Flags
		byte(idx >> 16), // Label Index (3 bytes, big-endian)
		byte(idx >> 8),
		byte(idx),
	}

	return PrefixSID{Bytes: tlv}, nil
}

// parsePrefixSIDWithSRGB parses format: "300, [( 800000,4096) ,( 1000000,5000)]".
//
// RFC 8669 TLV format:
//   - Type (1 byte) + Length (2 bytes) + Value (variable)
//
// SRGB TLV (Type 3):
//   - Flags (2 bytes) + SRGB descriptors (6 bytes each: Base(3) + Range(3))
func parsePrefixSIDWithSRGB(s string) (PrefixSID, error) {
	// Find the comma that separates label index from SRGB list
	// Format: "300, [( 800000,4096) ,( 1000000,5000)]"
	parts := strings.SplitN(s, ",", 2)
	if len(parts) < 2 {
		return PrefixSID{}, fmt.Errorf("invalid prefix-sid format: expected 'index, [(base,range)...]'")
	}

	// Parse label index
	idxStr := strings.TrimSpace(parts[0])
	idx, err := strconv.ParseUint(idxStr, 10, 32)
	if err != nil {
		return PrefixSID{}, fmt.Errorf("invalid prefix-sid label index %q: %w", idxStr, err)
	}

	// RFC 8669: Label-Index is 24 bits (max 16777215)
	if idx > 0xFFFFFF {
		return PrefixSID{}, fmt.Errorf("prefix-sid label index %d exceeds 24-bit maximum (16777215)", idx)
	}

	// Parse SRGB list
	srgbStr := parts[1]
	srgbs, err := parseSRGBList(srgbStr)
	if err != nil {
		return PrefixSID{}, err
	}

	// Build TLVs
	// Label Index TLV (Type 1)
	// Type(1) + Length(2) + Reserved(3) + Flags(1) + LabelIndex(3) = 10 bytes
	result := []byte{
		1,               // Type: Label Index
		0,               // Length high byte
		7,               // Length low byte (7 bytes value)
		0,               // Reserved byte 1
		0,               // Reserved byte 2
		0,               // Reserved byte 3
		0,               // Flags
		byte(idx >> 16), // Label Index (3 bytes, big-endian)
		byte(idx >> 8),
		byte(idx),
	}

	// SRGB TLV (Type 3) if we have SRGBs
	// Type(1) + Length(2) + Flags(2) + entries(6 each)
	if len(srgbs) > 0 {
		valueLen := 2 + len(srgbs)*6 // Flags(2) + entries
		srgbTLV := make([]byte, 3+valueLen)
		srgbTLV[0] = 3                   // Type: Originator SRGB
		srgbTLV[1] = byte(valueLen >> 8) // Length high byte
		srgbTLV[2] = byte(valueLen)      // Length low byte
		srgbTLV[3] = 0                   // Flags high byte
		srgbTLV[4] = 0                   // Flags low byte
		for i, entry := range srgbs {
			offset := 5 + i*6
			// Base (3 bytes, big-endian)
			srgbTLV[offset+0] = byte(entry.Base >> 16)
			srgbTLV[offset+1] = byte(entry.Base >> 8)
			srgbTLV[offset+2] = byte(entry.Base)
			// Range (3 bytes, big-endian)
			srgbTLV[offset+3] = byte(entry.Range >> 16)
			srgbTLV[offset+4] = byte(entry.Range >> 8)
			srgbTLV[offset+5] = byte(entry.Range)
		}
		result = append(result, srgbTLV...)
	}

	return PrefixSID{Bytes: result}, nil
}

// srgbEntry represents a single SRGB base,range pair.
type srgbEntry struct {
	Base  uint32
	Range uint32
}

// parseSRGBList parses "[( 800000,4096) ,( 1000000,5000)]" format.
func parseSRGBList(s string) ([]srgbEntry, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")

	var entries []srgbEntry

	// Find all (base,range) pairs
	for {
		start := strings.Index(s, "(")
		if start == -1 {
			break
		}
		end := strings.Index(s, ")")
		if end == -1 {
			return nil, fmt.Errorf("unmatched parenthesis in SRGB list")
		}

		pair := s[start+1 : end]
		parts := strings.Split(pair, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid SRGB pair %q: expected (base,range)", pair)
		}

		base, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid SRGB base %q: %w", parts[0], err)
		}
		// RFC 8669: SRGB base is 24 bits (max 16777215)
		if base > 0xFFFFFF {
			return nil, fmt.Errorf("SRGB base %d exceeds 24-bit maximum (16777215)", base)
		}

		rng, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid SRGB range %q: %w", parts[1], err)
		}
		// RFC 8669: SRGB range is 24 bits (max 16777215)
		if rng > 0xFFFFFF {
			return nil, fmt.Errorf("SRGB range %d exceeds 24-bit maximum (16777215)", rng)
		}

		entries = append(entries, srgbEntry{Base: uint32(base), Range: uint32(rng)})
		s = s[end+1:]
	}

	return entries, nil
}

// ParsePrefixSIDSRv6 parses SRv6 Prefix-SID format.
// Formats (ExaBGP compatible):
//   - "l3-service IPv6"
//   - "l3-service IPv6 behavior"
//   - "l3-service IPv6 behavior[struct]"
//   - "(l3-service IPv6 behavior[struct])"
//   - "l2-service IPv6 ..." (same variants)
//
// Where:
//   - IPv6 = 16-byte SRv6 SID address
//   - behavior = hex value like 0x48 (optional, default 0)
//   - struct = [LB,LN,Func,Arg,TransLen,TransOffset] (optional)
//
// RFC 9252 defines the wire format for SRv6-VPN SID.
func ParsePrefixSIDSRv6(s string) (PrefixSID, error) {
	if s == "" {
		return PrefixSID{}, nil
	}

	// Clean up input - remove outer parentheses
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = s[1 : len(s)-1]
		s = strings.TrimSpace(s)
	}

	// Parse service type
	var serviceType byte
	if strings.HasPrefix(s, "l3-service") {
		serviceType = 5 // TLV Type 5: SRv6 L3 Service
		s = strings.TrimPrefix(s, "l3-service")
	} else if strings.HasPrefix(s, "l2-service") {
		serviceType = 6 // TLV Type 6: SRv6 L2 Service
		s = strings.TrimPrefix(s, "l2-service")
	} else {
		return PrefixSID{}, fmt.Errorf("invalid srv6 prefix-sid: expected l3-service or l2-service")
	}
	s = strings.TrimSpace(s)

	// Parse IPv6 address (required)
	var ipv6 netip.Addr
	var behavior byte
	var sidStruct []byte

	// Find end of IPv6 address (space, 0x, or [)
	ipEnd := len(s)
	for i, c := range s {
		if c == ' ' || c == '[' {
			ipEnd = i
			break
		}
		if i > 0 && strings.HasPrefix(s[i:], "0x") {
			ipEnd = i
			break
		}
	}

	ipStr := strings.TrimSpace(s[:ipEnd])
	var err error
	ipv6, err = netip.ParseAddr(ipStr)
	if err != nil || !ipv6.Is6() {
		return PrefixSID{}, fmt.Errorf("invalid srv6 prefix-sid: expected IPv6 address, got %q", ipStr)
	}
	s = strings.TrimSpace(s[ipEnd:])

	// Parse optional behavior (0xNN format)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		// Find end of behavior value
		behEnd := len(s)
		for i := 2; i < len(s); i++ {
			c := s[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				behEnd = i
				break
			}
		}
		behStr := s[2:behEnd]
		behVal, err := strconv.ParseUint(behStr, 16, 8)
		if err != nil {
			return PrefixSID{}, fmt.Errorf("invalid srv6 behavior %q: %w", s[:behEnd], err)
		}
		behavior = byte(behVal)
		s = strings.TrimSpace(s[behEnd:])
	}

	// Parse optional SID structure [LB,LN,Func,Arg,TransLen,TransOffset]
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end == -1 {
			return PrefixSID{}, fmt.Errorf("invalid srv6 prefix-sid: unmatched [ in SID structure")
		}
		structStr := s[1:end]
		parts := strings.Split(structStr, ",")
		if len(parts) != 6 {
			return PrefixSID{}, fmt.Errorf("invalid srv6 SID structure: expected 6 values, got %d", len(parts))
		}
		for _, p := range parts {
			v, err := strconv.ParseUint(strings.TrimSpace(p), 10, 8)
			if err != nil {
				return PrefixSID{}, fmt.Errorf("invalid srv6 SID structure value %q: %w", p, err)
			}
			sidStruct = append(sidStruct, byte(v))
		}
	}

	// Build wire format per RFC 9252
	// Outer TLV: Type 5/6 (L3/L2 Service)
	//   Inner Sub-TLV: Type 1 (SRv6 SID Information)
	//     Value: reserved(1) + SID(16) + flags(1) + behavior(1) + [optional sub-sub-TLV]
	//       Optional sub-sub-TLV: Type 1 (SRv6 SID Structure, 6 bytes)

	// Build inner sub-TLV value (Type 1: SRv6 SID Information)
	// RFC 9252 format: reserved(1) + SID(16) + flags(1) + reserved(1) + behavior(1) + [sub-TLVs]
	var innerValue []byte
	innerValue = append(innerValue, 0) // reserved
	innerValue = append(innerValue, ipv6.AsSlice()...)
	innerValue = append(innerValue, 0)        // flags
	innerValue = append(innerValue, 0)        // reserved
	innerValue = append(innerValue, behavior) // behavior

	// Add SID structure sub-sub-TLV if provided
	if len(sidStruct) == 6 {
		innerValue = append(innerValue, 0, 1)                    // sub-sub-TLV type 1
		innerValue = append(innerValue, 0, byte(len(sidStruct))) // length
		innerValue = append(innerValue, sidStruct...)
	}

	// Build inner sub-TLV header (Type 1)
	innerLen := len(innerValue)
	innerTLV := []byte{0, 1, byte(innerLen >> 8), byte(innerLen)}
	innerTLV = append(innerTLV, innerValue...)

	// Build outer TLV header (Type 5 or 6)
	outerLen := len(innerTLV)
	result := []byte{serviceType, byte(outerLen >> 8), byte(outerLen)}
	result = append(result, innerTLV...)

	return PrefixSID{Bytes: result}, nil
}

// ParsedRouteAttributes holds all parsed route attributes.
type ParsedRouteAttributes struct {
	Prefix            netip.Prefix
	NextHop           netip.Addr
	Origin            Origin
	LocalPreference   uint32
	MED               uint32
	Community         Community
	ExtendedCommunity ExtendedCommunity
	LargeCommunity    LargeCommunity
	PathID            PathID
	Label             MPLSLabel
	RD                RouteDistinguisher
	ASPath            ASPath
	Aggregator        Aggregator
	AtomicAggregate   bool
	OriginatorID      uint32   // RFC 4456
	ClusterList       []uint32 // RFC 4456
	PrefixSID         PrefixSID
	RawAttributes     []RawAttribute
}

// ParseRouteAttributes parses all attributes from a StaticRouteConfig.
func ParseRouteAttributes(src StaticRouteConfig) (*ParsedRouteAttributes, error) {
	attrs := &ParsedRouteAttributes{
		Prefix:          src.Prefix,
		LocalPreference: src.LocalPreference,
		MED:             src.MED,
	}

	// NextHop
	if src.NextHop != "" && src.NextHop != "self" {
		ip, err := netip.ParseAddr(src.NextHop)
		if err != nil {
			return nil, fmt.Errorf("invalid next-hop %q: %w", src.NextHop, err)
		}
		attrs.NextHop = ip
	}

	// Origin
	origin, err := ParseOrigin(src.Origin)
	if err != nil {
		return nil, err
	}
	attrs.Origin = origin

	// Community (RFC 1997)
	comm, err := ParseCommunity(src.Community)
	if err != nil {
		return nil, err
	}
	attrs.Community = comm

	// Extended Community
	ec, err := ParseExtendedCommunity(src.ExtendedCommunity)
	if err != nil {
		return nil, err
	}
	attrs.ExtendedCommunity = ec

	// Large Community (RFC 8092)
	lc, err := ParseLargeCommunity(src.LargeCommunity)
	if err != nil {
		return nil, err
	}
	attrs.LargeCommunity = lc

	// Path ID
	pid, err := ParsePathID(src.PathInformation)
	if err != nil {
		return nil, err
	}
	attrs.PathID = pid

	// Label
	label, err := ParseMPLSLabel(src.Label)
	if err != nil {
		return nil, err
	}
	attrs.Label = label

	// RD
	rd, err := ParseRouteDistinguisher(src.RD)
	if err != nil {
		return nil, err
	}
	attrs.RD = rd

	// AS_PATH
	asp, err := ParseASPath(src.ASPath)
	if err != nil {
		return nil, err
	}
	attrs.ASPath = asp

	// Aggregator
	agg, err := ParseAggregator(src.Aggregator)
	if err != nil {
		return nil, err
	}
	attrs.Aggregator = agg

	// Atomic Aggregate
	attrs.AtomicAggregate = src.AtomicAggregate

	// Originator ID (RFC 4456)
	if src.OriginatorID != "" {
		ip, err := netip.ParseAddr(src.OriginatorID)
		if err != nil {
			return nil, fmt.Errorf("invalid originator-id %q: %w", src.OriginatorID, err)
		}
		if ip.Is4() {
			b := ip.As4()
			attrs.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Cluster List (RFC 4456)
	if src.ClusterList != "" {
		clStr := strings.TrimSpace(src.ClusterList)
		clStr = strings.TrimPrefix(clStr, "[")
		clStr = strings.TrimSuffix(clStr, "]")
		for _, p := range strings.Fields(clStr) {
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return nil, fmt.Errorf("invalid cluster-list entry %q: %w", p, err)
			}
			if ip.Is4() {
				b := ip.As4()
				attrs.ClusterList = append(attrs.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	// BGP Prefix-SID (RFC 8669)
	if src.PrefixSID != "" {
		sid, err := ParsePrefixSID(src.PrefixSID)
		if err != nil {
			return nil, err
		}
		attrs.PrefixSID = sid
	}

	// Raw Attributes (hex format: "0xNN 0xNN 0xVALUE")
	// Known attribute codes are converted to typed fields.
	if src.Attribute != "" {
		raw, err := ParseRawAttribute(src.Attribute)
		if err != nil {
			return nil, fmt.Errorf("invalid raw attribute: %w", err)
		}

		// Convert known attribute types to proper fields
		switch raw.Code {
		case 4: // MED
			if len(raw.Value) >= 4 {
				attrs.MED = uint32(raw.Value[0])<<24 | uint32(raw.Value[1])<<16 |
					uint32(raw.Value[2])<<8 | uint32(raw.Value[3])
			}
		case 5: // LOCAL_PREF
			if len(raw.Value) >= 4 {
				attrs.LocalPreference = uint32(raw.Value[0])<<24 | uint32(raw.Value[1])<<16 |
					uint32(raw.Value[2])<<8 | uint32(raw.Value[3])
			}
		case 32: // LARGE_COMMUNITY
			// Parse 12-byte tuples
			for i := 0; i+12 <= len(raw.Value); i += 12 {
				lc := [3]uint32{
					uint32(raw.Value[i])<<24 | uint32(raw.Value[i+1])<<16 |
						uint32(raw.Value[i+2])<<8 | uint32(raw.Value[i+3]),
					uint32(raw.Value[i+4])<<24 | uint32(raw.Value[i+5])<<16 |
						uint32(raw.Value[i+6])<<8 | uint32(raw.Value[i+7]),
					uint32(raw.Value[i+8])<<24 | uint32(raw.Value[i+9])<<16 |
						uint32(raw.Value[i+10])<<8 | uint32(raw.Value[i+11]),
				}
				attrs.LargeCommunity.Values = append(attrs.LargeCommunity.Values, lc)
			}
		default:
			// Unknown attribute - keep as raw
			attrs.RawAttributes = append(attrs.RawAttributes, raw)
		}
	}

	return attrs, nil
}
