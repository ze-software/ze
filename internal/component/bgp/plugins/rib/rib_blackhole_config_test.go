// VALIDATES: the per-peer RFC 7999 leaves resolve with the bgp, group and peer
// inheritance every other BGP plugin leaf uses, and key on the peer's remote IP
// rather than on its config name.
// PREVENTS: a group-level agreement silently canceled by every peer under it,
// and a honoring decision that cannot find its peer because the config keys by
// name while the RIB knows an address.

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"
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
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0/24","2001:db8::/32"]}
	}}}}`)

	addr := netip.MustParseAddr("198.51.100.1")
	cfg, ok := cfgs[addr]
	if !ok {
		t.Fatalf("no config for %v; got keys %v", addr, cfgs)
	}
	if !cfg.honor {
		t.Error("honor = false, want true")
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
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0/24"]},
		"peer":{"cust-a":{"connection":{"remote":{"ip":"198.51.100.1"}}}}
	}}}}`)

	cfg := cfgs[netip.MustParseAddr("198.51.100.1")]
	if !cfg.honor {
		t.Error("group-level honor did not reach the peer")
	}
	if len(cfg.authorized) != 1 {
		t.Errorf("group-level authorization did not reach the peer: %v", cfg.authorized)
	}
}

// A peer MUST be able to turn the group's agreement off. An unset leaf leaves
// the group value standing; an explicit false overrides it.
func TestParseBlackholeConfigPeerOverridesGroupOff(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"group":{"customers":{
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0/24"]},
		"peer":{"cust-a":{"connection":{"remote":{"ip":"198.51.100.1"}},"blackhole":{"honor":false}}}
	}}}}`)

	if cfgs[netip.MustParseAddr("198.51.100.1")].honor {
		t.Error("peer-level honor false did not override the group")
	}
}

// The authorization list is ze:cumulative, so a group statement and a peer
// statement ACCUMULATE rather than replacing each other. A replacing list would
// make a peer that adds one prefix silently drop the group's.
func TestParseBlackholeConfigAuthorizationAccumulates(t *testing.T) {
	cfgs := parseBlackholeJSON(t, `{"bgp":{"group":{"customers":{
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0/24"]},
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
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0/24","not-a-prefix"]}
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
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0"]}
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
		"blackhole":{"honor":true,"authorized-covering-prefix":["192.0.2.0/24"]}
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
