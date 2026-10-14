// Design: docs/architecture/core-design.md — community filter ingress path
// RFC: rfc/short/rfc7999.md — Section 3.2, the propagation guard implemented here
// RFC: rfc/short/rfc1997.md — NO_EXPORT and NO_ADVERTISE, the two communities it adds
// RFC: rfc/short/rfc7999.md — Section 3.1, the re-advertisement agreement gate implemented here
// Overview: filter_community.go — plugin entry point
// Related: scrub.go — Section 11 scrub that must not reach these values
// Related: filter.go — applyIngressFilter, which runs this after the scrub

package filter_community

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/blackholecfg"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Values of the blackhole-propagation leaf. The two live tokens are the two
// communities RFC 7999 Section 3.2 names, spelled as RFC 1997 spells them.
// enumeration: gated by TestBlackholeGuardTokensMatchTheYANGModel
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

// blackholeAgreements is every session's RFC 7999 agreement, the map
// blackholecfg.Parse returns re-keyed once at config time so the egress gate
// looks a destination up without building a string per route.
//
// A configured peer's entry is keyed by its remote address: blackholecfg.Parse
// keys every non-template entry by the remote IP and refuses a peer without one.
// A dynamic group's template is keyed by configjson.GroupKey. The split reads
// PeerConfigKey.Template and never whether the ID parses as an address, because
// naming.ValidateNodeName accepts a group named 10.0.0.1.
//
// A value is immutable once built: a reload builds a new set and swaps the
// pointer under mu, so a reader that copied the pointer keeps a whole set.
type blackholeAgreements struct {
	byAddress  map[netip.Addr]blackholecfg.Rule
	byTemplate map[configjson.PeerConfigKey]blackholecfg.Rule
}

// newBlackholeAgreements re-keys the rules blackholecfg.Parse returned.
func newBlackholeAgreements(rules map[configjson.PeerConfigKey]blackholecfg.Rule) (*blackholeAgreements, error) {
	set := &blackholeAgreements{
		byAddress:  make(map[netip.Addr]blackholecfg.Rule, len(rules)),
		byTemplate: make(map[configjson.PeerConfigKey]blackholecfg.Rule),
	}
	for key, rule := range rules {
		if key.Template {
			set.byTemplate[key] = rule
			continue
		}
		addr, err := netip.ParseAddr(key.ID)
		if err != nil {
			// Parse keys a peer by configjson.PeerRemoteIP, so this is a Ze defect,
			// refused rather than dropped: a dropped entry is an agreement the
			// operator reads as in force and which does nothing.
			return nil, fmt.Errorf("blackhole agreement for peer %q has no remote address: %w", key.ID, err)
		}
		set.byAddress[addr.Unmap()] = rule
	}
	return set, nil
}

// lookup resolves one destination's agreement in configjson.LookupPeerConfig's
// order: address first, then name, then the template of the dynamic group the
// peer was built from. A name entry cannot exist by Parse's construction, so
// the name step always misses; it stays so the order matches every other
// reader of the same map. The second value is false on a miss, which the
// caller MUST read as "not agreed". A nil set misses everything.
func (s *blackholeAgreements) lookup(dest filterapi.PeerFilterInfo) (blackholecfg.Rule, bool) {
	if s == nil {
		return blackholecfg.Rule{}, false
	}
	if rule, ok := s.byAddress[dest.Address.Unmap()]; ok {
		return rule, true
	}
	if dest.Name != "" {
		if rule, ok := s.byTemplate[configjson.PeerConfigKey{ID: dest.Name}]; ok {
			return rule, true
		}
	}
	if dest.GroupName != "" {
		if rule, ok := s.byTemplate[configjson.GroupKey(dest.GroupName)]; ok {
			return rule, true
		}
	}
	return blackholecfg.Rule{}, false
}

// wellKnownBlackhole is the want set the gate scans for, held at package level
// so the scan allocates nothing per route.
var wellKnownBlackhole = []attribute.Community{attribute.CommunityBlackhole}

// blackholeReadvertiseRefused reports whether RFC 7999 Section 3.1 withholds
// this route from dest.
//
// RFC 7999 Section 3.1: "In a bilateral peering relationship, use of the
// BLACKHOLE community MUST be agreed upon by the two networks before
// advertising it. In a multilateral peering relationship, the decision to
// honor or ignore the BLACKHOLE community is to be made according to the
// operator's routing policy."
//
// The agreement is stated in terms of the community, not of who created the
// route, so it binds a re-advertisement as it binds origination
// (cmd/announce's agreedSelector). An iBGP session is one network, so it is
// not a bilateral relationship between two, and a customer's RTBH signal keeps
// spreading inside the AS, which Section 3.2 assumes. A route-server client is
// the multilateral case the second sentence hands to operator policy. Ze has no
// confederation config, so every other session is a bilateral eBGP one.
//
// The checks run cheapest first, because this runs once per route per
// destination: the session type, then the 4-octet scan of COMMUNITIES, and
// only for a tagged route the agreement lookup. It allocates nothing.
func blackholeReadvertiseRefused(agreements *blackholeAgreements, dest filterapi.PeerFilterInfo, payload []byte) bool {
	if dest.IsIBGP {
		return false
	}
	if dest.RSClient {
		return false
	}
	_, _, dataStart, dataEnd, found := findAttribute(payload, attribute.AttrCommunity)
	if !found {
		return false
	}
	if !blackholecfg.Carries(payload[dataStart:dataEnd], wellKnownBlackhole) {
		return false
	}
	rule, agreedSession := agreements.lookup(dest)
	if !agreedSession {
		return true
	}
	return !rule.Agreed(attribute.CommunityBlackhole)
}
