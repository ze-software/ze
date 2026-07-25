// Design: docs/architecture/api/architecture.md -- gNMI show command tests

package gnmi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestShowGNMIStatus_NotRunning(t *testing.T) {
	RegisterGlobal(nil)
	defer RegisterGlobal(nil)

	resp, err := handleShowGNMI(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	st, ok := resp.Data.(ServerStatus)
	require.True(t, ok)
	assert.False(t, st.Enabled)
}

func TestShowGNMIStatus_Running(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr: "0.0.0.0:9339",
		Token:      "secret",
	}, nil, nil, nil, NewChangeNotifier())

	RegisterGlobal(srv)
	defer RegisterGlobal(nil)

	resp, err := handleShowGNMI(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	st, ok := resp.Data.(ServerStatus)
	require.True(t, ok)
	assert.True(t, st.Enabled)
	assert.True(t, st.TokenSet)
	assert.False(t, st.TLSConfigured)
	assert.Equal(t, 0, st.Subscribers)
}
