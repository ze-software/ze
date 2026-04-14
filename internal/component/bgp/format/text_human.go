// Design: docs/architecture/api/json-format.md — human-readable text rendering
// Overview: text.go — message format dispatch

package format

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	bgpfilter "codeberg.org/thomas-mangin/ze/internal/component/bgp/filter"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/textparse"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// formatFilterResultText formats FilterResult as text.
// Uses AnnouncedByFamily()/WithdrawnByFamily() for RFC 4760-correct next-hop per family.
// ctx provides ADD-PATH state per family.
func formatFilterResultText(peer *plugin.PeerInfo, result bgpfilter.FilterResult, msgID uint64, direction string, ctx *bgpctx.EncodingContext) string {
	var sb strings.Builder
	var scratch [64]byte

	// Uniform header: peer <ip> remote as <asn> <direction> update <id>
	sb.WriteString("peer ")
	sb.Write(peer.Address.AppendTo(scratch[:0]))
	sb.WriteString(" remote as ")
	sb.Write(strconv.AppendUint(scratch[:0], uint64(peer.PeerAS), 10))
	sb.WriteByte(' ')
	sb.WriteString(direction)
	sb.WriteString(" update ")
	sb.Write(strconv.AppendUint(scratch[:0], msgID, 10))

	announced := result.AnnouncedByFamily(ctx)
	withdrawn := result.WithdrawnByFamily(ctx)

	if len(announced) == 0 && len(withdrawn) == 0 {
		// Empty UPDATE (End-of-RIB marker or attribute-only): minimal text line
		sb.WriteString("\n")
		return sb.String()
	}

	// Attributes (shared across all families)
	formatAttributesText(&sb, result, scratch[:])

	// Announced routes: next <nh> nlri <fam> add <nlri>[,<nlri>]...
	for _, fam := range announced {
		sb.WriteString(" " + textparse.ShortNext + " ")
		sb.Write(fam.NextHop.AppendTo(scratch[:0]))
		sb.WriteString(" nlri ")
		sb.WriteString(fam.Family)
		sb.WriteString(" add ")
		writeNLRIList(&sb, fam.NLRIs)
	}

	// Withdrawn routes: nlri <fam> del <nlri>[,<nlri>]...
	for _, fam := range withdrawn {
		sb.WriteString(" nlri ")
		sb.WriteString(fam.Family)
		sb.WriteString(" del ")
		writeNLRIList(&sb, fam.NLRIs)
	}

	sb.WriteString("\n")
	return sb.String()
}

// writeNLRIList writes NLRIs in compact format.
// INET NLRIs use comma-separated CIDRs: prefix <a>,<b>.
// Other NLRIs use keyword boundary: <nlri1> <nlri2>.
// Uses AppendString/AppendKey for INET NLRIs to avoid per-NLRI string allocations.
func writeNLRIList(sb *strings.Builder, nlris []nlri.NLRI) {
	if len(nlris) == 0 {
		return
	}

	// Check if first NLRI is INET (supports zero-alloc append methods).
	firstINET, useComma := nlris[0].(*nlri.INET)

	var scratch [64]byte
	if useComma {
		sb.Write(firstINET.AppendString(scratch[:0]))
	} else {
		sb.WriteString(nlris[0].String())
	}
	for _, n := range nlris[1:] {
		if useComma {
			sb.WriteByte(',')
			if inet, ok := n.(*nlri.INET); ok {
				sb.Write(inet.AppendKey(scratch[:0]))
			} else {
				sb.WriteString(n.String())
			}
		} else {
			sb.WriteByte(' ')
			sb.WriteString(n.String())
		}
	}
}

// formatAttributesText formats attributes from FilterResult for text output.
// Only outputs what's in result.Attributes (lazy parsing - filter controls what's parsed).
// scratch is a caller-provided buffer for zero-alloc integer/address formatting.
func formatAttributesText(sb *strings.Builder, result bgpfilter.FilterResult, scratch []byte) {
	for code, attr := range result.Attributes {
		sb.WriteString(" ")
		formatAttributeText(sb, code, attr, scratch)
	}
}

// formatAttributeText formats a single attribute for text output.
// Known attribute types are formatted with named keys (short aliases for API output);
// unknown types use "attr-N" with hex value.
// Short forms: next (next-hop), path (as-path), pref (local-preference),
// s-com (community), l-com (large-community), e-com (extended-community).
// scratch is a caller-provided buffer for zero-alloc integer/address formatting.
func formatAttributeText(sb *strings.Builder, code attribute.AttributeCode, attr attribute.Attribute, scratch []byte) {
	switch code { //nolint:exhaustive // common attributes; unknown handled after switch
	case attribute.AttrOrigin:
		switch o := attr.(type) {
		case *attribute.Origin:
			sb.WriteString(textparse.KWOrigin + " ")
			sb.WriteString(strings.ToLower(o.String()))
		case attribute.Origin:
			sb.WriteString(textparse.KWOrigin + " ")
			sb.WriteString(strings.ToLower(o.String()))
		}
		return
	case attribute.AttrASPath:
		if ap, ok := attr.(*attribute.ASPath); ok {
			sb.WriteString(textparse.ShortPath + " ")
			first := true
			for _, seg := range ap.Segments {
				for _, asn := range seg.ASNs {
					if !first {
						sb.WriteString(",")
					}
					sb.Write(strconv.AppendUint(scratch[:0], uint64(asn), 10))
					first = false
				}
			}
		}
		return
	case attribute.AttrNextHop:
		if nh, ok := attr.(*attribute.NextHop); ok {
			sb.WriteString(textparse.ShortNext + " ")
			sb.Write(nh.Addr.AppendTo(scratch[:0]))
		}
		return
	case attribute.AttrMED:
		switch m := attr.(type) {
		case *attribute.MED:
			sb.WriteString(textparse.KWMED + " ")
			sb.Write(strconv.AppendUint(scratch[:0], uint64(uint32(*m)), 10))
		case attribute.MED:
			sb.WriteString(textparse.KWMED + " ")
			sb.Write(strconv.AppendUint(scratch[:0], uint64(uint32(m)), 10))
		}
		return
	case attribute.AttrLocalPref:
		switch lp := attr.(type) {
		case *attribute.LocalPref:
			sb.WriteString(textparse.ShortPref + " ")
			sb.Write(strconv.AppendUint(scratch[:0], uint64(uint32(*lp)), 10))
		case attribute.LocalPref:
			sb.WriteString(textparse.ShortPref + " ")
			sb.Write(strconv.AppendUint(scratch[:0], uint64(uint32(lp)), 10))
		}
		return
	case attribute.AttrCommunity:
		if c, ok := attr.(*attribute.Communities); ok {
			sb.WriteString(textparse.ShortSCom + " ")
			for i, comm := range *c {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(comm.String())
			}
		}
		return
	case attribute.AttrLargeCommunity:
		if lc, ok := attr.(*attribute.LargeCommunities); ok {
			sb.WriteString(textparse.ShortLCom + " ")
			for i, comm := range *lc {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(comm.String())
			}
		}
		return
	case attribute.AttrExtCommunity:
		if ec, ok := attr.(*attribute.ExtendedCommunities); ok {
			sb.WriteString(textparse.ShortXCom + " ")
			for i, comm := range *ec {
				if i > 0 {
					sb.WriteString(",")
				}
				fmt.Fprintf(sb, "%x", comm[:])
			}
		}
		return
	}
	// Unknown attribute code — format as "attr-N hex"
	attrBuf := make([]byte, attr.Len())
	attr.WriteTo(attrBuf, 0)
	fmt.Fprintf(sb, "attr-%d %x", code, attrBuf)
}

func formatStateChangeText(peer *plugin.PeerInfo, state, reason string) string {
	if reason != "" {
		return fmt.Sprintf("peer %s remote as %d state %s reason %s\n", peer.Address, peer.PeerAS, state, reason)
	}
	return fmt.Sprintf("peer %s remote as %d state %s\n", peer.Address, peer.PeerAS, state)
}
