// Design: docs/architecture/api/architecture.md -- gNMI show command tests

package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/gnmi"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestShowGNMIStatus_NotRunning(t *testing.T) {
	gnmi.RegisterGlobal(nil)
	defer gnmi.RegisterGlobal(nil)

	resp, err := handleShowGNMI(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	st, ok := resp.Data.(gnmi.ServerStatus)
	require.True(t, ok)
	assert.False(t, st.Enabled)
}

func TestShowGNMIStatus_Running(t *testing.T) {
	srv := gnmi.NewServer(gnmi.Config{
		ListenAddr: "0.0.0.0:9339",
		Token:      "secret",
	}, nil, nil, nil, gnmi.NewChangeNotifier())

	gnmi.RegisterGlobal(srv)
	defer gnmi.RegisterGlobal(nil)

	resp, err := handleShowGNMI(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	st, ok := resp.Data.(gnmi.ServerStatus)
	require.True(t, ok)
	assert.True(t, st.Enabled)
	assert.True(t, st.TokenSet)
	assert.False(t, st.TLSConfigured)
	assert.Equal(t, 0, st.Subscribers)
}
