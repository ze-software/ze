package functional

import "testing"

// TestStateValues verifies State enum values are sequential from 0.
//
// VALIDATES: State constants have expected integer values.
// PREVENTS: Breaking changes when State values are reordered.
func TestStateValues(t *testing.T) {
	tests := []struct {
		state State
		want  int
	}{
		{StateNone, 0},
		{StateStarting, 1},
		{StateRunning, 2},
		{StateSuccess, 3},
		{StateFail, 4},
		{StateTimeout, 5},
		{StateSkip, 6},
	}

	for _, tt := range tests {
		if int(tt.state) != tt.want {
			t.Errorf("State %s = %d, want %d", tt.state, tt.state, tt.want)
		}
	}
}

// TestStateString verifies State String() method returns human-readable names.
//
// VALIDATES: State values can be printed for debugging.
// PREVENTS: Missing String() implementations causing numeric output.
func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateNone, "none"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateSuccess, "success"},
		{StateFail, "fail"},
		{StateTimeout, "timeout"},
		{StateSkip, "skip"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestStateIsTerminal verifies IsTerminal() correctly identifies terminal states.
//
// VALIDATES: Terminal states (success, fail, timeout, skip) are identified.
// PREVENTS: Event loop continuing for completed tests.
func TestStateIsTerminal(t *testing.T) {
	tests := []struct {
		state    State
		terminal bool
	}{
		{StateNone, false},
		{StateStarting, false},
		{StateRunning, false},
		{StateSuccess, true},
		{StateFail, true},
		{StateTimeout, true},
		{StateSkip, true},
	}

	for _, tt := range tests {
		if got := tt.state.IsTerminal(); got != tt.terminal {
			t.Errorf("State(%s).IsTerminal() = %v, want %v", tt.state, got, tt.terminal)
		}
	}
}

// TestStateIsActive verifies IsActive() correctly identifies active states.
//
// VALIDATES: Active states (none, starting, running) are identified.
// PREVENTS: Tests being skipped when they should run.
func TestStateIsActive(t *testing.T) {
	tests := []struct {
		state  State
		active bool
	}{
		{StateNone, true},
		{StateStarting, true},
		{StateRunning, true},
		{StateSuccess, false},
		{StateFail, false},
		{StateTimeout, false},
		{StateSkip, false},
	}

	for _, tt := range tests {
		if got := tt.state.IsActive(); got != tt.active {
			t.Errorf("State(%s).IsActive() = %v, want %v", tt.state, got, tt.active)
		}
	}
}

// TestStateColor verifies Color() returns appropriate ANSI codes.
//
// VALIDATES: Each state has a color for terminal display.
// PREVENTS: Colorless or incorrect terminal output.
func TestStateColor(t *testing.T) {
	// Just verify colors are non-empty for each state
	for s := StateNone; s <= StateSkip; s++ {
		if got := s.Color(); got == "" {
			t.Errorf("State(%s).Color() = empty, want ANSI code", s)
		}
	}
}
