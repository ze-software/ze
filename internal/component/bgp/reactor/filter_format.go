// Design: docs/architecture/core-design.md — policy filter chain
// Related: filter_chain.go — policy filter chain execution

package reactor

import (
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// FormatUpdateForFilter formats both attributes AND NLRI from a wire UPDATE
// into the text protocol consumed by filter plugins. The format is:
//
//	"<attr> <val> ... nlri <family> <op> [<prefix>...]"
//
// Legacy IPv4-unicast NLRI (from the UPDATE's NLRI and Withdrawn-Routes
// sections) is emitted as "nlri ipv4/unicast add|del <prefix>...". MP_REACH_NLRI
// and MP_UNREACH_NLRI attributes (RFC 4760) are emitted with their own family
// tokens. Each family is a separate "nlri <family> <op> ..." block.
//
// For families whose NLRI is a plain CIDR prefix (unicast, multicast,
// mpls-label in AFI IPv4/IPv6), the prefix list is emitted inline so text-
// mode filters can match directly. For non-CIDR families (EVPN, Flowspec,
// VPN, BGP-LS, MVPN, etc.) a MARKER block `nlri <family> <op>` is emitted
// WITHOUT prefixes, so a text-mode filter plugin attached to a session
// carrying those families can still tell that an update exists for a given
// family. A filter plugin that needs to inspect non-CIDR NLRI bytes MUST
// declare `raw=true` in its `FilterRegistration` and parse the wire payload
// from `FilterUpdateInput.Raw`. The text marker alone is insufficient for
// per-prefix decisions on non-CIDR families and is intentionally advisory.
//
// Returns "" if attrs is nil and wireUpdate has no NLRI sections. Attrs-only
// output (no nlri tokens) is valid when wireUpdate is nil or carries no
// reachability / withdrawal information.
func FormatUpdateForFilter(attrs *attribute.AttributesWire, wireUpdate *wireu.WireUpdate, declared []string) string {
	attrText := FormatAttrsForFilter(attrs, declared)
	if wireUpdate == nil {
		return attrText
	}

	var blocks []string

	// Legacy IPv4 unicast NLRI (RFC 4271 Section 4.3).
	if raw, err := wireUpdate.NLRI(); err == nil && len(raw) > 0 {
		if prefixes := wireu.ParseIPv4Prefixes(raw); len(prefixes) > 0 {
			blocks = append(blocks, formatNLRIBlock("ipv4/unicast", "add", prefixes))
		}
	}
	// Legacy IPv4 unicast withdrawn (RFC 4271 Section 4.3 Withdrawn Routes).
	if raw, err := wireUpdate.Withdrawn(); err == nil && len(raw) > 0 {
		if prefixes := wireu.ParseIPv4Prefixes(raw); len(prefixes) > 0 {
			blocks = append(blocks, formatNLRIBlock("ipv4/unicast", "del", prefixes))
		}
	}

	// MP_REACH_NLRI and MP_UNREACH_NLRI (RFC 4760).
	if mp, err := wireUpdate.MPReach(); err == nil && mp != nil {
		if block := formatMPBlock(mp.Family(), "add", mp.Prefixes()); block != "" {
			blocks = append(blocks, block)
		}
	}
	if mpu, err := wireUpdate.MPUnreach(); err == nil && mpu != nil {
		if block := formatMPBlock(mpu.Family(), "del", mpu.Prefixes()); block != "" {
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 {
		return attrText
	}
	if attrText == "" {
		return strings.Join(blocks, " ")
	}
	return attrText + " " + strings.Join(blocks, " ")
}

// formatMPBlock formats one MP_REACH / MP_UNREACH NLRI section for a filter
// text block. For CIDR-prefix families (unicast/multicast/mpls-label in
// IPv4/IPv6) the prefixes are included inline. For non-CIDR families a
// marker block with no prefixes is emitted so text-mode filters still learn
// that the family is present in the update; filters needing per-NLRI
// decisions must declare raw=true.
func formatMPBlock(fam family.Family, op string, prefixes []netip.Prefix) string {
	if isCIDRFamily(fam) {
		if len(prefixes) == 0 {
			return ""
		}
		return formatNLRIBlock(fam.String(), op, prefixes)
	}
	// Non-CIDR: marker block, prefixes intentionally omitted. See
	// FormatUpdateForFilter godoc.
	return "nlri " + fam.String() + " " + op
}

// cidrSAFIs is the set of SAFIs whose NLRI wire format is a plain CIDR
// prefix parseable by `wireu.ParsePrefixes`. Declared as a map so the
// exhaustive linter does not flag the bounded check in isCIDRFamily.
var cidrSAFIs = map[family.SAFI]struct{}{
	family.SAFIUnicast:   {},
	family.SAFIMulticast: {},
	family.SAFIMPLSLabel: {},
}

// isCIDRFamily reports whether `fam` is an address family whose NLRI wire
// format is a plain CIDR prefix. Covers IPv4/IPv6 unicast, multicast, and
// mpls-label (RFC 8277). Everything else (EVPN, Flowspec, VPN, BGP-LS,
// MVPN, MUP, RTC, ...) has a family-specific NLRI encoding and is therefore
// marker-only in the filter text protocol.
func isCIDRFamily(fam family.Family) bool {
	if fam.AFI != family.AFIIPv4 && fam.AFI != family.AFIIPv6 {
		return false
	}
	_, ok := cidrSAFIs[fam.SAFI]
	return ok
}

// formatNLRIBlock formats one "nlri <family> <op> <prefix>..." block.
func formatNLRIBlock(family, op string, prefixes []netip.Prefix) string {
	parts := make([]string, 0, 3+len(prefixes))
	parts = append(parts, "nlri", family, op)
	for _, p := range prefixes {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, " ")
}

// attrNameToCode maps filter text attribute names to wire codes.
var attrNameToCode = map[string]attribute.AttributeCode{
	"origin":             attribute.AttrOrigin,
	"as-path":            attribute.AttrASPath,
	"next-hop":           attribute.AttrNextHop,
	"med":                attribute.AttrMED,
	"local-preference":   attribute.AttrLocalPref,
	"atomic-aggregate":   attribute.AttrAtomicAggregate,
	"aggregator":         attribute.AttrAggregator,
	"community":          attribute.AttrCommunity,
	"originator-id":      attribute.AttrOriginatorID,
	"cluster-list":       attribute.AttrClusterList,
	"extended-community": attribute.AttrExtCommunity,
	"large-community":    attribute.AttrLargeCommunity,
}

// FormatAttrsForFilter formats selected attributes from wire into text for filter input.
// Only attributes named in `declared` are included. Returns space-separated "name value" pairs.
// If declared is empty, all parseable attributes are included.
func FormatAttrsForFilter(attrs *attribute.AttributesWire, declared []string) string {
	if attrs == nil {
		return ""
	}

	var parts []string

	if len(declared) == 0 {
		// No declared list: format all attributes.
		parts = formatAllAttrs(attrs)
	} else {
		// Only format declared attributes.
		for _, name := range declared {
			code, ok := attrNameToCode[name]
			if !ok {
				continue
			}
			parsed, err := attrs.Get(code)
			if err != nil || parsed == nil {
				continue
			}
			if text := formatSingleAttr(parsed); text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, " ")
}

// formatAllAttrs formats all known attributes from wire.
func formatAllAttrs(attrs *attribute.AttributesWire) []string {
	order := []string{
		"origin", "as-path", "next-hop", "med", "local-preference",
		policyAttrAtomicAggregate, "aggregator", "community", "originator-id",
		"cluster-list", "extended-community", "large-community",
	}
	var parts []string
	for _, name := range order {
		code := attrNameToCode[name]
		parsed, err := attrs.Get(code)
		if err != nil || parsed == nil {
			continue
		}
		if text := formatSingleAttr(parsed); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

// formatSingleAttr formats one attribute as "name value" text.
func formatSingleAttr(attr attribute.Attribute) string {
	switch a := attr.(type) {
	case *attribute.Origin:
		return fmt.Sprintf("origin %s", attribute.FormatOrigin(uint8(*a)))

	case *attribute.ASPath:
		var asns []uint32
		for _, seg := range a.Segments {
			asns = append(asns, seg.ASNs...)
		}
		if len(asns) == 0 {
			return ""
		}
		return fmt.Sprintf("as-path %s", attribute.FormatASPath(asns))

	case *attribute.NextHop:
		if !a.Addr.IsValid() {
			return ""
		}
		return fmt.Sprintf("next-hop %s", a.Addr.String())

	case *attribute.MED:
		return fmt.Sprintf("med %d", uint32(*a))

	case *attribute.LocalPref:
		return fmt.Sprintf("local-preference %d", uint32(*a))

	case *attribute.AtomicAggregate:
		return policyAttrAtomicAggregate

	case *attribute.Aggregator:
		return fmt.Sprintf("aggregator %d:%s", a.ASN, a.Address.String())

	case attribute.Communities:
		comms := make([]uint32, len(a))
		for i, c := range a {
			comms[i] = uint32(c)
		}
		return fmt.Sprintf("community %s", attribute.FormatCommunities(comms))

	case *attribute.ClusterList:
		if len(*a) == 0 {
			return ""
		}
		ids := make([]string, len(*a))
		for i, id := range *a {
			ids[i] = fmt.Sprintf("%d.%d.%d.%d", byte(id>>24), byte(id>>16), byte(id>>8), byte(id))
		}
		return fmt.Sprintf("cluster-list %s", strings.Join(ids, " "))

	case attribute.LargeCommunities:
		lcs := make([]attribute.LargeCommunity, len(a))
		copy(lcs, a)
		return fmt.Sprintf("large-community %s", attribute.FormatLargeCommunities(lcs))

	case attribute.ExtendedCommunities:
		ecs := make([]attribute.ExtendedCommunity, len(a))
		copy(ecs, a)
		return fmt.Sprintf("extended-community %s", attribute.FormatExtendedCommunities(ecs))
	}

	return ""
}
