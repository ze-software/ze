// Design: docs/architecture/wire/attributes.md — path attribute encoding

package attribute

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/stringsx"
)

var errEmptyOriginValue = errors.New("empty origin value")

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
		return errEmptyOriginValue
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

	var tokens []string
	var tokenCount int
	if strings.Contains(s, ",") {
		tokens, tokenCount = stringsx.SplitCount(s, ",")
	} else {
		tokens = strings.Fields(s)
		tokenCount = len(tokens)
	}

	asPath := make([]uint32, 0, tokenCount)
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

	tokens := strings.FieldsSeq(s)
	for tok := range tokens {
		comm, err := parseSingleCommunity(tok)
		if err != nil {
			return err
		}
		b.communities = append(b.communities, Community(comm))
	}

	return nil
}

func parseSingleCommunity(s string) (uint32, error) {
	// Check well-known communities first, against the one registry. A private
	// table here accepted 5 names while the package parser accepted 31, so the
	// same community name resolved or failed depending on the entry point.
	if v, ok := CommunityValue(s); ok {
		return uint32(v), nil
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

	tokens := strings.FieldsSeq(s)
	for tok := range tokens {
		lc, err := ParseLargeCommunity(tok)
		if err != nil {
			return err
		}
		b.largeCommunities = append(b.largeCommunities, lc)
	}

	return nil
}

// ParseAIGP parses an AIGP metric value from string.
// Replaces any previously set AIGP value.
func (b *Builder) ParseAIGP(s string) error {
	metric, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid aigp: %s", s)
	}
	b.SetAIGP(metric)
	return nil
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

	tokens := strings.FieldsSeq(s)
	for tok := range tokens {
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
		addr, err := netip.ParseAddr(parts[1])
		if err != nil || !addr.Unmap().Is4() {
			return ExtendedCommunity{}, fmt.Errorf("extended-community requires IPv4 address, got: %s", parts[1])
		}
		ip4 := addr.Unmap().As4()

		val, err := strconv.ParseUint(parts[2], 10, 16)
		if err != nil {
			return ExtendedCommunity{}, fmt.Errorf("invalid extended-community value (IPv4 format max 65535): %s", parts[2])
		}

		ec[0] = 0x01 // Type: IPv4 Address
		ec[1] = subtype
		copy(ec[2:6], ip4[:])
		binary.BigEndian.PutUint16(ec[6:8], uint16(val)) //nolint:gosec // G115: bounded by ParseUint 16-bit
	} else {
		// ASN format: try to determine 2-byte vs 4-byte
		// Strip "L" suffix that forces 4-byte encoding (e.g., "120000L")
		asnStr := parts[1]
		forced4Byte := false
		if strings.HasSuffix(asnStr, "L") || strings.HasSuffix(asnStr, "l") {
			asnStr = asnStr[:len(asnStr)-1]
			forced4Byte = true
		}
		asn, err := strconv.ParseUint(asnStr, 10, 32)
		if err != nil {
			return ExtendedCommunity{}, fmt.Errorf("invalid extended-community ASN: %s", parts[1])
		}

		if !forced4Byte && asn <= 65535 {
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
