// Design: docs/architecture/core-design.md — policy dry run (show policy test)
// Related: policy_dryrun_display.go — the dry-run display these renderers write
// Related: filter_flowspec.go — the FlowSpec destination block of the same display

package reactor

import (
	"net/netip"
	"reflect"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// appendLegacyBlock appends the "nlri add|del <prefix>..." block of the legacy
// NLRI (op add) or Withdrawn Routes (op del) section. bufEmpty is true when buf
// has no content appended since the outer call began; no leading space
// separator is emitted then.
func appendLegacyBlock(buf []byte, op string, section []byte, addPath, withdraw, bufEmpty bool) []byte {
	if !bufEmpty {
		buf = append(buf, ' ')
	}
	buf = append(buf, policyAttrNLRI...)
	buf = append(buf, ' ')
	buf = append(buf, op...)
	return appendCIDRPrefixes(buf, family.IPv4Unicast, section, addPath, withdraw)
}

// appendMPBlock appends one MP_REACH_NLRI (op add) or MP_UNREACH_NLRI (op del)
// section, whose NLRI or Withdrawn Routes field is section, as an
// "mp <family> <op>" block. For CIDR-prefix families (unicast, multicast,
// mpls-label in IPv4/IPv6) the prefixes are included inline, and a section with
// none (an End-of-RIB MP_UNREACH_NLRI) emits no block. For non-CIDR families a
// marker block with no prefixes is emitted so text-mode filters still learn
// that the family is present; filters needing per-NLRI decisions must declare
// raw=true. bufEmpty is true when buf has no content appended since the outer
// call began; no leading space separator is emitted then.
func appendMPBlock(buf []byte, fam family.Family, op string, section []byte, addPath, bufEmpty bool) []byte {
	cidr := fam.PlainCIDR()
	if cidr && len(section) == 0 {
		return buf
	}
	if !bufEmpty {
		buf = append(buf, ' ')
	}
	buf = append(buf, policyAttrMP...)
	buf = append(buf, ' ')
	buf = append(buf, fam.String()...)
	buf = append(buf, ' ')
	buf = append(buf, op...)
	if !cidr {
		return buf // Non-CIDR: marker block, prefixes intentionally omitted.
	}
	return appendCIDRPrefixes(buf, fam, section, addPath, op == nlriOpDel)
}

// appendMalformedBlock appends the block of a section that does not parse:
// "nlri <op> invalid" for a legacy section (keyword policyAttrNLRI), or
// "mp afi-0/safi-0 <op> invalid" for an MP_REACH_NLRI (op add) or
// MP_UNREACH_NLRI (op del) attribute (keyword policyAttrMP). An MP attribute
// that does not parse names no family, so its block names the zero family
// (AFI 0 and SAFI 0 are reserved, so no real family reads as it), as
// nlriSectionOf names none. The "invalid" token is the one a section that does
// not frame ends with (appendCIDRPrefixes): no prefix filter accepts it.
// bufEmpty is true when buf has no content appended since the outer call
// began; no leading space separator is emitted then.
func appendMalformedBlock(buf []byte, keyword, op string, bufEmpty bool) []byte {
	if !bufEmpty {
		buf = append(buf, ' ')
	}
	buf = append(buf, keyword...)
	buf = append(buf, ' ')
	if keyword == policyAttrMP {
		buf = family.Family{}.AppendTo(buf)
		buf = append(buf, ' ')
	}
	buf = append(buf, op...)
	return append(buf, " invalid"...)
}

// attrForFilter returns the attribute to RENDER for one attribute code of the
// code, and nil when the UPDATE does not carry it or its wire bytes do not
// parse. Every attribute the dry run shows goes through here, so the AS path it
// shows is the path the route traversed (asPathForFilter).
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

// asPathForFilter returns the AS path the dry run shows: the AS numbers the
// route really traversed, whatever ASN width the session negotiated.
//
// A peer that did not negotiate the four-octet AS capability sends a two-octet
// AS_PATH holding AS_TRANS (23456) wherever a four-octet AS number belongs, and
// sends the real numbers in AS4_PATH. Rendering AS_PATH alone therefore shows
// the operator the ENCODING instead of the path, so a route a rule naming a
// four-octet ASN rejects reads as if it should pass.
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
//     Ze defect, so each one speaks. Silence would put the AS_TRANS path
//     this function exists to remove back on the branch nobody looks at, and
//     nothing else would ever report it.
//
// A miss degrades to the encoded path and MUST NOT drop the route: the dry run
// still answers, and the log line is what tells an operator that the display
// shows the encoded path. Every line goes through fwdLogger, so
// ze.log.bgp.reactor.forward damps a peer that repeats a malformed AS4_PATH.
func asPathForFilter(attrs *attribute.AttributesWire, parsed attribute.Attribute) attribute.Attribute {
	present, err := attrs.Has(attribute.AttrAS4Path)
	if err != nil {
		fwdLogger().Warn("policy dry run: AS path reconstruction skipped, attribute index unreadable",
			"error", err)
		return parsed
	}
	if !present {
		return parsed
	}

	asPath, ok := parsed.(*attribute.ASPath)
	if !ok {
		fwdLogger().Warn("policy dry run: AS path reconstruction skipped, AS_PATH parsed to an unexpected type",
			"type", reflect.TypeOf(parsed))
		return parsed
	}

	as4Parsed, err := attrs.Get(attribute.AttrAS4Path)
	if err != nil {
		// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
		// AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST
		// discard the attribute and continue processing the UPDATE message.
		// The error SHOULD be logged locally for analysis."
		fwdLogger().Warn("policy dry run: malformed AS4_PATH discarded, the encoded AS_PATH is shown",
			"error", err)
		return parsed
	}

	as4Path, ok := as4Parsed.(*attribute.AS4Path)
	if !ok {
		fwdLogger().Warn("policy dry run: AS path reconstruction skipped, AS4_PATH parsed to an unexpected type",
			"type", reflect.TypeOf(as4Parsed))
		return parsed
	}

	return attribute.MergeAS4Path(asPath, as4Path)
}

// filterUnnamedAttrPhrase is the one leading phrase the unnamed-type warning
// carries, so an operator's log scanner matches one string (ai/rules/cli.md).
// It names the consequence rather than the cause: the attribute is absent from
// what the dry run shows.
const filterUnnamedAttrPhrase = "policy dry run: attribute not shown, no renderer names its type"

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
		// The display is built once for the whole UPDATE, so a dropped attribute
		// is missing from the whole answer, and the dry run still
		// runs to a verdict. Degrade and SPEAK, for the reason asPathForFilter
		// states above: silence puts the wrong display on the branch nobody
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

// appendCIDRPrefixes appends " <prefix>" for every NLRI of a CIDR-family
// section, framed by wireu.CIDREntry and named by the family's route key, so a
// dry run shows exactly the prefixes a prefix match judges (policychain): a
// path identifier or a label stack is never read as prefix bits. A section that
// does not frame or key ends with the token "invalid", which names no prefix.
func appendCIDRPrefixes(buf []byte, fam family.Family, section []byte, addPath, withdraw bool) []byte {
	routeKey := nlrisplit.GetPrefixKey(fam)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	for off := 0; off < len(section); {
		_, cidr, next, ok := wireu.CIDREntry(section, off, addPath)
		if !ok {
			return append(buf, " invalid"...)
		}
		off = next
		key, err := routeKey(cidr, scratch[:], withdraw)
		if err != nil {
			return append(buf, " invalid"...)
		}
		p := cidrKeyPrefix(fam, key)
		if !p.IsValid() {
			return append(buf, " invalid"...)
		}
		buf = append(buf, ' ')
		buf = p.AppendTo(buf)
	}
	return buf
}

// cidrKeyPrefix reads a CIDR route key (length octet, then the prefix octets)
// back as the prefix it names. The zero Prefix, which IsValid refuses, answers
// a key that names none.
func cidrKeyPrefix(fam family.Family, key []byte) netip.Prefix {
	if len(key) == 0 {
		return netip.Prefix{}
	}
	var a [16]byte
	copy(a[:], key[1:])
	if fam.AFI == family.AFIIPv4 {
		return netip.PrefixFrom(netip.AddrFrom4([4]byte(a[:4])), int(key[0]))
	}
	return netip.PrefixFrom(netip.AddrFrom16(a), int(key[0]))
}
