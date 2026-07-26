// Design: docs/architecture/core-design.md — BGP session hold-timer lifecycle
// Related: session.go — the OnHoldTimerExpires callback under test

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/sim"
)

// newHoldGraceSession builds a real Session (so the production
// OnHoldTimerExpires closure installed by NewSession is the code under test)
// with its timers driven by a fake clock.
func newHoldGraceSession(t *testing.T, hold time.Duration) (*Session, *sim.FakeClock) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x0a000001)
	settings.ReceiveHoldTime = hold

	s := NewSession(settings)
	fc := sim.NewFakeClock(time.Now())
	s.timers.SetClock(fc)
	s.timers.SetHoldTime(hold)
	t.Cleanup(s.timers.StopAll)

	return s, fc
}

// TestGracedHoldExpiryKeepsDeadPeerDetectionArmed drives the REAL grace branch
// in the session's hold-expiry callback (session.go OnHoldTimerExpires), not a
// test-local model of it.
//
// VALIDATES: fixit-bgp-session-fsm-lifecycle AC-2 — after a hold expiry taken
// via the grace branch (recent read activity), the hold timer is still armed,
// so dead-peer detection survives.
//
// PREVENTS: the shipped defect where the grace branch called ResetHoldTimer(),
// which early-returns on !holdRunning because fireHold had just cleared it. The
// timer was never re-armed, so the FIRST graced expiry permanently disabled
// dead-peer detection for the rest of the session's life: a peer that then went
// silent was never torn down.
//
// The pre-existing fsm-package test could not catch this: its own callback
// called GraceRearmHoldTimer directly, modeling the grace branch instead of
// exercising the production one, so it passed with the defect fully live.
func TestGracedHoldExpiryKeepsDeadPeerDetectionArmed(t *testing.T) {
	const hold = 90 * time.Second

	s, fc := newHoldGraceSession(t, hold)

	// The read loop saw traffic: the next expiry must be graced, not fatal.
	s.recentRead.Store(true)

	s.timers.StartHoldTimer()
	require.True(t, s.timers.IsHoldTimerRunning(), "precondition: hold timer armed")

	// Fire the real callback via the clock.
	fc.Add(hold)

	require.True(t, s.timers.IsHoldTimerRunning(),
		"AC-2: hold timer must still be armed after a graced expiry; "+
			"a disarmed timer means dead-peer detection is dead for this session")

	select {
	case err := <-s.errChan:
		t.Fatalf("graced expiry must not tear the session down, got %v", err)
	default:
	}
}

// TestGracedHoldExpiryStillTearsDownOnNextExpiry is the other half of the
// contract: the grace is ONE bounded reprieve, not an exemption.
//
// VALIDATES: fixit-bgp-session-fsm-lifecycle AC-1 — a session that saw recent
// read activity survives one expiry, and the next expiry with no intervening
// traffic tears it down.
//
// PREVENTS: a grace re-arm that renews itself forever, which would replace a
// dead timer (the old defect) with a peer that can never time out at all.
func TestGracedHoldExpiryStillTearsDownOnNextExpiry(t *testing.T) {
	const hold = 90 * time.Second

	s, fc := newHoldGraceSession(t, hold)

	s.recentRead.Store(true)
	s.timers.StartHoldTimer()

	// First expiry: graced (recentRead true, and consumed by the Swap).
	fc.Add(hold)
	require.True(t, s.timers.IsHoldTimerRunning(), "first expiry should be graced")

	// Second expiry: no read happened since, so this one must tear down.
	fc.Add(hold)

	require.False(t, s.timers.IsHoldTimerRunning(),
		"AC-1: the timer must not be re-armed by the teardown expiry")

	select {
	case err := <-s.errChan:
		require.ErrorIs(t, err, ErrHoldTimerExpired,
			"AC-1: the second expiry must signal hold-timer expiry")
	default:
		t.Fatal("AC-1: second expiry with no intervening read must tear the session down")
	}
}
