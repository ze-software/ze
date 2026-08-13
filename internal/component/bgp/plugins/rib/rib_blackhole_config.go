// Design: docs/architecture/plugin/rib-storage-design.md -- RIB plugin config extraction
// RFC: rfc/short/rfc7999.md -- Section 3.3, the per-session agreement and the coverage condition
// Related: rib_admin_distance_config.go -- sibling extractor pattern
// Related: yang/ze-rib.yang -- blackhole-honor-fields, the leaves parsed here

package rib

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// blackholeConfig is one peer's resolved RFC 7999 configuration.
//
// communities are the values this session agreed to honor. A listed community
// IS the agreement RFC 7999 Section 3.3 requires on that particular BGP session,
// which is why no separate boolean records it: a peer that named no community
// agreed to nothing. authorized is the set of prefixes the neighbor may
// advertise, which the same section uses for its coverage condition. Neither
// alone lets a route be honored.
type blackholeConfig struct {
	communities []attribute.Community
	authorized  []netip.Prefix
}

// hasAnyRule reports whether this peer stated anything at all. A peer that
// stated nothing is left out of the map, so the honoring path on an
// unconfigured deployment costs one map miss and no wire scan.
func (c blackholeConfig) hasAnyRule() bool {
	return len(c.communities) > 0 || len(c.authorized) > 0
}

// blackholeLevel is one config level's statement, before inheritance resolves
// it. Both leaves are lists and both ACCUMULATE down the levels, so a group that
// agreed a community keeps it for every peer under it.
type blackholeLevel struct {
	communities []attribute.Community
	authorized  []netip.Prefix
}

// Config level names, used only to say where a rejected value came from.
const (
	blackholeLevelBGP   = "bgp"
	blackholeLevelGroup = "group of peer"
	blackholeLevelPeer  = "peer"
)

// parseBlackholeConfig resolves the RFC 7999 leaves for every configured peer,
// keyed by the peer's remote IP.
//
// Keyed by IP rather than by config name because the honoring path runs at
// best-path selection, where the source is an address. configjson.PeerRemoteIP
// is the single correct reader for that value, and the plugins that identify
// peers by IP at runtime (RPKI, watchdog, role) all key on it.
//
// Inheritance is bgp, then group, then peer, the same order every other BGP
// plugin leaf uses. Both lists ACCUMULATE, matching their ze:cumulative
// declaration. A peer that adds one authorized block must not silently drop the
// group's, and the same holds for a community.
func parseBlackholeConfig(bgpCfg map[string]any) (map[netip.Addr]blackholeConfig, error) {
	base, err := parseBlackholeLevel(bgpCfg, blackholeLevelBGP, "")
	if err != nil {
		return nil, err
	}

	out := make(map[netip.Addr]blackholeConfig)
	var walkErr error
	configjson.ForEachPeer(bgpCfg, func(peerName string, peerMap, groupMap map[string]any) {
		if walkErr != nil {
			return
		}
		level := base
		if groupMap != nil {
			groupLevel, err := parseBlackholeLevel(groupMap, blackholeLevelGroup, peerName)
			if err != nil {
				walkErr = err
				return
			}
			level = mergeBlackholeLevels(level, groupLevel)
		}
		if peerMap != nil {
			peerLevel, err := parseBlackholeLevel(peerMap, blackholeLevelPeer, peerName)
			if err != nil {
				walkErr = err
				return
			}
			level = mergeBlackholeLevels(level, peerLevel)
		}

		cfg := blackholeConfig(level)
		if !cfg.hasAnyRule() {
			return
		}

		ipStr := configjson.PeerRemoteIP(peerMap, groupMap)
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			// Refused rather than skipped. The lookup keys on the address, so a
			// peer kept under a name it never queries is a blackhole agreement
			// that reads as in force and does nothing.
			walkErr = fmt.Errorf("blackhole: peer %q states a blackhole block and has no usable remote IP (%q)", peerName, ipStr)
			return
		}
		out[addr] = cfg
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

// parseBlackholeLevel reads one level's blackhole container. level and peerName
// name the source of a rejected value; peerName is empty at the bgp level.
func parseBlackholeLevel(m map[string]any, level, peerName string) (blackholeLevel, error) {
	var out blackholeLevel
	block, ok := m["blackhole"].(map[string]any)
	if !ok {
		return out, nil
	}
	for _, s := range anyToStrings(block["community"]) {
		// Parsed here rather than compared as text later, because the wire scan
		// compares 4-octet values. attribute.ParseCommunity reads both the
		// well-known name and ASN:VAL, so "blackhole" and "65535:666" resolve to
		// the one value RFC 7999 registers.
		v, err := attribute.ParseCommunity(s)
		if err != nil {
			// Refused rather than dropped, for the same reason a prefix is: a
			// dropped community is an agreement the operator reads as in force
			// in the running config, and which honors nothing.
			return blackholeLevel{}, fmt.Errorf(
				"blackhole: %s %q: community %q is not a community value: %w", level, peerName, s, err)
		}
		out.communities = append(out.communities, attribute.Community(v))
	}

	for _, s := range anyToStrings(block["authorized-covering-prefix"]) {
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			// Refused rather than dropped. A dropped entry narrows what the
			// operator authorized without saying so, and the operator would read
			// the running config as authorizing a block it does not.
			return blackholeLevel{}, fmt.Errorf(
				"blackhole: %s %q: authorized-covering-prefix %q is not a prefix: %w", level, peerName, s, err)
		}
		out.authorized = append(out.authorized, pfx.Masked())
	}
	return out, nil
}

// mergeBlackholeLevels applies a more specific level over a less specific one.
// Both lists accumulate: a narrower level adds to what the wider one stated and
// never replaces it.
func mergeBlackholeLevels(base, overlay blackholeLevel) blackholeLevel {
	var out blackholeLevel
	out.communities = append(append([]attribute.Community{}, base.communities...), overlay.communities...)
	out.authorized = append(append([]netip.Prefix{}, base.authorized...), overlay.authorized...)
	return out
}

// anyToStrings normalizes a leaf-list across the shapes the config framework
// and a JSON round-trip both produce: []any, []string, and a bare string for a
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
