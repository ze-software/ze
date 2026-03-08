package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestHandlerRefresh verifies handleRefresh sends ROUTE-REFRESH.
//
// VALIDATES: Refresh handler parses family and calls reactor.SendRefresh.
// PREVENTS: Route refresh requests not reaching reactor (RFC 2918).
func TestHandlerRefresh(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleRefresh(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendRefreshCalled)
}

// TestHandlerBoRR verifies handleBoRR sends BoRR marker.
//
// VALIDATES: BoRR handler parses family and calls reactor.SendBoRR.
// PREVENTS: Route refresh markers not reaching reactor.
func TestHandlerBoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBoRR(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendBoRRCalled)
}

// TestHandlerEoRR verifies handleEoRR sends EoRR marker.
//
// VALIDATES: EoRR handler parses family and calls reactor.SendEoRR.
// PREVENTS: Route refresh markers not reaching reactor.
func TestHandlerEoRR(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleEoRR(ctx, []string{"ipv4/unicast"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.True(t, reactor.sendEoRRCalled)
}

// TestHandlerRefreshMissingFamily verifies refresh commands reject missing family.
//
// VALIDATES: BoRR/EoRR require family argument.
// PREVENTS: Panic on missing args.
func TestHandlerRefreshMissingFamily(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)

	resp, err := handleBoRR(ctx, nil)
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}
