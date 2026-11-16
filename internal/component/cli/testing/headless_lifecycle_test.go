package testing

import (
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
)

// TestHeadlessRestartFuncPropagation verifies that commandExecutor set on
// a HeadlessModel survives the TypeText + PressEnter chain.
// VALIDATES: .et lifecycle infrastructure -- commandExecutor through headless model.
// PREVENTS: commandExecutor nil when model processes "restart" command.
func TestHeadlessRestartFuncPropagation(t *testing.T) {
	hm, err := newHeadlessCommandModel()
	if err != nil {
		t.Fatalf("creating the command model: %v", err)
	}

	// Supply the canonical operational executor through the Model() pointer.
	called := false
	m := hm.Model()
	m.SetCommandExecutor(func(input string) (cli.CommandOutput, error) {
		if input != "request restart" {
			t.Errorf("restart dispatch = %q, want request restart", input)
		}
		return cli.CommandOutput{TransportComplete: func() { called = true }}, nil
	})

	if called {
		t.Fatal("executor ran before confirmation")
	}

	// Type "restart" and press Enter (same as .et input=type:text=restart + input=enter).
	hm.typeText("restart")
	hm.pressEnter()

	// The status message reveals whether commandExecutor was nil.
	// "restart not available" = nil, confirmation prompt = set.
	status := hm.StatusMessage()
	t.Logf("status: %q, called: %v", status, called)

	if status == "restart not available (not connected to daemon)" {
		t.Error("commandExecutor was nil when model processed 'restart' -- lost in headless Update chain")
	}
	if called {
		t.Fatal("accepted action ran before confirmation")
	}
	hm.typeText("y")
	if !called {
		t.Fatal("confirmed restart never completed its accepted action")
	}
}

// TestHeadlessRestartFuncViaRunTestCase runs an actual .et test case
// through the runner to reproduce the exact failure path.
//
// It calls runTestCaseIn, the runner's whole wiring (option=lifecycle:wired
// included), in t.TempDir(), and not runTestCase, whose only addition is the
// wall-clock budget runner_timeout_test.go already proves. This test is about
// the restart executor, not time: the store it opened used to fsync, and under
// a loaded -race verify one fsync on the TMPDIR disk outlasted the 5s budget it
// used to declare. The runner's tree-mode store is now in memory; `go test
// -timeout` stays the bound on a run that hangs.
func TestHeadlessRestartFuncViaRunTestCase(t *testing.T) {
	tc := &testCase{
		Options: []Option{
			{Type: "mode", Values: map[string]string{"value": "command"}},
			{Type: "lifecycle", Values: map[string]string{"mode": "wired"}},
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

	result := runTestCaseIn(tc, t.TempDir())
	if result.Error != "" {
		t.Errorf("runTestCaseIn failed: %s", result.Error)
	}
	if !result.Passed {
		t.Error("test did not pass")
	}
}
