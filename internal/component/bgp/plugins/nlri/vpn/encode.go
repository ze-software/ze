// Design: docs/architecture/wire/nlri.md — VPN NLRI plugin
// RFC: rfc/short/rfc4364.md

package vpn

import (
	"fmt"
	"strings"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"
)

// EncodeRoute encodes an L3VPN (mpls-vpn) route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)

	// Parse route command - expects "<prefix> rd <rd> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing route command")
	}

	// Parse using API parser
	parsed, err := route.ParseL3VPNAttributes(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse RD once for both NLRI and params
	var rd RouteDistinguisher
	if parsed.RD != "" {
		rd, err = ParseRDString(parsed.RD)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid RD: %w", err)
		}
	}

	// Convert L3VPNRoute to VPNParams (pass pre-parsed RD)
	params := l3vpnRouteToVPNParams(parsed, rd)

	// Build UPDATE
	update := ub.BuildVPN(&params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// Build NLRI for -n flag
	var fam Family
	if isIPv6 {
		fam = IPv6VPN
	} else {
		fam = IPv4VPN
	}
	var label uint32
	if len(parsed.Labels) > 0 {
		label = parsed.Labels[0]
	}
	vpnNLRI := NewVPN(fam, rd, []uint32{label}, parsed.Prefix, 0)
	nlriBytes := vpnNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// l3vpnRouteToVPNParams converts L3VPNRoute to VPNParams.
// Takes pre-parsed RD to avoid double parsing.
func l3vpnRouteToVPNParams(r bgptypes.L3VPNRoute, rd RouteDistinguisher) message.VPNParams {
	attrs := message.ExtractAttrsFromWire(r.Wire)

	p := message.VPNParams{
		Prefix:            r.Prefix,
		NextHop:           r.NextHop,
		Origin:            attrs.Origin,
		LocalPreference:   attrs.LocalPreference,
		MED:               attrs.MED,
		ASPath:            attrs.ASPath,
		Communities:       attrs.Communities,
		LargeCommunities:  attrs.LargeCommunities,
		ExtCommunityBytes: attrs.ExtCommunityBytes,
	}

	// Use pre-parsed RD
	rdBytes := rd.Bytes()
	copy(p.RDBytes[:], rdBytes)

	// Labels (copy from route)
	p.Labels = r.Labels

	return p
}
