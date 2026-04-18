package show

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
//
// VALIDATES: AC (op-0 commands 4 and 2) wiring -- the commands are
// reachable via the dispatcher, not only callable in unit tests.
// PREVENTS: regression where the handler exists but no init() registered
// it, so `ze show ip *` returns "unknown command" at runtime.
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
//
// VALIDATES: AC (op-0 command 4) -- unknown family produces a clear error
// naming the valid set, not a silent empty result.
// PREVENTS: regression where a typo in the flag silently returns the
// unfiltered table instead of rejecting.
func TestHandleShowArp_UnknownFamilyRejects(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"--family", "ipv5"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "ipv5")
	assert.Contains(t, msg, "ipv4")
	assert.Contains(t, msg, "ipv6")
}

// TestHandleShowArp_FamilyRequiresValue verifies --family without an
// argument rejects.
//
// VALIDATES: AC (op-0 command 4) -- --family without a value is a user
// error, not a crash or silent fall-through to unfiltered.
// PREVENTS: regression where the loop's i+1 index bypass lets --family
// at the end of args be silently ignored.
func TestHandleShowArp_FamilyRequiresValue(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"--family"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, msg, "requires a value")
}

// TestHandleShowIPRoute_DispatchShape verifies the handler dispatches to
// the backend and wraps the result under the `routes` key. The backend
// may return an error (no backend loaded, or VPP rejecting) -- either
// shape is valid evidence that the handler reached the backend layer.
//
// VALIDATES: AC (op-0 command 2) -- handler wraps the route list under
// the `routes` key, or surfaces the backend's operational error.
// PREVENTS: regression where the handler returns an un-wrapped slice or
// forgets to propagate a backend error.
func TestHandleShowIPRoute_DispatchShape(t *testing.T) {
	resp, err := handleShowIPRoute(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Status == plugin.StatusError {
		msg, ok := resp.Data.(string)
		require.True(t, ok, "error Data should be string")
		assert.True(t,
			strings.Contains(msg, "no backend") ||
				strings.Contains(msg, "ListKernelRoutes") ||
				strings.Contains(msg, "route"),
			"error should name the missing capability: %q", msg,
		)
		return
	}

	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	_, hasRoutes := data["routes"]
	assert.True(t, hasRoutes, "response should carry the routes key")
}

// TestHandleShowArp_UnknownPositional verifies the handler rejects an
// unknown positional arg rather than silently returning the full
// neighbor table.
//
// VALIDATES: review finding #3 -- `show ip arp eth0` (user expecting a
// per-interface filter that does not exist) is rejected with a usage
// hint, not ignored.
func TestHandleShowArp_UnknownPositional(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"eth0"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, _ := resp.Data.(string)
	assert.Contains(t, msg, "unknown argument")
}

// TestHandleShowArp_FamilyRepeatRejects verifies --family given twice
// rejects rather than last-wins.
//
// VALIDATES: review finding #14 -- repeated --family is a user error.
func TestHandleShowArp_FamilyRepeatRejects(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"--family", "ipv4", "--family", "ipv6"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	msg, _ := resp.Data.(string)
	assert.Contains(t, msg, "more than once")
}

// TestHandleShowIPRoute_InvalidPrefixRejects verifies CIDR validation
// at the handler level.
//
// VALIDATES: review finding #2 -- operator typos reject with a clear
// error; `default` is accepted as a synonym.
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
		msg, _ := resp.Data.(string)
		assert.Contains(t, msg, "invalid prefix", "arg=%q", arg)
	}

	// "default" is the documented synonym -- MUST NOT reject at parse time.
	resp, err := handleShowIPRoute(nil, []string{"default"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Backend may or may not be loaded; only assert we did not stop at prefix-validation.
	if resp.Status == plugin.StatusError {
		msg, _ := resp.Data.(string)
		assert.NotContains(t, msg, "invalid prefix", "default should not be rejected as bad CIDR")
	}
}

// TestHandleShowIPRoute_LimitParsing verifies --limit is parsed and
// rejects non-positive or non-numeric values.
//
// VALIDATES: review finding #1 -- default cap is on; explicit --limit
// requires a positive integer.
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
// backend and wraps the result under the `neighbors` key. The backend may
// return an error (no backend loaded in unit-test environment, or VPP
// returning errNotSupported) -- either shape is valid evidence that the
// handler reached the backend layer.
//
// VALIDATES: AC (op-0 command 4) -- handler wraps the neighbor list under
// the `neighbors` key, or surfaces the backend's operational error.
// PREVENTS: regression where the handler returns an un-wrapped slice or
// forgets to propagate a backend error.
func TestHandleShowArp_DispatchShape(t *testing.T) {
	resp, err := handleShowArp(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Status == plugin.StatusError {
		// No backend loaded, or VPP active -- handler correctly surfaced
		// the error from iface.ListNeighbors without wrapping.
		msg, ok := resp.Data.(string)
		require.True(t, ok, "error Data should be string")
		// Accept either "no backend loaded" or backend-specific reject.
		assert.True(t,
			strings.Contains(msg, "no backend") || strings.Contains(msg, "ListNeighbors") || strings.Contains(msg, "neigh"),
			"error should name the missing capability: %q", msg,
		)
		return
	}

	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	_, hasNeighbors := data["neighbors"]
	assert.True(t, hasNeighbors, "response should carry the neighbors key")
}
