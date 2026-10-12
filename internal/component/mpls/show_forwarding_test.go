// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` handler tests
package mpls

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

func TestShowMPLSForwardingArgs(t *testing.T) {
	t.Run("no args returns done envelope", func(t *testing.T) {
		withEmptyForwardingTable(t)
		resp, err := handleShowMPLSForwarding(nil, commandtest.Args())
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
	})

	t.Run("limit without value rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, commandtest.Args("limit"))
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "limit requires a value")
	})

	t.Run("non-numeric limit rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, commandtest.Args("limit", "abc"))
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "invalid limit")
	})

	t.Run("zero limit rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, commandtest.Args("limit", "0"))
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
	})

	t.Run("unexpected positional rejects with usage", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, commandtest.Args("bogus"))
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "show mpls forwarding")
	})
}

// withEmptyForwardingTable swaps the kernel reader for one that answers an
// empty table the way a Linux read of a host with no MPLS routes does: a nil
// slice and no error. The real reader answers from the host's kernel, which
// holds routes on some hosts and refuses off Linux.
func withEmptyForwardingTable(t *testing.T) {
	t.Helper()
	old := forwardingTable
	forwardingTable = func(int) ([]forwardingEntry, error) { return nil, nil }
	t.Cleanup(func() { forwardingTable = old })
}

// TestShowMPLSForwardingEntriesIsAlwaysArray guards the JSON list contract:
// `entries` must marshal as a JSON array even when the forwarding table is
// empty. A nil slice marshals to `null`, which broke the .ci consumer with
// `"entries" is not a list: NoneType`. dumpMPLSRoutes returns nil on a Linux
// host with no MPLS routes, so the handler must pin entries to an empty array.
//
// VALIDATES: show mpls forwarding always returns entries as a JSON list.
// PREVENTS: a regression where an empty table serializes entries as null.
//
// Method: the forwardingTable seam answers a nil, error-free table.
func TestShowMPLSForwardingEntriesIsAlwaysArray(t *testing.T) {
	withEmptyForwardingTable(t)
	resp, err := handleShowMPLSForwarding(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"entries":[]`, "entries must be an empty array, not null: %s", raw)
	assert.NotContains(t, string(raw), `"entries":null`)
}
