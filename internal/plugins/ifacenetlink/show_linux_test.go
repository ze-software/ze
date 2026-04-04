//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListInterfaces(t *testing.T) {
	b := &netlinkBackend{}
	ifaces, err := b.ListInterfaces()
	require.NoError(t, err)
	require.NotEmpty(t, ifaces, "should find at least one interface")

	var found bool
	for i := range ifaces {
		if ifaces[i].Name != "lo" {
			continue
		}
		found = true
		assert.Equal(t, "up", ifaces[i].State)
		assert.Greater(t, ifaces[i].MTU, 0)
		assert.Greater(t, ifaces[i].Index, 0)
		break
	}
	assert.True(t, found, "loopback interface not found")
}

func TestGetInterface(t *testing.T) {
	b := &netlinkBackend{}
	info, err := b.GetInterface("lo")
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "lo", info.Name)
	assert.Equal(t, "up", info.State)
	assert.Greater(t, info.Index, 0)
	assert.Greater(t, info.MTU, 0)
	assert.NotEmpty(t, info.Addresses, "loopback should have at least 127.0.0.1")
	assert.NotNil(t, info.Stats)
}

func TestGetInterfaceNotFound(t *testing.T) {
	b := &netlinkBackend{}
	_, err := b.GetInterface("nonexistent_iface99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_iface99")
}

func TestGetInterfaceInvalidName(t *testing.T) {
	b := &netlinkBackend{}
	_, err := b.GetInterface("")
	require.Error(t, err)

	_, err = b.GetInterface("a/b")
	require.Error(t, err)
}
