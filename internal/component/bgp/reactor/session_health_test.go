package reactor

// VALIDATES: sessionHealth raises/clears session-stuck, session-flap, and
// eor-timeout warnings driven by its injected clock.Clock timers (stuck timer,
// EOR/GR restart timer) and by state transitions.
// PREVENTS: Regression where timer-driven warnings fail to fire, fire early,
// fire after cancellation, or fail to clear on Established/Stop/EOR. Driven by a
// sim.FakeClock so AfterFunc callbacks fire on Add() instead of via wall-clock
// sleeps, keeping the suite deterministic and race-clean.

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/sim"
)

const testStuckTimeout = 50 * time.Millisecond

// newTestClock returns a sim.FakeClock: an Add-driven clock whose AfterFunc
// callbacks fire synchronously in the caller's goroutine when fake time is
// advanced past their deadline. This lets the session-health timers fire
// deterministically and instantly, with no wall-clock sleeping.
func newTestClock() *sim.FakeClock {
	return sim.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
}

func newTestSessionHealth(addr string, clk *sim.FakeClock) *sessionHealth {
	sh := newSessionHealth(addr, clk)
	sh.stuckTimeout = testStuckTimeout
	return sh
}

func TestSessionStuckWarning(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.1", clk)

	sh.onStateChange(PeerStateStopped, PeerStateConnecting)

	// Before timeout: no warning.
	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeSessionStuck {
			t.Fatal("session-stuck warning raised before timeout")
		}
	}

	clk.Add(testStuckTimeout + 20*time.Millisecond)

	warnings = report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeSessionStuck && w.Subject == "192.0.2.1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("session-stuck warning not raised after timeout")
	}

	// Reaching Established should clear it.
	sh.onStateChange(PeerStateConnecting, PeerStateEstablished)
	warnings = report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeSessionStuck && w.Subject == "192.0.2.1" {
			t.Fatal("session-stuck warning not cleared after Established")
		}
	}

	sh.stop()
}

func TestSessionStuckClearedOnStop(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.2", clk)

	sh.onStateChange(PeerStateStopped, PeerStateActive)

	clk.Add(testStuckTimeout + 20*time.Millisecond)

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeSessionStuck && w.Subject == "192.0.2.2" {
			found = true
		}
	}
	if !found {
		t.Fatal("session-stuck warning not raised")
	}

	sh.stop()

	warnings = report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeSessionStuck && w.Subject == "192.0.2.2" {
			t.Fatal("session-stuck warning not cleared after stop")
		}
	}
}

func TestSessionFlapDetection(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.3", clk)

	for range flapThreshold {
		clk.Add(10 * time.Second)
		sh.onStateChange(PeerStateEstablished, PeerStateActive)
		sh.onStateChange(PeerStateActive, PeerStateEstablished)
	}

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeSessionFlap && w.Subject == "192.0.2.3" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("session-flap warning not raised after rapid transitions")
	}

	sh.stop()

	warnings = report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeSessionFlap && w.Subject == "192.0.2.3" {
			t.Fatal("session-flap warning not cleared after stop")
		}
	}
}

func TestEORTimeoutWarning(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.10", clk)

	// Start EOR timer: 1 second timeout, 2 families expected.
	sh.startEORTimer(1, 2)

	// Before timeout: no warning.
	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout {
			t.Fatal("eor-timeout warning raised before timeout")
		}
	}

	clk.Add(1200 * time.Millisecond)

	warnings = report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout && w.Subject == "192.0.2.10" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("eor-timeout warning not raised after timeout")
	}

	sh.stop()
}

func TestEORTimeoutClearedOnAllFamilies(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.11", clk)

	// 2 families expected, 1 second timeout.
	sh.startEORTimer(1, 2)

	clk.Add(1200 * time.Millisecond)

	// Verify warning is raised.
	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout && w.Subject == "192.0.2.11" {
			found = true
		}
	}
	if !found {
		t.Fatal("eor-timeout warning not raised")
	}

	// First EOR: one family still pending, warning stays.
	sh.onEORReceived()
	warnings = report.Warnings()
	stillActive := false
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout && w.Subject == "192.0.2.11" {
			stillActive = true
		}
	}
	if !stillActive {
		t.Fatal("eor-timeout warning cleared after only 1 of 2 family EORs")
	}

	// Second EOR: all families done, warning clears.
	sh.onEORReceived()
	warnings = report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout && w.Subject == "192.0.2.11" {
			t.Fatal("eor-timeout warning not cleared after all family EORs received")
		}
	}

	sh.stop()
}

func TestEORTimeoutCancelledBeforeFiring(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.12", clk)

	// 1 family, 2 second timeout.
	sh.startEORTimer(2, 1)

	// EOR received before timeout.
	sh.onEORReceived()

	clk.Add(2200 * time.Millisecond)

	// Warning should never have fired.
	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout && w.Subject == "192.0.2.12" {
			t.Fatal("eor-timeout warning raised despite EOR received before timeout")
		}
	}

	sh.stop()
}

func TestEORTimeoutZeroRestartTime(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.13", clk)

	// Zero restart-time means GR timer is disabled.
	sh.startEORTimer(0, 2)

	clk.Add(100 * time.Millisecond)

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeEORTimeout {
			t.Fatal("eor-timeout should not fire with restart-time=0")
		}
	}

	sh.stop()
}

// TestSessionHealthFlapLifetime verifies the lifetime flap counter increments
// on every Established->non-Established transition.
func TestSessionHealthFlapLifetime(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.20", clk)

	if sh.FlapCount() != 0 {
		t.Fatal("expected zero flap count initially")
	}

	sh.onStateChange(PeerStateEstablished, PeerStateActive)
	if sh.FlapCount() != 1 {
		t.Fatalf("expected 1 flap, got %d", sh.FlapCount())
	}

	clk.Add(10 * time.Minute)
	sh.onStateChange(PeerStateActive, PeerStateEstablished)
	sh.onStateChange(PeerStateEstablished, PeerStateConnecting)
	if sh.FlapCount() != 2 {
		t.Fatalf("expected 2 flaps, got %d", sh.FlapCount())
	}

	sh.stop()
}

func TestSessionFlapNotTriggeredWithSlowTransitions(t *testing.T) {
	report.ResetForTest()

	clk := newTestClock()
	sh := newTestSessionHealth("192.0.2.4", clk)

	for range flapThreshold + 1 {
		clk.Add(flapWindow + time.Minute)
		sh.onStateChange(PeerStateEstablished, PeerStateActive)
		sh.onStateChange(PeerStateActive, PeerStateEstablished)
	}

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeSessionFlap && w.Subject == "192.0.2.4" {
			t.Fatal("session-flap warning raised for slow transitions")
		}
	}

	sh.stop()
}
