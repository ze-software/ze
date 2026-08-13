package configjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseBGPSubtree_Wrapped verifies extraction from {"bgp": {...}}.
//
// VALIDATES: ParseBGPSubtree returns the bgp subtree from a wrapped JSON object.
// PREVENTS: Misparse when bgp key is present.
func TestParseBGPSubtree_Wrapped(t *testing.T) {
	input := `{"bgp":{"router-id":"10.0.0.1","peer":{"192.0.2.1":{}}}}`

	bgp, ok := ParseBGPSubtree(input)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", bgp["router-id"])
	assert.NotNil(t, bgp["peer"])
}

// TestParseBGPSubtree_Bare verifies extraction from bare {...} JSON.
//
// VALIDATES: ParseBGPSubtree returns the root as-is when no "bgp" key.
// PREVENTS: Failure on bare config objects (e.g., peer-level subtree).
func TestParseBGPSubtree_Bare(t *testing.T) {
	input := `{"router-id":"10.0.0.1","peer":{"192.0.2.1":{}}}`

	bgp, ok := ParseBGPSubtree(input)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", bgp["router-id"])
}

// TestParseBGPSubtree_InvalidJSON verifies invalid JSON returns false.
//
// VALIDATES: ParseBGPSubtree fails gracefully on malformed input.
// PREVENTS: Panic on bad JSON.
func TestParseBGPSubtree_InvalidJSON(t *testing.T) {
	_, ok := ParseBGPSubtree(`{invalid`)
	assert.False(t, ok)
}

// TestParseBGPSubtree_Empty verifies empty object returns the empty map.
//
// VALIDATES: ParseBGPSubtree handles empty objects.
// PREVENTS: False negative on empty config.
func TestParseBGPSubtree_Empty(t *testing.T) {
	bgp, ok := ParseBGPSubtree(`{}`)
	require.True(t, ok)
	assert.Empty(t, bgp)
}

// TestForEachPeer_StandalonePeers verifies iteration over standalone peers.
//
// VALIDATES: ForEachPeer visits peers under bgpTree["peer"].
// PREVENTS: Standalone peers being skipped.
func TestForEachPeer_StandalonePeers(t *testing.T) {
	bgpTree := map[string]any{
		"peer": map[string]any{
			"192.0.2.1": map[string]any{"session": map[string]any{"asn": map[string]any{"remote": float64(65001)}}},
			"192.0.2.2": map[string]any{"session": map[string]any{"asn": map[string]any{"remote": float64(65002)}}},
		},
	}

	visited := make(map[string]map[string]any)
	ForEachPeer(bgpTree, func(addr string, peerMap, groupMap map[string]any, origin PeerOrigin) {
		assert.Nil(t, groupMap, "standalone peers should have nil groupMap")
		assert.Equal(t, PeerOrigin{}, origin, "a standalone peer belongs to no group")
		visited[addr] = peerMap
	})

	require.Len(t, visited, 2)
	assert.Contains(t, visited, "192.0.2.1")
	assert.Contains(t, visited, "192.0.2.2")
}

// TestForEachPeer_GroupedPeers verifies iteration over peers within groups.
//
// VALIDATES: ForEachPeer visits peers under bgpTree["group"][name]["peer"].
// PREVENTS: Grouped peers being missed.
func TestForEachPeer_GroupedPeers(t *testing.T) {
	bgpTree := map[string]any{
		"group": map[string]any{
			"transit": map[string]any{
				"session": map[string]any{"asn": map[string]any{"local": float64(65000)}},
				"peer": map[string]any{
					"192.0.2.1": map[string]any{},
				},
			},
			"peers": map[string]any{
				"peer": map[string]any{
					"192.0.2.2": map[string]any{},
				},
			},
		},
	}

	type visit struct {
		addr     string
		hasGroup bool
	}
	var visits []visit
	ForEachPeer(bgpTree, func(addr string, _, groupMap map[string]any, _ PeerOrigin) {
		visits = append(visits, visit{addr: addr, hasGroup: groupMap != nil})
	})

	require.Len(t, visits, 2)
	for _, v := range visits {
		assert.True(t, v.hasGroup, "grouped peers should have non-nil groupMap for %s", v.addr)
	}
}

// TestForEachPeer_MixedStandaloneAndGrouped verifies both paths in one tree.
//
// VALIDATES: ForEachPeer visits standalone and grouped peers together.
// PREVENTS: One path shadowing the other.
func TestForEachPeer_MixedStandaloneAndGrouped(t *testing.T) {
	bgpTree := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{},
		},
		"group": map[string]any{
			"transit": map[string]any{
				"peer": map[string]any{
					"10.0.0.2": map[string]any{},
				},
			},
		},
	}

	addrs := make(map[string]bool)
	ForEachPeer(bgpTree, func(addr string, _, _ map[string]any, _ PeerOrigin) {
		addrs[addr] = true
	})

	assert.Len(t, addrs, 2)
	assert.True(t, addrs["10.0.0.1"])
	assert.True(t, addrs["10.0.0.2"])
}

// TestForEachPeer_NilPeerMap verifies peers with no config fields.
//
// VALIDATES: ForEachPeer handles nil/non-map peer entries gracefully.
// PREVENTS: Panic on peer entries that are not maps.
func TestForEachPeer_NilPeerMap(t *testing.T) {
	bgpTree := map[string]any{
		"peer": map[string]any{
			"192.0.2.1": nil, // No config
		},
	}

	var visited bool
	ForEachPeer(bgpTree, func(addr string, peerMap, _ map[string]any, _ PeerOrigin) {
		assert.Equal(t, "192.0.2.1", addr)
		assert.Nil(t, peerMap)
		visited = true
	})
	assert.True(t, visited)
}

// TestForEachPeer_EmptyTree verifies no visits on empty tree.
//
// VALIDATES: ForEachPeer handles empty/missing peer and group keys.
// PREVENTS: Panic on empty config.
func TestForEachPeer_EmptyTree(t *testing.T) {
	var count int
	ForEachPeer(map[string]any{}, func(_ string, _, _ map[string]any, _ PeerOrigin) {
		count++
	})
	assert.Zero(t, count)
}

// TestPeerRemoteIP verifies remote-IP extraction from connection>remote>ip.
//
// VALIDATES: PeerRemoteIP reads the nested connection>remote>ip path, peer wins
// over group, and returns "" when absent (AC-11).
// PREVENTS: Regression to the stale flat remote/ip path that silently returns "".
func TestPeerRemoteIP(t *testing.T) {
	peerWithIP := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
	}
	groupWithIP := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "198.51.100.9"}},
	}

	// Peer's own remote IP.
	assert.Equal(t, "192.0.2.1", PeerRemoteIP(peerWithIP, nil))
	// Peer wins over group.
	assert.Equal(t, "192.0.2.1", PeerRemoteIP(peerWithIP, groupWithIP))
	// Falls back to group when peer lacks it.
	assert.Equal(t, "198.51.100.9", PeerRemoteIP(map[string]any{}, groupWithIP))
	// Empty when neither has it.
	assert.Equal(t, "", PeerRemoteIP(map[string]any{}, nil))
	// The flat remote/ip path (role's old bug) is NOT read.
	assert.Equal(t, "", PeerRemoteIP(map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}}, nil))
	// Nil-safe.
	assert.Equal(t, "", PeerRemoteIP(nil, nil))
}

// TestGetCapability_Present verifies capability extraction from session config.
//
// VALIDATES: GetCapability navigates session.capability correctly.
// PREVENTS: Wrong path for capability lookup.
func TestGetCapability_Present(t *testing.T) {
	m := map[string]any{
		"session": map[string]any{
			"capability": map[string]any{
				"route-refresh":    true,
				"graceful-restart": map[string]any{"time": float64(120)},
			},
		},
	}

	caps := GetCapability(m)
	require.NotNil(t, caps)
	assert.Equal(t, true, caps["route-refresh"])
	gr, ok := caps["graceful-restart"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(120), gr["time"])
}

// TestGetCapability_NoSession verifies nil return when session is absent.
//
// VALIDATES: GetCapability returns nil when session key is missing.
// PREVENTS: Panic on config without session block.
func TestGetCapability_NoSession(t *testing.T) {
	m := map[string]any{"connection": map[string]any{}}
	assert.Nil(t, GetCapability(m))
}

// TestGetCapability_NoCapability verifies nil return when capability is absent.
//
// VALIDATES: GetCapability returns nil when session exists but capability does not.
// PREVENTS: False positive on session-only config.
func TestGetCapability_NoCapability(t *testing.T) {
	m := map[string]any{
		"session": map[string]any{"asn": map[string]any{"local": float64(65000)}},
	}
	assert.Nil(t, GetCapability(m))
}

// TestGetCapability_NilInput verifies nil return on nil input.
//
// VALIDATES: GetCapability handles nil map gracefully.
// PREVENTS: Panic on nil.
func TestGetCapability_NilInput(t *testing.T) {
	assert.Nil(t, GetCapability(nil))
}

// dynamicGroupTree returns a bgp subtree holding one dynamic group named "ix",
// shaped as the operator writes it: `ip dynamic` plus a range, and no peer list.
// It is the route-server topology test/reload/reload-dynamic-peer-survives.ci runs.
func dynamicGroupTree() map[string]any {
	return map[string]any{
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{
						"ip":      "dynamic",
						"connect": false,
						"range":   []any{"192.0.2.0/24"},
					},
				},
				"role": map[string]any{"import": "rs"},
			},
		},
	}
}

// TestForEachPeerVisitsDynamicGroupTemplate verifies AC-1.
//
// VALIDATES: a dynamic group yields exactly one visit, carrying the group's name,
// the group's config map, a nil peer map, and origin.Template.
// PREVENTS: the whole defect. A dynamic group states a role, an RPKI action or a
// blackhole block and no plugin ever sees it, because the group carries no `peer`
// list and the traversal skipped it. An IXP route server got no enforcement and no
// error.
func TestForEachPeerVisitsDynamicGroupTemplate(t *testing.T) {
	type visit struct {
		name     string
		peerNil  bool
		groupNil bool
		origin   PeerOrigin
	}
	var visits []visit
	ForEachPeer(dynamicGroupTree(), func(name string, peerMap, groupMap map[string]any, origin PeerOrigin) {
		visits = append(visits, visit{name, peerMap == nil, groupMap == nil, origin})
	})

	require.Len(t, visits, 1, "a dynamic group must yield its template exactly once")
	assert.Equal(t, "ix", visits[0].name)
	assert.True(t, visits[0].peerNil, "a template visit carries no peer map")
	assert.False(t, visits[0].groupNil, "a template visit carries the group's map")
	assert.Equal(t, PeerOrigin{Group: "ix", Template: true}, visits[0].origin)
}

// TestForEachPeerVisitsStaticPeersOfADynamicGroup verifies AC-2.
//
// VALIDATES: a dynamic group that also lists static peers yields both, and the
// static peer keeps the name, maps and group it gets today.
// PREVENTS: the template visit replacing the group's static peers, which would
// trade one silent loss for another.
func TestForEachPeerVisitsStaticPeersOfADynamicGroup(t *testing.T) {
	tree := dynamicGroupTree()
	groups, ok := tree["group"].(map[string]any)
	require.True(t, ok)
	group, ok := groups["ix"].(map[string]any)
	require.True(t, ok)
	group["peer"] = map[string]any{
		"named": map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.7"}},
		},
	}

	origins := make(map[string]PeerOrigin)
	ForEachPeer(tree, func(name string, _, groupMap map[string]any, origin PeerOrigin) {
		origins[name] = origin
		assert.NotNil(t, groupMap, "every visit inside a group carries the group map")
	})

	require.Len(t, origins, 2)
	assert.Equal(t, PeerOrigin{Group: "ix", Template: true}, origins["ix"])
	assert.Equal(t, PeerOrigin{Group: "ix"}, origins["named"],
		"a static peer of a dynamic group is a configured peer, not a template")
}

// TestForEachPeerSkipsATemplateVisitForAPlainGroup verifies AC-3.
//
// VALIDATES: a group whose remote ip is not "dynamic" yields no template visit.
// PREVENTS: every ordinary peer-group gaining a phantom peer, which would give each
// plugin a config entry keyed by a group whose members are all named already.
func TestForEachPeerSkipsATemplateVisitForAPlainGroup(t *testing.T) {
	tree := map[string]any{
		"group": map[string]any{
			"transit": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
				"peer":       map[string]any{"192.0.2.1": map[string]any{}},
			},
		},
	}

	var templates int
	var peers int
	ForEachPeer(tree, func(_ string, _, _ map[string]any, origin PeerOrigin) {
		if origin.Template {
			templates++
			return
		}
		peers++
	})

	assert.Zero(t, templates, "a group with a literal remote ip is not dynamic")
	assert.Equal(t, 1, peers)
}

// TestIsDynamicGroupMatchesTheConfigResolver verifies the marker.
//
// VALIDATES: IsDynamicGroup keys on connection > remote > ip == "dynamic" and on
// nothing else, matching config.isDynamicGroup.
// PREVENTS: the traversal and the reactor disagreeing about which groups are
// dynamic. The reactor would build peers from a template no plugin delivered config
// for, which is the defect this spec fixes, restored by a second definition.
func TestIsDynamicGroupMatchesTheConfigResolver(t *testing.T) {
	withIP := func(ip any) map[string]any {
		return map[string]any{"connection": map[string]any{"remote": map[string]any{"ip": ip}}}
	}

	assert.True(t, IsDynamicGroup(withIP("dynamic")))
	assert.False(t, IsDynamicGroup(withIP("192.0.2.1")))
	assert.False(t, IsDynamicGroup(withIP(nil)), "a non-string ip is not the placeholder")
	assert.False(t, IsDynamicGroup(nil))
	assert.False(t, IsDynamicGroup(map[string]any{}))
	assert.False(t, IsDynamicGroup(map[string]any{"connection": map[string]any{}}),
		"a range alone does not make a group dynamic; the placeholder does")
}

// TestPeerConfigKeySeparatesAGroupFromAPeerOfTheSameName verifies A-7.
//
// VALIDATES: a group named "ix" and a peer named "ix" produce different keys.
// PREVENTS: a dynamic group's template answering a lookup meant for a peer.
// config.ResolveBGPTree collects every peer name into one uniqueness map, but a
// group name only goes through validateGroupName and is never compared against it,
// so `bgp { peer ix {...} group ix {...} }` is accepted. A bare string key would
// make the two indistinguishable to every reader.
func TestPeerConfigKeySeparatesAGroupFromAPeerOfTheSameName(t *testing.T) {
	peerMap := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
	}

	peerKey, ok := KeyFor("ix", peerMap, nil, PeerOrigin{})
	require.True(t, ok)
	groupKey, ok := KeyFor("ix", nil, map[string]any{}, PeerOrigin{Group: "ix", Template: true})
	require.True(t, ok)

	assert.NotEqual(t, peerKey, groupKey)

	// The sharper case: a peer whose NAME is the key it is stored under, because it
	// states no remote ip. Its id is then identical to the group's.
	named, ok := KeyFor("192.0.2.1", nil, nil, PeerOrigin{})
	require.True(t, ok)
	collide, ok := KeyFor("192.0.2.1", nil, map[string]any{}, PeerOrigin{Group: "192.0.2.1", Template: true})
	require.True(t, ok)
	assert.Equal(t, named.ID, collide.ID, "the ids really are equal")
	assert.NotEqual(t, named, collide, "and the keys still are not")
}

// TestPeerKeyRefusesAKeyNoReaderCanProduce verifies the miss is visible.
//
// VALIDATES: PeerKey reports ok=false for the "dynamic" placeholder and for a name
// that is not an address, rather than returning a key nothing looks up.
// PREVENTS: the zero-value trap. A config stored under an unreachable key reads to
// the operator as in force and does nothing, and the reader cannot tell that miss
// from "this peer has nothing configured", which is the permissive branch.
func TestPeerKeyRefusesAKeyNoReaderCanProduce(t *testing.T) {
	_, ok := PeerKey("ix", nil, map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "dynamic"}},
	})
	assert.False(t, ok, "the group placeholder is not an address")

	_, ok = PeerKey("upstream", nil, nil)
	assert.False(t, ok, "a name that is not an address is not a key any reader produces")

	key, ok := PeerKey("upstream", map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
	}, nil)
	require.True(t, ok)
	assert.Equal(t, PeerConfigKey{ID: "192.0.2.1"}, key)

	key, ok = PeerKey("192.0.2.2", nil, nil)
	require.True(t, ok)
	assert.Equal(t, PeerConfigKey{ID: "192.0.2.2"}, key, "a peer named by its own address")
}

// TestLookupPeerConfigPrefersThePeerOverItsGroup verifies AC-9.
//
// VALIDATES: the lookup order is address, then name, then group, so what a peer
// states beats what its group states, and a miss is reported rather than returned
// as a zero value.
// PREVENTS: a group's template overriding a peer's own configuration, which would
// invert the config precedence every other BGP leaf follows.
func TestLookupPeerConfigPrefersThePeerOverItsGroup(t *testing.T) {
	m := map[PeerConfigKey]string{
		{ID: "192.0.2.1"}: "by-address",
		{ID: "named"}:     "by-name",
		GroupKey("ix"):    "by-group",
	}

	got, ok := LookupPeerConfig(m, "192.0.2.1", "named", "ix")
	require.True(t, ok)
	assert.Equal(t, "by-address", got)

	got, ok = LookupPeerConfig(m, "198.51.100.9", "named", "ix")
	require.True(t, ok)
	assert.Equal(t, "by-name", got, "the name answers when the address does not")

	// A dynamic peer: its address is the one it connected from and its name is
	// "dyn-<addr>", so neither appears in the config. Only the group answers.
	got, ok = LookupPeerConfig(m, "192.0.2.55", "dyn-192.0.2.55", "ix")
	require.True(t, ok)
	assert.Equal(t, "by-group", got)

	_, ok = LookupPeerConfig(m, "192.0.2.55", "dyn-192.0.2.55", "other")
	assert.False(t, ok, "a miss must be reported, never returned as a zero value")

	_, ok = LookupPeerConfig(m, "", "", "")
	assert.False(t, ok, "an empty identity matches nothing, including an empty key")
}

// TestCapabilitySelectorSeparatesAGroupFromAPeerOfTheSameName covers the one key
// PeerConfigKey cannot type: rpc.CapabilityDecl.Peers is a []string crossing the
// plugin process boundary, so a group and a peer share one string space there.
//
// VALIDATES: the group selector is distinct from every peer selector, and the ":"
// that keeps them apart cannot occur in either name -- naming.ValidateNodeName
// accepts only alphanumerics, "_", "-" and ".".
// PREVENTS: two plugins declaring the same capability code under the same selector
// for a peer named "ix" and a group named "ix". plugin.AddPluginCapabilities reads
// that as a conflict and refuses the whole configuration at startup, on a config
// that loads today (A-7: nothing compares the two namespaces).
func TestCapabilitySelectorSeparatesAGroupFromAPeerOfTheSameName(t *testing.T) {
	peer := CapabilitySelector("ix", PeerOrigin{})
	group := CapabilitySelector("ix", PeerOrigin{Group: "ix", Template: true})

	assert.Equal(t, "ix", peer, "a configured peer keeps its own name as the selector")
	assert.NotEqual(t, peer, group)
	assert.Equal(t, CapabilityGroupKey("ix"), group)
	assert.Contains(t, group, ":", "the separator is what makes the two spaces disjoint")

	// A peer inside a dynamic group is a configured peer, not the template, so it
	// selects by its own name. Only the template visit carries Template.
	member := CapabilitySelector("member", PeerOrigin{Group: "ix"})
	assert.Equal(t, "member", member)
}
