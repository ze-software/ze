// Design: docs/guide/config-editor.md -- model timer lifetime.

package cli

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// modelTickMsg binds an expiry to the shared lifetime that scheduled it.
// Validation generations alone can repeat when an editor is restarted.
type modelTickMsg struct {
	owner *dispatchQueue
	msg   tea.Msg
}

// TimerMsg declares timer work separately from commands that change state.
// Model.Update starts it in Bubble Tea; a headless driver MUST call Wait once
// and apply its result through Update. The model owner MUST close the model.
type TimerMsg struct {
	owner *dispatchQueue
	delay time.Duration
	msg   tea.Msg
}

// IsValidation reports whether this timer belongs to validation debounce.
func (t TimerMsg) IsValidation() bool {
	_, ok := t.msg.(validationTickMsg)
	return ok
}

// IsConfirmation reports a countdown tick or a session's daemon-window poll.
func (t TimerMsg) IsConfirmation() bool {
	switch t.msg.(type) {
	case confirmCountdownMsg, draftPollMsg:
		return true
	default:
		return false
	}
}

// Wait waits for expiry or model cancellation and stops the timer before it
// returns. A driver MUST deliver a non-nil result to the originating model.
func (t TimerMsg) Wait() tea.Msg {
	if t.owner.ctx.Err() != nil {
		return nil
	}
	timer := t.owner.clock.NewTimer(t.delay)
	defer timer.Stop()
	select {
	case <-t.owner.ctx.Done():
		return nil
	case <-timer.C():
		if t.owner.ctx.Err() != nil {
			return nil
		}
		return modelTickMsg{owner: t.owner, msg: t.msg}
	}
}

// tick declares a session-owned timer. The model owner MUST call Close when
// the session ends; the driver MUST execute TimerMsg.Wait exactly once.
func (m Model) tick(delay time.Duration, message tea.Msg) tea.Cmd {
	owner := m.dispatch
	return func() tea.Msg {
		if owner.ctx.Err() != nil {
			return nil
		}
		return TimerMsg{owner: owner, delay: delay, msg: message}
	}
}
