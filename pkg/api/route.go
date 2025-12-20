package api

import (
	"errors"
	"fmt"
	"net/netip"
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
)

// RegisterRouteHandlers registers route-related command handlers.
func RegisterRouteHandlers(d *Dispatcher) {
	// Announce commands
	d.Register("announce route", handleAnnounceRoute, "Announce a route to peers")
	d.Register("announce eor", handleAnnounceEOR, "Send End-of-RIB marker")

	// Withdraw commands
	d.Register("withdraw route", handleWithdrawRoute, "Withdraw a route from peers")
}

// handleAnnounceRoute handles: announce route <prefix> next-hop <addr> [attributes...]
// Example: announce route 10.0.0.0/24 next-hop 192.168.1.1
// Example: announce route 10.0.0.0/24 next-hop self
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

	// Parse next-hop (after "next-hop" keyword)
	var nextHop netip.Addr
	nextHopSelf := false

	for i := 1; i < len(args)-1; i++ {
		if strings.EqualFold(args[i], "next-hop") {
			nhStr := args[i+1]
			if strings.EqualFold(nhStr, "self") {
				nextHopSelf = true
			} else {
				nextHop, err = netip.ParseAddr(nhStr)
				if err != nil {
					return &Response{
						Status: "error",
						Error:  fmt.Sprintf("invalid next-hop: %s", nhStr),
					}, ErrInvalidNextHop
				}
			}
			break
		}
	}

	if !nextHopSelf && !nextHop.IsValid() {
		return &Response{
			Status: "error",
			Error:  "missing next-hop",
		}, ErrMissingNextHop
	}

	// TODO: Parse additional attributes (origin, as-path, communities, etc.)
	// For now, we support basic routes with prefix + next-hop

	route := RouteSpec{
		Prefix:  prefix,
		NextHop: nextHop,
	}

	// Announce to all peers (selector "*")
	// TODO: Support peer-specific announcements
	if err := ctx.Reactor.AnnounceRoute("*", route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to announce: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"prefix":   prefix.String(),
			"next_hop": nextHop.String(),
		},
	}, nil
}

// handleWithdrawRoute handles: withdraw route <prefix>
// Example: withdraw route 10.0.0.0/24
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

	// Withdraw from all peers
	if err := ctx.Reactor.WithdrawRoute("*", prefix); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to withdraw: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"prefix": prefix.String(),
		},
	}, nil
}

// handleAnnounceEOR handles: announce eor [family]
// Example: announce eor (sends IPv4 unicast EOR)
// Example: announce eor ipv4 unicast
// Example: announce eor ipv6 unicast
func handleAnnounceEOR(ctx *CommandContext, args []string) (*Response, error) {
	// Default to IPv4 unicast
	afi := uint16(1) // IPv4
	safi := uint8(1) // Unicast
	family := "ipv4 unicast"

	// Parse optional family
	if len(args) >= 2 {
		afiStr := strings.ToLower(args[0])
		safiStr := strings.ToLower(args[1])

		switch afiStr {
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

		switch safiStr {
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

	// TODO: Send EOR to reactor when RIB integration is complete
	// For now, return success with the family info
	_ = afi
	_ = safi

	return &Response{
		Status: "done",
		Data: map[string]any{
			"family": family,
			"note":   "EOR queued for transmission",
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

		switch key {
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
