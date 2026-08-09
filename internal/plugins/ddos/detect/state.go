// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- trigger/clear state machine

package detect

type detectorState int

const (
	stateIdle       detectorState = iota
	stateConfirming               // rate above threshold, waiting for confirm-duration
	stateActive                   // attack confirmed, emitting events
	stateClearing                 // rate below threshold, waiting for clear-consecutive-checks
)

func (s detectorState) String() string {
	switch s {
	case stateIdle:
		return "idle"
	case stateConfirming:
		return "confirming"
	case stateActive:
		return "active"
	case stateClearing:
		return "clearing"
	default:
		return "unknown"
	}
}

type stateMachine struct {
	confirmDuration int
	clearChecks     int

	state        detectorState
	confirmCount int
	clearCount   int

	OnDetected func()
	OnCleared  func()
}

func newStateMachine(confirmDuration, clearChecks int) *stateMachine {
	return &stateMachine{
		confirmDuration: max(confirmDuration, 0),
		clearChecks:     max(clearChecks, 1),
		state:           stateIdle,
	}
}

func (sm *stateMachine) State() detectorState {
	return sm.state
}

func (sm *stateMachine) Tick(aboveThreshold bool) {
	switch sm.state {
	case stateIdle:
		if aboveThreshold {
			sm.confirmCount = 1
			if sm.confirmCount >= sm.confirmDuration {
				sm.activate()
			} else {
				sm.state = stateConfirming
			}
		}

	case stateConfirming:
		if aboveThreshold {
			sm.confirmCount++
			if sm.confirmCount >= sm.confirmDuration {
				sm.activate()
			}
		} else {
			sm.state = stateIdle
			sm.confirmCount = 0
		}

	case stateActive:
		if !aboveThreshold {
			sm.clearCount = 1
			sm.state = stateClearing
		}

	case stateClearing:
		if aboveThreshold {
			sm.clearCount = 0
			sm.state = stateActive
		} else {
			sm.clearCount++
			if sm.clearCount >= sm.clearChecks {
				sm.state = stateIdle
				sm.clearCount = 0
				if sm.OnCleared != nil {
					sm.OnCleared()
				}
			}
		}
	}
}

func (sm *stateMachine) activate() {
	sm.state = stateActive
	sm.confirmCount = 0
	if sm.OnDetected != nil {
		sm.OnDetected()
	}
}

func (sm *stateMachine) Reset() {
	sm.state = stateIdle
	sm.confirmCount = 0
	sm.clearCount = 0
}
