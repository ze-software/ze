// Design: docs/architecture/config/syntax.md — BGP route attribute parsing
// Detail: routeattr_community.go — community attribute types
// Detail: routeattr_prefixsid.go — prefix SID attribute types

package bgpconfig

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/parse"
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
// Delegates to parse.Origin() for the actual parsing logic.
func ParseOrigin(s string) (Origin, error) {
	v, err := parse.Origin(s)
	if err != nil {
		return 0, err
	}
	return Origin(v), nil
}

func (o Origin) String() string {
	return parse.OriginString(uint8(o))
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
	// Try as IPv4 address (legacy format)
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

// ParseASPath parses an AS_PATH string: "[ ASN1 ASN2 ... ]".
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

// ParseAggregator parses an AGGREGATOR string: "( ASN:IP )".
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
	Labels            []MPLSLabel // RFC 8277: MPLS label stack
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
func ParseRouteAttributes(src *StaticRouteConfig) (*ParsedRouteAttributes, error) {
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

	// Labels - RFC 8277 multi-label support
	// `labels [...]` takes precedence if both specified (user error, but deterministic)
	if len(src.Labels) > 0 {
		attrs.Labels = make([]MPLSLabel, len(src.Labels))
		for i, ls := range src.Labels {
			label, err := ParseMPLSLabel(ls)
			if err != nil {
				return nil, fmt.Errorf("label[%d]: %w", i, err)
			}
			attrs.Labels[i] = label
		}
	} else if src.Label != "" {
		label, err := ParseMPLSLabel(src.Label)
		if err != nil {
			return nil, err
		}
		attrs.Labels = []MPLSLabel{label}
	}

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
		for p := range strings.FieldsSeq(clStr) {
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

	// BGP Prefix-SID (RFC 8669 label-index or RFC 9252 SRv6)
	// SRv6 format starts with "l3-service" or "l2-service"
	if src.PrefixSID != "" {
		if strings.HasPrefix(src.PrefixSID, "l3-service") || strings.HasPrefix(src.PrefixSID, "l2-service") {
			sid, err := ParsePrefixSIDSRv6(src.PrefixSID)
			if err != nil {
				return nil, err
			}
			attrs.PrefixSID = sid
		} else {
			sid, err := ParsePrefixSID(src.PrefixSID)
			if err != nil {
				return nil, err
			}
			attrs.PrefixSID = sid
		}
	}

	// Raw Attributes (hex format: "0xNN 0xNN 0xVALUE")
	// Known attribute codes are converted to typed fields.
	if src.Attribute != "" {
		raw, err := ParseRawAttribute(src.Attribute)
		if err != nil {
			return nil, fmt.Errorf("invalid raw attribute: %w", err)
		}

		parseRawAttributeInto(attrs, raw)
	}

	return attrs, nil
}

// parseRawAttributeInto converts known raw attribute codes into typed fields.
// Unrecognized codes are preserved in RawAttributes for wire encoding.
func parseRawAttributeInto(attrs *ParsedRouteAttributes, raw RawAttribute) {
	// Known BGP attribute codes: convert to typed fields
	if raw.Code == 4 && len(raw.Value) >= 4 { // MED
		attrs.MED = uint32(raw.Value[0])<<24 | uint32(raw.Value[1])<<16 |
			uint32(raw.Value[2])<<8 | uint32(raw.Value[3])
		return
	}
	if raw.Code == 5 && len(raw.Value) >= 4 { // LOCAL_PREF
		attrs.LocalPreference = uint32(raw.Value[0])<<24 | uint32(raw.Value[1])<<16 |
			uint32(raw.Value[2])<<8 | uint32(raw.Value[3])
		return
	}
	if raw.Code == 32 { // LARGE_COMMUNITY — parse 12-byte tuples
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
		return
	}
	// Unrecognized attribute code: preserve as raw for wire encoding
	attrs.RawAttributes = append(attrs.RawAttributes, raw)
}
