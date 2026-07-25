package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestShowRoute_RegisteredWireMethod verifies the object-rooted `ze-show:route`
// RPC is installed and the pre-reorg `ze-show:ip-route` / `ze-show:kernel-routes`
// methods are gone (the kernel FIB read is now `show route`).
func TestShowRoute_RegisteredWireMethod(t *testing.T) {
	foundRoute := false
	for _, r := range pluginserver.AllBuiltinRPCs() {
		switch r.WireMethod {
		case "ze-show:route":
			require.NotNil(t, r.Handler, "ze-show:route handler must not be nil")
			foundRoute = true
		case "ze-show:ip-route", "ze-show:kernel-routes":
			t.Errorf("retired wire method %q is still registered after the object-rooting reorg", r.WireMethod)
		}
	}
	require.True(t, foundRoute, "ze-show:route not registered via pluginserver.RegisterRPCs")
}

// TestHandleShowRoute_DispatchShape verifies the handler dispatches to the
// backend and wraps the result under the `routes` key.
func TestHandleShowRoute_DispatchShape(t *testing.T) {
	resp, err := handleShowRoute(nil, nil)
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

// TestHandleShowRoute_InvalidPrefixRejects verifies CIDR validation at the
// handler level, and that "default" is accepted as a synonym.
func TestHandleShowRoute_InvalidPrefixRejects(t *testing.T) {
	bad := []string{
		"10.0.0.0",    // no mask
		"10.0.0.0/33", // invalid IPv4 mask
		"::/129",      // invalid IPv6 mask
		"not-a-cidr",
	}
	for _, arg := range bad {
		resp, err := handleShowRoute(nil, []string{arg})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status, "arg=%q should reject", arg)
		assert.Contains(t, resp.Error, "invalid prefix", "arg=%q", arg)
	}

	// "default" is the documented synonym -- MUST NOT reject at parse time.
	resp, err := handleShowRoute(nil, []string{"default"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == plugin.StatusError {
		assert.NotContains(t, resp.Error, "invalid prefix", "default should not be rejected as bad CIDR")
	}
}

// TestHandleShowRoute_LimitParsing verifies the `limit` keyword is parsed and
// rejects non-positive or non-numeric values.
func TestHandleShowRoute_LimitParsing(t *testing.T) {
	bad := [][]string{
		{"limit"},        // missing value
		{"limit", "0"},   // zero
		{"limit", "-5"},  // negative
		{"limit", "abc"}, // not a number
	}
	for _, args := range bad {
		resp, err := handleShowRoute(nil, args)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status, "args=%v should reject", args)
	}
}

// TestHandleShowRoute_RejectsDashLimitFlag verifies the retired `--limit` flag
// form is rejected (filters are keyword grammar, not flags); the operator is
// pointed at the `limit N` keyword.
func TestHandleShowRoute_RejectsDashLimitFlag(t *testing.T) {
	resp, err := handleShowRoute(nil, []string{"--limit", "50"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unknown flag")
}
