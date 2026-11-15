// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- configured peer-option retry limits.
// Related: pppoe_client_timing.go -- schema defaults and session snapshots.

package iface

import (
	"errors"
	"log/slog"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestPPPoEClientLCPMaxFailureFromConfig checks the declared default and both
// valid limits through the production parser and the Dial configuration.
func TestPPPoEClientLCPMaxFailureFromConfig(t *testing.T) {
	for _, maximum := range []int{0, 1, 255} {
		t.Run(strconv.Itoa(maximum), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var block map[string]any
				want := maximum
				if maximum == 0 {
					want = 5
				} else {
					block = map[string]any{"max-failure": strconv.Itoa(maximum)}
				}
				entry, populated, err := lcpTimingEntry(t, block)
				require.NoError(t, err)
				lcp, ok := populated["lcp"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, strconv.Itoa(want), lcp["max-failure"])
				d := &timingDialer{}
				startTimingClients(t, &ifaceConfig{PPPoEClient: []pppoeClientEntry{entry}}, d)
				synctest.Wait()
				configs, _ := d.snapshot()
				require.NotEmpty(t, configs)
				require.Equal(t, want, configs[0].Timing.LCPMaxFailure)
			})
		})
	}
}

// TestPPPoEClientLCPMaxFailureRefusesInvalidValues checks malformed provider
// values and the second boundary for programmatic timing snapshots.
func TestPPPoEClientLCPMaxFailureRefusesInvalidValues(t *testing.T) {
	for _, value := range []string{"0", "256", "-1", "1.5", "invalid"} {
		_, _, err := lcpTimingEntry(t, map[string]any{"max-failure": value})
		require.ErrorContains(t, err, "lcp max-failure")
		var refusal *opererr.Error
		require.True(t, errors.As(err, &refusal))
		require.Equal(t, "set interface pppoe-client pppoe0 lcp max-failure 5", refusal.Fix())
	}
	entry, _, err := lcpTimingEntry(t, nil)
	require.NoError(t, err)
	for _, value := range []int{0, -1, 256} {
		timing := entry.Timing
		timing.LCPMaxFailure = value
		require.ErrorContains(t, timing.Validate(), "lcp max-failure")
	}
}

// TestPPPoEClientLCPMaxFailureReloadKeepsSnapshot proves a changed limit
// retains the current session and reaches exactly its replacement Dial.
func TestPPPoEClientLCPMaxFailureReloadKeepsSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		done := make(chan struct{})
		session := pppoeTestSession(done)
		d := &timingDialer{session: &session}
		entry, _, err := lcpTimingEntry(t, nil)
		require.NoError(t, err)
		active := startTimingClients(t, &ifaceConfig{PPPoEClient: []pppoeClientEntry{entry}}, d)
		synctest.Wait()
		running := active["pppoe0"]
		changed, _, err := lcpTimingEntry(t, map[string]any{"max-failure": "2"})
		require.NoError(t, err)
		reconcilePPPoEClients(&ifaceConfig{PPPoEClient: []pppoeClientEntry{changed}}, active, nil, slog.Default(), nil)
		synctest.Wait()
		require.Same(t, running, active["pppoe0"])
		configs, _ := d.snapshot()
		require.Len(t, configs, 1)
		require.Equal(t, 5, configs[0].Timing.LCPMaxFailure)
		close(done)
		// sleep(test-clock): expire the configured one-second reconnect wait.
		time.Sleep(time.Second)
		synctest.Wait()
		configs, _ = d.snapshot()
		require.Len(t, configs, 2)
		require.Equal(t, 2, configs[1].Timing.LCPMaxFailure)
	})
}
