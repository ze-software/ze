// Design: plan/spec-healthcheck-0-umbrella.md -- FSM states and transitions
package healthcheck

// State represents the healthcheck FSM state.
type State int

const (
	StateInit     State = iota // Initial state at startup
	StateRising                // Consecutive successes accumulating
	StateUp                    // Service healthy
	StateFalling               // Consecutive failures accumulating
	StateDown                  // Service unhealthy
	StateDisabled              // Admin disabled
	StateExit                  // Shutdown requested
	StateEnd                   // Single-check complete (interval=0)
)

// fsm implements the 8-state healthcheck finite state machine.
// All transitions go through trigger() which applies rise/fall shortcuts.
type fsm struct {
	state State
	count uint32 // consecutive check count in current intermediate state
	rise  uint32
	fall  uint32
}

func newFSM(rise, fall uint32) *fsm {
	return &fsm{
		state: StateInit,
		rise:  rise,
		fall:  fall,
	}
}

// step processes one check result and transitions the FSM.
func (f *fsm) step(success bool) {
	switch f.state {
	case StateInit:
		if success {
			f.count = 1
			f.state = f.trigger(StateRising)
		} else {
			f.count = 1
			f.state = f.trigger(StateFalling)
		}
	case StateRising:
		if success {
			f.count++
			if f.count >= f.rise {
				f.state = StateUp
			}
		} else {
			f.count = 1
			f.state = f.trigger(StateFalling)
		}
	case StateFalling:
		if !success {
			f.count++
			if f.count >= f.fall {
				f.state = StateDown
			}
		} else {
			f.count = 1
			f.state = f.trigger(StateRising)
		}
	case StateUp:
		if !success {
			f.count = 1
			f.state = f.trigger(StateFalling)
		}
	case StateDown:
		if success {
			f.count = 1
			f.state = f.trigger(StateRising)
		}
	case StateDisabled, StateExit, StateEnd:
		// No transitions from these states via check results.
	}
}

// trigger applies the rise/fall shortcut: if the threshold is <= 1,
// skip the intermediate state and go directly to UP or DOWN.
func (f *fsm) trigger(target State) State {
	if target == StateRising && f.rise <= 1 {
		return StateUp
	}
	if target == StateFalling && f.fall <= 1 {
		return StateDown
	}
	return target
}
