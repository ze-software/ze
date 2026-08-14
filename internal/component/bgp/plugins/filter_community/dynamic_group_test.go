// VALIDATES: a dynamic group's community filter config is carried under a key of
// its own, and a peer sharing the group's name keeps its own.
// PREVENTS: the group's template overwriting that peer's entry. Groups are
// visited after standalone peers, so the last writer won and the peer silently
// received another object's community policy on every ingress and egress
// decision.

package filter_community

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// withCleanConfig restores the package-level filter state after one test.
func withCleanConfig(t *testing.T) {
	t.Helper()
	prevPeers, prevDefs := peerConfigs, definitions
	t.Cleanup(func() {
		mu.Lock()
		peerConfigs, definitions = prevPeers, prevDefs
		mu.Unlock()
	})
}

// A group name and a peer name share no uniqueness check: config.ResolveBGPTree
// refuses a duplicate PEER name and never compares a group's against it, so
// `bgp { peer ix {...} group ix {...} }` loads. The two must therefore occupy
// separate keys by construction, not by convention.
func TestDynamicGroupTemplateDoesNotOverwriteAPeerOfTheSameName(t *testing.T) {
	withCleanConfig(t)

	err := configureCommunityFilter(map[string]any{
		"community": map[string]any{
			"standard": map[string]any{
				"peer-tag":  map[string]any{"value": []any{"65001:1"}},
				"group-tag": map[string]any{"value": []any{"65001:2"}},
			},
		},
		"peer": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}},
				"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"tag": []any{"peer-tag"}}}},
			},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"tag": []any{"group-tag"}}}},
			},
		},
	})
	require.NoError(t, err)

	mu.RLock()
	got := peerConfigs
	mu.RUnlock()

	peer, ok := got["ix"]
	require.True(t, ok, "the peer lost its entry, got keys %v", keysOf(got))
	assert.Equal(t, []string{"peer-tag"}, peer.ingressTag,
		"the group's template overwrote the peer of the same name")

	tmpl, ok := got[configjson.CapabilityGroupKey("ix")]
	require.True(t, ok, "the dynamic group's template is not carried, got keys %v", keysOf(got))
	assert.Equal(t, []string{"group-tag"}, tmpl.ingressTag)
}

// An error about a template must name a group. "peer group:ix" sends the
// operator looking for a peer that does not exist.
func TestConfigErrorNamesAGroupAsAGroup(t *testing.T) {
	withCleanConfig(t)

	err := configureCommunityFilter(map[string]any{
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"tag": []any{"undefined-name"}}}},
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group ix")
	assert.NotContains(t, err.Error(), "peer group:")
}

func keysOf(m map[string]filterConfig) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// memberOf describes a peer the reactor created from a listen-range group. Its
// name appears nowhere in the config document, and the group's name is the one
// identity it shares with what the operator wrote.
func memberOf(group, name string) filterapi.PeerFilterInfo {
	return filterapi.PeerFilterInfo{
		Name:      name,
		GroupName: group,
		Address:   netip.MustParseAddr("192.0.2.50"),
		PeerAS:    64511,
		LocalAS:   65000,
	}
}

// taggedPayload is an UPDATE body carrying ORIGIN and one community, the input
// the ingress filter tags on to.
func taggedPayload() []byte {
	return buildPayload(append(buildOriginAttr(), buildCommunityAttr(0x0001_0001)...))
}

// AC-6 at filter-community: a peer built from a listen-range group has no entry
// of its own, so the group's template is the only community policy it can
// resolve. Before the fallback the member matched nothing and every ingress
// decision returned the permissive answer.
func TestIngressFilterFallsBackToTheGroupsConfig(t *testing.T) {
	withCleanConfig(t)

	require.NoError(t, configureCommunityFilter(map[string]any{
		"community": map[string]any{
			"standard": map[string]any{"group-tag": map[string]any{"value": []any{"65001:2"}}},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"tag": []any{"group-tag"}}}},
			},
		},
	}))

	_, modified := ingressFilter(memberOf("ix", "ix-192.0.2.50"), taggedPayload(), nil)
	require.NotNil(t, modified, "the member resolved no community policy from its group")
	assert.Equal(t, []uint32{0x0001_0001, 0xFDE9_0002}, extractCommunities(modified))

	// Negative control: a session outside the group states nothing and is left
	// alone, so the fallback is a group match rather than a match on anything.
	_, untouched := ingressFilter(memberOf("", "standalone"), taggedPayload(), nil)
	assert.Nil(t, untouched, "a peer in no group picked up a group's policy")
}

// AC-9: a peer that states its own leaves keeps its own entry, which already
// carries what its group states (the levels merge at parse time). Reversing the
// lookup order would give this member the group's template, which holds only the
// group's tag.
func TestCommunityConfigStillPrefersThePeerOverItsGroup(t *testing.T) {
	withCleanConfig(t)

	require.NoError(t, configureCommunityFilter(map[string]any{
		"community": map[string]any{
			"standard": map[string]any{
				"group-tag": map[string]any{"value": []any{"65001:2"}},
				"peer-tag":  map[string]any{"value": []any{"65001:1"}},
			},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"tag": []any{"group-tag"}}}},
				"peer": map[string]any{
					"member": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.9"}},
						"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"tag": []any{"peer-tag"}}}},
					},
				},
			},
		},
	}))

	_, modified := ingressFilter(memberOf("ix", "member"), taggedPayload(), nil)
	require.NotNil(t, modified)
	assert.Equal(t, []uint32{0x0001_0001, 0xFDE9_0002, 0xFDE9_0001}, extractCommunities(modified),
		"the named peer read its group's template instead of its own entry")
}

// AC-8: one config states the same community leaves on a static peer and on a
// dynamic group. Both must resolve to the same effective policy, so the template
// diverges from a static peer nowhere it was not told to.
func TestDynamicGroupCommunityConfigMatchesAStaticPeer(t *testing.T) {
	withCleanConfig(t)

	leaves := func() map[string]any {
		return map[string]any{
			"ingress": map[string]any{"community": map[string]any{
				"tag":   []any{"tag-a"},
				"strip": []any{"tag-b"},
			}},
			"egress": map[string]any{"community": map[string]any{"tag": []any{"tag-b"}}},
		}
	}
	require.NoError(t, configureCommunityFilter(map[string]any{
		"community": map[string]any{
			"standard": map[string]any{
				"tag-a": map[string]any{"value": []any{"65001:1"}},
				"tag-b": map[string]any{"value": []any{"65001:2"}},
			},
		},
		"peer": map[string]any{
			"static": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "198.51.100.1"}},
				"filter":     leaves(),
			},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     leaves(),
			},
		},
	}))

	mu.RLock()
	staticCfg, staticOK := lookupPeerConfigLocked("static", "")
	memberCfg, memberOK := lookupPeerConfigLocked("ix-192.0.2.50", "ix")
	mu.RUnlock()
	require.True(t, staticOK)
	require.True(t, memberOK)
	assert.Equal(t, staticCfg, memberCfg, "the template resolved a different policy than the static peer")

	// The same answer through the registered filter, which is where an operator
	// would see a divergence.
	_, fromStatic := ingressFilter(filterapi.PeerFilterInfo{Name: "static", PeerAS: 64511, LocalAS: 65000}, taggedPayload(), nil)
	_, fromMember := ingressFilter(memberOf("ix", "ix-192.0.2.50"), taggedPayload(), nil)
	assert.Equal(t, fromStatic, fromMember)
}

// The egress reader resolves the destination peer, so it needs the same
// fallback: a member of a listen-range group is a destination as often as it is
// a source.
func TestEgressFilterFallsBackToTheGroupsConfig(t *testing.T) {
	withCleanConfig(t)

	require.NoError(t, configureCommunityFilter(map[string]any{
		"community": map[string]any{
			"standard": map[string]any{"group-tag": map[string]any{"value": []any{"65001:2"}}},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     map[string]any{"egress": map[string]any{"community": map[string]any{"tag": []any{"group-tag"}}}},
			},
		},
	}))

	var mods filterapi.ModAccumulator
	require.True(t, egressFilter(filterapi.PeerFilterInfo{}, memberOf("ix", "ix-192.0.2.50"), nil, nil, &mods))
	assert.True(t, mods.HasModifications(), "the destination member resolved no egress policy from its group")

	var none filterapi.ModAccumulator
	require.True(t, egressFilter(filterapi.PeerFilterInfo{}, memberOf("", "standalone"), nil, nil, &none))
	assert.False(t, none.HasModifications(), "a destination in no group picked up a group's policy")
}

// The RFC 8195 relation tag is the third reader of the same map, and it is
// stated on the group for the same reason: an IXP writes its member policy
// once, on the listen-range group.
func TestRelationIngressFilterFallsBackToTheGroupsConfig(t *testing.T) {
	withCleanConfig(t)

	require.NoError(t, configureCommunityFilter(map[string]any{
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []any{"192.0.2.0/24"}}},
				"filter":     map[string]any{"ingress": map[string]any{"community": map[string]any{"relation-tag": "true"}}},
			},
		},
	}))

	meta := map[string]any{"src-peer-role": "customer"}
	_, modified := relationIngressFilter(memberOf("ix", "ix-192.0.2.50"), taggedPayload(), meta)
	assert.NotNil(t, modified, "the member resolved no relation tag from its group")
}
