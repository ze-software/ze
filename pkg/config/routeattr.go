package config

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
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
func parseOneCommunity(s string) (uint32, error) {
	// Check for well-known communities
	switch strings.ToLower(s) {
	case "no_export", "no-export":
		return 0xFFFFFF01, nil
	case "no_advertise", "no-advertise":
		return 0xFFFFFF02, nil
	case "no_export_subconfed", "no-export-subconfed":
		return 0xFFFFFF03, nil
	case "nopeer", "no-peer":
		return 0xFFFFFF04, nil
	}

	// Parse ASN:Value format
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		// Try as single integer
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid community %q: expected ASN:Value format", s)
		}
		return uint32(n), nil
	}

	asn, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community ASN %q", parts[0])
	}
	val, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community value %q", parts[1])
	}

	return uint32(asn)<<16 | uint32(val), nil
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
func parseOneLargeCommunity(s string) ([3]uint32, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return [3]uint32{}, fmt.Errorf("invalid large-community %q: expected GA:LD1:LD2 format", s)
	}

	ga, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return [3]uint32{}, fmt.Errorf("invalid large-community global-admin %q", parts[0])
	}
	ld1, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return [3]uint32{}, fmt.Errorf("invalid large-community local-data1 %q", parts[1])
	}
	ld2, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return [3]uint32{}, fmt.Errorf("invalid large-community local-data2 %q", parts[2])
	}

	return [3]uint32{uint32(ga), uint32(ld1), uint32(ld2)}, nil
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
func parseOneExtCommunity(s string) ([]byte, error) {
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
		case "mup":
			// MUP extended community: mup:ASN:NN
			return parseRouteTargetOrOrigin(0x0B, parts[1], parts[2]) // MUP subtype
		default:
			return nil, fmt.Errorf("unknown extended-community type %q", parts[0])
		}
	}

	if len(parts) == 5 && parts[0] == "l2info" {
		// Layer 2 Info Extended Community (RFC 4761): l2info:encaps:control:mtu:preference
		// Type 0x800A
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
		preference, err := strconv.ParseUint(parts[4], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid l2info preference %q", parts[4])
		}
		return []byte{
			0x80, 0x0A, // Type: Layer 2 Info
			byte(encaps), byte(control),
			byte(mtu >> 8), byte(mtu),
			0x00, byte(preference),
		}, nil
	}

	return nil, fmt.Errorf("invalid extended-community %q: expected format like target:ASN:NN", s)
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
func parseRouteTargetOrOrigin(subtype byte, asnStr, numStr string) ([]byte, error) {
	// Check if ASN part is an IP address
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
	// Type 2: 4-byte ASN, 2-byte number
	return []byte{
		ecTypeTransitive4ByteAS, subtype,
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
// Format: ( ASN:IP ) e.g., ( 18144:219.118.225.189 )
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
// Format: [ code flags value_hex ]
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
