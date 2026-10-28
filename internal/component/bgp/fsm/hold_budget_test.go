package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/test/sim"
)

// VALIDATES: HoldBudget observes the armed deadline, not the configured duration.
// PREVENTS: A reload borrowing time from an expired, stopped, or nearly spent timer.
func TestHoldBudgetTracksArmedExpiry(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	timers := NewTimers()
	timers.SetClock(fc)
	t.Cleanup(timers.StopAll)
	timers.SetHoldTime(10 * time.Second)
	require.Equal(t, HoldBudgetUnarmed, timers.HoldBudget().State)
	fired := 0
	timers.OnHoldTimerExpires(func() { fired++ })
	timers.StartHoldTimer()
	expires := fc.Now().Add(10 * time.Second)
	fc.Add(7 * time.Second)
	budget := timers.HoldBudget()
	require.Equal(t, HoldBudgetArmed, budget.State)
	require.Equal(t, 3*time.Second, budget.Remaining)
	require.Equal(t, expires, budget.Expires)
	// Changing configuration is not a restart of the armed timer.
	timers.SetHoldTime(90 * time.Second)
	require.Equal(t, budget, timers.HoldBudget())
	fc.Add(3 * time.Second)
	require.Equal(t, 1, fired)
	require.Equal(t, HoldBudgetExpired, timers.HoldBudget().State)
	require.Zero(t, timers.HoldBudget().Remaining)
	timers.ResetHoldTimer()
	require.Equal(t, HoldBudgetExpired, timers.HoldBudget().State)
	fc.Add(time.Second)
	require.Equal(t, 1, fired)
	timers.StopHoldTimer()
	require.Equal(t, HoldBudgetUnarmed, timers.HoldBudget().State)
	timers.SetHoldTime(0)
	require.Equal(t, HoldBudgetDisabled, timers.HoldBudget().State)
}

// VALIDATES: A legitimate message-driven reset publishes its new live deadline.
func TestHoldBudgetFollowsReset(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	timers := NewTimers()
	timers.SetClock(fc)
	t.Cleanup(timers.StopAll)
	timers.SetHoldTime(10 * time.Second)
	timers.StartHoldTimer()
	fc.Add(7 * time.Second)
	timers.ResetHoldTimer()
	require.Equal(t, 10*time.Second, timers.HoldBudget().Remaining)
	require.Equal(t, fc.Now().Add(10*time.Second), timers.HoldBudget().Expires)
}
