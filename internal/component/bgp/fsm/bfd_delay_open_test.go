// Design: docs/architecture/behavior/fsm.md — strict-mode DelayOpen substates.
package fsm

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/sim"
)

func strictDelayOpenFSM(t *testing.T, state State, hold time.Duration) (*FSM, *Timers) {
	t.Helper()
	f, _ := crcFSM(t, state, 3)
	timers := NewTimers()
	timers.SetClock(sim.NewFakeClock(time.Now()))
	timers.SetDelayOpenTime(time.Minute)
	timers.SetHoldTime(hold)
	f.SetTimers(timers)
	f.SetBFDStrict(true)
	t.Cleanup(timers.StopAll)
	if err := f.Event(EventTCPConnectionConfirmed); err != nil {
		t.Fatal(err)
	}
	if err := f.EnterDelayOpenBfdUpPending(); err != nil {
		t.Fatal(err)
	}
	return f, timers
}

// TestDraftBFDStrictDelayOpenSubStates covers both Event 20 parents, both
// negotiated timer branches and all three release events without RFC tags:
// the draft's Section 8 clauses contain no requirement keywords.
func TestDraftBFDStrictDelayOpenSubStates(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive} {
		for _, hold := range []time.Duration{0, 3 * time.Second} {
			for _, event := range []Event{EventBfdUp, EventBfdAdminDown, EventBfdDisabled} {
				t.Run(fmt.Sprintf("%s/%s/%s", state, hold, event), func(t *testing.T) {
					f, timers := strictDelayOpenFSM(t, state, hold)
					want := SubStateConnectDelayOpenBfdUpPending
					if state == StateActive {
						want = SubStateActiveDelayOpenBfdUpPending
					}
					if f.State() != state || f.BfdSubState() != want {
						t.Fatalf("entry = %s/%s", f.State(), f.BfdSubState())
					}
					if timers.IsDelayOpenTimerRunning() || timers.IsConnectRetryTimerRunning() || timers.IsKeepaliveTimerRunning() || timers.IsHoldTimerRunning() != (hold != 0) || timers.IsBfdHoldTimerRunning() != (hold == 0) {
						t.Fatal("strict Event 20 did not select exactly the negotiated wait bound")
					}
					if err := f.Event(EventBfdDown); err != nil || f.BfdSubState() != want {
						t.Fatalf("BfdDown changed pending state: %v", err)
					}
					if err := f.Event(event); err != nil {
						t.Fatal(err)
					}
					if f.State() != StateOpenConfirm || f.BfdSubState() != SubStateNone || timers.IsBfdHoldTimerRunning() || timers.IsKeepaliveTimerRunning() != (hold != 0) || timers.IsHoldTimerRunning() != (hold != 0) {
						t.Fatal("release did not clear pending and select negotiated timers")
					}
				})
			}
		}
	}
}

func TestDraftBFDStrictDelayOpenFailures(t *testing.T) {
	for _, state := range []State{StateConnect, StateActive} {
		for _, event := range []Event{EventBGPOpenWithDelayOpenTimer, EventBGPOpen, EventHoldTimerExpires, EventBfdHoldTimerExpires, EventBfdStrictConfigChanged, EventManualStop} {
			t.Run(state.String()+"/"+event.String(), func(t *testing.T) {
				f, timers := strictDelayOpenFSM(t, state, 0)
				err := f.Event(event)
				if err != nil && !errors.Is(err, ErrFSMError) {
					t.Fatal(err)
				}
				want := uint32(4)
				if event == EventBfdStrictConfigChanged || event == EventManualStop {
					want = 0
				}
				if f.State() != StateIdle || f.BfdSubState() != SubStateNone || f.crc.Load() != want || timers.IsBfdHoldTimerRunning() || timers.IsDelayOpenTimerRunning() || timers.IsConnectRetryTimerRunning() {
					t.Fatalf("failure left %s/%s counter=%d", f.State(), f.BfdSubState(), f.crc.Load())
				}
			})
		}
	}
}

func TestEnterDelayOpenBfdUpPendingRefusesInvalidEntry(t *testing.T) {
	for _, state := range []State{StateIdle, StateConnect, StateActive, StateOpenSent, StateOpenConfirm, StateEstablished} {
		f, _ := crcFSM(t, state, 3)
		timers := NewTimers()
		t.Cleanup(timers.StopAll)
		f.SetTimers(timers)
		f.SetBFDStrict(true)
		if err := f.EnterDelayOpenBfdUpPending(); !errors.Is(err, ErrFSMError) || f.State() != state || f.BfdSubState() != SubStateNone {
			t.Fatalf("accepted entry without running DelayOpen in %s: %v", state, err)
		}
		if state != StateConnect && state != StateActive {
			timers.SetDelayOpenTime(time.Minute)
			timers.StartDelayOpenTimer()
			if err := f.EnterDelayOpenBfdUpPending(); !errors.Is(err, ErrFSMError) ||
				f.State() != state || f.BfdSubState() != SubStateNone || !timers.IsDelayOpenTimerRunning() {
				t.Fatalf("wrong-parent refusal consumed the delay in %s: %v", state, err)
			}
		}
	}
	for _, state := range []State{StateConnect, StateActive} {
		f, _ := strictDelayOpenFSM(t, state, 0)
		if err := f.EnterDelayOpenBfdUpPending(); !errors.Is(err, ErrFSMError) {
			t.Fatalf("accepted duplicate strict Event 20 in %s", state)
		}
		if err := f.EnterBfdUpPending(); !errors.Is(err, ErrFSMError) {
			t.Fatalf("OpenSent-only entry accepted %s", state)
		}
		plain, _ := crcFSM(t, state, 3)
		timers := NewTimers()
		t.Cleanup(timers.StopAll)
		timers.SetDelayOpenTime(time.Minute)
		plain.SetTimers(timers)
		if err := plain.Event(EventTCPConnectionConfirmed); err != nil {
			t.Fatal(err)
		}
		if err := plain.EnterDelayOpenBfdUpPending(); !errors.Is(err, ErrFSMError) ||
			plain.BfdSubState() != SubStateNone || !timers.IsDelayOpenTimerRunning() {
			t.Fatalf("non-strict refusal consumed the delay in %s: %v", state, err)
		}
	}
}
