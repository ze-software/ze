package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// connectActiveAnyOtherEvents is every event of §8.2.2's Connect and Active
// "any other event" list that Ze declares: Events 8, 10, 11, 19, 23 and 25-28.
// Event 13, IdleHoldTimer_Expires, is the one listed event missing. §8.1.3
// marks it Optional and ties it to the optional DampPeerOscillations
// attribute, and state.go declares no such event, so nothing in Ze can raise
// it.
var connectActiveAnyOtherEvents = []Event{
	EventAutomaticStop,
	EventHoldTimerExpires,
	EventKeepaliveTimerExpires,
	EventBGPOpen,
	EventOpenCollisionDump,
	EventNotifMsg,
	EventKeepaliveMsg,
	EventUpdateMsg,
	EventUpdateMsgErr,
}

// TestRFC4271ConnectActiveAnyOtherEventCountsAndDropsToIdle verifies the
// action list Connect and Active run for every event of their "any other
// event" list.
//
// VALIDATES: Each declared listed event, delivered in Connect and in Active
// with the ConnectRetryCounter standing at 3, raises the counter to exactly 4
// and leaves the FSM in Idle. Every event starts from a fresh FSM, so one
// event's transition cannot hide another's.
//
// PREVENTS: An event of the list being given its own arm that forgets the
// counter, or being left in a state the RFC says the peer has left.
//
// RFC 4271 Section 8.2.2, Connect state: "In response to any other events
// (Events 8, 10-11, 13, 19, 23, 25-28), the local system: ... increments the
// ConnectRetryCounter by 1, ... changes its state to Idle." The Active state
// carries the same list with "increments the ConnectRetryCounter by one".
//
// RFC requirement: RFC4271-8.2.2-15 positive -- Events 8, 10, 11, 19, 23, 25,
// 26, 27 and 28, each in Connect and in Active, move the ConnectRetryCounter
// from 3 to 4 and the state to Idle (internal/component/bgp/fsm/fsm.go,
// handleConnect and handleActive). The timer, resource and TCP clauses are not
// asserted here: Ze runs no DelayOpenTimer and never starts the
// ConnectRetryTimer, and an FSM holds no connection.
func TestRFC4271ConnectActiveAnyOtherEventCountsAndDropsToIdle(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive} {
		for _, event := range connectActiveAnyOtherEvents {
			f, c := crcFSM(t, state, 3)

			// The default arm's ErrFSMError is a log signal, not part of
			// this clause, so the error is deliberately not judged here.
			_ = f.Event(event)

			require.Equal(t, uint32(4), c.Load(),
				"%s + %s must increment the ConnectRetryCounter by 1", state, event)
			require.Equal(t, StateIdle, f.State(),
				"%s + %s must change the state to Idle", state, event)
		}
	}
}

// TestRFC4271ConnectActiveUnlistedEventKeepsTheState verifies the events the
// "any other event" list leaves out do not run its action list.
//
// VALIDATES: ManualStart (Event 1) and AutomaticStart_with_DampPeerOscillations
// (Event 6), which §8.2.2 says "are ignored" in Connect and Active, leave the
// state where it was and the ConnectRetryCounter at 3. ConnectRetryTimer_Expires
// (Event 9) in Connect keeps Connect and the counter too, because its own
// paragraph ends "stays in the Connect state".
//
// PREVENTS: The incrementing catch-all spreading to events the RFC handles
// otherwise, which would count a duplicate start as a failed attempt.
//
// RFC requirement: RFC4271-8.2.2-15 negative -- Events 1 and 6 in Connect and
// in Active, and Event 9 in Connect, leave the state unchanged and the
// ConnectRetryCounter at 3 (internal/component/bgp/fsm/fsm.go, handleConnect
// and handleActive).
func TestRFC4271ConnectActiveUnlistedEventKeepsTheState(t *testing.T) {
	cases := []struct {
		state State
		event Event
	}{
		{StateConnect, EventManualStart},
		{StateConnect, EventAutomaticStartWithDampPeerOscillations},
		{StateConnect, EventConnectRetryTimerExpires},
		{StateActive, EventManualStart},
		{StateActive, EventAutomaticStartWithDampPeerOscillations},
	}
	for _, tc := range cases {
		f, c := crcFSM(t, tc.state, 3)

		require.NoError(t, f.Event(tc.event), "%s + %s is not an error", tc.state, tc.event)

		require.Equal(t, uint32(3), c.Load(),
			"%s + %s is not in the list and must leave the ConnectRetryCounter alone", tc.state, tc.event)
		require.Equal(t, tc.state, f.State(),
			"%s + %s is not in the list and must leave the state alone", tc.state, tc.event)
	}
}
