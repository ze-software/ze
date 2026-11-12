// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` handler tests
package mpls

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/opererr"
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

// withEmptyForwardingTable swaps the data-plane reader for one that answers
// an empty table the way a Linux read of a host with no MPLS routes does: a
// nil slice and no error. The real readers answer from the host's kernel,
// which holds routes on some hosts and refuses off Linux, or from VPP.
func withEmptyForwardingTable(t *testing.T) {
	t.Helper()
	withForwardingReader(t, func(int) ([]mplsfib.ForwardingEntry, error) { return nil, nil })
}

// withForwardingReader makes reader the one the handler selects.
func withForwardingReader(t *testing.T, reader mplsfib.ForwardingReader) {
	t.Helper()
	old := forwardingReaderFor
	forwardingReaderFor = func(*pluginserver.CommandContext) (mplsfib.ForwardingReader, *opererr.Error) {
		return reader, nil
	}
	t.Cleanup(func() { forwardingReaderFor = old })
}

// TestShowMPLSForwardingAnswersTheSelectedReader proves the handler answers
// the rows of the reader the data plane selects, in the shared row shape, and
// caps them at limit with truncated set.
//
// Method: a stand-in reader answers three VPP-shaped rows; limit 2.
func TestShowMPLSForwardingAnswersTheSelectedReader(t *testing.T) {
	withForwardingReader(t, func(limit int) ([]mplsfib.ForwardingEntry, error) {
		assert.Equal(t, 3, limit, "the handler asks for limit+1 rows to detect truncation")
		return []mplsfib.ForwardingEntry{
			{InLabel: 100, Operation: "pop"},
			{InLabel: 101, Operation: "swap", OutLabels: []int{200}, NextHop: "10.0.0.2"},
			{FEC: "10.9.0.0/24", Operation: "push", OutLabels: []int{300}, NextHop: "10.0.0.2"},
		}, nil
	})
	resp, err := handleShowMPLSForwarding(nil, commandtest.Args("limit", "2"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `{"in-label":100,"operation":"pop"}`)
	assert.Contains(t, string(raw), `{"in-label":101,"operation":"swap","out-labels":[200],"next-hop":"10.0.0.2"}`)
	assert.NotContains(t, string(raw), "10.9.0.0/24", "limit 2 drops the third row")
	assert.Contains(t, string(raw), `"truncated":true`)
}

// TestShowMPLSForwardingReaderFailureIsAnError proves a reader that cannot
// read its table (VPP unreachable, a dump refused) reaches the operator as an
// error carrying its fix, never as an empty table.
func TestShowMPLSForwardingReaderFailureIsAnError(t *testing.T) {
	withForwardingReader(t, func(int) ([]mplsfib.ForwardingEntry, error) {
		return nil, opererr.New("VPP MPLS table 0 cannot be read", "VPP is not connected", "ze show vpp status")
	})
	resp, err := handleShowMPLSForwarding(nil, commandtest.Args())
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "VPP MPLS table 0 cannot be read")
	assert.Nil(t, resp.Data, "a failed read carries no entries")
}

// TestSelectedForwardingReaderRefusesWithoutConfig proves the selection never
// falls back to the kernel when it cannot read which data plane the config
// selects: on a VPP box that fallback is the empty table this command exists
// not to answer.
func TestSelectedForwardingReaderRefusesWithoutConfig(t *testing.T) {
	reader, refusal := selectedForwardingReader(nil)
	assert.Nil(t, reader)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error(), "configuration is unavailable")
}

// TestReaderForDataPlane proves this package's kernel reader serves the
// "netlink" data plane, and a data plane no reader serves is refused by name
// rather than answered with the kernel's table.
func TestReaderForDataPlane(t *testing.T) {
	reader, refusal := readerForDataPlane(kernelDataPlane)
	require.Nil(t, refusal)
	require.NotNil(t, reader)

	reader, refusal = readerForDataPlane("no-such-plane")
	assert.Nil(t, reader)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error(), `"no-such-plane"`)
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
// Method: the forwardingReaderFor seam answers a nil, error-free table.
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
