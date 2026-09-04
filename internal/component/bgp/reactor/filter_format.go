// Design: docs/architecture/core-design.md — policy filter chain
// Related: filter_chain.go — policy filter chain execution
// Related: filter_delta.go — text delta to wire-mod-ops consumer of the same format

package reactor

import (
	"net/netip"
	"reflect"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
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
// The "as-path" token carries the AS path information the route traversed, not
// the AS_PATH attribute as encoded: on a session that did not negotiate the
// four-octet AS capability the two are different (RFC 6793 Section 4.2.3, and
// asPathForFilter below).
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
	policyAttrOrigin:            attribute.AttrOrigin,
	policyAttrASPath:            attribute.AttrASPath,
	policyAttrNextHop:           attribute.AttrNextHop,
	policyAttrMED:               attribute.AttrMED,
	policyAttrLocalPreference:   attribute.AttrLocalPref,
	policyAttrAtomicAggregate:   attribute.AttrAtomicAggregate,
	policyAttrAggregator:        attribute.AttrAggregator,
	policyAttrCommunity:         attribute.AttrCommunity,
	policyAttrOriginatorID:      attribute.AttrOriginatorID,
	policyAttrClusterList:       attribute.AttrClusterList,
	policyAttrExtendedCommunity: attribute.AttrExtCommunity,
	policyAttrAIGP:              attribute.AttrAIGP,
	policyAttrLargeCommunity:    attribute.AttrLargeCommunity,
}

// AppendAttrsForFilter appends selected attributes from wire into buf as
// space-separated "<name> <value>" pairs. Only attributes named in declared
// are included. If declared is empty, all parseable attributes are included.
// Returns buf unchanged when attrs is nil.
//
// The "as-path" pair carries the AS path information rather than the AS_PATH
// attribute: see asPathForFilter.
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
		parsed := attrForFilter(attrs, code)
		if parsed == nil {
			continue
		}
		buf, first = appendSingleAttr(buf, parsed, first)
	}
	return buf
}

// attrForFilter returns the attribute to RENDER for one filter-text attribute
// code, and nil when the UPDATE does not carry it or its wire bytes do not
// parse. Both arms of AppendAttrsForFilter go through here, so a filter that
// declares its attributes and a filter that takes them all read one subject.
func attrForFilter(attrs *attribute.AttributesWire, code attribute.AttributeCode) attribute.Attribute {
	parsed, err := attrs.Get(code)
	if err != nil || parsed == nil {
		return nil
	}
	if code != attribute.AttrASPath {
		return parsed
	}
	return asPathForFilter(attrs, parsed)
}

// asPathForFilter returns the AS path a filter must judge: the AS numbers the
// route really traversed, whatever ASN width the session negotiated.
//
// A peer that did not negotiate the four-octet AS capability sends a two-octet
// AS_PATH holding AS_TRANS (23456) wherever a four-octet AS number belongs, and
// sends the real numbers in AS4_PATH. Rendering AS_PATH alone therefore shows
// every text-mode filter the ENCODING instead of the path, so a rule naming a
// four-octet ASN accepts the route it exists to reject.
//
// RFC 6793 Section 4.2.3: "If the AS4_PATH attribute is also received, both of
// the attributes will be used to construct the exact AS path information, and
// therefore the information carried by both of the attributes will be
// considered for AS path loop detection."
//
// attribute.MergeAS4Path owns the construction, including the AS-number-count
// comparison that ignores an AS4_PATH longer than the AS_PATH. Presence is read
// from the span bitset, which takes no lock, no scan and no parse, so the
// common path (a four-octet session carrying no AS4_PATH) costs one bit test
// and allocates nothing.
//
// Four branches return the encoded AS_PATH instead of a reconstruction, and a
// reader MUST be able to tell them apart:
//
//   - AS4_PATH ABSENT is the legitimate answer. The AS_PATH already carries the
//     AS path information, and nothing is logged.
//   - A MALFORMED AS4_PATH is the peer's doing, and RFC 6793 Section 6 decides
//     it: "A NEW BGP speaker that receives a malformed AS4_PATH attribute in an
//     UPDATE message from an OLD BGP speaker MUST discard the attribute and
//     continue processing the UPDATE message. The error SHOULD be logged
//     locally for analysis." Ze discards it and logs it.
//   - The two type assertions and the index read are GUARD MISSES. No peer can
//     reach them: attrs.Get answers AttrASPath with *attribute.ASPath and
//     AttrAS4Path with *attribute.AS4Path by construction (knownAttrParsers,
//     internal/core/bgp/attribute/wire.go), and the index verdict Has reads is
//     the immutable one Get already passed in attrForFilter. Reaching one is a
//     Ze defect, so each one speaks. Silence would put the AS_TRANS subject
//     this function exists to remove back on the branch nobody looks at, and
//     nothing else would ever report it.
//
// A miss degrades to the encoded path and MUST NOT drop the route: the filter
// chain still runs, and the log line is what tells an operator that its verdict
// was taken on the wrong subject. Every line goes through fwdLogger, so
// ze.log.bgp.reactor.forward damps a peer that repeats a malformed AS4_PATH.
func asPathForFilter(attrs *attribute.AttributesWire, parsed attribute.Attribute) attribute.Attribute {
	present, err := attrs.Has(attribute.AttrAS4Path)
	if err != nil {
		fwdLogger().Warn("filter text: AS path reconstruction skipped, attribute index unreadable",
			"error", err)
		return parsed
	}
	if !present {
		return parsed
	}

	asPath, ok := parsed.(*attribute.ASPath)
	if !ok {
		fwdLogger().Warn("filter text: AS path reconstruction skipped, AS_PATH parsed to an unexpected type",
			"type", reflect.TypeOf(parsed))
		return parsed
	}

	as4Parsed, err := attrs.Get(attribute.AttrAS4Path)
	if err != nil {
		// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
		// AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST
		// discard the attribute and continue processing the UPDATE message.
		// The error SHOULD be logged locally for analysis."
		fwdLogger().Warn("filter text: malformed AS4_PATH discarded, filters judge the encoded AS_PATH",
			"error", err)
		return parsed
	}

	as4Path, ok := as4Parsed.(*attribute.AS4Path)
	if !ok {
		fwdLogger().Warn("filter text: AS path reconstruction skipped, AS4_PATH parsed to an unexpected type",
			"type", reflect.TypeOf(as4Parsed))
		return parsed
	}

	return attribute.MergeAS4Path(asPath, as4Path)
}

// appendAllAttrs appends all known attributes from wire in a stable order.
func appendAllAttrs(buf []byte, attrs *attribute.AttributesWire) []byte {
	order := []string{
		policyAttrOrigin, policyAttrASPath, policyAttrNextHop, policyAttrMED, policyAttrLocalPreference,
		policyAttrAtomicAggregate, policyAttrAggregator, policyAttrCommunity, policyAttrOriginatorID,
		policyAttrClusterList, policyAttrExtendedCommunity, policyAttrAIGP, policyAttrLargeCommunity,
	}
	first := true
	for _, name := range order {
		parsed := attrForFilter(attrs, attrNameToCode[name])
		if parsed == nil {
			continue
		}
		buf, first = appendSingleAttr(buf, parsed, first)
	}
	return buf
}

// filterUnnamedAttrPhrase is the one leading phrase the unnamed-type warning
// carries, so an operator's log scanner matches one string (ai/rules/cli.md).
// It names the consequence rather than the cause: the attribute is absent from
// the subject, so every filter on the chain judges the route without it.
const filterUnnamedAttrPhrase = "filter text: attribute dropped, no renderer names its type"

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
	case attribute.Origin:
		buf = a.AppendText(buf)
	case *attribute.ASPath:
		buf = a.AppendText(buf)
	case *attribute.NextHop:
		buf = a.AppendText(buf)
	case attribute.MED:
		buf = a.AppendText(buf)
	case attribute.LocalPref:
		buf = a.AppendText(buf)
	case attribute.AtomicAggregate:
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
	case attribute.ClusterList:
		buf = a.AppendText(buf)
	case attribute.LargeCommunities:
		buf = a.AppendText(buf)
	case attribute.ExtendedCommunities:
		buf = a.AppendText(buf)
	case *attribute.AIGP:
		buf = a.AppendText(buf)
	default:
		// A GUARD MISS, and the only one this switch can have. Every arm above
		// names the type its parser returns (knownAttrParsers,
		// internal/core/bgp/attribute/wire.go), and attrForFilter hands over
		// nothing else, so no peer can reach this branch: arriving here is a Ze
		// defect. Five arms named a pointer form no parser builds, and this
		// branch is what would have said so on the first UPDATE.
		//
		// The subject is built once for the whole chain, so a dropped attribute
		// is dropped for every filter on that peer at once, and the chain still
		// runs to a verdict. Degrade and SPEAK, for the reason asPathForFilter
		// states above: silence puts the wrong subject on the branch nobody
		// looks at, and nothing else would ever report it. Every line goes
		// through fwdLogger, so ze.log.bgp.reactor.forward damps it.
		//
		// No compile-time exhaustiveness check can replace this. The switch is
		// over an interface, and any package can implement attribute.Attribute.
		fwdLogger().Warn(filterUnnamedAttrPhrase, "type", reflect.TypeOf(attr))
	}

	if len(buf) == sep {
		// Nothing was appended — restore buf (drop the leading space too).
		return buf[:start], first
	}
	return buf, false
}
