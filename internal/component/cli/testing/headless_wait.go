// Design: docs/architecture/testing/ci-format.md -- authored editor waits.

package testing

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
)

const waitTimerDrain = "drain"

func (w WaitAction) parameters() (string, time.Duration, error) {
	if len(w.Values) != 1 {
		return "", 0, errors.New("wait requires exactly one action")
	}
	for kind, value := range w.Values {
		switch {
		case kind == "validation" && value == "":
			return kind, 0, nil
		case kind == etTimer && value == "expire":
			return kind, 0, nil
		case kind == etTimer && value == waitTimerDrain:
			return kind, 0, nil
		case kind == "ms":
			ms, err := strconv.ParseInt(value, 10, 64)
			if err != nil || ms < 0 || ms > math.MaxInt64/int64(time.Millisecond) {
				return "", 0, fmt.Errorf("wait=ms requires nonnegative integer milliseconds within the duration limit, got %q", value)
			}
			return kind, time.Duration(ms) * time.Millisecond, nil
		default:
			return "", 0, fmt.Errorf("invalid wait action %s:%s: use ms:<N>, validation, timer:drain or timer:expire", kind, value)
		}
	}
	return "", 0, errors.New("wait requires an action")
}

// wait executes an authored wait with real time, applying timer results and
// their complete state-command chains on this goroutine.
func (hm *headlessModel) wait(action WaitAction) error {
	kind, duration, err := action.parameters()
	if err != nil {
		return err
	}
	select {
	case <-hm.stopped:
		return errors.New("editor wait canceled")
	case <-hm.canceled:
		return errors.New("editor wait canceled")
	default:
	}
	switch kind {
	case "validation":
		return hm.waitValidation()
	case "ms":
		deadline := time.NewTimer(duration)
		defer deadline.Stop()
		for {
			elapsed, err := hm.waitEvent(deadline.C)
			if err != nil || elapsed {
				return err
			}
		}
	case etTimer:
		if action.Values[etTimer] == waitTimerDrain {
			return hm.waitPendingTimers()
		}
		if !hm.ConfirmTimerActive() {
			return errors.New("timer expiry wait requires an active confirmation timer at entry")
		}
		for hm.ConfirmTimerActive() {
			pending := false
			for _, tick := range hm.pending {
				pending = pending || tick.request.IsConfirmation()
			}
			if !pending {
				return errors.New("active confirmation timer has no scheduled countdown or window poll")
			}
			if _, err := hm.waitEvent(nil); err != nil {
				return err
			}
			if err := hm.model.Error(); err != nil {
				return fmt.Errorf("confirmation expiry failed: %w", err)
			}
		}
		if hm.StatusMessage() != contract.CommitTimedOut {
			return errors.New("confirmation timer closed without expiry")
		}
		return nil
	}
	return errors.New("unrecognized editor wait")
}

// waitPendingTimers applies each timer pending at entry exactly once. A timer
// rearmed by a delivery belongs to the next wait, not this finite barrier.
func (hm *headlessModel) waitPendingTimers() error {
	if len(hm.pending) == 0 {
		return errors.New("timer drain wait requires a pending timer at entry")
	}
	entry := hm.pending
	hm.pending = nil
	for i, tick := range entry {
		select {
		case msg := <-tick.result:
			if msg == nil {
				hm.pending = append(hm.pending, entry[i+1:]...)
				return errors.New("editor timer canceled during drain")
			}
			hm.processMsg(msg)
		case <-hm.stopped:
			hm.pending = append(hm.pending, entry[i:]...)
			return errors.New("editor timer drain canceled")
		case <-hm.canceled:
			hm.pending = append(hm.pending, entry[i:]...)
			return errors.New("editor timer drain canceled")
		}
	}
	select {
	case <-hm.stopped:
		return errors.New("editor timer drain canceled")
	case <-hm.canceled:
		return errors.New("editor timer drain canceled")
	default:
		return nil
	}
}

// waitEvent waits on the finite set of declared timers, a duration deadline
// and shutdown. It removes exactly the selected timer before applying its
// message, since that result may schedule another timer.
func (hm *headlessModel) waitEvent(deadline <-chan time.Time) (bool, error) {
	for {
		select {
		case <-deadline:
			return true, nil
		case <-hm.stopped:
			return false, errors.New("editor wait canceled")
		case <-hm.canceled:
			return false, errors.New("editor wait canceled")
		default:
		}
		for i, tick := range hm.pending {
			select {
			case msg := <-tick.result:
				hm.pending = append(hm.pending[:i], hm.pending[i+1:]...)
				if msg == nil {
					return false, errors.New("editor timer canceled during wait")
				}
				hm.processMsg(msg)
				return false, nil
			default:
			}
		}
		select {
		case <-deadline:
			return true, nil
		case <-hm.stopped:
			return false, errors.New("editor wait canceled")
		case <-hm.canceled:
			return false, errors.New("editor wait canceled")
		case <-hm.ready:
		}
	}
}
