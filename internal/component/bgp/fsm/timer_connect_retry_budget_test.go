// Design: docs/architecture/behavior/peer-lifecycle.md -- retry budget transfer at session retirement.
package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConnectRetryBudgetSurvivesExpiryUntilStopped proves that transferring a
// retry neither grants cleanup a new interval nor loses an already due retry.
// Method: choose a jitter endpoint, advance the actual timer clock through
// expiry, then cancel, rearm and retire the timer owner.
func TestConnectRetryBudgetSurvivesExpiryUntilStopped(t *testing.T) {
	timers, fc := newFakeTimers(0)
	t.Cleanup(timers.Retire)
	timers.SetConnectRetryTime(8 * time.Second)
	timers.random = func(int64) int64 { return 0 }

	budget := timers.ConnectRetryBudget()
	require.Nil(t, budget)
	timers.StartConnectRetryTimer()
	budget = timers.ConnectRetryBudget()
	require.NotNil(t, budget)
	require.Equal(t, 6*time.Second, budget.Remaining())
	fc.Add(2 * time.Second)
	budget = timers.ConnectRetryBudget()
	require.NotNil(t, budget)
	require.Equal(t, 4*time.Second, budget.Remaining())
	fc.Add(5 * time.Second)
	require.False(t, timers.IsConnectRetryTimerRunning())
	budget = timers.ConnectRetryBudget()
	require.NotNil(t, budget, "an expired arm still owes its immediate retry")
	require.Zero(t, budget.Remaining())

	timers.StopConnectRetryTimer()
	budget = timers.ConnectRetryBudget()
	require.Nil(t, budget, "cancellation removes the transferable budget")
	timers.StartConnectRetryTimer()
	budget = timers.ConnectRetryBudget()
	require.NotNil(t, budget)
	require.Equal(t, 6*time.Second, budget.Remaining(), "rearming starts a fresh interval")
	timers.Retire()
	require.Nil(t, timers.ConnectRetryBudget())
	require.Equal(t, 6*time.Second, budget.Remaining(), "a transferred budget outlives its timer owner")
	fc.Add(2 * time.Second)
	require.Equal(t, 4*time.Second, budget.Remaining(), "a transferred budget keeps measuring its original clock")
	timers.StartConnectRetryTimer()
	require.Nil(t, timers.ConnectRetryBudget(), "retired timer owners cannot resurrect a retry")
}
