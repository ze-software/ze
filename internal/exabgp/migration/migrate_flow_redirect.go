// Design: docs/architecture/config/exabgp-syntax.md -- ExaBGP flow route redirect and copy
// Overview: migrate_routes.go -- flow route conversion to Ze update blocks

package migration

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/family"
)

// The ze extended-community words a redirect or copy migrates to
// (internal/component/bgp/config routeattr_community.go). The IETF words take
// an address and send draft-ietf-idr-flowspec-redirect-ip-16's redirect-to-IP
// community; the bare words send draft-simpson-idr-flowspec-redirect-02's
// 0x0800 community, whose target is the route's next hop.
const (
	zeRedirectIETF    = "redirect-to-nexthop "
	zeCopyIETF        = "copy-to-nexthop "
	zeRedirectSimpson = "redirect-to-nexthop-draft"
	zeCopySimpson     = "copy-to-nexthop"
)

// The ExaBGP 6.0 flow then-block keywords that redirect or copy traffic.
const (
	exaRedirect                 = "redirect"
	exaRedirectSimpson          = "redirect-simpson"
	exaCopy                     = "copy"
	exaCopySimpson              = "copy-simpson"
	exaRedirectToNextHop        = "redirect-to-nexthop"
	exaRedirectToNextHopIETF    = "redirect-to-nexthop-ietf"
	exaRedirectToNextHopSimpson = "redirect-to-nexthop-simpson"
)

// flowRedirect is what one ExaBGP redirect or copy action becomes in ze: the
// extended community, and for the Simpson forms the next hop the community
// points at.
type flowRedirect struct {
	extCommunity string
	nextHop      string // empty for every form but the Simpson ones
}

// flowRedirectAction converts one redirect or copy action to its ze form, per
// the ExaBGP 6.0 grammar (exabgp/configuration/grammar/types/flow.py, the
// REDIRECT, REDIRECT_SIMPSON, COPY, COPY_SIMPSON and REDIRECT_TO_NEXTHOP*
// operations). ExaBGP 6.0 made `redirect <ip>` and `copy <ip>` the IETF form,
// which carries the address in the community, and kept the older Simpson form
// under the `-simpson` spellings. A config written for 4.x or 5.x meant the
// Simpson form by `redirect <ip>`; ExaBGP 6.0 reads it as IETF too, and so does
// this migration.
//
// A keyword that takes an address and is given something else is refused:
// guessing would announce a next hop or a community the operator never wrote.
func flowRedirectAction(action, value string) (flowRedirect, error) {
	address, isAddress := flowRedirectAddress(value)

	switch action {
	case exaRedirect:
		// redirect <asn>:<nn> (and [<ipv6>]:<nn>) is a redirect to a VRF.
		if !isAddress {
			return flowRedirect{extCommunity: "redirect:" + value}, nil
		}
		return flowRedirect{extCommunity: zeRedirectIETF + address.String()}, nil
	case exaRedirectToNextHop:
		// Bare, the Simpson form; with an address, the IETF one.
		if value == "" {
			return flowRedirect{extCommunity: zeRedirectSimpson}, nil
		}
		if !isAddress {
			return flowRedirect{}, flowRedirectNeedsAddress(action, value, "redirect-to-nexthop-ietf <ip>")
		}
		return flowRedirect{extCommunity: zeRedirectIETF + address.String()}, nil
	case exaRedirectToNextHopIETF:
		if !isAddress {
			return flowRedirect{}, flowRedirectNeedsAddress(action, value, "redirect-to-nexthop-ietf <ip>")
		}
		return flowRedirect{extCommunity: zeRedirectIETF + address.String()}, nil
	case exaRedirectToNextHopSimpson:
		if value != "" {
			return flowRedirect{}, fmt.Errorf(
				"%q takes no value and was given %q: the Simpson redirect sends to the route's next hop, "+
					"so write \"redirect-simpson %s\" to set it", action, value, value)
		}
		return flowRedirect{extCommunity: zeRedirectSimpson}, nil
	case exaRedirectSimpson:
		if !isAddress {
			return flowRedirect{}, flowRedirectNeedsAddress(action, value, "redirect "+value)
		}
		return flowRedirect{extCommunity: zeRedirectSimpson, nextHop: address.String()}, nil
	case exaCopy:
		if !isAddress {
			return flowRedirect{}, flowRedirectNeedsAddress(action, value, "copy <ip>")
		}
		return flowRedirect{extCommunity: zeCopyIETF + address.String()}, nil
	case exaCopySimpson:
		if !isAddress {
			return flowRedirect{}, flowRedirectNeedsAddress(action, value, "copy-simpson <ip>")
		}
		return flowRedirect{extCommunity: zeCopySimpson, nextHop: address.String()}, nil
	}
	return flowRedirect{}, fmt.Errorf("BUG: %q is not a redirect or copy action (convertFlowToUpdate)", action)
}

// flowRedirectAddress reads the target of a redirect as an address. ExaBGP
// accepts an IPv6 address bare or in brackets.
func flowRedirectAddress(value string) (netip.Addr, bool) {
	inner := strings.TrimSpace(value)
	if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
		inner = strings.TrimSpace(inner[1 : len(inner)-1])
	}
	address, err := netip.ParseAddr(inner)
	if err != nil {
		return netip.Addr{}, false
	}
	return address, true
}

// flowRedirectNeedsAddress is the refusal for a keyword given a value that is
// not an address.
func flowRedirectNeedsAddress(action, value, instead string) error {
	return fmt.Errorf(
		"%q needs an IPv4 or IPv6 address and %q is not one: the address is where the traffic is "+
			"redirected or copied to. Write \"%s\" in the ExaBGP config and migrate again", action, value, instead)
}

// zeFlowRedirectNextHopLeaf is the peer session leaf that enables the
// draft-simpson-idr-flowspec-redirect-02 Section 3 next hop on 0x0800 FlowSpec
// routes. RFC 8955 Section 4 keeps it false by default.
const zeFlowRedirectNextHopLeaf = "flowspec-redirect-next-hop"

// peerNeedsFlowSpecRedirectNextHop reports whether a migrated ExaBGP peer may
// originate draft-simpson 0x0800 FlowSpec routes carrying an MP_REACH next hop:
// either a static migrated update carries a Simpson community, or an attached
// API process serves a peer negotiating a FlowSpec family (because the process
// script can announce `redirect-simpson` / `copy-simpson` dynamically).
func peerNeedsFlowSpecRedirectNextHop(peer *config.Tree, hasAPIProcess bool) bool {
	if peerHasFlowSpecSimpsonRoute(peer) {
		return true
	}
	if !hasAPIProcess {
		return false
	}
	return peerNegotiatesFlowSpec(peer)
}

// peerHasFlowSpecSimpsonRoute reports whether any migrated update block on peer
// carries the draft-simpson 0x0800 redirect-to-IP-next-hop community
// (`redirect-to-nexthop-draft` or a bare `copy-to-nexthop` with no IP operand).
func peerHasFlowSpecSimpsonRoute(peer *config.Tree) bool {
	for _, entry := range peer.GetListOrdered("update") {
		attr := entry.Value.GetContainer("attribute")
		if attr == nil {
			continue
		}
		raw, ok := attr.Get(routeAttributeExtendedCommunity)
		if !ok {
			continue
		}
		if extCommunityHasSimpson(raw) {
			return true
		}
	}
	return false
}

// extCommunityHasSimpson reports whether a bracketed extended-community list
// string contains `redirect-to-nexthop-draft` or bare `copy-to-nexthop` (i.e.
// not followed by an IP address operand).
func extCommunityHasSimpson(raw string) bool {
	tokens := strings.Fields(strings.Trim(raw, "[] "))
	for i, tok := range tokens {
		if tok == zeRedirectSimpson {
			return true
		}
		if tok != zeCopySimpson {
			continue
		}
		// `copy-to-nexthop <ip>` is the IETF form; bare `copy-to-nexthop` is Simpson.
		if i+1 < len(tokens) {
			if _, isAddr := flowRedirectAddress(tokens[i+1]); isAddr {
				continue
			}
		}
		return true
	}
	return false
}

// peerNegotiatesFlowSpec reports whether peer's migrated `session > family`
// list contains any FlowSpec family (SAFI 133 or 134).
func peerNegotiatesFlowSpec(peer *config.Tree) bool {
	session := peer.GetContainer("session")
	if session == nil {
		return false
	}
	for _, entry := range session.GetListOrdered("family") {
		fam, ok := family.LookupFamily(entry.Key)
		if !ok {
			continue
		}
		if fam.SAFI == family.SAFIFlowSpec {
			return true
		}
		if fam.SAFI == family.SAFIFlowSpecVPN {
			return true
		}
	}
	return false
}
