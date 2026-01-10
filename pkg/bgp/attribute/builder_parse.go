package attribute

import (
	"encoding/binary"
	"fmt"
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
func (b *Builder) ParseMED(s string) error {
	med, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid med: %s", s)
	}
	b.SetMED(uint32(med)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	return nil
}

// ParseLocalPref parses a LOCAL_PREF value from string.
func (b *Builder) ParseLocalPref(s string) error {
	lp, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid local-preference: %s", s)
	}
	b.SetLocalPref(uint32(lp)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	return nil
}

// ParseASPath parses an AS_PATH from string.
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
// Supports formats:
//   - "target:65000:100" - Route Target (2-byte ASN)
//   - "origin:65000:100" - Route Origin (2-byte ASN)
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
		return ExtendedCommunity{}, fmt.Errorf("invalid extended-community format: %s (expected type:ASN:value)", s)
	}

	var ec ExtendedCommunity

	// Determine type and subtype
	// RFC 4360: Type 0x00 = 2-byte ASN, Type 0x01 = IPv4, Type 0x02 = 4-byte ASN
	switch strings.ToLower(parts[0]) {
	case "target", "rt":
		ec[0] = 0x00 // 2-byte ASN
		ec[1] = 0x02 // Route Target
	case "origin", "soo":
		ec[0] = 0x00 // 2-byte ASN
		ec[1] = 0x03 // Route Origin
	default:
		return ExtendedCommunity{}, fmt.Errorf("unknown extended-community type: %s (expected target or origin)", parts[0])
	}

	// Parse ASN (2-byte)
	asn, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("invalid extended-community ASN: %s", parts[1])
	}
	binary.BigEndian.PutUint16(ec[2:4], uint16(asn)) //nolint:gosec // G115: bounded by ParseUint 16-bit

	// Parse value (4-byte)
	val, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return ExtendedCommunity{}, fmt.Errorf("invalid extended-community value: %s", parts[2])
	}
	binary.BigEndian.PutUint32(ec[4:8], uint32(val)) //nolint:gosec // G115: bounded by ParseUint 32-bit

	return ec, nil
}
