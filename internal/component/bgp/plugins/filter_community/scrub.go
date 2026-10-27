// Design: docs/architecture/core-design.md — community filter ingress path
// RFC: rfc/short/rfc7454.md — Section 11, inbound community scrubbing
// RFC: rfc/short/rfc1997.md — the reserved community ranges the scrub may not enter
// Overview: filter_community.go — plugin entry point
// Related: relation.go — RFC 8195 relation tag written after this scrub
// Related: filter.go — ingressStripCommunities, the rebuild this reuses

package filter_community

import (
	"bytes"
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// reservedCommunityHigh is the high half of RFC 1997 reserved range
// 0xFFFF0000-0xFFFFFFFF, which holds every well-known community: NO_EXPORT,
// NO_ADVERTISE, NO_EXPORT_SUBCONFED, GRACEFUL_SHUTDOWN, BLACKHOLE and the
// rest.
//
// No AS is assigned 65535 (RFC 7300 reserves it), so this value can never
// be a legitimate Global Administrator. The guard below refuses to treat it
// as one whatever the local AS is configured to. That is deliberate rather
// than arithmetic luck: RFC 7454 Section 11's fourth bullet protects
// NO_EXPORT by name. A scrub that deletes an operator's blackhole request
// turns a DDoS mitigation into a no-op.
const reservedCommunityHigh uint32 = 0xFFFF

// scrubOwnGACommunities removes every COMMUNITY and LARGE_COMMUNITY value
// whose Global Administrator is the local AS and whose function is not
// kept. It returns the rewritten payload, or nil when nothing matched.
//
// RFC 7454 Section 11 (RFC7454-11-1): "Network administrators SHOULD scrub
// inbound communities with their number in the high-order bits. Allow only
// those communities that customers/peers can use as a signaling mechanism."
//
// That sentence is ONE obligation with a carve-out, which is why `keep` is
// a keep-list and not a denylist. A denylist fails open for every function
// a neighbor invents, and inventing one is exactly what Section 11 exists
// to prevent. An empty keep-list therefore removes every own-GA value; it
// does not mean "keep everything" (ai/rules/evidence.md, the zero-value
// trap).
//
// RFC 7454 Section 11 (RFC7454-11-2): "Networks administrators SHOULD NOT
// remove other communities applied on received routes." Only our own Global
// Administrator is in scope. So a value carrying any other ASN is never
// touched.
//
// relationFunction, when non-zero, is never kept whatever `keep` says: a
// stored relation tag a neighbor sent would be that neighbor stating its
// own relation to us. Pass 0 when no relation function is configured.
//
// routeServerAS is the ASN the route server reads and strips its control
// communities by (PeerFilterInfo.RouteServerAS: the router's global
// local-as, the value both forward rails pass to wireu.ParseCommunityPolicy
// and the strip helpers), and zero on a session that is not a route-server
// client. On a client's session the values those rails act on
// (wireu.IsRSControlCommunity, wireu.IsRSControlLargeCommunity, matched
// against routeServerAS) are kept whatever `keep` says, because they are the
// signaling mechanism Section 11 tells the operator to allow, and the route
// server reads them only AFTER this ingress pass: the forward rails parse the
// cached, post-filter payload. Scrubbing them would silently disable every
// announce, do-not-announce and prepend request the client sends. The route
// server removes them itself before a client receives the route
// (wireu.StripControlCommunities, wireu.StripLargeControlCommunities), so
// keeping them here leaks nothing.
//
// The keep matches against routeServerAS and never against localAS. The two
// differ when the client's session carries a per-peer local-as override: a
// value under the override AS shaped like a control is then one the route
// server neither acts on nor strips, so keeping it would hand it to every
// client. It is scrubbed like any other own-GA value. A true control under
// the global AS is not under this session's Global Administrator at all, so
// the scrub never touches it. The set kept is therefore exactly the set the
// route server acts on and strips.
// Every other own-GA value is still scrubbed on that session.
//
// The two attribute widths are scanned separately and rewritten through
// ingressStripCommunities, the same exact-value rebuild the operator's
// named `strip` sets use. Reusing it is what keeps `strip` an exact
// whole-value match: the scrub selects DIFFERENT values, it does not give
// `strip` a wildcard.
//
// Each width's removal is counted (recordScrub) once its rebuild succeeded:
// ownGA*Values lists every occurrence, and the rebuild removes each one.
func scrubOwnGACommunities(payload []byte, localAS uint32, keep map[uint32]bool, relationFunction, routeServerAS uint32) []byte {
	current := payload

	if vals := ownGAStandardValues(current, localAS, keep, relationFunction, routeServerAS); len(vals) > 0 {
		if modified := ingressStripCommunities(current, attribute.AttrCommunity, 4, vals); modified != nil {
			current = modified
			recordScrub(attribute.AttrCommunity, len(vals))
		}
	}
	if vals := ownGALargeValues(current, localAS, keep, relationFunction, routeServerAS); len(vals) > 0 {
		if modified := ingressStripCommunities(current, attribute.AttrLargeCommunity, 12, vals); modified != nil {
			current = modified
			recordScrub(attribute.AttrLargeCommunity, len(vals))
		}
	}

	if bytes.Equal(current, payload) {
		return nil
	}
	return current
}

// keepsRouteServerControl reports whether the session is a route-server
// client whose control communities the scrub keeps. A zero routeServerAS is
// the "not a client" answer PeerFilterInfo.RouteServerAS documents, and it
// MUST keep nothing. The guard is named here rather than left to the
// predicates happening to match nothing for ASN 0: wireu.IsRSControlCommunity
// treats Global Administrator 0 as a control for every ASN.
func keepsRouteServerControl(routeServerAS uint32) bool {
	return routeServerAS != 0
}

// scrubKeepsFunction answers the carve-out for one function number.
func scrubKeepsFunction(fn uint32, keep map[uint32]bool, relationFunction uint32) bool {
	if relationFunction != 0 && fn == relationFunction {
		return false
	}
	return keep[fn]
}

// ownGAStandardValues collects RFC 1997 community values to remove.
//
// A standard community's "number in the high-order bits" is its high
// sixteen bits. So this can only match a local AS that fits in sixteen
// bits. A four-octet local AS has no standard-community form at all, which
// is a property of RFC 1997 rather than a limit of this scan. The loop
// below matches nothing. The low sixteen bits are read as the
// function, the same role LD1 plays in the large-community form, so one
// keep-list serves both widths.
func ownGAStandardValues(payload []byte, localAS uint32, keep map[uint32]bool, relationFunction, routeServerAS uint32) [][]byte {
	if localAS == 0 || localAS > 0xFFFF || localAS == reservedCommunityHigh {
		return nil
	}
	_, _, dataStart, dataEnd, found := findAttribute(payload, attribute.AttrCommunity)
	if !found {
		return nil
	}

	var out [][]byte
	for i := dataStart; i+4 <= dataEnd; i += 4 {
		high := uint32(binary.BigEndian.Uint16(payload[i : i+2]))
		if high != localAS {
			continue
		}
		if keepsRouteServerControl(routeServerAS) && wireu.IsRSControlCommunity(uint16(high), routeServerAS) {
			continue
		}
		fn := uint32(binary.BigEndian.Uint16(payload[i+2 : i+4]))
		if scrubKeepsFunction(fn, keep, relationFunction) {
			continue
		}
		out = append(out, bytes.Clone(payload[i:i+4]))
	}
	return out
}

// ownGALargeValuesWithFunction collects every own-Global-Administrator
// large community carrying exactly this function number. It is the de-forge
// selector applyRelationTag uses: whatever Section 11 scrub is or is not
// doing, a value stating OUR relation to a route must be one Ze wrote.
func ownGALargeValuesWithFunction(payload []byte, localAS, function uint32) [][]byte {
	if localAS == 0 {
		return nil
	}
	_, _, dataStart, dataEnd, found := findAttribute(payload, attribute.AttrLargeCommunity)
	if !found {
		return nil
	}

	var out [][]byte
	for i := dataStart; i+12 <= dataEnd; i += 12 {
		if binary.BigEndian.Uint32(payload[i:i+4]) != localAS {
			continue
		}
		if binary.BigEndian.Uint32(payload[i+4:i+8]) != function {
			continue
		}
		out = append(out, bytes.Clone(payload[i:i+12]))
	}
	return out
}

// ownGALargeValues collects RFC 8092 large-community values to remove:
// Global Administrator equal to the local AS, Local Data Part 1 read as the
// function, per RFC 8195 Section 2 convention.
func ownGALargeValues(payload []byte, localAS uint32, keep map[uint32]bool, relationFunction, routeServerAS uint32) [][]byte {
	if localAS == 0 {
		return nil
	}
	_, _, dataStart, dataEnd, found := findAttribute(payload, attribute.AttrLargeCommunity)
	if !found {
		return nil
	}

	var out [][]byte
	for i := dataStart; i+12 <= dataEnd; i += 12 {
		if binary.BigEndian.Uint32(payload[i:i+4]) != localAS {
			continue
		}
		fn := binary.BigEndian.Uint32(payload[i+4 : i+8])
		if keepsRouteServerControl(routeServerAS) && wireu.IsRSControlLargeCommunity(localAS, fn, routeServerAS) {
			continue
		}
		if scrubKeepsFunction(fn, keep, relationFunction) {
			continue
		}
		out = append(out, bytes.Clone(payload[i:i+12]))
	}
	return out
}
