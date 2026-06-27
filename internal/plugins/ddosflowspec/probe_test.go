package ddosflowspec

import (
	"testing"
)

func TestHoldDownBlocksEarlyProbe(t *testing.T) {
	// VALIDATES: AC-3 -- no probe within hold-down
	p := newProbe(300, 60, 10, 1000000, 3600)
	p.Start()
	for range 100 {
		action := p.Tick(500000)
		if action == probeActionWithdraw || action == probeActionProbe {
			t.Fatalf("probe/withdraw during hold-down at tick %d", p.elapsed)
		}
	}
}

func TestLeakProbeSaturatedReTightens(t *testing.T) {
	// VALIDATES: AC-4 -- saturated probe re-tightens + extends hold-down
	p := newProbe(3, 1, 3, 1000000, 3600)
	p.Start()

	// Advance past hold-down (4 ticks: elapsed 1,2,3 stay; 4 transitions to waiting)
	// then 1 more tick enters probing (sinceLastProbe reaches probeInterval)
	for range 5 {
		p.Tick(0)
	}
	if p.state != probeStateProbing {
		t.Fatalf("expected probing state, got %v", p.state)
	}

	action := p.Tick(2000000) // saturates probe-rate
	if action != probeActionReTighten {
		t.Errorf("expected re-tighten on saturated probe, got %v", action)
	}
}

func TestLeakProbeClearWithdraws(t *testing.T) {
	// VALIDATES: AC-5 -- sub-probe-rate after hold-down triggers withdraw
	p := newProbe(3, 1, 3, 1000000, 3600)
	p.Start()

	// Advance past hold-down + enter probing
	for range 5 {
		p.Tick(0)
	}
	if p.state != probeStateProbing {
		t.Fatalf("expected probing state, got %v", p.state)
	}

	// Feed sub-probe-rate for probe-window ticks
	var lastAction probeAction
	for range 3 {
		lastAction = p.Tick(100)
	}
	if lastAction != probeActionWithdraw {
		t.Errorf("expected withdraw after sub-probe-rate window, got %v", lastAction)
	}
}

func TestProbeBackoffExtendsHoldDown(t *testing.T) {
	p := newProbe(3, 1, 3, 1000000, 3600)
	p.Start()

	initialHoldDown := p.currentHoldDown

	// Advance past hold-down + enter probing
	for range 5 {
		p.Tick(0)
	}

	// Saturate to trigger backoff
	p.Tick(2000000)

	if p.currentHoldDown <= initialHoldDown {
		t.Errorf("hold-down should have increased: got %d, was %d", p.currentHoldDown, initialHoldDown)
	}
}

func TestProbeNotStarted(t *testing.T) {
	p := newProbe(5, 10, 3, 1000000, 3600)
	action := p.Tick(500000)
	if action != probeActionNone {
		t.Errorf("tick before Start should be none, got %v", action)
	}
}

func TestProbeCustomRate(t *testing.T) {
	p := newProbe(2, 1, 3, 500000, 1800)
	p.Start()
	// hold-down=2: ticks 1,2 stay; tick 3 transitions to waiting; tick 4 enters probing
	for range 4 {
		p.Tick(0)
	}
	if p.state != probeStateProbing {
		t.Fatalf("expected probing, got %v", p.state)
	}
	action := p.Tick(600000) // saturates 500000
	if action != probeActionReTighten {
		t.Errorf("expected re-tighten at custom rate, got %v", action)
	}
}
