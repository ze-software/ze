package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// These cover the OS-tool `resolve traceroute` (ze-resolve:traceroute) argument
// validation, moved here with the handler from internal/component/resolve/cmd.

func TestHandleResolveTraceroute_InvalidTarget(t *testing.T) {
	resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, []string{"foo;bar"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "invalid character")
}

func TestHandleResolveTraceroute_InvalidSource(t *testing.T) {
	resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, []string{"192.168.1.1", "source", "not-ip"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "not a valid IP address")
}

func TestHandleResolveTraceroute_UnknownOption(t *testing.T) {
	resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, []string{"192.168.1.1", "bogus"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unknown option")
}

func TestHandleResolveTraceroute_SourceMissingValue(t *testing.T) {
	resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, []string{"192.168.1.1", "source"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "requires a value")
}

// --- validateResolveTarget (moved here with the resolve traceroute handler) ---

func TestValidateResolveTarget_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"ipv4", "192.168.1.1"},
		{"ipv6", "::1"},
		{"hostname", "example.com"},
		{"subdomain", "foo.bar.example.com"},
		{"hyphen", "my-host.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, validateResolveTarget(tt.input))
		})
	}
}

func TestValidateResolveTarget_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{"empty", "", "must not be empty"},
		{"space", "foo bar", "invalid character"},
		{"semicolon", "foo;bar", "invalid character"},
		{"pipe", "foo|bar", "invalid character"},
		{"long", strings.Repeat("a", 254), "253-character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResolveTarget(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// --- validateSourceIP (moved here with the resolve traceroute handler) ---

func TestValidateSourceIP_Valid(t *testing.T) {
	assert.NoError(t, validateSourceIP("192.168.1.1"))
	assert.NoError(t, validateSourceIP("::1"))
}

func TestValidateSourceIP_Invalid(t *testing.T) {
	err := validateSourceIP("not-an-ip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid IP address")
}
