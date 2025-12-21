package rib

import (
	"net/netip"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// BuildGroupedUpdate creates an UPDATE message from a RouteGroup.
//
// The UPDATE includes:
//   - Path attributes from the group (including NEXT_HOP)
//   - All NLRIs from the routes in the group
//
// RFC 4271 Section 4.3: An UPDATE message can advertise multiple routes
// that share the same path attributes to a peer.
func BuildGroupedUpdate(group *RouteGroup) (*message.Update, error) {
	if len(group.Routes) == 0 {
		return &message.Update{}, nil
	}

	// Build path attributes
	pathAttrs := buildPathAttributes(group)

	// Build NLRI bytes
	nlriBytes := buildNLRIBytes(group)

	return &message.Update{
		PathAttributes: pathAttrs,
		NLRI:           nlriBytes,
	}, nil
}

// buildPathAttributes packs all path attributes for the group.
// Adds NEXT_HOP if not already present in attributes.
func buildPathAttributes(group *RouteGroup) []byte {
	// Collect all attributes, ensuring NEXT_HOP is included
	attrs := make([]attribute.Attribute, 0, len(group.Attributes)+1)

	hasNextHop := false
	for _, attr := range group.Attributes {
		if attr.Code() == attribute.AttrNextHop {
			hasNextHop = true
		}
		attrs = append(attrs, attr)
	}

	// Add NEXT_HOP if not present
	if !hasNextHop && len(group.NextHop) > 0 {
		nh, ok := netip.AddrFromSlice(group.NextHop)
		if ok {
			attrs = append(attrs, &attribute.NextHop{Addr: nh})
		}
	}

	// Pack attributes in order (RFC 4271 Appendix F.3 recommends ordering by code)
	return attribute.PackAttributesOrdered(attrs)
}

// buildNLRIBytes packs all NLRIs from the group into wire format.
func buildNLRIBytes(group *RouteGroup) []byte {
	if len(group.Routes) == 0 {
		return nil
	}

	// Calculate total size
	totalLen := 0
	for _, route := range group.Routes {
		totalLen += len(route.NLRI().Bytes())
	}

	// Pack all NLRIs
	buf := make([]byte, 0, totalLen)
	for _, route := range group.Routes {
		buf = append(buf, route.NLRI().Bytes()...)
	}

	return buf
}

// BuildGroupedUpdates creates UPDATE messages from multiple RouteGroups.
// Returns one UPDATE per group.
func BuildGroupedUpdates(groups []RouteGroup) ([]*message.Update, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	updates := make([]*message.Update, 0, len(groups))
	for i := range groups {
		update, err := BuildGroupedUpdate(&groups[i])
		if err != nil {
			return nil, err
		}
		updates = append(updates, update)
	}

	return updates, nil
}
