// Design: docs/architecture/behavior/fsm.md — final timer retire after a session ends.
package fsm

import (
	"testing"
	"time"
)

// TestTimersRetireSealsEveryArm checks that a retired timer set arms nothing.
// Method: arm every timer, Retire, then call every arm path again and assert no
// timer runs. The bool-answering starts must answer "hold" (true), so a dead
// session never advances on a timer that was not armed.
func TestTimersRetireSealsEveryArm(t *testing.T) {
	timers := NewTimers()
	timers.SetHoldTime(90 * time.Second)
	timers.SetDelayOpenTime(5 * time.Second)
	timers.SetBfdHoldDown(5 * time.Second)

	timers.StartHoldTimer()
	timers.StartKeepaliveTimer()
	timers.StartConnectRetryTimer()
	timers.StartBfdHoldTimer()
	if !timers.StartBfdHoldDownTimer() {
		t.Fatal("BFD hold-down did not arm before Retire")
	}
	if !timers.StartDelayOpenTimer() {
		t.Fatal("DelayOpen did not arm before Retire")
	}

	timers.Retire()

	timers.StartHoldTimer()
	timers.ResetHoldTimer()
	timers.StartKeepaliveTimer()
	timers.StartConnectRetryTimer()
	timers.StartBfdHoldTimer()
	if !timers.StartBfdHoldDownTimerFor(time.Second) {
		t.Error("retired StartBfdHoldDownTimerFor answered false: a dead session would advance")
	}
	if !timers.StartDelayOpenTimer() {
		t.Error("retired StartDelayOpenTimer answered false: a dead session would send OPEN")
	}

	running := map[string]bool{
		"hold":          timers.IsHoldTimerRunning(),
		"keepalive":     timers.IsKeepaliveTimerRunning(),
		"connect-retry": timers.IsConnectRetryTimerRunning(),
		"bfd-hold":      timers.IsBfdHoldTimerRunning(),
		"bfd-hold-down": timers.IsBfdHoldDownTimerRunning(),
		"delay-open":    timers.IsDelayOpenTimerRunning(),
	}
	for name, armed := range running {
		if armed {
			t.Errorf("%s timer armed after Retire", name)
		}
	}
}
