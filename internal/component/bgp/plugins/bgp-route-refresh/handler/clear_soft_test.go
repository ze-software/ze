package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestPeerClearSoftHandler verifies clear soft sends route-refresh for negotiated families.
//
// VALIDATES: AC-5 — bgp peer clear soft triggers route-refresh per negotiated family.
// PREVENTS: Soft clear not reaching the reactor or not reporting refreshed families.
func TestPeerClearSoftHandler(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handleBgpPeerClearSoft(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", data["peer"])
	assert.Equal(t, "soft-clear", data["action"])

	families, ok := data["families-refreshed"].([]string)
	require.True(t, ok)
	assert.Contains(t, families, "ipv4/unicast")

	// Verify reactor was called
	require.Len(t, reactor.softClearCalls, 1)
	assert.Equal(t, "192.0.2.1", reactor.softClearCalls[0])
}

// TestPeerClearSoftWildcard verifies clear soft rejects wildcard selector.
//
// VALIDATES: Soft clear requires specific peer address.
// PREVENTS: Accidentally clearing all peers with wildcard.
func TestPeerClearSoftWildcard(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "*"

	resp, err := handleBgpPeerClearSoft(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}
