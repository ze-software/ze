// Design: docs/architecture/exabgp-bridge.md — ExaBGP `flow route` to the ze update-text command
// Overview: bridge_command.go — TranslateLine, the flat family forms, and the ze flowspec
// spellings this file reuses (normalizeFlowSpecComponentToken, isFlowSpecComponentKeyword,
// normalizeFlowSpecComponentValue, normalizeFlowSpecExtCommunityToken, flowSpecVPNFamily)
// Related: bridge_event.go — the JSON direction, which renders the same actions back to ExaBGP

package bridge

import (
	"errors"
	"fmt"
	"strings"

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

	// flowRouteRedirectNextHop is ze's spelling of the pre-IETF
	// redirect-to-next-hop action, type 0x08 subtype 0x00 value 0. ExaBGP
	// writes the same eight octets from `redirect-to-nexthop` and from a
	// `redirect` whose value is a bare address
	// (src/exabgp/configuration/flow/parser.py, redirect_next_hop and redirect).
	flowRouteRedirectNextHop = "redirect-to-nexthop-draft"

	// flowRouteCopyNextHop is ze's spelling of ExaBGP's `copy <address>`
	// action: the same type and subtype as the line above with the low value
	// bit set, which is the copy semantic.
	flowRouteCopyNextHop = "copy-to-nexthop"
)

var (
	errFlowRouteNoMatch      = errors.New("exabgp flow route: the route states no match component")
	errFlowRouteUnterminated = errors.New("exabgp flow route: a value list opens with '[' and never closes with ']'")
)

// ConvertFlowRoute translates one ExaBGP `flow route ...` body, in either the
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
func ConvertFlowRoute(selector, family, verb, body string) (string, bool, error) {
	tokens, ok := flowRouteBody(body)
	if !ok {
		return "", false, nil
	}

	if verb != flowVerbAdd && verb != flowVerbDel {
		return "", true, fmt.Errorf("exabgp flow route: verb %q is neither %q nor %q", verb, flowVerbAdd, flowVerbDel)
	}
	if !flowRouteFamily(family) {
		return "", true, fmt.Errorf("exabgp flow route: %q is not a flowspec family", family)
	}

	route := flowRoute{family: family}
	if err := route.read(tokens); err != nil {
		return "", true, err
	}
	return route.command(selector, verb), true, nil
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
		r.actions = append(r.actions, flowRouteRedirectNextHop)
		return i + 1, nil

	case "copy":
		value, next, ok := flowRouteScalar(tokens, i)
		if !ok {
			return 0, flowRouteMissingValue(tokens[i])
		}
		r.nhop = value
		r.actions = append(r.actions, flowRouteCopyNextHop)
		return next, nil

	case "interface-set":
		items, next, err := flowRouteList(tokens, i)
		if err != nil {
			return 0, err
		}
		var tb textbuf.Buffer
		for _, item := range items {
			r.extended = append(r.extended, tb.Reset().Str("interface-set:").Str(item).String())
		}
		return next, nil

	case "community", "large-community", "extended-community":
		return r.readCommunity(word, tokens, i)

	case "mark", "action", "redirect-to-nexthop-ietf":
		// Each of these three has an encoder in ze
		// (attribute.FlowSpecTrafficMarking, attribute.FlowSpecTrafficAction,
		// attribute.FlowSpecRedirectToIPv4) and no name in the extended
		// community vocabulary `update text` reads
		// (route/route_community.go parseExtendedCommunity), so there is no
		// command to write. Naming the word is what tells the operator which
		// action was refused.
		return 0, fmt.Errorf("exabgp flow route: %q has no ze update-text spelling", tokens[i])
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

// readRedirect reads `redirect <target>`.
//
// ExaBGP reads one value two ways. A value carrying no colon is an address, and
// it sets the next-hop and adds the redirect-to-next-hop action. Any other value
// is a route target, and ze spells that `redirect:<administrator>:<value>`
// (src/exabgp/configuration/flow/parser.py, redirect;
// route/route_community.go parseRedirectExtCommunity).
func (r *flowRoute) readRedirect(tokens []string, i int) (int, error) {
	target, next, ok := flowRouteScalar(tokens, i)
	if !ok {
		return 0, flowRouteMissingValue(tokens[i])
	}

	if !strings.Contains(target, ":") {
		r.nhop = target
		r.actions = append(r.actions, flowRouteRedirectNextHop)
		return next, nil
	}

	var tb textbuf.Buffer
	r.actions = append(r.actions, tb.Str("redirect:").Str(target).String())
	return next, nil
}

// readCommunity reads one of the three community keywords and its value list.
//
// The extended communities are normalized to ze's spelling, which is what the
// flat form already does: ExaBGP writes a packet rate as
// `rate-limit-packets:N` and a byte rate as `rate-limit:N:bytes`, and ze writes
// `rate-limit:N:packets` and `rate-limit:N`.
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
			r.extended = append(r.extended, normalizeFlowSpecExtCommunityToken(item))
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
	if !isFlowSpecComponentKeyword(normalizeFlowSpecComponentToken(r.family, word)) {
		return 0, fmt.Errorf("exabgp flow route: unknown keyword %q", tokens[i])
	}

	items, next, err := flowRouteList(tokens, i)
	if err != nil {
		return 0, err
	}

	if word == "source" || word == "destination" {
		if strings.Contains(items[0], ":") {
			r.family = flowRouteIPv6Family(r.family)
		}
	}

	component := normalizeFlowSpecComponentToken(r.family, word)
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
func (r *flowRoute) command(selector, verb string) string {
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
	nlri.Str("nlri ").Str(r.family).Byte(' ').Str(verb)
	if r.rd != "" {
		nlri.Str(" rd ").Str(r.rd)
	}
	for _, token := range r.nlri {
		nlri.Byte(' ').Str(token)
	}
	parts = append(parts, nlri.String())

	return textbuf.Join(parts, " ")
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
