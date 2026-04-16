// Design: docs/architecture/wire/nlri-evpn.md — EVPN NLRI plugin
// RFC: rfc/short/rfc7432.md

package evpn

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// EncodeRoute encodes an EVPN route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, _ string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	ub := message.GetUpdateBuilder(localAS, isIBGP, asn4, addPath)
	defer message.PutUpdateBuilder(ub)

	// Parse route command - expects "mac-ip|ip-prefix|... <args>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing EVPN route type")
	}

	// Parse using L2VPN argument parser
	parsed, err := parseL2VPNArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert L2VPNRoute to EVPNParams (builds NLRI bytes internally)
	params, err := l2vpnRouteToEVPNParams(parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("conversion error: %w", err)
	}

	// Build UPDATE
	update := ub.BuildEVPN(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// NLRI bytes are pre-built in params
	return updateBody, params.NLRI, nil
}

// l2vpnRouteToEVPNParams converts L2VPNRoute to EVPNParams.
// Builds EVPN NLRI bytes internally using NewEVPNType*().
//
//nolint:goconst // String literals are clearer for route type matching.
func l2vpnRouteToEVPNParams(r bgptypes.L2VPNRoute) (message.EVPNParams, error) {
	p := message.EVPNParams{
		NextHop: r.NextHop,
		Origin:  attribute.OriginIGP,
	}

	// Parse RD
	var rd RouteDistinguisher
	if r.RD != "" {
		var err error
		rd, err = ParseRDString(r.RD)
		if err != nil {
			return p, fmt.Errorf("invalid RD: %w", err)
		}
	}

	// Parse ESI
	esi, err := ParseESIString(r.ESI)
	if err != nil {
		return p, fmt.Errorf("invalid ESI: %w", err)
	}
	esiArr := [10]byte(esi)

	// Build EVPN NLRI based on route type
	var evpnNLRI EVPN
	switch r.RouteType {
	case "ethernet-ad":
		var labels []uint32
		if r.Label1 != 0 {
			labels = []uint32{r.Label1}
		}
		evpnNLRI = NewEVPNType1(rd, esiArr, r.EthernetTag, labels)
	case "mac-ip":
		mac, macErr := parseMAC(r.MAC)
		if macErr != nil {
			return p, fmt.Errorf("invalid MAC: %w", macErr)
		}
		var labels []uint32
		if r.Label1 != 0 {
			labels = []uint32{r.Label1}
			if r.Label2 != 0 {
				labels = append(labels, r.Label2)
			}
		}
		evpnNLRI = NewEVPNType2(rd, esiArr, r.EthernetTag, mac, r.IP, labels)
	case "multicast":
		evpnNLRI = NewEVPNType3(rd, r.EthernetTag, r.NextHop)
	case RouteNameEthernetSegment:
		evpnNLRI = NewEVPNType4(rd, esiArr, r.NextHop)
	case RouteNameIPPrefix:
		var labels []uint32
		if r.Label1 != 0 {
			labels = []uint32{r.Label1}
		}
		evpnNLRI = NewEVPNType5(rd, esiArr, r.EthernetTag, r.Prefix, r.Gateway, labels)
	default:
		return p, fmt.Errorf("unknown EVPN route type: %s", r.RouteType)
	}

	p.NLRI = evpnNLRI.Bytes()
	return p, nil
}

// parseMAC parses a MAC address string like "00:11:22:33:44:55".
func parseMAC(s string) ([6]byte, error) {
	var mac [6]byte
	if s == "" {
		return mac, nil
	}

	// Handle different separators
	s = strings.ReplaceAll(s, "-", ":")
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return mac, fmt.Errorf("invalid MAC format: %s", s)
	}

	for i, p := range parts {
		b, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return mac, fmt.Errorf("invalid MAC byte: %s", p)
		}
		mac[i] = byte(b)
	}

	return mac, nil
}

// parseL2VPNArgs parses L2VPN/EVPN command arguments for encode command.
//
//nolint:goconst // String literals are clearer for route type parsing
func parseL2VPNArgs(args []string) (bgptypes.L2VPNRoute, error) {
	var route bgptypes.L2VPNRoute

	if len(args) < 1 {
		return route, fmt.Errorf("missing route type")
	}

	// First argument is route type
	routeType := strings.ToLower(args[0])
	switch routeType {
	case "mac-ip", "macip", "type2":
		route.RouteType = "mac-ip"
	case RouteNameIPPrefix, "ipprefix", "type5":
		route.RouteType = RouteNameIPPrefix
	case "multicast", RouteNameInclusiveMulticast, "type3":
		route.RouteType = "multicast"
	case RouteNameEthernetSegment, "es", "type4":
		route.RouteType = RouteNameEthernetSegment
	case "ethernet-ad", "ead", "type1":
		route.RouteType = "ethernet-ad"
	default:
		return route, fmt.Errorf("invalid route type: %s", routeType)
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
				return route, fmt.Errorf("invalid prefix: %s", value)
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
				return route, fmt.Errorf("invalid next-hop: %s", value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown l2vpn keyword: %s", key)
		}
	}

	// Validate required fields based on route type
	if route.RD == "" {
		return route, fmt.Errorf("missing route-distinguisher")
	}

	if route.RouteType == "mac-ip" && route.MAC == "" {
		return route, fmt.Errorf("missing mac address")
	}

	if route.RouteType == RouteNameIPPrefix && !route.Prefix.IsValid() {
		return route, fmt.Errorf("missing prefix")
	}

	return route, nil
}
