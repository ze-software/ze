// Design: docs/architecture/route-types.md — route definitions
// Detail: route_vpn.go — VPN route parsing
// Detail: route_mup.go — MUP route parsing
// Detail: route_labeled.go — labeled unicast route parsing
// Detail: route_community.go — community attribute parsing
// Detail: route_flowspec.go — FlowSpec route parsing

//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package route

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
)

// Errors for route parsing.
var (
	ErrMissingPrefix      = errors.New("missing prefix")
	ErrMissingNextHop     = errors.New("missing next-hop")
	ErrInvalidPrefix      = errors.New("invalid prefix")
	ErrInvalidNextHop     = errors.New("invalid next-hop")
	ErrMissingPeerAddress = errors.New("missing peer address")
	ErrInvalidFamily      = errors.New("invalid address family")
	ErrMissingRD          = errors.New("missing route-distinguisher")
	ErrInvalidRD          = errors.New("invalid route-distinguisher")
	ErrMissingRouteType   = errors.New("missing route-type")
	ErrInvalidRouteType   = errors.New("invalid route-type")
	ErrMissingMAC         = errors.New("missing mac address")
	ErrInvalidMAC         = errors.New("invalid mac address")
	ErrInvalidProtocol    = errors.New("invalid protocol")
	ErrInvalidPort        = errors.New("invalid port")
	ErrInvalidSplit       = errors.New("invalid split length")
)

// UpdateText parsing errors (moved from internal/plugin/errors.go).
var (
	ErrInvalidAttrMode    = errors.New("invalid attr mode (expected set, add, or del)")
	ErrMissingAttrMode    = errors.New("missing attr mode")
	ErrUnknownAttribute   = errors.New("unknown attribute")
	ErrAddOnScalar        = errors.New("'add' not valid for scalar attribute (use 'set')")
	ErrDelOnScalar        = errors.New("'del' not valid for scalar attribute (use 'set')")
	ErrASPathNotAddable   = errors.New("as-path does not support add/del (use 'set')")
	ErrMissingAddDel      = errors.New("expected 'add' or 'del' before prefix")
	ErrEmptyNLRISection   = errors.New("nlri section has no prefixes")
	ErrFamilyMismatch     = errors.New("NLRI does not match declared family")
	ErrFamilyNotSupported = errors.New("family not supported in text mode")
)

// Reactor errors (moved from internal/plugin/errors.go).
var (
	ErrNoPeersMatch          = errors.New("no peers match selector")
	ErrNoPeersAcceptedFamily = errors.New("no peers have family negotiated")
)

// splitPrefix splits a prefix into more-specific prefixes with the given length.
// For example, 10.0.0.0/21 split to /23 produces 4 prefixes.
// Returns error if targetLen is less than prefix length or exceeds address size.
func splitPrefix(prefix netip.Prefix, targetLen int) ([]netip.Prefix, error) {
	sourceBits := prefix.Bits()

	// Validate target length
	maxBits := 32
	if prefix.Addr().Is6() {
		maxBits = 128
	}

	if targetLen < sourceBits {
		return nil, fmt.Errorf("%w: target /%d is smaller than source /%d", ErrInvalidSplit, targetLen, sourceBits)
	}
	if targetLen > maxBits {
		return nil, fmt.Errorf("%w: target /%d exceeds maximum /%d", ErrInvalidSplit, targetLen, maxBits)
	}

	// Calculate number of resulting prefixes: 2^(targetLen - sourceBits)
	numPrefixes := 1 << (targetLen - sourceBits)
	result := make([]netip.Prefix, 0, numPrefixes)

	// Get base address as bytes
	baseAddr := prefix.Addr()

	for i := range numPrefixes {
		// Calculate the new address by adding i * (size of each sub-prefix)
		newAddr := addToAddr(baseAddr, i, targetLen)
		newPrefix := netip.PrefixFrom(newAddr, targetLen)
		result = append(result, newPrefix)
	}

	return result, nil
}

// addToAddr adds an offset to an address at the given prefix boundary.
// For example, for a /23 prefix, offset 1 means +512 addresses (2^(32-23) = 512).
func addToAddr(addr netip.Addr, offset, prefixLen int) netip.Addr {
	if offset == 0 {
		return addr
	}

	// Calculate bits to add: offset << (maxBits - prefixLen)
	maxBits := 32
	if addr.Is6() {
		maxBits = 128
	}

	shift := maxBits - prefixLen

	if addr.Is4() {
		// IPv4: simple uint32 arithmetic
		v4 := addr.As4()
		val := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
		//nolint:gosec // offset is bounded by number of prefixes (max 2^32)
		val += uint32(offset) << shift
		return netip.AddrFrom4([4]byte{
			byte(val >> 24),
			byte(val >> 16),
			byte(val >> 8),
			byte(val),
		})
	}

	// IPv6: use big-endian byte arithmetic
	v6 := addr.As16()
	// Convert to two uint64s for easier arithmetic
	hi := uint64(v6[0])<<56 | uint64(v6[1])<<48 | uint64(v6[2])<<40 | uint64(v6[3])<<32 |
		uint64(v6[4])<<24 | uint64(v6[5])<<16 | uint64(v6[6])<<8 | uint64(v6[7])
	lo := uint64(v6[8])<<56 | uint64(v6[9])<<48 | uint64(v6[10])<<40 | uint64(v6[11])<<32 |
		uint64(v6[12])<<24 | uint64(v6[13])<<16 | uint64(v6[14])<<8 | uint64(v6[15])

	// Add the offset at the right position
	//nolint:gosec // offset is bounded by number of prefixes (max 2^128 for IPv6)
	if shift >= 64 {
		// Shift affects high 64 bits
		hi += uint64(offset) << (shift - 64)
	} else {
		// Shift affects low 64 bits, may carry to high
		addLo := uint64(offset) << shift
		newLo := lo + addLo
		if newLo < lo {
			hi++ // carry
		}
		lo = newLo
	}

	var result [16]byte
	result[0] = byte(hi >> 56)
	result[1] = byte(hi >> 48)
	result[2] = byte(hi >> 40)
	result[3] = byte(hi >> 32)
	result[4] = byte(hi >> 24)
	result[5] = byte(hi >> 16)
	result[6] = byte(hi >> 8)
	result[7] = byte(hi)
	result[8] = byte(lo >> 56)
	result[9] = byte(lo >> 48)
	result[10] = byte(lo >> 40)
	result[11] = byte(lo >> 32)
	result[12] = byte(lo >> 24)
	result[13] = byte(lo >> 16)
	result[14] = byte(lo >> 8)
	result[15] = byte(lo)

	return netip.AddrFrom16(result)
}

// parseSplitArg looks for "split /N" in args and returns the target prefix length.
// Returns (0, false) if not found or invalid.
func parseSplitArg(args []string) (int, bool) {
	for i := range len(args) - 1 {
		if !strings.EqualFold(args[i], "split") {
			continue
		}
		val := args[i+1]
		if !strings.HasPrefix(val, "/") {
			return 0, false
		}
		length, err := strconv.Atoi(val[1:])
		if err != nil {
			return 0, false
		}
		return length, true
	}
	return 0, false
}

// parseSAFI validates SAFI and returns remaining args with the normalized SAFI name.
// Supported SAFIs: unicast, nlri-mpls (or labeled-unicast), mpls-vpn, mup.
// Note: "labeled-unicast" is normalized to "nlri-mpls" for legacy reasons.
func parseSAFI(args []string) (safi string, rest []string, err error) {
	if len(args) < 1 {
		return "", nil, fmt.Errorf("missing SAFI (expected: %s, %s, %s, or %s)",
			bgptypes.SAFINameUnicast, bgptypes.SAFINameNLRIMPLS, bgptypes.SAFINameMPLSVPN, bgptypes.SAFINameMUP)
	}
	safi = strings.ToLower(args[0])
	switch safi {
	case bgptypes.SAFINameUnicast, bgptypes.SAFINameMPLSVPN, bgptypes.SAFINameMUP:
		return safi, args[1:], nil
	case bgptypes.SAFINameNLRIMPLS, "labeled-unicast":
		// Normalize to nlri-mpls for legacy reasons
		return bgptypes.SAFINameNLRIMPLS, args[1:], nil
	default: // reject unsupported SAFI
		return "", nil, fmt.Errorf("unsupported SAFI: %s (expected: %s, %s, %s, or %s)",
			args[0], bgptypes.SAFINameUnicast, bgptypes.SAFINameNLRIMPLS, bgptypes.SAFINameMPLSVPN, bgptypes.SAFINameMUP)
	}
}

// ErrInvalidKeyword is returned when a keyword is not valid for the route family.
var ErrInvalidKeyword = errors.New("invalid keyword for route family")

// ParsedRoute holds the result of parsing route attributes.
type ParsedRoute struct {
	Route bgptypes.RouteSpec
}

// ParseRouteAttributes parses route attributes from args with keyword validation.
// Exported for use by encode command and tests.
//
// Args format: <prefix> [keyword value]...
// Example: 10.0.0.0/24 next-hop 1.2.3.4 origin igp.
func ParseRouteAttributes(args []string, allowedKeywords KeywordSet) (ParsedRoute, error) {
	return parseRouteAttributes(args, allowedKeywords)
}

// parseCommonAttributeBuilder parses a common BGP attribute by keyword into a Builder.
// This is the wire-first version of parseCommonAttribute.
// Returns the number of args consumed (0 if keyword not handled), or error.
func parseCommonAttributeBuilder(key string, args []string, idx int, b *attribute.Builder) (int, error) {
	switch key {
	case "origin":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing origin value")
		}
		if err := b.ParseOrigin(args[idx+1]); err != nil {
			return 0, err
		}
		return 1, nil

	case "local-preference":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing local-preference value")
		}
		if err := b.ParseLocalPref(args[idx+1]); err != nil {
			return 0, err
		}
		return 1, nil

	case "med":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing med value")
		}
		if err := b.ParseMED(args[idx+1]); err != nil {
			return 0, err
		}
		return 1, nil

	case "as-path":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing as-path value")
		}
		// Collect tokens until boundary or end
		tokens, consumed := attribute.ParseBracketedList(args[idx+1:])
		if err := b.ParseASPath(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil

	case "community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing community value")
		}
		// Collect tokens until boundary or end
		tokens, consumed := attribute.ParseBracketedList(args[idx+1:])
		if err := b.ParseCommunity(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil

	case "large-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing large-community value")
		}
		tokens, consumed := attribute.ParseBracketedList(args[idx+1:])
		if err := b.ParseLargeCommunity(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil

	case "extended-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing extended-community value")
		}
		tokens, consumed := attribute.ParseBracketedList(args[idx+1:])
		if err := b.ParseExtCommunity(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil
	default: // not a common attribute — caller handles
		return 0, nil
	}
}

// parseRouteAttributes parses route attributes from args with keyword validation.
// The allowedKeywords set defines which keywords are valid for the route family.
// Returns error for unknown or invalid keywords.
//
// Args format: <prefix> [keyword value]...
// Example: 10.0.0.0/24 next-hop 1.2.3.4 origin igp.
func parseRouteAttributes(args []string, allowedKeywords KeywordSet) (ParsedRoute, error) {
	if len(args) < 1 {
		return ParsedRoute{}, ErrMissingPrefix
	}

	// Parse prefix (first arg)
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return ParsedRoute{}, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[0])
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	result := ParsedRoute{
		Route: bgptypes.RouteSpec{
			Prefix: prefix,
		},
	}

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against allowed set
		if !allowedKeywords[key] {
			return ParsedRoute{}, fmt.Errorf("%w: '%s' not valid for this route family", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing with Builder (wire-first)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return ParsedRoute{}, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Handle route-specific keywords
		switch key {
		case "next-hop":
			if i+1 >= len(args) {
				return ParsedRoute{}, ErrMissingNextHop
			}
			nhStr := args[i+1]
			if strings.EqualFold(nhStr, "self") {
				result.Route.NextHop = bgptypes.NewNextHopSelf()
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return ParsedRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				result.Route.NextHop = bgptypes.NewNextHopExplicit(nh)
			}
			i++

		case "split":
			// Just skip - split is handled by caller
			if i+1 < len(args) {
				i++
			}
		}
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		result.Route.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return result, nil
}

// parseASPath parses AS_PATH in format [ ASN1 ASN2 ... ] or [ASN1,ASN2,...].
// Returns the parsed AS numbers and how many tokens were consumed.
func parseASPath(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("missing as-path value")
	}

	tokens, consumed := attribute.ParseBracketedList(args)
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

// parseParenthesizedValue parses a parenthesis-delimited value from args.
// Used for bgp-prefix-sid-srv6 ( l3-service ... ) format.
// Returns the content between parentheses as a single string, and consumed count.
func parseParenthesizedValue(args []string) (string, int, error) {
	if len(args) == 0 {
		return "", 0, fmt.Errorf("empty args for parenthesized value")
	}

	// Must start with "("
	if args[0] != "(" {
		return "", 0, fmt.Errorf("expected '(' at start of parenthesized value, got %q", args[0])
	}

	// Collect tokens until ")"
	// Pre-allocate with estimated capacity (args length minus parens)
	parts := make([]string, 0, len(args)-2)
	consumed := 0

	for i, arg := range args {
		consumed++
		if i == 0 {
			// Skip the opening paren
			continue
		}
		if arg == ")" {
			// Found closing paren
			return strings.Join(parts, " "), consumed, nil
		}
		parts = append(parts, arg)
	}

	return "", 0, fmt.Errorf("unclosed parenthesis in value")
}

// ErrMissingNLRI is returned when nlri keyword or prefixes are missing.
var ErrMissingNLRI = errors.New("missing nlri")

// BatchAttributes holds parsed attributes for batch announcements.
type BatchAttributes struct {
	NextHop bgptypes.RouteNextHop
	Attrs   *attribute.Builder
}

// parseAttributesNLRI parses: <attrs>... nlri <prefix>...
// Returns the parsed attributes and list of prefixes.
func parseAttributesNLRI(args []string) (BatchAttributes, []netip.Prefix, error) {
	attrs := BatchAttributes{
		Attrs: attribute.NewBuilder(),
	}
	var prefixes []netip.Prefix
	nlriIndex := -1

	// Find "nlri" keyword
	for i, arg := range args {
		if strings.EqualFold(arg, "nlri") {
			nlriIndex = i
			break
		}
	}

	if nlriIndex < 0 {
		return attrs, nil, fmt.Errorf("%w: 'nlri' keyword not found", ErrMissingNLRI)
	}

	// Parse attributes before "nlri"
	for i := 0; i < nlriIndex; i++ {
		key := strings.ToLower(args[i])

		// Handle next-hop
		if key == "next-hop" {
			if i+1 >= nlriIndex {
				return attrs, nil, ErrMissingNextHop
			}
			nhStr := args[i+1]
			if strings.EqualFold(nhStr, "self") {
				attrs.NextHop = bgptypes.NewNextHopSelf()
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return attrs, nil, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				attrs.NextHop = bgptypes.NewNextHopExplicit(nh)
			}
			i++
			continue
		}

		// Try common attribute parsing
		consumed, err := parseCommonAttributeBuilder(key, args, i, attrs.Attrs)
		if err != nil {
			return attrs, nil, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Forward-compatible: skip unknown attributes with their value
		if i+1 < nlriIndex {
			i++ // Skip value
		}
	}

	// Parse prefixes after "nlri"
	for i := nlriIndex + 1; i < len(args); i++ {
		prefix, err := netip.ParsePrefix(args[i])
		if err != nil {
			return attrs, nil, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[i])
		}
		prefixes = append(prefixes, prefix)
	}

	return attrs, prefixes, nil
}

// parseUpdateCommand parses: <attrs>... <afi> <safi> [nlri] <prefix>...
// Returns attributes, AFI, SAFI, and list of prefixes.
func parseUpdateCommand(args []string) (BatchAttributes, string, string, []netip.Prefix, error) {
	attrs := BatchAttributes{Attrs: attribute.NewBuilder()}
	var prefixes []netip.Prefix
	var afi, safi string

	// Find family token (afi/safi format, e.g., "ipv4/unicast")
	familyIndex := -1
	for i, arg := range args {
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, bgptypes.AFINameIPv4+"/") || strings.HasPrefix(lower, bgptypes.AFINameIPv6+"/") {
			parts := strings.SplitN(lower, "/", 2)
			afi = parts[0]
			safi = parts[1]
			familyIndex = i
			break
		}
	}

	if familyIndex < 0 {
		return attrs, "", "", nil, fmt.Errorf("%w: family not found (expected afi/safi format)", ErrInvalidFamily)
	}

	// Validate SAFI
	switch safi {
	case bgptypes.SAFINameUnicast, bgptypes.SAFINameMulticast:
		// OK
	default: // reject unsupported SAFI
		return attrs, "", "", nil, fmt.Errorf("%w: unsupported SAFI '%s'", ErrInvalidFamily, safi)
	}

	// Parse attributes before family
	for i := 0; i < familyIndex; i++ {
		key := strings.ToLower(args[i])

		// Handle next-hop
		if key == "next-hop" {
			if i+1 >= familyIndex {
				return attrs, "", "", nil, ErrMissingNextHop
			}
			nhStr := args[i+1]
			if strings.EqualFold(nhStr, "self") {
				attrs.NextHop = bgptypes.NewNextHopSelf()
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return attrs, "", "", nil, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				attrs.NextHop = bgptypes.NewNextHopExplicit(nh)
			}
			i++
			continue
		}

		// Try common attribute parsing
		consumed, err := parseCommonAttributeBuilder(key, args, i, attrs.Attrs)
		if err != nil {
			return attrs, "", "", nil, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Forward-compatible: skip unknown attributes with their value
		if i+1 < familyIndex {
			i++
		}
	}

	// Parse prefixes after family [nlri]
	startIdx := familyIndex + 1
	if startIdx < len(args) && strings.EqualFold(args[startIdx], "nlri") {
		startIdx++ // Skip optional "nlri" keyword
	}

	for i := startIdx; i < len(args); i++ {
		prefix, err := netip.ParsePrefix(args[i])
		if err != nil {
			return attrs, "", "", nil, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[i])
		}
		prefixes = append(prefixes, prefix)
	}

	return attrs, afi, safi, prefixes, nil
}

// ParseRouteArgs parses route arguments into a bgptypes.RouteSpec.
// This is exported for use by external callers that want to build routes.
func ParseRouteArgs(args []string) (bgptypes.RouteSpec, error) {
	var route bgptypes.RouteSpec

	if len(args) < 1 {
		return route, ErrMissingPrefix
	}

	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return route, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[0])
	}
	route.Prefix = prefix

	// Parse key-value pairs
	for i := 1; i < len(args)-1; i += 2 {
		key := strings.ToLower(args[i])
		value := args[i+1]

		switch key { //nolint:goconst,gocritic // String literals are clearer; switch for future cases
		case "next-hop":
			if strings.EqualFold(value, "self") {
				route.NextHop = bgptypes.NewNextHopSelf()
				continue
			}
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("%w: %s", ErrInvalidNextHop, value)
			}
			route.NextHop = bgptypes.NewNextHopExplicit(nh)

			// TODO: Add more attribute parsing
			// case "origin":
			// case "as-path":
			// case "community":
			// case "local-preference":
			// case "med":
		}
	}

	return route, nil
}
