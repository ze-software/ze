//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package plugin

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/bgp/context"
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
	ErrMissingWatchdog    = errors.New("missing watchdog name")
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

	for i := 0; i < numPrefixes; i++ {
		// Calculate the new address by adding i * (size of each sub-prefix)
		newAddr := addToAddr(baseAddr, i, targetLen)
		newPrefix := netip.PrefixFrom(newAddr, targetLen)
		result = append(result, newPrefix)
	}

	return result, nil
}

// addToAddr adds an offset to an address at the given prefix boundary.
// For example, for a /23 prefix, offset 1 means +512 addresses (2^(32-23) = 512).
func addToAddr(addr netip.Addr, offset int, prefixLen int) netip.Addr {
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
	for i := 0; i < len(args)-1; i++ {
		if strings.EqualFold(args[i], "split") {
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
	}
	return 0, false
}

// parseSAFI validates SAFI and returns remaining args with the normalized SAFI name.
// Supported SAFIs: unicast, nlri-mpls (or labeled-unicast), mpls-vpn, mup.
// Note: "labeled-unicast" is normalized to "nlri-mpls" for ExaBGP compatibility.
func parseSAFI(args []string) (safi string, rest []string, err error) {
	if len(args) < 1 {
		return "", nil, fmt.Errorf("missing SAFI (expected: %s, %s, %s, or %s)",
			SAFINameUnicast, SAFINameNLRIMPLS, SAFINameMPLSVPN, SAFINameMUP)
	}
	safi = strings.ToLower(args[0])
	switch safi {
	case SAFINameUnicast, SAFINameMPLSVPN, SAFINameMUP:
		return safi, args[1:], nil
	case SAFINameNLRIMPLS, "labeled-unicast":
		// Normalize to nlri-mpls for ExaBGP compatibility
		return SAFINameNLRIMPLS, args[1:], nil
	default:
		return "", nil, fmt.Errorf("unsupported SAFI: %s (expected: %s, %s, %s, or %s)",
			args[0], SAFINameUnicast, SAFINameNLRIMPLS, SAFINameMPLSVPN, SAFINameMUP)
	}
}

func init() {
	// Update command (multi-family batch with attr accumulation)
	// This is the primary route announcement/withdrawal interface.
	// Syntax: bgp peer <sel> update text <attrs>... nlri <family> add/del <prefix>...
	// Syntax: bgp peer <sel> update text nlri <family> eor
	RegisterBuiltin("bgp peer update", handleUpdate, "Batch UPDATE with text/hex/b64 encoding")

	// Watchdog commands - control routes by watchdog group
	RegisterBuiltin("bgp watchdog announce", handleWatchdogAnnounce, "Announce routes in watchdog group")
	RegisterBuiltin("bgp watchdog withdraw", handleWatchdogWithdraw, "Withdraw routes in watchdog group")
}

// parseOrigin parses origin value: igp, egp, or incomplete.
func parseOrigin(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "igp":
		return 0, nil
	case "egp":
		return 1, nil
	case "incomplete", "?":
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid origin: %s (expected igp, egp, or incomplete)", s)
	}
}

// ErrInvalidKeyword is returned when a keyword is not valid for the route family.
var ErrInvalidKeyword = errors.New("invalid keyword for route family")

// ParsedRoute holds the result of parsing route attributes.
type ParsedRoute struct {
	Route RouteSpec
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
		tokens, consumed := parseBracketedList(args[idx+1:])
		if err := b.ParseASPath(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil

	case "community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing community value")
		}
		// Collect tokens until boundary or end
		tokens, consumed := parseBracketedList(args[idx+1:])
		if err := b.ParseCommunity(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil

	case "large-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing large-community value")
		}
		tokens, consumed := parseBracketedList(args[idx+1:])
		if err := b.ParseLargeCommunity(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil

	case "extended-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing extended-community value")
		}
		tokens, consumed := parseBracketedList(args[idx+1:])
		if err := b.ParseExtCommunity(strings.Join(tokens, " ")); err != nil {
			return 0, err
		}
		return consumed, nil
	}

	// Not a common attribute
	return 0, nil
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
		Route: RouteSpec{
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
				result.Route.NextHop = NewNextHopSelf()
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return ParsedRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				result.Route.NextHop = NewNextHopExplicit(nh)
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

	tokens, consumed := parseBracketedList(args)
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

// parseBracketedList parses a list of tokens.
// Supports:
//   - Bracketed: [token1 token2 ...] or [token1,token2,...]
//   - Single value: token (no brackets, returns single-element list)
//
// Returns the individual tokens and how many args were consumed.
func parseBracketedList(args []string) ([]string, int) {
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
			if strings.HasSuffix(arg, "]") {
				arg = strings.TrimSuffix(arg, "]")
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
			parts := strings.Split(tok, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					expanded = append(expanded, p)
				}
			}
		}

		return expanded, consumed
	}

	// Single value without brackets (like ExaBGP: community 2914:666)
	// Expand comma-separated if present
	parts := strings.Split(args[0], ",")
	var expanded []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			expanded = append(expanded, p)
		}
	}
	return expanded, 1
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

// parseCommunities parses communities in format [ASN:VAL ASN:VAL ...].
// Returns the parsed communities and how many tokens were consumed.
func parseCommunities(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("missing community value")
	}

	tokens, consumed := parseBracketedList(args)
	comms := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		comm, err := parseCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		comms = append(comms, comm)
	}

	return comms, consumed, nil
}

// parseCommunity parses a single community value.
// Delegates to attribute.ParseCommunity for shared parsing logic.
func parseCommunity(s string) (uint32, error) {
	return attribute.ParseCommunity(s)
}

// parseLargeCommunities parses large communities in format [GA:LD1:LD2 ...].
func parseLargeCommunities(args []string) ([]LargeCommunity, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("missing large-community value")
	}

	tokens, consumed := parseBracketedList(args)
	lcomms := make([]LargeCommunity, 0, len(tokens))
	for _, tok := range tokens {
		lc, err := parseLargeCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		lcomms = append(lcomms, lc)
	}

	return lcomms, consumed, nil
}

// parseLargeCommunity parses a single large community GA:LD1:LD2.
// Delegates to attribute.ParseLargeCommunity for shared parsing logic.
func parseLargeCommunity(s string) (LargeCommunity, error) {
	return attribute.ParseLargeCommunity(s)
}

// parseExtendedCommunities parses extended communities in format [type:value:value ...].
// RFC 4360 (Extended Communities), RFC 5575 (FlowSpec Actions).
//
// Supported formats:
//   - List syntax: [origin:ASN:IP] or [redirect:ASN:target] etc.
//   - Function syntax: traffic-rate <asn> <rate>, redirect <asn> <target>, etc.
//
// List format types:
//   - origin:ASN:IP (Type 0x00, Subtype 0x03) - 2-byte ASN + IPv4
//   - origin:IP:ASN (Type 0x01, Subtype 0x03) - IPv4 + 2-byte ASN
//   - redirect:ASN:target (Type 0x80, Subtype 0x08) - Traffic redirect
//   - rate-limit:bps (Type 0x80, Subtype 0x06) - Traffic rate limit
//
// Function format types (RFC 5575 FlowSpec actions):
//   - traffic-rate <asn> <rate> - Rate limit in bytes/sec
//   - discard - Sugar for traffic-rate 0 0
//   - redirect <asn> <target> - Redirect to VRF
//   - traffic-marking <dscp> - Set DSCP value
func parseExtendedCommunities(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("missing extended-community value")
	}

	// Check for function-style syntax (FlowSpec actions)
	firstToken := strings.ToLower(args[0])
	switch firstToken {
	case "traffic-rate":
		return parseTrafficRateFunction(args)
	case "discard":
		return parseDiscardFunction()
	case "redirect":
		return parseRedirectFunction(args)
	case "traffic-marking":
		return parseTrafficMarkingFunction(args)
	}

	// Fall back to list syntax
	tokens, consumed := parseBracketedList(args)
	comms := make([]attribute.ExtendedCommunity, 0, len(tokens))
	for _, tok := range tokens {
		ec, err := parseExtendedCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		comms = append(comms, ec)
	}

	return comms, consumed, nil
}

// parseTrafficRateFunction parses: traffic-rate <asn> <rate>
// RFC 5575 Section 7.2: Traffic-rate action (Type 0x80, Subtype 0x06).
// Format: 2-byte ASN + 4-byte IEEE 754 float (rate in bytes/sec).
// Rate of 0 means discard (drop all matching traffic).
func parseTrafficRateFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 3 {
		return nil, 0, fmt.Errorf("traffic-rate requires <asn> <rate>")
	}

	asn, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ASN in traffic-rate: %s", args[1])
	}

	rate, err := strconv.ParseFloat(args[2], 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid rate in traffic-rate: %s", args[2])
	}

	bits := math.Float32bits(float32(rate))
	ec := attribute.ExtendedCommunity{
		0x80, 0x06, // Type=0x80 (transitive), Subtype=0x06 (traffic-rate)
		byte(asn >> 8), byte(asn), // 2-byte ASN
		byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits), // IEEE 754 float
	}

	return []attribute.ExtendedCommunity{ec}, 3, nil
}

// parseDiscardFunction parses: discard
// RFC 5575 Section 7.2: Sugar for traffic-rate 0 0.
// Sets rate to 0.0 which means drop all matching traffic.
func parseDiscardFunction() ([]attribute.ExtendedCommunity, int, error) {
	ec := attribute.ExtendedCommunity{
		0x80, 0x06, // Type=0x80 (transitive), Subtype=0x06 (traffic-rate)
		0x00, 0x00, // ASN = 0
		0x00, 0x00, 0x00, 0x00, // Rate = 0.0 (IEEE 754)
	}
	return []attribute.ExtendedCommunity{ec}, 1, nil
}

// parseRedirectFunction parses: redirect <asn> <target>
// RFC 5575 Section 7.5: Redirect action (Type 0x80, Subtype 0x08).
// Format: 2-byte ASN + 4-byte local administrator (target VRF).
// Note: 4-byte ASN redirect (Type 0x82) not yet supported.
func parseRedirectFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 3 {
		return nil, 0, fmt.Errorf("redirect requires <asn> <target>")
	}

	asn, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ASN in redirect: %s", args[1])
	}

	target, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid target in redirect: %s", args[2])
	}

	ec := attribute.ExtendedCommunity{
		0x80, 0x08, // Type=0x80 (transitive), Subtype=0x08 (redirect)
		byte(asn >> 8), byte(asn), // 2-byte ASN
		byte(target >> 24), byte(target >> 16), byte(target >> 8), byte(target), // 4-byte target
	}

	return []attribute.ExtendedCommunity{ec}, 3, nil
}

// parseTrafficMarkingFunction parses: traffic-marking <dscp>
// RFC 5575 Section 7.6: Traffic-marking action (Type 0x80, Subtype 0x09).
// Format: 5 reserved bytes + 1-byte DSCP value (0-63 per RFC 2474).
// Sets the DSCP bits in the IP TOS/Traffic Class field.
func parseTrafficMarkingFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 2 {
		return nil, 0, fmt.Errorf("traffic-marking requires <dscp>")
	}

	dscp, err := strconv.ParseUint(args[1], 10, 8)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid DSCP in traffic-marking: %s", args[1])
	}
	if dscp > 63 {
		return nil, 0, fmt.Errorf("DSCP must be 0-63, got %d", dscp)
	}

	ec := attribute.ExtendedCommunity{
		0x80, 0x09, // Type=0x80 (transitive), Subtype=0x09 (traffic-marking)
		0x00, 0x00, 0x00, 0x00, 0x00, // Reserved
		byte(dscp), // DSCP value
	}

	return []attribute.ExtendedCommunity{ec}, 2, nil
}

// parseExtendedCommunity parses a single extended community string.
// RFC 4360: Extended communities are 8 octets with Type:Subtype:Value encoding.
// RFC 5575: FlowSpec traffic actions use specific type/subtype combinations.
//
// Formats:
//   - origin:ASN:IP     -> Type 0x00, Subtype 0x03 (2-byte ASN + 4-byte IP)
//   - origin:IP:ASN     -> Type 0x01, Subtype 0x03 (4-byte IP + 2-byte ASN)
//   - redirect:ASN:target -> Type 0x80, Subtype 0x08 (2-byte ASN + 4-byte target)
//   - rate-limit:bps    -> Type 0x80, Subtype 0x06 (IEEE 754 float rate)
func parseExtendedCommunity(s string) (attribute.ExtendedCommunity, error) {
	if s == "" {
		return attribute.ExtendedCommunity{}, fmt.Errorf("empty extended community")
	}

	// Split on first colon to get type prefix
	colonIdx := strings.Index(s, ":")
	if colonIdx == -1 {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid extended community format: %s", s)
	}

	typePrefix := strings.ToLower(s[:colonIdx])
	value := s[colonIdx+1:]

	switch typePrefix {
	case "origin":
		return parseOriginExtCommunity(value)
	case "redirect":
		return parseRedirectExtCommunity(value)
	case "rate-limit":
		return parseRateLimitExtCommunity(value)
	default:
		return attribute.ExtendedCommunity{}, fmt.Errorf("unknown extended community type: %s", typePrefix)
	}
}

// parseOriginExtCommunity parses origin extended community.
// RFC 4360/7153: Origin can be:
//   - Type 0x00: 2-byte ASN + 4-byte IPv4 (origin:ASN:IP)
//   - Type 0x01: 4-byte IPv4 + 2-byte ASN (origin:IP:ASN)
func parseOriginExtCommunity(value string) (attribute.ExtendedCommunity, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid origin format: %s", value)
	}

	// Try to determine format: ASN:IP or IP:ASN
	// If first part contains '.', it's IP:ASN format
	if strings.Contains(parts[0], ".") {
		// Type 0x01: IP:ASN format
		ip := net.ParseIP(parts[0])
		if ip == nil || ip.To4() == nil {
			return attribute.ExtendedCommunity{}, fmt.Errorf("invalid IPv4 in origin: %s", parts[0])
		}
		asn, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return attribute.ExtendedCommunity{}, fmt.Errorf("invalid ASN in origin: %s", parts[1])
		}
		ip4 := ip.To4()
		return attribute.ExtendedCommunity{
			0x01, 0x03, // Type=1, Subtype=3 (Origin)
			ip4[0], ip4[1], ip4[2], ip4[3], // IPv4 address
			byte(asn >> 8), byte(asn), // 2-byte ASN
		}, nil
	}

	// Type 0x00: ASN:IP format
	asn, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid ASN in origin: %s", parts[0])
	}
	ip := net.ParseIP(parts[1])
	if ip == nil || ip.To4() == nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid IPv4 in origin: %s", parts[1])
	}
	ip4 := ip.To4()
	return attribute.ExtendedCommunity{
		0x00, 0x03, // Type=0, Subtype=3 (Origin)
		byte(asn >> 8), byte(asn), // 2-byte ASN
		ip4[0], ip4[1], ip4[2], ip4[3], // IPv4 address
	}, nil
}

// parseRedirectExtCommunity parses FlowSpec redirect extended community.
// RFC 5575/7674: Traffic redirect to VRF.
// Format: redirect:ASN:target (Type 0x80, Subtype 0x08).
func parseRedirectExtCommunity(value string) (attribute.ExtendedCommunity, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid redirect format: %s", value)
	}

	asn, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid ASN in redirect: %s", parts[0])
	}
	target, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid target in redirect: %s", parts[1])
	}

	return attribute.ExtendedCommunity{
		0x80, 0x08, // Type=0x80, Subtype=0x08 (Redirect)
		byte(asn >> 8), byte(asn), // 2-byte ASN
		byte(target >> 24), byte(target >> 16), byte(target >> 8), byte(target), // 4-byte target
	}, nil
}

// parseRateLimitExtCommunity parses FlowSpec rate-limit extended community.
// RFC 5575: Traffic rate limiting.
// Format: rate-limit:bps (Type 0x80, Subtype 0x06)
// The rate is encoded as an IEEE 754 single-precision float.
func parseRateLimitExtCommunity(value string) (attribute.ExtendedCommunity, error) {
	rate, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid rate in rate-limit: %s", value)
	}

	// Convert to IEEE 754 single-precision float (4 bytes)
	bits := math.Float32bits(float32(rate))

	return attribute.ExtendedCommunity{
		0x80, 0x06, // Type=0x80, Subtype=0x06 (Rate Limit)
		0x00, 0x00, // Reserved bytes
		byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits), // IEEE 754 float
	}, nil
}

// ErrMissingNLRI is returned when nlri keyword or prefixes are missing.
var ErrMissingNLRI = errors.New("missing nlri")

// BatchAttributes holds parsed attributes for batch announcements.
type BatchAttributes struct {
	NextHop RouteNextHop
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
				attrs.NextHop = NewNextHopSelf()
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return attrs, nil, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				attrs.NextHop = NewNextHopExplicit(nh)
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

		// Unknown attribute - allow it for forward compatibility
		// (ExaBGP might have attributes we don't know about)
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
		if strings.HasPrefix(lower, AFINameIPv4+"/") || strings.HasPrefix(lower, AFINameIPv6+"/") {
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
	case SAFINameUnicast, SAFINameMulticast:
		// OK
	default:
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
				attrs.NextHop = NewNextHopSelf()
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return attrs, "", "", nil, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				attrs.NextHop = NewNextHopExplicit(nh)
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

		// Unknown attribute - skip with value
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

// ErrMissingLabel is returned when label is required but not provided.
var ErrMissingLabel = errors.New("missing label")

// ErrInvalidLabel is returned when label value is out of range.
var ErrInvalidLabel = errors.New("invalid label")

// MaxMPLSLabel is the maximum valid MPLS label value (20 bits).
const MaxMPLSLabel = 1048575

// validateLabel validates MPLS label value (20-bit, 0-1048575).
func validateLabel(label uint32) error {
	if label > MaxMPLSLabel {
		return fmt.Errorf("%w: must be 0-%d, got %d", ErrInvalidLabel, MaxMPLSLabel, label)
	}
	return nil
}

// parseLabels parses MPLS label(s) from args.
// Supports single value or bracketed list: "100" or "[100 200 300]" or "[100,200]".
func parseLabels(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, ErrMissingLabel
	}

	tokens, consumed := parseBracketedList(args)
	if len(tokens) == 0 {
		return nil, consumed, ErrMissingLabel
	}

	labels := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		val, err := strconv.ParseUint(tok, 10, 32)
		if err != nil {
			return nil, consumed, fmt.Errorf("%w: '%s'", ErrInvalidLabel, tok)
		}
		label := uint32(val)
		if err := validateLabel(label); err != nil {
			return nil, consumed, err
		}
		labels = append(labels, label)
	}

	return labels, consumed, nil
}

// parseLabeledUnicastAttributes parses MPLS labeled unicast route attributes.
// Args format: <prefix> [keyword value]...
// Supports MPLSKeywords: label plus all unicast keywords (no RD/RT).
func parseLabeledUnicastAttributes(args []string) (LabeledUnicastRoute, error) {
	if len(args) < 1 {
		return LabeledUnicastRoute{}, ErrMissingPrefix
	}

	// Parse prefix (first arg)
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return LabeledUnicastRoute{}, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[0])
	}

	route := LabeledUnicastRoute{
		Prefix: prefix,
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against MPLS keywords (not VPN - no RD/RT)
		if !MPLSKeywords[key] {
			return LabeledUnicastRoute{}, fmt.Errorf("%w: '%s' not valid for labeled-unicast", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing with Builder (wire-first)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return LabeledUnicastRoute{}, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Handle MPLS-specific keywords
		switch key {
		case "label":
			if i+1 >= len(args) {
				return LabeledUnicastRoute{}, ErrMissingLabel
			}
			labels, consumed, err := parseLabels(args[i+1:])
			if err != nil {
				return LabeledUnicastRoute{}, err
			}
			route.Labels = labels
			i += consumed

		case "next-hop":
			if i+1 >= len(args) {
				return LabeledUnicastRoute{}, ErrMissingNextHop
			}
			nh, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return LabeledUnicastRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, args[i+1])
			}
			route.NextHop = nh
			i++

		case "split":
			// Just skip - split is handled by caller (announceLabeledUnicastImpl)
			if i+1 < len(args) {
				i++
			}

		case "path-id":
			// RFC 7911 ADD-PATH identifier
			if i+1 >= len(args) {
				return LabeledUnicastRoute{}, fmt.Errorf("missing path-id value")
			}
			var pathID uint64
			pathID, err = strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return LabeledUnicastRoute{}, fmt.Errorf("invalid path-id: %s", args[i+1])
			}
			route.PathID = uint32(pathID)
			i++
		}
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		route.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return route, nil
}

// parseL3VPNAttributes parses L3VPN route attributes from args.
// Args format: <prefix> [keyword value]...
// Supports VPNKeywords: rd, rt, label, plus all unicast keywords.
func parseL3VPNAttributes(args []string) (L3VPNRoute, error) {
	if len(args) < 1 {
		return L3VPNRoute{}, ErrMissingPrefix
	}

	// Parse prefix (first arg)
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return L3VPNRoute{}, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[0])
	}

	route := L3VPNRoute{
		Prefix: prefix,
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against VPN keywords
		if !VPNKeywords[key] {
			return L3VPNRoute{}, fmt.Errorf("%w: '%s' not valid for L3VPN", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing with Builder (wire-first)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return L3VPNRoute{}, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Handle VPN-specific keywords
		switch key {
		case "rd":
			if i+1 >= len(args) {
				return L3VPNRoute{}, ErrMissingRD
			}
			route.RD = args[i+1]
			i++

		case "rt":
			if i+1 >= len(args) {
				return L3VPNRoute{}, fmt.Errorf("missing rt value")
			}
			route.RT = args[i+1]
			i++

		case "label":
			if i+1 >= len(args) {
				return L3VPNRoute{}, ErrMissingLabel
			}
			labels, consumed, err := parseLabels(args[i+1:])
			if err != nil {
				return L3VPNRoute{}, err
			}
			route.Labels = labels
			i += consumed

		case "next-hop":
			if i+1 >= len(args) {
				return L3VPNRoute{}, ErrMissingNextHop
			}
			nh, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return L3VPNRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, args[i+1])
			}
			route.NextHop = nh
			i++
		}
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		route.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return route, nil
}

// ParseRouteArgs parses route arguments into a RouteSpec.
// This is exported for use by external callers that want to build routes.
func ParseRouteArgs(args []string) (RouteSpec, error) {
	var route RouteSpec

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
				route.NextHop = NewNextHopSelf()
				continue
			}
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("%w: %s", ErrInvalidNextHop, value)
			}
			route.NextHop = NewNextHopExplicit(nh)

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

// ParseFlowSpecArgs parses FlowSpec command arguments.
// Format: match <spec> then <action>.
// Example: match destination 10.0.0.0/24 destination-port 80 then discard.
func ParseFlowSpecArgs(args []string) (FlowSpecRoute, error) {
	var route FlowSpecRoute
	route.Family = AFINameIPv4 // default

	inMatch := false
	inThen := false

	for i := 0; i < len(args); i++ {
		arg := strings.ToLower(args[i])

		switch arg {
		case "match":
			inMatch = true
			inThen = false
			continue
		case "then":
			inMatch = false
			inThen = true
			continue
		}

		switch {
		case inMatch:
			if i+1 >= len(args) {
				return route, fmt.Errorf("missing value for %s", arg)
			}
			value := args[i+1]

			switch arg {
			case "destination":
				prefix, err := netip.ParsePrefix(value)
				if err != nil {
					return route, fmt.Errorf("%w: %s", ErrInvalidPrefix, value)
				}
				route.DestPrefix = &prefix
				if prefix.Addr().Is6() {
					route.Family = AFINameIPv6
				}
				i++

			case "source":
				prefix, err := netip.ParsePrefix(value)
				if err != nil {
					return route, fmt.Errorf("%w: %s", ErrInvalidPrefix, value)
				}
				route.SourcePrefix = &prefix
				if prefix.Addr().Is6() {
					route.Family = AFINameIPv6
				}
				i++

			case "protocol":
				proto, err := parseProtocol(value)
				if err != nil {
					return route, err
				}
				route.Protocols = append(route.Protocols, proto)
				i++

			case "port":
				port, err := parsePort(value)
				if err != nil {
					return route, err
				}
				route.Ports = append(route.Ports, port)
				i++

			case "destination-port":
				port, err := parsePort(value)
				if err != nil {
					return route, err
				}
				route.DestPorts = append(route.DestPorts, port)
				i++

			case "source-port":
				port, err := parsePort(value)
				if err != nil {
					return route, err
				}
				route.SourcePorts = append(route.SourcePorts, port)
				i++

			default:
				return route, fmt.Errorf("unknown match keyword: %s", arg)
			}

		case inThen:
			switch arg {
			case "accept":
				route.Actions.Accept = true
			case "discard":
				route.Actions.Discard = true
			case "rate-limit":
				if i+1 >= len(args) {
					return route, fmt.Errorf("missing rate limit value")
				}
				rate, err := strconv.ParseUint(args[i+1], 10, 32)
				if err != nil {
					return route, fmt.Errorf("invalid rate limit: %s", args[i+1])
				}
				route.Actions.RateLimit = uint32(rate)
				i++
			case "redirect":
				if i+1 >= len(args) {
					return route, fmt.Errorf("missing redirect target")
				}
				route.Actions.Redirect = args[i+1]
				i++
			case "mark":
				if i+1 >= len(args) {
					return route, fmt.Errorf("missing DSCP value")
				}
				dscp, err := strconv.ParseUint(args[i+1], 10, 8)
				if err != nil {
					return route, fmt.Errorf("invalid DSCP: %s", args[i+1])
				}
				route.Actions.MarkDSCP = uint8(dscp)
				i++

			default:
				return route, fmt.Errorf("unknown then keyword: %s", arg)
			}

		default:
			// Provide helpful error: is it a misplaced match/then keyword or unknown?
			switch arg {
			case "destination", "source", "protocol", "port", "destination-port", "source-port":
				return route, fmt.Errorf("match keyword %q must appear after 'match'", arg)
			case "accept", "discard", "rate-limit", "redirect", "mark":
				return route, fmt.Errorf("then keyword %q must appear after 'then'", arg)
			default:
				return route, fmt.Errorf("unknown keyword %q", arg)
			}
		}
	}

	return route, nil
}

// parseProtocol parses a protocol name or number.
func parseProtocol(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "icmp":
		return 1, nil
	case "tcp":
		return 6, nil
	case "udp":
		return 17, nil
	case "gre":
		return 47, nil
	case "icmpv6", "icmp6":
		return 58, nil
	default:
		n, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			return 0, fmt.Errorf("%w: %s", ErrInvalidProtocol, s)
		}
		return uint8(n), nil
	}
}

// parsePort parses a port number.
func parsePort(s string) (uint16, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidPort, s)
	}
	return uint16(n), nil
}

// ParseL3VPNAttributes parses L3VPN (mpls-vpn) command arguments.
// Exported for use by encode command.
// Format: <prefix> rd <rd> next-hop <addr> label <label> [attributes...].
func ParseL3VPNAttributes(args []string) (L3VPNRoute, error) {
	return parseL3VPNAttributes(args)
}

// ParseLabeledUnicastAttributes parses labeled unicast (nlri-mpls) command arguments.
// Exported for use by encode command.
// Format: <prefix> next-hop <addr> label <label> [attributes...].
func ParseLabeledUnicastAttributes(args []string) (LabeledUnicastRoute, error) {
	return parseLabeledUnicastAttributes(args)
}

// handleWatchdogAnnounce handles: watchdog announce <name>
// Announces all routes in the named watchdog group that are currently withdrawn.
func handleWatchdogAnnounce(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "missing watchdog name",
		}, ErrMissingWatchdog
	}

	name := args[0]
	peerSelector := ctx.PeerSelector()

	if err := ctx.Reactor.AnnounceWatchdog(peerSelector, name); err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":     peerSelector,
			"watchdog": name,
		},
	}, nil
}

// handleWatchdogWithdraw handles: watchdog withdraw <name>
// Withdraws all routes in the named watchdog group that are currently announced.
func handleWatchdogWithdraw(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "missing watchdog name",
		}, ErrMissingWatchdog
	}

	name := args[0]
	peerSelector := ctx.PeerSelector()

	if err := ctx.Reactor.WithdrawWatchdog(peerSelector, name); err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":     peerSelector,
			"watchdog": name,
		},
	}, nil
}

// MUP route type constants.
const (
	MUPRouteTypeISD  = "mup-isd"  // Interwork Segment Discovery
	MUPRouteTypeDSD  = "mup-dsd"  // Direct Segment Discovery
	MUPRouteTypeT1ST = "mup-t1st" // Type 1 Session Transformed
	MUPRouteTypeT2ST = "mup-t2st" // Type 2 Session Transformed
)

// validMUPRouteTypes is the set of valid MUP route types.
var validMUPRouteTypes = map[string]bool{
	MUPRouteTypeISD:  true,
	MUPRouteTypeDSD:  true,
	MUPRouteTypeT1ST: true,
	MUPRouteTypeT2ST: true,
}

// ParseMUPArgs parses MUP route arguments.
// Format: <route-type> <prefix/addr> rd <RD> next-hop <NH> [extended-community [...]] [bgp-prefix-sid-srv6 (...)].
// Route types: mup-isd, mup-dsd, mup-t1st, mup-t2st.
func ParseMUPArgs(args []string, isIPv6 bool) (MUPRouteSpec, error) {
	spec := MUPRouteSpec{
		IsIPv6: isIPv6,
	}

	if len(args) < 1 {
		return spec, fmt.Errorf("missing MUP route type")
	}

	// First arg is route type
	routeType := strings.ToLower(args[0])
	if !validMUPRouteTypes[routeType] {
		return spec, fmt.Errorf("invalid MUP route type: %s (expected: mup-isd, mup-dsd, mup-t1st, mup-t2st)", args[0])
	}
	spec.RouteType = routeType

	if len(args) < 2 {
		return spec, fmt.Errorf("missing prefix/address for MUP route")
	}

	// Second arg is prefix or address depending on route type
	switch routeType {
	case MUPRouteTypeISD, MUPRouteTypeT1ST:
		spec.Prefix = args[1]
	case MUPRouteTypeDSD, MUPRouteTypeT2ST:
		spec.Address = args[1]
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	// Parse remaining args as key-value pairs
	for i := 2; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Handle MUP-specific attributes BEFORE common attribute parsing.
		// These must be set in MUPRouteSpec fields, not just in the builder.
		switch key {
		case "rd":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing rd value")
			}
			spec.RD = args[i+1]
			i++
			continue

		case "next-hop":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing next-hop value")
			}
			spec.NextHop = args[i+1]
			i++
			continue

		case "teid":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing teid value")
			}
			spec.TEID = args[i+1]
			i++
			continue

		case "qfi":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing qfi value")
			}
			qfi, err := strconv.ParseUint(args[i+1], 10, 8)
			if err != nil {
				return spec, fmt.Errorf("invalid qfi value: %s", args[i+1])
			}
			spec.QFI = uint8(qfi)
			i++
			continue

		case "endpoint":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing endpoint value")
			}
			spec.Endpoint = args[i+1]
			i++
			continue

		case "source":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing source value")
			}
			spec.Source = args[i+1]
			i++
			continue

		case "extended-community":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing extended-community value")
			}
			// Collect bracketed value - must set spec.ExtCommunity for MUP
			tokens, consumed := parseBracketedList(args[i+1:])
			spec.ExtCommunity = "[" + strings.Join(tokens, " ") + "]"
			i += consumed
			continue

		case "bgp-prefix-sid-srv6":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing bgp-prefix-sid-srv6 value")
			}
			// Parse parenthesized value
			value, consumed, err := parseParenthesizedValue(args[i+1:])
			if err != nil {
				return spec, fmt.Errorf("invalid bgp-prefix-sid-srv6: %w", err)
			}
			spec.PrefixSID = value
			i += consumed
			continue
		}

		// Try common attribute parsing with Builder (wire-first)
		// for attributes that are NOT MUP-specific (origin, local-preference, etc.)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return spec, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Unknown keyword - could be future extension, skip silently
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		spec.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return spec, nil
}
