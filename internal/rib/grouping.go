package rib

import (
	"bytes"
	"sort"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
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

// AttributeGroup represents routes sharing the same non-AS_PATH attributes.
// This is level 1 of the two-level grouping hierarchy.
//
// Routes in the same AttributeGroup share the same Attributes slice (memory efficient),
// but may have different AS_PATHs and are thus split into separate ASPathGroups.
type AttributeGroup struct {
	Key        string                // Family + NextHop + Attributes hash (excludes AS_PATH)
	Family     nlri.Family           // Address family
	NextHop    []byte                // Next-hop address bytes
	Attributes []attribute.Attribute // Shared path attributes (memory efficient reference)
	ByASPath   []ASPathGroup         // Level 2 sub-groups by AS_PATH
}

// ASPathGroup represents routes sharing the same AS_PATH within an AttributeGroup.
// Each ASPathGroup produces exactly one UPDATE message.
//
// RFC 4271 Section 4.3: Path attributes apply to ALL NLRIs in an UPDATE.
// Routes with different AS_PATHs cannot share an UPDATE.
type ASPathGroup struct {
	Key    string            // AS_PATH hash for grouping
	ASPath *attribute.ASPath // nil = no AS_PATH (locally originated)
	Routes []*Route          // Routes/NLRIs with this AS_PATH
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
	// IMPORTANT: Exclude AS_PATH from key - it's handled at level 2 of grouping
	attrs := route.Attributes()
	if len(attrs) > 0 {
		// Sort by attribute code
		sorted := make([]attribute.Attribute, len(attrs))
		copy(sorted, attrs)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Code() < sorted[j].Code()
		})

		// Pack each attribute (excluding AS_PATH)
		for _, attr := range sorted {
			if attr.Code() == attribute.AttrASPath {
				continue // AS_PATH is handled at level 2
			}
			buf.WriteByte(byte(attr.Code()))
			attrBuf := make([]byte, attr.Len())
			attr.WriteTo(attrBuf, 0)
			buf.Write(attrBuf)
		}
	}

	return buf.String()
}

// hashASPathString returns a string key for grouping routes by AS_PATH.
// Returns empty string for nil or empty AS_PATH (locally originated routes).
// Routes with the same AS_PATH hash can share an UPDATE message.
func hashASPathString(asPath *attribute.ASPath) string {
	if asPath == nil || len(asPath.Segments) == 0 {
		return ""
	}

	var buf bytes.Buffer
	for _, seg := range asPath.Segments {
		buf.WriteByte(byte(seg.Type))
		for _, asn := range seg.ASNs {
			buf.WriteByte(byte(asn >> 24))
			buf.WriteByte(byte(asn >> 16))
			buf.WriteByte(byte(asn >> 8))
			buf.WriteByte(byte(asn))
		}
	}
	return buf.String()
}

// GroupByAttributesTwoLevel groups routes first by attributes, then by AS_PATH.
// Returns attribute groups, each containing AS_PATH sub-groups.
// Each ASPathGroup can be sent as a single UPDATE message.
//
// This enables memory-efficient attribute sharing while ensuring RFC 4271 compliance:
// routes with different AS_PATHs produce separate UPDATE messages.
func GroupByAttributesTwoLevel(routes []*Route) []AttributeGroup {
	if len(routes) == 0 {
		return nil
	}

	// First pass: Group by attributes (excluding AS_PATH) using existing buildGroupKey
	attrGroups := make(map[string]*AttributeGroup)

	for _, route := range routes {
		key := buildGroupKey(route)

		if g, ok := attrGroups[key]; ok {
			// Add to existing AttributeGroup
			addRouteToASPathGroup(g, route)
		} else {
			// Create new AttributeGroup
			attrGroups[key] = &AttributeGroup{
				Key:        key,
				Family:     route.NLRI().Family(),
				NextHop:    route.NextHop().AsSlice(),
				Attributes: route.Attributes(),
				ByASPath:   nil,
			}
			addRouteToASPathGroup(attrGroups[key], route)
		}
	}

	// Convert to slice and sort for deterministic ordering
	result := make([]AttributeGroup, 0, len(attrGroups))
	for _, g := range attrGroups {
		// Sort ASPathGroups within each AttributeGroup
		sort.Slice(g.ByASPath, func(i, j int) bool {
			return g.ByASPath[i].Key < g.ByASPath[j].Key
		})
		result = append(result, *g)
	}

	// Sort AttributeGroups by key
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

// addRouteToASPathGroup adds a route to the appropriate ASPathGroup within an AttributeGroup.
func addRouteToASPathGroup(attrGroup *AttributeGroup, route *Route) {
	// Get AS_PATH: prefer route.ASPath(), fall back to searching in attributes
	asPath := getRouteASPath(route)
	asPathKey := hashASPathString(asPath)

	// Find existing ASPathGroup
	for i := range attrGroup.ByASPath {
		if attrGroup.ByASPath[i].Key == asPathKey {
			attrGroup.ByASPath[i].Routes = append(attrGroup.ByASPath[i].Routes, route)
			return
		}
	}

	// Create new ASPathGroup
	attrGroup.ByASPath = append(attrGroup.ByASPath, ASPathGroup{
		Key:    asPathKey,
		ASPath: asPath,
		Routes: []*Route{route},
	})
}

// getRouteASPath returns the AS_PATH for a route.
// Prefers route.ASPath() if set, otherwise searches in route.Attributes().
func getRouteASPath(route *Route) *attribute.ASPath {
	// Prefer explicit AS_PATH
	if route.ASPath() != nil {
		return route.ASPath()
	}

	// Fall back to AS_PATH in attributes (backward compatibility)
	for _, attr := range route.Attributes() {
		if attr.Code() == attribute.AttrASPath {
			if asp, ok := attr.(*attribute.ASPath); ok {
				return asp
			}
		}
	}
	return nil
}
