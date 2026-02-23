// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md

package bgp_nlri_labeled

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// EncodeNLRIHex encodes labeled unicast NLRI from CLI-style args and returns uppercase hex.
// Args format: ["prefix", "10.0.0.0/24", "label", "100", "path-id", "1"]
// This implements the InProcessNLRIEncoder signature for the plugin registry.
func EncodeNLRIHex(family string, args []string) (string, error) {
	fam, ok := nlri.ParseFamily(family)
	if !ok {
		return "", fmt.Errorf("unknown family: %s", family)
	}
	if fam.SAFI != SAFIMPLSLabel {
		return "", fmt.Errorf("unsupported family for labeled unicast: %s", family)
	}

	var prefix netip.Prefix
	var labels []uint32
	var pathID uint32
	var hasPrefix bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "prefix":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("prefix requires value")
			}
			p, err := netip.ParsePrefix(args[i])
			if err != nil {
				return "", fmt.Errorf("invalid prefix: %w", err)
			}
			prefix = p
			hasPrefix = true
		case "label":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("label requires value")
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid label: %w", err)
			}
			labels = append(labels, uint32(v)) //nolint:gosec // validated by ParseUint with bitSize 32
		case "path-id":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("path-id requires value")
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid path-id: %w", err)
			}
			pathID = uint32(v) //nolint:gosec // validated by ParseUint with bitSize 32
		}
	}

	if !hasPrefix {
		return "", fmt.Errorf("prefix required for labeled unicast")
	}
	if len(labels) == 0 {
		return "", fmt.Errorf("label required for labeled unicast")
	}

	n := NewLabeledUnicast(fam, prefix, labels, pathID)
	nlriBytes := n.Bytes()

	return strings.ToUpper(hex.EncodeToString(nlriBytes)), nil
}

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
	update := ub.BuildLabeledUnicast(&params)

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
