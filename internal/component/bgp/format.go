// Design: docs/architecture/plugin/rib-storage-design.md — route command formatting
// Related: route.go — Route struct formatted by this file
// Related: event.go — event parsing and family operations
// Related: nlri.go — NLRI value parsing
package bgp

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FormatAnnounceCommand builds an announce command with full attributes.
// When route.RawAttrs is set (from format=full sent events), uses "update hex"
// format to preserve ALL transitive attributes (OTC, unknown attrs) through replay.
// Otherwise uses "update text" with per-field attributes.
// The peer selector is passed separately to updateRoute.
func FormatAnnounceCommand(route *Route) string {
	// Text format is used for replay: prefix is stored as text ("192.168.1.0/24"),
	// not hex wire bytes. The hex command requires hex NLRIs which we don't have.
	return formatAnnounceText(route)
}

// formatAnnounceText builds an "update text" command with per-field attributes.
// Used when raw attributes are not available (e.g., plugin-originated routes).
func formatAnnounceText(route *Route) string {
	var sb textbuf.Buffer

	// Base command (peer selector is handled by updateRoute).
	sb.Str("update text")

	// Origin.
	if route.Origin != nil {
		if s := route.Origin.LowerString(); s != "" {
			sb.Str(" origin ").Str(s)
		}
	}

	// AS-Path (use [] for list).
	if len(route.ASPath) > 0 {
		p := &attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: route.ASPath}}}
		var scratch [128]byte
		sb.Byte(' ')
		sb.Write(p.AppendText(scratch[:0]))
	}

	// MED.
	if route.MED != nil {
		sb.Str(" med ").Uint32(*route.MED)
	}

	// Local-Preference.
	if route.LocalPreference != nil {
		sb.Str(" local-preference ").Uint32(*route.LocalPreference)
	}

	// Communities (use [] for list).
	if len(route.Communities) > 0 {
		sb.Str(" community [")
		for i, c := range route.Communities {
			if i > 0 {
				sb.Byte(' ')
			}
			sb.Str(c.String())
		}
		sb.Byte(']')
	}

	// Large Communities (use [] for list).
	if len(route.LargeCommunities) > 0 {
		sb.Str(" large-community [")
		for i, lc := range route.LargeCommunities {
			if i > 0 {
				sb.Byte(' ')
			}
			sb.Str(lc.String())
		}
		sb.Byte(']')
	}

	// Extended Communities (use [] for list).
	if len(route.ExtendedCommunities) > 0 {
		sb.Str(" extended-community [")
		for i, ec := range route.ExtendedCommunities {
			if i > 0 {
				sb.Byte(' ')
			}
			sb.Str(hex.EncodeToString(ec[:]))
		}
		sb.Byte(']')
	}

	// Next-hop (required).
	sb.Str(" nhop ").Str(route.NextHop)

	// NLRI with family and optional modifiers (RFC 7911, RFC 4364).
	sb.Str(" nlri ").Str(route.Family.String())
	writeNLRIModifiers(&sb, route)
	sb.Str(" add ").Str(route.Prefix)

	return sb.String()
}

// FormatWithdrawCommand builds an "update text" withdrawal command.
// Withdrawals only need family, prefix, and NLRI modifiers (no attributes).
func FormatWithdrawCommand(route *Route) string {
	var sb textbuf.Buffer
	sb.Str("update text nlri ").Str(route.Family.String())
	writeNLRIModifiers(&sb, route)
	sb.Str(" del ").Str(route.Prefix)

	return sb.String()
}

// writeNLRIModifiers writes per-NLRI-section modifiers: rd, label stack, path-information.
func writeNLRIModifiers(sb *textbuf.Buffer, route *Route) {
	if route.RD != "" {
		sb.Str(" rd ").Str(route.RD)
	}
	for _, label := range route.Labels {
		sb.Str(" label ").Uint32(label)
	}
	if route.PathID != 0 {
		sb.Str(" path-information ").Uint32(route.PathID)
	}
}
