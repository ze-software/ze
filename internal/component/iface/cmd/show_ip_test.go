package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestShowIP_RegisteredWireMethods verifies the `ze-show:ip-arp` and
// `ze-show:ip-route` RPCs are installed in the builtin registry so the
// dispatcher can route `ze show ip arp` / `ze show ip route` to their
// handlers.
func TestShowIP_RegisteredWireMethods(t *testing.T) {
	wanted := map[string]bool{
		"ze-show:ip-arp":   false,
		"ze-show:ip-route": false,
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

// TestHandleShowArp_UnknownFamilyRejects verifies the handler rejects an
// invalid --family value with the valid-set in the error message.
func TestHandleShowArp_UnknownFamilyRejects(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"--family", "ipv5"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "ipv5")
	assert.Contains(t, msg, "ipv4")
	assert.Contains(t, msg, "ipv6")
}

// TestHandleShowArp_FamilyRequiresValue verifies --family without an
// argument rejects.
func TestHandleShowArp_FamilyRequiresValue(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"--family"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "requires a value")
}

// TestHandleShowIPRoute_DispatchShape verifies the handler dispatches to
// the backend and wraps the result under the `routes` key.
func TestHandleShowIPRoute_DispatchShape(t *testing.T) {
	resp, err := handleShowIPRoute(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Status == plugin.StatusError {
		msg := resp.Error
		assert.True(t,
			strings.Contains(msg, "no backend") ||
				strings.Contains(msg, "ListKernelRoutes") ||
				strings.Contains(msg, "route"),
			"error should name the missing capability: %q", msg,
		)
		return
	}

	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	_, hasRoutes := data["routes"]
	assert.True(t, hasRoutes, "response should carry the routes key")
}

// TestHandleShowArp_UnknownPositional verifies the handler rejects an
// unknown positional arg rather than silently returning the full
// neighbor table.
func TestHandleShowArp_UnknownPositional(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"eth0"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "unknown argument")
}

// TestHandleShowArp_FamilyRepeatRejects verifies --family given twice
// rejects rather than last-wins.
func TestHandleShowArp_FamilyRepeatRejects(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"--family", "ipv4", "--family", "ipv6"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg := resp.Error
	assert.Contains(t, msg, "more than once")
}

// TestHandleShowIPRoute_InvalidPrefixRejects verifies CIDR validation
// at the handler level.
func TestHandleShowIPRoute_InvalidPrefixRejects(t *testing.T) {
	bad := []string{
		"10.0.0.0",    // no mask
		"10.0.0.0/33", // invalid IPv4 mask
		"::/129",      // invalid IPv6 mask
		"not-a-cidr",
	}
	for _, arg := range bad {
		resp, err := handleShowIPRoute(nil, []string{arg})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status, "arg=%q should reject", arg)
		msg := resp.Error
		assert.Contains(t, msg, "invalid prefix", "arg=%q", arg)
	}

	// "default" is the documented synonym -- MUST NOT reject at parse time.
	resp, err := handleShowIPRoute(nil, []string{"default"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Backend may or may not be loaded; only assert we did not stop at prefix-validation.
	if resp.Status == plugin.StatusError {
		msg := resp.Error
		assert.NotContains(t, msg, "invalid prefix", "default should not be rejected as bad CIDR")
	}
}

// TestHandleShowIPRoute_LimitParsing verifies --limit is parsed and
// rejects non-positive or non-numeric values.
func TestHandleShowIPRoute_LimitParsing(t *testing.T) {
	bad := [][]string{
		{"--limit"},        // missing value
		{"--limit", "0"},   // zero
		{"--limit", "-5"},  // negative
		{"--limit", "abc"}, // not a number
	}
	for _, args := range bad {
		resp, err := handleShowIPRoute(nil, args)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status, "args=%v should reject", args)
	}
}

// TestHandleShowArp_DispatchShape verifies the handler dispatches to the
// backend and wraps the result under the `neighbors` key.
func TestHandleShowArp_DispatchShape(t *testing.T) {
	resp, err := handleShowArp(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Status == plugin.StatusError {
		msg := resp.Error
		assert.True(t,
			strings.Contains(msg, "no backend") || strings.Contains(msg, "ListNeighbors") || strings.Contains(msg, "neigh"),
			"error should name the missing capability: %q", msg,
		)
		return
	}

	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	_, hasNeighbors := data["neighbors"]
	assert.True(t, hasNeighbors, "response should carry the neighbors key")
}
