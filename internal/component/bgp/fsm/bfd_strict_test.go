package fsm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// bfdStrictFSM builds an FSM already in state, with the strict-mode flag set and
// a ConnectRetryCounter wired, so each case below can assert the counter clause
// its draft section names.
func bfdStrictFSM(t *testing.T, state State, strict bool) (*FSM, *ConnectRetryCounter) {
	t.Helper()
	f := New()
	crc := &ConnectRetryCounter{}
	f.SetConnectRetryCounter(crc)
	f.SetBFDStrict(strict)
	f.setState(state)
	return f, crc
}

// TestBFDStrictOpenSentHoldsUntilUp is the core of the feature: a session in the
// OpenSentBfdUpPending sub-state stays in OpenSent until a BFD Up event arrives.
//
// VALIDATES: EnterBfdUpPending leaves the state at OpenSent and records the
// sub-state; EventBfdUp then moves the FSM to OpenConfirm and clears it.
//
// PREVENTS: A strict peer establishing while the forwarding path is unproven,
// which is the whole failure draft-ietf-idr-bgp-bfd-strict-mode exists to stop.
func TestBFDStrictOpenSentHoldsUntilUp(t *testing.T) {
	f, crc := bfdStrictFSM(t, StateOpenSent, true)

	require.NoError(t, f.EnterBfdUpPending())
	require.Equal(t, StateOpenSent, f.State(), "the FSM stays in OpenSent while it waits")
	require.Equal(t, SubStateOpenSentBfdUpPending, f.BfdSubState())

	require.NoError(t, f.Event(EventBfdUp))
	require.Equal(t, StateOpenConfirm, f.State(), "draft Section 8.5.1 changes state to OpenConfirm")
	require.Equal(t, SubStateNone, f.BfdSubState(), "leaving OpenSent leaves the sub-state")
	require.Equal(t, uint32(0), crc.Load(), "an attempt that is succeeding touches no counter")
}

// TestBFDStrictAdminDownAndDisabledRelease covers the other two events draft
// Section 8.5.1 groups with BfdUp.
//
// VALIDATES: BfdAdminDown and BfdDisabled release a pending sub-state exactly as
// BfdUp does.
//
// PREVENTS: A peer held down forever because BFD was administratively disabled
// or the operator turned BFD off, neither of which says anything about the path.
func TestBFDStrictAdminDownAndDisabledRelease(t *testing.T) {
	for _, event := range []Event{EventBfdAdminDown, EventBfdDisabled} {
		t.Run(event.String(), func(t *testing.T) {
			f, _ := bfdStrictFSM(t, StateOpenSent, true)
			require.NoError(t, f.EnterBfdUpPending())
			require.NoError(t, f.Event(event))
			require.Equal(t, StateOpenConfirm, f.State())
		})
	}
}

// TestBFDStrictKeepaliveBeforeUp is draft Section 8.5.6: the remote BFD session
// can come Up first, so the peer's KEEPALIVE can arrive while ze still waits.
//
// VALIDATES: Event 26 in OpenSentBfdUpPending moves to
// OpenSentConfirmedBfdUpPending without leaving OpenSent, and the next BfdUp
// goes straight to Established.
//
// PREVENTS: Treating that KEEPALIVE as the FSM error the unmodified RFC 4271
// OpenSent state calls it, which would drop a session that is behaving.
func TestBFDStrictKeepaliveBeforeUp(t *testing.T) {
	f, crc := bfdStrictFSM(t, StateOpenSent, true)
	require.NoError(t, f.EnterBfdUpPending())

	require.NoError(t, f.Event(EventKeepaliveMsg))
	require.Equal(t, StateOpenSent, f.State(), "the confirmed sub-state is still OpenSent")
	require.Equal(t, SubStateOpenSentConfirmedBfdUpPending, f.BfdSubState())

	require.NoError(t, f.Event(EventBfdUp))
	require.Equal(t, StateEstablished, f.State(), "draft Section 8.5.1 goes straight to Established")
	require.Equal(t, uint32(0), crc.Load())
}

// TestBFDStrictKeepaliveInOpenSentStaysAnError holds the negative half of draft
// Section 8.5.6: the new arm must not soften the unmodified FSM.
//
// VALIDATES: A KEEPALIVE in OpenSent is still an FSM error when strict mode is
// not negotiated, and when it is negotiated but no sub-state is pending.
//
// PREVENTS: The strict-mode arm accepting an out-of-order KEEPALIVE on every
// session, which would remove an RFC 4271 error check from the whole daemon.
func TestBFDStrictKeepaliveInOpenSentStaysAnError(t *testing.T) {
	cases := []struct {
		name   string
		strict bool
	}{
		{"strict not negotiated", false},
		{"strict negotiated, nothing pending", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, crc := bfdStrictFSM(t, StateOpenSent, tc.strict)
			require.ErrorIs(t, f.Event(EventKeepaliveMsg), ErrFSMError)
			require.Equal(t, StateIdle, f.State())
			require.Equal(t, uint32(1), crc.Load(), "the FSM-error clause increments by 1")
		})
	}
}

// TestBFDStrictSecondOpenIsAnFSMError is draft Section 8.5.5's first branch.
//
// VALIDATES: An OPEN received while a pending sub-state is set drops to Idle
// with ErrFSMError and increments the ConnectRetryCounter.
//
// PREVENTS: A second OPEN re-running negotiation on a session that is mid-wait.
func TestBFDStrictSecondOpenIsAnFSMError(t *testing.T) {
	f, crc := bfdStrictFSM(t, StateOpenSent, true)
	require.NoError(t, f.EnterBfdUpPending())

	require.ErrorIs(t, f.Event(EventBGPOpen), ErrFSMError)
	require.Equal(t, StateIdle, f.State())
	require.Equal(t, uint32(1), crc.Load())
}

// TestBFDStrictDownConnectRetryCounter pins the counter direction the draft
// deliberately splits between its states.
//
// VALIDATES: BfdDown ZEROES the counter in OpenSent and OpenConfirm (Sections
// 8.5.2 and 8.6.2) and INCREMENTS it in Established (Section 8.7.2).
//
// PREVENTS: One arm being written for all three, which would make the retry
// history wrong in two of them and is the mistake the draft's own wording
// invites.
func TestBFDStrictDownConnectRetryCounter(t *testing.T) {
	cases := []struct {
		state State
		want  uint32
	}{
		{StateOpenSent, 0},
		{StateOpenConfirm, 0},
		{StateEstablished, 8},
	}
	for _, tc := range cases {
		t.Run(tc.state.String(), func(t *testing.T) {
			f, crc := bfdStrictFSM(t, tc.state, true)
			for range 7 {
				crc.Increment()
			}
			require.NoError(t, f.Event(EventBfdDown))
			require.Equal(t, StateIdle, f.State())
			require.Equal(t, tc.want, crc.Load())
		})
	}
}

// TestBFDStrictDownIgnoredWhereTheDraftSaysSo is the negative half of the event
// that closes sessions.
//
// VALIDATES: BfdDown changes nothing in Connect and Active (Sections 8.3.2 and
// 8.4.2), and nothing in OpenSent or OpenConfirm when strict mode is not
// negotiated (Sections 8.5.2 and 8.6.2 "else, stays in the ... State").
//
// PREVENTS: A BFD session that is merely starting up -- Down is where every BFD
// session begins -- tearing a BGP session that never asked for strict mode.
func TestBFDStrictDownIgnoredWhereTheDraftSaysSo(t *testing.T) {
	cases := []struct {
		state  State
		strict bool
	}{
		{StateConnect, true},
		{StateActive, true},
		{StateOpenSent, false},
		{StateOpenConfirm, false},
	}
	for _, tc := range cases {
		t.Run(tc.state.String(), func(t *testing.T) {
			f, crc := bfdStrictFSM(t, tc.state, tc.strict)
			require.NoError(t, f.Event(EventBfdDown))
			require.Equal(t, tc.state, f.State(), "the draft says the local system stays put")
			require.Equal(t, uint32(0), crc.Load())
		})
	}
}

// TestBFDStrictHoldTimerExpires is draft Sections 8.3.3, 8.4.3 and 8.5.3.
//
// VALIDATES: Event 34 drops to Idle and INCREMENTS the counter, which is the
// opposite direction from BfdDown one paragraph above it in Section 8.5.
//
// PREVENTS: A session that waited out its whole BfdHoldTime being recorded as no
// attempt at all.
func TestBFDStrictHoldTimerExpires(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive, StateOpenSent} {
		t.Run(state.String(), func(t *testing.T) {
			f, crc := bfdStrictFSM(t, state, true)
			require.NoError(t, f.Event(EventBfdHoldTimerExpires))
			require.Equal(t, StateIdle, f.State())
			require.Equal(t, uint32(1), crc.Load())
		})
	}
}

// TestBFDStrictEstablishedIgnoresTheThreeUpEvents is draft Section 8.7.1, and it
// is the clause that changed ze's behavior: AdminDown used to tear the session.
//
// VALIDATES: BfdAdminDown, BfdDisabled and BfdUp leave an Established session
// Established, and the ConnectRetryCounter untouched.
//
// PREVENTS: An operator disabling BFD for maintenance dropping every BGP session
// that used it, which RFC 5882 Section 4.2 forbids: "clients SHOULD NOT take any
// control protocol action".
func TestBFDStrictEstablishedIgnoresTheThreeUpEvents(t *testing.T) {
	for _, event := range []Event{EventBfdAdminDown, EventBfdDisabled, EventBfdUp} {
		t.Run(event.String(), func(t *testing.T) {
			f, crc := bfdStrictFSM(t, StateEstablished, true)
			require.NoError(t, f.Event(event))
			require.Equal(t, StateEstablished, f.State())
			require.Equal(t, uint32(0), crc.Load())
		})
	}
}

// TestBFDStrictIdleIgnoresEverything is draft Section 8.2.
//
// VALIDATES: All six events leave an Idle FSM in Idle with no error, which
// matters because a strict peer's BFD session outlives its BGP connections
// (Section 7) and so delivers events while the FSM is Idle.
//
// PREVENTS: The Idle default arm being reached by an event that has a clause of
// its own.
func TestBFDStrictIdleIgnoresEverything(t *testing.T) {
	events := []Event{
		EventBfdAdminDown, EventBfdDown, EventBfdUp,
		EventBfdDisabled, EventBfdHoldTimerExpires, EventBfdStrictConfigChanged,
	}
	for _, event := range events {
		t.Run(event.String(), func(t *testing.T) {
			f, crc := bfdStrictFSM(t, StateIdle, true)
			require.NoError(t, f.Event(event))
			require.Equal(t, StateIdle, f.State())
			require.Equal(t, uint32(0), crc.Load())
		})
	}
}

// TestBFDStrictConfigChangedResetsToIdle is draft Sections 8.3.4, 8.4.4, 8.5.4,
// 8.6.3 and 8.7.3.
//
// VALIDATES: Event 35 drops every pre-Established state to Idle with the counter
// zeroed, and is ignored in Established.
//
// PREVENTS: A configuration change leaving the session "lingering on a pending
// event", which is the reason Section 8.1 gives for the event existing.
func TestBFDStrictConfigChangedResetsToIdle(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive, StateOpenSent, StateOpenConfirm} {
		t.Run(state.String(), func(t *testing.T) {
			f, crc := bfdStrictFSM(t, state, true)
			crc.Increment()
			require.NoError(t, f.Event(EventBfdStrictConfigChanged))
			require.Equal(t, StateIdle, f.State())
			require.Equal(t, uint32(0), crc.Load(), "the clause sets ConnectRetryCounter to zero")
		})
	}

	f, crc := bfdStrictFSM(t, StateEstablished, true)
	crc.Increment()
	require.NoError(t, f.Event(EventBfdStrictConfigChanged))
	require.Equal(t, StateEstablished, f.State(), "Section 8.7.3 ignores it in Established")
	require.Equal(t, uint32(1), crc.Load())
}

// TestBFDStrictConnectAndActiveStayPut is draft Sections 8.3.1 and 8.4.1, read
// through the fact that ze never enters the two DelayOpen sub-states.
//
// VALIDATES: The three release events leave Connect and Active unchanged.
//
// PREVENTS: These events reaching the default arm, which answers with an FSM
// error and a dropped connection attempt.
func TestBFDStrictConnectAndActiveStayPut(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive} {
		for _, event := range []Event{EventBfdAdminDown, EventBfdDisabled, EventBfdUp} {
			t.Run(state.String()+"/"+event.String(), func(t *testing.T) {
				f, _ := bfdStrictFSM(t, state, true)
				require.NoError(t, f.Event(event))
				require.Equal(t, state, f.State())
			})
		}
	}
}

// TestBFDStrictOpenConfirmIgnoresTheThreeUpEvents is draft Section 8.6.1.
//
// VALIDATES: BfdAdminDown, BfdDisabled and BfdUp leave OpenConfirm alone.
//
// PREVENTS: A late BFD Up disturbing a session whose wait is already over.
func TestBFDStrictOpenConfirmIgnoresTheThreeUpEvents(t *testing.T) {
	for _, event := range []Event{EventBfdAdminDown, EventBfdDisabled, EventBfdUp} {
		t.Run(event.String(), func(t *testing.T) {
			f, _ := bfdStrictFSM(t, StateOpenConfirm, true)
			require.NoError(t, f.Event(event))
			require.Equal(t, StateOpenConfirm, f.State())
		})
	}
}

// TestEnterBfdUpPendingRefusesOutsideOpenSent guards the one sub-state ze
// declares against being set where the draft has no sub-state at all.
//
// VALIDATES: EnterBfdUpPending returns ErrFSMError and writes nothing in every
// state but OpenSent.
//
// PREVENTS: A sub-state nobody can leave, because change() only clears it on a
// transition out of OpenSent.
func TestEnterBfdUpPendingRefusesOutsideOpenSent(t *testing.T) {
	for _, state := range []State{StateIdle, StateConnect, StateActive, StateOpenConfirm, StateEstablished} {
		t.Run(state.String(), func(t *testing.T) {
			f, _ := bfdStrictFSM(t, state, true)
			require.ErrorIs(t, f.EnterBfdUpPending(), ErrFSMError)
			require.Equal(t, SubStateNone, f.BfdSubState())
			require.Equal(t, state, f.State())
		})
	}
}

// TestBfdHoldTimerFiresAtBfdHoldTime is the timer half of draft Section 3
// attribute 18 and Section 8.5.5.
//
// VALIDATES: The timer fires once at BfdHoldTime, a zero configuration restores
// the draft's 30-second default, and StopBfdHoldTimer disarms it.
//
// PREVENTS: A strict session with a negotiated hold time of zero waiting for BFD
// forever, and a stopped timer still firing.
func TestBfdHoldTimerFiresAtBfdHoldTime(t *testing.T) {
	timers, fc := newFakeTimers(0)
	require.Equal(t, DefaultBfdHoldTime, timers.BfdHoldTime(), "an unset BfdHoldTime is the draft default")

	fires := 0
	timers.OnBfdHoldTimerExpires(func() { fires++ })
	timers.SetBfdHoldTime(5 * time.Second)
	timers.StartBfdHoldTimer()
	require.True(t, timers.IsBfdHoldTimerRunning())

	fc.Add(4999 * time.Millisecond)
	require.Equal(t, 0, fires)
	fc.Add(time.Millisecond)
	require.Equal(t, 1, fires)
	require.False(t, timers.IsBfdHoldTimerRunning())

	timers.SetBfdHoldTime(0)
	require.Equal(t, DefaultBfdHoldTime, timers.BfdHoldTime(), "zero restores the draft default, never disables")

	timers.StartBfdHoldTimer()
	timers.StopBfdHoldTimer()
	fc.Add(time.Hour)
	require.Equal(t, 1, fires, "a stopped BfdHoldTimer never fires")
}

// TestBfdHoldTimerStopsOnTransitionToIdle is the one-line rule draft Sections
// 8.3, 8.4 and 8.5 each restate: "The BfdHoldTimer is reset to zero and stopped
// on any transition to the Idle state."
//
// VALIDATES: A running BfdHoldTimer is stopped by the FSM's own transition to
// Idle, without any handler asking for it.
//
// PREVENTS: A timer surviving into the next connection cycle and closing a
// session that had nothing to do with it.
func TestBfdHoldTimerStopsOnTransitionToIdle(t *testing.T) {
	timers, fc := newFakeTimers(0)
	fires := 0
	timers.OnBfdHoldTimerExpires(func() { fires++ })

	f, _ := bfdStrictFSM(t, StateOpenSent, true)
	f.SetTimers(timers)
	require.NoError(t, f.EnterBfdUpPending())
	timers.StartBfdHoldTimer()

	require.NoError(t, f.Event(EventBfdDown))
	require.Equal(t, StateIdle, f.State())
	require.False(t, timers.IsBfdHoldTimerRunning())

	fc.Add(time.Hour)
	require.Equal(t, 0, fires)
}
