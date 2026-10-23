// Design: docs/architecture/exabgp-bridge.md — ExaBGP `flow route` to the ze update-text command
// Overview: bridge_command.go — TranslateLine, the flat family forms, and the ze flowspec
// spellings this file reuses (normalizeFlowSpecComponentToken, flowSpecComponentType,
// normalizeFlowSpecComponentValue, normalizeFlowSpecExtCommunityToken, flowSpecVPNFamily)
// Related: bridge_event.go — the JSON direction, which renders the same actions back to ExaBGP
// RFC: rfc/short/rfc8955.md -- the FlowSpec components and traffic actions read here
// RFC: rfc/short/rfc8956.md -- the IPv6 prefix components and their offset

package bridge

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// flowVerbAdd and flowVerbDel are the two ze update-text verbs. ExaBGP's
	// `announce` is one and its `withdraw` is the other, and the caller has
	// already read which.
	flowVerbAdd = "add"
	flowVerbDel = "del"

	// flowRouteDiscard is the ze extended community for ExaBGP's `discard`.
	//
	// ExaBGP encodes the bare word as a traffic-rate of 0 with a zero AS
	// (src/exabgp/configuration/flow/parser.py, discard), which is the same
	// eight octets `rate-limit 0` produces, and the same eight octets ze writes
	// for `rate-limit:0`. The compat fixtures spell it that way too, so a
	// reader comparing test/exabgp-compat/api/api-flow-merge.ci with this file
	// sees one word for one wire value.
	flowRouteDiscard = "rate-limit:0"

	// flowRouteRedirectNextHop is ze's spelling of the Simpson
	// redirect-to-next-hop action of draft-simpson-idr-flowspec-redirect-02,
	// type 0x08 subtype 0x00 value 0, which points at the route's next hop.
	// ExaBGP 6.0 writes it from a bare `redirect-to-nexthop`, from
	// `redirect-to-nexthop-simpson`, and from `redirect-simpson <ip>`, which
	// also sets the next hop (configuration/grammar/types/flow.py, _simpson).
	flowRouteRedirectNextHop = "redirect-to-nexthop-draft"

	// flowRouteCopyNextHop is ze's spelling of ExaBGP's `copy-simpson <ip>`
	// action: the same type and subtype as the line above with the low value
	// bit set, which is the copy semantic. Written alone, ze reads the word as
	// this one-word action.
	flowRouteCopyNextHop = "copy-to-nexthop"

	// flowRouteIETFRedirect and flowRouteIETFCopy are ze's words for the
	// redirect-to-IP community of draft-ietf-idr-flowspec-redirect-ip-16, which
	// holds the address itself, written `<word> <ip>`. ExaBGP 6.0 writes it
	// from `redirect <ip>`, `redirect-to-nexthop <ip>`,
	// `redirect-to-nexthop-ietf <ip>` and, with the C bit, `copy <ip>`
	// (configuration/grammar/types/flow.py, _nexthop_ietf).
	flowRouteIETFRedirect = "redirect-to-nexthop"
	flowRouteIETFCopy     = "copy-to-nexthop"
)

var (
	errFlowRouteNoMatch      = errors.New("exabgp flow route: the route states no match component")
	errFlowRouteUnterminated = errors.New("exabgp flow route: a value list opens with '[' and never closes with ']'")
)

// convertFlowRoute translates one ExaBGP `flow route ...` body, in either the
// braced or the flat form, into the ze update-text command for the family.
// It reports false when the body is not a flow route at all.
//
// The two answers are separate questions, and a caller MUST read both. The bool
// says the body is a flow route, so this function owns it. The error says the
// translation failed, and it names the token that stopped it. A flow route the
// bridge cannot read is refused with that name rather than sent as a route with
// the token dropped, because a filter that silently loses a match component
// installs a wider filter than the operator wrote (ai/rules/principles.md).
//
// family is the ze family the caller read from the line, such as `ipv4/flow`.
// The body can still move it: an `rd` makes the route a VPN route, and an IPv6
// prefix in a bare `source` or `destination` makes it an IPv6 route. ExaBGP
// reads the AFI off the prefix the same way, because `flow route` states none
// (src/exabgp/configuration/flow/parser.py, source and destination).
func convertFlowRoute(selector, family, verb, body string) (Command, bool, error) {
	tokens, ok := flowRouteBody(body)
	if !ok {
		return Command{}, false, nil
	}

	if verb != flowVerbAdd && verb != flowVerbDel {
		return Command{}, true, fmt.Errorf("exabgp flow route: verb %q is neither %q nor %q", verb, flowVerbAdd, flowVerbDel)
	}
	if !flowRouteFamily(family) {
		return Command{}, true, fmt.Errorf("exabgp flow route: %q is not a flowspec family", family)
	}

	route := flowRoute{family: family}
	if err := route.read(tokens); err != nil {
		return Command{}, true, err
	}
	// RFC 8955 Section 4.2
	if err := flowRouteMatchable(body, route.family, route.types); err != nil {
		return Command{}, true, err
	}
	return route.command(selector, verb), true, nil
}

// flowRouteMatchable answers flowspec.ErrFlowSpecUnmatchable, naming the route
// as the script wrote it, when types holds a combination no packet can match,
// and nil when a packet can. The verb does not matter: ExaBGP 6.0 drops an ICMP
// and port route as it parses the line, before announce_flow or withdraw_flow
// sees it (configuration/grammar/tree/flow.py, propagated), so api_flow returns
// no route and both answer the script answer_error. The refusal is the
// bridge's rather than ze's, so the log line names the ExaBGP route and nothing
// is sent.
//
// The bridge holds the component types, not their operator lists, so it judges
// the combinations of types (ICMP beside a port or TCP flags). A route whose
// protocol or fragment values contradict a transport component reaches ze,
// whose update text encoder judges it again with the values and refuses it
// (docs/exabgp/exabgp-differences.md).
func flowRouteMatchable(route, family string, types []flowspec.FlowComponentType) error {
	afi := flowspec.AFIIPv4
	if strings.HasPrefix(family, "ipv6/") {
		afi = flowspec.AFIIPv6
	}
	// The nil operator lookup: the bridge holds types only.
	err := flowspec.CheckMatchable(afi, func(t flowspec.FlowComponentType) bool { return slices.Contains(types, t) }, nil)
	if err != nil {
		return fmt.Errorf("exabgp flow route %q is not sent: %w", route, err)
	}
	return nil
}

// flowRoute is what one ExaBGP flow route body says, held in the words ze uses
// rather than the words ExaBGP wrote.
type flowRoute struct {
	// family is the ze family this route lands in. It starts as the caller's
	// and the body can move it, so it is read at the end rather than the start.
	family string
	// rd is the route distinguisher of a VPN flow route, empty otherwise.
	rd string
	// nhop is the next-hop, which ExaBGP's `next-hop`, `copy` and address-form
	// `redirect` each set.
	nhop string
	// nlri holds the match components, already in ze's qualified spelling, in
	// the order the body wrote them. ze sorts them by RFC 8955 type code when it
	// encodes, so this order reaches no wire (types.go writeComponentsSorted).
	nlri []string
	// types holds the component type of each match component in nlri, so the
	// route's matchability is judged on the components themselves rather than
	// on a reading back of the words (flowRouteMatchable).
	types []flowspec.FlowComponentType
	// communities and largeCommunities pass through untouched: ze and ExaBGP
	// spell both the same way.
	communities      []string
	largeCommunities []string
	// extended holds the extended communities the body names directly, plus the
	// `scope` interface sets.
	extended []string
	// actions holds the extended communities the `then` block's traffic actions
	// produce. They are written after everything in extended, because ExaBGP
	// sorts its extended communities by value before packing them
	// (bgp/message/update/attribute/community/extended/communities.py, pack) and
	// every traffic action's type octet (0x80) sorts after an interface set's
	// (0x07 or 0x47). Writing them last reproduces that order.
	actions []string
}

// flowRouteBody reads the `flow route` header off a body and answers the tokens
// that follow it. It reports false for a body that names another form.
//
// The optional name ExaBGP allows between `route` and `{` is dropped: it names
// the route inside an ExaBGP configuration and ze carries no counterpart
// (src/exabgp/configuration/flow/route.py, syntax).
func flowRouteBody(body string) ([]string, bool) {
	tokens := flowRouteTokens(body)
	if len(tokens) < 2 {
		return nil, false
	}
	if !strings.EqualFold(tokens[0], "flow") || !strings.EqualFold(tokens[1], "route") {
		return nil, false
	}

	tokens = tokens[2:]
	if len(tokens) >= 2 && tokens[0] != "{" && tokens[1] == "{" {
		tokens = tokens[1:]
	}
	return tokens, true
}

// flowRouteTokens splits a flow route body into words.
//
// A brace is its own word wherever it appears, so `{match{` reads as three, and
// a semicolon is dropped: it terminates a statement in the braced form and the
// keyword that follows already says where the previous value ended. A bracket
// stays attached to its word, because ExaBGP writes an IPv6 redirect target as
// `[2001:db8::1]:5678`, and flowRouteList reads the list shape off the attached
// form instead.
func flowRouteTokens(body string) []string {
	var tb textbuf.Buffer
	for i := range len(body) {
		switch body[i] {
		case '{', '}':
			tb.Byte(' ').Byte(body[i]).Byte(' ')
		case ';':
			tb.Byte(' ')
		default:
			tb.Byte(body[i])
		}
	}
	return strings.Fields(tb.String())
}

// flowRouteFamily answers whether a family names one of the two flowspec SAFIs.
// A family that names another one would produce a command that reads as a flow
// route and encodes as something else, so it is refused where it arrives.
func flowRouteFamily(family string) bool {
	afi, safi, ok := strings.Cut(family, "/")
	if !ok {
		return false
	}
	if afi != bridgeAFIv4 && afi != bridgeAFIv6 {
		return false
	}
	return safi == bridgeFlowSAFI || safi == bridgeFlowVPNSAFI
}

// flowRouteIPv6Family answers the IPv6 family holding the same SAFI. The caller
// has already checked the family with flowRouteFamily.
func flowRouteIPv6Family(family string) string {
	_, safi, _ := strings.Cut(family, "/")
	var tb textbuf.Buffer
	return tb.Str("ipv6/").Str(safi).String()
}

// read fills the route from the body's tokens.
//
// The braced and the flat forms reach the same keyword table, because ExaBGP
// declares one for both: `match`, `then` and `scope` group the keywords for a
// reader, and its flat `announce flow route` schema lists every one of them in
// a single namespace (src/exabgp/configuration/announce/flow.py, schema). So a
// block word here checks that a block opens and carries nothing else.
func (r *flowRoute) read(tokens []string) error {
	for i := 0; i < len(tokens); {
		word := strings.ToLower(tokens[i])

		if word == "{" || word == "}" {
			i++
			continue
		}
		if word == "match" || word == "then" || word == "scope" {
			if i+1 >= len(tokens) || tokens[i+1] != "{" {
				return fmt.Errorf("exabgp flow route: %q opens no block with '{'", tokens[i])
			}
			i++
			continue
		}

		next, err := r.readWord(tokens, i)
		if err != nil {
			return err
		}
		i = next
	}

	if len(r.nlri) == 0 {
		return errFlowRouteNoMatch
	}
	return nil
}

// readWord reads the keyword at index i with everything it consumes, and
// answers the index of the next keyword.
func (r *flowRoute) readWord(tokens []string, i int) (int, error) {
	word := strings.ToLower(tokens[i])

	switch word {
	case "rd", bridgeAttrRouteDist:
		value, next, ok := flowRouteScalar(tokens, i)
		if !ok {
			return 0, flowRouteMissingValue(tokens[i])
		}
		r.rd = value
		r.family = flowSpecVPNFamily(r.family)
		return next, nil

	case bridgeAttrNextHop:
		value, next, ok := flowRouteScalar(tokens, i)
		if !ok {
			return 0, flowRouteMissingValue(tokens[i])
		}
		r.nhop = value
		return next, nil

	case "accept":
		// ExaBGP's accept states the default and carries no extended community
		// at all (src/exabgp/configuration/flow/parser.py, accept), so there is
		// nothing for ze to write.
		return i + 1, nil

	case "discard":
		r.actions = append(r.actions, flowRouteDiscard)
		return i + 1, nil

	case "rate-limit":
		return r.readRateLimit(tokens, i)

	case "redirect":
		return r.readRedirect(tokens, i)

	case "redirect-to-nexthop":
		// With an address the IETF form, with none the Simpson one
		// (configuration/grammar/types/flow.py, _redirect_to_nexthop).
		if i+1 < len(tokens) {
			if addr, ok := flowRouteAddress(tokens[i+1]); ok {
				r.actions = append(r.actions, flowRouteRedirectToIP(flowRouteIETFRedirect, addr))
				return i + 2, nil
			}
		}
		r.actions = append(r.actions, flowRouteRedirectNextHop)
		return i + 1, nil

	case "redirect-to-nexthop-simpson":
		r.actions = append(r.actions, flowRouteRedirectNextHop)
		return i + 1, nil

	case "redirect-to-nexthop-ietf":
		return r.readIETFRedirect(flowRouteIETFRedirect, tokens, i)

	case "copy":
		return r.readIETFRedirect(flowRouteIETFCopy, tokens, i)

	case "redirect-simpson":
		return r.readSimpson(flowRouteRedirectNextHop, tokens, i)

	case "copy-simpson":
		return r.readSimpson(flowRouteCopyNextHop, tokens, i)

	case "interface-set":
		items, next, err := flowRouteList(tokens, i)
		if err != nil {
			return 0, err
		}
		var tb textbuf.Buffer
		for _, item := range items {
			if err := CheckInterfaceSet(item); err != nil {
				return 0, fmt.Errorf("exabgp flow route: %w", err)
			}
			r.extended = append(r.extended, tb.Reset().Str("interface-set:").Str(item).String())
		}
		return next, nil

	case "community", "large-community", "extended-community":
		return r.readCommunity(word, tokens, i)

	case "mark":
		return r.readMark(tokens, i)

	case "action":
		return r.readAction(tokens, i)
	}

	return r.readComponent(tokens, i)
}

// readRateLimit reads `rate-limit <rate> [bytes|packets]`.
//
// ExaBGP's unit word is optional and defaults to bytes
// (src/exabgp/configuration/flow/parser.py, rate_limit). ze spells the byte
// unit by leaving it out and the packet unit with a `:packets` suffix
// (route/route_community.go parseRateLimitExtCommunity), which is the same
// spelling the flat form already produces.
func (r *flowRoute) readRateLimit(tokens []string, i int) (int, error) {
	rate, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}

	var tb textbuf.Buffer
	tb.Str("rate-limit:").Str(rate)

	if next < len(tokens) {
		switch strings.ToLower(tokens[next]) {
		case "packets":
			tb.Str(":packets")
			next++
		case "bytes":
			next++
		}
	}

	r.actions = append(r.actions, tb.String())
	return next, nil
}

// readMark reads `mark <dscp>`, the traffic-marking action, which ze's update
// text spells `mark:<dscp>` (attribute.ParseExtendedCommunityKeyword).
//
// RFC 8955 Section 7.5: "The traffic marking Extended Community instructs a
// system to modify the DSCP bits in the IP header (Section 3 of [RFC2474]) of
// a transiting IP packet to the corresponding value encoded in the 6 least
// significant bits of the Extended Community value". ExaBGP takes
// the decimal digits alone (configuration/grammar/types/flow.py, _mark, and
// 5.0.1 configuration/flow/parser.py, mark), so a sign or a hex value is
// refused here where ze's integer parser would read it.
func (r *flowRoute) readMark(tokens []string, i int) (int, error) {
	value, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}
	if strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("exabgp flow route: mark %q is not a DSCP value: write it with the digits 0 to 9, from 0 to %d",
			value, attribute.FlowSpecDSCPMax)
	}
	var tb textbuf.Buffer
	r.actions = append(r.actions, tb.Str("mark:").Str(value).String())
	return next, nil
}

// readAction reads `action <words>`, the traffic-action action, which ze's
// update text spells `traffic-action:<sample|terminal|sample-terminal>`.
//
// RFC 8955 Section 7.3: "Terminal Action (bit 47): When this bit is set, the
// traffic filtering engine will evaluate any subsequent Flow Specifications
// (as defined by the ordering procedure Section 5.1)." and "Sample (bit 46):
// Enables traffic sampling and logging for this Flow Specification (only
// effective when set)." ExaBGP sets each bit when
// its word appears anywhere in the value, and refuses a value holding neither
// (configuration/grammar/types/flow.py, _action, and 5.0.1
// configuration/flow/parser.py, action), so the value is read that way rather
// than handed to ze's stricter word list.
func (r *flowRoute) readAction(tokens []string, i int) (int, error) {
	value, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}
	sample := strings.Contains(value, "sample")
	terminal := strings.Contains(value, "terminal")

	var tb textbuf.Buffer
	tb.Str("traffic-action:")
	switch {
	case sample && terminal:
		tb.Str("sample-terminal")
	case sample:
		tb.Str("sample")
	case terminal:
		tb.Str("terminal")
	default:
		return 0, fmt.Errorf("exabgp flow route: action %q is not a flow action: write sample, terminal or sample-terminal", value)
	}
	r.actions = append(r.actions, tb.String())
	return next, nil
}

// readRedirect reads `redirect <target>`.
//
// ExaBGP 6.0 reads one value three ways (configuration/grammar/types/flow.py,
// _redirect). An address, bare or in brackets, is the IETF redirect-to-IP
// community holding that address, and the next hop is left alone. Any other
// value is a route target, and ze spells that `redirect:<target>`. That covers
// both route-target forms: `<administrator>:<value>`, the 8-octet RFC 8955
// Section 7.4 redirect (route/route_community.go parseRedirectExtCommunity),
// and `[<IPv6>]:<nn>`, the 20-octet RFC 8956 Section 6.1 rt-redirect-ipv6 that
// ExaBGP builds as TrafficRedirectIPv6 (attribute.ParseFlowSpecRedirectIPv6).
// The bracketed form is ExaBGP's spelling and ze's alike, so it passes through
// unchanged, and ze refuses a malformed one with the reason.
func (r *flowRoute) readRedirect(tokens []string, i int) (int, error) {
	target, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}

	if addr, isAddr := flowRouteAddress(target); isAddr {
		r.actions = append(r.actions, flowRouteRedirectToIP(flowRouteIETFRedirect, addr))
		return next, nil
	}

	var tb textbuf.Buffer
	r.actions = append(r.actions, tb.Str("redirect:").Str(target).String())
	return next, nil
}

// readIETFRedirect reads `redirect-to-nexthop-ietf <ip>` or `copy <ip>`: the
// redirect-to-IP community of draft-ietf-idr-flowspec-redirect-ip-16 holding
// the address, written by ze's `word <ip>`. ExaBGP 6.0 leaves the next hop
// alone for both (configuration/grammar/types/flow.py, _copy and
// REDIRECT_TO_NEXTHOP_IETF).
func (r *flowRoute) readIETFRedirect(word string, tokens []string, i int) (int, error) {
	value, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}
	addr, isAddr := flowRouteAddress(value)
	if !isAddr {
		return 0, fmt.Errorf("exabgp flow route: %s takes an address and %q is not one", tokens[i], value)
	}
	r.actions = append(r.actions, flowRouteRedirectToIP(word, addr))
	return next, nil
}

// readSimpson reads `redirect-simpson <ip>` or `copy-simpson <ip>`: the
// address becomes the next hop and action is the 0x0800 community of
// draft-simpson-idr-flowspec-redirect-02, which points at it
// (configuration/grammar/types/flow.py, _redirect_simpson and COPY_SIMPSON).
func (r *flowRoute) readSimpson(action string, tokens []string, i int) (int, error) {
	value, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}
	addr, isAddr := flowRouteAddress(value)
	if !isAddr {
		return 0, fmt.Errorf("exabgp flow route: %s takes an address and %q is not one", tokens[i], value)
	}
	r.nhop = addr
	r.actions = append(r.actions, action)
	return next, nil
}

// flowRouteAddress answers the address a redirect value names, bare or in the
// brackets ExaBGP allows around one, and false for anything else.
func flowRouteAddress(value string) (string, bool) {
	if inner, ok := strings.CutPrefix(value, "["); ok {
		value, ok = strings.CutSuffix(inner, "]")
		if !ok {
			return "", false
		}
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", false
	}
	return addr.String(), true
}

// flowRouteRedirectToIP renders the two-word update-text action `<word> <addr>`.
func flowRouteRedirectToIP(word, addr string) string {
	var tb textbuf.Buffer
	return tb.Str(word).Str(" ").Str(addr).String()
}

// readCommunity reads one of the three community keywords and its value list.
//
// The extended communities are normalized to ze's spelling, which is what the
// flat form already does: ExaBGP writes a packet rate as
// `rate-limit-packets:N` and a byte rate as `rate-limit:N:bytes`, and ze writes
// `rate-limit:N:packets` and `rate-limit:N`. A bare `<asn>:<n>` is a route
// target to ExaBGP, as in a unicast route (qualifyExtCommunity).
func (r *flowRoute) readCommunity(word string, tokens []string, i int) (int, error) {
	items, next, err := flowRouteList(tokens, i)
	if err != nil {
		return 0, err
	}

	switch word {
	case "community":
		r.communities = append(r.communities, items...)
	case "large-community":
		r.largeCommunities = append(r.largeCommunities, items...)
	default:
		for _, item := range items {
			if err := CheckExtendedCommunityItem(item); err != nil {
				return 0, fmt.Errorf("exabgp flow route: %w", err)
			}
			r.extended = append(r.extended, normalizeFlowSpecExtCommunityToken(qualifyExtCommunity(item)))
		}
	}
	return next, nil
}

// readComponent reads one match component and its values.
//
// A bare `source` or `destination` names no family, so the prefix decides one,
// and it decides the whole route's family because a flow route carries one AFI.
// The check runs before the value is read so an unknown keyword is reported as
// one rather than as a keyword whose value is missing.
func (r *flowRoute) readComponent(tokens []string, i int) (int, error) {
	word := strings.ToLower(tokens[i])
	keywordType, ok := flowSpecComponentType(normalizeFlowSpecComponentToken(r.family, word))
	if !ok {
		return 0, fmt.Errorf("exabgp flow route: unknown keyword %q", tokens[i])
	}
	prefixWord := keywordType == flowspec.FlowDestPrefix || keywordType == flowspec.FlowSourcePrefix

	// A condition keyword that ends the line holds no {operator, value} pair,
	// and ExaBGP reads it as no component at all: its condition reader loops
	// over the words of the value and yields nothing for an empty one
	// (configuration/grammar/types/flow.py, _expression, and 5.0.1
	// configuration/flow/parser.py, _generic_condition), so the route goes out
	// without it. RFC 8955 Section 4.2.2.3: "Encoding: <type (1 octet),
	// [numeric_op, value]+>", so a component with no pair has nothing to
	// encode, and every condition type shares that shape. A prefix
	// keyword is not a condition, and ExaBGP refuses it with no value.
	if i+1 == len(tokens) && !prefixWord {
		return i + 1, nil
	}

	items, next, err := flowRouteList(tokens, i)
	if err != nil {
		return 0, err
	}

	// Every spelling of the two prefix keywords reaches one ExaBGP reader,
	// which takes the AFI from the prefix (5.0.1 configuration/announce/flow.py,
	// `destination-ipv6: destination`), so the prefix decides the family and
	// the qualified word is rebuilt from it.
	if prefixWord {
		word = strings.TrimSuffix(strings.TrimSuffix(word, "-ipv4"), "-ipv6")
		if strings.Contains(items[0], ":") {
			r.family = flowRouteIPv6Family(r.family)
		}
	}

	// The family can have moved since the keyword was checked above, and the
	// keyword follows it (source-ipv4 to source-ipv6), so it is resolved again.
	component := normalizeFlowSpecComponentToken(r.family, word)
	componentType, ok := flowSpecComponentType(component)
	if !ok {
		return 0, fmt.Errorf("exabgp flow route: unknown keyword %q", tokens[i])
	}
	r.types = append(r.types, componentType)
	r.nlri = append(r.nlri, component)
	for _, item := range items {
		r.nlri = append(r.nlri, normalizeFlowSpecComponentValue(component, item))
	}
	return next, nil
}

// command renders the route as the ze update-text command.
//
// An announcement and a withdrawal are the same command under a different NLRI
// verb, attributes included. ze matches a withdrawn FlowSpec rule on its
// components alone, so the attributes name no route; what they do is reach the
// wire, because RFC 4760 Section 4 lets a withdrawal carry a path attribute
// block and ExaBGP writes one.
func (r *flowRoute) command(selector, verb string) Command {
	parts := make([]string, 0, 6)

	var head textbuf.Buffer
	parts = append(parts, head.Str("send bgp ").Str(selector).Str(" update text").String())

	// Gating this on the verb dropped every `then` action, every community and the
	// next-hop from a withdrawal, so the translator lost what the script wrote:
	// api-broken-flow.ci withdraws `flow route { match { ... } then
	// { rate-limit 1; } }` and expects the rate-limit extended community on the
	// withdrawal (ai/rules/principles.md).
	parts = r.appendAttributes(parts)

	var nlri textbuf.Buffer
	// RFC 8955 Section 4: FlowSpec NLRI travels in MP_REACH_NLRI, ze's `mp` section.
	nlri.Str("mp ").Str(r.family).Byte(' ').Str(verb)
	if r.rd != "" {
		nlri.Str(" rd ").Str(r.rd)
	}
	for _, token := range r.nlri {
		nlri.Byte(' ').Str(token)
	}
	parts = append(parts, nlri.String())

	// The key is stated HERE, where the NLRI tokens are written, so the batch
	// netting never reads them back out of the finished command. A withdrawal
	// of this route writes the same rd and the same match components, so the
	// two carry one key whatever the `then` block held (bridge_batch.go, Net).
	command := Command{Text: textbuf.Join(parts, " "), Key: r.key(verb)}
	if verb == flowVerbAdd {
		command.Withdrawal = r.command(selector, flowVerbDel).Text
	}
	return command
}

// key names the route this flow command carries.
//
// The route distinguisher joins the match components, because a VPN flow route
// and a global one that match the same traffic are two routes (RFC 8955
// Section 4.3 and RFC 8956 Section 3.1: the RD is part of the NLRI).
func (r *flowRoute) key(verb string) RouteKey {
	tokens := make([]string, 0, len(r.nlri)+2)
	if r.rd != "" {
		tokens = append(tokens, "rd", r.rd)
	}
	tokens = append(tokens, r.nlri...)
	return RouteKey{
		Family:   r.family,
		NLRI:     textbuf.Join(tokens, " "),
		Withdraw: verb == flowVerbDel,
	}
}

// appendAttributes writes the path attributes the route carries, on an
// announcement and on a withdrawal alike.
func (r *flowRoute) appendAttributes(parts []string) []string {
	if r.nhop != "" {
		var tb textbuf.Buffer
		parts = append(parts, tb.Str("nhop ").Str(r.nhop).String())
	}
	if len(r.communities) != 0 {
		parts = append(parts, flowRouteAttrList("community", r.communities))
	}
	if len(r.largeCommunities) != 0 {
		parts = append(parts, flowRouteAttrList("large-community", r.largeCommunities))
	}

	extended := make([]string, 0, len(r.extended)+len(r.actions))
	extended = append(extended, r.extended...)
	extended = append(extended, r.actions...)
	if len(extended) != 0 {
		parts = append(parts, flowRouteAttrList("extended-community", extended))
	}
	return parts
}

// flowRouteAttrList renders one community attribute and its values.
//
// The bracketed form is written for one value as well as for several. ze reads
// both (attribute.ParseBracketedList), and one shape means a reader of the
// output never has to ask which case produced it.
func flowRouteAttrList(keyword string, items []string) string {
	var tb textbuf.Buffer
	tb.Str(keyword).Str(" [")
	for i, item := range items {
		if i != 0 {
			tb.Byte(' ')
		}
		tb.Str(item)
	}
	return tb.Byte(']').String()
}

// flowRouteScalar reads the single-word value that follows the keyword at index
// i, and answers the index after it.
//
// A brace is never a value: it ends the block the keyword sits in, so a keyword
// written with no value at the end of a block is reported as missing one rather
// than sent on with a brace for a value.
func flowRouteScalar(tokens []string, i int) (string, int, bool) {
	if i+1 >= len(tokens) {
		return "", 0, false
	}
	value := tokens[i+1]
	if value == "{" || value == "}" {
		return "", 0, false
	}
	return value, i + 2, true
}

// flowRouteList reads the value list that follows the keyword at index i, and
// answers the index after it. A keyword whose list is absent or empty is an
// error naming the keyword, so a caller never has to read an empty slice as
// either "no values" or "a value it could not parse".
//
// ExaBGP writes several values inside brackets and one value bare
// (src/exabgp/configuration/flow/match.py, definition). The brackets are
// stripped, because ze reads a match component's values as bare words
// (nlri/flowspec/plugin_encode_text.go parseNumericComponentText) and
// flowRouteAttrList puts them back for the community attributes that want them.
func flowRouteList(tokens []string, i int) ([]string, int, error) {
	first, _, ok := flowRouteScalar(tokens, i)
	if !ok {
		return nil, 0, flowRouteMissingValue(tokens[i])
	}
	if !strings.HasPrefix(first, "[") {
		return []string{first}, i + 2, nil
	}

	items := make([]string, 0, 4)
	for at := i + 1; at < len(tokens); at++ {
		token := tokens[at]
		closed := strings.HasSuffix(token, "]")

		token = strings.TrimPrefix(token, "[")
		token = strings.TrimSuffix(token, "]")
		if token != "" {
			items = append(items, token)
		}
		if !closed {
			continue
		}
		if len(items) == 0 {
			return nil, 0, flowRouteMissingValue(tokens[i])
		}
		return items, at + 1, nil
	}
	return nil, 0, errFlowRouteUnterminated
}

// flowRouteMissingValue names the keyword whose value is absent.
func flowRouteMissingValue(keyword string) error {
	return fmt.Errorf("exabgp flow route: %q states no value", keyword)
}

// CheckInterfaceSet refuses an interface-set value ExaBGP refuses and Ze's own
// parser would accept, or would refuse for another fault than the one ExaBGP names.
//
// ExaBGP's _one_interface_set (configuration/grammar/types/flow.py) judges, in
// this order, the transitivity of a four-field value, then an AS field holding
// a period ("'<asn>' is not a valid ASN, it is a 32-bit integer"), then the
// direction. attribute.parseInterfaceSet keeps every RFC 5396 spelling for
// Ze-native text and judges the direction before the AS number, so the
// transitivity and the period are checked here first: a value wrong in more
// than one field is then refused for the fault ExaBGP names. ExaBGP's config
// and its API commands share that grammar, so an ExaBGP script and a config
// migrated from ExaBGP are refused the same way.
//
// The value is `[transitive|non-transitive:]<direction>:<asn>:<group-id>`. A
// nil answer means only that these two checks do not refuse the value: the
// caller MUST still hand it to the interface-set parser, which judges the
// direction and every other fault, including a field count that names no AS
// number at all.
func CheckInterfaceSet(value string) error {
	fields := strings.Split(value, ":")
	var asText string
	switch len(fields) {
	case 4:
		if fields[0] != "transitive" && fields[0] != "non-transitive" {
			return fmt.Errorf("interface-set %q: %q is neither transitive nor non-transitive; write transitive or non-transitive",
				value, fields[0])
		}
		asText = fields[2]
	case 3:
		asText = fields[1]
	default:
		return nil
	}
	if !strings.Contains(asText, ".") {
		return nil
	}

	number, err := asn.Parse(asText)
	if err != nil {
		return fmt.Errorf("interface-set %q: the AS number %q holds a period, which ExaBGP refuses in an interface-set; write the AS number as one integer, 0 to 4294967295",
			value, asText)
	}
	return fmt.Errorf("interface-set %q: the AS number %q is in asdot form, which ExaBGP refuses in an interface-set; write the 32-bit integer %d",
		value, asText, number)
}

// CheckExtendedCommunityItem refuses an `interface-set:` item written inside an
// extended-community list.
//
// ExaBGP's extended_community (configuration/grammar/types/bgp.py) reads the
// word before the first colon as the type, and interface-set is not one of its
// types, so it raises "invalid extended community: <item>, expecting 4 dotted
// decimal parts". Its flow `then` block reads extended-community through the
// same type (configuration/grammar/tree/flow.py). An interface-set is written
// with its own keyword, and Ze's extended community parser accepts the item in
// the list, so the rule lives in the ExaBGP layer, for the bridge and the
// migration alike.
func CheckExtendedCommunityItem(item string) error {
	const prefix = "interface-set:"
	if len(item) < len(prefix) {
		return nil
	}
	if !strings.EqualFold(item[:len(prefix)], prefix) {
		return nil
	}
	return fmt.Errorf("extended-community %q: ExaBGP refuses an interface-set inside extended-community; write it with the interface-set keyword of a flow route: interface-set [ %s ]",
		item, item[len(prefix):])
}
