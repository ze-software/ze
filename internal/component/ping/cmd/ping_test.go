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
	mp, err := parseMonitorPingArgs([]string{"10.0.0.1", "interval", "500ms"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", mp.Dest.String())
	assert.Equal(t, 500*time.Millisecond, mp.Interval)
	assert.Equal(t, defaultPingTimeout, mp.Timeout)

	mp, err = parseMonitorPingArgs([]string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, defaultPingMonitorInterval, mp.Interval, "omitted interval uses the default")
	assert.Equal(t, 0, mp.Count, "omitted count streams until interrupted")
	assert.Equal(t, 0, mp.Size, "omitted size uses the engine default payload")
}

// TestMonitorPingParseArgsCountAndSize verifies both reach the caller.
//
// VALIDATES: `monitor ping <dest> count 5 size 1400` parses both (AC: the
// streaming session bounds its probes and carries the payload).
// PREVENTS: the trap this fixes -- monitorPingLocal parsed both via the shared
// parsePingArgs and then discarded them, so an explicit request silently did
// nothing. Accept-and-ignore is banned by
// ai/rules/no-workarounds-for-missing-behavior.md.
func TestMonitorPingParseArgsCountAndSize(t *testing.T) {
	mp, err := parseMonitorPingArgs([]string{"10.0.0.1", "count", "5", "size", "1400"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", mp.Dest.String())
	assert.Equal(t, 5, mp.Count)
	assert.Equal(t, 1400, mp.Size)
}

// TestMonitorPingParseArgsBounds pins the accepted ranges.
//
// VALIDATES: interval 100ms-30s, count 1-100, size 1-65507; each rejects
// outside its range.
// PREVENTS: drift from the interactive CLI's own bounds (model_ping.go
// parsePingMonitorArgs), which must agree so `monitor ping` behaves the same
// with and without a daemon.
func TestMonitorPingParseArgsBounds(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "interval minimum", args: []string{"interval", "100ms"}},
		{name: "interval maximum", args: []string{"interval", "30s"}},
		{name: "interval below", args: []string{"interval", "99ms"}, wantErr: "interval must be"},
		{name: "interval above", args: []string{"interval", "31s"}, wantErr: "interval must be"},
		{name: "interval not a duration", args: []string{"interval", "soon"}, wantErr: "interval must be"},
		{name: "count minimum", args: []string{"count", "1"}},
		{name: "count maximum", args: []string{"count", "100"}},
		{name: "count zero", args: []string{"count", "0"}, wantErr: "count must be"},
		{name: "count above", args: []string{"count", "101"}, wantErr: "count must be"},
		{name: "size minimum", args: []string{"size", "1"}},
		{name: "size maximum", args: []string{"size", "65507"}},
		{name: "size zero", args: []string{"size", "0"}, wantErr: "size must be"},
		{name: "size above", args: []string{"size", "65508"}, wantErr: "size must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseMonitorPingArgs(append([]string{"127.0.0.1"}, tc.args...))
			if tc.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestMonitorPingParseArgsErrors covers the remaining rejection paths.
//
// VALIDATES: missing destination, a trailing keyword, and a second positional
// argument all error.
// PREVENTS: a trailing keyword or a stray token being silently swallowed.
func TestMonitorPingParseArgsErrors(t *testing.T) {
	_, err := parseMonitorPingArgs([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing destination")

	for _, arg := range []string{"interval", "timeout", "count", "size"} {
		_, err = parseMonitorPingArgs([]string{"127.0.0.1", arg})
		assert.Error(t, err, "trailing %s must error", arg)
		assert.Contains(t, err.Error(), "requires a value")
	}

	_, err = parseMonitorPingArgs([]string{"127.0.0.1", "8.8.8.8"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

// TestPingPayload verifies the bytes both ping engines put on the wire.
//
// VALIDATES: size 0 sends the default marker; size N sends exactly N bytes with
// the marker at the front.
// PREVENTS: `size N` reaching the parser but not the packet. This is the one
// part of the size path that cannot be driven end-to-end here -- raw ICMP needs
// CAP_NET_RAW, so doPing/streamPing cannot open a socket unprivileged -- and it
// is shared by both, so a regression would silently affect show ping and
// monitor ping together.
func TestPingPayload(t *testing.T) {
	assert.Equal(t, []byte("ze-ping"), pingPayload(0), "size 0 uses the default marker")
	assert.Equal(t, []byte("ze-ping"), pingPayload(-1), "negative size is treated as unset")

	p := pingPayload(1400)
	assert.Len(t, p, 1400, "payload is exactly the requested size")
	assert.Equal(t, []byte("ze-ping"), p[:7], "marker is copied to the front")
	assert.Equal(t, make([]byte, 1393), p[7:], "remainder is zero-filled")

	assert.Len(t, pingPayload(1), 1, "a 1-byte payload truncates the marker")
	assert.Len(t, pingPayload(maxPingSize), maxPingSize)
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
