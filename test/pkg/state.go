// Package functional provides the test infrastructure for ZeBGP functional tests.
//
// This package is modeled after ExaBGP's qa/bin/functional test runner,
// providing state machine-based test lifecycle management, concurrent
// execution, timing tracking, and rich output.
package functional

// State represents the lifecycle state of a test.
// States progress: None -> Starting -> Running -> Success/Fail/Timeout.
type State int

// Test lifecycle states.
const (
	StateNone     State = iota // Test not yet started
	StateStarting              // Test server starting
	StateRunning               // Test actively running
	StateSuccess               // Test completed successfully
	StateFail                  // Test failed
	StateTimeout               // Test timed out
	StateSkip                  // Test skipped (not selected)
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// String returns the human-readable name of the state.
func (s State) String() string {
	switch s {
	case StateNone:
		return "none"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateSuccess:
		return "success"
	case StateFail:
		return "fail"
	case StateTimeout:
		return "timeout"
	case StateSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if the state is a terminal state
// (success, fail, timeout, or skip).
func (s State) IsTerminal() bool {
	switch s { //nolint:exhaustive // Only checking terminal states
	case StateSuccess, StateFail, StateTimeout, StateSkip:
		return true
	default:
		return false
	}
}

// IsActive returns true if the state is an active (non-terminal) state.
// Active tests are those that need processing: none, starting, running.
func (s State) IsActive() bool {
	return !s.IsTerminal()
}

// Color returns the ANSI color code for the state.
func (s State) Color() string {
	switch s {
	case StateNone:
		return colorGray
	case StateStarting:
		return colorGray
	case StateRunning:
		return colorCyan
	case StateSuccess:
		return colorGreen
	case StateFail:
		return colorRed
	case StateTimeout:
		return colorYellow
	case StateSkip:
		return colorBlue
	default:
		return colorReset
	}
}

// ColoredSymbol returns a colored symbol representing the state.
func (s State) ColoredSymbol() string {
	switch s { //nolint:exhaustive // Non-terminal states use default
	case StateSuccess:
		return colorGreen + "✓" + colorReset
	case StateFail:
		return colorRed + "✗" + colorReset
	case StateTimeout:
		return colorYellow + "⏱" + colorReset
	case StateSkip:
		return colorBlue + "⊘" + colorReset
	default:
		return colorGray + "·" + colorReset
	}
}
