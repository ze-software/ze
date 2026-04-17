// Design: docs/architecture/core-design.md — policy filter chain
// Related: filter_chain.go — policy filter chain execution
// Related: filter_delta.go — text delta to wire-mod-ops consumer of the same format

package reactor

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// AppendUpdateForFilter appends the filter-text rendering of an UPDATE
// (attributes + NLRI) into buf and returns the extended slice. The format is:
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
// VPN, BGP-LS, MVPN, etc.) a marker block `nlri <family> <op>` is emitted
// WITHOUT prefixes, so a text-mode filter plugin attached to a session
// carrying those families can still tell that an update exists for a given
// family. A filter plugin that needs to inspect non-CIDR NLRI bytes MUST
// declare `raw=true` in its `FilterRegistration` and parse the wire payload
// from `FilterUpdateInput.Raw`.
//
// Returns buf unchanged if attrs is nil and wireUpdate has no NLRI sections.
// Attrs-only output (no nlri tokens) is valid when wireUpdate is nil or
// carries no reachability / withdrawal information.
//
// Zero-alloc: the caller owns buf; AppendUpdateForFilter appends via the
// stdlib builtin `append` and returns the extended slice. No fmt.Sprintf,
// no strings.Join, no intermediate []string.
func AppendUpdateForFilter(buf []byte, attrs *attribute.AttributesWire, wireUpdate *wireu.WireUpdate, declared []string) []byte {
	start := len(buf)
	buf = AppendAttrsForFilter(buf, attrs, declared)

	if wireUpdate == nil {
		return buf
	}

	// Legacy IPv4 unicast NLRI (RFC 4271 Section 4.3).
	if raw, err := wireUpdate.NLRI(); err == nil && len(raw) > 0 {
		if prefixes := wireu.ParseIPv4Prefixes(raw); len(prefixes) > 0 {
			if len(buf) > start {
				buf = append(buf, ' ')
			}
			buf = appendNLRIBlock(buf, "ipv4/unicast", "add", prefixes)
		}
	}
	// Legacy IPv4 unicast withdrawn (RFC 4271 Section 4.3 Withdrawn Routes).
	if raw, err := wireUpdate.Withdrawn(); err == nil && len(raw) > 0 {
		if prefixes := wireu.ParseIPv4Prefixes(raw); len(prefixes) > 0 {
			if len(buf) > start {
				buf = append(buf, ' ')
			}
			buf = appendNLRIBlock(buf, "ipv4/unicast", "del", prefixes)
		}
	}

	// MP_REACH_NLRI and MP_UNREACH_NLRI (RFC 4760).
	if mp, err := wireUpdate.MPReach(); err == nil && mp != nil {
		buf = appendMPBlock(buf, mp.Family(), "add", mp.Prefixes(), len(buf) == start)
	}
	if mpu, err := wireUpdate.MPUnreach(); err == nil && mpu != nil {
		buf = appendMPBlock(buf, mpu.Family(), "del", mpu.Prefixes(), len(buf) == start)
	}

	return buf
}

// appendMPBlock appends one MP_REACH / MP_UNREACH NLRI section. For CIDR-prefix
// families (unicast/multicast/mpls-label in IPv4/IPv6) the prefixes are
// included inline. For non-CIDR families a marker block with no prefixes is
// emitted so text-mode filters still learn that the family is present;
// filters needing per-NLRI decisions must declare raw=true. bufEmpty is true
// when buf currently has no appended content since the outer call began; no
// leading space separator is emitted in that case.
func appendMPBlock(buf []byte, fam family.Family, op string, prefixes []netip.Prefix, bufEmpty bool) []byte {
	if isCIDRFamily(fam) {
		if len(prefixes) == 0 {
			return buf
		}
		if !bufEmpty {
			buf = append(buf, ' ')
		}
		return appendNLRIBlock(buf, fam.String(), op, prefixes)
	}
	// Non-CIDR: marker block, prefixes intentionally omitted.
	if !bufEmpty {
		buf = append(buf, ' ')
	}
	buf = append(buf, "nlri "...)
	buf = append(buf, fam.String()...)
	buf = append(buf, ' ')
	buf = append(buf, op...)
	return buf
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

// appendNLRIBlock appends one "nlri <family> <op> <prefix>..." block to buf.
// prefixes are rendered via netip.Prefix.AppendTo (zero-alloc on warm buf).
func appendNLRIBlock(buf []byte, fam, op string, prefixes []netip.Prefix) []byte {
	buf = append(buf, "nlri "...)
	buf = append(buf, fam...)
	buf = append(buf, ' ')
	buf = append(buf, op...)
	for _, p := range prefixes {
		buf = append(buf, ' ')
		buf = p.AppendTo(buf)
	}
	return buf
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

// AppendAttrsForFilter appends selected attributes from wire into buf as
// space-separated "<name> <value>" pairs. Only attributes named in declared
// are included. If declared is empty, all parseable attributes are included.
// Returns buf unchanged when attrs is nil.
func AppendAttrsForFilter(buf []byte, attrs *attribute.AttributesWire, declared []string) []byte {
	if attrs == nil {
		return buf
	}
	if len(declared) == 0 {
		return appendAllAttrs(buf, attrs)
	}
	first := true
	for _, name := range declared {
		code, ok := attrNameToCode[name]
		if !ok {
			continue
		}
		parsed, err := attrs.Get(code)
		if err != nil || parsed == nil {
			continue
		}
		buf, first = appendSingleAttr(buf, parsed, first)
	}
	return buf
}

// appendAllAttrs appends all known attributes from wire in a stable order.
func appendAllAttrs(buf []byte, attrs *attribute.AttributesWire) []byte {
	order := []string{
		"origin", "as-path", "next-hop", "med", "local-preference",
		policyAttrAtomicAggregate, "aggregator", "community", "originator-id",
		"cluster-list", "extended-community", "large-community",
	}
	first := true
	for _, name := range order {
		code := attrNameToCode[name]
		parsed, err := attrs.Get(code)
		if err != nil || parsed == nil {
			continue
		}
		buf, first = appendSingleAttr(buf, parsed, first)
	}
	return buf
}

// appendSingleAttr appends one attribute as "<name> <value>" text into buf,
// with a leading space separator when first is false. Returns the updated
// buffer and the updated first flag (false if anything was appended).
func appendSingleAttr(buf []byte, attr attribute.Attribute, first bool) ([]byte, bool) {
	start := len(buf)
	if !first {
		buf = append(buf, ' ')
	}
	sep := len(buf)

	switch a := attr.(type) {
	case *attribute.Origin:
		buf = a.AppendText(buf)
	case *attribute.ASPath:
		buf = a.AppendText(buf)
	case *attribute.NextHop:
		buf = a.AppendText(buf)
	case *attribute.MED:
		buf = a.AppendText(buf)
	case *attribute.LocalPref:
		buf = a.AppendText(buf)
	case *attribute.AtomicAggregate:
		buf = a.AppendText(buf)
	case *attribute.Aggregator:
		// (*Aggregator).AppendText emits just the element form "<asn>:<ip>"
		// (and nothing when Address is invalid). Prepend the attribute token
		// only if AppendText will actually write something, so an invalid
		// aggregator drops cleanly without leaving a dangling "aggregator ".
		before := len(buf)
		buf = append(buf, "aggregator "...)
		after := len(buf)
		buf = a.AppendText(buf)
		if len(buf) == after {
			buf = buf[:before]
		}
	case attribute.Communities:
		buf = a.AppendText(buf)
	case attribute.OriginatorID:
		buf = a.AppendText(buf)
	case *attribute.ClusterList:
		buf = a.AppendText(buf)
	case attribute.LargeCommunities:
		buf = a.AppendText(buf)
	case attribute.ExtendedCommunities:
		buf = a.AppendText(buf)
	}

	if len(buf) == sep {
		// Nothing was appended — restore buf (drop the leading space too).
		return buf[:start], first
	}
	return buf, false
}
