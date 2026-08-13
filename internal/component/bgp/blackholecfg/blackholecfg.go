// Design: docs/architecture/plugin/rib-storage-design.md -- the honoring path that consumes a Rule
// RFC: rfc/short/rfc7999.md -- Sections 3.1 and 3.3, the agreement this container records

// Package blackholecfg reads the per-peer `blackhole` container of a BGP config
// subtree.
//
// It holds what the container MEANS and nothing about who acts on it. Three
// deciders need the same answer about one session and each acts on a different
// side of it: the honoring path installs a discard route, origin validation
// keeps a route RFC 6811 would drop, and the origination path decides whether
// BLACKHOLE may go on that peer's wire. A second copy of this walk is what would
// let those three answers drift, which is the failure the filtertext package was
// extracted to prevent for the COMMUNITIES-out-of-filter-text reading.
//
// Delivery is the caller's problem and differs by caller: a plugin that declares
// WantsConfig is handed the subtree, and a command handler pulls it from the
// running tree. Neither shape appears here.
package blackholecfg

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Rule is one peer's resolved `blackhole` container, after the bgp, group and
// peer levels are accumulated.
//
// Communities are the values this session agreed to. A listed community IS the
// agreement RFC 7999 asks for, which is why no separate boolean records it: a
// peer that named no community agreed to nothing. RFC 7999 Section 3.3 requires
// that agreement "on that particular BGP session" before an announcement is
// honored, and Section 3.1 requires it of "the two networks" before the
// community is advertised. One list answers both, because one session is what
// both sentences are about.
//
// Authorized is the set of prefixes the neighbor may advertise, which Section
// 3.3's other condition uses. Neither list alone lets a route be honored.
type Rule struct {
	Communities []attribute.Community
	Authorized  []netip.Prefix
}

// applyDefaultCommunity fills in the well-known value for a session that stated
// which prefixes its neighbor may blackhole within and named no community.
//
// RFC 7999 Section 4 states that "without an explicit configuration directive
// set by the operator, network elements SHOULD NOT discard traffic" toward a
// tagged prefix. The prefixes list IS that directive: an operator who writes the
// blocks a neighbor may blackhole within has configured blackhole handling for
// that session, and the same act is Section 3.3's agreement "on that particular
// BGP session". Section 4's default survives untouched, because a container with
// neither list still reaches nothing here.
//
// A STATED set is taken exactly and the well-known value is NOT unioned into it.
// An operator who names 65001:666 alone means it, and RFC 7999 Section 3.1 makes
// turning the well-known value off their choice to make.
//
// It runs on the RESOLVED rule, after bgp, group and peer are accumulated, so a
// community stated at any level suppresses the default for every level.
func (r Rule) applyDefaultCommunity() Rule {
	if len(r.Communities) > 0 || len(r.Authorized) == 0 {
		return r
	}
	r.Communities = []attribute.Community{attribute.CommunityBlackhole}
	return r
}

// Stated reports whether this peer stated anything at all. A peer that stated
// nothing is left out of the map Parse returns, so a deployment that does not
// use the feature pays one map miss and no wire scan.
func (r Rule) Stated() bool {
	return len(r.Communities) > 0 || len(r.Authorized) > 0
}

// Agreed reports whether this session agreed to one community value.
//
// RFC 7999 Section 3.1: "In a bilateral peering relationship, use of the
// BLACKHOLE community MUST be agreed upon by the two networks before advertising
// it." A peer that named 65001:666 and not the well-known value agreed to
// 65001:666, so this answers per value rather than per session.
func (r Rule) Agreed(c attribute.Community) bool {
	return slices.Contains(r.Communities, c)
}

// Carries reports whether a COMMUNITIES attribute value carries any of want.
//
// value is the attribute's value bytes, a sequence of 4-octet communities. The
// scan steps 4 octets at a time rather than searching for a byte pattern,
// because a match that straddles two adjacent communities is not the value.
// 0x0000FFFF followed by 0x029A0000 contains the four bytes of BLACKHOLE and
// carries no BLACKHOLE.
//
// A trailing partial community is ignored. RFC 7606 governs a malformed
// attribute, and this function is not where that is decided. An empty want set
// matches nothing, which is the closed state.
func Carries(value []byte, want []attribute.Community) bool {
	for i := 0; i+4 <= len(value); i += 4 {
		v := attribute.Community(binary.BigEndian.Uint32(value[i : i+4]))
		if slices.Contains(want, v) {
			return true
		}
	}
	return false
}

// level is one config level's statement, before inheritance resolves it. Both
// leaves are lists and both ACCUMULATE down the levels, so a group that agreed a
// community keeps it for every peer under it.
type level struct {
	communities []attribute.Community
	authorized  []netip.Prefix
}

// Config level names, used only to say where a rejected value came from.
const (
	levelBGP   = "bgp"
	levelGroup = "group of peer"
	levelPeer  = "peer"
)

// Parse resolves the `blackhole` container for every configured peer, keyed by
// the peer's remote IP.
//
// Keyed by IP rather than by config name because every consumer identifies a
// session by address at runtime. configjson.PeerRemoteIP is the single correct
// reader for that value, and the plugins that identify peers by IP (RPKI,
// watchdog, role) all key on it.
//
// Inheritance is bgp, then group, then peer, the same order every other BGP
// plugin leaf uses. Both lists ACCUMULATE, matching their ze:cumulative
// declaration. A peer that adds one authorized block must not silently drop the
// group's, and the same holds for a community.
//
// A malformed value is REFUSED rather than dropped. The container decides
// whether a peer can make Ze discard traffic, and whether Ze may ask a peer to
// discard it, so a dropped entry is an agreement the operator reads as in force
// in the running config and which does nothing.
func Parse(bgpCfg map[string]any) (map[netip.Addr]Rule, error) {
	base, err := parseLevel(bgpCfg, levelBGP, "")
	if err != nil {
		return nil, err
	}

	out := make(map[netip.Addr]Rule)
	var walkErr error
	configjson.ForEachPeer(bgpCfg, func(peerName string, peerMap, groupMap map[string]any) {
		if walkErr != nil {
			return
		}
		lvl := base
		if groupMap != nil {
			groupLevel, err := parseLevel(groupMap, levelGroup, peerName)
			if err != nil {
				walkErr = err
				return
			}
			lvl = merge(lvl, groupLevel)
		}
		if peerMap != nil {
			peerLevel, err := parseLevel(peerMap, levelPeer, peerName)
			if err != nil {
				walkErr = err
				return
			}
			lvl = merge(lvl, peerLevel)
		}

		rule := Rule{Communities: lvl.communities, Authorized: lvl.authorized}
		if !rule.Stated() {
			return
		}
		rule = rule.applyDefaultCommunity()

		ipStr := configjson.PeerRemoteIP(peerMap, groupMap)
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			// Refused rather than skipped. Every lookup keys on the address, so a
			// peer kept under a name nothing queries is a blackhole agreement that
			// reads as in force and does nothing.
			walkErr = fmt.Errorf("blackhole: peer %q states a blackhole block and has no usable remote IP (%q)", peerName, ipStr)
			return
		}
		out[addr] = rule
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

// parseLevel reads one level's blackhole container. lvl and peerName name the
// source of a rejected value; peerName is empty at the bgp level.
func parseLevel(m map[string]any, lvl, peerName string) (level, error) {
	var out level
	block, ok := m["blackhole"].(map[string]any)
	if !ok {
		return out, nil
	}
	for _, s := range anyToStrings(block["communities"]) {
		// Parsed here rather than compared as text later, because the wire scan
		// compares 4-octet values. attribute.ParseCommunity reads both the
		// well-known name and ASN:VAL, so "blackhole" and "65535:666" resolve to
		// the one value RFC 7999 registers.
		v, err := attribute.ParseCommunity(s)
		if err != nil {
			return level{}, fmt.Errorf(
				"blackhole: %s %q: communities %q is not a community value: %w", lvl, peerName, s, err)
		}
		out.communities = append(out.communities, attribute.Community(v))
	}

	for _, s := range anyToStrings(block["prefixes"]) {
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			// A dropped entry narrows what the operator authorized without saying
			// so, and the operator would read the running config as authorizing a
			// block it does not.
			return level{}, fmt.Errorf(
				"blackhole: %s %q: prefixes %q is not a prefix: %w", lvl, peerName, s, err)
		}
		out.authorized = append(out.authorized, pfx.Masked())
	}
	return out, nil
}

// merge applies a more specific level over a less specific one. Both lists
// accumulate: a narrower level adds to what the wider one stated and never
// replaces it.
func merge(base, overlay level) level {
	var out level
	out.communities = append(append([]attribute.Community{}, base.communities...), overlay.communities...)
	out.authorized = append(append([]netip.Prefix{}, base.authorized...), overlay.authorized...)
	return out
}

// anyToStrings normalizes a leaf-list across the shapes the config framework and
// a JSON round-trip both produce: []any, []string, and a bare string for a
// single value.
func anyToStrings(v any) []string {
	switch s := v.(type) {
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok && str != "" {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return s
	case string:
		if s == "" {
			return nil
		}
		return []string{s}
	}
	return nil
}
