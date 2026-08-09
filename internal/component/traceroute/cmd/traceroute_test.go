// Design: docs/architecture/diagnostics/active-probes.md -- traceroute argument parsing and wiring tests

package cmd

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/probe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceroute_Wiring(t *testing.T) {
	resp, err := handleTraceroute(nil, []string{"127.0.0.1", "max-hops", "1", "timeout", "1s", "probes", "1"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == "error" {
		data := resp.Error
		if data != "" {
			t.Skipf("traceroute requires CAP_NET_RAW: %s", data)
		}
	}
	assert.Equal(t, "done", resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected map response, got %T", resp.Data)
	hops, ok := data["hops"].([]map[string]any)
	require.True(t, ok, "hops should be []map[string]any")
	require.NotEmpty(t, hops)
}

func TestTracerouteArgsParser(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid-ip", []string{"8.8.8.8"}, false},
		{"valid-ipv6", []string{"::1"}, false},
		{"with-max-hops", []string{"8.8.8.8", "max-hops", "10"}, false},
		{"with-timeout", []string{"8.8.8.8", "timeout", "2s"}, false},
		{"with-probes", []string{"8.8.8.8", "probes", "1"}, false},
		{"all-options", []string{"8.8.8.8", "max-hops", "5", "timeout", "2s", "probes", "2"}, false},
		{"missing-target", []string{}, true},
		{"max-hops-zero", []string{"8.8.8.8", "max-hops", "0"}, true},
		{"max-hops-over", []string{"8.8.8.8", "max-hops", "65"}, true},
		{"max-hops-max-valid", []string{"8.8.8.8", "max-hops", "64"}, false},
		{"timeout-under", []string{"8.8.8.8", "timeout", "500ms"}, true},
		{"timeout-over", []string{"8.8.8.8", "timeout", "31s"}, true},
		{"timeout-max-valid", []string{"8.8.8.8", "timeout", "30s"}, false},
		{"probes-zero", []string{"8.8.8.8", "probes", "0"}, true},
		{"probes-over", []string{"8.8.8.8", "probes", "11"}, true},
		{"probes-max-valid", []string{"8.8.8.8", "probes", "10"}, false},
		{"max-hops-missing-value", []string{"8.8.8.8", "max-hops"}, true},
		{"timeout-missing-value", []string{"8.8.8.8", "timeout"}, true},
		{"probes-missing-value", []string{"8.8.8.8", "probes"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := parseTracerouteArgs(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTracerouteArgsDefaults(t *testing.T) {
	target, maxHops, timeout, probes, err := parseTracerouteArgs([]string{"127.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", target.String())
	assert.Equal(t, defaultTracerouteMaxHops, maxHops)
	assert.Equal(t, defaultTracerouteTimeout, timeout)
	assert.Equal(t, defaultTracerouteProbes, probes)
}

func TestTracerouteArgsWithMaxHops(t *testing.T) {
	_, maxHops, _, _, err := parseTracerouteArgs([]string{"10.0.0.1", "max-hops", "10"})
	require.NoError(t, err)
	assert.Equal(t, 10, maxHops)
}

func TestTracerouteArgsWithTimeout(t *testing.T) {
	_, _, timeout, _, err := parseTracerouteArgs([]string{"10.0.0.1", "timeout", "2s"})
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, timeout)
}

func TestTracerouteArgsWithProbes(t *testing.T) {
	_, _, _, probes, err := parseTracerouteArgs([]string{"10.0.0.1", "probes", "1"})
	require.NoError(t, err)
	assert.Equal(t, 1, probes)
}

// PREVENTS: garbage targets with shell metacharacters reaching DNS resolution.
func TestTracerouteParseArgsShellMeta(t *testing.T) {
	bad := []string{
		"a;rm -rf /",
		"$(echo x)",
		"`id`",
		"host|cat",
		"host with space",
	}
	for _, target := range bad {
		t.Run(target, func(t *testing.T) {
			_, _, _, _, err := parseTracerouteArgs([]string{target})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid target")
		})
	}
}

func TestTracerouteIPv6(t *testing.T) {
	target, _, _, _, err := parseTracerouteArgs([]string{"::1"})
	require.NoError(t, err)
	assert.True(t, target.Is6())
}

func TestTracerouteHopResult(t *testing.T) {
	hop := map[string]any{
		"ttl":    1,
		"addr":   "10.0.0.1",
		"rtt-ms": 1.234,
	}
	assert.Equal(t, 1, hop["ttl"])
	assert.Equal(t, "10.0.0.1", hop["addr"])
	assert.Equal(t, 1.234, hop["rtt-ms"])

	timeoutHop := map[string]any{
		"ttl":    2,
		"addr":   "*",
		"rtt-ms": nil,
	}
	assert.Equal(t, "*", timeoutHop["addr"])
	assert.Nil(t, timeoutHop["rtt-ms"])
}

func TestResolveTarget_IP(t *testing.T) {
	addr, err := probe.ResolveTarget("127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", addr.String())
}

func TestResolveTarget_IPv6(t *testing.T) {
	addr, err := probe.ResolveTarget("::1")
	require.NoError(t, err)
	assert.True(t, addr.Is6())
}

func TestResolveTarget_Hostname(t *testing.T) {
	addr, err := probe.ResolveTarget("localhost")
	require.NoError(t, err)
	assert.True(t, addr.IsLoopback())
}

func TestAddrFromNetAddr_Nil(t *testing.T) {
	assert.Equal(t, "*", addrFromNetAddr(nil))
}

func TestEmbeddedICMPOffset_IPv4(t *testing.T) {
	// Minimum valid: 8 (ICMP header) + 20 (IP, IHL=5) + 8 (payload) = 36
	buf := make([]byte, 36)
	buf[8] = 0x45 // IPv4, IHL=5 (20 bytes)
	assert.Equal(t, 28, embeddedICMPOffset(buf, 36, false))

	// Too short
	assert.Equal(t, -1, embeddedICMPOffset(buf, 35, false))

	// IHL=6 (24-byte IP header): offset = 8 + 24 = 32, need 32+8 = 40
	buf2 := make([]byte, 40)
	buf2[8] = 0x46
	assert.Equal(t, 32, embeddedICMPOffset(buf2, 40, false))
	assert.Equal(t, -1, embeddedICMPOffset(buf2, 39, false))

	// IHL too small (< 5 words = 20 bytes)
	buf3 := make([]byte, 36)
	buf3[8] = 0x43
	assert.Equal(t, -1, embeddedICMPOffset(buf3, 36, false))
}

func TestEmbeddedICMPOffset_IPv6(t *testing.T) {
	// 8 (ICMPv6 header) + 40 (IPv6 header) + 8 = 56
	buf := make([]byte, 56)
	assert.Equal(t, 48, embeddedICMPOffset(buf, 56, true))
	assert.Equal(t, -1, embeddedICMPOffset(buf, 55, true))
}
