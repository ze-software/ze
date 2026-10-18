// Design: docs/architecture/wire/attributes.md -- BGP Prefix-SID (attribute 40) TLV encoding
// RFC: rfc/short/rfc8669.md -- Label-Index TLV (Type 1) and Originator SRGB TLV (Type 3)
// RFC: rfc/short/rfc9252.md -- SRv6 L3/L2 Service TLV and the SRv6 SID Information Sub-TLV
// Related: builder.go -- SetPrefixSID, which puts these bytes on the wire
//
// The encoder sits in the attribute package because TWO entry points reach it
// for the same operator text: the config file
// (component/bgp/config/routeattr.go ParseRouteAttributes) and `update text`
// (component/bgp/plugins/cmd/update/update_text.go parseCommonAttributeText).
// The config package cannot serve the second one -- it already depends on the
// command plugin -- and a second encoder beside this one is a future
// disagreement with nothing to arbitrate it (ai/rules/principles.md).

package attribute

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/stringsx"
)

var (
	errInvalidPrefixSidFormatExpectedIndex = errors.New("invalid prefix-sid format: expected 'index, [(base,range)...]'")
	errUnmatchedParenthesisInSrgbList      = errors.New("unmatched parenthesis in SRGB list")
	errInvalidSrv6PrefixSidExpectedL3      = errors.New("invalid srv6 prefix-sid: expected l3-service or l2-service")
	errInvalidSrv6PrefixSidUnmatched       = errors.New("invalid srv6 prefix-sid: unmatched [ in SID structure")
	errEmptyPrefixSID                      = errors.New("prefix-sid requires a label index, or a list \"index, [(base,range)...]\"")
	errEmptyPrefixSIDSRv6                  = errors.New("srv6 prefix-sid requires a service and a SID: l3-service <ipv6> [0xNN] [struct]")
)

// EncodePrefixSID encodes operator text into BGP Prefix-SID TLV bytes.
// It is the ENCODE direction; ParsePrefixSID (prefixsid_wire.go) is the
// DECODE direction and reads the same TLVs back off the wire.
// Formats:
//   - Simple: "777" → Label Index 777
//   - With SRGB: "300, [( 800000,4096) ,( 1000000,5000)]"
//
// RFC 8669 TLV format:
//   - Type (1 byte) + Length (2 bytes) + Value (variable)
//
// Label Index TLV (Type 1):
//   - Reserved (1 byte) + Flags (2 bytes) + Label-Index (4 bytes)
func EncodePrefixSID(s string) ([]byte, error) {
	// An empty value names no SID. It is refused rather than answered with a nil
	// TLV, which a caller cannot tell from a Prefix-SID it asked for and did not
	// get (ai/rules/principles.md).
	if strings.TrimSpace(s) == "" {
		return nil, errEmptyPrefixSID
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
		return nil, fmt.Errorf("invalid prefix-sid label index %q: %w", s, err)
	}

	// Build TLV for Label Index (Type 1)
	// RFC 8669 Section 3.1: Type(1) + Length(2) + Reserved(1) + Flags(2) + LabelIndex(4) = 10 bytes
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

	return tlv, nil
}

// parsePrefixSIDWithSRGB parses format: "300, [( 800000,4096) ,( 1000000,5000)]".
//
// RFC 8669 TLV format:
//   - Type (1 byte) + Length (2 bytes) + Value (variable)
//
// SRGB TLV (Type 3):
//   - Flags (2 bytes) + SRGB descriptors (6 bytes each: Base(3) + Range(3))
func parsePrefixSIDWithSRGB(s string) ([]byte, error) {
	// Find the comma that separates label index from SRGB list
	// Format: "300, [( 800000,4096) ,( 1000000,5000)]"
	parts := strings.SplitN(s, ",", 2)
	if len(parts) < 2 {
		return nil, errInvalidPrefixSidFormatExpectedIndex
	}

	// Parse label index
	idxStr := strings.TrimSpace(parts[0])
	idx, err := strconv.ParseUint(idxStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix-sid label index %q: %w", idxStr, err)
	}

	// Parse SRGB list
	srgbStr := parts[1]
	srgbs, err := parseSRGBList(srgbStr)
	if err != nil {
		return nil, err
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

	return result, nil
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

// EncodePrefixSIDSRv6 encodes operator text into SRv6 Service TLV bytes.
// It is the ENCODE direction; ParsePrefixSID (prefixsid_wire.go) is the
// DECODE direction and reads the same TLVs back off the wire.
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
func EncodePrefixSIDSRv6(s string) ([]byte, error) {
	// As EncodePrefixSID: an empty value is refused by name, so
	// `bgp-prefix-sid-srv6 ( )` cannot silently drop the attribute.
	if strings.TrimSpace(s) == "" {
		return nil, errEmptyPrefixSIDSRv6
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
		return nil, errInvalidSrv6PrefixSidExpectedL3
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
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected IPv6 address, got %q", ipStr)
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
			return nil, fmt.Errorf("invalid srv6 behavior %q: %w", s[:behEnd], err)
		}
		behavior = uint16(behVal)
		s = strings.TrimSpace(s[behEnd:])
	}

	// Parse optional SID structure [LB,LN,Func,Arg,TransLen,TransOffset]
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end == -1 {
			return nil, errInvalidSrv6PrefixSidUnmatched
		}
		structStr := s[1:end]
		parts, count := stringsx.SplitCount(structStr, ",")
		if count != 6 {
			return nil, fmt.Errorf("invalid srv6 SID structure: expected 6 values, got %d", count)
		}
		for _, p := range parts {
			v, err := strconv.ParseUint(strings.TrimSpace(p), 10, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid srv6 SID structure value %q: %w", p, err)
			}
			sidStruct = append(sidStruct, byte(v))
		}
		s = strings.TrimSpace(s[end+1:])
		if err := validateSRv6OriginStructure(ipv6, behavior, sidStruct); err != nil {
			return nil, err
		}
	}
	if s != "" {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: unexpected trailing value %q", s)
	}

	// Build wire format per RFC 9252
	// Outer TLV: Type 5/6 (L3/L2 Service)
	//   Inner Sub-TLV: Type 1 (SRv6 SID Information)
	//     Value: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + [optional sub-sub-TLV]
	//       Optional sub-sub-TLV: Type 1 (SRv6 SID Structure, 6 bytes)

	// Build inner sub-TLV value (Type 1: SRv6 SID Information)
	// RFC 9252 Section 3.1: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + [sub-sub-TLVs]
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

	return result, nil
}

// validateSRv6OriginStructure checks the SID supplied for local advertisement.
// Transposed bits must already be carried in the NLRI label, not in both fields.
// RFC 9252 Section 3.2.1, including verified erratum 7817: the structure
// sum may equal the end of the transposed field.
func validateSRv6OriginStructure(sid netip.Addr, behavior uint16, structure []byte) error {
	total := int(structure[0]) + int(structure[1]) + int(structure[2]) + int(structure[3])
	length, offset := int(structure[4]), int(structure[5])
	if total > 128 {
		return errors.New("invalid srv6 SID structure: field lengths exceed 128 bits")
	}
	if offset+length > total {
		return errors.New("invalid srv6 SID structure: field lengths do not cover the transposed bits")
	}
	if length == 0 {
		if offset != 0 {
			return errors.New("invalid srv6 SID structure: zero transposition length requires zero offset")
		}
	}
	// RFC 8986 Section 4.12 defines Arg.FE2 for End.DT2M (0x0018).
	// No other endpoint behavior supported by this encoder defines arguments.
	if structure[3] != 0 && behavior != 0x0018 {
		return errors.New("invalid srv6 SID structure: arguments require the End.DT2M endpoint behavior")
	}
	bits := sid.As16()
	for bit := offset; bit < offset+length; bit++ {
		if bits[bit/8]&(1<<uint(7-bit%8)) != 0 {
			return errors.New("invalid srv6 SID: transposed bits must be zero in the advertised SID")
		}
	}
	return nil
}

// CheckSRv6TranspositionForFamily refuses an SRv6 SID Structure whose
// transposition the route family cannot carry: any transposition on a family
// that has no label field, and a Transposition Length wider than the label
// field on a family that has one.
//
// prefixSID is a Prefix-SID attribute value as EncodePrefixSIDSRv6 produces it;
// TLVs other than the SRv6 Service TLVs are not judged here. The check is not in
// the encoder because the encoder sees no route: `update text` declares its
// attributes before the sections that name a family, and one attribute set can
// serve sections of several families. Each entry point calls this where the
// attribute meets the family of the routes it is advertised with.
//
// nlrisplit.TranspositionLabelWidth answers which families have a label field
// and how wide it is, the same answer the receive side bounds the transposition
// length with, so a structure this accepts is never refused by that bound. For
// the VPN families a ze receiver also rebuilds the transposed SID. For EVPN it
// does not: nlrisplit.TranspositionLabel reads no EVPN label field, so an EVPN
// route ze originates with a non-zero transposition yields no SID at a ze
// receiver.
func CheckSRv6TranspositionForFamily(prefixSID []byte, fam family.Family) error {
	return forEachPrefixSIDTLV("TLV", prefixSID, func(tlvType uint8, value []byte) error {
		if tlvType != PrefixSIDTLVSRv6L3Service && tlvType != PrefixSIDTLVSRv6L2Service {
			return nil
		}
		if len(value) < srv6ServiceReservedLen {
			return fmt.Errorf("prefix-sid: SRv6 service TLV type %d has no RESERVED octet", tlvType)
		}
		return forEachPrefixSIDTLV("SRv6 sub-TLV", value[srv6ServiceReservedLen:], func(subType uint8, subValue []byte) error {
			if subType != srv6SubTLVSIDInformation {
				return nil
			}
			if len(subValue) < srv6SIDInfoLen {
				return fmt.Errorf("prefix-sid: SRv6 SID information sub-TLV is %d octets, RFC 9252 Section 3.1 needs %d",
					len(subValue), srv6SIDInfoLen)
			}
			return forEachPrefixSIDTLV("SRv6 sub-sub-TLV", subValue[srv6SIDInfoLen:], func(subSubType uint8, structure []byte) error {
				if subSubType != srv6SubSubTLVSIDStructure {
					return nil
				}
				return checkSIDStructureForFamily(structure, fam)
			})
		})
	})
}

// checkSIDStructureForFamily judges one SID Structure Sub-Sub-TLV value against
// the label field of fam: none, or one of a known width.
func checkSIDStructureForFamily(structure []byte, fam family.Family) error {
	if len(structure) != srv6SIDStructureLen {
		return fmt.Errorf("prefix-sid: SRv6 SID structure is %d octets, RFC 9252 Section 3.2.1 needs %d",
			len(structure), srv6SIDStructureLen)
	}
	width := nlrisplit.TranspositionLabelWidth(fam)
	if width == 0 {
		return checkTranspositionNotApplicable(structure, fam)
	}
	return checkTranspositionFitsLabel(structure, fam, width)
}

// checkTranspositionFitsLabel judges one SID Structure Sub-Sub-TLV value of
// srv6SIDStructureLen octets for a family whose label field is width bits wide.
func checkTranspositionFitsLabel(structure []byte, fam family.Family, width uint8) error {
	length := structure[4]
	// RFC 9252 Sections 5.1 and 5.2, for the VPN label field: "When using the
	// Transposition Scheme, the Transposition Length MUST be less than or equal to
	// 20 and less than or equal to the FL." Sections 6.2, 6.3 and 6.5 say the same
	// of the 24-bit EVPN field: "the Transposition Length MUST be less than or
	// equal to 24". The FL half is not judged here.
	if length <= width {
		return nil
	}
	return fmt.Errorf("invalid srv6 SID structure for %s routes: transposition length %d exceeds the %d-bit label field "+
		"that carries the transposed SID bits (RFC 9252 Sections 5 and 6); "+
		"set the fifth structure value to %d or less: [%d,%d,%d,%d,%d,%d]",
		fam, length, width, width, structure[0], structure[1], structure[2], structure[3], width, structure[5])
}

// checkTranspositionNotApplicable judges one SID Structure Sub-Sub-TLV value of
// srv6SIDStructureLen octets for a family where the Transposition Scheme is not
// applicable.
func checkTranspositionNotApplicable(structure []byte, fam family.Family) error {
	length, offset := structure[4], structure[5]
	// RFC 9252 Section 7: "The transposition offset and length MUST be 0 when the
	// Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is
	// not applicable (e.g., for global IPv6 service [RFC2545] where there is no label
	// field)."
	if length == 0 && offset == 0 {
		return nil
	}
	return fmt.Errorf("invalid srv6 SID structure for %s routes: transposition length %d and offset %d must both be 0, "+
		"because routes of family %s have no label field to carry transposed SID bits (RFC 9252 Section 7); "+
		"set the last two structure values to 0: [%d,%d,%d,%d,0,0]",
		fam, length, offset, fam, structure[0], structure[1], structure[2], structure[3])
}
