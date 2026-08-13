// VALIDATES: a dynamic group's community filter config is carried under a key of
// its own, and a peer sharing the group's name keeps its own.
// PREVENTS: the group's template overwriting that peer's entry. Groups are
// visited after standalone peers, so the last writer won and the peer silently
// received another object's community policy on every ingress and egress
// decision.

package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
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
