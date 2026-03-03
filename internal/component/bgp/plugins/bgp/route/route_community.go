// Design: docs/architecture/route-types.md — community route parsing
// Overview: route.go — core route types and attribute parsing

//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package route

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/types"
)

// parseCommunities parses communities in format [ASN:VAL ASN:VAL ...].
// Returns the parsed communities and how many tokens were consumed.
func parseCommunities(args []string) ([]uint32, int, error) {
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("missing community value")
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
		return nil, 0, fmt.Errorf("missing large-community value")
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
func ParseExtendedCommunities(args []string) ([]attribute.ExtendedCommunity, int, error) {
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
	before, after, ok := strings.Cut(s, ":")
	if !ok {
		return attribute.ExtendedCommunity{}, fmt.Errorf("invalid extended community format: %s", s)
	}

	typePrefix := strings.ToLower(before)
	value := after

	switch typePrefix {
	case "origin":
		return parseOriginExtCommunity(value)
	case "redirect":
		return parseRedirectExtCommunity(value)
	case "rate-limit":
		return parseRateLimitExtCommunity(value)
	default: // reject unknown extended community type
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
