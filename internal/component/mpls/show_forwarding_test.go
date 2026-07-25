// Design: plan/spec-mpls-1-kernel.md -- `show mpls forwarding` handler tests
package mpls

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestShowMPLSForwardingArgs(t *testing.T) {
	t.Run("no args returns done envelope", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
	})

	t.Run("limit without value rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"limit"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "limit requires a value")
	})

	t.Run("non-numeric limit rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"limit", "abc"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "invalid limit")
	})

	t.Run("zero limit rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"limit", "0"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
	})

	t.Run("unexpected positional rejects with usage", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"bogus"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "show mpls forwarding")
	})
}

// TestShowMPLSForwardingEntriesIsAlwaysArray guards the JSON list contract:
// `entries` must marshal as a JSON array even when the forwarding table is
// empty. A nil slice marshals to `null`, which broke the .ci consumer with
// `"entries" is not a list: NoneType`. dumpMPLSRoutes returns nil on a host
// with no MPLS routes (forwarding_other.go, and any Linux read that yields
// none), so the handler must pin entries to an empty array.
//
// VALIDATES: show mpls forwarding always returns entries as a JSON list.
// PREVENTS: a regression where an empty table serializes entries as null.
func TestShowMPLSForwardingEntriesIsAlwaysArray(t *testing.T) {
	resp, err := handleShowMPLSForwarding(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"entries":[]`, "entries must be an empty array, not null: %s", raw)
	assert.NotContains(t, string(raw), `"entries":null`)
}
