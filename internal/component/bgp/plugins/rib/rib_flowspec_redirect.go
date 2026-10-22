// Design: docs/architecture/plugin/rib-storage-design.md -- the redirect-to-IP EBGP check
// Related: rib_flowspec_validation.go -- reconcileFlowSpecs, which computes the verdict
// Related: yang/ze-rib.yang -- flowspec-redirect-check-fields, the per-session off switch

package rib

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// flowSpecRedirectConfig is the resolved per-session switch of the
// draft-simpson-idr-flowspec-redirect-02 Section 3 EBGP check. base is the
// bgp-level answer, which a peer the walk did not visit falls back to; peers
// holds every visited peer's and dynamic group's resolved answer. true means
// the check runs, which is the draft's default.
type flowSpecRedirectConfig struct {
	base  bool
	peers map[configjson.PeerConfigKey]bool
}

// flowSpecRedirectChecked answers whether the session at addr runs the check.
// The caller MUST hold peerMu, which peerGroupName reads under.
func (r *RIBManager) flowSpecRedirectChecked(addr netip.Addr) bool {
	cfg := r.flowSpecRedirectCfg.Load()
	if cfg == nil {
		return true
	}
	if enabled, ok := configjson.LookupPeerConfig(cfg.peers, addr.String(), "", r.peerGroupName(addr)); ok {
		return enabled
	}
	return cfg.base
}

// parseFlowSpecRedirectConfig resolves flowspec-redirect-check enabled across
// the bgp, group and peer levels: the peer's statement beats its group's, and
// the group's beats the bgp level's. Nothing stated anywhere is the draft's
// default, the check on.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "It MUST be possible to
// disable this additional validation check on a per-EBGP session basis."
func parseFlowSpecRedirectConfig(bgpCfg map[string]any) (flowSpecRedirectConfig, error) {
	cfg := flowSpecRedirectConfig{base: true, peers: map[configjson.PeerConfigKey]bool{}}
	var err error
	if cfg.base, err = flowSpecRedirectLevel(bgpCfg, true, "bgp"); err != nil {
		return flowSpecRedirectConfig{}, err
	}
	configjson.ForEachPeer(bgpCfg, func(name string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		if err != nil {
			return
		}
		enabled := cfg.base
		if groupMap != nil {
			if enabled, err = flowSpecRedirectLevel(groupMap, enabled, "bgp group "+origin.Group); err != nil {
				return
			}
		}
		if peerMap != nil {
			if enabled, err = flowSpecRedirectLevel(peerMap, enabled, "bgp peer "+name); err != nil {
				return
			}
		}
		if key, ok := configjson.KeyFor(name, peerMap, groupMap, origin); ok {
			cfg.peers[key] = enabled
		}
	})
	if err != nil {
		return flowSpecRedirectConfig{}, err
	}
	return cfg, nil
}

// flowSpecRedirectLevel reads one level's flowspec-redirect-check enabled
// leaf, answering inherited when the level states none. A value that is not a
// boolean is refused: reading it as either answer would tell the operator an
// edit took effect that did not.
func flowSpecRedirectLevel(level map[string]any, inherited bool, where string) (bool, error) {
	container, ok := level["flowspec-redirect-check"].(map[string]any)
	if !ok {
		return inherited, nil
	}
	switch v := container["enabled"].(type) {
	case nil:
		return inherited, nil
	case bool:
		return v, nil
	case string:
		if v == "true" {
			return true, nil
		}
		if v == "false" {
			return false, nil
		}
	}
	return false, fmt.Errorf("%s flowspec-redirect-check enabled %v is not a boolean: "+
		"the RIB cannot tell whether to check a peer's FlowSpec redirect-to-IP next hop; "+
		"fix: set %s flowspec-redirect-check enabled true (or false)", where, container["enabled"], where)
}

// flowSpecRedirectTarget answers the redirect-to-IP next hop X the EBGP check
// owes a verdict on, and an invalid address when the path owes none: no
// effective 0x0800 action (attribute.FlowSpecRedirectsToNextHop, which a
// redirect-to-VRF voids) or no IPv4 or IPv6 address in the MP_REACH next hop,
// the two cases where the draft ignores the community anyway.
func flowSpecRedirectTarget(entry storage.RouteEntry) netip.Addr {
	bundle := entry.GetBundle()
	if !bundle.ExtCommunities.IsValid() || !bundle.HasOtherAttrs() {
		return netip.Addr{}
	}
	communities, err := pool.ExtCommunities.Get(bundle.ExtCommunities)
	if err != nil || !attribute.FlowSpecRedirectsToNextHop(communities) {
		return netip.Addr{}
	}
	return flowSpecMPNextHop(bundle)
}

// flowSpecMPNextHop reads the MP_REACH_NLRI next hop of a stored FlowSpec
// path when it is a 4- or 16-octet address, the lengths the forward rails
// treat as a redirect target (reactor flowSpecRedirectNextHop). Every other
// length answers an invalid address. mpNextHopAddr answers nothing for a
// FlowSpec SAFI on purpose, because RFC 8955 Section 4 ignores the field for
// every other use.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "If an MP_REACH_NLRI
// containing one or more flow-spec NLRI does not have a valid IPv4 or IPv6
// address in its next-hop field, or the length of the next-hop is 0, then the
// redirect-to-IP extended community, if present, should be ignored."
func flowSpecMPNextHop(bundle storage.Bundle) netip.Addr {
	data, err := pool.OtherAttrs.Get(bundle.OtherAttrs)
	if err != nil {
		return netip.Addr{}
	}
	// Stored attributes carry a four-octet header: code, flags, two-octet length.
	for off := 0; off+4 <= len(data); {
		code, length := data[off], int(data[off+2])<<8|int(data[off+3])
		off += 4
		if off+length > len(data) {
			return netip.Addr{}
		}
		if code != byte(attribute.AttrMPReachNLRI) {
			off += length
			continue
		}
		// AFI(2) + SAFI(1) + next-hop length(1) + next hop.
		value := data[off : off+length]
		if len(value) < 4 || len(value) < 4+int(value[3]) {
			return netip.Addr{}
		}
		switch hop := value[4 : 4+int(value[3])]; len(hop) {
		case 4:
			return netip.AddrFrom4([4]byte(hop))
		case 16:
			return netip.AddrFrom16([16]byte(hop)).Unmap()
		}
		return netip.Addr{}
	}
	return netip.Addr{}
}

// flowSpecRedirectRefused applies the check to one EBGP path that owes it.
// The longest unicast match for X is found in the rule's own domain (a VPN
// rule's Route Distinguisher), in X's address family. Its last AS is the
// rightmost AS_SEQUENCE ASN, the origin: the draft's authors restate this
// check in draft-ietf-idr-flowspec-redirect-ip-02 Section 3 as "the best
// matching route to X is not a BGP route with origin AS matching the peer
// AS". No match, an AS_SET origin, and an empty path have no last AS that can
// equal the peer's, so each refuses.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "If the flow-spec route
// has an IP next-hop X and includes a redirect-to-IP extended community, then
// the BGP speaker SHOULD discard the redirect-to-ip extended community (and
// not propagate it further with the flow-spec route) if the last AS in the
// AS_PATH or AS4_PATH attribute of the longest prefix match for X does not
// match the AS of the EBGP peer."
func flowSpecRedirectRefused(flow *flowSpecRoute, unicast []flowSpecUnicast) bool {
	domain := flow.domain
	domain.family.AFI = family.AFIIPv4
	if flow.redirectTarget.Is6() {
		domain.family.AFI = family.AFIIPv6
	}
	var best *flowSpecUnicast
	for i := range unicast {
		u := &unicast[i]
		if u.domain != domain || !u.prefix.Contains(flow.redirectTarget) {
			continue
		}
		if best == nil || u.prefix.Bits() > best.prefix.Bits() || (u.prefix == best.prefix && ComparePair(u.candidate, best.candidate) < 0) {
			best = u
		}
	}
	return best == nil || best.lastAS == 0 || best.lastAS != flow.candidate.PeerASN
}

// stripFlowSpecRedirect removes every redirect-to-IP community from an owned
// copy of an EXTENDED_COMMUNITIES value, in place, for a winner whose action
// the check refused, so the local install sees the route without it.
func stripFlowSpecRedirect(communities []byte) []byte {
	out := communities[:0]
	for off := 0; off+8 <= len(communities); off += 8 {
		ec := communities[off : off+8]
		if attribute.FlowSpecRedirectsToNextHop(ec) {
			continue
		}
		out = append(out, ec...)
	}
	return out
}
