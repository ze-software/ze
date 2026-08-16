package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestShowNeighbor_RegisteredWireMethods verifies the object-rooted
// `ze-show:neighbor` and `ze-show:arp` RPCs are installed and the pre-reorg
// `ze-show:ip-arp` / `ze-show:neighbors` methods are gone.
func TestShowNeighbor_RegisteredWireMethods(t *testing.T) {
	want := map[string]bool{"ze-show:neighbor": false, "ze-show:arp": false}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		switch r.WireMethod {
		case "ze-show:neighbor", "ze-show:arp":
			require.NotNil(t, r.Handler, "%s handler must not be nil", r.WireMethod)
			want[r.WireMethod] = true
		case "ze-show:ip-arp", "ze-show:neighbors":
			t.Errorf("retired wire method %q is still registered after the object-rooting reorg", r.WireMethod)
		}
	}
	for wm, seen := range want {
		require.True(t, seen, "%s not registered via pluginserver.RegisterRPCs", wm)
	}
}

// TestHandleShowNeighbor_UnknownFamilyRejects verifies an invalid positional
// family rejects with the valid-set in the message.
func TestHandleShowNeighbor_UnknownFamilyRejects(t *testing.T) {
	resp, err := handleShowNeighbor(nil, []string{"ipv5"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "ipv5")
	assert.Contains(t, resp.Error, "ipv4")
	assert.Contains(t, resp.Error, "ipv6")
}

// removed TestHandleShowNeighbor_FamilyCaseInsensitive -- the CLI is
// case-sensitive by design. The dispatcher validates the `family` enum against
// the YANG leaf (lowercase) before this handler runs, so the handler never sees
// an uppercase token; a case-insensitivity test asserted a non-feature.

// TestHandleShowNeighbor_TooManyArgsRejects verifies a second positional arg
// rejects rather than being silently ignored.
func TestHandleShowNeighbor_TooManyArgsRejects(t *testing.T) {
	resp, err := handleShowNeighbor(nil, []string{"ipv4", "extra"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "too many arguments")
}

// TestHandleShowNeighbor_DispatchShape verifies the handler dispatches to the
// backend and wraps the result under the `neighbors` key.
func TestHandleShowNeighbor_DispatchShape(t *testing.T) {
	resp, err := handleShowNeighbor(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Status == plugin.StatusError {
		assert.True(t,
			strings.Contains(resp.Error, "no backend") ||
				strings.Contains(resp.Error, "ListNeighbors") ||
				strings.Contains(resp.Error, "neigh"),
			"error should name the missing capability: %q", resp.Error,
		)
		return
	}

	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	_, hasNeighbors := data["neighbors"]
	assert.True(t, hasNeighbors, "response should carry the neighbors key")
}

// TestHandleShowArp_RejectsArgs verifies the IPv4 alias takes no argument and
// points the operator at `show neighbor` for family selection.
func TestHandleShowArp_RejectsArgs(t *testing.T) {
	resp, err := handleShowArp(nil, []string{"ipv6"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unexpected argument")
	assert.Contains(t, resp.Error, "neighbor")
}

// TestHandleShowArp_DispatchShape verifies `show arp` dispatches to the backend
// (forcing the IPv4 family) and wraps the result under the `neighbors` key.
func TestHandleShowArp_DispatchShape(t *testing.T) {
	resp, err := handleShowArp(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Status == plugin.StatusError {
		assert.True(t,
			strings.Contains(resp.Error, "no backend") ||
				strings.Contains(resp.Error, "ListNeighbors") ||
				strings.Contains(resp.Error, "neigh"),
			"error should name the missing capability: %q", resp.Error,
		)
		return
	}

	assert.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	_, hasNeighbors := data["neighbors"]
	assert.True(t, hasNeighbors, "response should carry the neighbors key")
}
