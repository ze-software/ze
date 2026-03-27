package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/fleet"
)

// TestHubConfigFetch verifies that config-fetch returns config and version hash.
//
// VALIDATES: Fetch returns config + version hash (AC-1, AC-2).
// PREVENTS: Client receiving empty config on fetch.
func TestHubConfigFetch(t *testing.T) {
	t.Parallel()

	configData := []byte("bgp { peer 10.0.0.1 { peer-as 65001; } }")
	svc := NewManagedConfigService(func(name string) ([]byte, error) {
		if name == "edge-01" {
			return configData, nil
		}
		return nil, ErrClientConfigNotFound
	})

	resp, err := svc.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: ""})
	require.NoError(t, err)

	expectedVersion := fleet.VersionHash(configData)
	assert.Equal(t, expectedVersion, resp.Version)
	assert.NotEmpty(t, resp.Config, "config should be non-empty for first fetch")
	assert.Empty(t, resp.Status, "status should be empty when config is returned")
}

// TestHubConfigFetchCurrent verifies that matching version returns "current".
//
// VALIDATES: Matching version returns status=current, no config (AC-13).
// PREVENTS: Unnecessary config re-transfer on reconnect.
func TestHubConfigFetchCurrent(t *testing.T) {
	t.Parallel()

	configData := []byte("bgp { peer 10.0.0.1 { peer-as 65001; } }")
	currentVersion := fleet.VersionHash(configData)

	svc := NewManagedConfigService(func(name string) ([]byte, error) {
		return configData, nil
	})

	resp, err := svc.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: currentVersion})
	require.NoError(t, err)

	assert.Equal(t, "current", resp.Status)
	assert.Empty(t, resp.Config, "no config when version matches")
}

// TestHubConfigFetchMissing verifies that missing config returns an error.
//
// VALIDATES: No config entry for name returns error.
// PREVENTS: Client receiving empty config silently.
func TestHubConfigFetchMissing(t *testing.T) {
	t.Parallel()

	svc := NewManagedConfigService(func(name string) ([]byte, error) {
		return nil, ErrClientConfigNotFound
	})

	_, err := svc.HandleConfigFetch("unknown-client", fleet.ConfigFetchRequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrClientConfigNotFound)
}

// TestHubConfigChanged verifies that config change produces correct notification.
//
// VALIDATES: Config change generates notification with new version hash.
// PREVENTS: Stale version hash in notification.
func TestHubConfigChanged(t *testing.T) {
	t.Parallel()

	newConfig := []byte("bgp { peer 10.0.0.2 { peer-as 65002; } }")

	svc := NewManagedConfigService(func(name string) ([]byte, error) {
		return newConfig, nil
	})

	notification, err := svc.BuildConfigChanged("edge-01")
	require.NoError(t, err)

	expectedVersion := fleet.VersionHash(newConfig)
	assert.Equal(t, expectedVersion, notification.Version)
}

// TestHubConfigFetchUpdated verifies that updated config is returned when versions differ.
//
// VALIDATES: Different version -> full config returned.
// PREVENTS: Client stuck on old config after hub update.
func TestHubConfigFetchUpdated(t *testing.T) {
	t.Parallel()

	newConfig := []byte("bgp { peer 10.0.0.2 { peer-as 65002; } }")
	svc := NewManagedConfigService(func(name string) ([]byte, error) {
		return newConfig, nil
	})

	resp, err := svc.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: "old-version-hash!"})
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Config, "should return full config when versions differ")
	assert.Equal(t, fleet.VersionHash(newConfig), resp.Version)
	assert.Empty(t, resp.Status, "status empty when config is returned")
}
