// Design: docs/architecture/pool-architecture.md — RIB wire storage

package rib

import (
	"fmt"
	"log/slog"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// BuildGroupedUpdate creates an UPDATE message from a RouteGroup.
//
// The UPDATE includes:
//   - Path attributes from the group (including NEXT_HOP)
//   - All NLRIs from the routes in the group
//
// RFC 4271 Section 4.3: An UPDATE message can advertise multiple routes
// that share the same path attributes to a peer.
// RFC 7911: addPath indicates if ADD-PATH is negotiated for NLRI encoding.
func BuildGroupedUpdate(group *RouteGroup, addPath bool) (*message.Update, error) {
	if len(group.Routes) == 0 {
		return &message.Update{}, nil
	}

	// Build path attributes
	pathAttrs := buildPathAttributes(group)

	// Build NLRI bytes
	// RFC 7911: WriteTo uses ADD-PATH encoding when negotiated
	nlriBytes, err := buildNLRIBytes(group, addPath)
	if err != nil {
		return nil, err
	}

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

	// Write attributes in order (RFC 4271 Appendix F.3 recommends ordering by code)
	attrBytes := make([]byte, attribute.AttributesSize(attrs))
	attribute.WriteAttributesOrdered(attrs, attrBytes, 0)
	return attrBytes
}

// buildNLRIBytes packs all NLRIs from the group into wire format.
// RFC 7911: Uses WriteNLRI for centralized ADD-PATH handling.
// Zero-allocation: calculates size then writes with copy.
func buildNLRIBytes(group *RouteGroup, addPath bool) ([]byte, error) {
	if len(group.Routes) == 0 {
		return nil, nil
	}

	// Calculate total size
	totalLen := 0
	for _, route := range group.Routes {
		totalLen += nlri.LenWithContext(route.NLRI(), addPath)
	}

	// Write all NLRIs using WriteNLRI for centralized ADD-PATH handling
	buf := make([]byte, totalLen)
	off := 0
	for _, route := range group.Routes {
		off += nlri.WriteNLRI(route.NLRI(), buf, off, addPath)
	}

	// Invariant: LenWithContext must match WriteNLRI
	if off != totalLen {
		slog.Error("NLRI size mismatch: LenWithContext disagrees with WriteNLRI",
			"predicted", totalLen,
			"actual", off,
			"routes", len(group.Routes))
		return nil, fmt.Errorf("BUG: NLRI size mismatch: LenWithContext=%d WriteNLRI=%d", totalLen, off)
	}

	return buf, nil
}

// BuildGroupedUpdates creates UPDATE messages from multiple RouteGroups.
// Returns one UPDATE per group.
// RFC 7911: addPath indicates if ADD-PATH is negotiated for NLRI encoding.
func BuildGroupedUpdates(groups []RouteGroup, addPath bool) ([]*message.Update, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	updates := make([]*message.Update, 0, len(groups))
	for i := range groups {
		update, err := BuildGroupedUpdate(&groups[i], addPath)
		if err != nil {
			return nil, err
		}
		updates = append(updates, update)
	}

	return updates, nil
}
