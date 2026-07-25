// Design: docs/architecture/config/syntax.md — prefix-SID and SRv6 attribute parsing
// RFC: rfc/short/rfc8669.md — Label-Index TLV (Type 1) wire format
// RFC: rfc/short/rfc9252.md — SRv6 SID Information Sub-TLV wire format
// Overview: routeattr.go — core route attribute types

package bgpconfig

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/stringsx"
)

var (
	errInvalidPrefixSidFormatExpectedIndex = errors.New("invalid prefix-sid format: expected 'index, [(base,range)...]'")
	errUnmatchedParenthesisInSrgbList      = errors.New("unmatched parenthesis in SRGB list")
	errInvalidSrv6PrefixSidExpectedL3      = errors.New("invalid srv6 prefix-sid: expected l3-service or l2-service")
	errInvalidSrv6PrefixSidUnmatched       = errors.New("invalid srv6 prefix-sid: unmatched [ in SID structure")
)

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
//   - Reserved (1 byte) + Flags (2 bytes) + Label-Index (4 bytes)
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

	// Build TLV for Label Index (Type 1)
	// RFC 8669 Section 4.1: Type(1) + Length(2) + Reserved(1) + Flags(2) + LabelIndex(4) = 10 bytes
	tlv := []byte{
		1,               // Type: Label Index
		0,               // Length high byte
		7,               // Length low byte (7 bytes value)
		0,               // Reserved
		0,               // Flags high byte
		0,               // Flags low byte
		byte(idx >> 24), // Label Index (4 bytes, big-endian)
		byte(idx >> 16),
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
		return PrefixSID{}, errInvalidPrefixSidFormatExpectedIndex
	}

	// Parse label index
	idxStr := strings.TrimSpace(parts[0])
	idx, err := strconv.ParseUint(idxStr, 10, 32)
	if err != nil {
		return PrefixSID{}, fmt.Errorf("invalid prefix-sid label index %q: %w", idxStr, err)
	}

	// Parse SRGB list
	srgbStr := parts[1]
	srgbs, err := parseSRGBList(srgbStr)
	if err != nil {
		return PrefixSID{}, err
	}

	// Build TLVs
	// Label Index TLV (Type 1)
	// Type(1) + Length(2) + Reserved(1) + Flags(2) + LabelIndex(4) = 10 bytes
	result := []byte{
		1,               // Type: Label Index
		0,               // Length high byte
		7,               // Length low byte (7 bytes value)
		0,               // Reserved
		0,               // Flags high byte
		0,               // Flags low byte
		byte(idx >> 24), // Label Index (4 bytes, big-endian)
		byte(idx >> 16),
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
			return nil, errUnmatchedParenthesisInSrgbList
		}

		pair := s[start+1 : end]
		parts, count := stringsx.SplitCount(pair, ",")
		if count != 2 {
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
// Formats:
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
	switch {
	case strings.HasPrefix(s, "l3-service"):
		serviceType = 5 // TLV Type 5: SRv6 L3 Service
		s = strings.TrimPrefix(s, "l3-service")
	case strings.HasPrefix(s, "l2-service"):
		serviceType = 6 // TLV Type 6: SRv6 L2 Service
		s = strings.TrimPrefix(s, "l2-service")
	default:
		return PrefixSID{}, errInvalidSrv6PrefixSidExpectedL3
	}
	s = strings.TrimSpace(s)

	// Parse IPv6 address (required)
	var ipv6 netip.Addr
	var behavior uint16
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
			isHexDigit := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHexDigit {
				behEnd = i
				break
			}
		}
		behStr := s[2:behEnd]
		behVal, err := strconv.ParseUint(behStr, 16, 16)
		if err != nil {
			return PrefixSID{}, fmt.Errorf("invalid srv6 behavior %q: %w", s[:behEnd], err)
		}
		behavior = uint16(behVal)
		s = strings.TrimSpace(s[behEnd:])
	}

	// Parse optional SID structure [LB,LN,Func,Arg,TransLen,TransOffset]
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end == -1 {
			return PrefixSID{}, errInvalidSrv6PrefixSidUnmatched
		}
		structStr := s[1:end]
		parts, count := stringsx.SplitCount(structStr, ",")
		if count != 6 {
			return PrefixSID{}, fmt.Errorf("invalid srv6 SID structure: expected 6 values, got %d", count)
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
	//     Value: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + [optional sub-sub-TLV]
	//       Optional sub-sub-TLV: Type 1 (SRv6 SID Structure, 6 bytes)

	// Build inner sub-TLV value (Type 1: SRv6 SID Information)
	// RFC 9252 Section 3.2: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + [sub-sub-TLVs]
	var innerValue []byte
	innerValue = append(innerValue, 0) // Reserved
	innerValue = append(innerValue, ipv6.AsSlice()...)
	innerValue = append(innerValue, 0, byte(behavior>>8), byte(behavior), 0) // Flags(1), Behavior(2), Reserved(1)

	// Add SID structure sub-sub-TLV if provided.
	// RFC 9252 Section 3.2.1: Sub-Sub-TLV header = Type(1 octet) + Length(2 octets).
	// Reserved2 is already emitted above (trailing 0 of the Flags/Behavior/Reserved
	// group); the sub-sub-TLV starts directly with its Type, with no extra byte.
	if len(sidStruct) == 6 {
		innerValue = append(innerValue, 1, 0, byte(len(sidStruct))) // type 1, length(2)
		innerValue = append(innerValue, sidStruct...)
	}

	// Build inner sub-TLV header (Type 1)
	innerLen := len(innerValue)
	innerTLV := make([]byte, 0, 4+len(innerValue))
	innerTLV = append(innerTLV, 0, 1, byte(innerLen>>8), byte(innerLen))
	innerTLV = append(innerTLV, innerValue...)

	// Build outer TLV header (Type 5 or 6)
	outerLen := len(innerTLV)
	result := make([]byte, 0, 3+len(innerTLV))
	result = append(result, serviceType, byte(outerLen>>8), byte(outerLen))
	result = append(result, innerTLV...)

	return PrefixSID{Bytes: result}, nil
}
