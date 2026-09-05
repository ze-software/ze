package cmd

import (
	"strings"
	"testing"
	"time"

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

// --- parseSourceIP ---

func TestParseSourceIP_Valid(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"192.168.1.1", "192.168.1.1"},
		{"::1", "::1"},
		{"fe80::1%eth0", "fe80::1%eth0"},
		{"::ffff:192.168.1.1", "192.168.1.1"},
	} {
		addr, err := parseSourceIP(tt.input)
		require.NoError(t, err, tt.input)
		assert.Equal(t, tt.want, addr.String())
	}
}

func TestParseSourceIP_Invalid(t *testing.T) {
	addr, err := parseSourceIP("not-an-ip")
	require.Error(t, err)
	assert.False(t, addr.IsValid(), "a rejected source must not come back as an address")
	assert.Contains(t, err.Error(), "not a valid IP address")
}

// TestHandleResolveTraceroute_SourceFamilyDrivesResolution verifies the source
// address is read BEFORE the target is resolved, and constrains the family the
// target resolves in. A conflict is reported as the conflict, naming the source
// and the target, instead of reaching the socket and failing at the bind.
func TestHandleResolveTraceroute_SourceFamilyDrivesResolution(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name:     "v6-source-v4-target",
			args:     []string{"192.0.2.1", "source", "2001:db8::1"},
			contains: []string{"source 2001:db8::1 is IPv6", "target \"192.0.2.1\" has no IPv6 address"},
		},
		{
			name:     "v4-source-v6-target",
			args:     []string{"2001:db8::1", "source", "192.0.2.1"},
			contains: []string{"source 192.0.2.1 is IPv4", "target \"2001:db8::1\" has no IPv4 address"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleResolveTraceroute(&pluginserver.CommandContext{}, tt.args)
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusError, resp.Status)
			for _, want := range tt.contains {
				assert.Contains(t, resp.Error, want)
			}
			assert.NotContains(t, resp.Error, "CAP_NET_RAW", "the conflict must be named before any socket is opened")
		})
	}
}

// TestParseResolveTracerouteArgs verifies the option parser that now runs
// before resolution still reads every keyword, and carries the source out as a
// typed address the family hint is derived from.
func TestParseResolveTracerouteArgs(t *testing.T) {
	req, errResp := parseResolveTracerouteArgs([]string{"example.com", "source", "2001:db8::1", "max-hops", "5", "timeout", "2s", "probes", "1"})
	require.Nil(t, errResp)
	assert.Equal(t, "2001:db8::1", req.source.String())
	assert.Equal(t, 5, req.maxHops)
	assert.Equal(t, 2*time.Second, req.timeout)
	assert.Equal(t, 1, req.probes)

	req, errResp = parseResolveTracerouteArgs([]string{"example.com"})
	require.Nil(t, errResp)
	assert.False(t, req.source.IsValid(), "no source keyword leaves the family unconstrained")
	assert.Equal(t, defaultTracerouteMaxHops, req.maxHops)
	assert.Equal(t, defaultTracerouteTimeout, req.timeout)
	assert.Equal(t, defaultTracerouteProbes, req.probes)
}
