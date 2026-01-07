//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package api

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/parse"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
)

// Errors for route parsing.
var (
	ErrMissingPrefix  = errors.New("missing prefix")
	ErrMissingNextHop = errors.New("missing next-hop")
	ErrInvalidPrefix  = errors.New("invalid prefix")
)

// parsePrefixWithDefault parses a prefix string, defaulting to /32 for IPv4
// and /128 for IPv6 if no prefix length is specified.
// This matches ExaBGP behavior where "1.2.3.4" means "1.2.3.4/32".
func parsePrefixWithDefault(s string) (netip.Prefix, error) {
	// First try parsing as a prefix with length
	if prefix, err := netip.ParsePrefix(s); err == nil {
		return prefix, nil
	}

	// If that fails, try parsing as an address and default the prefix length
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%w: %s", ErrInvalidPrefix, s)
	}

	// Default to /32 for IPv4, /128 for IPv6
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32), nil
	}
	return netip.PrefixFrom(addr, 128), nil
}

var (
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

// parseWatchdogArg looks for "watchdog <name>" in args and returns the pool name.
// Returns ("", false) if not found.
func parseWatchdogArg(args []string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if strings.EqualFold(args[i], "watchdog") {
			return args[i+1], true
		}
	}
	return "", false
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

// RegisterRouteHandlers registers route-related command handlers.
func RegisterRouteHandlers(d *Dispatcher) {
	// Announce commands
	d.Register("announce route", handleAnnounceRoute, "Announce a route to peers")
	d.Register("announce eor", handleAnnounceEOR, "Send End-of-RIB marker")
	d.Register("announce flow", handleAnnounceFlow, "Announce a FlowSpec route")
	d.Register("announce vpls", handleAnnounceVPLS, "Announce a VPLS route")
	d.Register("announce l2vpn", handleAnnounceL2VPN, "Announce an L2VPN/EVPN route")

	// Family-explicit announce commands (ExaBGP compatibility)
	d.Register("announce ipv4", handleAnnounceIPv4, "Announce IPv4 route (family-explicit)")
	d.Register("announce ipv6", handleAnnounceIPv6, "Announce IPv6 route (family-explicit)")

	// Family-explicit announce commands with slash format
	d.Register("announce ipv4/unicast", handleAnnounceIPv4Unicast, "Announce IPv4 unicast route")
	d.Register("announce ipv4/mpls-vpn", handleAnnounceIPv4MPLSVPN, "Announce IPv4 MPLS-VPN route")
	d.Register("announce ipv4/nlri-mpls", handleAnnounceIPv4NLRIMPLS, "Announce IPv4 labeled unicast route")
	d.Register("announce ipv4/mup", handleAnnounceIPv4MUP, "Announce IPv4 MUP route")
	d.Register("announce ipv6/unicast", handleAnnounceIPv6Unicast, "Announce IPv6 unicast route")
	d.Register("announce ipv6/mpls-vpn", handleAnnounceIPv6MPLSVPN, "Announce IPv6 MPLS-VPN route")
	d.Register("announce ipv6/nlri-mpls", handleAnnounceIPv6NLRIMPLS, "Announce IPv6 labeled unicast route")
	d.Register("announce ipv6/mup", handleAnnounceIPv6MUP, "Announce IPv6 MUP route")

	// Batch announce commands (multiple NLRIs per UPDATE)
	d.Register("announce attributes", handleAnnounceAttributes, "Announce routes with shared attributes (ExaBGP compat)")
	d.Register("announce nlri", handleAnnounceNLRI, "Queue routes to active commit with explicit AFI/SAFI")
	d.Register("announce update", handleAnnounceUpdate, "Auto-commit wrapper: announce routes with explicit AFI/SAFI")

	// Update text command (multi-family batch with attr accumulation)
	d.Register("update", handleUpdate, "Batch UPDATE with text/hex/b64/cbor encoding")

	// Withdraw commands
	d.Register("withdraw route", handleWithdrawRoute, "Withdraw a route from peers")
	d.Register("withdraw flow", handleWithdrawFlow, "Withdraw a FlowSpec route")
	d.Register("withdraw vpls", handleWithdrawVPLS, "Withdraw a VPLS route")
	d.Register("withdraw l2vpn", handleWithdrawL2VPN, "Withdraw an L2VPN/EVPN route")

	// Family-explicit withdraw commands (ExaBGP compatibility)
	d.Register("withdraw ipv4", handleWithdrawIPv4, "Withdraw IPv4 route (family-explicit)")
	d.Register("withdraw ipv6", handleWithdrawIPv6, "Withdraw IPv6 route (family-explicit)")

	// Family-explicit withdraw commands with slash format
	d.Register("withdraw ipv4/unicast", handleWithdrawIPv4Unicast, "Withdraw IPv4 unicast route")
	d.Register("withdraw ipv4/mpls-vpn", handleWithdrawIPv4MPLSVPN, "Withdraw IPv4 MPLS-VPN route")
	d.Register("withdraw ipv4/nlri-mpls", handleWithdrawIPv4NLRIMPLS, "Withdraw IPv4 labeled unicast route")
	d.Register("withdraw ipv4/mup", handleWithdrawIPv4MUP, "Withdraw IPv4 MUP route")
	d.Register("withdraw ipv6/unicast", handleWithdrawIPv6Unicast, "Withdraw IPv6 unicast route")
	d.Register("withdraw ipv6/mpls-vpn", handleWithdrawIPv6MPLSVPN, "Withdraw IPv6 MPLS-VPN route")
	d.Register("withdraw ipv6/nlri-mpls", handleWithdrawIPv6NLRIMPLS, "Withdraw IPv6 labeled unicast route")
	d.Register("withdraw ipv6/mup", handleWithdrawIPv6MUP, "Withdraw IPv6 MUP route")

	// Watchdog commands - control routes by watchdog group
	d.Register("announce watchdog", handleAnnounceWatchdog, "Announce routes in watchdog group")
	d.Register("withdraw watchdog", handleWithdrawWatchdog, "Withdraw routes in watchdog group")
}

// handleAnnounceRoute handles: announce route <prefix> next-hop <addr> [attributes...] [split /N].
// This is a convenience command that auto-detects the address family from the prefix.
// Example: announce route 10.0.0.0/24 next-hop 192.168.1.1.
// Example: announce route 2001:db8::/32 next-hop 2001::1.
func handleAnnounceRoute(ctx *CommandContext, args []string) (*Response, error) {
	// Auto-detect family from prefix and delegate to shared implementation
	if len(args) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}

	// Parse prefix, allowing bare IP addresses (defaults to /32 for IPv4, /128 for IPv6)
	prefix, err := parsePrefixWithDefault(args[0])
	if err != nil {
		return &Response{Status: "error", Data: fmt.Sprintf("invalid prefix: %s", args[0])}, ErrInvalidPrefix
	}

	// Normalize args to include prefix length for downstream processing
	normalizedArgs := make([]string, len(args))
	copy(normalizedArgs, args)
	normalizedArgs[0] = prefix.String()

	// Delegate to shared implementation (wire encoding is determined by prefix in reactor)
	return announceRouteImpl(ctx, normalizedArgs)
}

// announceRouteImpl is the shared implementation for route announcements.
// Handles: <prefix> next-hop <addr> [attributes...] [split /N].
// Example: 10.0.0.0/24 next-hop 192.168.1.1.
// Example: 10.0.0.0/24 next-hop self.
// Example: 10.0.0.0/24 next-hop 1.2.3.4 origin igp local-preference 100 med 200 community [2:1].
// Example: 10.0.0.0/21 next-hop 1.2.3.4 split /23 (announces 4 /23 prefixes).
func announceRouteImpl(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 3 {
		return &Response{
			Status: "error",
			Data:   "usage: announce route <prefix> next-hop <addr|self>",
		}, ErrMissingPrefix
	}

	// Parse route with unicast keyword validation
	parsed, err := parseRouteAttributes(args, UnicastKeywords)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if !parsed.NextHopSelf && !parsed.Route.NextHop.IsValid() {
		return &Response{
			Status: "error",
			Data:   "missing next-hop",
		}, ErrMissingNextHop
	}

	peerSelector := ctx.PeerSelector()

	// Check for watchdog suffix - route goes to global pool
	watchdogName, hasWatchdog := parseWatchdogArg(args)
	if hasWatchdog {
		if err := ctx.Reactor.AddWatchdogRoute(parsed.Route, watchdogName); err != nil {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("failed to add to watchdog pool: %v", err),
			}, err
		}
		nextHopStr := parsed.Route.NextHop.String()
		if parsed.NextHopSelf {
			nextHopStr = "self"
		}
		return &Response{
			Status: "done",
			Data: map[string]any{
				"watchdog": watchdogName,
				"prefix":   parsed.Route.Prefix.String(),
				"next_hop": nextHopStr,
			},
		}, nil
	}

	// Check for split argument
	splitLen, hasSplit := parseSplitArg(args)

	// Handle split: announce multiple prefixes
	if hasSplit {
		prefixes, err := splitPrefix(parsed.Route.Prefix, splitLen)
		if err != nil {
			return &Response{
				Status: "error",
				Data:   err.Error(),
			}, err
		}

		// Announce each split prefix separately
		for _, p := range prefixes {
			splitRoute := parsed.Route
			splitRoute.Prefix = p
			if err := ctx.Reactor.AnnounceRoute(peerSelector, splitRoute); err != nil {
				return &Response{
					Status: "error",
					Data:   fmt.Sprintf("failed to announce %s: %v", p.String(), err),
				}, err
			}
		}

		return &Response{
			Status: "done",
			Data: map[string]any{
				"peer":           peerSelector,
				"prefix":         parsed.Route.Prefix.String(),
				"split":          splitLen,
				"prefixes_count": len(prefixes),
			},
		}, nil
	}

	// Announce single route
	if err := ctx.Reactor.AnnounceRoute(peerSelector, parsed.Route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":     peerSelector,
			"prefix":   parsed.Route.Prefix.String(),
			"next_hop": parsed.Route.NextHop.String(),
		},
	}, nil
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
	Route       RouteSpec
	NextHopSelf bool // true if "next-hop self" was specified
}

// ParseRouteAttributes parses route attributes from args with keyword validation.
// Exported for use by encode command and tests.
//
// Args format: <prefix> [keyword value]...
// Example: 10.0.0.0/24 next-hop 1.2.3.4 origin igp.
func ParseRouteAttributes(args []string, allowedKeywords KeywordSet) (ParsedRoute, error) {
	return parseRouteAttributes(args, allowedKeywords)
}

// parseCommonAttribute parses a common BGP attribute by keyword.
// Returns the number of args consumed (0 if keyword not handled), or error.
// This centralizes parsing logic for origin, med, local-preference, as-path,
// community, and large-community to avoid duplication across route types.
func parseCommonAttribute(key string, args []string, idx int, attrs *PathAttributes) (int, error) {
	switch key {
	case "origin":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing origin value")
		}
		origin, err := parseOrigin(args[idx+1])
		if err != nil {
			return 0, err
		}
		attrs.Origin = &origin
		return 1, nil

	case "local-preference":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing local-preference value")
		}
		lp, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid local-preference: %s", args[idx+1])
		}
		lpVal := uint32(lp)
		attrs.LocalPreference = &lpVal
		return 1, nil

	case "med":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing med value")
		}
		med, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid med: %s", args[idx+1])
		}
		medVal := uint32(med)
		attrs.MED = &medVal
		return 1, nil

	case "as-path":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing as-path value")
		}
		asPath, consumed, err := parseASPath(args[idx+1:])
		if err != nil {
			return 0, err
		}
		attrs.ASPath = asPath
		return consumed, nil

	case "community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing community value")
		}
		comms, consumed, err := parseCommunities(args[idx+1:])
		if err != nil {
			return 0, err
		}
		attrs.Communities = comms
		return consumed, nil

	case "large-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing large-community value")
		}
		lcomms, consumed, err := parseLargeCommunities(args[idx+1:])
		if err != nil {
			return 0, err
		}
		attrs.LargeCommunities = lcomms
		return consumed, nil

	case "extended-community":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("missing extended-community value")
		}
		extcomms, consumed, err := parseExtendedCommunities(args[idx+1:])
		if err != nil {
			return 0, err
		}
		attrs.ExtendedCommunities = extcomms
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

		// Try common attribute parsing first
		consumed, err := parseCommonAttribute(key, args, i, &result.Route.PathAttributes)
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
				result.NextHopSelf = true
				result.Route.NextHopSelf = true
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return ParsedRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, nhStr)
				}
				result.Route.NextHop = nh
			}
			i++

		case "split":
			// Just skip - split is handled by caller
			if i+1 < len(args) {
				i++
			}
		}
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
// Delegates to parse.Community for shared parsing logic.
func parseCommunity(s string) (uint32, error) {
	return parse.Community(s)
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
// Delegates to parse.LargeCommunity for shared parsing logic.
func parseLargeCommunity(s string) (LargeCommunity, error) {
	vals, err := parse.LargeCommunity(s)
	if err != nil {
		return LargeCommunity{}, err
	}
	return LargeCommunity{
		GlobalAdmin: vals[0],
		LocalData1:  vals[1],
		LocalData2:  vals[2],
	}, nil
}

// parseExtendedCommunities parses extended communities in format [type:value:value ...].
// RFC 4360 (Extended Communities), RFC 5575 (FlowSpec Actions).
//
// Supported formats:
//   - origin:ASN:IP (Type 0x00, Subtype 0x03) - 2-byte ASN + IPv4
//   - origin:IP:ASN (Type 0x01, Subtype 0x03) - IPv4 + 2-byte ASN
//   - redirect:ASN:target (Type 0x80, Subtype 0x08) - Traffic redirect
//   - rate-limit:bps (Type 0x80, Subtype 0x06) - Traffic rate limit
func parseExtendedCommunities(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("missing extended-community value")
	}

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

// handleWithdrawRoute handles: withdraw route <prefix>.
// This is a convenience command that auto-detects the address family from the prefix.
// Example: withdraw route 10.0.0.0/24.
// Example: withdraw route 2001:db8::/32.
func handleWithdrawRoute(ctx *CommandContext, args []string) (*Response, error) {
	// Auto-detect family from prefix and delegate
	if len(args) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}
	_, err := netip.ParsePrefix(args[0])
	if err != nil {
		return &Response{Status: "error", Data: fmt.Sprintf("invalid prefix: %s", args[0])}, ErrInvalidPrefix
	}

	// Delegate to shared implementation
	return withdrawRouteImpl(ctx, args)
}

// withdrawRouteImpl is the shared implementation for route withdrawals.
// Handles: <prefix> [watchdog <name>].
// Example: 10.0.0.0/24.
// Example: 10.0.0.0/24 watchdog health.
func withdrawRouteImpl(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: withdraw route <prefix>",
		}, ErrMissingPrefix
	}

	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("invalid prefix: %s", args[0]),
		}, ErrInvalidPrefix
	}

	// Check for watchdog suffix - remove from global pool
	watchdogName, hasWatchdog := parseWatchdogArg(args)
	if hasWatchdog {
		// Route key format: "prefix#pathID" (pathID is 0 for API routes)
		routeKey := fmt.Sprintf("%s#0", prefix.String())
		if err := ctx.Reactor.RemoveWatchdogRoute(routeKey, watchdogName); err != nil {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("failed to remove from watchdog pool: %v", err),
			}, err
		}
		return &Response{
			Status: "done",
			Data: map[string]any{
				"watchdog": watchdogName,
				"prefix":   prefix.String(),
			},
		}, nil
	}

	// Withdraw from matching peers (default "*" for all)
	peerSelector := ctx.PeerSelector()
	if err := ctx.Reactor.WithdrawRoute(peerSelector, prefix); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":   peerSelector,
			"prefix": prefix.String(),
		},
	}, nil
}

// announceMUPImpl handles MUP route announcements.
// Format: <route-type> <prefix/addr> rd <RD> next-hop <NH> [extended-community [...]] [bgp-prefix-sid-srv6 (...)].
// Example: mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1.
func announceMUPImpl(ctx *CommandContext, args []string, isIPv6 bool) (*Response, error) {
	spec, err := ParseMUPArgs(args, isIPv6)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	peerSelector := ctx.PeerSelector()

	if err := ctx.Reactor.AnnounceMUPRoute(peerSelector, spec); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce MUP route: %v", err),
		}, err
	}

	afi := AFINameIPv4
	if isIPv6 {
		afi = AFINameIPv6
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":       peerSelector,
			"family":     afi + " " + SAFINameMUP,
			"route_type": spec.RouteType,
			"prefix":     spec.Prefix,
			"address":    spec.Address,
			"rd":         spec.RD,
		},
	}, nil
}

// withdrawMUPImpl handles MUP route withdrawals.
// Format: <route-type> <prefix/addr> rd <RD> next-hop <NH> [extended-community [...]] [bgp-prefix-sid-srv6 (...)].
// Example: mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1.
func withdrawMUPImpl(ctx *CommandContext, args []string, isIPv6 bool) (*Response, error) {
	spec, err := ParseMUPArgs(args, isIPv6)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	peerSelector := ctx.PeerSelector()

	if err := ctx.Reactor.WithdrawMUPRoute(peerSelector, spec); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw MUP route: %v", err),
		}, err
	}

	afi := AFINameIPv4
	if isIPv6 {
		afi = AFINameIPv6
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":       peerSelector,
			"family":     afi + " " + SAFINameMUP,
			"route_type": spec.RouteType,
			"prefix":     spec.Prefix,
			"address":    spec.Address,
			"rd":         spec.RD,
		},
	}, nil
}

// handleAnnounceIPv4 handles: announce ipv4 <safi> <prefix> [attributes...].
// Supported SAFIs: unicast, nlri-mpls (or labeled-unicast), mpls-vpn, mup.
// Example: announce ipv4/unicast 10.0.0.0/24 next-hop 192.168.1.1.
// Example: announce ipv4/nlri-mpls 10.0.0.0/24 label 100 next-hop 1.2.3.4.
// Example: announce ipv4/mpls-vpn 10.0.0.0/24 rd 100:100 label 100 next-hop 1.2.3.4.
// Example: announce ipv4/mup mup-isd 10.0.0.0/24 rd 100:100 next-hop 2001::1.
func handleAnnounceIPv4(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, args, false, false)
}

// handleAnnounceIPv6 handles: announce ipv6 <safi> <prefix> [attributes...].
// Supported SAFIs: unicast, nlri-mpls (or labeled-unicast), mpls-vpn, mup.
// Example: announce ipv6/unicast 2001:db8::/32 next-hop 2001::1.
// Example: announce ipv6/nlri-mpls 2001:db8::/32 label 100 next-hop 2001::1.
// Example: announce ipv6/mpls-vpn 2001:db8::/32 rd 100:100 label 100 next-hop 2001::1.
// Example: announce ipv6/mup mup-isd 2001:db8::/32 rd 100:100 next-hop 2001::1.
func handleAnnounceIPv6(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, args, true, false)
}

// handleWithdrawIPv4 handles: withdraw ipv4 <safi> <prefix> [attributes...].
// Supported SAFIs: unicast, nlri-mpls (or labeled-unicast), mpls-vpn, mup.
// Example: withdraw ipv4/unicast 10.0.0.0/24.
// Example: withdraw ipv4/nlri-mpls 10.0.0.0/24 label 100.
// Example: withdraw ipv4/mpls-vpn 10.0.0.0/24 rd 100:100.
// Example: withdraw ipv4/mup mup-isd 10.0.0.0/24 rd 100:100.
func handleWithdrawIPv4(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, args, false, true)
}

// handleWithdrawIPv6 handles: withdraw ipv6 <safi> <prefix> [attributes...].
// Supported SAFIs: unicast, nlri-mpls (or labeled-unicast), mpls-vpn, mup.
// Example: withdraw ipv6/unicast 2001:db8::/32.
// Example: withdraw ipv6/nlri-mpls 2001:db8::/32 label 100.
// Example: withdraw ipv6/mpls-vpn 2001:db8::/32 rd 100:100.
// Example: withdraw ipv6/mup mup-isd 2001:db8::/32 rd 100:100.
func handleWithdrawIPv6(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, args, true, true)
}

// Slash-format announce handlers for ipv4/<safi>.
func handleAnnounceIPv4Unicast(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"unicast"}, args...), false, false)
}
func handleAnnounceIPv4MPLSVPN(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mpls-vpn"}, args...), false, false)
}
func handleAnnounceIPv4NLRIMPLS(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"nlri-mpls"}, args...), false, false)
}
func handleAnnounceIPv4MUP(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mup"}, args...), false, false)
}

// Slash-format announce handlers for ipv6/<safi>.
func handleAnnounceIPv6Unicast(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"unicast"}, args...), true, false)
}
func handleAnnounceIPv6MPLSVPN(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mpls-vpn"}, args...), true, false)
}
func handleAnnounceIPv6NLRIMPLS(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"nlri-mpls"}, args...), true, false)
}
func handleAnnounceIPv6MUP(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mup"}, args...), true, false)
}

// Slash-format withdraw handlers for ipv4/<safi>.
func handleWithdrawIPv4Unicast(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"unicast"}, args...), false, true)
}
func handleWithdrawIPv4MPLSVPN(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mpls-vpn"}, args...), false, true)
}
func handleWithdrawIPv4NLRIMPLS(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"nlri-mpls"}, args...), false, true)
}
func handleWithdrawIPv4MUP(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mup"}, args...), false, true)
}

// Slash-format withdraw handlers for ipv6/<safi>.
func handleWithdrawIPv6Unicast(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"unicast"}, args...), true, true)
}
func handleWithdrawIPv6MPLSVPN(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mpls-vpn"}, args...), true, true)
}
func handleWithdrawIPv6NLRIMPLS(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"nlri-mpls"}, args...), true, true)
}
func handleWithdrawIPv6MUP(ctx *CommandContext, args []string) (*Response, error) {
	return handleAFIRoute(ctx, append([]string{"mup"}, args...), true, true)
}

// handleAFIRoute is a common handler for announce/withdraw ipv4/ipv6.
func handleAFIRoute(ctx *CommandContext, args []string, isIPv6, isWithdraw bool) (*Response, error) {
	// Parse SAFI
	safi, rest, err := parseSAFI(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	// MUP has different arg format: route-type prefix ... (not prefix first)
	if safi == SAFINameMUP {
		if isWithdraw {
			return withdrawMUPImpl(ctx, rest, isIPv6)
		}
		return announceMUPImpl(ctx, rest, isIPv6)
	}

	// Validate prefix
	if len(rest) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}
	prefix, err := netip.ParsePrefix(rest[0])
	if err != nil {
		return &Response{Status: "error", Data: fmt.Sprintf("invalid prefix: %s", rest[0])}, ErrInvalidPrefix
	}

	// Check AFI matches
	afiName := "ipv4"
	if isIPv6 {
		afiName = "ipv6"
	}
	action := "announce"
	if isWithdraw {
		action = "withdraw"
	}
	if isIPv6 != prefix.Addr().Is6() {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("expected %s prefix for '%s %s', got: %s", afiName, action, afiName, rest[0]),
		}, ErrInvalidPrefix
	}

	// Route to appropriate handler based on SAFI
	switch safi {
	case SAFINameMPLSVPN:
		if isWithdraw {
			return withdrawL3VPNImpl(ctx, rest)
		}
		return announceL3VPNImpl(ctx, rest)
	case SAFINameNLRIMPLS:
		if isWithdraw {
			return withdrawLabeledUnicastImpl(ctx, rest)
		}
		return announceLabeledUnicastImpl(ctx, rest)
	default:
		if isWithdraw {
			return withdrawRouteImpl(ctx, rest)
		}
		return announceRouteImpl(ctx, rest)
	}
}

// ErrMissingNLRI is returned when nlri keyword or prefixes are missing.
var ErrMissingNLRI = errors.New("missing nlri")

// handleAnnounceAttributes handles: announce attributes <attrs>... nlri <prefix>...
// This is the ExaBGP-compatible syntax for announcing multiple NLRIs with shared attributes.
// Example: announce attributes next-hop 10.11.12.13 origin igp nlri 16.17.18.19/32 20.21.22.23/32
// Example: announce attributes med 100 next-hop 10.0.0.1 nlri 1.0.0.0/24 2.0.0.0/24.
func handleAnnounceAttributes(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 3 {
		return &Response{
			Status: "error",
			Data:   "usage: announce attributes <attrs>... nlri <prefix>...",
		}, ErrMissingNLRI
	}

	// Parse attributes and NLRIs
	attrs, prefixes, err := parseAttributesNLRI(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	if len(prefixes) == 0 {
		return &Response{Status: "error", Data: "no prefixes after nlri keyword"}, ErrMissingNLRI
	}

	// Validate next-hop is present
	if !attrs.NextHop.IsValid() {
		return &Response{Status: "error", Data: "missing next-hop"}, ErrMissingNextHop
	}

	peerSelector := ctx.PeerSelector()

	// Announce each prefix with shared attributes
	for _, prefix := range prefixes {
		route := RouteSpec{
			Prefix:         prefix,
			NextHop:        attrs.NextHop,
			PathAttributes: attrs.PathAttributes,
		}
		if err := ctx.Reactor.AnnounceRoute(peerSelector, route); err != nil {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("failed to announce %s: %v", prefix.String(), err),
			}, err
		}
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":     peerSelector,
			"prefixes": len(prefixes),
			"next_hop": attrs.NextHop.String(),
		},
	}, nil
}

// handleAnnounceNLRI handles: announce nlri <attrs>... <afi> <safi> [nlri] <prefix>...
// Queues routes to an active commit. Requires commit to be started first.
// Example: announce nlri next-hop 10.0.0.1 origin igp ipv4/unicast 1.0.0.0/24 2.0.0.0/24.
func handleAnnounceNLRI(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 4 {
		return &Response{
			Status: "error",
			Data:   "usage: announce nlri <attrs>... <afi> <safi> [nlri] <prefix>...",
		}, ErrMissingNLRI
	}

	// Parse attributes, AFI/SAFI, and NLRIs
	attrs, afi, safi, prefixes, err := parseUpdateCommand(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	if len(prefixes) == 0 {
		return &Response{Status: "error", Data: "no prefixes specified"}, ErrMissingNLRI
	}

	// Validate next-hop is present
	if !attrs.NextHop.IsValid() {
		return &Response{Status: "error", Data: "missing next-hop"}, ErrMissingNextHop
	}

	// Validate prefix families match AFI
	for _, prefix := range prefixes {
		isIPv4 := prefix.Addr().Is4()
		if afi == AFINameIPv4 && !isIPv4 {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("prefix %s is not IPv4", prefix.String()),
			}, ErrInvalidPrefix
		}
		if afi == AFINameIPv6 && isIPv4 {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("prefix %s is not IPv6", prefix.String()),
			}, ErrInvalidPrefix
		}
	}

	// Queue routes to active commit
	return queueRoutesToCommit(ctx, attrs, afi, safi, prefixes)
}

// handleAnnounceUpdate handles: announce update <attrs>... <afi> <safi> [nlri] <prefix>...
// This is an auto-commit wrapper: starts commit, queues routes, ends with EOR.
// Equivalent to: commit <auto> start; announce nlri ...; commit <auto> eor
// Example: announce update next-hop 10.0.0.1 origin igp ipv4/unicast 1.0.0.0/24 2.0.0.0/24.
func handleAnnounceUpdate(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 4 {
		return &Response{
			Status: "error",
			Data:   "usage: announce update <attrs>... <afi> <safi> [nlri] <prefix>...",
		}, ErrMissingNLRI
	}

	// Parse attributes, AFI/SAFI, and NLRIs
	attrs, afi, safi, prefixes, err := parseUpdateCommand(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	if len(prefixes) == 0 {
		return &Response{Status: "error", Data: "no prefixes specified"}, ErrMissingNLRI
	}

	// Validate next-hop is present
	if !attrs.NextHop.IsValid() {
		return &Response{Status: "error", Data: "missing next-hop"}, ErrMissingNextHop
	}

	// Validate prefix families match AFI
	for _, prefix := range prefixes {
		isIPv4 := prefix.Addr().Is4()
		if afi == AFINameIPv4 && !isIPv4 {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("prefix %s is not IPv4", prefix.String()),
			}, ErrInvalidPrefix
		}
		if afi == AFINameIPv6 && isIPv4 {
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("prefix %s is not IPv6", prefix.String()),
			}, ErrInvalidPrefix
		}
	}

	peerSelector := ctx.PeerSelector()

	// Auto-commit: start, queue routes, end with EOR
	commitName := fmt.Sprintf("_auto_update_%d", time.Now().UnixNano())

	// Start commit
	if err := ctx.CommitManager.Start(commitName, peerSelector); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to start auto-commit: %v", err),
		}, err
	}

	// Queue routes
	tx, err := ctx.CommitManager.Get(commitName)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	for _, prefix := range prefixes {
		route := buildRoute(prefix, attrs, afi, safi)
		tx.QueueAnnounce(route)
	}

	// End commit with EOR
	tx, err = ctx.CommitManager.End(commitName)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to end auto-commit: %v", err),
		}, err
	}

	// Send routes
	routes := tx.Routes()
	result, err := ctx.Reactor.SendRoutes(peerSelector, routes, nil, true)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to send routes: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":             peerSelector,
			"family":           afi + " " + safi,
			"prefixes":         len(prefixes),
			"routes_announced": result.RoutesAnnounced,
			"updates_sent":     result.UpdatesSent,
			"eor_sent":         true,
		},
	}, nil
}

// BatchAttributes holds parsed attributes for batch announcements.
type BatchAttributes struct {
	NextHop netip.Addr
	PathAttributes
}

// parseAttributesNLRI parses: <attrs>... nlri <prefix>...
// Returns the parsed attributes and list of prefixes.
func parseAttributesNLRI(args []string) (BatchAttributes, []netip.Prefix, error) {
	var attrs BatchAttributes
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
			nh, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return attrs, nil, fmt.Errorf("%w: %s", ErrInvalidNextHop, args[i+1])
			}
			attrs.NextHop = nh
			i++
			continue
		}

		// Try common attribute parsing
		consumed, err := parseCommonAttribute(key, args, i, &attrs.PathAttributes)
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
	var attrs BatchAttributes
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
			nh, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return attrs, "", "", nil, fmt.Errorf("%w: %s", ErrInvalidNextHop, args[i+1])
			}
			attrs.NextHop = nh
			i++
			continue
		}

		// Try common attribute parsing
		consumed, err := parseCommonAttribute(key, args, i, &attrs.PathAttributes)
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

// buildRoute creates a rib.Route from prefix and attributes.
func buildRoute(prefix netip.Prefix, attrs BatchAttributes, afiStr, safiStr string) *rib.Route {
	// Determine AFI/SAFI
	var afi nlri.AFI
	var safi nlri.SAFI

	switch afiStr {
	case AFINameIPv4:
		afi = nlri.AFIIPv4
	case AFINameIPv6:
		afi = nlri.AFIIPv6
	default:
		if prefix.Addr().Is4() {
			afi = nlri.AFIIPv4
		} else {
			afi = nlri.AFIIPv6
		}
	}

	switch safiStr {
	case SAFINameMulticast:
		safi = nlri.SAFIMulticast
	default:
		safi = nlri.SAFIUnicast
	}

	// Build NLRI
	n := nlri.NewINET(nlri.Family{AFI: afi, SAFI: safi}, prefix, 0)

	// Build attributes - start with default Origin IGP
	var pathAttrs []attribute.Attribute

	// Add Origin - use specified or default to IGP
	if attrs.Origin != nil {
		pathAttrs = append(pathAttrs, attribute.Origin(*attrs.Origin))
	} else {
		pathAttrs = append(pathAttrs, attribute.OriginIGP)
	}

	// Add optional attributes
	if attrs.LocalPreference != nil {
		pathAttrs = append(pathAttrs, attribute.LocalPref(*attrs.LocalPreference))
	}
	if attrs.MED != nil {
		pathAttrs = append(pathAttrs, attribute.MED(*attrs.MED))
	}
	if len(attrs.ASPath) > 0 {
		pathAttrs = append(pathAttrs, &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: attrs.ASPath,
			}},
		})
	}
	if len(attrs.Communities) > 0 {
		comms := make(attribute.Communities, len(attrs.Communities))
		for i, c := range attrs.Communities {
			comms[i] = attribute.Community(c)
		}
		pathAttrs = append(pathAttrs, comms)
	}
	if len(attrs.LargeCommunities) > 0 {
		lc := make(attribute.LargeCommunities, len(attrs.LargeCommunities))
		copy(lc, attrs.LargeCommunities)
		pathAttrs = append(pathAttrs, lc)
	}

	return rib.NewRoute(n, attrs.NextHop, pathAttrs)
}

// queueRoutesToCommit queues routes to the active commit for the peer.
// Returns error if no commit is active.
func queueRoutesToCommit(ctx *CommandContext, attrs BatchAttributes, afi, safi string, prefixes []netip.Prefix) (*Response, error) {
	// Get active commit for this peer
	peerSelector := ctx.PeerSelector()

	// Find active commit - look for any commit with matching peer
	commits := ctx.CommitManager.List()
	if len(commits) == 0 {
		return &Response{
			Status: "error",
			Data:   "no active commit - use 'commit <name> start' first",
		}, fmt.Errorf("no active commit")
	}

	// Use the first (most recent) commit
	// TODO: Support explicit commit name in command
	commitName := commits[0]
	tx, err := ctx.CommitManager.Get(commitName)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("commit not found: %v", err),
		}, err
	}

	// Queue each route
	for _, prefix := range prefixes {
		route := buildRoute(prefix, attrs, afi, safi)
		tx.QueueAnnounce(route)
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"commit":   commitName,
			"peer":     peerSelector,
			"family":   afi + " " + safi,
			"prefixes": len(prefixes),
			"queued":   tx.Count(),
		},
	}, nil
}

// ErrMissingLabel is returned when label is required but not provided.
var ErrMissingLabel = errors.New("missing label")

// ErrInvalidLabel is returned when label value is out of range.
var ErrInvalidLabel = errors.New("invalid label")

// MaxMPLSLabel is the maximum valid MPLS label value (20 bits).
const MaxMPLSLabel = 1048575

// validateRD validates Route Distinguisher format per RFC 4364.
// Valid formats:
//   - Type 0: <2-byte ASN>:<4-byte value> (e.g., "65000:100")
//   - Type 1: <4-byte IP>:<2-byte value> (e.g., "1.2.3.4:100")
//   - Type 2: <4-byte ASN>:<2-byte value> (e.g., "4200000000:100")
func validateRD(rd string) error {
	parts := strings.SplitN(rd, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w: must be in format 'value:value', got '%s'", ErrInvalidRD, rd)
	}

	prefix, suffix := parts[0], parts[1]

	// Check if prefix is IP address (Type 1)
	if strings.Contains(prefix, ".") {
		ip, err := netip.ParseAddr(prefix)
		if err != nil || !ip.Is4() {
			return fmt.Errorf("%w: invalid IP in '%s'", ErrInvalidRD, rd)
		}
		// Suffix must be 16-bit for Type 1
		val, err := strconv.ParseUint(suffix, 10, 16)
		if err != nil || val > 65535 {
			return fmt.Errorf("%w: suffix must be 0-65535 for IP:value format, got '%s'", ErrInvalidRD, suffix)
		}
		return nil
	}

	// Prefix is ASN (Type 0 or Type 2)
	prefixVal, err := strconv.ParseUint(prefix, 10, 32)
	if err != nil {
		return fmt.Errorf("%w: invalid ASN '%s'", ErrInvalidRD, prefix)
	}

	suffixVal, err := strconv.ParseUint(suffix, 10, 32)
	if err != nil {
		return fmt.Errorf("%w: invalid value '%s'", ErrInvalidRD, suffix)
	}

	// Type 0: 2-byte ASN : 4-byte value (suffix is uint32, always valid)
	if prefixVal <= 65535 {
		return nil
	}
	// Type 2: 4-byte ASN : 2-byte value
	if suffixVal <= 65535 {
		return nil
	}

	return fmt.Errorf("%w: 4-byte ASN requires 2-byte value (0-65535), got %d", ErrInvalidRD, suffixVal)
}

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

// announceL3VPNImpl handles L3VPN route announcements.
// Args format: <prefix> rd <rd> label <label|[labels...]> next-hop <nh> [attributes...].
func announceL3VPNImpl(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}

	route, err := parseL3VPNAttributes(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	// Validate required fields
	if route.RD == "" {
		return &Response{Status: "error", Data: "missing rd (route-distinguisher)"}, ErrMissingRD
	}
	if err := validateRD(route.RD); err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}
	if len(route.Labels) == 0 {
		return &Response{Status: "error", Data: "missing label"}, ErrMissingLabel
	}
	if !route.NextHop.IsValid() {
		return &Response{Status: "error", Data: "missing next-hop"}, ErrMissingNextHop
	}

	peerSelector := ctx.PeerSelector()
	if err := ctx.Reactor.AnnounceL3VPN(peerSelector, route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce L3VPN: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":   peerSelector,
			"prefix": route.Prefix.String(),
			"rd":     route.RD,
			"labels": route.Labels,
		},
	}, nil
}

// withdrawL3VPNImpl handles L3VPN route withdrawals.
// Args format: <prefix> rd <rd>.
func withdrawL3VPNImpl(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}

	route, err := parseL3VPNAttributes(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	// RD is required for withdrawal to identify the VPN route
	if route.RD == "" {
		return &Response{Status: "error", Data: "missing rd (route-distinguisher)"}, ErrMissingRD
	}
	if err := validateRD(route.RD); err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	peerSelector := ctx.PeerSelector()
	if err := ctx.Reactor.WithdrawL3VPN(peerSelector, route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw L3VPN: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":   peerSelector,
			"prefix": route.Prefix.String(),
			"rd":     route.RD,
		},
	}, nil
}

// announceLabeledUnicastImpl handles MPLS labeled unicast route announcements (SAFI 4).
// Args format: <prefix> label <labels> next-hop <addr> [attributes...] [split /N].
func announceLabeledUnicastImpl(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}

	route, err := parseLabeledUnicastAttributes(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	// Validate required fields
	if len(route.Labels) == 0 {
		return &Response{Status: "error", Data: "missing label"}, ErrMissingLabel
	}
	if !route.NextHop.IsValid() {
		return &Response{Status: "error", Data: "missing next-hop"}, ErrMissingNextHop
	}

	peerSelector := ctx.PeerSelector()

	// Check for split argument
	splitLen, hasSplit := parseSplitArg(args)

	// Handle split: announce multiple prefixes with same label
	if hasSplit {
		prefixes, err := splitPrefix(route.Prefix, splitLen)
		if err != nil {
			return &Response{
				Status: "error",
				Data:   err.Error(),
			}, err
		}

		// Announce each split prefix separately with same labels
		for _, p := range prefixes {
			splitRoute := route
			splitRoute.Prefix = p
			if err := ctx.Reactor.AnnounceLabeledUnicast(peerSelector, splitRoute); err != nil {
				return &Response{
					Status: "error",
					Data:   fmt.Sprintf("failed to announce %s: %v", p.String(), err),
				}, err
			}
		}

		return &Response{
			Status: "done",
			Data: map[string]any{
				"peer":           peerSelector,
				"prefix":         route.Prefix.String(),
				"labels":         route.Labels,
				"split":          splitLen,
				"prefixes_count": len(prefixes),
			},
		}, nil
	}

	// Single prefix announcement
	if err := ctx.Reactor.AnnounceLabeledUnicast(peerSelector, route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce labeled-unicast: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":   peerSelector,
			"prefix": route.Prefix.String(),
			"labels": route.Labels,
		},
	}, nil
}

// withdrawLabeledUnicastImpl handles MPLS labeled unicast route withdrawals.
// Args format: <prefix> label <labels>.
func withdrawLabeledUnicastImpl(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{Status: "error", Data: "missing prefix"}, ErrMissingPrefix
	}

	route, err := parseLabeledUnicastAttributes(args)
	if err != nil {
		return &Response{Status: "error", Data: err.Error()}, err
	}

	peerSelector := ctx.PeerSelector()
	if err := ctx.Reactor.WithdrawLabeledUnicast(peerSelector, route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw labeled-unicast: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":   peerSelector,
			"prefix": route.Prefix.String(),
		},
	}, nil
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

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against MPLS keywords (not VPN - no RD/RT)
		if !MPLSKeywords[key] {
			return LabeledUnicastRoute{}, fmt.Errorf("%w: '%s' not valid for labeled-unicast", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing first (writes directly to embedded PathAttributes)
		consumed, err := parseCommonAttribute(key, args, i, &route.PathAttributes)
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

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against VPN keywords
		if !VPNKeywords[key] {
			return L3VPNRoute{}, fmt.Errorf("%w: '%s' not valid for L3VPN", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing first (writes directly to embedded PathAttributes)
		consumed, err := parseCommonAttribute(key, args, i, &route.PathAttributes)
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

	return route, nil
}

// handleAnnounceEOR handles: announce eor [family].
// Example: announce eor (sends IPv4 unicast EOR).
// Example: announce eor ipv4/unicast.
// Example: announce eor ipv6/unicast.
func handleAnnounceEOR(ctx *CommandContext, args []string) (*Response, error) {
	// Default to IPv4 unicast
	afi := uint16(1) // IPv4
	safi := uint8(1) // Unicast
	family := AFINameIPv4 + "/" + SAFINameUnicast

	// Parse optional family - supports "ipv4/unicast" or "ipv4 unicast"
	if len(args) >= 1 {
		var afiStr, safiStr string

		switch {
		case strings.Contains(args[0], "/"):
			// Slash format (ipv4/unicast)
			parts := strings.SplitN(args[0], "/", 2)
			afiStr = strings.ToLower(parts[0])
			safiStr = strings.ToLower(parts[1])
		case len(args) >= 2:
			// Space-separated format (ipv4 unicast)
			afiStr = strings.ToLower(args[0])
			safiStr = strings.ToLower(args[1])
		default:
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("invalid family format: %s (expected afi/safi)", args[0]),
			}, ErrInvalidFamily
		}

		switch afiStr {
		case AFINameIPv4:
			afi = 1
		case AFINameIPv6:
			afi = 2
		case AFINameL2VPN:
			afi = 25
		default:
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("unknown AFI: %s", afiStr),
			}, ErrInvalidFamily
		}

		switch safiStr {
		case SAFINameUnicast:
			safi = 1
		case SAFINameMulticast:
			safi = 2
		case SAFINameEVPN:
			safi = 70
		case "vpn", SAFINameMPLSVPN:
			safi = 128
		case SAFINameFlowSpec:
			safi = 133
		default:
			return &Response{
				Status: "error",
				Data:   fmt.Sprintf("unknown SAFI: %s", safiStr),
			}, ErrInvalidFamily
		}

		family = afiStr + "/" + safiStr
	}

	// Send EOR to all established peers
	if err := ctx.Reactor.AnnounceEOR("*", afi, safi); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to send EOR: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"family": family,
		},
	}, nil
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
				// next-hop self is handled by the reactor
				continue
			}
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("%w: %s", ErrInvalidNextHop, value)
			}
			route.NextHop = nh

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

// handleAnnounceFlow handles: announce flow [match|then] ....
// Example: announce flow match destination 10.0.0.0/24 protocol tcp then discard.
// Example: announce flow match source 192.168.1.0/24 destination-port 80 then rate-limit 1000000.
func handleAnnounceFlow(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 2 {
		return &Response{
			Status: "error",
			Data:   "usage: announce flow match <spec> then <action>",
		}, fmt.Errorf("insufficient arguments")
	}

	route, err := ParseFlowSpecArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if err := ctx.Reactor.AnnounceFlowSpec("*", route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce flowspec: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type":   "flowspec",
			"family": route.Family,
		},
	}, nil
}

// handleWithdrawFlow handles: withdraw flow [match] ...
func handleWithdrawFlow(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 2 {
		return &Response{
			Status: "error",
			Data:   "usage: withdraw flow match <spec>",
		}, fmt.Errorf("insufficient arguments")
	}

	route, err := ParseFlowSpecArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if err := ctx.Reactor.WithdrawFlowSpec("*", route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw flowspec: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type":   "flowspec",
			"family": route.Family,
		},
	}, nil
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

// handleAnnounceVPLS handles: announce vpls rd <rd> ... next-hop <addr>.
func handleAnnounceVPLS(ctx *CommandContext, args []string) (*Response, error) {
	route, err := ParseVPLSArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if err := ctx.Reactor.AnnounceVPLS("*", route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce vpls: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type": "vpls",
			"rd":   route.RD,
		},
	}, nil
}

// handleWithdrawVPLS handles: withdraw vpls rd <rd>.
func handleWithdrawVPLS(ctx *CommandContext, args []string) (*Response, error) {
	route, err := ParseVPLSArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if err := ctx.Reactor.WithdrawVPLS("*", route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw vpls: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type": "vpls",
			"rd":   route.RD,
		},
	}, nil
}

// ParseVPLSArgs parses VPLS command arguments.
// Format: rd <rd> ve-block-offset <n> ve-block-size <n> label <n> next-hop <addr>.
func ParseVPLSArgs(args []string) (VPLSRoute, error) {
	var route VPLSRoute

	for i := 0; i < len(args)-1; i += 2 {
		key := strings.ToLower(args[i])
		value := args[i+1]

		switch key {
		case "rd":
			route.RD = value
		case "ve-block-offset":
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return route, fmt.Errorf("invalid ve-block-offset: %s", value)
			}
			route.VEBlockOffset = uint16(n)
		case "ve-block-size":
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return route, fmt.Errorf("invalid ve-block-size: %s", value)
			}
			route.VEBlockSize = uint16(n)
		case "label-base", "label":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label: %s", value)
			}
			route.LabelBase = uint32(n)
		case "next-hop":
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("%w: %s", ErrInvalidNextHop, value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown vpls keyword: %s", key)
		}
	}

	if route.RD == "" {
		return route, ErrMissingRD
	}

	return route, nil
}

// handleAnnounceL2VPN handles: announce l2vpn <type> ....
// Example: announce l2vpn mac-ip rd 1:1 mac 00:11:22:33:44:55 label 100 next-hop 192.168.1.1.
// Example: announce l2vpn ip-prefix rd 1:1 prefix 10.0.0.0/24 label 100 next-hop 192.168.1.1.
func handleAnnounceL2VPN(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: announce l2vpn <mac-ip|ip-prefix|multicast> ...",
		}, ErrMissingRouteType
	}

	route, err := parseL2VPNArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if err := ctx.Reactor.AnnounceL2VPN("*", route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to announce l2vpn: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type":       AFINameL2VPN,
			"route_type": route.RouteType,
			"rd":         route.RD,
		},
	}, nil
}

// handleWithdrawL2VPN handles: withdraw l2vpn <type> ...
func handleWithdrawL2VPN(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Data:   "usage: withdraw l2vpn <mac-ip|ip-prefix|multicast> ...",
		}, ErrMissingRouteType
	}

	route, err := parseL2VPNArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Data:   err.Error(),
		}, err
	}

	if err := ctx.Reactor.WithdrawL2VPN("*", route); err != nil {
		return &Response{
			Status: "error",
			Data:   fmt.Sprintf("failed to withdraw l2vpn: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type":       AFINameL2VPN,
			"route_type": route.RouteType,
			"rd":         route.RD,
		},
	}, nil
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

// ParseL2VPNArgs parses L2VPN/EVPN command arguments.
// Exported for use by encode command.
func ParseL2VPNArgs(args []string) (L2VPNRoute, error) {
	return parseL2VPNArgs(args)
}

// parseL2VPNArgs parses L2VPN/EVPN command arguments.
func parseL2VPNArgs(args []string) (L2VPNRoute, error) {
	var route L2VPNRoute

	if len(args) < 1 {
		return route, ErrMissingRouteType
	}

	// First argument is route type
	routeType := strings.ToLower(args[0])
	switch routeType { //nolint:goconst // String literals are clearer here
	case "mac-ip", "macip", "type2":
		route.RouteType = "mac-ip" //nolint:goconst // String literal is assignment value
	case "ip-prefix", "ipprefix", "type5":
		route.RouteType = "ip-prefix" //nolint:goconst // String literal is assignment value
	case "multicast", "inclusive-multicast", "type3":
		route.RouteType = "multicast"
	case "ethernet-segment", "es", "type4":
		route.RouteType = "ethernet-segment"
	case "ethernet-ad", "ead", "type1":
		route.RouteType = "ethernet-ad"
	default:
		return route, fmt.Errorf("%w: %s", ErrInvalidRouteType, routeType)
	}

	// Parse remaining key-value pairs
	for i := 1; i < len(args)-1; i += 2 {
		key := strings.ToLower(args[i])
		value := args[i+1]

		switch key {
		case "rd":
			route.RD = value
		case "esi":
			route.ESI = value
		case "ethernet-tag", "etag":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid ethernet-tag: %s", value)
			}
			route.EthernetTag = uint32(n)
		case "mac":
			route.MAC = value
		case "ip":
			ip, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid ip: %s", value)
			}
			route.IP = ip
		case "prefix":
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return route, fmt.Errorf("%w: %s", ErrInvalidPrefix, value)
			}
			route.Prefix = prefix
		case "gateway", "gw":
			gw, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid gateway: %s", value)
			}
			route.Gateway = gw
		case "label", "label1":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label: %s", value)
			}
			route.Label1 = uint32(n)
		case "label2":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label2: %s", value)
			}
			route.Label2 = uint32(n)
		case "next-hop":
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("%w: %s", ErrInvalidNextHop, value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown l2vpn keyword: %s", key)
		}
	}

	// Validate required fields based on route type
	if route.RD == "" {
		return route, ErrMissingRD
	}

	if route.RouteType == "mac-ip" && route.MAC == "" {
		return route, ErrMissingMAC
	}

	if route.RouteType == "ip-prefix" && !route.Prefix.IsValid() {
		return route, ErrMissingPrefix
	}

	return route, nil
}

// handleAnnounceWatchdog handles: announce watchdog <name>
// Announces all routes in the named watchdog group that are currently withdrawn.
func handleAnnounceWatchdog(ctx *CommandContext, args []string) (*Response, error) {
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

// handleWithdrawWatchdog handles: withdraw watchdog <name>
// Withdraws all routes in the named watchdog group that are currently announced.
func handleWithdrawWatchdog(ctx *CommandContext, args []string) (*Response, error) {
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

	// Parse remaining args as key-value pairs
	for i := 2; i < len(args); i++ {
		key := strings.ToLower(args[i])

		switch key {
		case "rd":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing rd value")
			}
			spec.RD = args[i+1]
			i++

		case "next-hop":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing next-hop value")
			}
			spec.NextHop = args[i+1]
			i++

		case "teid":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing teid value")
			}
			spec.TEID = args[i+1]
			i++

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

		case "endpoint":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing endpoint value")
			}
			spec.Endpoint = args[i+1]
			i++

		case "source":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing source value")
			}
			spec.Source = args[i+1]
			i++

		case "extended-community":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing extended-community value")
			}
			// Collect bracketed value
			tokens, consumed := parseBracketedList(args[i+1:])
			spec.ExtCommunity = "[" + strings.Join(tokens, " ") + "]"
			i += consumed

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

		case "origin":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing origin value")
			}
			origin, err := parseOrigin(args[i+1])
			if err != nil {
				return spec, err
			}
			spec.Origin = &origin
			i++

		case "local-preference":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing local-preference value")
			}
			lp, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return spec, fmt.Errorf("invalid local-preference: %s", args[i+1])
			}
			lpVal := uint32(lp)
			spec.LocalPreference = &lpVal
			i++

		case "as-path":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing as-path value")
			}
			tokens, consumed := parseBracketedList(args[i+1:])
			for _, tok := range tokens {
				asn, err := strconv.ParseUint(tok, 10, 32)
				if err != nil {
					return spec, fmt.Errorf("invalid ASN in as-path: %s", tok)
				}
				spec.ASPath = append(spec.ASPath, uint32(asn))
			}
			i += consumed

		default:
			// Unknown keyword - could be future extension, skip silently
		}
	}

	return spec, nil
}
