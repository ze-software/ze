// Design: docs/architecture/route-types.md — L3VPN route parsing
// Overview: route.go — core route types and attribute parsing

package route

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/context"
)

var errMissingRtValue = errors.New("missing rt value")

// parseL3VPNAttributes parses L3VPN route attributes from args.
// Args format: <prefix> [keyword value]...
// Supports VPNKeywords: rd, rt, label, plus all unicast keywords.
func parseL3VPNAttributes(args []string) (bgptypes.L3VPNRoute, error) {
	if len(args) < 1 {
		return bgptypes.L3VPNRoute{}, ErrMissingPrefix
	}

	// Parse prefix (first arg)
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil {
		return bgptypes.L3VPNRoute{}, fmt.Errorf("%w: %s", ErrInvalidPrefix, args[0])
	}

	route := bgptypes.L3VPNRoute{
		Prefix: prefix,
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	// Parse remaining args as key-value pairs
	for i := 1; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Validate keyword against VPN keywords
		if !VPNKeywords[key] {
			return bgptypes.L3VPNRoute{}, fmt.Errorf("%w: '%s' not valid for L3VPN", ErrInvalidKeyword, key)
		}

		// Try common attribute parsing with Builder (wire-first)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return bgptypes.L3VPNRoute{}, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Handle VPN-specific keywords
		switch key {
		case "rd":
			if i+1 >= len(args) {
				return bgptypes.L3VPNRoute{}, ErrMissingRD
			}
			route.RD = args[i+1]
			i++

		case "rt":
			if i+1 >= len(args) {
				return bgptypes.L3VPNRoute{}, errMissingRtValue
			}
			route.RT = args[i+1]
			i++

		case "label":
			if i+1 >= len(args) {
				return bgptypes.L3VPNRoute{}, ErrMissingLabel
			}
			labels, consumed, err := parseLabels(args[i+1:])
			if err != nil {
				return bgptypes.L3VPNRoute{}, err
			}
			route.Labels = labels
			i += consumed

		case keywordNextHop:
			if i+1 >= len(args) {
				return bgptypes.L3VPNRoute{}, ErrMissingNextHop
			}
			nh, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return bgptypes.L3VPNRoute{}, fmt.Errorf("%w: %s", ErrInvalidNextHop, args[i+1])
			}
			route.NextHop = nh
			i++
		}
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		route.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return route, nil
}

// ParseL3VPNAttributes parses L3VPN (mpls-vpn) command arguments.
// Exported for use by encode command.
// Format: <prefix> rd <rd> next-hop <addr> label <label> [attributes...].
func ParseL3VPNAttributes(args []string) (bgptypes.L3VPNRoute, error) {
	return parseL3VPNAttributes(args)
}
