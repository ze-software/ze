// Design: docs/architecture/plugin/rib-storage-design.md -- RIB plugin config extraction
// RFC: rfc/short/rfc7999.md -- Section 3.3, the per-session agreement and the coverage condition
// Related: rib_admin_distance_config.go -- sibling extractor pattern
// Related: yang/ze-rib.yang -- blackhole-honor-fields, the leaves the shared reader parses

package rib

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/blackholecfg"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// blackholeConfig is one peer's resolved RFC 7999 configuration, in the shape
// the honoring path reads it.
//
// communities are the values this session agreed to honor. A listed community IS
// the agreement RFC 7999 Section 3.3 requires on that particular BGP session,
// which is why no separate boolean records it. authorized is the set of prefixes
// the neighbor may advertise, which the same section uses for its coverage
// condition. Neither alone lets a route be honored.
//
// These are RESOLVED values, so communities is never empty here for a peer that
// stated authorized prefixes: blackholecfg fills in the well-known value for
// that case, because stating the prefixes IS RFC 7999 Section 4's explicit
// configuration directive. A peer that stated neither never reaches this map.
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

// parseBlackholeConfig resolves the RFC 7999 leaves for every configured peer,
// keyed by the peer's remote IP.
//
// The walk itself lives in blackholecfg, because two other deciders read the
// same container: origin validation keeps a route RFC 6811 would drop, and the
// origination path decides whether BLACKHOLE may go on a peer's wire. This is
// the one place the shared shape becomes the honoring path's own, so the
// coverage test below it can stay where the honoring decision is made.
func parseBlackholeConfig(bgpCfg map[string]any) (map[netip.Addr]blackholeConfig, error) {
	rules, err := blackholecfg.Parse(bgpCfg)
	if err != nil {
		return nil, err
	}
	out := make(map[netip.Addr]blackholeConfig, len(rules))
	for addr, rule := range rules {
		out[addr] = blackholeConfig{communities: rule.Communities, authorized: rule.Authorized}
	}
	return out, nil
}
