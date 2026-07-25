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
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/stringsx"
)

var (
	errMissingAsPathValue         = errors.New("missing as-path value")
	errMissingCommunityValue      = errors.New("missing community value")
	errMissingLargeCommunityValue = errors.New("missing large-community value")
)

// Well-known community values per IANA BGP Well-Known Communities registry.
const (
	TextCommunityNoExport                uint32 = 0xFFFFFF01 // RFC 1997
	TextCommunityNoAdvertise             uint32 = 0xFFFFFF02 // RFC 1997
	TextCommunityNoExportSubconfed       uint32 = 0xFFFFFF03 // RFC 1997
	TextCommunityNoPeer                  uint32 = 0xFFFFFF04 // RFC 3765
	TextCommunityGracefulShutdown        uint32 = 0xFFFF0000 // RFC 8326
	TextCommunityAcceptOwn               uint32 = 0xFFFF0001 // RFC 7611
	TextCommunityRouteFilterTranslatedV4 uint32 = 0xFFFF0002 // IANA
	TextCommunityRouteFilterV4           uint32 = 0xFFFF0003 // IANA
	TextCommunityRouteFilterTranslatedV6 uint32 = 0xFFFF0004 // IANA
	TextCommunityRouteFilterV6           uint32 = 0xFFFF0005 // IANA
	TextCommunityLLGRStale               uint32 = 0xFFFF0006 // RFC 9494
	TextCommunityNoLLGR                  uint32 = 0xFFFF0007 // RFC 9494
	TextCommunityAcceptOwnNexthop        uint32 = 0xFFFF0008 // IANA (draft)
	TextCommunityStandbyPE               uint32 = 0xFFFF0009 // RFC 9026
	TextCommunityBlackhole               uint32 = 0xFFFF029A // RFC 7999
)

// wellKnownCommunityNames maps lowercase text names (kebab-case and underscore
// variants) to their uint32 wire values. Used by ParseCommunity for name lookup.
var wellKnownCommunityNames = map[string]uint32{
	"no-export":                  TextCommunityNoExport,
	"no_export":                  TextCommunityNoExport,
	"no-advertise":               TextCommunityNoAdvertise,
	"no_advertise":               TextCommunityNoAdvertise,
	"no-export-subconfed":        TextCommunityNoExportSubconfed,
	"no_export_subconfed":        TextCommunityNoExportSubconfed,
	"nopeer":                     TextCommunityNoPeer,
	"no-peer":                    TextCommunityNoPeer,
	"no_peer":                    TextCommunityNoPeer,
	"graceful-shutdown":          TextCommunityGracefulShutdown,
	"graceful_shutdown":          TextCommunityGracefulShutdown,
	"gshut":                      TextCommunityGracefulShutdown,
	"accept-own":                 TextCommunityAcceptOwn,
	"accept_own":                 TextCommunityAcceptOwn,
	"route-filter-translated-v4": TextCommunityRouteFilterTranslatedV4,
	"route_filter_translated_v4": TextCommunityRouteFilterTranslatedV4,
	"route-filter-v4":            TextCommunityRouteFilterV4,
	"route_filter_v4":            TextCommunityRouteFilterV4,
	"route-filter-translated-v6": TextCommunityRouteFilterTranslatedV6,
	"route_filter_translated_v6": TextCommunityRouteFilterTranslatedV6,
	"route-filter-v6":            TextCommunityRouteFilterV6,
	"route_filter_v6":            TextCommunityRouteFilterV6,
	"llgr-stale":                 TextCommunityLLGRStale,
	"llgr_stale":                 TextCommunityLLGRStale,
	"no-llgr":                    TextCommunityNoLLGR,
	"no_llgr":                    TextCommunityNoLLGR,
	"accept-own-nexthop":         TextCommunityAcceptOwnNexthop,
	"accept_own_nexthop":         TextCommunityAcceptOwnNexthop,
	"standby-pe":                 TextCommunityStandbyPE,
	"standby_pe":                 TextCommunityStandbyPE,
	"blackhole":                  TextCommunityBlackhole,
}

// wellKnownCanonicalNames is the sorted, deterministic list of canonical
// (kebab-case) well-known community names. Built once from communityNames.
var wellKnownCanonicalNames = func() []string {
	names := make([]string, 0, len(communityNames))
	for _, name := range communityNames {
		names = append(names, name)
	}
	sort.Strings(names)
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
func ParseCommunity(s string) (uint32, error) {
	if v, ok := wellKnownCommunityNames[strings.ToLower(s)]; ok {
		return v, nil
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

// ParseLargeCommunity parses a single large community GA:LD1:LD2.
// RFC 8092: LARGE_COMMUNITIES attribute.
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

// ParseOriginText parses origin string to uint8.
// RFC 4271: ORIGIN attribute.
func ParseOriginText(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "igp":
		return 0, nil
	case "egp":
		return 1, nil
	case "incomplete":
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid origin %q: expected igp, egp, or incomplete", s)
	}
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
		asn, err := strconv.ParseUint(tok, 10, 32)
		if err != nil {
			return nil, consumed, fmt.Errorf("invalid ASN in as-path: %s", tok)
		}
		asPath = append(asPath, uint32(asn))
	}

	return asPath, consumed, nil
}

// ParseCommunities parses communities in format [ASN:VAL ASN:VAL ...].
// Returns the parsed communities and how many tokens were consumed.
func ParseCommunitiesText(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, errMissingCommunityValue
	}

	tokens, consumed := ParseBracketedList(args)
	comms := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		comm, err := ParseCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		comms = append(comms, comm)
	}

	return comms, consumed, nil
}

// ParseLargeCommunities parses large communities in format [GA:LD1:LD2 ...].
// Returns the parsed communities and how many tokens were consumed.
func ParseLargeCommunitiesText(args []string) ([]LargeCommunity, int, error) {
	if len(args) == 0 {
		return nil, 0, errMissingLargeCommunityValue
	}

	tokens, consumed := ParseBracketedList(args)
	lcomms := make([]LargeCommunity, 0, len(tokens))
	for _, tok := range tokens {
		lc, err := ParseLargeCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		lcomms = append(lcomms, lc)
	}

	return lcomms, consumed, nil
}

// Text formatting for BGP attributes lives in text_append.go (attribute-level
// AppendText methods and element-level *.AppendText helpers on Aggregator,
// LargeCommunity, and ExtendedCommunity). The legacy Format* helpers that
// returned strings were deleted as part of the fmt-0-append migration per
// `.claude/rules/no-layering.md`.
