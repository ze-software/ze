package managed

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/fleet"
)

// TestClientHandleConfigChanged verifies that a config-changed notification triggers fetch.
//
// VALIDATES: Notification triggers fetch (AC-3).
// PREVENTS: Client ignoring config change notifications.
func TestClientHandleConfigChanged(t *testing.T) {
	t.Parallel()

	var fetchCalled bool
	var fetchVersion string

	h := &Handler{
		OnFetch: func(version string) {
			fetchCalled = true
			fetchVersion = version
		},
	}

	h.HandleConfigChanged(fleet.ConfigChanged{Version: "abcdef0123456789"})
	assert.True(t, fetchCalled, "fetch should be triggered")
	assert.Equal(t, "abcdef0123456789", fetchVersion)
}

// TestClientValidateConfigOk verifies that valid config is accepted without caching.
//
// VALIDATES: Managed handler validates only; cache is deferred until OnCommit succeeds.
// PREVENTS: Managed push mutating local config before runtime transaction succeeds.
func TestClientValidateConfigOk(t *testing.T) {
	t.Parallel()

	configData := []byte("bgp { peer 10.0.0.1 { peer-as 65001; } }")
	encoded := base64.StdEncoding.EncodeToString(configData)

	var cacheWasCalled bool

	h := &Handler{
		Validate: func(data []byte) error {
			return nil // valid
		},
		Cache: func(data []byte) error {
			cacheWasCalled = true
			return nil
		},
	}

	ack := h.ProcessConfig(fleet.ConfigFetchResponse{
		Version: fleet.VersionHash(configData),
		Config:  encoded,
	})

	assert.True(t, ack.OK, "should accept valid config")
	assert.Equal(t, fleet.VersionHash(configData), ack.Version)
	assert.Empty(t, ack.Error)
	assert.False(t, cacheWasCalled, "cache should be deferred to OnCommit")
}

// TestClientValidateConfigBad verifies that invalid config is rejected.
//
// VALIDATES: Invalid config rejected, blob unchanged (AC-8).
// PREVENTS: Broken config being cached and applied.
func TestClientValidateConfigBad(t *testing.T) {
	t.Parallel()

	configData := []byte("invalid config {{{")
	encoded := base64.StdEncoding.EncodeToString(configData)

	var cacheWasCalled bool

	h := &Handler{
		Validate: func(data []byte) error {
			return assert.AnError // validation fails
		},
		Cache: func(data []byte) error {
			cacheWasCalled = true
			return nil
		},
	}

	ack := h.ProcessConfig(fleet.ConfigFetchResponse{
		Version: "some-version-hash",
		Config:  encoded,
	})

	assert.False(t, ack.OK, "should reject invalid config")
	assert.Equal(t, "some-version-hash", ack.Version)
	assert.NotEmpty(t, ack.Error)
	assert.False(t, cacheWasCalled, "cache should not be called on invalid config")
}

// TestClientValidateConfigBadBase64 verifies that bad base64 is rejected.
//
// VALIDATES: Corrupted base64 config rejected.
// PREVENTS: Panic on decode failure.
func TestClientValidateConfigBadBase64(t *testing.T) {
	t.Parallel()

	h := &Handler{
		Validate: func(data []byte) error { return nil },
		Cache:    func(data []byte) error { return nil },
	}

	ack := h.ProcessConfig(fleet.ConfigFetchResponse{
		Version: "some-version-hash",
		Config:  "not-valid-base64!!!",
	})

	assert.False(t, ack.OK)
	assert.Contains(t, ack.Error, "decode")
}

// TestClientHandleConfigChangedNilFetch verifies nil OnFetch is safe.
//
// VALIDATES: Nil callback doesn't panic.
// PREVENTS: Crash when handler not fully wired.
func TestClientHandleConfigChangedNilFetch(t *testing.T) {
	t.Parallel()

	h := &Handler{}
	require.NotPanics(t, func() {
		h.HandleConfigChanged(fleet.ConfigChanged{Version: "abc"})
	})
}
