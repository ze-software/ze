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
	"net/netip"
	"sort"
	"strconv"
	"strings"

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
	// arityGroup is a keyword followed by a parenthesised group, `( ... )`.
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
	// consuming it and writing nothing is the whole translation. `name` is the
	// only one: ExaBGP labels the route for its own `show` output.
	WireSilent bool
}

var bridgeRouteAttrs = map[string]bridgeRouteAttr{
	"next-hop":            {Ze: "nhop", Arity: arityValue},
	"origin":              {Ze: "origin", Arity: arityValue},
	"med":                 {Ze: "med", Arity: arityValue},
	"local-preference":    {Ze: "local-preference", Arity: arityValue},
	"as-path":             {Ze: "as-path", Arity: arityList},
	"community":           {Ze: "community", Arity: arityList},
	"large-community":     {Ze: "large-community", Arity: arityList},
	"extended-community":  {Ze: "extended-community", Arity: arityList},
	"atomic-aggregate":    {Ze: "atomic-aggregate", Arity: arityFlag},
	"aggregator":          {Ze: "aggregator", Arity: arityValue},
	"originator-id":       {Ze: "originator-id", Arity: arityValue},
	"cluster-list":        {Ze: "cluster-list", Arity: arityList},
	"aigp":                {Ze: "aigp", Arity: arityValue},
	"label":               {Ze: "label", Arity: arityList},
	"rd":                  {Ze: "rd", Arity: arityValue},
	"route-distinguisher": {Ze: "rd", Arity: arityValue},
	"path-information":    {Ze: "path-information", Arity: arityValue},
	"watchdog":            {Ze: "watchdog", Arity: arityValue},
	"name":                {Arity: arityValue, WireSilent: true},

	// `split` is not an attribute: it re-cuts the prefix into smaller ones, so
	// the parser carries its length out to the caller that owns the NLRI
	// (routeAttributes.Split) rather than writing a keyword.
	"split": {Arity: arityValue},

	// ze has no update-text spelling for these two, so a line carrying one is
	// REFUSED rather than announced without it. `attribute` is ExaBGP's generic
	// `attribute [ 0xFF 0x01 0x00 ]` escape hatch, which ze offers only through
	// `update hex`; `bgp-prefix-sid-srv6` carries the SRv6 L3 service TLV of
	// RFC 9252.
	"attribute":           {Arity: arityList},
	"bgp-prefix-sid-srv6": {Arity: arityGroup},
}

// bridgeAttrNames lists the table's keywords, longest first, so a refusal can
// say what the bridge does read. Sorted so the message is stable.
func bridgeAttrNames() []string {
	names := make([]string, 0, len(bridgeRouteAttrs))
	for name := range bridgeRouteAttrs {
		names = append(names, name)
	}
	sort.Strings(names)
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
	// Withdraw is set by ExaBGP's `withdraw` flag, which announces the route in
	// its withdrawn state rather than adding it.
	Withdraw bool
	// Split is the prefix length ExaBGP's `split /<n>` cuts the route into, or
	// zero when the line carries none.
	Split int
	// written records each ze keyword the line carried, so a caller can ask
	// what was present without re-reading the token stream it produced.
	written map[string]bool
}

// Command renders the attributes as the ze text that precedes the nlri section.
func (a routeAttributes) Command() string { return textbuf.Join(a.Tokens, " ") }

// Has reports whether the ze keyword was written. It reads the ze spelling
// rather than the ExaBGP one, so `rd` answers for `route-distinguisher` too.
func (a routeAttributes) Has(zeKeyword string) bool { return a.written[zeKeyword] }

// parseRouteAttributes reads the attribute tail of one ExaBGP route line: every
// token after the NLRI that the line begins with.
//
// It REFUSES an attribute the table does not hold and one ze cannot encode,
// naming the keyword in both cases. It never skips a token: a route announced
// without an attribute the operator wrote is a different route, and the script
// that wrote it is acknowledged either way.
func parseRouteAttributes(parts []string) (routeAttributes, error) {
	out := routeAttributes{written: make(map[string]bool, len(parts)/2)}
	for i := 0; i < len(parts); {
		key := strings.ToLower(parts[i])

		// ExaBGP's `withdraw` flag turns an announce into a withdrawal of the
		// same route, so it is read here rather than encoded.
		if key == "withdraw" {
			out.Withdraw = true
			i++
			continue
		}

		attr, known := bridgeRouteAttrs[key]
		if !known {
			return routeAttributes{}, fmt.Errorf("%w: %q; the bridge reads %s",
				ErrLineNotTranslated, parts[i], textbuf.Join(bridgeAttrNames(), ", "))
		}

		value, next, err := takeAttrValue(attr.Arity, parts, i+1)
		if err != nil {
			return routeAttributes{}, fmt.Errorf("%s: %w", key, err)
		}
		i = next

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

		// ExaBGP writes an ADD-PATH path identifier as an IPv4 address as well
		// as a number, and its own qa/api corpus uses the dotted form. ze reads
		// a uint32, so the dotted form is converted rather than passed on to
		// fail at a parser that would name the wrong thing.
		if attr.Ze == "path-information" {
			converted, err := pathIDNumber(value)
			if err != nil {
				return routeAttributes{}, err
			}
			value = converted
		}

		out.written[attr.Ze] = true
		out.Tokens = append(out.Tokens, attr.Ze)
		if attr.Arity != arityFlag {
			out.Tokens = append(out.Tokens, value)
		}
	}
	return out, nil
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
	for i, token := range parts {
		key := strings.ToLower(token)
		if _, known := bridgeRouteAttrs[key]; known || key == "withdraw" {
			return parts[:i], parts[i:]
		}
	}
	return parts, nil
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
	}
	return "", start, errMissingAttrValue
}

var (
	errMissingAttrValue = errors.New("missing value")
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
