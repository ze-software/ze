// Design: docs/architecture/plugin/rib-storage-design.md — route command formatting
// Related: route.go — Route struct formatted by this file
// Related: event.go — event parsing and family operations
// Related: nlri.go — NLRI value parsing
package bgp

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/rib/store"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FormatAnnounceCommand builds an announce command with full attributes.
// When route.RawAttrs is set (from format=full sent events), uses "update hex"
// format to preserve ALL transitive attributes (OTC, unknown attrs) through replay.
// Otherwise uses "update text" with per-field attributes.
// The peer selector is passed separately to updateRoute.
func FormatAnnounceCommand(route *Route) string {
	if route.RawAttrs != "" {
		var b textbuf.Buffer
		b.Str("update hex attr set ").Str(route.RawAttrs)
		if nextHop, err := netip.ParseAddr(route.NextHop); err == nil {
			b.Str(" nhop set ").Str(hex.EncodeToString(nextHop.AsSlice()))
		}
		b.Str(" nlri ").Str(route.Family.String())
		if route.AddPath || route.PathID != 0 {
			b.Str(" addpath")
		}
		b.Str(" add ").Str(RouteNLRIHex(route))
		return b.String()
	}
	return formatAnnounceText(route)
}

// RouteNLRIHex returns the native wire representation used by update hex.
// Opaque routes already own it; CIDR routes are encoded only on this cold path.
func RouteNLRIHex(route *Route) string {
	if route.RawNLRI != "" {
		return route.RawNLRI
	}
	prefix, err := netip.ParsePrefix(route.Prefix)
	if err != nil {
		return ""
	}
	var raw [21]byte
	offset := 0
	if route.AddPath || route.PathID != 0 {
		binary.BigEndian.PutUint32(raw[:4], route.PathID)
		offset = 4
	}
	nlri := store.PrefixToNLRIInto(prefix, raw[offset:])
	return hex.EncodeToString(raw[:offset+len(nlri)])
}

// formatAnnounceText builds an "update text" command with per-field attributes.
// Used when raw attributes are not available (e.g., plugin-originated routes).
//
// Every AS number here is asplain, and the bgp/as-notation leaf does NOT reach
// this function. The result is a COMMAND Ze replays to itself, not a line an
// operator reads. A replay that spelled its AS path in a display notation would
// depend on that notation still being configured when it runs.
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
	if route.RawNLRI != "" {
		var b textbuf.Buffer
		b.Str("update hex nlri ").Str(route.Family.String())
		if route.AddPath || route.PathID != 0 {
			b.Str(" addpath")
		}
		return b.Str(" del ").Str(route.RawNLRI).String()
	}
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
	if route.AddPath || route.PathID != 0 {
		sb.Str(" path-information ").Uint32(route.PathID)
	}
}
