package configjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexByKey walks one bgp subtree the way every ForEachPeer caller does: it
// stores each visit's effective config under the key KeyFor returns, so a runtime
// reader finds it from an identity it holds. leaf names the plugin container the
// caller reads (role, rpki, blackhole), and a peer's own statement wins over its
// group's, which is the bgp -> group -> peer order PeerRemoteIP already applies.
//
// It is deliberately the whole of what a caller does with a visit. The tests below
// are about the delivery layer, so they assert what ForEachPeer, KeyFor and
// LookupPeerConfig produce together, never what a plugin later decides with it.
func indexByKey(t *testing.T, tree map[string]any, leaf string) map[PeerConfigKey]map[string]any {
	t.Helper()
	index := make(map[PeerConfigKey]map[string]any)
	ForEachPeer(tree, func(name string, peerMap, groupMap map[string]any, origin PeerOrigin) {
		key, ok := KeyFor(name, peerMap, groupMap, origin)
		if !ok {
			return
		}
		for _, m := range []map[string]any{peerMap, groupMap} {
			if cfg, stated := m[leaf].(map[string]any); stated {
				index[key] = cfg
				return
			}
		}
	})
	return index
}

// TestDynamicGroupPluginConfigMatchesAStaticPeer verifies AC-8 at the shared
// config delivery layer.
//
// VALIDATES: one config states the same plugin leaves on a static peer and on a
// dynamic group. Both are keyed reachably, and the config a member of the group
// resolves is the config the static peer resolves, field for field.
// PREVENTS: a dynamic group's template diverging from a static peer where the
// operator stated no difference. The member's own address and its generated name
// ("dyn-<addr>", reactor.buildDynamicPeerSettings) appear nowhere in the document,
// so the group name is the only identity that can answer for it. Before the
// template visit existed, nothing answered at all: the operator got no error and
// no enforcement.
func TestDynamicGroupPluginConfigMatchesAStaticPeer(t *testing.T) {
	tree := map[string]any{
		"peer": map[string]any{
			"transit-one": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
				"role":       map[string]any{"import": "rs", "strict": true},
			},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{
					"ip":    "dynamic",
					"range": []any{"198.51.100.0/24"},
				}},
				"role": map[string]any{"import": "rs", "strict": true},
			},
		},
	}

	index := indexByKey(t, tree, "role")

	// The static peer, found from the address every filter chain passes
	// (filterapi.PeerFilterInfo.Address).
	static, ok := LookupPeerConfig(index, "192.0.2.1", "transit-one", "")
	require.True(t, ok, "a configured peer resolves from its remote address")

	// A member of the group, found from PeerFilterInfo.GroupName alone.
	member, ok := LookupPeerConfig(index, "198.51.100.7", "dyn-198.51.100.7", "ix")
	require.True(t, ok, "a member of a dynamic group resolves its group's config")

	assert.Equal(t, static, member,
		"the same statement on a group and on a peer must deliver the same config")
	assert.Equal(t, map[string]any{"import": "rs", "strict": true}, member,
		"the delivered config is what the operator stated, not an empty map two misses would also match")
}

// TestAPeerInsideADynamicGroupKeepsItsOwnConfig verifies AC-9 at the delivery
// layer.
//
// VALIDATES: a peer listed inside a dynamic group is keyed under its own address,
// so its own statement is what a reader finds, while the group's template keeps a
// separate key and still answers for the members built from it.
// PREVENTS: the template visit answering for a peer that stated something else.
// The two configurations are both in force, and an IXP that overrides one named
// member must keep that override.
func TestAPeerInsideADynamicGroupKeepsItsOwnConfig(t *testing.T) {
	tree := map[string]any{
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{
					"ip":    "dynamic",
					"range": []any{"198.51.100.0/24"},
				}},
				"role": map[string]any{"import": "rs"},
				"peer": map[string]any{
					"well-known": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": "198.51.100.2"}},
						"role":       map[string]any{"import": "peer"},
					},
				},
			},
		},
	}

	index := indexByKey(t, tree, "role")
	require.Len(t, index, 2, "the template and the named peer are two configurations, both in force")

	named, ok := LookupPeerConfig(index, "198.51.100.2", "well-known", "ix")
	require.True(t, ok)
	assert.Equal(t, map[string]any{"import": "peer"}, named,
		"a peer's own statement still beats its group's")

	member, ok := LookupPeerConfig(index, "198.51.100.9", "dyn-198.51.100.9", "ix")
	require.True(t, ok)
	assert.Equal(t, map[string]any{"import": "rs"}, member,
		"a peer built from the template still gets what the group states")
}
