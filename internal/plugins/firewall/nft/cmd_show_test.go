// Design: plan/learned/786-backend-command-dispatch.md -- nft firewall show handler tests

package firewallnft

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// VALIDATES: wiring -- firewall show RPCs reachable via the dispatcher.
// PREVENTS: regression where the handler exists but no init() registers
// it, so `ze show firewall ...` returns "unknown command" at runtime.
func TestShowFirewall_RegisteredWireMethods(t *testing.T) {
	wanted := map[string]bool{
		"ze-show:firewall-ruleset": false,
		"ze-show:firewall-group":   false,
	}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		if _, ok := wanted[r.WireMethod]; ok {
			require.NotNil(t, r.Handler, "%s handler must not be nil", r.WireMethod)
			wanted[r.WireMethod] = true
		}
	}
	for wm, seen := range wanted {
		require.True(t, seen, "%s not registered via pluginserver.RegisterRPCs", wm)
	}
}

// VALIDATES: show firewall ruleset without a name is a usage error, not a silent full dump.
func TestHandleShowFirewallRuleset_MissingArg(t *testing.T) {
	resp, err := handleShowFirewallRuleset(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "usage")
}

// VALIDATES: reject clearly under exact-or-reject when the firewall plugin is idle.
// PREVENTS: handler returns an empty ruleset instead of saying "no backend configured".
func TestHandleShowFirewallRuleset_NoBackend(t *testing.T) {
	if firewall.GetBackend() != nil {
		if err := firewall.CloseBackend(); err != nil {
			t.Fatalf("close previous backend: %v", err)
		}
	}
	resp, err := handleShowFirewallRuleset(nil, []string{"wan"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "no backend")
}

// VALIDATES: consistent `groups: []` envelope when the daemon is idle.
func TestHandleShowFirewallGroup_Empty(t *testing.T) {
	firewall.StoreLastApplied(nil)

	resp, err := handleShowFirewallGroup(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	groups, ok := data["groups"].([]map[string]any)
	require.True(t, ok, "groups should be []map[string]any, got %T", data["groups"])
	assert.Empty(t, groups)
}

// VALIDATES: named lookup returns set elements; unknown name rejects with valid list.
// PREVENTS: handler returns an empty result for an unknown name rather than rejecting.
func TestHandleShowFirewallGroup_Lookup(t *testing.T) {
	firewall.StoreLastApplied([]firewall.Table{{
		Name:   "ze_wan",
		Family: firewall.FamilyInet,
		Sets: []firewall.Set{{
			Name: "allow-src",
			Type: firewall.SetTypeIPv4,
			Elements: []firewall.SetElement{
				{Value: "10.0.0.0/8"},
				{Value: "192.168.0.0/16"},
			},
		}},
	}})
	defer firewall.StoreLastApplied(nil)

	resp, err := handleShowFirewallGroup(nil, []string{"allow-src"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "allow-src", data["name"])
	tables, ok := data["tables"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, tables, 1)
	assert.Equal(t, "wan", tables[0]["table"])
	elems, ok := tables[0]["elements"].([]string)
	require.True(t, ok)
	assert.Contains(t, elems, "10.0.0.0/8")
	assert.Contains(t, elems, "192.168.0.0/16")

	resp, err = handleShowFirewallGroup(nil, []string{"nonexistent"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "nonexistent")
	assert.Contains(t, msg, "allow-src")
}
