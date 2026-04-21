// Design: docs/architecture/api/json-format.md — human-readable text rendering
// Overview: text.go — message format dispatch

package format

import (
	"encoding/hex"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// appendFilterResultText appends FilterResult as text to buf.
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
func appendFilterResultText(buf []byte, peer *plugin.PeerInfo, result bgpfilter.FilterResult, msgID uint64, direction rpc.MessageDirection, ctx *bgpctx.EncodingContext) []byte {
	// Uniform header: peer <ip> remote as <asn> <direction> update <id>
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, ' ')
	buf = direction.AppendTo(buf)
	buf = append(buf, " update "...)
	buf = strconv.AppendUint(buf, msgID, 10)

	announced := result.AnnouncedByFamily(ctx)
	withdrawn := result.WithdrawnByFamily(ctx)

	if len(announced) == 0 && len(withdrawn) == 0 {
		// Empty UPDATE (End-of-RIB marker or attribute-only): minimal text line
		buf = append(buf, '\n')
		return buf
	}

	// Attributes (shared across all families)
	buf = appendAttributesText(buf, result)

	// Announced routes: next <nh> nlri <fam> add <nlri>[,<nlri>]...
	for _, fam := range announced {
		buf = append(buf, ' ')
		buf = append(buf, textparse.ShortNext...)
		buf = append(buf, ' ')
		buf = fam.NextHop.AppendTo(buf)
		buf = append(buf, " nlri "...)
		buf = fam.Family.AppendTo(buf)
		buf = append(buf, " add "...)
		buf = appendNLRIList(buf, fam.NLRIs)
	}

	// Withdrawn routes: nlri <fam> del <nlri>[,<nlri>]...
	for _, fam := range withdrawn {
		buf = append(buf, " nlri "...)
		buf = fam.Family.AppendTo(buf)
		buf = append(buf, " del "...)
		buf = appendNLRIList(buf, fam.NLRIs)
	}

	buf = append(buf, '\n')
	return buf
}

// appendNLRIList appends NLRIs in compact format.
// INET NLRIs use comma-separated CIDRs: prefix <a>,<b>.
// Other NLRIs use keyword boundary: <nlri1> <nlri2>.
// Uses AppendString/AppendKey for INET NLRIs to avoid per-NLRI string allocations.
func appendNLRIList(buf []byte, nlris []nlri.NLRI) []byte {
	if len(nlris) == 0 {
		return buf
	}

	// Check if first NLRI is INET (supports zero-alloc append methods).
	firstINET, useComma := nlris[0].(*nlri.INET)

	if useComma {
		buf = firstINET.AppendString(buf)
	} else {
		buf = append(buf, nlris[0].String()...)
	}
	for _, n := range nlris[1:] {
		if useComma {
			buf = append(buf, ',')
			if inet, ok := n.(*nlri.INET); ok {
				buf = inet.AppendKey(buf)
			} else {
				buf = append(buf, n.String()...)
			}
		} else {
			buf = append(buf, ' ')
			buf = append(buf, n.String()...)
		}
	}
	return buf
}

// appendAttributesText appends attributes from FilterResult for text output.
// Only outputs what's in result.Attributes (lazy parsing - filter controls what's parsed).
func appendAttributesText(buf []byte, result bgpfilter.FilterResult) []byte {
	for code, attr := range result.Attributes {
		buf = append(buf, ' ')
		buf = appendAttributeText(buf, code, attr)
	}
	return buf
}

// appendAttributeText appends a single attribute for text output.
// Known attribute types are formatted with named keys (short aliases for API output);
// unknown types use "attr-N" with hex value.
// Short forms: next (next-hop), path (as-path), pref (local-preference),
// s-com (community), l-com (large-community), e-com (extended-community).
func appendAttributeText(buf []byte, code attribute.AttributeCode, attr attribute.Attribute) []byte {
	switch code { //nolint:exhaustive // common attributes; unknown handled after switch
	case attribute.AttrOrigin:
		switch o := attr.(type) {
		case *attribute.Origin:
			buf = append(buf, textparse.KWOrigin...)
			buf = append(buf, ' ')
			buf = appendLower(buf, o.String())
		case attribute.Origin:
			buf = append(buf, textparse.KWOrigin...)
			buf = append(buf, ' ')
			buf = appendLower(buf, o.String())
		}
		return buf
	case attribute.AttrASPath:
		if ap, ok := attr.(*attribute.ASPath); ok {
			buf = append(buf, textparse.ShortPath...)
			buf = append(buf, ' ')
			first := true
			for _, seg := range ap.Segments {
				for _, asn := range seg.ASNs {
					if !first {
						buf = append(buf, ',')
					}
					buf = strconv.AppendUint(buf, uint64(asn), 10)
					first = false
				}
			}
		}
		return buf
	case attribute.AttrNextHop:
		if nh, ok := attr.(*attribute.NextHop); ok {
			buf = append(buf, textparse.ShortNext...)
			buf = append(buf, ' ')
			buf = nh.Addr.AppendTo(buf)
		}
		return buf
	case attribute.AttrMED:
		switch m := attr.(type) {
		case *attribute.MED:
			buf = append(buf, textparse.KWMED...)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(uint32(*m)), 10)
		case attribute.MED:
			buf = append(buf, textparse.KWMED...)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(uint32(m)), 10)
		}
		return buf
	case attribute.AttrLocalPref:
		switch lp := attr.(type) {
		case *attribute.LocalPref:
			buf = append(buf, textparse.ShortPref...)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(uint32(*lp)), 10)
		case attribute.LocalPref:
			buf = append(buf, textparse.ShortPref...)
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(uint32(lp)), 10)
		}
		return buf
	case attribute.AttrCommunity:
		if c, ok := attr.(*attribute.Communities); ok {
			buf = append(buf, textparse.ShortSCom...)
			buf = append(buf, ' ')
			for i, comm := range *c {
				if i > 0 {
					buf = append(buf, ',')
				}
				buf = append(buf, comm.String()...)
			}
		}
		return buf
	case attribute.AttrLargeCommunity:
		if lc, ok := attr.(*attribute.LargeCommunities); ok {
			buf = append(buf, textparse.ShortLCom...)
			buf = append(buf, ' ')
			for i, comm := range *lc {
				if i > 0 {
					buf = append(buf, ',')
				}
				buf = append(buf, comm.String()...)
			}
		}
		return buf
	case attribute.AttrExtCommunity:
		if ec, ok := attr.(*attribute.ExtendedCommunities); ok {
			buf = append(buf, textparse.ShortXCom...)
			buf = append(buf, ' ')
			for i, comm := range *ec {
				if i > 0 {
					buf = append(buf, ',')
				}
				buf = hex.AppendEncode(buf, comm[:])
			}
		}
		return buf
	}
	// Unknown attribute code — format as "attr-N hex".
	// attr.Len() bounds hex output to RFC 4271 extended max (65535 bytes of
	// attribute value, 131070 hex chars). Stack scratch sized for the common
	// case; pathological inputs spill via append growth.
	var scratch [512]byte
	raw := scratch[:0]
	if n := attr.Len(); n > cap(raw) {
		raw = make([]byte, n)
	} else {
		raw = scratch[:n]
	}
	attr.WriteTo(raw, 0)
	buf = append(buf, "attr-"...)
	buf = strconv.AppendUint(buf, uint64(code), 10)
	buf = append(buf, ' ')
	buf = hex.AppendEncode(buf, raw)
	return buf
}

// appendStateChangeText appends a peer state change text line to buf,
// terminated by '\n'.
// Format: peer <ip> remote as <asn> state <state> [reason <reason>] .
func appendStateChangeText(buf []byte, peer *plugin.PeerInfo, state rpc.SessionState, reason string) []byte {
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	buf = strconv.AppendUint(buf, uint64(peer.PeerAS), 10)
	buf = append(buf, " state "...)
	buf = state.AppendTo(buf)
	if reason != "" {
		buf = append(buf, " reason "...)
		buf = append(buf, reason...)
	}
	buf = append(buf, '\n')
	return buf
}

// appendLower appends s to buf lowercased (ASCII only).
// Origin names are RFC-defined tokens (IGP/EGP/INCOMPLETE) -- strings.ToLower
// would allocate; this stays on the stack.
func appendLower(buf []byte, s string) []byte {
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf = append(buf, c)
	}
	return buf
}
