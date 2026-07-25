package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

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

func TestHandleResolveTraceroute_MaxHopsBoundary(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid", []string{"127.0.0.1", "max-hops", "10"}, false},
		{"zero", []string{"127.0.0.1", "max-hops", "0"}, true},
		{"over", []string{"127.0.0.1", "max-hops", "65"}, true},
		{"max-valid", []string{"127.0.0.1", "max-hops", "64"}, false},
		{"missing-value", []string{"127.0.0.1", "max-hops"}, true},
		{"not-a-number", []string{"127.0.0.1", "max-hops", "abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, tt.args)
			require.NoError(t, err)
			if tt.wantErr {
				assert.Equal(t, plugin.StatusError, resp.Status)
			} else if resp.Status == plugin.StatusError && strings.Contains(resp.Error, "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %s", resp.Error)
			}
		})
	}
}

func TestHandleResolveTraceroute_TimeoutBoundary(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid", []string{"127.0.0.1", "timeout", "2s"}, false},
		{"under", []string{"127.0.0.1", "timeout", "500ms"}, true},
		{"over", []string{"127.0.0.1", "timeout", "31s"}, true},
		{"max-valid", []string{"127.0.0.1", "timeout", "30s"}, false},
		{"missing-value", []string{"127.0.0.1", "timeout"}, true},
		{"not-a-duration", []string{"127.0.0.1", "timeout", "abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, tt.args)
			require.NoError(t, err)
			if tt.wantErr {
				assert.Equal(t, plugin.StatusError, resp.Status)
			} else if resp.Status == plugin.StatusError && strings.Contains(resp.Error, "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %s", resp.Error)
			}
		})
	}
}

func TestHandleResolveTraceroute_ProbesBoundary(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid", []string{"127.0.0.1", "probes", "1"}, false},
		{"zero", []string{"127.0.0.1", "probes", "0"}, true},
		{"over", []string{"127.0.0.1", "probes", "11"}, true},
		{"max-valid", []string{"127.0.0.1", "probes", "10"}, false},
		{"missing-value", []string{"127.0.0.1", "probes"}, true},
		{"not-a-number", []string{"127.0.0.1", "probes", "abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, tt.args)
			require.NoError(t, err)
			if tt.wantErr {
				assert.Equal(t, plugin.StatusError, resp.Status)
			} else if resp.Status == plugin.StatusError && strings.Contains(resp.Error, "CAP_NET_RAW") {
				t.Skipf("requires CAP_NET_RAW: %s", resp.Error)
			}
		})
	}
}

// --- validateResolveTarget ---

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

// --- validateSourceIP ---

func TestValidateSourceIP_Valid(t *testing.T) {
	assert.NoError(t, validateSourceIP("192.168.1.1"))
	assert.NoError(t, validateSourceIP("::1"))
}

func TestValidateSourceIP_Invalid(t *testing.T) {
	err := validateSourceIP("not-an-ip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid IP address")
}
