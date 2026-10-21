// Design: docs/architecture/wire/attributes.md -- FlowSpec traffic-action extended communities
// RFC: rfc/short/rfc8955.md -- traffic filtering actions (Section 7)
// Related: community.go -- ExtendedCommunity, the 8-octet value these produce
// Related: extcomm_decoded.go -- the decode half, rendering those values back to their names

package attribute

import (
	"fmt"
	"net/netip"
)

// FlowSpec traffic actions that are written as ONE word, with no colon.
//
// Every other extended community carries its value after a colon
// (`target:65000:1`, `rate-limit:1000`), so a parser can find the type by
// splitting on the first one. These three carry their whole meaning in the name,
// and a parser that splits first rejects them as malformed rather than as
// unknown -- which is exactly what happened: the config path understood
// `copy-to-nexthop` while the `update text` API path answered "invalid extended
// community format", so a route an operator could write in config could not be
// expressed through the API.
//
// One table, consumed by both parsers, so the two vocabularies cannot drift
// again (config/routeattr_community.go parseOneExtCommunity,
// route/route_community.go parseExtendedCommunity).
var flowSpecActionKeywords = map[string]ExtendedCommunity{
	// Pre-IETF draft: redirect to next-hop. Type 0x08, subtype 0x00, value 0.
	"redirect-to-nexthop-draft": {0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	// RFC 5575bis: copy and redirect to next-hop. Same type/subtype, value 1 --
	// the low bit is the copy semantic, which is why these two differ by one byte.
	"copy-to-nexthop": {0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
	// RFC 8955 Section 7.1: traffic-rate 0 means discard. Type 0x80, subtype 0x06.
	// Section 7.3 is traffic-action, which this keyword does not encode.
	"discard": {0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
}

// FlowSpecActionKeyword returns the extended community for a colon-less FlowSpec
// action keyword, and whether the name is one.
//
// Callers must try this BEFORE splitting on ':', because these names contain no
// colon and a split-first parser reports them as a malformed format rather than
// as an unrecognized action.
func FlowSpecActionKeyword(name string) (ExtendedCommunity, bool) {
	ec, ok := flowSpecActionKeywords[name]
	return ec, ok
}

// FlowSpecActionKeywords returns the accepted keywords, sorted, for an error
// message that tells the operator what IS accepted rather than only that their
// input was not (ai/rules/cli.md). Derived from the table, so a new
// keyword cannot be added without the diagnostic naming it
// (ai/rules/evidence.md).
func FlowSpecActionKeywords() []string {
	names := make([]string, 0, len(flowSpecActionKeywords))
	for name := range flowSpecActionKeywords {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// sortStrings is an insertion sort over a 3-element table: pulling in "sort"
// here would be the only import in this file, for a slice this size.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// flowSpecRedirectNextHopType and flowSpecRedirectNextHopSubtype name the
// redirect/mirror-to-IP-next-hop community the `redirect-to-nexthop-draft` and
// `copy-to-nexthop` keywords above write.
const (
	flowSpecRedirectNextHopType    = 0x08
	flowSpecRedirectNextHopSubtype = 0x00

	// flowSpecRTRedirectSubtype is the sub-type of the RFC 8955 Section 7.4
	// rt-redirect action under types 0x80, 0x81 and 0x82.
	flowSpecRTRedirectSubtype = 0x08
)

// FlowSpecRedirectsToNextHop reports whether an EXTENDED_COMMUNITIES attribute
// value carries the redirect/mirror-to-IP-next-hop action, whose target is the
// MP_REACH_NLRI next hop rather than an address inside the community.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: matching packets go
// "towards the IPv4 or IPv6 address specified in the 'Network Address of
// Next-Hop' field of the associated MP_REACH_NLRI", and "If an MP_REACH_NLRI
// containing one or more flow-spec NLRI does not have a valid IPv4 or IPv6
// address in its next-hop field, or the length of the next-hop is 0, then the
// redirect-to-IP extended community, if present, should be ignored." So a
// FlowSpec route carrying it is the one case where the RFC 8955 Section 4 zero
// next-hop length would void the route's own action.
//
// The same paragraph voids the action beside a redirect-to-VRF: "The
// redirect-to-IP extended community is valid with any other set of flow-spec
// extended communities except if that set includes a redirect-to-VRF extended
// community (type 0x8008) and in that case the redirect-to-IP extended
// community should be ignored." RFC 8955 Section 7.4 registers that action
// under three types, 0x8008, 0x8108 and 0x8208 (its Table 2), so any of them
// voids it.
//
// The value is read in whole 8-octet communities; a trailing partial one is
// not a community and is not read.
func FlowSpecRedirectsToNextHop(value []byte) bool {
	found := false
	for off := 0; off+8 <= len(value); off += 8 {
		ec := ExtendedCommunity(value[off : off+8])
		if ec.redirectsToVRF() {
			return false
		}
		if ec.redirectsToNextHop() {
			found = true
		}
	}
	return found
}

// RedirectsToNextHop is FlowSpecRedirectsToNextHop over decoded communities.
func (e ExtendedCommunities) RedirectsToNextHop() bool {
	found := false
	for _, ec := range e {
		if ec.redirectsToVRF() {
			return false
		}
		if ec.redirectsToNextHop() {
			found = true
		}
	}
	return found
}

// redirectsToVRF reports whether this one community is an RFC 8955 Section 7.4
// rt-redirect, the redirect-to-VRF action of
// draft-simpson-idr-flowspec-redirect-02 Section 3.
func (e ExtendedCommunity) redirectsToVRF() bool {
	if e[1] != flowSpecRTRedirectSubtype {
		return false
	}
	return e[0] == 0x80 || e[0] == 0x81 || e[0] == 0x82
}

// redirectsToNextHop reports whether this one community is the
// redirect/mirror-to-IP-next-hop action of draft-simpson-idr-flowspec-redirect-02
// Section 3, "type value 0x0800"; the copy bit does not change the answer.
func (e ExtendedCommunity) redirectsToNextHop() bool {
	if e[0] != flowSpecRedirectNextHopType {
		return false
	}
	return e[1] == flowSpecRedirectNextHopSubtype
}

// flowSpecRedirectToIPKeywords are the words that take an address and write the
// redirect-to-IP community of draft-ietf-idr-flowspec-redirect-ip-16, mapped to
// the copy bit each sets. Section 2: "The redirect-to-IP extended community
// also contains a flag called 'C' (copy)."
//
// One table, read by both parsers through ParseFlowSpecRedirectToIP
// (config/routeattr_community.go ParseExtendedCommunity,
// route/route_community.go ParseExtendedCommunities). Only the config knew
// these words once, so `update text` could not send what the config could.
var flowSpecRedirectToIPKeywords = map[string]bool{
	"redirect-to-nexthop": false,
	"copy-to-nexthop":     true,
}

// FlowSpecRedirectToIP is one redirect-to-IP action: the redirection target and
// whether the copy bit is set. Only ParseFlowSpecRedirectToIP builds one, so
// Addr is always a valid address.
type FlowSpecRedirectToIP struct {
	Addr netip.Addr
	Copy bool
}

// ParseFlowSpecRedirectToIP reads the two-word action `<word> <next>`, where
// word is `redirect-to-nexthop` or `copy-to-nexthop` and next is the target
// address. next is "" when word is the last token.
//
// matched is false when word is not one of the two, and when next is not an
// address and word also has a one-word meaning in flowSpecActionKeywords: bare
// `copy-to-nexthop` is the 0x0800 copy action of
// draft-simpson-idr-flowspec-redirect-02, and the caller reads it as that one
// word. `redirect-to-nexthop` has no one-word meaning, so without an address it
// is matched and refused, naming what it needs.
func ParseFlowSpecRedirectToIP(word, next string) (FlowSpecRedirectToIP, bool, error) {
	copyTraffic, ok := flowSpecRedirectToIPKeywords[word]
	if !ok {
		return FlowSpecRedirectToIP{}, false, nil
	}

	addr, err := netip.ParseAddr(next)
	if err == nil {
		return FlowSpecRedirectToIP{Addr: addr.Unmap(), Copy: copyTraffic}, true, nil
	}
	if _, oneWord := flowSpecActionKeywords[word]; oneWord {
		return FlowSpecRedirectToIP{}, false, nil
	}
	if next == "" {
		return FlowSpecRedirectToIP{}, true, fmt.Errorf(
			"%s needs the redirection target address after it: write %s <ip>", word, word)
	}
	return FlowSpecRedirectToIP{}, true, fmt.Errorf(
		"%s needs the redirection target address after it, and %q is not an address: write %s <ip>",
		word, next, word)
}

// AppendTo appends the action's community to the list its address family
// needs and returns both lists: an IPv4 target is the 8-octet community of
// EXTENDED_COMMUNITIES (attribute 16), an IPv6 target the 20-octet community of
// IPV6_EXTENDED_COMMUNITIES (attribute 25, RFC 5701 Section 2), so exactly one
// list grows.
func (r FlowSpecRedirectToIP) AppendTo(ec []ExtendedCommunity, ec6 []IPv6ExtendedCommunity) ([]ExtendedCommunity, []IPv6ExtendedCommunity, error) {
	if r.Addr.Is4() {
		// draft-ietf-idr-flowspec-redirect-ip-16 Section 2
		one, err := FlowSpecRedirectToIPv4(r.Addr, r.Copy)
		if err != nil {
			return ec, ec6, err
		}
		return append(ec, one), ec6, nil
	}
	// draft-ietf-idr-flowspec-redirect-ip-16 Section 2
	one, err := FlowSpecRedirectToIPv6(r.Addr, r.Copy)
	if err != nil {
		return ec, ec6, err
	}
	return ec, append(ec6, one), nil
}
