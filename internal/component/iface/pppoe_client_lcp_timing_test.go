// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured LCP timing.
// Related: pppoe_client_timing.go -- schema defaults, validation and session handoff.

package iface

import (
	"log/slog"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// lcpTimingEntry passes string-valued configuration through the production
// parser. The parser fills absent containers from the registered YANG schema.
func lcpTimingEntry(t *testing.T, lcp map[string]any) (pppoeClientEntry, map[string]any, error) {
	t.Helper()
	entry := map[string]any{
		"source-interface": "eth0",
		"authentication":   map[string]any{"username": "user", "password": "pass"},
	}
	if lcp != nil {
		entry["lcp"] = lcp
	}
	parsed, err := parsePPPoEClientEntry("pppoe0", entry, testPPPoEClientSchema(t))
	return parsed, entry, err
}

func TestPPPoEClientLCPDefaults(t *testing.T) {
	_, entry, err := lcpTimingEntry(t, nil)
	require.NoError(t, err)
	lcp, ok := entry["lcp"].(map[string]any)
	require.True(t, ok, "the absent LCP block must receive its schema defaults")
	require.Equal(t, "3000", lcp["restart-timer"])
	require.Equal(t, "10", lcp["max-configure"])
}

func TestPPPoEClientLCPBounds(t *testing.T) {
	for _, leaf := range []struct {
		name     string
		min, max int
	}{
		{"restart-timer", 100, 60000},
		{"max-configure", 1, 255},
	} {
		t.Run(leaf.name, func(t *testing.T) {
			for _, value := range []int{leaf.min, leaf.max} {
				_, _, err := lcpTimingEntry(t, map[string]any{leaf.name: strconv.Itoa(value)})
				require.NoError(t, err, "%s %d must be accepted", leaf.name, value)
			}
			for _, value := range []int{leaf.min - 1, leaf.max + 1} {
				_, _, err := lcpTimingEntry(t, map[string]any{leaf.name: strconv.Itoa(value)})
				require.Error(t, err, "%s %d must be refused", leaf.name, value)
				require.Contains(t, err.Error(), "lcp "+leaf.name)
			}
		})
	}
}

func TestPPPoEClientLCPTimingFromConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entry, _, err := lcpTimingEntry(t, map[string]any{"restart-timer": "500", "max-configure": "3"})
		require.NoError(t, err)
		d := &timingDialer{}
		startTimingClients(t, &ifaceConfig{PPPoEClient: []pppoeClientEntry{entry}}, d)
		synctest.Wait()
		configs, _ := d.snapshot()
		require.NotEmpty(t, configs)
		require.Equal(t, 500*time.Millisecond, configs[0].Timing.LCPRestartTimer)
		require.Equal(t, 3, configs[0].Timing.LCPMaxConfigure)
	})
}

func TestPPPoEClientLCPReloadKeepsSessionSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		done := make(chan struct{})
		session := pppoeTestSession(done)
		d := &timingDialer{session: &session}
		entry, _, err := lcpTimingEntry(t, nil)
		require.NoError(t, err)
		active := startTimingClients(t, &ifaceConfig{PPPoEClient: []pppoeClientEntry{entry}}, d)
		synctest.Wait()
		running := active["pppoe0"]
		changed, _, err := lcpTimingEntry(t, map[string]any{"restart-timer": "500", "max-configure": "3"})
		require.NoError(t, err)
		reconcilePPPoEClients(&ifaceConfig{PPPoEClient: []pppoeClientEntry{changed}}, active, nil, slog.Default(), nil)
		synctest.Wait()
		require.Same(t, running, active["pppoe0"])
		configs, _ := d.snapshot()
		require.Len(t, configs, 1, "an LCP timing reload must not interrupt the session")
		require.Equal(t, 3*time.Second, configs[0].Timing.LCPRestartTimer, "the existing Dial retains its snapshot")
		require.Equal(t, 10, configs[0].Timing.LCPMaxConfigure)
		close(done)
		// sleep(test-clock): expire the existing one-second reconnect wait after the session ends.
		time.Sleep(time.Second)
		synctest.Wait()
		configs, _ = d.snapshot()
		require.Len(t, configs, 2)
		require.Equal(t, 500*time.Millisecond, configs[1].Timing.LCPRestartTimer)
		require.Equal(t, 3, configs[1].Timing.LCPMaxConfigure)
	})
}

func TestPPPoEClientLCPValidateRefusesInvalidTiming(t *testing.T) {
	for _, interval := range []time.Duration{0, 99 * time.Millisecond, 60001 * time.Millisecond, 100*time.Millisecond + time.Nanosecond} {
		timing := testPPPoETiming
		timing.LCPRestartTimer = interval
		require.ErrorContains(t, timing.Validate(), "lcp restart-timer")
	}
	for _, count := range []int{0, 256} {
		timing := testPPPoETiming
		timing.LCPMaxConfigure = count
		require.ErrorContains(t, timing.Validate(), "lcp max-configure")
	}
}
