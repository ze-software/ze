package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleShowInterface(t *testing.T) {
	// List all interfaces -- requires iface backend.
	resp, err := handleShowInterface(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == "error" && resp.Data == "iface: no backend loaded" {
		t.Skip("iface backend not available in test environment")
	}
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Data, "lo") // loopback always exists

	// Show specific interface -- loopback always exists.
	resp, err = handleShowInterface(nil, []string{"lo"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Data, "lo")

	// Show nonexistent interface -- should return error response.
	resp, err = handleShowInterface(nil, []string{"nonexistent_iface99"})
	require.NoError(t, err) // Go error nil, operational error in Response
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}
