// Design: docs/architecture/exabgp-bridge.md -- the route attribute vocabulary
// Overview: bridge_command.go -- the line translator that reads this table
//
// One table states what an ExaBGP route line can carry, and one parser reads
// it. There were two parsers before 2026-09-05, one for `announce route` and
// one for `announce <afi> <safi>`, and they knew different halves of the
// grammar: the family-qualified one read four keywords and SKIPPED the rest, so
// `announce ipv4 unicast 10.0.1.0/24 next-hop 10.0.1.254 local-preference 200`
// put a route on the wire carrying the DEFAULT local preference. The script was
// acknowledged, nothing was logged, and the peer received a different route
// from the one the operator wrote (ai/rules/principles.md).

// RFC: rfc/short/rfc7911.md
package bridge

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// bridgeAttrArity says how an ExaBGP route attribute's value is written, which
// is what decides how many tokens the parser takes after the keyword.
type bridgeAttrArity uint8

const (
	// arityFlag is a keyword that stands alone and carries no value.
	arityFlag bridgeAttrArity = iota
	// arityValue is a keyword followed by exactly one token.
	arityValue
	// arityList is a keyword followed by one token or by a bracketed list,
	// which ExaBGP writes `[ a b c ]` and may write unspaced as `[a b c]`.
	arityList
	// arityGroup is a keyword followed by one token or a parenthesised group,
	// `( ... )`.
	arityGroup
)

// bridgeRouteAttr is one attribute of ExaBGP's route grammar and the ze
// update-text keyword it becomes.
//
// The table states the WHOLE ExaBGP vocabulary rather than the part ze happens
// to speak. A member with an empty Ze field is one ze cannot put on a wire, and
// the parser REFUSES a line that carries it, by name. Dropping it instead would
// announce a route the operator did not write, and the ack would tell the
// script it had worked.
//
// Source: ExaBGP src/exabgp/configuration/announce/ip.py, the RouteBuilder
// schema `children` map, plus the per-family additions in label.py (`label`),
// vpn.py (`rd`), path.py (`path-information`) and mup.py
// (`bgp-prefix-sid-srv6`).
type bridgeRouteAttr struct {
	// Ze is the update-text keyword this attribute becomes. Empty means ze has
	// no way to encode the attribute, and the line is refused.
	Ze string
	// Arity is how many tokens follow the keyword.
	Arity bridgeAttrArity
	// WireSilent marks an attribute that changes nothing a peer receives, so
	// consuming it and writing nothing into update text is the whole
	// translation. `name` labels the route for ExaBGP's own `show` output, and
	// `watchdog` names the group the route is given to, which the bridge
	// carries out as routeAttributes.Watchdog rather than as a keyword.
	WireSilent bool
	// EmptyAtEnd marks an attribute ExaBGP reads with the empty word when the
	// line ends at its keyword: the pin's Words.take answers the empty token at
	// the end (configuration/grammar/words.py) and lists '' among WATCHDOG's
	// examples, and 4.2 and 5.0 took the empty tokeniser() answer as the name
	// (watchdog and name, configuration/static/parser.py). Neither empty
	// value reaches a peer: `name` never does, and no `announce watchdog
	// <name>` can name the empty watchdog, so the route is sent without one.
	EmptyAtEnd bool
}

// bridgeAttrPrefixSIDSRv6 is ExaBGP's SRv6 service SID attribute, which ze's
// update text reads under the same name.
const bridgeAttrPrefixSIDSRv6 = "bgp-prefix-sid-srv6"

var bridgeRouteAttrs = map[string]bridgeRouteAttr{
	bridgeAttrNextHop:         {Ze: "nhop", Arity: arityValue},
	bridgeAttrOrigin:          {Ze: bridgeAttrOrigin, Arity: arityValue},
	bridgeAttrMED:             {Ze: bridgeAttrMED, Arity: arityValue},
	bridgeAttrLocalPreference: {Ze: bridgeAttrLocalPreference, Arity: arityValue},
	bridgeAttrASPath:          {Ze: bridgeAttrASPath, Arity: arityList},
	bridgeAttrCommunity:       {Ze: bridgeAttrCommunity, Arity: arityList},
	bridgeAttrLargeCommunity:  {Ze: bridgeAttrLargeCommunity, Arity: arityList},
	bridgeAttrExtCommunity:    {Ze: bridgeAttrExtCommunity, Arity: arityList},
	bridgeAttrAtomicAggregate: {Ze: bridgeAttrAtomicAggregate, Arity: arityFlag},
	bridgeAttrAggregator:      {Ze: bridgeAttrAggregator, Arity: arityGroup},
	bridgeAttrOriginatorID:    {Ze: bridgeAttrOriginatorID, Arity: arityValue},
	bridgeAttrClusterList:     {Ze: bridgeAttrClusterList, Arity: arityList},
	bridgeAttrAIGP:            {Ze: bridgeAttrAIGP, Arity: arityValue},
	bridgeAttrLabel:           {Ze: bridgeAttrLabel, Arity: arityList},
	"rd":                      {Ze: "rd", Arity: arityValue},
	bridgeAttrRouteDist:       {Ze: "rd", Arity: arityValue},
	bridgeAttrPathInformation: {Ze: bridgeAttrPathInformation, Arity: arityValue},
	bridgeAttrWatchdog:        {Arity: arityValue, WireSilent: true, EmptyAtEnd: true},
	bridgeAttrRouteName:       {Arity: arityValue, WireSilent: true, EmptyAtEnd: true},

	// `split` is not an attribute: it re-cuts the prefix into smaller ones, so
	// the parser carries its length out to the caller that owns the NLRI
	// (routeAttributes.Split) rather than writing a keyword.
	"split": {Arity: arityValue},

	bridgeAttrPrefixSIDSRv6: {Ze: bridgeAttrPrefixSIDSRv6, Arity: arityGroup},
	// The label-index Prefix-SID, `[ <index> ]` or `[ <index>, [ ( <base>,<range> )
	// ... ] ]` (prefix_sid, configuration/static/mpls.py in 4.2.25 and 5.0.1),
	// which ze's update text reads under the same name and spelling.
	"bgp-prefix-sid": {Ze: "bgp-prefix-sid", Arity: arityList},

	// ExaBGP's generic `attribute [ 0x<code> 0x<flag> 0x<data> ]`, one
	// attribute given as its wire octets (HexAttribute,
	// configuration/grammar/types/bgp.py at f9bfb853b), which ze's update text
	// reads under the same name and spelling. Its code is in the value, so it
	// is keyed by that code rather than by keyword (genericAsKeyword).
	bridgeAttrGeneric: {Ze: bridgeAttrGeneric, Arity: arityList},
}

// bridgeAttrGeneric is ExaBGP's generic attribute keyword.
const bridgeAttrGeneric = "attribute"

// codeRepeat reports whether an attribute writing a type code the line already
// wrote is a repeat the per-code rule owns: one of the two is ExaBGP's generic
// `attribute`, or two different keywords write the one code, as
// `bgp-prefix-sid` and `bgp-prefix-sid-srv6` both write the BGP Prefix-SID
// (code 40). The same keyword given twice is left to the per-keyword rules,
// which merge a community list. ExaBGP's API keeps the first of two attributes
// of one code when the code is not a list attribute or either is generic
// (add_attribute, "attribute.ID not in LIST_ATTRIBUTES or attribute.GENERIC or
// existing.GENERIC", configuration/grammar/tree/static.py at f9bfb853b); no two
// keywords share a list attribute's code (textparse.KeywordAttributeCode).
func codeRepeat(generic bool, holder, keyword string) bool {
	if generic {
		return true
	}
	if holder == bridgeAttrGeneric {
		return true
	}
	return holder != keyword
}

// genericCode answers the type code a generic attribute value writes, zero
// for a value ze's update text will refuse with its own reason. ze reads a
// generic attribute of a code one of its keywords writes into that keyword's
// typed form, so the bridge passes every generic attribute on as written and
// needs its code only to keep the first of two attributes of one code.
func genericCode(value string) attribute.AttributeCode {
	raw, err := textparse.ParseRawAttribute(value)
	if err != nil {
		return 0
	}
	return attribute.AttributeCode(raw.Code)
}

// hexOctet spells a type code as the generic attribute writes it, `0x` and two
// hexadecimal digits.
func hexOctet(code attribute.AttributeCode) string {
	var tb textbuf.Buffer
	return tb.Str("0x").Hex([]byte{byte(code)}).String()
}

// bridgeAttrNames lists the table's keywords, longest first, so a refusal can
// say what the bridge does read. Sorted so the message is stable.
func bridgeAttrNames() []string {
	names := make([]string, 0, len(bridgeRouteAttrs))
	for name := range bridgeRouteAttrs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// errAttributeNotEncodable is what the parser answers for an attribute ExaBGP
// writes and ze cannot put on a wire.
var errAttributeNotEncodable = errors.New("the ExaBGP bridge cannot encode this route attribute")

// routeAttributes is one ExaBGP route line's attributes, in the ze spelling.
type routeAttributes struct {
	// Tokens are the ze update-text attribute tokens, in the order they were
	// written. They sit BEFORE the nlri section in the finished command.
	Tokens []string
	// Split is the prefix length ExaBGP's `split /<n>` cuts the route into, or
	// zero when the line carries none.
	Split int
	// Warnings are what the line asked for and does not get, one sentence
	// each: a repeated attribute after its first, a `split` shorter than the
	// prefix. ExaBGP's API keeps 5.0's leniency for both and logs it (X-26,
	// X-27); a configuration file refuses them, which is not the bridge's to
	// judge. warnRouteLine logs them.
	Warnings []string
	// Watchdog is the watchdog the line gives its route to, or empty when it
	// names none. An announce under a watchdog is the watchdog's, as a
	// configured route is (c43f90f79, add_to_rib_watchdog from announce_route
	// at the pin), so the runner hands it to the bgp-watchdog plugin
	// (watchdogGiveCommand) rather than to the peers.
	Watchdog string
	// Held is ExaBGP's `withdraw` flag (INTERNAL_WITHDRAW): a route given to a
	// watchdog starts withdrawn, and waits for `announce watchdog <name>`. A
	// route under no watchdog is announced whatever the flag says
	// (add_to_rib_watchdog reads it only beside a watchdog).
	Held bool
	// written records each ze keyword the line carried, so a caller can ask
	// what was present without re-reading the token stream it produced.
	written map[string]bool
	// valueAt is where in Tokens the value of each community list and each
	// field of the NLRI sits, so a repeat of a list is merged into it
	// (mergedListValue) and a repeat of a field replaces it.
	valueAt map[string]int
	// codes records the ze keyword that wrote each path attribute type code,
	// so a generic attribute and a keyword of the same code keep the first.
	// A keyword repeated without a generic one is left to the rules below.
	codes map[attribute.AttributeCode]string
}

// Command renders the attributes as the ze text that precedes the nlri section.
func (a routeAttributes) Command() string { return textbuf.Join(a.Tokens, " ") }

// nextHopIPv6 reports whether the line names an IPv6 next hop, which moves an
// IPv4 unicast announcement from ze's `nlri` section to `mp ipv4/unicast`
// (RFC 8950 Section 3).
func (a routeAttributes) nextHopIPv6() bool {
	for i := 0; i+1 < len(a.Tokens); i++ {
		if a.Tokens[i] == a.nextHopKeyword() {
			return textparse.IPv6NextHopText(a.Tokens[i+1])
		}
	}
	return false
}

// nextHopKeyword is the ze spelling the attribute table writes for ExaBGP's
// next-hop, read from the table rather than restated.
func (a routeAttributes) nextHopKeyword() string { return bridgeRouteAttrs[bridgeAttrNextHop].Ze }

// Has reports whether the ze keyword was written. It reads the ze spelling
// rather than the ExaBGP one, so `rd` answers for `route-distinguisher` too.
func (a routeAttributes) Has(zeKeyword string) bool { return a.written[zeKeyword] }

// bridgeWatchdogUnnamed is logged for a route line that ends at `watchdog`.
// ExaBGP reads the empty word as the name (EmptyAtEnd), and no `announce
// watchdog <name>` can name the empty watchdog, so the route is given to none
// and sent now. The operator most likely left the name out.
const bridgeWatchdogUnnamed = "the line names watchdog with no name, so the route is given to no watchdog and sent now: " +
	"write watchdog <name> to have the route wait for announce watchdog <name>"

// parseRouteAttributes reads the attribute tail of one ExaBGP route line: every
// token after the NLRI that the line begins with.
//
// It REFUSES an attribute the table does not hold and one ze cannot encode,
// naming the keyword in both cases. It never skips a token: a route announced
// without an attribute the operator wrote is a different route, and the script
// that wrote it is acknowledged either way.
func parseRouteAttributes(parts []string) (routeAttributes, error) {
	return parseFamilyRouteAttributes("", parts)
}

// parseFamilyRouteAttributes is parseRouteAttributes for a line that names its
// family, which decides what a repeated next-hop means (bridgeNextHopLastFamily).
func parseFamilyRouteAttributes(family string, parts []string) (routeAttributes, error) {
	nextHopLast := bridgeNextHopLastFamily(family)
	out := routeAttributes{written: make(map[string]bool, len(parts)/2), valueAt: map[string]int{}, codes: map[attribute.AttributeCode]string{}}
	for i := 0; i < len(parts); {
		key := strings.ToLower(parts[i])

		// ExaBGP's `withdraw` flag is the INTERNAL_WITHDRAW attribute, which
		// add_to_rib_watchdog reads beside a watchdog alone (rib/outgoing.py at
		// the pin): an API route reaches it since c43f90f79, so a route given to
		// a watchdog with the flag starts withdrawn, and one under no watchdog is
		// announced. pack skips internal attributes, so the flag never reaches
		// the wire.
		if key == withdrawVerb {
			out.Held = true
			i++
			continue
		}

		attr, known := bridgeRouteAttrs[key]
		if !known {
			return routeAttributes{}, fmt.Errorf("%w: %q; the bridge reads %s",
				ErrLineNotTranslated, parts[i], textbuf.Join(bridgeAttrNames(), ", "))
		}

		if key == bridgeAttrWatchdog && i+1 == len(parts) {
			out.Warnings = append(out.Warnings, bridgeWatchdogUnnamed)
		}
		if attr.EmptyAtEnd && i+1 == len(parts) {
			break
		}
		value, next, err := takeAttrValue(attr.Arity, parts, i+1)
		if err != nil {
			return routeAttributes{}, fmt.Errorf("%w: %s %w; write %s %s",
				ErrLineNotTranslated, key, err, key, attrValueName(key))
		}
		i = next
		if attr.Ze == out.nextHopKeyword() {
			value = familyNextHop(family, value)
		}

		// `watchdog <name>` gives the route to a group rather than adding an
		// attribute, so it is carried out to the runner, which knows the script
		// that gives it (watchdogGiveCommand). The last one written wins, as a
		// repeated attribute keyword does in ExaBGP's collection.
		if key == bridgeAttrWatchdog {
			out.Watchdog = value
			continue
		}
		// `split /<n>` cuts the NLRI rather than adding an attribute, so it is
		// carried out to the caller that owns the prefix.
		if key == "split" {
			length, err := parseSplitLength(value)
			if err != nil {
				return routeAttributes{}, err
			}
			out.Split = length
			continue
		}
		if attr.WireSilent {
			continue
		}
		if attr.Ze == "" {
			return routeAttributes{}, fmt.Errorf("%w: %q", errAttributeNotEncodable, key)
		}

		code, _ := textparse.KeywordAttributeCode(attr.Ze)
		generic := attr.Ze == bridgeAttrGeneric
		written := strings.TrimSpace(key + " " + value)
		if generic {
			code = genericCode(value)
		}
		// An attribute whose type code the line already wrote, through its
		// keyword or as a generic attribute, keeps the first, and a warning
		// names the one dropped. This comes before the list merge: a generic
		// community after the community keyword is a second attribute, which
		// ExaBGP drops, not more communities. Two generic attributes of
		// different codes are two attributes.
		holder, held := out.codes[code]
		if held && codeRepeat(generic, holder, attr.Ze) {
			out.Warnings = append(out.Warnings, "the line gives attribute "+hexOctet(code)+" twice, through "+holder+
				" and through "+written+"; the first is kept and the second dropped: remove one of the two")
			continue
		}
		if code != 0 && !held {
			out.codes[code] = attr.Ze
			if generic {
				out.codes[code] = bridgeAttrGeneric
			}
		}
		if attr.Ze == bridgeAttrGeneric {
			out.Tokens = append(out.Tokens, attr.Ze, value)
			continue
		}

		if attr.Ze == bridgeAttrAggregator {
			single, err := aggregatorValue(value)
			if err != nil {
				return routeAttributes{}, err
			}
			value = single
		}
		if attr.Ze == bridgeAttrExtCommunity {
			if err := CheckExtendedCommunityList(value); err != nil {
				return routeAttributes{}, err
			}
			value = qualifyExtCommunities(value)
		}
		if attr.Ze == bridgeAttrPrefixSIDSRv6 {
			value = srv6WithBehavior(value)
		}
		// ExaBGP writes an ADD-PATH path identifier as an IPv4 address as well
		// as a number, and its own qa/api corpus uses the dotted form. ze reads
		// a uint32, so the dotted form is converted rather than passed on to
		// fail at a parser that would name the wrong thing.
		if attr.Ze == bridgeAttrPathInformation {
			converted, err := pathIDNumber(value)
			if err != nil {
				return routeAttributes{}, err
			}
			value = converted
		}

		// A community list given again is merged into the first: ExaBGP
		// merges "a list one given twice" for an API command as for a file
		// (add_attribute, configuration/grammar/tree/static.py at f9bfb853b).
		if at, listed := out.valueAt[attr.Ze]; listed && bridgeListAttrs[attr.Ze] {
			out.Tokens[at] = mergedListValue(out.Tokens[at], value)
			continue
		}
		// A label, a route distinguisher or a path identifier given again
		// replaces the first: each is a field of the NLRI, which ExaBGP sets
		// as it reads it (Collected.apply, `self.settings.set(spec.field,
		// value)` for Target.NLRI, configuration/grammar/tree/static.py at
		// f9bfb853b), so the last one read is the route's.
		if at, field := out.valueAt[attr.Ze]; field && bridgeNLRIFields[attr.Ze] {
			out.Tokens[at] = value
			continue
		}
		// A next-hop given again on a mup or mcast-vpn route replaces the
		// first, as the next-hop of every other family does not
		// (bridgeNextHopLastFamily).
		if at, field := out.valueAt[attr.Ze]; field && nextHopLast && attr.Ze == out.nextHopKeyword() {
			out.Tokens[at] = value
			continue
		}
		// Any other repeated attribute keeps its first value: the RIB holds
		// one attribute of each kind, and ExaBGP's API keeps the one it read
		// first and logs the one it dropped (X-26, X-27). Forwarding both
		// would let ze's parser pick, and it does not pick the first.
		if out.written[attr.Ze] {
			out.Warnings = append(out.Warnings, "the line repeats "+key+
				", the first is kept and "+strings.TrimSpace(key+" "+value)+" is dropped")
			continue
		}
		out.written[attr.Ze] = true
		out.Tokens = append(out.Tokens, attr.Ze)
		if attr.Arity != arityFlag {
			if bridgeListAttrs[attr.Ze] || bridgeNLRIFields[attr.Ze] || nextHopLast && attr.Ze == out.nextHopKeyword() {
				out.valueAt[attr.Ze] = len(out.Tokens)
			}
			out.Tokens = append(out.Tokens, value)
		}
	}
	return out, nil
}

// bridgeNextHopLastFamily reports whether a repeated next-hop replaces the
// first on a route of the family. ExaBGP reads a mup and an mcast-vpn route
// through SelectLine, whose _apply sets the next hop each time it reads one
// (`route = route.with_nexthop(ip)`, configuration/grammar/tree/select.py at
// f9bfb853b), as 5.0's mup and mvpn_route did (`change.nlri.nexthop =
// nexthop`); every other family keeps the first (static.first_next_hop). These
// two are the families whose NLRI is a field list, bridgePluginNLRIFamily.
func bridgeNextHopLastFamily(family string) bool {
	return bridgePluginNLRIFamily(family)
}

// bridgeMappedNextHopFamilies are the IPv6 families on which ExaBGP sends an
// IPv4 next hop IPv4-mapped, `::ffff:<ipv4>`, in 16 octets: the AFI 2 rows of
// Family.size whose next-hop lengths hold no 4 (protocol/family.py at
// f9bfb853b), which the encoder maps ("an IPv6 family without a four octet
// next hop of its own carries an IPv4 one as an IPv4-mapped IPv6 address",
// NLRICollection, bgp/message/update/nlri/collection.py at f9bfb853b), and mup,
// whose grammar maps it before the encoder sees it (MupNextHop, "an IPv4
// address is mapped into IPv6 for an IPv6 route", configuration/grammar/tree/
// select.py at f9bfb853b). The AFI 2 families that keep four octets are
// mcast-vpn and sr-policy, whose next hop is either family by its length.
//
// RFC 2545 Section 3 gives an IPv6 next hop no four octet form: "the global
// IPv6 address of the next hop, potentially followed by the link-local IPv6
// address of the next hop". 5.0 sent the four octets, which a peer resets the
// session on (RFC 7606 Section 7.11), so this is a change a 5.0 script meets.
var bridgeMappedNextHopFamilies = map[string]bool{
	bridgeAFIv6 + "/" + bridgeUnicastSAFI:   true,
	bridgeAFIv6 + "/" + bridgeMulticastSAFI: true,
	bridgeAFIv6 + "/" + zeLabeledSAFI:       true,
	bridgeAFIv6 + "/" + bridgeVPNSAFI:       true,
	bridgeAFIv6 + "/" + bridgeFlowSAFI:      true,
	bridgeAFIv6 + "/" + bridgeFlowVPNSAFI:   true,
	bridgeAFIv6 + "/" + bridgeMUPSAFI:       true,
}

// familyNextHop answers the next-hop of a route of the family as ExaBGP sends
// it: an IPv4 address on a bridgeMappedNextHopFamilies route is mapped into
// IPv6. Every other value, `self` included, is answered as written: `self`
// resolves against the session, and the engine refuses an IPv4 session's
// address for an IPv6 route (resolveNextHop), as ExaBGP answers the script
// "the route (ipv6) does not have the same family as the BGP tcp session"
// (Session.ip_self, bgp/neighbor/session.py at f9bfb853b).
func familyNextHop(family, value string) string {
	if !bridgeMappedNextHopFamilies[family] {
		return value
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return value
	}
	if !address.Is4() {
		return value
	}
	return netip.AddrFrom16(address.As16()).String()
}

// bridgeListAttrs are the attributes ExaBGP merges when a line gives one
// twice, its LIST_ATTRIBUTES (configuration/grammar/tree/static.py at
// f9bfb853b). 5.0 merged the extended communities alone (Attributes.add), and
// 4.2 none; the pin's merge is what a 4.2 or 5.0 script meets on upgrade.
var bridgeListAttrs = map[string]bool{
	bridgeAttrCommunity:      true,
	bridgeAttrLargeCommunity: true,
	bridgeAttrExtCommunity:   true,
}

// bridgeNLRIFields are the ze keywords of the values ExaBGP writes into the
// NLRI rather than into an attribute: RouteValue with Target.NLRI
// (configuration/grammar/tree/static.py at f9bfb853b).
var bridgeNLRIFields = map[string]bool{
	bridgeAttrLabel:           true,
	"rd":                      true,
	bridgeAttrPathInformation: true,
}

// mergedListValue is one community list holding the items of first, then
// those of added, each a bare item or a bracketed list, a repeated item
// included. The pin's add_attribute concatenates the communities and the
// extended communities, and keeps a repeated one; it merges the large
// communities as sorted(set(...)), so a repeat is dropped there. The bridge
// leaves that repeat in the list because ze drops it on the way out:
// compactLargeCommunities (bgp/reactor/session_egress_normalize.go) follows
// RFC 8092 Section 3, "Duplicate BGP Large Community values MUST NOT be
// transmitted", and the order of the values carries no meaning.
func mergedListValue(first, added string) string {
	items := listItems(first)
	items = append(items, listItems(added)...)
	var tb textbuf.Buffer
	return tb.Byte('[').Str(textbuf.Join(items, " ")).Byte(']').String()
}

// listItems answers the items of one list value, unwrapped from its brackets.
func listItems(value string) []string {
	inner := strings.TrimSpace(value)
	inner = strings.TrimPrefix(inner, "[")
	inner = strings.TrimSuffix(inner, "]")
	return strings.Fields(inner)
}

// srv6WithBehavior answers an ExaBGP SRv6 service group with its endpoint
// behavior written out: the opaque 0xFFFF when the group names a service and
// a SID and nothing more, any other group as written.
//
// ExaBGP reads `( l3-service <sid> )` with `behavior = 0xFFFF` until a value
// follows the SID (prefix_sid_srv6, configuration/static/mpls.py, 5.0.1), and
// ze's update text reads an omitted behavior as 0, so the bridge writes the
// pin's default out.
//
// RFC 9252 Section 3.1: "The opaque SRv6 Endpoint Behavior (i.e., value
// 0xFFFF) MAY be used when the advertising router wishes to abstract the
// actual behavior of its locally instantiated SRv6 SID.".
func srv6WithBehavior(group string) string {
	inner, bracketed := strings.CutPrefix(strings.TrimSpace(group), "(")
	if !bracketed {
		return group
	}
	fields := strings.Fields(strings.TrimSuffix(inner, ")"))
	if len(fields) != 2 {
		return group
	}
	if fields[0] != "l3-service" && fields[0] != "l2-service" {
		return group
	}
	var tb textbuf.Buffer
	return tb.Str("( ").Str(fields[0]).Byte(' ').Str(fields[1]).Str(" 0xFFFF )").String()
}

// warnRouteLine logs each warning a route line's attributes carry, once, when
// the line becomes ze commands.
func warnRouteLine(attrs routeAttributes) {
	for _, warning := range attrs.Warnings {
		slog.Warn("exabgp-bridge route line", "warning", warning)
	}
}

// pathIDNumber answers an ADD-PATH path identifier as the decimal ze reads.
//
// RFC 7911 Section 3: the Path Identifier is a four-octet field. ExaBGP accepts
// it as a number or as an IPv4 address, whose four octets ARE that field in
// network order, and its qa/api corpus writes `path-information 1.2.3.4`.
func pathIDNumber(value string) (string, error) {
	if _, err := strconv.ParseUint(value, 10, 32); err == nil {
		return value, nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.Is4() {
		return "", fmt.Errorf("%w: path-information %q is neither a number nor an IPv4 address",
			ErrLineNotTranslated, value)
	}
	octets := addr.As4()
	id := uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])
	var tb textbuf.Buffer
	return tb.Uint32(id).String(), nil
}

// parseSplitLength reads ExaBGP's `/<n>` split length, which it writes with the
// leading slash a prefix carries.
func parseSplitLength(value string) (int, error) {
	length, err := strconv.Atoi(strings.TrimPrefix(value, "/"))
	if err != nil || length <= 0 {
		return 0, fmt.Errorf("%w: %q", errSplitLength, value)
	}
	return length, nil
}

// splitRouteBody cuts one ExaBGP route body into its NLRI tokens and its
// attribute tokens, at the first token the attribute table names.
//
// The NLRI is not always one token. `announce route 10.0.0.0/24` writes one,
// `announce ipv4 mup mup-isd 10.0.1.0/24` writes two, and VPLS writes eight.
// Reading parts[0] as the whole NLRI is what made `announce ipv4 mup mup-isd
// 10.0.1.0/24 ...` announce a prefix literally spelled `mup-isd`.
func splitRouteBody(parts []string) (nlri, attrs []string) {
	return splitRouteBodyForFamily("", parts)
}

// bridgeNLRIOwnedAttrs are the three keywords that belong to the NLRI of a
// family whose wire form a PLUGIN encodes, rather than to the attribute block.
//
// ze reads them in both positions for a family it parses itself, so cutting the
// NLRI at them costs nothing there. For mup and mvpn it costs everything: the
// plugin encoder is handed the whole token run and reads the route
// distinguisher out of it, so `mp ipv4/mup add route-type mup-isd rd 100:100
// prefix 10.0.1.0/24` arrived at the encoder with no RD at all.
var bridgeNLRIOwnedAttrs = map[string]bool{
	"rd": true, bridgeAttrRouteDist: true, bridgeAttrLabel: true, bridgeAttrPathInformation: true,
}

// splitRouteBodyForFamily cuts a route body at the first token that belongs to
// the attribute block rather than the NLRI, which depends on the family.
func splitRouteBodyForFamily(family string, parts []string) (nlri, attrs []string) {
	pluginNLRI := bridgePluginNLRIFamily(family)
	for i, token := range parts {
		key := strings.ToLower(token)
		if pluginNLRI && bridgeNLRIOwnedAttrs[key] {
			continue
		}
		if _, known := bridgeRouteAttrs[key]; known || key == withdrawVerb {
			return parts[:i], parts[i:]
		}
	}
	return parts, nil
}

// bridgePluginNLRIFamily reports whether a family's NLRI is encoded by a ze
// plugin from a whole token run, rather than parsed prefix by prefix.
//
// The two members are the two SAFIs whose ExaBGP spelling is a field list:
// `mup-isd 10.0.1.0/24` and `shared-join rp 10.99.199.1 group 239.251.255.228`.
func bridgePluginNLRIFamily(family string) bool {
	_, safi, ok := strings.Cut(family, "/")
	return ok && (safi == bridgeMUPSAFI || safi == "mvpn")
}

// takeAttrValue reads one attribute's value at index start and answers it with
// the index that follows it.
func takeAttrValue(arity bridgeAttrArity, parts []string, start int) (string, int, error) {
	switch arity {
	case arityFlag:
		return "", start, nil
	case arityValue:
		if start >= len(parts) {
			return "", start, errMissingAttrValue
		}
		return parts[start], start + 1, nil
	case arityList:
		return takeDelimited(parts, start, '[', ']')
	case arityGroup:
		return takeDelimited(parts, start, '(', ')')
	default:
		panic("BUG: invalid bridge attribute arity")
	}
}

// attrValueName names what an attribute's value is, for a refusal that tells
// the operator what to write after the keyword.
func attrValueName(key string) string {
	if key == bridgeAttrPathInformation {
		// RFC 7911 Section 3: the Path Identifier, which ExaBGP reads as a
		// number or as an IPv4 address (pathIDNumber).
		return "<path identifier, a number or an IPv4 address>"
	}
	if key == bridgeAttrAggregator {
		return "<asn>:<router-id>, in brackets or not"
	}
	return "<value>"
}

// aggregatorValue answers the one `<asn>:<router-id>` word of an AGGREGATOR
// value, unwrapped from its brackets, or refuses the value.
//
// ExaBGP reads an AGGREGATOR "in brackets or not: `( 65000:10.0.0.1 )`,
// `(65000:10.0.0.1)`" (AggregatorType.parse, configuration/grammar/types/bgp.py
// at f9bfb853b), and refuses anything but one word, bare or bracketed. ze's
// update text reads the bare word. A group of several words would otherwise
// become extra ze tokens past the repeated-attribute check, so it is refused.
func aggregatorValue(written string) (string, error) {
	word := written
	if inner, bracketed := strings.CutPrefix(word, "("); bracketed {
		word = strings.TrimSpace(strings.TrimSuffix(inner, ")"))
	}
	if strings.ContainsAny(word, " \t()") {
		return "", fmt.Errorf("%w: aggregator %q: %w", ErrLineNotTranslated, written, errAggregatorShape)
	}
	return word, nil
}

var (
	errAggregatorShape = errors.New("aggregator takes one <asn>:<router-id>, " +
		"in brackets or not: write aggregator ( 65000:10.0.0.1 )")
	errMissingAttrValue = errors.New("is written with no value")
	errUnclosedGroup    = errors.New("unclosed group")
)

// takeDelimited reads a value that MAY be written as a delimited group. ExaBGP
// spaces its brackets inconsistently -- `[ 2:1 ]`, `[2:1]` and `[2:1` followed
// by `]` all occur in its own test corpus -- so the group is found by counting
// the delimiters rather than by matching whole tokens.
func takeDelimited(parts []string, start int, open, close byte) (string, int, error) {
	if start >= len(parts) {
		return "", start, errMissingAttrValue
	}
	first := parts[start]
	if first[0] != open {
		return first, start + 1, nil
	}

	depth := 0
	var value textbuf.Buffer
	for i := start; i < len(parts); i++ {
		if i > start {
			value.Byte(' ')
		}
		value.Str(parts[i])
		depth += strings.Count(parts[i], string(open)) - strings.Count(parts[i], string(close))
		if depth == 0 {
			return value.String(), i + 1, nil
		}
	}
	return "", start, errUnclosedGroup
}

// bridgeMUPRouteTypes maps the four MUP route types, which ExaBGP writes as
// the FIRST token of the NLRI and ze writes after the keyword `route-type`, to
// the ze keyword naming the field ExaBGP writes bare after the type.
//
// ExaBGP reads `mup-isd <ip prefix>`, `mup-dsd <ip address>`, `mup-t1st <ip
// prefix>` and `mup-t2st <endpoint address>` (the grammar comments above each
// reader in configuration/static/mpls.py, 5.0.1), and ze's encoder reads those
// fields as `prefix` and `address` (nlri/mup/encode.go, EncodeNLRIHex).
var bridgeMUPRouteTypes = map[string]string{
	"mup-isd": mupFieldPrefix, "mup-dsd": mupFieldAddress, "mup-t1st": mupFieldPrefix, "mup-t2st": mupFieldAddress,
}

// The two ze keywords that name the field after a MUP route type.
const (
	mupFieldPrefix  = "prefix"
	mupFieldAddress = "address"
)

// shapePluginNLRI rewrites an NLRI token run into the spelling the family's ze
// plugin encoder reads. It answers the run unchanged for every family whose
// spelling already agrees.
//
// mvpn agrees: ExaBGP writes `shared-join rp <ip> group <ip> rd <rd> source-as
// <asn>` and the ze encoder reads exactly that. mup does not: ExaBGP leads with
// the route type and its prefix or address bare, ze names both
// (internal/component/bgp/plugins/nlri/mup/encode.go, EncodeNLRIHex).
func shapePluginNLRI(family string, tokens []string) []string {
	_, safi, ok := strings.Cut(family, "/")
	if !ok {
		return tokens
	}
	if safi != bridgeMUPSAFI {
		return tokens
	}
	if len(tokens) < 2 {
		return tokens
	}
	field, isRouteType := bridgeMUPRouteTypes[strings.ToLower(tokens[0])]
	if !isRouteType {
		return tokens
	}

	shaped := make([]string, 0, len(tokens)+2)
	shaped = append(shaped, "route-type", tokens[0], field, tokens[1])
	return append(shaped, tokens[2:]...)
}

// CheckExtendedCommunityList answers CheckExtendedCommunityItem for every item
// of an extended-community value, one item or a bracketed list.
func CheckExtendedCommunityList(value string) error {
	for item := range strings.FieldsSeq(strings.Trim(value, "[] ")) {
		if err := CheckExtendedCommunityItem(item); err != nil {
			return err
		}
	}
	return nil
}

// qualifyExtCommunities names the TYPE of an extended community ExaBGP wrote
// bare, and leaves every other spelling alone.
//
// ExaBGP accepts `0:0` and means a route target by it: its own decoder renders
// the same eight octets back as `target:0:0`, which is what
// test/exabgp-compat/api/api-attributes-vpn.ci carries in the command column.
// ze reads the word before the first colon as the type, so the bare form
// reaches it as the unknown type `0`.
//
// RFC 4360 Section 4: the Route Target is "the two-octet AS specific" and the
// four-octet forms of the transitive extended community, which is the pair and
// the triple this writes the word onto.
func qualifyExtCommunities(value string) string {
	if !strings.HasPrefix(value, "[") {
		return qualifyExtCommunity(value)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	fields := strings.Fields(inner)
	for i := range fields {
		fields[i] = qualifyExtCommunity(fields[i])
	}
	var tb textbuf.Buffer
	return tb.Byte('[').Str(textbuf.Join(fields, " ")).Byte(']').String()
}

// qualifyExtCommunity writes `target:` onto one bare `<asn>:<n>` community, and
// the IPv4 administrator form onto ExaBGP's `target4:` and `origin4:`
// (ipv4AdministratorCommunity).
func qualifyExtCommunity(value string) string {
	head, tail, ok := strings.Cut(value, ":")
	if !ok || tail == "" {
		return value
	}
	if zeType, isIPv4Type := bridgeIPv4AdministratorTypes[strings.ToLower(head)]; isIPv4Type {
		return ipv4AdministratorCommunity(zeType, tail, value)
	}
	// A leading NUMBER is an administrator field, so the type word is missing.
	// A leading word is the type itself: `target`, `origin`, `rate-limit`.
	if _, err := strconv.ParseUint(head, 10, 32); err != nil {
		return value
	}
	var tb textbuf.Buffer
	return tb.Str("target:").Str(value).String()
}

// bridgeIPv4AdministratorTypes maps ExaBGP's `target4` and `origin4` to the ze
// type that carries the same sub-type.
//
// ExaBGP gives both the IPv4 address specific header whatever the administrator
// is written as: `'target4': bytes([0x01, 0x02])`, `'origin4': bytes([0x01,
// 0x03])`, packed `'LH'`, a four octet administrator then a two octet local one
// (_HEADER and _ENCODE, configuration/static/parser.py in 5.0.1,
// configuration/grammar/types/bgp.py at the 6.0 pin). 5.0 sent
// `target4:71200:12000` as 01 02 00011620 2EE0 (qa/old/commands-5.0.json).
// RFC 4360 Section 4: the IPv4 Address Specific Route Target is type 0x01,
// sub-type 0x02, and Section 5 the Route Origin is sub-type 0x03.
var bridgeIPv4AdministratorTypes = map[string]string{
	"target4": "target",
	"origin4": "origin",
}

// ipv4AdministratorCommunity answers `<zeType>:<a.b.c.d>:<local>` for the
// `<administrator>:<local>` of a `target4:` or `origin4:` community, which ze
// reads as the IPv4 address specific form. The administrator is a dotted quad,
// or a number ExaBGP's _integer reads (decimal, `0x` hexadecimal, a trailing
// `L`) that is written as the four octets it packs. A value that is neither is
// answered as written, so ze's parser refuses it by name.
func ipv4AdministratorCommunity(zeType, tail, written string) string {
	administrator, local, ok := strings.Cut(tail, ":")
	if !ok {
		return written
	}
	if _, err := netip.ParseAddr(administrator); err == nil {
		var tb textbuf.Buffer
		return tb.Str(zeType).Byte(':').Str(administrator).Byte(':').Str(local).String()
	}
	number, err := strconv.ParseUint(strings.TrimSuffix(administrator, "L"), 0, 32)
	if err != nil {
		return written
	}
	address := netip.AddrFrom4([4]byte{byte(number >> 24), byte(number >> 16), byte(number >> 8), byte(number)})
	var tb textbuf.Buffer
	return tb.Str(zeType).Byte(':').Str(address.String()).Byte(':').Str(local).String()
}
