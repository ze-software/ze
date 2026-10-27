// Design: docs/architecture/behavior/fsm.md — RFC 4271 DelayOpen state actions.
package fsm

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/sim"
)

// TestRFC4271DelayOpenHoldsOpen compares the optional path with the zero default.
func TestRFC4271DelayOpenHoldsOpen(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive} {
		for _, delay := range []time.Duration{0, time.Second, 240 * time.Second} {
			f, _ := crcFSM(t, state, 3)
			timers := NewTimers()
			t.Cleanup(timers.StopAll)
			timers.SetDelayOpenTime(delay)
			f.SetTimers(timers)
			if err := f.Event(EventTCPConnectionConfirmed); err != nil {
				t.Fatal(err)
			}
			want := state
			if delay == 0 {
				want = StateOpenSent
			}
			if f.State() != want || timers.IsDelayOpenTimerRunning() != (delay != 0) {
				t.Fatalf("state=%s delay=%s: state=%s running=%v", state, delay, f.State(), timers.IsDelayOpenTimerRunning())
			}
			if timers.IsConnectRetryTimerRunning() {
				t.Fatal("Event 17 left ConnectRetryTimer running")
			}
		}
	}
}

// TestRFC4271DelayOpenEventsConnect checks each Connect action and counter clause.
func TestRFC4271DelayOpenEventsConnect(t *testing.T) { testDelayOpenEvents(t, StateConnect) }

// TestRFC4271DelayOpenEventsActive checks the corresponding Active action clauses.
func TestRFC4271DelayOpenEventsActive(t *testing.T) { testDelayOpenEvents(t, StateActive) }

func testDelayOpenEvents(t *testing.T, state State) {
	t.Helper()
	for _, event := range []Event{EventDelayOpenTimerExpires, EventBGPOpenWithDelayOpenTimer,
		EventTCPConnectionFails, EventNotifMsgVerErr, EventAutomaticStop, EventOpenCollisionDump,
		EventHoldTimerExpires, EventKeepaliveTimerExpires, EventBGPOpen, EventNotifMsg,
		EventKeepaliveMsg, EventUpdateMsg, EventUpdateMsgErr, EventBGPHeaderErr, EventBGPOpenMsgErr,
		EventManualStop} {
		t.Run(event.String(), func(t *testing.T) {
			f, counter := crcFSM(t, state, 3)
			timers := NewTimers()
			t.Cleanup(timers.StopAll)
			timers.SetDelayOpenTime(time.Minute)
			f.SetTimers(timers)
			if err := f.Event(EventTCPConnectionConfirmed); err != nil {
				t.Fatal(err)
			}
			err := f.Event(event)
			if err != nil && !errors.Is(err, ErrFSMError) {
				t.Fatal(err)
			}
			wantState, wantCounter, retry := StateIdle, uint32(4), false
			//exhaustive:ignore — only events with a non-default expectation.
			switch event {
			case EventDelayOpenTimerExpires:
				wantState, wantCounter = StateOpenSent, 3
			case EventBGPOpenWithDelayOpenTimer:
				wantState, wantCounter = StateOpenConfirm, 3
				if !timers.IsHoldTimerRunning() || !timers.IsKeepaliveTimerRunning() {
					t.Fatal("Event 20 failed to arm negotiated timers")
				}
			case EventNotifMsgVerErr:
				wantCounter = 3
			case EventManualStop:
				wantCounter = 0
			case EventTCPConnectionFails:
				retry = true
				if state == StateConnect {
					wantState, wantCounter = StateActive, 3
				}
			}
			if f.State() != wantState || counter.Load() != wantCounter {
				t.Fatalf("got %s counter %d; want %s counter %d", f.State(), counter.Load(), wantState, wantCounter)
			}
			if timers.IsDelayOpenTimerRunning() || timers.IsConnectRetryTimerRunning() != retry {
				t.Fatalf("timer cleanup: delay=%v retry=%v", timers.IsDelayOpenTimerRunning(), timers.IsConnectRetryTimerRunning())
			}
		})
	}
	for _, hold := range []time.Duration{0, 3 * time.Second} {
		f, _ := crcFSM(t, state, 3)
		timers := NewTimers()
		t.Cleanup(timers.StopAll)
		timers.SetDelayOpenTime(time.Minute)
		timers.SetHoldTime(hold)
		f.SetTimers(timers)
		if err := f.Event(EventTCPConnectionConfirmed); err != nil {
			t.Fatal(err)
		}
		if err := f.Event(EventBGPOpenWithDelayOpenTimer); err != nil {
			t.Fatal(err)
		}
		if timers.IsHoldTimerRunning() != (hold != 0) || timers.IsKeepaliveTimerRunning() != (hold != 0) {
			t.Fatalf("negotiated hold %s not respected", hold)
		}
	}
}

// TestDelayOpenTimerCancellation checks exact expiry and stale generations.
func TestDelayOpenTimerCancellation(t *testing.T) {
	clock := sim.NewFakeClock(time.Now())
	timers := NewTimers()
	timers.SetClock(clock)
	timers.SetDelayOpenTime(240 * time.Second)
	t.Cleanup(timers.StopAll)
	fired := 0
	timers.OnDelayOpenTimerExpires(func() { fired++; timers.StopDelayOpenTimer() })
	if !timers.StartDelayOpenTimer() {
		t.Fatal("timer disabled")
	}
	old := timers.delayOpenGen
	timers.StopDelayOpenTimer()
	if !timers.StartDelayOpenTimer() {
		t.Fatal("timer disabled")
	}
	timers.fireDelayOpen(old)
	if fired != 0 || !timers.IsDelayOpenTimerRunning() {
		t.Fatal("stale callback consumed live timer")
	}
	clock.Add(240*time.Second - time.Nanosecond)
	if fired != 0 {
		t.Fatal("timer fired early")
	}
	clock.Add(time.Nanosecond)
	if fired != 1 || timers.IsDelayOpenTimerRunning() {
		t.Fatal("timer did not expire exactly once")
	}
	timers.StartDelayOpenTimer()
	timers.StopAll()
	clock.Add(240 * time.Second)
	if fired != 1 {
		t.Fatal("timer survived StopAll")
	}
}
