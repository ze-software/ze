// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured IPv6CP timing.
// Related: pppoe_client_timing.go -- schema defaults and next-session snapshots.

package iface

import (
	"log/slog"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// ipv6cpTimingEntry parses the operator's string-valued timing and lets the
// registered YANG schema populate all omitted timing containers.
func ipv6cpTimingEntry(t *testing.T, interval string) (pppoeClientEntry, map[string]any, error) {
	t.Helper()
	entry := map[string]any{
		"source-interface": "eth0",
		"authentication":   map[string]any{"username": "user", "password": "pass"},
	}
	if interval != "" {
		entry["ipv6cp"] = map[string]any{"restart-timer": interval}
	}
	parsed, err := parsePPPoEClientEntry("pppoe0", entry, testPPPoEClientSchema(t))
	return parsed, entry, err
}

// TestPPPoEClientIPv6CPDefault reads both the effective value and the populated
// YANG default, so a missed container cannot masquerade as a valid zero.
func TestPPPoEClientIPv6CPDefault(t *testing.T) {
	parsed, entry, err := ipv6cpTimingEntry(t, "")
	require.NoError(t, err)
	block, ok := entry["ipv6cp"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "3000", block["restart-timer"])
	require.Equal(t, 3*time.Second, parsed.Timing.IPv6CPRestartTimer)
}

// TestPPPoEClientIPv6CPBounds checks the valid boundaries and the first value
// outside each one through the real config reader.
func TestPPPoEClientIPv6CPBounds(t *testing.T) {
	for _, interval := range []int{100, 60000} {
		entry, _, err := ipv6cpTimingEntry(t, strconv.Itoa(interval))
		require.NoError(t, err)
		require.Equal(t, time.Duration(interval)*time.Millisecond, entry.Timing.IPv6CPRestartTimer)
	}
	for _, interval := range []string{"0", "99", "60001", "500.5", "no"} {
		_, _, err := ipv6cpTimingEntry(t, interval)
		require.ErrorContains(t, err, "ipv6cp restart-timer")
	}
}

// TestPPPoEClientIPv6CPRefusesProgrammaticTiming checks the second boundary
// for callers constructing a config without the YANG parser.
func TestPPPoEClientIPv6CPRefusesProgrammaticTiming(t *testing.T) {
	entry, _, err := ipv6cpTimingEntry(t, "500")
	require.NoError(t, err)
	for _, period := range []time.Duration{0, 99 * time.Millisecond, 60001 * time.Millisecond, 100*time.Millisecond + time.Nanosecond} {
		timing := entry.Timing
		timing.IPv6CPRestartTimer = period
		require.ErrorContains(t, timing.Validate(), "ipv6cp restart-timer")
	}
}

// TestPPPoEClientIPv6CPReloadKeepsSnapshot reaches Dial through the client
// reconciler, then proves timing reload changes exactly the next Dial.
func TestPPPoEClientIPv6CPReloadKeepsSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		done := make(chan struct{})
		session := pppoeTestSession(done)
		d := &timingDialer{session: &session}
		entry, _, err := ipv6cpTimingEntry(t, "")
		require.NoError(t, err)
		active := startTimingClients(t, &ifaceConfig{PPPoEClient: []pppoeClientEntry{entry}}, d)
		synctest.Wait()
		running := active["pppoe0"]
		changed, _, err := ipv6cpTimingEntry(t, "500")
		require.NoError(t, err)
		reconcilePPPoEClients(&ifaceConfig{PPPoEClient: []pppoeClientEntry{changed}}, active, nil, slog.Default(), nil)
		synctest.Wait()
		require.Same(t, running, active["pppoe0"])
		configs, _ := d.snapshot()
		require.Len(t, configs, 1)
		require.Equal(t, 3*time.Second, configs[0].Timing.IPv6CPRestartTimer)
		close(done)
		// sleep(test-clock): expire the configured one-second reconnect hold-off.
		time.Sleep(time.Second)
		synctest.Wait()
		configs, _ = d.snapshot()
		require.Len(t, configs, 2)
		require.Equal(t, 500*time.Millisecond, configs[1].Timing.IPv6CPRestartTimer)
	})
}
