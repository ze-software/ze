// Design: docs/architecture/wire/attributes.md — path attribute encoding
// Related: text_append.go — zero-alloc AppendText helpers (filter-text output)
//
// Text format parsing for BGP attributes. Used by plugin system and any
// component needing text serialization. The reverse direction (attribute
// to filter text) lives in text_append.go as zero-alloc AppendText methods.
//
// Format rules:
//   - Scalars: "name value" (e.g., "origin igp", "med 100")
//   - Lists with 1 element: "name value" (e.g., "as-path 65001")
//   - Lists with >1 elements: "name [v1 v2 ...]" (e.g., "as-path [65001 65002]")
package attribute

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/stringsx"
)

var errMissingAsPathValue = errors.New("missing as-path value")

// wellKnownCanonicalNames is the sorted, deterministic list of canonical
// (kebab-case) well-known community names. Built once from communityNames.
var wellKnownCanonicalNames = func() []string {
	names := make([]string, 0, len(communityNames))
	for _, name := range communityNames {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}()

// WellKnownCommunityNames returns the sorted list of canonical well-known
// community name strings (kebab-case). Used for tab completion.
func WellKnownCommunityNames() []string {
	return wellKnownCanonicalNames
}

// -----------------------------------------------------------------------------
// Parsing Functions
// -----------------------------------------------------------------------------

// ParseCommunity parses a single standard community string to uint32.
// RFC 1997: COMMUNITIES attribute.
//
// Supports:
//   - ASN:VAL format per RFC 1997
//   - All IANA well-known community names (e.g., no-export, blackhole, graceful-shutdown)
//   - Bare integers: raw 32-bit community value
//   - Hex values: 0xNNNNNNNN format
//
// The ASN half is read in decimal only. The field width is the reason.
// RFC 1997 page 3: "The rest of the community attribute values shall be
// encoded using an autonomous system number in the first two octets."
// Two octets hold no four-byte AS number. The only RFC 5396 dotted spelling
// that fits is 0.Y, and that names the AS number the plain Y already names.
// asn.Parse would therefore accept no value an operator cannot type today.
func ParseCommunity(s string) (uint32, error) {
	if v, ok := communityValue(s); ok {
		return uint32(v), nil
	}

	// Check for hex format (0xNNNNNNNN)
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		val, err := strconv.ParseUint(s[2:], 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid community hex value %q", s)
		}
		return uint32(val), nil
	}

	// Check for ASN:Value format
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid community %q: expected ASN:Value format", s)
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

	// Bare integer: raw 32-bit community value (ExaBGP compatible)
	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid community %q: expected ASN:Value, hex, integer, or well-known name", s)
	}
	return uint32(val), nil
}

// ParseExtCommunityAdmin reads the administrator half of an extended community
// that names an AS number, and is the ONE declaration of that text form.
//
// Four parsers used to read it, in four packages, each with its own answer to
// the same question: `attribute.ParseSingleExtCommunity`,
// `attribute.FlowSpecRedirect`, `bgpconfig.parseExtCommunityASN` and
// `route.parseRouteTargetExtCommunity`. They agreed only because all four were
// wrong in the same direction.
//
// The AS number is read in every RFC 5396 spelling. RFC 5668 Section 2 gives
// the 4-octet AS specific extended community a "4-octet Autonomous System
// number" in its Global Administrator. A dotted spelling fits that field, and
// no RFC pins the text form to decimal.
//
// The `L` suffix forces the 4-octet encoding whatever the value. It is
// orthogonal to the spelling, so `65000L`, `0.100L` and `1.10L` all mean
// "encode as four octets". For a value of more than 65535 the suffix is
// redundant, because the value forces that encoding by itself. Redundant is
// accepted rather than refused: an operator who writes the suffix everywhere
// does not have to learn where it stops being needed.
//
// The caller decides the IPv4 form FIRST, with netip. A dot does not say the
// field is an address, because RFC 5396 Section 2 writes AS 65546 as `1.10`.
func ParseExtCommunityAdmin(s string) (number uint32, forced4Byte bool, err error) {
	if trimmed, ok := strings.CutSuffix(s, "L"); ok {
		s, forced4Byte = trimmed, true
	} else if trimmed, ok := strings.CutSuffix(s, "l"); ok {
		s, forced4Byte = trimmed, true
	}
	number, err = asn.Parse(s)
	if err != nil {
		return 0, false, err
	}
	return number, forced4Byte, nil
}

// ParseLargeCommunity parses a single large community GA:LD1:LD2.
// RFC 8092: LARGE_COMMUNITIES attribute.
//
// The Global Administrator is a four-byte AS number, so an RFC 5396 dotted
// spelling would fit it. It is still read in decimal only. RFC 8092
// Section 5 pins the spelling: "The canonical representation of BGP Large
// Communities is three separate unsigned integers in decimal notation."
// A dotted Global Administrator is a large community no other implementation
// writes or reads.
func ParseLargeCommunity(s string) (LargeCommunity, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return LargeCommunity{}, fmt.Errorf("invalid large-community %q: expected GA:LD1:LD2 format", s)
	}

	ga, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community global-admin %q", parts[0])
	}
	ld1, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local-data1 %q", parts[1])
	}
	ld2, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local-data2 %q", parts[2])
	}

	return LargeCommunity{
		GlobalAdmin: uint32(ga),  //nolint:gosec // G115: bounded by ParseUint 32-bit
		LocalData1:  uint32(ld1), //nolint:gosec // G115: bounded by ParseUint 32-bit
		LocalData2:  uint32(ld2), //nolint:gosec // G115: bounded by ParseUint 32-bit
	}, nil
}

// ParseBracketedList parses a list of tokens from command args.
// Supports:
//   - Bracketed: [token1 token2 ...] or [token1,token2,...]
//   - Single value: token (no brackets, returns single-element list)
//
// Returns the individual tokens and how many args were consumed.
func ParseBracketedList(args []string) ([]string, int) {
	if len(args) == 0 {
		return nil, 0
	}

	// Check if bracketed
	if strings.HasPrefix(args[0], "[") {
		var tokens []string
		consumed := 0

		for i, arg := range args {
			consumed++
			if i == 0 {
				arg = strings.TrimPrefix(arg, "[")
			}
			if before, ok := strings.CutSuffix(arg, "]"); ok {
				arg = before
				if arg != "" {
					tokens = append(tokens, arg)
				}
				break
			}
			if arg != "" {
				tokens = append(tokens, arg)
			}
		}

		// Expand comma-separated values
		var expanded []string
		for _, tok := range tokens {
			parts := strings.SplitSeq(tok, ",")
			for p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					expanded = append(expanded, p)
				}
			}
		}

		return expanded, consumed
	}

	// Single value without brackets
	// Expand comma-separated if present
	parts, count := stringsx.SplitCount(args[0], ",")
	expanded := make([]string, 0, count)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			expanded = append(expanded, p)
		}
	}
	return expanded, 1
}

// ParseASPath parses AS_PATH in format [ ASN1 ASN2 ... ] or [ASN1,ASN2,...].
// Returns the parsed AS numbers and how many tokens were consumed.
func ParseASPathText(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, errMissingAsPathValue
	}

	tokens, consumed := ParseBracketedList(args)
	asPath := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		// asn.Parse reads asplain, asdot and asdot+, so an AS path pasted from
		// a router that speaks a dotted notation is accepted as typed.
		number, err := asn.Parse(tok)
		if err != nil {
			return nil, consumed, fmt.Errorf("invalid ASN in as-path: %s", tok)
		}
		asPath = append(asPath, number)
	}

	return asPath, consumed, nil
}

// Text formatting for BGP attributes lives in text_append.go (attribute-level
// AppendText methods and element-level *.AppendText helpers on Aggregator,
// LargeCommunity, and ExtendedCommunity). The legacy Format* helpers that
// returned strings were deleted as part of the fmt-0-append migration per
// `.claude/rules/no-layering.md`.
