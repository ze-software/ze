package trend

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// VALIDATES: AC-19 -- with no monitor running, the read of the trend state
// answers "not running", never an empty list, and the monitor check reports it
// with the command that restarts the daemon, which starts the monitor; the
// rule's own check adds no second row. A monitor that stops reads the same.
// Method: the accessor and the registered runtime checks, called before any
// monitor starts and after one stops.
func TestTrendMonitorNotRunningIsReported(t *testing.T) {
	isolateTrend(t, nil)
	monitorCheck := registeredRuntimeCheck(t, monitorCheckName)
	if !slices.Contains(monitorCheck.Codes, CodeNotRunning) {
		t.Fatalf("check %s declares %v, want %s", monitorCheck.Name, monitorCheck.Codes, CodeNotRunning)
	}
	kernelCheck := registeredRuntimeCheck(t, kernelMemoryCheckName)

	assertNotRunning := func(when string) {
		t.Helper()
		statuses, err := Rules()
		if !errors.Is(err, ErrNotRunning) {
			t.Fatalf("%s: Rules error = %v, want ErrNotRunning", when, err)
		}
		if statuses != nil {
			t.Fatalf("%s: Rules answered %d rules with the monitor not running", when, len(statuses))
		}
		rows := monitorCheck.Check(diagnostic.DoctorCheckContext{})
		if len(rows) != 1 {
			t.Fatalf("%s: monitor check rows = %d, want 1: %+v", when, len(rows), rows)
		}
		if rows[0].Code != CodeNotRunning || rows[0].Severity != diagnostic.SeverityWarning ||
			rows[0].Fix != NotRunningFix() || rows[0].Message != notRunningMessage {
			t.Errorf("%s: monitor check row = %+v", when, rows[0])
		}
		if rows := kernelCheck.Check(diagnostic.DoctorCheckContext{}); len(rows) != 0 {
			t.Errorf("%s: the rule check added rows %+v", when, rows)
		}
	}

	assertNotRunning("before Start")
	run := startTrend(t, &host.Detector{Root: t.TempDir()})
	if _, err := Rules(); err != nil {
		t.Fatalf("Rules while running: %v", err)
	}
	run.monitor.Stop()
	assertNotRunning("after Stop")
}

// registeredRuntimeCheck answers the runtime doctor check registered as name.
func registeredRuntimeCheck(t *testing.T, name string) diagnostic.DoctorCheck {
	t.Helper()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no runtime doctor check named %s", name)
	return diagnostic.DoctorCheck{}
}
