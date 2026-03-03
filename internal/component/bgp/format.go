// Design: docs/architecture/plugin/rib-storage-design.md — route command formatting
// Related: route.go — Route struct formatted by this file
// Related: event.go — event parsing and family operations
// Related: nlri.go — NLRI value parsing
package bgp

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
)

// FormatAnnounceCommand builds an "update text" announce command with full attributes.
// Format: update text [attrs...] nhop <nh> nlri <family> [modifiers] add <prefix>.
// The peer selector is passed separately to updateRoute.
func FormatAnnounceCommand(route *Route) string {
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

	// NLRI with family and optional modifiers (RFC 7911, RFC 4364).
	sb.WriteString(" nlri ")
	sb.WriteString(route.Family)
	writeNLRIModifiers(&sb, route)
	sb.WriteString(" add ")
	sb.WriteString(route.Prefix)

	return sb.String()
}

// FormatWithdrawCommand builds an "update text" withdrawal command.
// Withdrawals only need family, prefix, and NLRI modifiers (no attributes).
func FormatWithdrawCommand(route *Route) string {
	var sb strings.Builder
	sb.WriteString("update text nlri ")
	sb.WriteString(route.Family)
	writeNLRIModifiers(&sb, route)
	sb.WriteString(" del ")
	sb.WriteString(route.Prefix)

	return sb.String()
}

// writeNLRIModifiers writes per-NLRI-section modifiers: rd, label stack, path-information.
func writeNLRIModifiers(sb *strings.Builder, route *Route) {
	if route.RD != "" {
		sb.WriteString(" rd ")
		sb.WriteString(route.RD)
	}
	for _, label := range route.Labels {
		fmt.Fprintf(sb, " label %d", label)
	}
	if route.PathID != 0 {
		fmt.Fprintf(sb, " path-information %d", route.PathID)
	}
}
