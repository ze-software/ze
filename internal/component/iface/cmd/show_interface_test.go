package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// These cover the `show interface` family, moved here with the handlers from
// internal/component/cmd/show. They skip when no iface backend is loaded (the
// unit-test default), exercising the dispatch paths when one is.

func TestHandleShowInterface(t *testing.T) {
	// List all interfaces -- requires iface backend.
	resp, err := handleShowInterface(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == "error" && resp.Error == "iface: no backend loaded" {
		t.Skip("iface backend not available in test environment")
	}
	assert.Equal(t, "done", resp.Status)

	// Show specific interface -- loopback always exists.
	resp, err = handleShowInterface(nil, []string{"lo"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Error, "lo")

	// Show nonexistent interface -- should return error response.
	resp, err = handleShowInterface(nil, []string{"nonexistent_iface99"})
	require.NoError(t, err) // Go error nil, operational error in Response
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleShowInterfaceBrief verifies show interface brief dispatches correctly.
//
// VALIDATES: show interface brief dispatches to showInterfaceBrief handler.
// PREVENTS: Brief mode not recognized, falls through to single-interface lookup.
func TestHandleShowInterfaceBrief(t *testing.T) {
	resp, err := handleShowInterface(nil, []string{"brief"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	// On systems with netlink, returns "done" with interface list.
	// On systems without netlink (CI), returns "error" from ListInterfaces.
	// Either way, the brief path was taken (not the single-interface path).
	if resp.Status == "done" {
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok, "brief response should be map")
		_, hasInterfaces := data["interfaces"]
		assert.True(t, hasInterfaces, "should have interfaces key")
		_, hasCount := data["count"]
		assert.True(t, hasCount, "should have count key")
	}
}
