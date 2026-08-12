// Design: docs/architecture/core-design.md -- zero-copy community extraction from UPDATE wire bytes
// RFC: rfc/short/rfc1997.md -- well-known communities and the egress scope each one names
// Related: community.go -- the route-server control-community parser, a different concern

package wireu

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// WellKnown is the set of RFC 1997 well-known communities one UPDATE carries.
//
// It is a BIT SET rather than a single value because an UPDATE may carry more
// than one of them and the strictest one decides. Scanning produces it once per
// UPDATE; every destination of a fan-out then asks AllowsEgressTo, which reads
// no memory outside this value (ai/rules/performance.md).
type WellKnown uint8

// RFC 1997 "Well-known Communities": the three values whose operations "shall be
// implemented in any community-attribute-aware BGP speaker".
const (
	// WKNoExport is NO_EXPORT (0xFFFFFF01): "MUST NOT be advertised outside a
	// BGP confederation boundary".
	WKNoExport WellKnown = 1 << iota
	// WKNoAdvertise is NO_ADVERTISE (0xFFFFFF02): "MUST NOT be advertised to
	// other BGP peers".
	WKNoAdvertise
	// WKNoExportSubconfed is NO_EXPORT_SUBCONFED (0xFFFFFF03): "MUST NOT be
	// advertised to external BGP peers (this includes peers in other members
	// autonomous systems inside a BGP confederation)".
	WKNoExportSubconfed
)

// ScanWellKnown reports which RFC 1997 well-known communities an UPDATE payload
// carries in its COMMUNITIES attribute (type 8). The second return says whether
// the payload was read.
//
// A payload with no COMMUNITIES attribute and a payload carrying only ordinary
// communities both answer (0, true). RFC 1997 constrains a route by the values it
// CARRIES and says nothing about a route that carries none.
//
// A payload whose sections do not parse answers (0, false), and an empty set
// advertises to everyone. This gate FAILS OPEN by design. Refusing a route Ze
// could not read would drop legitimate routes on a parse hiccup, and no received
// UPDATE reaches the branch: everything on the forward rails parsed once already,
// on the receive goroutine (reactor/session_read.go, enforceRFC7606).
//
// The second return is what keeps that fail-open from being SILENT. Ze cannot
// honor an obligation it cannot see, so the caller MUST say so rather than read
// an empty set as "no prohibition" (ai/rules/evidence.md).
//
// It reads the attribute SECTION rather than the payload bytes, so an NLRI octet
// run that spells a well-known value is not mistaken for the attribute.
// Allocation free: ParseUpdateSections computes offsets and AttrFind walks the
// attribute headers in place -- the pair payloadHasLocalPref already uses for
// RFC 4271 Section 5.1.5 in the reactor.
//
// A trailing partial value is ignored rather than treated as an error. RFC 1997
// requires the attribute length to be a multiple of 4 (RFC1997-Encoding-1) and
// ParseCommunities enforces that on the decode path, so a payload arriving here
// with a remainder was already refused where the refusal belongs.
func ScanWellKnown(payload []byte) (WellKnown, bool) {
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return 0, false
	}
	// Neither of the next two is a read failure. A payload with no attribute
	// section is a withdrawal-only UPDATE, and one with no COMMUNITIES attribute
	// carries no RFC 1997 value: both are routes the RFC says nothing about.
	attrs := sections.Attrs(payload)
	if attrs == nil {
		return 0, true
	}
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrCommunity)
	if !found {
		return 0, true
	}
	var w WellKnown
	// Any value this chain does not name carries no RFC 1997 egress operation and
	// is left to the RFC that defines it: RFC 3765 NOPEER, RFC 7999 BLACKHOLE,
	// RFC 8326 GRACEFUL_SHUTDOWN and RFC 9494 LLGR_STALE all sit in the same
	// reserved block and none of them is an egress prohibition.
	for i := 0; i+4 <= len(value); i += 4 {
		c := attribute.Community(binary.BigEndian.Uint32(value[i:]))
		if c == attribute.CommunityNoExport {
			w |= WKNoExport
		}
		if c == attribute.CommunityNoAdvertise {
			w |= WKNoAdvertise
		}
		if c == attribute.CommunityNoExportSubconfed {
			w |= WKNoExportSubconfed
		}
	}
	return w, true
}

// AllowsEgressTo answers RFC 1997 for one destination peer: may a route carrying
// this set be advertised to a peer that is internal (isIBGP) or external?
//
// RFC 1997 "Well-known Communities", verbatim:
//
//   - NO_EXPORT: "MUST NOT be advertised outside a BGP confederation boundary
//     (a stand-alone autonomous system that is not part of a confederation
//     should be considered a confederation itself)".
//   - NO_ADVERTISE: "MUST NOT be advertised to other BGP peers". The strictest
//     of the three: no peer at all, internal ones included.
//   - NO_EXPORT_SUBCONFED: "MUST NOT be advertised to external BGP peers (this
//     includes peers in other members autonomous systems inside a BGP
//     confederation)".
//
// THE CONFEDERATION BOUNDARY IS THE AS BOUNDARY HERE, and that is the RFC's own
// answer rather than a shortcut. Ze has no confederation configuration surface:
// a session is internal when LocalAS == PeerAS (PeerSettings.IsIBGP) and
// external otherwise, and neither PeerSettings nor the YANG tree names a
// confederation member-AS. localPrefAllowedTo in the reactor records the same
// fact for RFC 4271 Section 5.1.5 and is the site that exception grows from. So
// every AS Ze runs is "a stand-alone autonomous system that is not part of a
// confederation", which the RFC tells us to consider a confederation itself, and
// NO_EXPORT and NO_EXPORT_SUBCONFED name the same set of peers: the external
// ones. The two are kept as separate bits rather than merged, so the day a
// member-AS becomes configurable this function is the one place their scopes
// diverge: NO_EXPORT_SUBCONFED would then also refuse a confederation peer in
// another member AS, and NO_EXPORT would still permit it.
//
// CALLER OBLIGATION: ask this only for a route RECEIVED from a BGP peer.
// RFC 1997 opens each of the three with "All routes received carrying a
// communities attribute containing this value", and the word is load-bearing. A
// speaker that ORIGINATES a route tagged NO_EXPORT is using the community for
// its purpose -- telling its external neighbor not to pass the route on -- and
// refusing to send it would make the community unusable. Ze originates exactly
// such routes: the AS112 covering prefixes carry NO_EXPORT to an external peer
// (test/interop/scenarios/as112-community-frr).
func (w WellKnown) AllowsEgressTo(isIBGP bool) bool {
	if w == 0 {
		return true // The fast path: no well-known community, nothing to decide.
	}
	if w&WKNoAdvertise != 0 {
		return false
	}
	if isIBGP {
		return true
	}
	return w&(WKNoExport|WKNoExportSubconfed) == 0
}

// BlockingName names the RFC 1997 community that refuses this destination, for a
// metric label or an operator-facing message. It returns "" when the route is
// allowed, and the STRICTEST name when more than one applies, so the label set
// is closed and one suppressed route contributes one observation.
func (w WellKnown) BlockingName(isIBGP bool) string {
	switch {
	case w.AllowsEgressTo(isIBGP):
		return ""
	case w&WKNoAdvertise != 0:
		return "no-advertise"
	case w&WKNoExportSubconfed != 0:
		return "no-export-subconfed"
	default:
		return "no-export"
	}
}
