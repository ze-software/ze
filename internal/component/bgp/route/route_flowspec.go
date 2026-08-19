// Design: docs/architecture/route-types.md — FlowSpec route parsing
// Overview: route.go — core route types and attribute parsing

//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package route

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

var (
	errMissingRateLimitValue = errors.New("missing rate limit value")
	errMissingRedirectTarget = errors.New("missing redirect target")
	errMissingDscpValue      = errors.New("missing DSCP value")
)

// ParseFlowSpecArgs parses FlowSpec command arguments.
// Format: match <spec> then <action>.
// Example: match destination 10.0.0.0/24 destination-port 80 then discard.
func ParseFlowSpecArgs(args []string) (bgptypes.FlowSpecRoute, error) {
	var route bgptypes.FlowSpecRoute
	route.Family = family.AFIIPv4 // default

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
					route.Family = family.AFIIPv6
				}
				i++

			case "source":
				prefix, err := netip.ParsePrefix(value)
				if err != nil {
					return route, fmt.Errorf("%w: %s", ErrInvalidPrefix, value)
				}
				route.SourcePrefix = &prefix
				if prefix.Addr().Is6() {
					route.Family = family.AFIIPv6
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

			default: // reject unknown match keyword
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
					return route, errMissingRateLimitValue
				}
				rate, err := strconv.ParseUint(args[i+1], 10, 32)
				if err != nil {
					return route, fmt.Errorf("invalid rate limit: %s", args[i+1])
				}
				consumedUnit := false
				if i+2 < len(args) {
					switch {
					case strings.EqualFold(args[i+2], "packets"):
						route.Actions.RateLimitPackets = uint32(rate)
						route.Actions.RateLimitPacketsSet = true
						i += 2
						consumedUnit = true
					case strings.EqualFold(args[i+2], "bytes"):
						route.Actions.RateLimit = uint32(rate)
						route.Actions.RateLimitSet = true
						i += 2
						consumedUnit = true
					}
				}
				if !consumedUnit {
					route.Actions.RateLimit = uint32(rate)
					route.Actions.RateLimitSet = true
					i++
				}
			case "rate-limit-packets":
				if i+1 >= len(args) {
					return route, errMissingRateLimitValue
				}
				rate, err := strconv.ParseUint(args[i+1], 10, 32)
				if err != nil {
					return route, fmt.Errorf("invalid packet rate limit: %s", args[i+1])
				}
				route.Actions.RateLimitPackets = uint32(rate)
				route.Actions.RateLimitPacketsSet = true
				i++
			case "redirect":
				if i+1 >= len(args) {
					return route, errMissingRedirectTarget
				}
				route.Actions.Redirect = args[i+1]
				i++
			case "mark":
				if i+1 >= len(args) {
					return route, errMissingDscpValue
				}
				dscp, err := strconv.ParseUint(args[i+1], 10, 8)
				if err != nil {
					return route, fmt.Errorf("invalid DSCP: %s", args[i+1])
				}
				// RFC 8955 Section 7.5: the DSCP is the 6 least significant
				// bits of the extended community value, and the bits above it
				// are "reserved (r): MUST be set to 0 on encoding". Say so
				// here rather than truncating in silence, which would install
				// a marking the operator did not ask for. The encoder refuses
				// the same value again, from the same bound.
				if dscp > attribute.FlowSpecDSCPMax {
					return route, fmt.Errorf("DSCP must be 0-%d, got %d", attribute.FlowSpecDSCPMax, dscp)
				}
				route.Actions.MarkDSCP = uint8(dscp)
				route.Actions.MarkDSCPSet = true
				i++

			default: // reject unknown then keyword
				return route, fmt.Errorf("unknown then keyword: %s", arg)
			}

		default: // reject keywords outside match/then blocks
			// Provide helpful error: is it a misplaced match/then keyword or unknown?
			switch arg {
			case "destination", "source", "protocol", "port", "destination-port", "source-port":
				return route, fmt.Errorf("match keyword %q must appear after 'match'", arg)
			case "accept", "discard", "rate-limit", "rate-limit-packets", "redirect", "mark":
				return route, fmt.Errorf("then keyword %q must appear after 'then'", arg)
			default: // reject completely unknown keyword
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
	default: // reject unknown protocol name, try numeric
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
