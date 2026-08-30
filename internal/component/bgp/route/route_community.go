// Design: docs/architecture/route-types.md — community route parsing
// Overview: route.go — core route types and attribute parsing

package route

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

var (
	errTrafficRateRequiresAsnRate = errors.New("traffic-rate requires <asn> <rate> [bytes|packets]")
	errRedirectRequiresAsnTarget  = errors.New("redirect requires <asn> <target>")
	errTrafficMarkingRequiresDscp = errors.New("traffic-marking requires <dscp>")
	errEmptyExtendedCommunity     = errors.New("empty extended community")
)

// parseCommunities parses communities in format [ASN:VAL ASN:VAL ...].
// Returns the parsed communities and how many tokens were consumed.
func parseCommunities(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, errMissingCommunityValue
	}

	tokens, consumed := attribute.ParseBracketedList(args)
	comms := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		comm, err := attribute.ParseCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		comms = append(comms, comm)
	}

	return comms, consumed, nil
}

// parseLargeCommunities parses large communities in format [GA:LD1:LD2 ...].
func parseLargeCommunities(args []string) ([]bgptypes.LargeCommunity, int, error) {
	if len(args) == 0 {
		return nil, 0, errMissingLargeCommunityValue
	}

	tokens, consumed := attribute.ParseBracketedList(args)
	lcomms := make([]bgptypes.LargeCommunity, 0, len(tokens))
	for _, tok := range tokens {
		lc, err := attribute.ParseLargeCommunity(tok)
		if err != nil {
			return nil, consumed, err
		}
		lcomms = append(lcomms, lc)
	}

	return lcomms, consumed, nil
}

// ParseExtendedCommunities parses extended communities in format [type:value:value ...].
// RFC 4360 (Extended Communities), RFC 8955 (FlowSpec Actions).
//
// Supported formats:
//   - List syntax: [origin:ASN:IP] or [redirect:ASN:target] etc.
//   - Function syntax: traffic-rate <asn> <rate>, redirect <asn> <target>, etc.
//
// List format types:
//   - origin:ASN:IP (Type 0x00, Subtype 0x03) - 2-byte ASN + IPv4
//   - origin:IP:ASN (Type 0x01, Subtype 0x03) - IPv4 + 2-byte ASN
//   - redirect:ADMIN:value (Sub-type 0x08) - Traffic redirect to a route-target
//   - rate-limit:bps (Type 0x80, Subtype 0x06) - Traffic rate limit in bytes/sec
//   - rate-limit:bps:packets (Type 0x80, Subtype 0x0c) - Traffic rate limit in packets/sec
//
// Function format types (RFC 8955 FlowSpec actions):
//   - traffic-rate <asn> <rate> [bytes|packets] - Rate limit in bytes/sec or packets/sec
//   - discard - Sugar for traffic-rate 0 0
//   - redirect <asn> <target> - Redirect to VRF
//   - traffic-marking <dscp> - Set DSCP value
func ParseExtendedCommunities(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) == 0 {
		return nil, 0, errMissingExtendedCommunityValue
	}

	// Check for function-style syntax (FlowSpec actions)
	firstToken := strings.ToLower(args[0])
	switch firstToken {
	case "traffic-rate":
		return parseTrafficRateFunction(args)
	case "traffic-rate-packets":
		return parseTrafficRatePacketsFunction(args)
	case flowspecActionDiscard:
		return parseDiscardFunction()
	case flowspecActionRedirect:
		return parseRedirectFunction(args)
	case "traffic-marking":
		return parseTrafficMarkingFunction(args)
	}

	// Fall back to list syntax
	tokens, consumed := attribute.ParseBracketedList(args)
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

// parseTrafficRateFunction parses: traffic-rate <asn> <rate> [bytes|packets]
// RFC 8955 Section 7.1: Traffic-rate-bytes action (Type 0x80, Subtype 0x06).
// RFC 8955 Section 7.2: Traffic-rate-packets action (Type 0x80, Subtype 0x0c).
// Format: 2-byte ASN + 4-byte IEEE 754 float.
// Rate of 0 means discard (drop all matching traffic).
func parseTrafficRateFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 3 {
		return nil, 0, errTrafficRateRequiresAsnRate
	}

	asn, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ASN in traffic-rate: %s", args[1])
	}

	unit := attribute.FlowSpecRateBytes
	consumed := 3
	if len(args) > 3 {
		switch strings.ToLower(args[3]) {
		case "bytes":
			consumed = 4
		case "packets":
			unit = attribute.FlowSpecRatePackets
			consumed = 4
		}
	}

	rate, err := strconv.ParseFloat(args[2], 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid rate in traffic-rate: %s", args[2])
	}

	ec, err := attribute.FlowSpecTrafficRate(unit, uint16(asn), rate)
	if err != nil {
		return nil, 0, fmt.Errorf("traffic-rate: %w", err)
	}

	return []attribute.ExtendedCommunity{ec}, consumed, nil
}

// parseTrafficRatePacketsFunction accepts ExaBGP 5.0 packet-rate function spelling.
func parseTrafficRatePacketsFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 3 {
		return nil, 0, errTrafficRateRequiresAsnRate
	}
	aliasArgs := []string{"traffic-rate", args[1], args[2], "packets"}
	comms, _, err := parseTrafficRateFunction(aliasArgs)
	return comms, 3, err
}

// parseDiscardFunction parses: discard
// RFC 8955 Section 7.1: sugar for traffic-rate 0, which the section gives the
// meaning "all traffic for the particular flow to be discarded".
func parseDiscardFunction() ([]attribute.ExtendedCommunity, int, error) {
	ec, err := attribute.FlowSpecTrafficRate(attribute.FlowSpecRateBytes, 0, 0)
	if err != nil {
		return nil, 0, err
	}
	return []attribute.ExtendedCommunity{ec}, 1, nil
}

// parseRedirectFunction parses: redirect <administrator> <value>
// RFC 8955 Section 7.4: rt-redirect action, sub-type 0x08.
// The administrator is a 2-octet AS, a 4-octet AS, or an IPv4 address, and it
// selects the type octet (0x80, 0x82, 0x81) and the width of the value.
func parseRedirectFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 3 {
		return nil, 0, errRedirectRequiresAsnTarget
	}

	ec, err := attribute.FlowSpecRedirect(args[1], args[2])
	if err != nil {
		return nil, 0, err
	}

	return []attribute.ExtendedCommunity{ec}, 3, nil
}

// parseTrafficMarkingFunction parses: traffic-marking <dscp>
// RFC 8955 Section 7.5: Traffic-marking action (Type 0x80, Subtype 0x09).
// Format: 5 reserved octets, then the DSCP in the 6 least significant bits of
// the last one. Sets the DSCP bits in the IP TOS/Traffic Class field.
func parseTrafficMarkingFunction(args []string) ([]attribute.ExtendedCommunity, int, error) {
	if len(args) < 2 {
		return nil, 0, errTrafficMarkingRequiresDscp
	}

	dscp, err := strconv.ParseUint(args[1], 10, 8)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid DSCP in traffic-marking: %s", args[1])
	}

	ec, err := attribute.FlowSpecTrafficMarking(dscp)
	if err != nil {
		return nil, 0, err
	}

	return []attribute.ExtendedCommunity{ec}, 2, nil
}

// parseExtendedCommunity parses a single extended community string.
// RFC 4360: Extended communities are 8 octets with Type:Subtype:Value encoding.
// RFC 8955: FlowSpec traffic actions use specific type/subtype combinations.
//
// Formats:
//   - origin:ASN:IP     -> Type 0x00, Subtype 0x03 (2-byte ASN + 4-byte IP)
//   - origin:IP:ASN     -> Type 0x01, Subtype 0x03 (4-byte IP + 2-byte ASN)
//   - redirect:ADMIN:value -> Sub-type 0x08; ADMIN picks type 0x80, 0x81 or 0x82
//   - rate-limit:bps    -> Type 0x80, Subtype 0x06 (IEEE 754 float rate)
//   - rate-limit:pps:packets -> Type 0x80, Subtype 0x0c (IEEE 754 float rate)
func parseExtendedCommunity(s string) (attribute.ExtendedCommunity, error) {
	if s == "" {
		return attribute.ExtendedCommunity{}, errEmptyExtendedCommunity
	}

	// FlowSpec traffic actions written as one word, BEFORE the colon split: they
	// carry no colon, so splitting first reports them as a malformed format rather
	// than as an action this parser does not know. The config path
	// (config/routeattr_community.go parseOneExtCommunity) has always accepted
	// them, so a route an operator could write in config -- `copy-to-nexthop` --
	// could not be expressed through `update text`. One shared table now, so the
	// two vocabularies cannot drift again.
	if ec, ok := attribute.FlowSpecActionKeyword(s); ok {
		return ec, nil
	}

	// Split on first colon to get type prefix
	before, after, ok := strings.Cut(s, ":")
	if !ok {
		return attribute.ExtendedCommunity{}, fmt.Errorf(
			"invalid extended community %q: expected <type>:<value>, or one of %v",
			s, attribute.FlowSpecActionKeywords())
	}

	typePrefix := strings.ToLower(before)
	value := after

	switch typePrefix {
	case "target":
		return parseRouteTargetExtCommunity(value)
	case "origin":
		return parseOriginExtCommunity(value)
	case flowspecActionRedirect:
		return parseRedirectExtCommunity(value)
	case flowspecActionRateLimit:
		return parseRateLimitExtCommunity(value)
	case flowspecActionRateLimitPackets:
		return parseRateLimitExtCommunityWithSubtype(value, attribute.FlowSpecRatePackets)
	default:
		return attribute.ExtendedCommunity{}, fmt.Errorf("unknown extended community type: %s", typePrefix)
	}
}

// parseRouteTargetExtCommunity parses route target extended community.
// RFC 4360: Route Target (subtype 0x02).
// Format: target:ASN:NN (Type 0x00 for 2-byte ASN, Type 0x02 for 4-byte ASN).
//
// The two forms split the 6-octet value field differently. RFC 4360 Section 3.1
// gives type 0x00 a 2-octet global administrator and a 4-octet local
// administrator; RFC 5668 Section 2 gives type 0x02 a 4-octet global
// administrator and a 2-octet local administrator. So the number a four-byte AS
// can carry stops at 65535, and a larger one is refused rather than truncated.
func parseRouteTargetExtCommunity(value string) (attribute.ExtendedCommunity, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid target format: %s", value)
	}

	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid ASN in target: %s", parts[0])
	}

	if asn > 0xFFFF {
		num, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return attribute.ExtendedCommunity{}, fmt.Errorf("invalid value in target: %s (4-byte ASN format max 65535)", parts[1])
		}
		return attribute.ExtendedCommunity{
			0x02, 0x02,
			byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
			byte(num >> 8), byte(num),
		}, nil
	}

	num, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid value in target: %s", parts[1])
	}
	return attribute.ExtendedCommunity{
		0x00, 0x02,
		byte(asn >> 8), byte(asn),
		byte(num >> 24), byte(num >> 16), byte(num >> 8), byte(num),
	}, nil
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
		addr, err := netip.ParseAddr(parts[0])
		if err != nil || !addr.Unmap().Is4() {
			return attribute.ExtendedCommunity{}, fmt.Errorf("invalid IPv4 in origin: %s", parts[0])
		}
		asn, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return attribute.ExtendedCommunity{}, fmt.Errorf("invalid ASN in origin: %s", parts[1])
		}
		ip4 := addr.Unmap().As4()
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
	addr, err := netip.ParseAddr(parts[1])
	if err != nil || !addr.Unmap().Is4() {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid IPv4 in origin: %s", parts[1])
	}
	ip4 := addr.Unmap().As4()
	return attribute.ExtendedCommunity{
		0x00, 0x03, // Type=0, Subtype=3 (Origin)
		byte(asn >> 8), byte(asn), // 2-byte ASN
		ip4[0], ip4[1], ip4[2], ip4[3], // IPv4 address
	}, nil
}

// parseRedirectExtCommunity parses the FlowSpec rt-redirect extended community.
// RFC 8955 Section 7.4: traffic redirect to a VRF that imports the route-target.
// Format: redirect:<administrator>:<value>, RFC 8955 Section 7.4 sub-type 0x08.
func parseRedirectExtCommunity(value string) (attribute.ExtendedCommunity, error) {
	admin, local, ok := strings.Cut(value, ":")
	if !ok {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid redirect format: %s", value)
	}

	return attribute.FlowSpecRedirect(admin, local)
}

// parseRateLimitExtCommunity parses FlowSpec rate-limit extended community.
// RFC 8955 Section 7.1: Traffic rate limiting in bytes per second.
// RFC 8955 Section 7.2: Traffic rate limiting in packets per second.
// Formats:
//   - rate-limit:bps (Type 0x80, Subtype 0x06)
//   - rate-limit:pps:packets (Type 0x80, Subtype 0x0c)
//
// The rate is encoded as an IEEE 754 single-precision float.
func parseRateLimitExtCommunity(value string) (attribute.ExtendedCommunity, error) {
	rate, unit, hasUnit := strings.Cut(value, ":")
	if hasUnit {
		switch {
		case strings.EqualFold(unit, "packets"):
			return parseRateLimitExtCommunityWithSubtype(rate, attribute.FlowSpecRatePackets)
		case strings.EqualFold(unit, "bytes"):
			return parseRateLimitExtCommunityWithSubtype(rate, attribute.FlowSpecRateBytes)
		default:
			return attribute.ExtendedCommunity{}, fmt.Errorf("unknown rate-limit unit: %s", unit)
		}
	}
	return parseRateLimitExtCommunityWithSubtype(rate, attribute.FlowSpecRateBytes)
}

// parseRateLimitExtCommunityWithSubtype builds one traffic-rate community. The
// 2-octet id stays zero: the list form names no AS, and RFC 8955 Section 7.1
// calls the field "purely informational".
func parseRateLimitExtCommunityWithSubtype(value string, unit attribute.FlowSpecRateUnit) (attribute.ExtendedCommunity, error) {
	rate, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid rate in rate-limit: %s", value)
	}

	ec, err := attribute.FlowSpecTrafficRate(unit, 0, rate)
	if err != nil {
		return attribute.ExtendedCommunity{}, fmt.Errorf("rate-limit: %w", err)
	}
	return ec, nil
}
