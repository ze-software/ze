//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package api

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
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

// RegisterRouteHandlers registers route-related command handlers.
func RegisterRouteHandlers(d *Dispatcher) {
	// Announce commands
	d.Register("announce route", handleAnnounceRoute, "Announce a route to peers")
	d.Register("announce eor", handleAnnounceEOR, "Send End-of-RIB marker")
	d.Register("announce flow", handleAnnounceFlow, "Announce a FlowSpec route")
	d.Register("announce vpls", handleAnnounceVPLS, "Announce a VPLS route")
	d.Register("announce l2vpn", handleAnnounceL2VPN, "Announce an L2VPN/EVPN route")

	// Withdraw commands
	d.Register("withdraw route", handleWithdrawRoute, "Withdraw a route from peers")
	d.Register("withdraw flow", handleWithdrawFlow, "Withdraw a FlowSpec route")
	d.Register("withdraw vpls", handleWithdrawVPLS, "Withdraw a VPLS route")
	d.Register("withdraw l2vpn", handleWithdrawL2VPN, "Withdraw an L2VPN/EVPN route")
}

// handleAnnounceRoute handles: announce route <prefix> next-hop <addr> [attributes...] [split /N].
// Example: announce route 10.0.0.0/24 next-hop 192.168.1.1.
// Example: announce route 10.0.0.0/24 next-hop self.
// Example: announce route 10.0.0.0/24 next-hop 1.2.3.4 origin igp local-preference 100 med 200 community [2:1].
// Example: announce route 10.0.0.0/21 next-hop 1.2.3.4 split /23 (announces 4 /23 prefixes).
func handleAnnounceRoute(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 3 {
		return &Response{
			Status: "error",
			Error:  "usage: announce route <prefix> next-hop <addr|self>",
		}, ErrMissingPrefix
	}

	// Parse prefix (first arg)
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("invalid prefix: %s", args[0]),
		}, ErrInvalidPrefix
	}

	route := RouteSpec{
		Prefix: prefix,
	}

	// Check for split argument
	splitLen, hasSplit := parseSplitArg(args)

	// Parse remaining args as key-value pairs
	nextHopSelf := false
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		switch key {
		case "next-hop":
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing next-hop value"}, ErrMissingNextHop
			}
			nhStr := args[i+1]
			if strings.EqualFold(nhStr, "self") {
				nextHopSelf = true
			} else {
				nh, err := netip.ParseAddr(nhStr)
				if err != nil {
					return &Response{Status: "error", Error: fmt.Sprintf("invalid next-hop: %s", nhStr)}, ErrInvalidNextHop
				}
				route.NextHop = nh
			}
			i++

		case "origin":
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing origin value"}, fmt.Errorf("missing origin value")
			}
			origin, err := parseOrigin(args[i+1])
			if err != nil {
				return &Response{Status: "error", Error: err.Error()}, err
			}
			route.Origin = &origin
			i++

		case "local-preference":
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing local-preference value"}, fmt.Errorf("missing local-preference value")
			}
			lp, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return &Response{Status: "error", Error: fmt.Sprintf("invalid local-preference: %s", args[i+1])}, err
			}
			lpVal := uint32(lp)
			route.LocalPreference = &lpVal
			i++

		case "med":
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing med value"}, fmt.Errorf("missing med value")
			}
			med, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return &Response{Status: "error", Error: fmt.Sprintf("invalid med: %s", args[i+1])}, err
			}
			medVal := uint32(med)
			route.MED = &medVal
			i++

		case "as-path":
			// Parse as-path [ ASN1 ASN2 ... ] or as-path [ASN1,ASN2,...]
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing as-path value"}, fmt.Errorf("missing as-path value")
			}
			asPath, consumed, err := parseASPath(args[i+1:])
			if err != nil {
				return &Response{Status: "error", Error: err.Error()}, err
			}
			route.ASPath = asPath
			i += consumed

		case "community":
			// Parse community [ASN:VAL ASN:VAL ...] or community [ASN:VAL,ASN:VAL]
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing community value"}, fmt.Errorf("missing community value")
			}
			comms, consumed, err := parseCommunities(args[i+1:])
			if err != nil {
				return &Response{Status: "error", Error: err.Error()}, err
			}
			route.Communities = comms
			i += consumed

		case "large-community":
			// Parse large-community [GA:LD1:LD2 ...]
			if i+1 >= len(args) {
				return &Response{Status: "error", Error: "missing large-community value"}, fmt.Errorf("missing large-community value")
			}
			lcomms, consumed, err := parseLargeCommunities(args[i+1:])
			if err != nil {
				return &Response{Status: "error", Error: err.Error()}, err
			}
			route.LargeCommunities = lcomms
			i += consumed

		case "split":
			// Already parsed above, just skip the value
			i++
		}
	}

	if !nextHopSelf && !route.NextHop.IsValid() {
		return &Response{
			Status: "error",
			Error:  "missing next-hop",
		}, ErrMissingNextHop
	}

	peerSelector := ctx.PeerSelector()

	// Handle split: announce multiple prefixes
	if hasSplit {
		prefixes, err := splitPrefix(prefix, splitLen)
		if err != nil {
			return &Response{
				Status: "error",
				Error:  err.Error(),
			}, err
		}

		// Announce each split prefix separately
		for _, p := range prefixes {
			splitRoute := route
			splitRoute.Prefix = p
			if err := ctx.Reactor.AnnounceRoute(peerSelector, splitRoute); err != nil {
				return &Response{
					Status: "error",
					Error:  fmt.Sprintf("failed to announce %s: %v", p.String(), err),
				}, err
			}
		}

		return &Response{
			Status: "done",
			Data: map[string]any{
				"peer":           peerSelector,
				"prefix":         prefix.String(),
				"split":          splitLen,
				"prefixes_count": len(prefixes),
			},
		}, nil
	}

	// Announce single route
	if err := ctx.Reactor.AnnounceRoute(peerSelector, route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to announce: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"peer":     peerSelector,
			"prefix":   prefix.String(),
			"next_hop": route.NextHop.String(),
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

// Well-known community values per RFC 1997 and RFC 3765.
const (
	// CommunityNoExport - RFC 1997: Do not advertise outside confederation.
	CommunityNoExport uint32 = 0xFFFFFF01
	// CommunityNoAdvertise - RFC 1997: Do not advertise to any peer.
	CommunityNoAdvertise uint32 = 0xFFFFFF02
	// CommunityNoExportSubconfed - RFC 1997: Do not advertise to external peers.
	CommunityNoExportSubconfed uint32 = 0xFFFFFF03
	// CommunityNoPeer - RFC 3765: Do not advertise to peers.
	CommunityNoPeer uint32 = 0xFFFFFF04
	// CommunityBlackhole - RFC 7999: Trigger remote blackholing.
	CommunityBlackhole uint32 = 0xFFFF029A
)

// parseCommunity parses a single community value.
// Supports:
//   - ASN:VAL format per RFC 1997
//   - Well-known names: no-export, no-advertise, no-export-subconfed, nopeer, blackhole
func parseCommunity(s string) (uint32, error) {
	if s == "" {
		return 0, fmt.Errorf("empty community value")
	}

	// Check for well-known community names (case-insensitive)
	lower := strings.ToLower(s)
	switch lower {
	case "no-export", "no_export":
		return CommunityNoExport, nil
	case "no-advertise", "no_advertise":
		return CommunityNoAdvertise, nil
	case "no-export-subconfed", "no_export_subconfed":
		return CommunityNoExportSubconfed, nil
	case "nopeer", "no-peer", "no_peer":
		return CommunityNoPeer, nil
	case "blackhole":
		return CommunityBlackhole, nil
	}

	// Parse ASN:VAL format
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid community format: %s (expected ASN:VAL or well-known name)", s)
	}
	asn, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community ASN: %s", parts[0])
	}
	val, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid community value: %s", parts[1])
	}
	return uint32(asn)<<16 | uint32(val), nil
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
func parseLargeCommunity(s string) (LargeCommunity, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return LargeCommunity{}, fmt.Errorf("invalid large-community format: %s (expected GA:LD1:LD2)", s)
	}
	ga, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community global-admin: %s", parts[0])
	}
	ld1, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local-data1: %s", parts[1])
	}
	ld2, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return LargeCommunity{}, fmt.Errorf("invalid large-community local-data2: %s", parts[2])
	}
	return LargeCommunity{
		GlobalAdmin: uint32(ga),
		LocalData1:  uint32(ld1),
		LocalData2:  uint32(ld2),
	}, nil
}

// handleWithdrawRoute handles: withdraw route <prefix>.
// Example: withdraw route 10.0.0.0/24.
func handleWithdrawRoute(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: "error",
			Error:  "usage: withdraw route <prefix>",
		}, ErrMissingPrefix
	}

	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("invalid prefix: %s", args[0]),
		}, ErrInvalidPrefix
	}

	// Withdraw from matching peers (default "*" for all)
	peerSelector := ctx.PeerSelector()
	if err := ctx.Reactor.WithdrawRoute(peerSelector, prefix); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to withdraw: %v", err),
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

// handleAnnounceEOR handles: announce eor [family].
// Example: announce eor (sends IPv4 unicast EOR).
// Example: announce eor ipv4 unicast.
// Example: announce eor ipv6 unicast.
func handleAnnounceEOR(ctx *CommandContext, args []string) (*Response, error) {
	// Default to IPv4 unicast
	afi := uint16(1) // IPv4
	safi := uint8(1) // Unicast
	family := "ipv4 unicast"

	// Parse optional family
	if len(args) >= 2 {
		afiStr := strings.ToLower(args[0])
		safiStr := strings.ToLower(args[1])

		switch afiStr { //nolint:goconst // String literals are clearer here
		case "ipv4":
			afi = 1
		case "ipv6":
			afi = 2
		case "l2vpn":
			afi = 25
		default:
			return &Response{
				Status: "error",
				Error:  fmt.Sprintf("unknown AFI: %s", afiStr),
			}, ErrInvalidFamily
		}

		switch safiStr { //nolint:goconst // String literals are clearer here
		case "unicast":
			safi = 1
		case "multicast":
			safi = 2
		case "evpn":
			safi = 70
		case "vpn", "mpls-vpn":
			safi = 128
		case "flowspec":
			safi = 133
		default:
			return &Response{
				Status: "error",
				Error:  fmt.Sprintf("unknown SAFI: %s", safiStr),
			}, ErrInvalidFamily
		}

		family = afiStr + " " + safiStr
	}

	// Send EOR to all established peers
	if err := ctx.Reactor.AnnounceEOR("*", afi, safi); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to send EOR: %v", err),
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
			Error:  "usage: announce flow match <spec> then <action>",
		}, fmt.Errorf("insufficient arguments")
	}

	route, err := parseFlowSpecArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	if err := ctx.Reactor.AnnounceFlowSpec("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to announce flowspec: %v", err),
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
			Error:  "usage: withdraw flow match <spec>",
		}, fmt.Errorf("insufficient arguments")
	}

	route, err := parseFlowSpecArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	if err := ctx.Reactor.WithdrawFlowSpec("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to withdraw flowspec: %v", err),
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

// parseFlowSpecArgs parses FlowSpec command arguments.
func parseFlowSpecArgs(args []string) (FlowSpecRoute, error) {
	var route FlowSpecRoute
	route.Family = "ipv4" // default

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

		if inMatch {
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
					route.Family = "ipv6"
				}
				i++

			case "source":
				prefix, err := netip.ParsePrefix(value)
				if err != nil {
					return route, fmt.Errorf("%w: %s", ErrInvalidPrefix, value)
				}
				route.SourcePrefix = &prefix
				if prefix.Addr().Is6() {
					route.Family = "ipv6"
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
			}
		}

		if inThen {
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
	route, err := parseVPLSArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	if err := ctx.Reactor.AnnounceVPLS("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to announce vpls: %v", err),
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
	route, err := parseVPLSArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	if err := ctx.Reactor.WithdrawVPLS("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to withdraw vpls: %v", err),
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

// parseVPLSArgs parses VPLS command arguments.
func parseVPLSArgs(args []string) (VPLSRoute, error) {
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
			Error:  "usage: announce l2vpn <mac-ip|ip-prefix|multicast> ...",
		}, ErrMissingRouteType
	}

	route, err := parseL2VPNArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	if err := ctx.Reactor.AnnounceL2VPN("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to announce l2vpn: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type":       "l2vpn",
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
			Error:  "usage: withdraw l2vpn <mac-ip|ip-prefix|multicast> ...",
		}, ErrMissingRouteType
	}

	route, err := parseL2VPNArgs(args)
	if err != nil {
		return &Response{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	if err := ctx.Reactor.WithdrawL2VPN("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to withdraw l2vpn: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"type":       "l2vpn",
			"route_type": route.RouteType,
			"rd":         route.RD,
		},
	}, nil
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
