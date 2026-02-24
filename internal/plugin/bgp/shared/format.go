// Design: docs/architecture/plugin/rib-storage-design.md — shared route formatting
// Related: route.go — Route struct formatted by this file
// Related: event.go — event parsing and family operations
// Related: nlri.go — NLRI value parsing
package shared

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
)

// FormatRouteCommand builds the update text command with full attributes.
// Format: update text [attrs...] nhop set <nh> nlri <family> add <prefix>.
// The peer selector is passed separately to updateRoute.
func FormatRouteCommand(route *Route) string {
	var sb strings.Builder

	// Base command (peer selector is handled by updateRoute).
	sb.WriteString("update text")

	// Path-ID (RFC 7911) - must come before nlri.
	if route.PathID != 0 {
		fmt.Fprintf(&sb, " path-information set %d", route.PathID)
	}

	// Origin.
	if route.Origin != "" {
		sb.WriteString(" origin set ")
		sb.WriteString(route.Origin)
	}

	// AS-Path (use [] for list).
	if len(route.ASPath) > 0 {
		sb.WriteString(" as-path set ")
		sb.WriteString(attribute.FormatASPath(route.ASPath))
	}

	// MED.
	if route.MED != nil {
		fmt.Fprintf(&sb, " med set %d", *route.MED)
	}

	// Local-Preference.
	if route.LocalPreference != nil {
		fmt.Fprintf(&sb, " local-preference set %d", *route.LocalPreference)
	}

	// Communities (use [] for list).
	if len(route.Communities) > 0 {
		sb.WriteString(" community set [")
		sb.WriteString(strings.Join(route.Communities, " "))
		sb.WriteString("]")
	}

	// Large Communities (use [] for list).
	if len(route.LargeCommunities) > 0 {
		sb.WriteString(" large-community set [")
		sb.WriteString(strings.Join(route.LargeCommunities, " "))
		sb.WriteString("]")
	}

	// Extended Communities (use [] for list).
	if len(route.ExtendedCommunities) > 0 {
		sb.WriteString(" extended-community set [")
		sb.WriteString(strings.Join(route.ExtendedCommunities, " "))
		sb.WriteString("]")
	}

	// Next-hop (required).
	sb.WriteString(" nhop set ")
	sb.WriteString(route.NextHop)

	// NLRI with family.
	sb.WriteString(" nlri ")
	sb.WriteString(route.Family)
	sb.WriteString(" add ")
	sb.WriteString(route.Prefix)

	return sb.String()
}
