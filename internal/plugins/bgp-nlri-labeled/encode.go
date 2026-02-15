package bgp_nlri_labeled

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// EncodeRoute encodes a labeled unicast (nlri-mpls) route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)

	// Parse route command - expects "<prefix> next-hop <addr> label <label> [attributes...]"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing route command")
	}

	// Parse using API parser
	parsed, err := route.ParseLabeledUnicastAttributes(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert to LabeledUnicastParams
	params := labeledUnicastRouteToParams(parsed)

	// Build UPDATE
	update := ub.BuildLabeledUnicast(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	// Build NLRI for -n flag
	var fam Family
	if isIPv6 {
		fam = Family{AFI: AFIIPv6, SAFI: SAFIMPLSLabel}
	} else {
		fam = Family{AFI: AFIIPv4, SAFI: SAFIMPLSLabel}
	}
	var label uint32
	if len(parsed.Labels) > 0 {
		label = parsed.Labels[0]
	}
	labeledNLRI := NewLabeledUnicast(fam, parsed.Prefix, []uint32{label}, parsed.PathID)
	nlriBytes := labeledNLRI.Bytes()

	return updateBody, nlriBytes, nil
}

// labeledUnicastRouteToParams converts LabeledUnicastRoute to LabeledUnicastParams.
func labeledUnicastRouteToParams(r bgptypes.LabeledUnicastRoute) message.LabeledUnicastParams {
	attrs := message.ExtractAttrsFromWire(r.Wire)

	p := message.LabeledUnicastParams{
		Prefix:            r.Prefix,
		NextHop:           r.NextHop,
		PathID:            r.PathID,
		Origin:            attrs.Origin,
		LocalPreference:   attrs.LocalPreference,
		MED:               attrs.MED,
		ASPath:            attrs.ASPath,
		Communities:       attrs.Communities,
		LargeCommunities:  attrs.LargeCommunities,
		ExtCommunityBytes: attrs.ExtCommunityBytes,
	}

	// Labels (copy from route)
	p.Labels = r.Labels

	return p
}
