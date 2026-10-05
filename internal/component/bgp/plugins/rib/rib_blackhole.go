// Design: docs/architecture/plugin/rib-storage-design.md -- best-path to FIB candidate
// RFC: rfc/short/rfc7999.md -- Sections 3.3, 4 and 6, the honoring decision
// Related: rib_bestchange.go -- checkBestPathChange, which asks for the route type
// Related: rib_blackhole_config.go -- the per-peer leaves this reads
// Related: yang/ze-rib.yang -- blackhole-honor-fields, the operator surface

package rib

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/blackholecfg"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// errRibInvalidBgpConfigJson refuses a bgp section the RIB cannot parse. It is
// returned rather than logged because the blackhole configuration decides
// whether a peer can make Ze discard traffic.
var errRibInvalidBgpConfigJson = errors.New("rib: invalid bgp config JSON")

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
//  1. cfg.communities is non-empty. RFC7999-3.3-2, the receiving party agreed
//     to honor BLACKHOLE on that particular BGP session. A session that agreed
//     to nothing has an empty set here and discards nothing, which is
//     RFC7999-4-1: without an explicit configuration directive, do not discard.
//     Which configuration counts as that directive is blackholecfg's decision,
//     not this one's: it resolves prefixes-without-communities to the well-known
//     value before the map is built.
//  2. A covering prefix in cfg.authorized. RFC7999-3.3-1.
//  3. hasCommunity. The announcement carries one of the agreed communities. An
//     untagged route is an ordinary announcement whatever the peer is
//     authorized for.
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
	if len(cfg.communities) == 0 {
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

// blackholeHonorRuleCount reports how many RFC 7999 rules the configuration
// resolved to. It exists for the configure log line, so an operator can see the
// leaves landed.
//
// It counts RULES rather than sessions: a dynamic group contributes one entry
// that every member of its listen range resolves to, and the count cannot say
// how many members will connect.
func (r *RIBManager) blackholeHonorRuleCount() int {
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
// best names the winner, so the answer is per session, which is what
// RFC 7999 Section 3.3 requires. A prefix announced by two peers is honored
// only when the peer that WON the best-path selection is the one authorized for
// it: the FIB installs one entry, and it must reflect the route it installs.
//
// A session created from a dynamic group is resolved by that group's name. Such
// a session's address is written nowhere in the operator's document, so the
// group is the one identity the document and the session share, and the rule
// the operator stated on the listen-range group would otherwise reach none of
// its members. The group is consulted only after the address misses, so a
// member that states its own rule keeps it.
//
// Caller MUST hold peerMu.RLock and retain best.entry until this function returns.
func (r *RIBManager) blackholeRouteTypeForBest(prefix netip.Prefix, best *Candidate) routetype.Type {
	p := r.blackholeCfg.Load()
	if p == nil || len(*p) == 0 {
		return 0
	}
	// An unconfigured deployment stops before the peer-group metadata read or
	// community scan. A configured deployment uses the candidate's already
	// canonical peer address, without formatting or another storage lookup.
	//
	// The name arm is empty because this plugin identifies a session by address
	// and carries no config name for it. It answers a peer whose config key IS
	// its own name, and configjson.PeerKey stores one there only when the name
	// parses as an address, which config.validatePeerName refuses.
	cfg, ok := configjson.LookupPeerConfig(*p, best.PeerAddr, "", r.peerGroupName(best.PeerIP))
	if !ok {
		return 0
	}
	// Asked for CIDR families only: authorization is by the elected prefix.
	return blackholeRouteType(cfg, prefix, func() bool {
		return entryCarriesBlackhole(best.entry, cfg.communities)
	})
}

// peerGroupName returns the peer-group one session belongs to. It is empty for
// a standalone peer, and for a peer no event has been received from yet.
//
// Caller MUST hold peerMu.RLock.
func (r *RIBManager) peerGroupName(peerAddr netip.Addr) string {
	meta := r.peerMeta[peerAddr]
	if meta == nil {
		return ""
	}
	return meta.GroupName
}

// entryCarriesBlackhole reports whether the retained winning snapshot carries a
// configured BLACKHOLE community. The caller MUST retain entry through this read.
func entryCarriesBlackhole(entry storage.RouteEntry, want []attribute.Community) bool {
	b := entry.GetBundle()
	if !b.HasCommunities() {
		return false
	}
	data, err := pool.Communities.Get(b.Communities)
	if err != nil {
		return false
	}
	return blackholecfg.Carries(data, want)
}
