// VALIDATES: the per-peer RFC 7999 leaves resolve with the bgp, group and peer
// inheritance every other BGP plugin leaf uses, and key on the peer's remote IP
// rather than on its config name.
// PREVENTS: a group-level agreement silently dropped by a peer that states one
// of its own, and a honoring decision that cannot find its peer because the
// config keys by name while the RIB knows an address.

package rib

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func parseBlackholeJSON(t *testing.T, jsonStr string) map[netip.Addr]blackholeConfig {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &tree); err != nil {
		t.Fatalf("bad test config: %v", err)
	}
	bgp, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("test config has no bgp subtree")
	}
	got, err := parseBlackholeConfig(bgp)
	if err != nil {
		t.Fatalf("parseBlackholeConfig: %v", err)
	}
	return got
}

func TestParseBlackholeConfigPeerLevel(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"peer":{"upstream":{
		"connection":{"remote":{"ip":"198.51.100.1"}},
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0/24","2001:db8::/32"]}
	}}}}`)

	addr := netip.MustParseAddr("198.51.100.1")
	cfg, ok := cfgs[addr]
	if !ok {
		t.Fatalf("no config for %v; got keys %v", addr, cfgs)
	}
	if !slices.Contains(cfg.communities, attribute.CommunityBlackhole) {
		t.Errorf("communities = %v, want the well-known BLACKHOLE value", cfg.communities)
	}
	if len(cfg.authorized) != 2 {
		t.Fatalf("authorized = %v, want 2 prefixes", cfg.authorized)
	}
}

// A peer that says nothing inherits the group. RFC 7999 Section 3.3 binds the
// agreement to the session, and an operator who states it once for a group of
// sessions has stated it for each of them.
func TestParseBlackholeConfigInheritsFromGroup(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"group":{"customers":{
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0/24"]},
		"peer":{"cust-a":{"connection":{"remote":{"ip":"198.51.100.1"}}}}
	}}}}`)

	cfg := cfgs[netip.MustParseAddr("198.51.100.1")]
	if !slices.Contains(cfg.communities, attribute.CommunityBlackhole) {
		t.Error("the group's agreed community did not reach the peer")
	}
	if len(cfg.authorized) != 1 {
		t.Errorf("group-level authorization did not reach the peer: %v", cfg.authorized)
	}
}

// A peer adds its own community to the group's rather than replacing it, and a
// peer that states a blackhole block for its own reasons does not lose the
// group's agreement by doing so.
//
// The shape this pins also states what it CANNOT express: with the agreement
// carried by a list, a peer cannot opt out of a community its group agreed. An
// operator who needs one session excluded states the community on the peers
// rather than on the group.
func TestParseBlackholeConfigPeerAddsToGroupCommunity(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"group":{"customers":{
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0/24"]},
		"peer":{"cust-a":{"connection":{"remote":{"ip":"198.51.100.1"}},"blackhole":{"community":["65001:666"]}}}
	}}}}`)

	cfg := cfgs[netip.MustParseAddr("198.51.100.1")]
	if !slices.Contains(cfg.communities, attribute.CommunityBlackhole) {
		t.Error("the peer's own community replaced the group's instead of adding to it")
	}
	if !slices.Contains(cfg.communities, attribute.Community(65001<<16|666)) {
		t.Errorf("the peer's own community did not reach it: %v", cfg.communities)
	}
}

// An unparseable community is a refused config, for the same reason an
// unparseable prefix is: an agreement the operator reads as in force in the
// running config, and which honors nothing.
func TestParseBlackholeConfigRejectsBadCommunity(t *testing.T) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(`{"bgp":{"peer":{"upstream":{
		"connection":{"remote":{"ip":"198.51.100.1"}},
		"blackhole":{"community":["not-a-community"],"authorized-covering-prefix":["192.0.2.0/24"]}
	}}}}`), &tree); err != nil {
		t.Fatalf("bad test config: %v", err)
	}
	bgpCfg, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("test config has no bgp subtree")
	}
	if _, err := parseBlackholeConfig(bgpCfg); err == nil {
		t.Fatal("parseBlackholeConfig accepted an unparseable community")
	}
}

// The authorization list is ze:cumulative, so a group statement and a peer
// statement ACCUMULATE rather than replacing each other. A replacing list would
// make a peer that adds one prefix silently drop the group's.
func TestParseBlackholeConfigAuthorizationAccumulates(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"group":{"customers":{
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0/24"]},
		"peer":{"cust-a":{
			"connection":{"remote":{"ip":"198.51.100.1"}},
			"blackhole":{"authorized-covering-prefix":["203.0.113.0/24"]}
		}}
	}}}}`)

	cfg := cfgs[netip.MustParseAddr("198.51.100.1")]
	if len(cfg.authorized) != 2 {
		t.Fatalf("authorized = %v, want the group's and the peer's", cfg.authorized)
	}
}

// A peer that configures nothing gets no entry at all, so the honoring path
// costs an unconfigured deployment one map miss and no wire scan.
func TestParseBlackholeConfigSkipsPeersWithNoRule(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"peer":{"plain":{
		"connection":{"remote":{"ip":"198.51.100.1"}}
	}}}}`)
	if len(cfgs) != 0 {
		t.Errorf("got %d configs for a peer with no blackhole block, want 0", len(cfgs))
	}
}

// An unparseable authorized prefix is a refused config, not a silently dropped
// authorization. Dropping it would narrow what the operator authorized without
// telling them, and dropping the whole peer would widen nothing but would hide
// the typo.
func TestParseBlackholeConfigRejectsBadPrefix(t *testing.T) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(`{"bgp":{"peer":{"upstream":{
		"connection":{"remote":{"ip":"198.51.100.1"}},
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0/24","not-a-prefix"]}
	}}}}`), &tree); err != nil {
		t.Fatalf("bad test config: %v", err)
	}
	bgpCfg, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("test config has no bgp subtree")
	}
	if _, err := parseBlackholeConfig(bgpCfg); err == nil {
		t.Fatal("parseBlackholeConfig accepted an unparseable authorized prefix")
	}
}

// A bare address with no prefix length is refused too. "192.0.2.0" is a common
// operator slip, and reading it as a /32 would authorize one address where the
// operator meant a block.
func TestParseBlackholeConfigRejectsBareAddress(t *testing.T) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(`{"bgp":{"peer":{"upstream":{
		"connection":{"remote":{"ip":"198.51.100.1"}},
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0"]}
	}}}}`), &tree); err != nil {
		t.Fatalf("bad test config: %v", err)
	}
	bgpCfg, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("test config has no bgp subtree")
	}
	if _, err := parseBlackholeConfig(bgpCfg); err == nil {
		t.Fatal("parseBlackholeConfig accepted a bare address as an authorization")
	}
}

// A peer whose remote IP is absent cannot be keyed, and the honoring path looks
// up by address. Keeping it under a name the lookup never uses would be a
// configuration that reads as in force and does nothing.
func TestParseBlackholeConfigRefusesPeerWithNoRemoteIP(t *testing.T) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(`{"bgp":{"peer":{"upstream":{
		"blackhole":{"community":["blackhole"],"authorized-covering-prefix":["192.0.2.0/24"]}
	}}}}`), &tree); err != nil {
		t.Fatalf("bad test config: %v", err)
	}
	bgpCfg, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatal("test config has no bgp subtree")
	}
	if _, err := parseBlackholeConfig(bgpCfg); err == nil {
		t.Fatal("parseBlackholeConfig accepted a blackhole block on a peer with no remote IP")
	}
}
