package bgp_nlri_mup

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// EncodeRoute encodes a MUP route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)

	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing MUP command")
	}

	// Parse using API parser
	parsed, err := route.ParseMUPArgs(args, isIPv6)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Build MUP NLRI
	nlriBytes, routeType, err := buildMUPNLRI(parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("build NLRI: %w", err)
	}

	// Parse next-hop
	var nextHop netip.Addr
	if parsed.NextHop != "" {
		nextHop, err = netip.ParseAddr(parsed.NextHop)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid next-hop: %w", err)
		}
	}

	// Convert to MUPParams
	params := message.MUPParams{
		RouteType: routeType,
		IsIPv6:    isIPv6,
		NLRI:      nlriBytes,
		NextHop:   nextHop,
	}

	// Build UPDATE
	update := ub.BuildMUP(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// buildMUPNLRI builds MUP NLRI bytes from MUPRouteSpec.
// Returns (nlri bytes, route type code, error).
func buildMUPNLRI(spec bgptypes.MUPRouteSpec) ([]byte, uint8, error) {
	// Determine route type code
	var routeType MUPRouteType
	switch spec.RouteType {
	case route.MUPRouteTypeISD:
		routeType = MUPISD
	case route.MUPRouteTypeDSD:
		routeType = MUPDSD
	case route.MUPRouteTypeT1ST:
		routeType = MUPT1ST
	case route.MUPRouteTypeT2ST:
		routeType = MUPT2ST
	default:
		return nil, 0, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse RD
	var rd RouteDistinguisher
	if spec.RD != "" {
		parsed, err := ParseRDString(spec.RD)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid RD %q: %w", spec.RD, err)
		}
		rd = parsed
	}

	// Build route-type-specific data
	var data []byte
	switch routeType {
	case MUPISD:
		if spec.Prefix == "" {
			return nil, 0, fmt.Errorf("MUP ISD requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
		}
		data = buildMUPPrefixBytes(prefix)

	case MUPDSD:
		if spec.Address == "" {
			return nil, 0, fmt.Errorf("MUP DSD requires address")
		}
		addr, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
		}
		data = addr.AsSlice()

	case MUPT1ST:
		if spec.Prefix == "" {
			return nil, 0, fmt.Errorf("MUP T1ST requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
		}
		data = buildMUPPrefixBytes(prefix)

	case MUPT2ST:
		if spec.Address == "" {
			return nil, 0, fmt.Errorf("MUP T2ST requires address")
		}
		ep, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
		}
		epBytes := ep.AsSlice()
		data = append(data, byte(len(epBytes)*8))
		data = append(data, epBytes...)
	}

	// Determine AFI
	afi := AFIIPv4
	if spec.IsIPv6 {
		afi = AFIIPv6
	}

	m := NewMUPFull(afi, MUPArch3GPP5G, routeType, rd, data)
	nlriBytes := m.Bytes()

	return nlriBytes, uint8(routeType), nil //nolint:gosec // MUP route type is always 0-4
}

// buildMUPPrefixBytes encodes a prefix for MUP NLRI.
func buildMUPPrefixBytes(prefix netip.Prefix) []byte {
	bits := prefix.Bits()
	addr := prefix.Addr()
	addrBytes := addr.AsSlice()
	prefixBytes := (bits + 7) / 8
	result := make([]byte, 1+prefixBytes)
	result[0] = byte(bits)
	copy(result[1:], addrBytes[:prefixBytes])
	return result
}
