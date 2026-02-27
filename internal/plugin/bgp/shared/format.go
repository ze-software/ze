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
// Format: update text [attrs...] nhop <nh> nlri <family> [path-information <id>] add <prefix>.
// The peer selector is passed separately to updateRoute.
func FormatRouteCommand(route *Route) string {
	var sb strings.Builder

	// Base command (peer selector is handled by updateRoute).
	sb.WriteString("update text")

	// Origin.
	if route.Origin != "" {
		sb.WriteString(" origin ")
		sb.WriteString(route.Origin)
	}

	// AS-Path (use [] for list).
	if len(route.ASPath) > 0 {
		sb.WriteString(" as-path ")
		sb.WriteString(attribute.FormatASPath(route.ASPath))
	}

	// MED.
	if route.MED != nil {
		fmt.Fprintf(&sb, " med %d", *route.MED)
	}

	// Local-Preference.
	if route.LocalPreference != nil {
		fmt.Fprintf(&sb, " local-preference %d", *route.LocalPreference)
	}

	// Communities (use [] for list).
	if len(route.Communities) > 0 {
		sb.WriteString(" community [")
		sb.WriteString(strings.Join(route.Communities, " "))
		sb.WriteString("]")
	}

	// Large Communities (use [] for list).
	if len(route.LargeCommunities) > 0 {
		sb.WriteString(" large-community [")
		sb.WriteString(strings.Join(route.LargeCommunities, " "))
		sb.WriteString("]")
	}

	// Extended Communities (use [] for list).
	if len(route.ExtendedCommunities) > 0 {
		sb.WriteString(" extended-community [")
		sb.WriteString(strings.Join(route.ExtendedCommunities, " "))
		sb.WriteString("]")
	}

	// Next-hop (required).
	sb.WriteString(" nhop ")
	sb.WriteString(route.NextHop)

	// NLRI with family and optional path-id (RFC 7911).
	sb.WriteString(" nlri ")
	sb.WriteString(route.Family)
	if route.PathID != 0 {
		fmt.Fprintf(&sb, " path-information %d", route.PathID)
	}
	sb.WriteString(" add ")
	sb.WriteString(route.Prefix)

	return sb.String()
}
