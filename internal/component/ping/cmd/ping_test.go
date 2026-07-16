// Design: plan/learned/664-diag-5-active-probes.md -- ping argument parsing tests

package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingParseArgsValid(t *testing.T) {
	dest, count, timeout, opts, err := parsePingArgs([]string{"127.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", dest.String())
	assert.Equal(t, defaultPingCount, count)
	assert.Equal(t, defaultPingTimeout, timeout)
	assert.Equal(t, 0, opts.size, "size unset means the engine default")
}

func TestPingParseArgsWithCount(t *testing.T) {
	dest, count, _, _, err := parsePingArgs([]string{"10.0.0.1", "count", "3"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", dest.String())
	assert.Equal(t, 3, count)
}

func TestPingParseArgsWithTimeout(t *testing.T) {
	_, _, timeout, _, err := parsePingArgs([]string{"10.0.0.1", "timeout", "2s"})
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, timeout)
}

// TestPingParseArgsWithSize verifies `show ping <dest> size <bytes>` reaches opts.
//
// VALIDATES: the size argument parses and lands in pingOpts, which doPing uses
// to build the ICMP payload.
// PREVENTS: the web Ping tool's Packet Size field being silently dropped.
// handleShowPing used to pass a zero pingOpts and parsePingArgs had no size
// case, so an operator-chosen size never changed the packet.
func TestPingParseArgsWithSize(t *testing.T) {
	dest, _, _, opts, err := parsePingArgs([]string{"10.0.0.1", "size", "1400"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", dest.String())
	assert.Equal(t, 1400, opts.size)
}

// TestPingParseArgsSizeBounds pins the accepted size range.
//
// VALIDATES: 1 and maxPingSize are accepted; 0 and maxPingSize+1 are rejected.
// PREVENTS: a size that cannot fit an IP datagram reaching the ICMP engine, and
// drift from the range on the show/ping size leaf in ze-ping-cmd.yang.
func TestPingParseArgsSizeBounds(t *testing.T) {
	cases := []struct {
		name    string
		size    string
		want    int
		wantErr bool
	}{
		{name: "minimum", size: "1", want: 1},
		{name: "maximum", size: "65507", want: maxPingSize},
		{name: "zero", size: "0", wantErr: true},
		{name: "above maximum", size: "65508", wantErr: true},
		{name: "not a number", size: "big", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, opts, err := parsePingArgs([]string{"127.0.0.1", "size", tc.size})
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "size must be")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, opts.size)
		})
	}
}

// TestPingParseArgsSizeMissingValue verifies a trailing `size` is rejected.
//
// VALIDATES: `size` without a value errors instead of being ignored.
// PREVENTS: silently pinging with the default payload when the operator asked
// for a specific size.
func TestPingParseArgsSizeMissingValue(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "size"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "size requires a value")
}

// TestMonitorPingParseArgsInterval verifies `monitor ping` honors interval.
//
// VALIDATES: interval reaches the caller; the default applies when omitted.
// PREVENTS: the silent default this fixes. monitorPingLocal used parsePingArgs,
// which has no interval case, so `monitor ping <dest> interval 500ms` fell
// through to the destination branch and streamed at a hardcoded 1s -- while
// docs/guide/command-reference.md advertised the flag as working.
func TestMonitorPingParseArgsInterval(t *testing.T) {
	dest, interval, timeout, err := parseMonitorPingArgs([]string{"10.0.0.1", "interval", "500ms"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", dest.String())
	assert.Equal(t, 500*time.Millisecond, interval)
	assert.Equal(t, defaultPingTimeout, timeout)

	_, interval, _, err = parseMonitorPingArgs([]string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, defaultPingMonitorInterval, interval, "omitted interval uses the default")
}

// TestMonitorPingParseArgsIntervalBounds pins the accepted interval range.
//
// VALIDATES: 100ms and 30s are accepted; below/above are rejected.
// PREVENTS: drift from the interactive CLI's own bounds
// (model_ping.go parsePingMonitorArgs), which must agree so `monitor ping`
// behaves the same with and without a daemon.
func TestMonitorPingParseArgsIntervalBounds(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		wantErr bool
	}{
		{name: "minimum", val: "100ms"},
		{name: "maximum", val: "30s"},
		{name: "below minimum", val: "99ms", wantErr: true},
		{name: "above maximum", val: "31s", wantErr: true},
		{name: "not a duration", val: "soon", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := parseMonitorPingArgs([]string{"127.0.0.1", "interval", tc.val})
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "interval must be")
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestMonitorPingRejectsShowOnlyArgs verifies count and size are refused.
//
// VALIDATES: `monitor ping <dest> count N` and `... size N` error, naming the
// show ping alternative.
// PREVENTS: the operator trap this fixes. monitorPingLocal parsed both via the
// shared parsePingArgs and discarded them, so the probe silently ignored an
// explicit request. Rejecting matches the interactive CLI, whose parser already
// errors on these ("unexpected argument"). Accept-and-ignore is banned by
// ai/rules/no-workarounds-for-missing-behavior.md.
func TestMonitorPingRejectsShowOnlyArgs(t *testing.T) {
	for _, arg := range []string{"count", "size"} {
		t.Run(arg, func(t *testing.T) {
			_, _, _, err := parseMonitorPingArgs([]string{"127.0.0.1", arg, "5"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "is not supported")
			assert.Contains(t, err.Error(), "show ping", "the error must name the command that does support it")
		})
	}
}

// TestMonitorPingParseArgsErrors covers the remaining rejection paths.
//
// VALIDATES: missing destination, missing interval value, and a second
// positional argument all error.
// PREVENTS: a trailing keyword or a stray token being silently swallowed.
func TestMonitorPingParseArgsErrors(t *testing.T) {
	_, _, _, err := parseMonitorPingArgs([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing destination")

	_, _, _, err = parseMonitorPingArgs([]string{"127.0.0.1", "interval"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interval requires a value")

	_, _, _, err = parseMonitorPingArgs([]string{"127.0.0.1", "8.8.8.8"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

func TestPingParseArgsMissingDest(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing destination")
}

func TestPingParseArgsInvalidDest(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"not-an-ip"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid destination")
}

func TestPingParseArgsCountTooHigh(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "count", "200"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count must be")
}

func TestPingParseArgsCountZero(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "count", "0"})
	assert.Error(t, err)
}

func TestPingParseArgsTimeoutTooHigh(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "timeout", "60s"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be")
}

func TestPingParseArgsIPv6(t *testing.T) {
	dest, _, _, _, err := parsePingArgs([]string{"::1"})
	require.NoError(t, err)
	assert.True(t, dest.Is6())
}

// PREVENTS: garbage targets with shell metacharacters reaching DNS resolution.
func TestPingParseArgsShellMeta(t *testing.T) {
	bad := []string{
		"a;rm -rf /",
		"$(echo x)",
		"`id`",
		"host|cat",
		"host with space",
	}
	for _, target := range bad {
		t.Run(target, func(t *testing.T) {
			_, _, _, _, err := parsePingArgs([]string{target})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid destination")
		})
	}
}
