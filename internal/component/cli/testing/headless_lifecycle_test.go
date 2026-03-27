package testing

import (
	"testing"
)

// TestHeadlessRestartFuncPropagation verifies that restartFunc set on
// a HeadlessModel survives the TypeText + PressEnter chain.
// VALIDATES: .et lifecycle infrastructure -- restartFunc through headless model.
// PREVENTS: restartFunc nil when model processes "restart" command.
func TestHeadlessRestartFuncPropagation(t *testing.T) {
	hm := NewHeadlessCommandModel()

	// Set restartFunc via the Model() pointer (same as runner line 225-227).
	called := false
	m := hm.Model()
	m.SetShutdownFunc(func() {})
	m.SetRestartFunc(func() { called = true })

	// Type "restart" and press Enter (same as .et input=type:text=restart + input=enter).
	hm.TypeText("restart")
	hm.PressEnter()

	// The status message reveals whether restartFunc was nil.
	// "restart not available" = nil, confirmation prompt = set.
	status := hm.StatusMessage()
	t.Logf("status: %q, called: %v", status, called)

	if status == "restart not available (not connected to daemon)" {
		t.Error("restartFunc was nil when model processed 'restart' -- lost in headless Update chain")
	}
}

// TestHeadlessRestartFuncViaRunTestCase runs an actual .et test case
// through the runner to reproduce the exact failure path.
func TestHeadlessRestartFuncViaRunTestCase(t *testing.T) {
	tc := &TestCase{
		Options: []Option{
			{Type: "mode", Values: map[string]string{"value": "command"}},
			{Type: "lifecycle", Values: map[string]string{"mode": "wired"}},
			{Type: "timeout", Values: map[string]string{"value": "5s"}},
		},
		Steps: []Step{
			{Type: StepInput, InputIndex: 0},
			{Type: StepInput, InputIndex: 1},
			{Type: StepExpect, ExpectIndex: 0},
		},
		Inputs: []InputAction{
			{Action: "type", Values: map[string]string{"text": "restart"}},
			{Action: "key", Values: map[string]string{"name": "enter"}},
		},
		Expects: []Expectation{
			{Type: "status", Values: map[string]string{"contains": "restart the daemon"}},
		},
	}

	result := runTestCase(tc)
	if result.Error != "" {
		t.Errorf("runTestCase failed: %s", result.Error)
	}
	if !result.Passed {
		t.Error("test did not pass")
	}
}
