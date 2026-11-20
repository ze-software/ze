// Design: docs/architecture/traffic/traffic-analysis-layers.md -- the shared confirm-and-clear state machine

package stats

// HysteresisState is where a Hysteresis machine stands: between a watched
// condition first holding and its confirmation, and between it first failing
// and its clearing.
type HysteresisState int

const (
	// HysteresisUnspecified is the zero value: a machine NewHysteresis did not
	// build. No transition leads to it.
	HysteresisUnspecified HysteresisState = iota
	// HysteresisIdle: the condition does not hold, or it was cleared.
	HysteresisIdle
	// HysteresisConfirming: the condition holds, for fewer consecutive
	// evaluations than the confirm count.
	HysteresisConfirming
	// HysteresisActive: the condition was confirmed and holds.
	HysteresisActive
	// HysteresisClearing: the condition was confirmed and has failed for fewer
	// consecutive evaluations than the clear count.
	HysteresisClearing
)

// String names the state for a human, in a log line or a test message. Never
// compare with it.
func (s HysteresisState) String() string {
	switch s {
	case HysteresisUnspecified:
		return "unspecified"
	case HysteresisIdle:
		return "idle"
	case HysteresisConfirming:
		return "confirming"
	case HysteresisActive:
		return "active"
	case HysteresisClearing:
		return "clearing"
	default:
		panic("BUG: invalid hysteresis state")
	}
}

// Hysteresis is the confirm-and-clear state machine every detector shares, so
// that one noisy evaluation neither raises an alarm nor drops one. The watched
// condition must hold for confirmAfter consecutive evaluations to be confirmed,
// and fail for clearAfter consecutive evaluations to be cleared. One failing
// evaluation returns a confirming machine to idle and restarts its count; one
// holding evaluation returns a clearing machine to active and drops its count.
// The detector owns the condition and its threshold: the machine counts only
// the answers it is fed.
//
// Tick runs OnConfirmed and OnCleared synchronously, on the caller's goroutine
// and under whatever lock the caller holds.
//
// Not safe for concurrent use; callers serialize access with their own lock.
type Hysteresis struct {
	confirmAfter int // consecutive holding evaluations that confirm
	clearAfter   int // consecutive failing evaluations that clear

	state      HysteresisState
	confirmRun int // consecutive holding evaluations while idle or confirming
	clearRun   int // consecutive failing evaluations while clearing

	// OnConfirmed, when set, runs on each transition to active from idle or
	// confirming. A clearing machine that returns to active does not run it.
	OnConfirmed func()
	// OnCleared, when set, runs on each transition from clearing to idle.
	OnCleared func()
}

// NewHysteresis returns an idle machine. confirmAfter is the count of
// consecutive evaluations the condition must hold to be confirmed: 0 and 1 both
// confirm on the first one, and a negative count reads as 0. clearAfter is the
// count of consecutive evaluations it must fail to be cleared, and a count
// below 1 reads as 1: the evaluation that leaves active is the first failing
// one.
func NewHysteresis(confirmAfter, clearAfter int) *Hysteresis {
	return &Hysteresis{
		confirmAfter: max(confirmAfter, 0),
		clearAfter:   max(clearAfter, 1),
		state:        HysteresisIdle,
	}
}

// State returns where the machine stands after the last Tick.
func (h *Hysteresis) State() HysteresisState {
	return h.state
}

// Tick feeds one evaluation. holds is the detector's answer at this
// evaluation: true when the watched condition holds (a rate above its
// threshold, a level above its share).
func (h *Hysteresis) Tick(holds bool) {
	switch h.state {
	case HysteresisUnspecified:
		panic("BUG: Hysteresis ticked without NewHysteresis")

	case HysteresisIdle:
		if holds {
			h.confirmRun = 1
			if h.confirmRun >= h.confirmAfter {
				h.confirm()
			} else {
				h.state = HysteresisConfirming
			}
		}

	case HysteresisConfirming:
		if holds {
			h.confirmRun++
			if h.confirmRun >= h.confirmAfter {
				h.confirm()
			}
		} else {
			h.state = HysteresisIdle
			h.confirmRun = 0
		}

	case HysteresisActive:
		if !holds {
			h.clearRun = 1
			h.state = HysteresisClearing
		}

	case HysteresisClearing:
		if holds {
			h.clearRun = 0
			h.state = HysteresisActive
		} else {
			h.clearRun++
			if h.clearRun >= h.clearAfter {
				h.state = HysteresisIdle
				h.clearRun = 0
				if h.OnCleared != nil {
					h.OnCleared()
				}
			}
		}

	default:
		panic("BUG: invalid hysteresis state")
	}
}

// confirm moves the machine to active and runs OnConfirmed.
func (h *Hysteresis) confirm() {
	h.state = HysteresisActive
	h.confirmRun = 0
	if h.OnConfirmed != nil {
		h.OnConfirmed()
	}
}

// Reset returns the machine to idle with both counts dropped, running neither
// callback.
func (h *Hysteresis) Reset() {
	h.state = HysteresisIdle
	h.confirmRun = 0
	h.clearRun = 0
}
