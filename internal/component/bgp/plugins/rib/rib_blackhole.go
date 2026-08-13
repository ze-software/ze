// Design: docs/architecture/plugin/rib-storage-design.md -- best-path to FIB candidate
// RFC: rfc/short/rfc7999.md -- Sections 3.3, 4 and 6, the honoring decision
// Related: rib_bestchange.go -- checkBestPathChange, which asks for the route type
// Related: rib_blackhole_config.go -- the per-peer leaves this reads
// Related: yang/ze-rib.yang -- blackhole-honor-fields, the operator surface

package rib

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// errRibInvalidBgpConfigJson refuses a bgp section the RIB cannot parse. It is
// returned rather than logged because the blackhole configuration decides
// whether a peer can make Ze discard traffic.
var errRibInvalidBgpConfigJson = errors.New("rib: invalid bgp config JSON")

// blackholeCommunityWire is BLACKHOLE, 0xFFFF029A, in network byte order. RFC
// 7999 Section 5 registered it, and RFC 1997 gives the COMMUNITIES attribute
// its set semantics. The value is one 4-octet element at any position.
var blackholeCommunityWire = func() [4]byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(attribute.CommunityBlackhole))
	return b
}()

// carriesBlackholeCommunity reports whether a COMMUNITIES attribute value
// carries BLACKHOLE.
//
// data is the attribute's value bytes, a sequence of 4-octet communities. The
// scan steps 4 octets at a time rather than searching for the byte pattern,
// because a match that straddles two adjacent communities is not the value.
// 0x0000FFFF followed by 0x029A0000 contains the four bytes and carries no
// BLACKHOLE.
//
// A trailing partial community is ignored. RFC 7606 governs a malformed
// attribute, and this function is not where that is decided.
func carriesBlackholeCommunity(data []byte) bool {
	for i := 0; i+4 <= len(data); i += 4 {
		if data[i] == blackholeCommunityWire[0] &&
			data[i+1] == blackholeCommunityWire[1] &&
			data[i+2] == blackholeCommunityWire[2] &&
			data[i+3] == blackholeCommunityWire[3] {
			return true
		}
	}
	return false
}

// coveredByAuthorized reports whether route is covered by an equal or shorter
// prefix in authorized.
//
// RFC 7999 Section 3.3 (RFC7999-3.3-1): a speaker "MUST only accept and honor"
// a BLACKHOLE announcement when "the announced IP prefix is covered by an equal
// or shorter IP prefix that the neighboring network is authorized to
// advertise".
//
// COVERAGE, not prefix-list membership. A prefix-list entry carries ge and le
// bounds that say which announcement lengths the operator accepts, and a /24
// entry with no le bound rejects the /32 blackhole inside it. The RFC asks a
// different question. Is there a shorter prefix this neighbor may advertise
// that contains the announced one? So this walks the authorized set directly
// and tests containment plus length, with no bound to fall outside of.
//
// An empty set covers nothing. That is the closed state, and it is why honoring
// cannot be turned on with one leaf. RFC 7999 Section 6 names the unauthorized
// addition of BLACKHOLE as a denial-of-reachability vector.
func coveredByAuthorized(authorized []netip.Prefix, route netip.Prefix) bool {
	if !route.IsValid() {
		return false
	}
	routeIs4 := route.Addr().Is4()
	for _, a := range authorized {
		if a.Addr().Is4() != routeIs4 {
			continue
		}
		// "equal or shorter" binds the AUTHORIZED prefix. It must be no more
		// specific than the announcement, so a /24 authorization covers the /32
		// blackhole inside it and a /16 announcement is covered by nothing here.
		if a.Bits() > route.Bits() {
			continue
		}
		if a.Contains(route.Addr()) {
			return true
		}
	}
	return false
}

// blackholeRouteType returns the forwarding action for one best path, given the
// source peer's blackhole configuration and a test for whether the route
// carries BLACKHOLE.
//
// The zero Type means "no opinion" and the FIB installs an ordinary route. It
// is returned for every case except the one RFC 7999 Section 3.3 permits, and
// the three inputs are the three the RFC names:
//
//  1. cfg.honor. RFC7999-3.3-2, the receiving party agreed to honor BLACKHOLE
//     on that particular BGP session. Default false, which is RFC7999-4-1.
//     Without an explicit configuration directive, do not discard.
//  2. A covering prefix in cfg.authorized. RFC7999-3.3-1.
//  3. hasCommunity. The announcement is tagged. An untagged route is an
//     ordinary announcement whatever the peer is authorized for.
//
// All three hold, or the route forwards. RFC 7999 Section 3.3 states its two
// conditions as one sentence with two bullets, and both hold or the
// announcement is neither accepted nor honored.
//
// hasCommunity is a function rather than a bool because it is the expensive
// input: on the production path it takes r.peerMu and scans an attribute. The
// two config tests run first and it is never called for a route they refuse, so
// each condition is tested in exactly one place. A second copy of the honor
// test in the caller would be a guard no test can distinguish from this one.
func blackholeRouteType(cfg blackholeConfig, route netip.Prefix, hasCommunity func() bool) routetype.Type {
	if !cfg.honor {
		return 0
	}
	if !coveredByAuthorized(cfg.authorized, route) {
		return 0
	}
	if !hasCommunity() {
		return 0
	}
	return routetype.Blackhole
}

// blackholeHonorPeerCount reports how many peers stated an RFC 7999 rule. It
// exists for the configure log line, so an operator can see the leaves landed.
func (r *RIBManager) blackholeHonorPeerCount() int {
	p := r.blackholeCfg.Load()
	if p == nil {
		return 0
	}
	return len(*p)
}

// blackholeRouteTypeForBest returns the forwarding action to stamp on one
// winning best path.
//
// It runs once per best-path CHANGE, not per UPDATE, and it returns before any
// wire scan for a peer that stated no rule. That is the whole cost on a
// deployment that does not use the feature: one atomic load and one map miss.
//
// peerAddr is the winner's peer, so the answer is per session, which is what
// RFC 7999 Section 3.3 requires. A prefix announced by two peers is honored
// only when the peer that WON the best-path selection is the one authorized for
// it: the FIB installs one entry, and it must reflect the route it installs.
//
// Caller must not hold r.peerMu.
func (r *RIBManager) blackholeRouteTypeForBest(fam family.Family, nlriBytes []byte, pfx netip.Prefix, peerAddr netip.Addr) routetype.Type {
	p := r.blackholeCfg.Load()
	if p == nil {
		return 0
	}
	cfg, ok := (*p)[peerAddr]
	if !ok {
		return 0
	}
	return blackholeRouteType(cfg, pfx, func() bool {
		return r.bestCarriesBlackhole(fam, nlriBytes, peerAddr)
	})
}

// bestCarriesBlackhole reports whether the winning peer's stored route for this
// NLRI carries the BLACKHOLE community.
//
// It reads the interned COMMUNITIES attribute out of the peer's own RouteEntry,
// the same route the best-path selection just chose, rather than re-parsing a
// wire payload. lookupSRv6SIDForBest reads the same bundle for the same reason.
//
// Caller must not hold r.peerMu.
func (r *RIBManager) bestCarriesBlackhole(fam family.Family, nlriBytes []byte, peerAddr netip.Addr) bool {
	r.peerMu.RLock()
	peerRIB := r.bgpPeers[peerAddr]
	r.peerMu.RUnlock()
	if peerRIB == nil {
		return false
	}
	entry, ok := peerRIB.Lookup(fam, nlriBytes)
	if !ok {
		return false
	}
	b := entry.GetBundle()
	if !b.HasCommunities() {
		return false
	}
	data, err := pool.Communities.Get(b.Communities)
	if err != nil {
		return false
	}
	return carriesBlackholeCommunity(data)
}
