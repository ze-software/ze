// Design: docs/architecture/core-design.md — community filter ingress path
// RFC: rfc/short/rfc7999.md — Section 3.2, the propagation guard implemented here
// RFC: rfc/short/rfc1997.md — NO_EXPORT and NO_ADVERTISE, the two communities it adds
// Overview: filter_community.go — plugin entry point
// Related: scrub.go — Section 11 scrub that must not reach these values
// Related: filter.go — applyIngressFilter, which runs this after the scrub

package filter_community

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Values of the blackhole-propagation leaf. The two live tokens are the two
// communities RFC 7999 Section 3.2 names, spelled as RFC 1997 spells them.
const (
	blackholeGuardNone        = "none"
	blackholeGuardNoExport    = "no-export"
	blackholeGuardNoAdvertise = "no-advertise"
)

// blackholeGuardCommunity maps a leaf value to the community to add, or
// false when the guard is off.
//
// The default branch is OFF, and every unrecognized token lands in it. A
// guard that stamped a route on an unknown token would suppress
// advertisement of that prefix on the strength of a config typo.
func blackholeGuardCommunity(guard string) (attribute.Community, bool) {
	switch guard {
	case blackholeGuardNoExport:
		return attribute.CommunityNoExport, true
	case blackholeGuardNoAdvertise:
		return attribute.CommunityNoAdvertise, true
	}
	return 0, false
}

// blackholePropagationGuard adds NO_EXPORT or NO_ADVERTISE to a received
// route that carries the BLACKHOLE community. It returns the rewritten
// payload, or nil when nothing changes.
//
// RFC 7999 Section 3.2 (RFC7999-3.2-1): "A BGP speaker receiving an
// announcement tagged with the BLACKHOLE community SHOULD add the
// NO_ADVERTISE or NO_EXPORT community as defined in [RFC1997], or a similar
// community, to prevent propagation of the prefix outside the local AS."
//
// RFC 7999 Section 3.2 (RFC7999-3.2-2): "The community to prevent
// propagation SHOULD be chosen according to the operator's routing policy."
// Both are offered and neither is forced, which is why `guard` is an
// enumeration and not a bool.
//
// RFC 7999 Section 3.1 makes ignoring the BLACKHOLE community a conformant
// choice. So the guard is opt-in per peer and its OFF state is a supported
// configuration. It also means this function must cost nothing when off:
// the caller checks the leaf before calling, and the wire scan below never
// runs for a peer that did not ask for it.
//
// THIS IS THE PRODUCTION READER of wireu.CommunityPolicy.RFC7999Blackhole.
// That field was set by wireu.parseCommunityAttr and read by nothing
// outside a test, because ParseCommunityPolicy is otherwise reached only
// from the two route-server rails, both behind an rsClient gate. Calling it
// here is the first time a normal eBGP peer's communities are parsed on
// ingress.
//
// WHAT REACHES THE PURPOSE. The RFC sentence above states a purpose this
// function cannot reach alone. Adding the community is what Section 3.2 asks
// a receiver to DO. Preventing propagation is what it asks the community to
// ACHIEVE.
//
// Ze's own egress closes that second half. Reactor.wellKnownAllowsEgress
// (reactor/forward_wellknown.go) suppresses a route that carries NO_EXPORT
// or NO_ADVERTISE. RFC1997-Well-1, Well-2 and Well-3 each carry a tagged
// positive and negative pair.
//
// The limit that remains is the OFF state. It is a configuration state, not
// a gap. This guard is opt-in per peer, so a peer that did not ask for it
// adds nothing. A BLACKHOLE-tagged route from that peer is re-advertised as
// any other route is. Section 3.1 makes ignoring the community conformant,
// so that state is supported.
func blackholePropagationGuard(payload []byte, localAS uint32, guard string) []byte {
	add, on := blackholeGuardCommunity(guard)
	if !on {
		return nil
	}

	if !wireu.ParseCommunityPolicy(payload, localAS).RFC7999Blackhole {
		return nil
	}
	if hasStandardCommunity(payload, uint32(add)) {
		// RFC 1997 gives the attribute set semantics, so a second copy carries
		// no meaning. Returning nil also keeps a replayed UPDATE
		// byte-identical.
		return nil
	}

	wire := make([]byte, 4)
	binary.BigEndian.PutUint32(wire, uint32(add))
	return ingressTagCommunities(payload, attribute.AttrCommunity, [][]byte{wire})
}

// hasStandardCommunity reports whether the COMMUNITY attribute already
// carries the given four-octet value.
func hasStandardCommunity(payload []byte, value uint32) bool {
	_, _, dataStart, dataEnd, found := findAttribute(payload, attribute.AttrCommunity)
	if !found {
		return false
	}
	for i := dataStart; i+4 <= dataEnd; i += 4 {
		if binary.BigEndian.Uint32(payload[i:i+4]) == value {
			return true
		}
	}
	return false
}
