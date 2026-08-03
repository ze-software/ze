package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// crcFSM builds an FSM parked in `state` with a ConnectRetryCounter already
// standing at `start`, so a test can tell "reset to zero" from "was never
// raised" and "incremented" from "left alone".
func crcFSM(t *testing.T, state State, start uint32) (*FSM, *ConnectRetryCounter) {
	t.Helper()
	f := New()
	c := &ConnectRetryCounter{}
	for range start {
		c.Increment()
	}
	require.Equal(t, start, c.Load(), "counter seeding")
	f.SetConnectRetryCounter(c)
	f.setState(state)
	return f, c
}

// allNonIdleStates is every state whose §8.2.2 paragraph has an action list.
// Idle's events either transition without a teardown or are explicitly
// ignored, which is why it is not here.
var allNonIdleStates = []State{
	StateConnect, StateActive, StateOpenSent, StateOpenConfirm, StateEstablished,
}

// TestRFC4271ConnectRetryCounterZeroedOnManualStart verifies an operator start
// clears the retry history.
//
// VALIDATES: Idle + ManualStart drives the counter to zero on both branches --
// the active one (to Connect) and the passive one (to Active) -- each of which
// the RFC gives its own, identically worded clause.
//
// PREVENTS: A peer the operator just restarted reporting the retry count of
// the run before it.
//
// RFC requirement: RFC4271-8.2.2-7 positive -- handleIdle's EventManualStart
// arm calls f.crc.Reset() before choosing the branch, so both the Events 1/3
// and the Events 4/5 clause are satisfied by one line
// (internal/component/bgp/fsm/fsm.go, handleIdle).
func TestRFC4271ConnectRetryCounterZeroedOnManualStart(t *testing.T) {
	for _, passive := range []bool{false, true} {
		f, c := crcFSM(t, StateIdle, 7)
		f.SetPassive(passive)

		require.NoError(t, f.Event(EventManualStart))
		require.Equal(t, uint32(0), c.Load(),
			"passive=%v: ManualStart in Idle must set ConnectRetryCounter to zero", passive)

		want := StateConnect
		if passive {
			want = StateActive
		}
		require.Equal(t, want, f.State(), "passive=%v: start branch", passive)
	}
}

// TestRFC4271ConnectRetryCounterSurvivesDampedStart verifies the events the RFC
// gives no counter clause leave the counter standing.
//
// VALIDATES: A damped automatic restart (Event 6) leaves the count untouched,
// and so does a start event arriving in Connect or Active, where §8.2.2 says
// the start events "are ignored".
//
// PREVENTS: The reset clause spreading to every path out of Idle, which is the
// failure that makes the counter structurally incapable of ever reading more
// than one -- ze fires a start event on every reconnect cycle because each
// cycle builds a fresh FSM.
//
// RFC requirement: RFC4271-8.2.2-7 negative -- only handleIdle's
// EventManualStart arm calls Reset. The
// EventAutomaticStartWithDampPeerOscillations arm beside it deliberately does
// not, matching §8.2.2's Idle text, which gives Events 6, 7 and 13 no action
// list at all (internal/component/bgp/fsm/fsm.go, handleIdle, handleConnect,
// handleActive).
func TestRFC4271ConnectRetryCounterSurvivesDampedStart(t *testing.T) {
	f, c := crcFSM(t, StateIdle, 7)
	require.NoError(t, f.Event(EventAutomaticStartWithDampPeerOscillations))
	require.Equal(t, uint32(7), c.Load(),
		"a damped automatic restart has no ConnectRetryCounter clause")
	require.Equal(t, StateConnect, f.State())

	for _, st := range []State{StateConnect, StateActive} {
		for _, ev := range []Event{EventManualStart, EventAutomaticStartWithDampPeerOscillations} {
			f, c := crcFSM(t, st, 7)
			require.NoError(t, f.Event(ev))
			require.Equal(t, uint32(7), c.Load(),
				"%s + %s: start events are ignored in this state", st, ev)
			require.Equal(t, st, f.State(), "%s + %s: ignored means no state change", st, ev)
		}
	}
}

// TestRFC4271ConnectRetryCounterZeroedOnManualStop verifies an operator stop
// clears the retry history in every state that can see the event.
//
// VALIDATES: ManualStop drives the counter to zero from Connect, Active,
// OpenSent, OpenConfirm and Established alike.
//
// PREVENTS: A peer the operator shut down and brought back reporting the
// attempts of the previous run.
//
// RFC requirement: RFC4271-8.2.2-8 positive -- each of the five handlers has
// its own EventManualStop arm and each calls f.crc.Reset()
// (internal/component/bgp/fsm/fsm.go, handleConnect, handleActive,
// handleOpenSent, handleOpenConfirm, handleEstablished).
func TestRFC4271ConnectRetryCounterZeroedOnManualStop(t *testing.T) {
	for _, st := range allNonIdleStates {
		f, c := crcFSM(t, st, 9)
		require.NoError(t, f.Event(EventManualStop))
		require.Equal(t, uint32(0), c.Load(),
			"%s + ManualStop must set ConnectRetryCounter to zero", st)
		require.Equal(t, StateIdle, f.State(), "%s + ManualStop", st)
	}
}

// TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop verifies the one
// state where ManualStop carries no clause.
//
// VALIDATES: ManualStop in Idle leaves the counter where it was.
//
// PREVENTS: A stray stop against an already-idle peer erasing the retry
// history that explains why it is idle.
//
// RFC requirement: RFC4271-8.2.2-8 negative -- handleIdle's EventManualStop
// arm is empty, matching §8.2.2's "The ManualStop event (Event 2) and
// AutomaticStop (Event 8) event are ignored in the Idle state"
// (internal/component/bgp/fsm/fsm.go, handleIdle).
func TestRFC4271ConnectRetryCounterNotZeroedByIdleManualStop(t *testing.T) {
	f, c := crcFSM(t, StateIdle, 9)
	require.NoError(t, f.Event(EventManualStop))
	require.Equal(t, uint32(9), c.Load(),
		"ManualStop is ignored in Idle, so the counter is untouched")
	require.Equal(t, StateIdle, f.State())
}

// TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry verifies a session
// lost to silence is counted as a failed attempt.
//
// VALIDATES: HoldTimer_Expires raises the counter by exactly one in OpenSent,
// OpenConfirm and Established.
//
// PREVENTS: A peer that repeatedly goes silent looking, to the counter, like a
// peer that never had trouble.
//
// RFC requirement: RFC4271-8.2.2-9 positive -- each of the three handlers has
// an EventHoldTimerExpires arm calling f.crc.Increment()
// (internal/component/bgp/fsm/fsm.go, handleOpenSent, handleOpenConfirm,
// handleEstablished).
func TestRFC4271ConnectRetryCounterIncrementsOnHoldTimerExpiry(t *testing.T) {
	for _, st := range []State{StateOpenSent, StateOpenConfirm, StateEstablished} {
		f, c := crcFSM(t, st, 3)
		require.NoError(t, f.Event(EventHoldTimerExpires))
		require.Equal(t, uint32(4), c.Load(),
			"%s + HoldTimerExpires must increment ConnectRetryCounter by exactly 1", st)
		require.Equal(t, StateIdle, f.State(), "%s + HoldTimerExpires", st)
	}
}

// TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic verifies the
// events that keep a session alive cost it nothing.
//
// VALIDATES: KEEPALIVE and UPDATE in Established, KEEPALIVE in OpenConfirm,
// OPEN in OpenSent, and a hold expiry in Idle all leave the counter alone.
//
// PREVENTS: An increment landing on the success path, where a busy healthy
// session would climb the counter faster than a broken one.
//
// RFC requirement: RFC4271-8.2.2-9 negative -- the Event 26, 27 and 19 arms
// carry no counter mutation because their §8.2.2 action lists carry no
// ConnectRetryCounter line, and handleIdle's default arm is a deliberate
// RFC-mandated ignore rather than a teardown
// (internal/component/bgp/fsm/fsm.go, handleEstablished, handleOpenConfirm,
// handleOpenSent, handleIdle).
func TestRFC4271ConnectRetryCounterQuietOnHealthyEstablishedTraffic(t *testing.T) {
	quiet := []struct {
		state State
		event Event
	}{
		{StateEstablished, EventKeepaliveMsg},
		{StateEstablished, EventUpdateMsg},
		{StateEstablished, EventKeepaliveTimerExpires},
		{StateOpenConfirm, EventKeepaliveMsg},
		{StateOpenConfirm, EventKeepaliveTimerExpires},
		{StateOpenSent, EventBGPOpen},
		{StateIdle, EventHoldTimerExpires},
	}
	for _, q := range quiet {
		f, c := crcFSM(t, q.state, 3)
		require.NoError(t, f.Event(q.event))
		require.Equal(t, uint32(3), c.Load(),
			"%s + %s has no ConnectRetryCounter clause", q.state, q.event)
	}
}

// TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors verifies a
// malformed header or OPEN is counted as a failed attempt everywhere.
//
// VALIDATES: BGPHeaderErr and BGPOpenMsgErr each raise the counter by one in
// all five non-Idle states.
//
// PREVENTS: A peer sending malformed messages on every reconnect looking
// healthy to the counter.
//
// RFC requirement: RFC4271-8.2.2-10 positive -- Events 21 and 22 reach an
// incrementing arm in every non-Idle state: an explicit shared arm in
// handleConnect, handleActive, handleOpenSent and handleOpenConfirm, and in
// handleEstablished the explicit EventBGPHeaderErr arm plus the default arm
// that Event 22 lands in (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterIncrementsOnHeaderAndOpenErrors(t *testing.T) {
	for _, st := range allNonIdleStates {
		for _, ev := range []Event{EventBGPHeaderErr, EventBGPOpenMsgErr} {
			f, c := crcFSM(t, st, 0)
			_ = f.Event(ev) // Established + BGPOpenMsgErr is an FSM error; the counter is the assertion
			require.Equal(t, uint32(1), c.Load(),
				"%s + %s must increment ConnectRetryCounter by 1", st, ev)
			require.Equal(t, StateIdle, f.State(), "%s + %s", st, ev)
		}
	}
}

// TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors verifies message
// errors arriving with no session behind them cost nothing.
//
// VALIDATES: BGPHeaderErr, BGPOpenMsgErr and NotifMsg in Idle leave the
// counter alone.
//
// PREVENTS: Late events on a torn-down session inflating a count that is
// supposed to record connection attempts.
//
// RFC requirement: RFC4271-8.2.2-10 negative -- handleIdle's default arm
// returns without touching the counter, matching §8.2.2's "Any other event
// (Events 9-12, 15-28) received in the Idle state does not cause change in the
// state of the local system" (internal/component/bgp/fsm/fsm.go, handleIdle).
func TestRFC4271ConnectRetryCounterNotIncrementedByIdleErrors(t *testing.T) {
	for _, ev := range []Event{EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsg, EventUpdateMsgErr} {
		f, c := crcFSM(t, StateIdle, 2)
		require.NoError(t, f.Event(ev))
		require.Equal(t, uint32(2), c.Load(), "Idle + %s is ignored, counter untouched", ev)
		require.Equal(t, StateIdle, f.State())
	}
}

// TestRFC4271ConnectRetryCounterIncrementsOnNotification verifies a peer that
// tears the session down is counted.
//
// VALIDATES: NotifMsg (Event 25) raises the counter by one in all five
// non-Idle states.
//
// PREVENTS: A peer that answers every attempt with a NOTIFICATION reading as
// zero attempts.
//
// RFC requirement: RFC4271-8.2.2-11 positive -- Event 25 reaches an
// incrementing arm in every non-Idle state: the shared error arm in
// handleConnect and handleActive, its own arm in handleOpenSent and
// handleOpenConfirm, and the grouped Event 24/25 arm in handleEstablished
// (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterIncrementsOnNotification(t *testing.T) {
	for _, st := range allNonIdleStates {
		f, c := crcFSM(t, st, 0)
		require.NoError(t, f.Event(EventNotifMsg),
			"%s + NotifMsg is a handled teardown, not an FSM error", st)
		require.Equal(t, uint32(1), c.Load(),
			"%s + NotifMsg must increment ConnectRetryCounter by 1", st)
		require.Equal(t, StateIdle, f.State(), "%s + NotifMsg", st)
	}
}

// TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification verifies the
// step size, not merely the direction.
//
// VALIDATES: Ten NOTIFICATION teardowns of the same peer leave the counter at
// ten, never at twenty and never at one.
//
// PREVENTS: An arm double-counting because two producers both increment, and
// an arm that saturates at one because it stores rather than adds.
//
// RFC requirement: RFC4271-8.2.2-11 negative -- Increment adds one and returns
// the new value, and no §8.2.2 arm calls it twice, so a run of N teardowns
// reads exactly N (internal/component/bgp/fsm/connect_retry_counter.go,
// Increment).
func TestRFC4271ConnectRetryCounterStepsByExactlyOnePerNotification(t *testing.T) {
	c := &ConnectRetryCounter{}
	for i := uint32(1); i <= 10; i++ {
		f := New()
		f.SetConnectRetryCounter(c)
		f.setState(StateEstablished)
		require.NoError(t, f.Event(EventNotifMsg))
		require.Equal(t, i, c.Load(), "attempt %d", i)
	}
}

// TestRFC4271ConnectRetryCounterOnVersionErrorPerState verifies the split the
// RFC draws for Event 24.
//
// VALIDATES: NotifMsgVerErr increments in Connect, Active and Established, and
// does not in OpenSent or OpenConfirm.
//
// PREVENTS: Grouping Event 24 with Event 25 in a single handler arm, which was
// the shipped shape and makes one of the two events wrong in OpenSent and
// OpenConfirm whichever way the arm is written.
//
// RFC requirement: RFC4271-8.2.2-12 positive -- Event 24 shares the
// incrementing error arm in handleConnect and handleActive (its clause there
// sits in the DelayOpenTimer-is-not-running branch, the only branch ze can
// take) and the grouped Event 24/25 arm in handleEstablished
// (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterOnVersionErrorPerState(t *testing.T) {
	for _, st := range []State{StateConnect, StateActive, StateEstablished} {
		f, c := crcFSM(t, st, 0)
		require.NoError(t, f.Event(EventNotifMsgVerErr))
		require.Equal(t, uint32(1), c.Load(),
			"%s + NotifMsgVerErr must increment ConnectRetryCounter by 1", st)
		require.Equal(t, StateIdle, f.State(), "%s + NotifMsgVerErr", st)
	}
}

// TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates verifies the
// two states where Event 24's action list has no counter line.
//
// VALIDATES: NotifMsgVerErr in OpenSent and in OpenConfirm tears the session
// down without touching the counter.
//
// PREVENTS: Re-merging Event 24 into the neighboring incrementing arm, which
// would over-count every version-mismatched peer once per handshake.
//
// RFC requirement: RFC4271-8.2.2-12 negative -- handleOpenSent and
// handleOpenConfirm each give EventNotifMsgVerErr its own arm with no counter
// mutation, matching the four-item action lists in §8.2.2's OpenSent and
// OpenConfirm text (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterQuietOnVersionErrorInOpenStates(t *testing.T) {
	for _, st := range []State{StateOpenSent, StateOpenConfirm} {
		f, c := crcFSM(t, st, 4)
		require.NoError(t, f.Event(EventNotifMsgVerErr))
		require.Equal(t, uint32(4), c.Load(),
			"%s + NotifMsgVerErr has no ConnectRetryCounter clause", st)
		require.Equal(t, StateIdle, f.State(), "%s + NotifMsgVerErr still tears down", st)
	}
}

// TestRFC4271ConnectRetryCounterOnTCPFailurePerState verifies the split the
// RFC draws for Event 18.
//
// VALIDATES: TcpConnectionFails increments in Active, OpenConfirm and
// Established.
//
// PREVENTS: A peer whose TCP connection keeps dropping mid-session reading as
// zero attempts.
//
// RFC requirement: RFC4271-8.2.2-13 positive -- handleActive,
// handleOpenConfirm and handleEstablished each call f.crc.Increment() in their
// EventTCPConnectionFails arm (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterOnTCPFailurePerState(t *testing.T) {
	for _, st := range []State{StateActive, StateOpenConfirm, StateEstablished} {
		f, c := crcFSM(t, st, 0)
		require.NoError(t, f.Event(EventTCPConnectionFails))
		require.Equal(t, uint32(1), c.Load(),
			"%s + TCPConnectionFails must increment ConnectRetryCounter by 1", st)
		require.Equal(t, StateIdle, f.State(), "%s + TCPConnectionFails", st)
	}
}

// TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent verifies
// the two states where Event 18's action list has no counter line.
//
// VALIDATES: TcpConnectionFails in Connect and in OpenSent leaves the counter
// alone.
//
// PREVENTS: Making Event 18 increment uniformly, which would count a dial that
// never reached a peer twice -- once here and once when the peer-level
// reconnect loop tries again.
//
// RFC requirement: RFC4271-8.2.2-13 negative -- handleConnect and
// handleOpenSent give EventTCPConnectionFails an arm with no counter
// mutation. Connect's §8.2.2 Event 18 text has two branches and neither
// carries the clause; OpenSent's leaves for Active rather than tearing the
// peering down (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent(t *testing.T) {
	for _, st := range []State{StateConnect, StateOpenSent} {
		f, c := crcFSM(t, st, 4)
		require.NoError(t, f.Event(EventTCPConnectionFails))
		require.Equal(t, uint32(4), c.Load(),
			"%s + TCPConnectionFails has no ConnectRetryCounter clause", st)
		require.Equal(t, StateIdle, f.State(), "%s + TCPConnectionFails still leaves the state", st)
	}
}

// TestRFC4271ConnectRetryCounterIncrementsOnUpdateError verifies a bad UPDATE
// is counted as a failed attempt.
//
// VALIDATES: UpdateMsgErr in Established raises the counter by one.
//
// PREVENTS: A peer whose UPDATEs are rejected on every session reading as a
// peer that never failed.
//
// RFC requirement: RFC4271-8.2.2-14 positive -- handleEstablished's
// EventUpdateMsgErr arm calls f.crc.Increment()
// (internal/component/bgp/fsm/fsm.go, handleEstablished).
func TestRFC4271ConnectRetryCounterIncrementsOnUpdateError(t *testing.T) {
	f, c := crcFSM(t, StateEstablished, 2)
	require.NoError(t, f.Event(EventUpdateMsgErr))
	require.Equal(t, uint32(3), c.Load(),
		"Established + UpdateMsgErr must increment ConnectRetryCounter by 1")
	require.Equal(t, StateIdle, f.State())
}

// TestRFC4271ConnectRetryCounterQuietOnGoodUpdate verifies the neighboring
// event that must not be counted.
//
// VALIDATES: A valid UPDATE in Established leaves the counter alone and leaves
// the session up.
//
// PREVENTS: The Event 28 increment being written one arm too high, where every
// route a healthy peer sends would raise the retry count.
//
// RFC requirement: RFC4271-8.2.2-14 negative -- handleEstablished's
// EventUpdateMsg arm restarts the HoldTimer and nothing else; §8.2.2's Event
// 27 action list has three items and no ConnectRetryCounter line
// (internal/component/bgp/fsm/fsm.go, handleEstablished).
func TestRFC4271ConnectRetryCounterQuietOnGoodUpdate(t *testing.T) {
	f, c := crcFSM(t, StateEstablished, 2)
	for range 5 {
		require.NoError(t, f.Event(EventUpdateMsg))
	}
	require.Equal(t, uint32(2), c.Load(), "a valid UPDATE has no ConnectRetryCounter clause")
	require.Equal(t, StateEstablished, f.State())
}

// TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent verifies the catch-all
// action list of every non-Idle state.
//
// VALIDATES: An event that lands in each state's "any other event" arm raises
// the counter by one and drops to Idle.
//
// PREVENTS: A new event, or an event removed from an explicit arm, falling
// through to the default and being torn down without being counted.
//
// RFC requirement: RFC4271-8.2.2-15 positive -- the default arm of
// handleConnect, handleActive, handleOpenSent, handleOpenConfirm and
// handleEstablished each call f.crc.Increment() before change(StateIdle)
// (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterIncrementsOnAnyOtherEvent(t *testing.T) {
	defaultArm := []struct {
		state State
		event Event
	}{
		{StateConnect, EventKeepaliveMsg},
		{StateActive, EventKeepaliveMsg},
		{StateOpenSent, EventUpdateMsg},
		{StateOpenConfirm, EventUpdateMsg},
		{StateEstablished, EventConnectRetryTimerExpires},
	}
	for _, d := range defaultArm {
		f, c := crcFSM(t, d.state, 0)
		require.ErrorIs(t, f.Event(d.event), ErrFSMError,
			"%s + %s must land in the error default arm", d.state, d.event)
		require.Equal(t, uint32(1), c.Load(),
			"%s + %s must increment ConnectRetryCounter by 1", d.state, d.event)
		require.Equal(t, StateIdle, f.State(), "%s + %s", d.state, d.event)
	}
}

// TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing verifies the one
// state whose catch-all has no action list.
//
// VALIDATES: Every event Idle does not name leaves both the state and the
// counter unchanged.
//
// PREVENTS: Copying the incrementing default arm into handleIdle, which would
// let a peer that is not even connected climb the counter on every stray
// event.
//
// RFC requirement: RFC4271-8.2.2-15 negative -- handleIdle's default arm
// returns immediately, matching §8.2.2's "Any other event (Events 9-12, 15-28)
// received in the Idle state does not cause change in the state of the local
// system", which lists no actions at all
// (internal/component/bgp/fsm/fsm.go, handleIdle).
func TestRFC4271ConnectRetryCounterIdleDefaultArmCountsNothing(t *testing.T) {
	f, c := crcFSM(t, StateIdle, 6)
	for _, ev := range []Event{
		EventConnectRetryTimerExpires, EventHoldTimerExpires, EventKeepaliveTimerExpires,
		EventTCPConnectionConfirmed, EventTCPConnectionFails, EventBGPOpen,
		EventBGPHeaderErr, EventBGPOpenMsgErr, EventNotifMsgVerErr, EventNotifMsg,
		EventKeepaliveMsg, EventUpdateMsg, EventUpdateMsgErr,
	} {
		require.NoError(t, f.Event(ev), "Idle + %s is a deliberate ignore", ev)
	}
	require.Equal(t, uint32(6), c.Load(), "no Idle event may move the ConnectRetryCounter")
	require.Equal(t, StateIdle, f.State())
}

// TestConnectRetryCounterNilIsSafe verifies an FSM with no counter wired is
// legal.
//
// VALIDATES: Every mutation and read tolerates a nil counter, which is the
// state of every pure-FSM unit test and of any FSM built before its owner
// hands one over.
//
// PREVENTS: A nil dereference in the FSM handlers when SetConnectRetryCounter
// was never called.
func TestConnectRetryCounterNilIsSafe(t *testing.T) {
	var c *ConnectRetryCounter
	require.Equal(t, uint32(0), c.Increment())
	require.Equal(t, uint32(0), c.Load())
	c.Reset()

	f := New()
	require.Nil(t, f.crc, "a fresh FSM counts nothing until an owner wires a counter in")
	for _, st := range allNonIdleStates {
		f.setState(st)
		_ = f.Event(EventNotifMsg)
	}
	f.setState(StateIdle)
	require.NoError(t, f.Event(EventManualStart))
}

// TestConnectRetryCounterSaturatesRatherThanWrapping verifies the counter never
// reads as "never retried" after a very long run.
//
// VALIDATES: At math.MaxUint32 a further Increment returns the same value
// instead of wrapping to zero.
//
// PREVENTS: A peer with a pathological reconnect rate reporting a fresh
// counter, which reads as the opposite of what it means.
func TestConnectRetryCounterSaturatesRatherThanWrapping(t *testing.T) {
	c := &ConnectRetryCounter{}
	c.n.Store(^uint32(0) - 1)
	require.Equal(t, ^uint32(0), c.Increment(), "one below the ceiling still steps")
	require.Equal(t, ^uint32(0), c.Increment(), "at the ceiling it holds")
	require.Equal(t, ^uint32(0), c.Load())
	c.Reset()
	require.Equal(t, uint32(0), c.Load(), "Reset still works from the ceiling")
}

// TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop verifies a stop the
// system chose is counted as a failed attempt.
//
// VALIDATES: AutomaticStop raises the counter by one in every non-Idle state.
//
// PREVENTS: Routing a BFD-down or out-of-resources teardown through the
// operator's ManualStop, which ZEROES the counter. That failure does not just
// miss a count, it erases the history the counter exists to keep.
//
// RFC requirement: RFC4271-8.2.2-16 positive -- handleOpenSent,
// handleOpenConfirm and handleEstablished each give EventAutomaticStop its own
// incrementing arm, and handleConnect and handleActive reach the same clause
// through the shared Event 8/23 arm the RFC's "any other events" list covers
// (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterIncrementsOnAutomaticStop(t *testing.T) {
	for _, st := range allNonIdleStates {
		f, c := crcFSM(t, st, 4)
		require.NoError(t, f.Event(EventAutomaticStop))
		require.Equal(t, uint32(5), c.Load(),
			"%s + AutomaticStop must increment ConnectRetryCounter by 1", st)
		require.Equal(t, StateIdle, f.State(), "%s + AutomaticStop", st)
	}
}

// TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop verifies the two
// stop events stay apart.
//
// VALIDATES: From the same state and the same starting count, ManualStop and
// AutomaticStop leave the counter at opposite ends, and AutomaticStop is
// ignored in Idle exactly as ManualStop is.
//
// PREVENTS: Collapsing Event 8 back into Event 2 because "they both tear the
// session down". They do; §8.2.2 gives them different action lists, and this
// is the line that differs.
//
// RFC requirement: RFC4271-8.2.2-16 negative -- Event 8 never resets, and in
// Idle it takes the same do-nothing arm as Event 2, per §8.2.2's "The
// ManualStop event (Event 2) and AutomaticStop (Event 8) event are ignored in
// the Idle state" (internal/component/bgp/fsm/fsm.go, handleIdle).
func TestRFC4271ConnectRetryCounterAutomaticStopIsNotAManualStop(t *testing.T) {
	for _, st := range allNonIdleStates {
		manual, mc := crcFSM(t, st, 6)
		require.NoError(t, manual.Event(EventManualStop))

		auto, ac := crcFSM(t, st, 6)
		require.NoError(t, auto.Event(EventAutomaticStop))

		require.Equal(t, uint32(0), mc.Load(), "%s: ManualStop zeroes", st)
		require.Equal(t, uint32(7), ac.Load(), "%s: AutomaticStop increments", st)
	}

	idle, ic := crcFSM(t, StateIdle, 6)
	require.NoError(t, idle.Event(EventAutomaticStop))
	require.Equal(t, uint32(6), ic.Load(), "AutomaticStop is ignored in Idle")
	require.Equal(t, StateIdle, idle.State())
}

// TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump verifies a
// connection lost to collision resolution is counted.
//
// VALIDATES: OpenCollisionDump raises the counter by one in every non-Idle
// state.
//
// PREVENTS: The RFC 6.8 collision close reaching the FSM as ManualStop, which
// would zero the counter every time two speakers race a connection.
//
// RFC requirement: RFC4271-8.2.2-17 positive -- handleOpenSent,
// handleOpenConfirm and handleEstablished each give EventOpenCollisionDump its
// own incrementing arm, and handleConnect and handleActive reach the same
// clause through the shared Event 8/23 arm
// (internal/component/bgp/fsm/fsm.go).
func TestRFC4271ConnectRetryCounterIncrementsOnOpenCollisionDump(t *testing.T) {
	for _, st := range allNonIdleStates {
		f, c := crcFSM(t, st, 4)
		require.NoError(t, f.Event(EventOpenCollisionDump))
		require.Equal(t, uint32(5), c.Load(),
			"%s + OpenCollisionDump must increment ConnectRetryCounter by 1", st)
		require.Equal(t, StateIdle, f.State(), "%s + OpenCollisionDump", st)
	}
}

// TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle verifies the one
// state where Event 23 carries no action list.
//
// VALIDATES: OpenCollisionDump in Idle changes neither the state nor the
// counter.
//
// PREVENTS: A late collision-dump event on a peer that already went Idle
// inflating a count that is meant to record connection attempts.
//
// RFC requirement: RFC4271-8.2.2-17 negative -- handleIdle has no Event 23
// case, so it takes the default arm, which §8.2.2 defines as "Any other event
// (Events 9-12, 15-28) received in the Idle state does not cause change in the
// state of the local system", with no actions
// (internal/component/bgp/fsm/fsm.go, handleIdle).
func TestRFC4271ConnectRetryCounterCollisionDumpIsQuietInIdle(t *testing.T) {
	f, c := crcFSM(t, StateIdle, 4)
	require.NoError(t, f.Event(EventOpenCollisionDump))
	require.Equal(t, uint32(4), c.Load(), "Idle + OpenCollisionDump is ignored")
	require.Equal(t, StateIdle, f.State())
}
