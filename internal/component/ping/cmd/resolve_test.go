package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// These cover the OS-tool `resolve ping` (ze-resolve:ping) argument validation,
// moved here with the handler from internal/component/resolve/cmd.

func TestHandleResolvePing_InvalidTarget(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"foo;bar"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "invalid character")
}

func TestHandleResolvePing_InvalidSource(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "source", "not-ip"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "not a valid IP address")
}

func TestHandleResolvePing_InvalidCount(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "count", "abc"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "not a valid number")
}

func TestHandleResolvePing_CountOutOfRange(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "count", "200"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "out of range")
}

func TestHandleResolvePing_SizeOutOfRange(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "size", "99999"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "out of range")
}

func TestHandleResolvePing_UnknownOption(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "bogus", "val"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unknown option")
}

func TestHandleResolvePing_TrailingKeyword(t *testing.T) {
	resp, err := handleResolvePing(&pluginserver.CommandContext{}, []string{"192.168.1.1", "count"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "requires a value")
}

// --- validateUint (moved here with the resolve ping handler) ---

func TestValidateUint_Valid(t *testing.T) {
	assert.NoError(t, validateUint("5", "count", 1, 100))
	assert.NoError(t, validateUint("1", "count", 1, 100))
	assert.NoError(t, validateUint("100", "count", 1, 100))
}

func TestValidateUint_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		lo, hi      uint64
		errContains string
	}{
		{"not number", "abc", 1, 100, "not a valid number"},
		{"too low", "0", 1, 100, "out of range"},
		{"too high", "101", 1, 100, "out of range"},
		{"negative", "-1", 1, 100, "not a valid number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUint(tt.input, "count", tt.lo, tt.hi)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
