package interoplab

import (
	"strings"
	"testing"
)

// VALIDATES: user-facing-errors: a listing failure in Suite.Run's pre-run
// orphan sweep says it was that sweep, why the run stops, and the commands that
// retry it. Method: the labeled container listing fails over the fake runner.
// PREVENTS: a bare docker error with no word of the sweep and no fix.
func TestSuiteRunOrphanSweepFailureNamesWhatWhyAndFix(t *testing.T) {
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if strings.HasPrefix(strings.Join(command.Arguments, " "), "docker ps -a") {
			return ProcessResult{ExitCode: 1, Stderr: "permission denied"}, nil
		}
		return ProcessResult{}, nil
	}}
	suite := Suite{Docker: newDocker(runner), NoBuild: true, Scenarios: []ScenarioPlan{{Source: ScenarioSource{Name: "one"}}}}
	report := suite.Run(t.Context())
	if report.Code != 1 {
		t.Fatalf("Suite code = %d, want 1", report.Code)
	}
	for _, want := range []string{"pre-run", "permission denied", "Fix:", "./le test integration cleanup"} {
		if !strings.Contains(report.SetupError, want) {
			t.Errorf("setup error %q does not name %q", report.SetupError, want)
		}
	}
}
