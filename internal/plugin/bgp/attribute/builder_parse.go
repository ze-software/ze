package attribute

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Origin string constants for parsing.
const (
	originIGP        = "igp"
	originEGP        = "egp"
	originIncomplete = "incomplete"
)

// ParseOrigin parses an origin string: "igp", "egp", "incomplete", or "?".
// Replaces any previously set origin value.
func (b *Builder) ParseOrigin(s string) error {
	if s == "" {
		return fmt.Errorf("empty origin value")
	}
	switch strings.ToLower(s) {
	case originIGP:
		b.SetOrigin(0)
	case originEGP:
		b.SetOrigin(1)
	case originIncomplete, "?":
		b.SetOrigin(2)
	default:
		return fmt.Errorf("invalid origin: %s (expected %s, %s, or %s)", s, originIGP, originEGP, originIncomplete)
	}
	return nil
}

// ParseMED parses a MED value from string.
// Replaces any previously set MED value.
func (b *Builder) ParseMED(s string) error {
	med, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid med: %s", s)
	}
	b.SetMED(uint32(med)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	return nil
}

// ParseLocalPref parses a LOCAL_PREF value from string.
// Replaces any previously set LOCAL_PREF value.
func (b *Builder) ParseLocalPref(s string) error {
	lp, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid local-preference: %s", s)
	}
	b.SetLocalPref(uint32(lp)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	return nil
}

// ParseASPath parses an AS_PATH from string.
// Replaces any previously set AS_PATH.
// Supports formats:
//   - "[65001 65002]" - bracketed with spaces
//   - "[65001,65002]" - bracketed with commas
//   - "65001 65002" - space-separated
//   - "65001" - single ASN
func (b *Builder) ParseASPath(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Handle brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	if s == "" {
		// Empty brackets: []
		b.SetASPath(nil)
		return nil
	}

	// Split by space or comma
	var tokens []string
	if strings.Contains(s, ",") {
		tokens = strings.Split(s, ",")
	} else {
		tokens = strings.Fields(s)
	}

	asPath := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		asn, err := strconv.ParseUint(tok, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid ASN in as-path: %s", tok)
		}
		asPath = append(asPath, uint32(asn)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	}

	b.SetASPath(asPath)
	return nil
}

// Well-known community values per RFC 1997 and RFC 3765.
var wellKnownCommunities = map[string]uint32{
	"no-export":           0xFFFFFF01, // RFC 1997
	"no-advertise":        0xFFFFFF02, // RFC 1997
	"no-export-subconfed": 0xFFFFFF03, // RFC 1997
	"nopeer":              0xFFFFFF04, // RFC 3765
	"blackhole":           0xFFFF029A, // RFC 7999
}

// ParseCommunity parses a community string.
// APPENDS to any previously set communities (does not replace).
// Supports formats:
//   - "65000:100" - ASN:value
//   - "no-export" - well-known community
//   - "[65000:100 65000:200]" - bracketed list
//   - "65000:100 65000:200" - space-separated
func (b *Builder) ParseCommunity(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Handle brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	tokens := strings.Fields(s)
	for _, tok := range tokens {
		comm, err := parseSingleCommunity(tok)
		if err != nil {
			return err
		}
		b.communities = append(b.communities, comm)
	}

	return nil
}

func parseSingleCommunity(s string) (uint32, error) {
	// Check well-known communities first
	if v, ok := wellKnownCommunities[strings.ToLower(s)]; ok {
		return v, nil
	}

	// Parse ASN:value format
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid community format: %s (expected ASN:value or well-known name)", s)
	}

	high, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community ASN: %s", parts[0])
	}

	low, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community value: %s", parts[1])
	}

	return uint32(high)<<16 | uint32(low), nil //nolint:gosec // G115: bounded by ParseUint 16-bit
}

// ParseLargeCommunity parses a large community string.
// APPENDS to any previously set large communities (does not replace).
// Supports formats:
//   - "65000:1:2" - global:local1:local2
//   - "[65000:1:2 65001:3:4]" - bracketed list
//   - "65000:1:2 65001:3:4" - space-separated
func (b *Builder) ParseLargeCommunity(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Handle brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	tokens := strings.Fields(s)
	for _, tok := range tokens {
		lc, err := parseSingleLargeCommunity(tok)
		if err != nil {
			return err
		}
		b.largeCommunities = append(b.largeCommunities, lc)
	}

	return nil
}

func parseSingleLargeCommunity(s string) (LargeCommunity, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return LargeCommunity{}, fmt.Errorf("invalid large-community format: %s (expected global:local1:local2)", s)
	}

	global, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community global: %s", parts[0])
	}

	local1, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local1: %s", parts[1])
	}

	local2, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local2: %s", parts[2])
	}

	return LargeCommunity{
		GlobalAdmin: uint32(global), //nolint:gosec // G115: bounded by ParseUint 32-bit
		LocalData1:  uint32(local1), //nolint:gosec // G115: bounded by ParseUint 32-bit
		LocalData2:  uint32(local2), //nolint:gosec // G115: bounded by ParseUint 32-bit
	}, nil
}

// ParseExtCommunity parses an extended community string.
// APPENDS to any previously set extended communities (does not replace).
// Supports formats:
//   - "target:65000:100" or "rt:65000:100" - Route Target (2-byte ASN)
//   - "origin:65000:100" or "soo:65000:100" - Route Origin (2-byte ASN)
//   - "target:1.2.3.4:100" - Route Target (IPv4 address)
//   - "origin:1.2.3.4:100" - Route Origin (IPv4 address)
func (b *Builder) ParseExtCommunity(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Handle brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	tokens := strings.Fields(s)
	for _, tok := range tokens {
		ec, err := parseSingleExtCommunity(tok)
		if err != nil {
			return err
		}
		b.extCommunities = append(b.extCommunities, ec)
	}

	return nil
}

func parseSingleExtCommunity(s string) (ExtendedCommunity, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return ExtendedCommunity{}, fmt.Errorf("invalid extended-community format: %s (expected type:admin:value)", s)
	}

	var ec ExtendedCommunity

	// Determine subtype from keyword
	// RFC 4360: Subtype 0x02 = Route Target, Subtype 0x03 = Route Origin
	var subtype byte
	switch strings.ToLower(parts[0]) {
	case "target", "rt":
		subtype = 0x02 // Route Target
	case "origin", "soo":
		subtype = 0x03 // Route Origin
	default:
		return ExtendedCommunity{}, fmt.Errorf("unknown extended-community type: %s (expected target or origin)", parts[0])
	}

	// Detect admin field format: IPv4 address or ASN
	// RFC 4360: Type 0x00 = 2-byte ASN, Type 0x01 = IPv4, Type 0x02 = 4-byte ASN
	if strings.Contains(parts[1], ".") {
		// IPv4 address format: target:1.2.3.4:100
		// Type 0x01 (IPv4 Address), 4-byte IP, 2-byte value
		ip := net.ParseIP(parts[1])
		if ip == nil {
			return ExtendedCommunity{}, fmt.Errorf("invalid extended-community IPv4 address: %s", parts[1])
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return ExtendedCommunity{}, fmt.Errorf("extended-community requires IPv4 address, got: %s", parts[1])
		}

		val, err := strconv.ParseUint(parts[2], 10, 16)
		if err != nil {
			return ExtendedCommunity{}, fmt.Errorf("invalid extended-community value (IPv4 format max 65535): %s", parts[2])
		}

		ec[0] = 0x01 // Type: IPv4 Address
		ec[1] = subtype
		copy(ec[2:6], ip4)
		binary.BigEndian.PutUint16(ec[6:8], uint16(val)) //nolint:gosec // G115: bounded by ParseUint 16-bit
	} else {
		// ASN format: try to determine 2-byte vs 4-byte
		asn, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return ExtendedCommunity{}, fmt.Errorf("invalid extended-community ASN: %s", parts[1])
		}

		if asn <= 65535 {
			// 2-byte ASN format: target:65000:100
			// Type 0x00 (2-byte AS), 2-byte ASN, 4-byte value
			val, err := strconv.ParseUint(parts[2], 10, 32)
			if err != nil {
				return ExtendedCommunity{}, fmt.Errorf("invalid extended-community value: %s", parts[2])
			}

			ec[0] = 0x00 // Type: 2-byte ASN
			ec[1] = subtype
			binary.BigEndian.PutUint16(ec[2:4], uint16(asn)) //nolint:gosec // G115: bounded by check
			binary.BigEndian.PutUint32(ec[4:8], uint32(val)) //nolint:gosec // G115: bounded by ParseUint 32-bit
		} else {
			// 4-byte ASN format: target:4200000001:100
			// Type 0x02 (4-byte AS), 4-byte ASN, 2-byte value
			val, err := strconv.ParseUint(parts[2], 10, 16)
			if err != nil {
				return ExtendedCommunity{}, fmt.Errorf("invalid extended-community value (4-byte ASN format max 65535): %s", parts[2])
			}

			ec[0] = 0x02 // Type: 4-byte ASN
			ec[1] = subtype
			binary.BigEndian.PutUint32(ec[2:6], uint32(asn)) //nolint:gosec // G115: bounded by ParseUint 32-bit
			binary.BigEndian.PutUint16(ec[6:8], uint16(val)) //nolint:gosec // G115: bounded by ParseUint 16-bit
		}
	}

	return ec, nil
}
