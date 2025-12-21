package rib

import (
	"bytes"
	"sort"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// RouteGroup represents routes that share identical attributes.
// Routes in the same group can be sent in a single UPDATE message.
type RouteGroup struct {
	Key        string                // Unique key for this attribute set
	Family     nlri.Family           // Address family
	NextHop    []byte                // Next-hop address bytes
	Attributes []attribute.Attribute // Shared path attributes
	Routes     []*Route              // Routes in this group
}

// NLRIs returns all NLRIs in this group.
func (g *RouteGroup) NLRIs() []nlri.NLRI {
	nlris := make([]nlri.NLRI, len(g.Routes))
	for i, r := range g.Routes {
		nlris[i] = r.NLRI()
	}
	return nlris
}

// GroupByAttributes groups routes by their attribute set.
// Routes with identical attributes (including next-hop) can share a single UPDATE.
//
// The grouping key is: Family + NextHop + sorted attribute bytes.
// This ensures routes with the same attributes are grouped together.
func GroupByAttributes(routes []*Route) []RouteGroup {
	if len(routes) == 0 {
		return nil
	}

	// Map of group key -> group
	groups := make(map[string]*RouteGroup)

	for _, route := range routes {
		key := buildGroupKey(route)

		if g, ok := groups[key]; ok {
			// Add to existing group
			g.Routes = append(g.Routes, route)
		} else {
			// Create new group
			groups[key] = &RouteGroup{
				Key:        key,
				Family:     route.NLRI().Family(),
				NextHop:    route.NextHop().AsSlice(),
				Attributes: route.Attributes(),
				Routes:     []*Route{route},
			}
		}
	}

	// Convert to slice and sort for deterministic ordering
	result := make([]RouteGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}

	// Sort by key for deterministic ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

// buildGroupKey creates a unique key for grouping routes.
// Key includes: Family + NextHop + sorted attributes.
func buildGroupKey(route *Route) string {
	var buf bytes.Buffer

	// Family
	family := route.NLRI().Family()
	buf.WriteByte(byte(family.AFI >> 8))
	buf.WriteByte(byte(family.AFI))
	buf.WriteByte(byte(family.SAFI))

	// Next-hop
	if nh := route.NextHop(); nh.IsValid() {
		buf.Write(nh.AsSlice())
	}

	// Attributes (sorted by code for consistency)
	attrs := route.Attributes()
	if len(attrs) > 0 {
		// Sort by attribute code
		sorted := make([]attribute.Attribute, len(attrs))
		copy(sorted, attrs)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Code() < sorted[j].Code()
		})

		// Pack each attribute
		for _, attr := range sorted {
			buf.WriteByte(byte(attr.Code()))
			buf.Write(attr.Pack())
		}
	}

	return buf.String()
}
