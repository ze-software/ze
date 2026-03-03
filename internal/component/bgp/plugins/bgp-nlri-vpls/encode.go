// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin
// RFC: rfc/short/rfc4761.md

package bgp_nlri_vpls

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/types"
)

// EncodeRoute encodes a VPLS route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, _ string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)

	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing VPLS command")
	}

	// Parse using VPLS argument parser
	parsed, err := parseVPLSArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse RD
	rd, err := ParseRDString(parsed.RD)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid RD: %w", err)
	}

	// Convert to VPLSParams
	params := vplsRouteToParams(parsed, rd)

	// Build UPDATE
	update := ub.BuildVPLS(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// For -n flag, build VPLS NLRI
	vplsNLRI := NewVPLSFull(rd, parsed.VEBlockOffset, parsed.VEBlockOffset, parsed.VEBlockSize, parsed.LabelBase)
	nlriBytes := vplsNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// vplsRouteToParams converts VPLSRoute to VPLSParams.
func vplsRouteToParams(r bgptypes.VPLSRoute, rd RouteDistinguisher) message.VPLSParams {
	p := message.VPLSParams{
		NextHop:  r.NextHop,
		Offset:   r.VEBlockOffset,
		Size:     r.VEBlockSize,
		Base:     r.LabelBase,
		Endpoint: r.VEBlockOffset, // VE ID typically matches offset
		Origin:   attribute.OriginIGP,
	}

	// Copy RD bytes
	rdBytes := rd.Bytes()
	copy(p.RD[:], rdBytes)

	return p
}

// parseVPLSArgs parses VPLS command arguments for encode command.
// Format: rd <rd> ve-block-offset <n> ve-block-size <n> label <n> next-hop <addr>.
func parseVPLSArgs(args []string) (bgptypes.VPLSRoute, error) {
	var route bgptypes.VPLSRoute

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
				return route, fmt.Errorf("invalid next-hop: %s", value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown vpls keyword: %s", key)
		}
	}

	if route.RD == "" {
		return route, fmt.Errorf("missing route-distinguisher")
	}

	return route, nil
}
