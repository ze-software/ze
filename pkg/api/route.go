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
)

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

// handleAnnounceRoute handles: announce route <prefix> next-hop <addr> [attributes...].
// Example: announce route 10.0.0.0/24 next-hop 192.168.1.1.
// Example: announce route 10.0.0.0/24 next-hop self.
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

	// Announce to matching peers (default "*" for all)
	peerSelector := ctx.NeighborSelector()
	if err := ctx.Reactor.AnnounceRoute(peerSelector, route); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to announce: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"neighbor": peerSelector,
			"prefix":   prefix.String(),
			"next_hop": nextHop.String(),
		},
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
	peerSelector := ctx.NeighborSelector()
	if err := ctx.Reactor.WithdrawRoute(peerSelector, prefix); err != nil {
		return &Response{
			Status: "error",
			Error:  fmt.Sprintf("failed to withdraw: %v", err),
		}, err
	}

	return &Response{
		Status: "done",
		Data: map[string]any{
			"neighbor": peerSelector,
			"prefix":   prefix.String(),
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
