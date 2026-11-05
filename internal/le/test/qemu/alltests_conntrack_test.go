// Design: docs/architecture/testing/qemu-integration.md -- native guest conntrack preparation.

package testqemu

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// TestQEMUAllPreparesConntrackBeforeSuites exercises both actual guest entry
// paths. A complete run and a selected plugin CI must prepare the conntrack
// source before any suite can observe it; successful fixture exit alone cannot
// establish that a live recent-flow ring was available.
func TestQEMUAllPreparesConntrackBeforeSuites(t *testing.T) {
	for _, selected := range []bool{false, true} {
		name := "whole-run"
		if selected {
			name = "one-ci"
		}
		t.Run(name, func(t *testing.T) {
			run := vmFixture(t)
			if selected {
				run.Test = "test/plugin/ddos-incident-confidence.ci"
				writeFile(t, run.Workspace, run.Test, "cmd=foreground:seq=1:exec=ze doctor\n")
			}
			prepared, suites := 0, 0
			run.Run = func(argv, _ []string, tally io.Writer) int {
				command := strings.Join(argv, " ")
				if strings.Contains(command, "nf_conntrack_acct") && strings.Contains(command, "ct state") {
					prepared++
				}
				if tally != nil {
					if prepared != 1 {
						t.Errorf("suite started after %d conntrack preparations, want exactly one", prepared)
					}
					suites++
					fmt.Fprintln(tally, "pass  1/1  100.0%  1s") //nolint:errcheck // in-memory tally
				}
				return 0
			}
			if _, code := run.Execute(); code != 0 {
				t.Fatalf("successful guest execution answered %d", code)
			}
			if prepared != 1 || suites == 0 {
				t.Fatalf("conntrack preparations=%d, suites=%d; want one preparation and an executed suite", prepared, suites)
			}
		})
	}
}

// TestQEMUAllRefusesConntrackPreparationFailure drives setup failure through
// each consumer. No later child or counted test may disguise missing kernel
// data as a fixture failure or a successful empty run.
func TestQEMUAllRefusesConntrackPreparationFailure(t *testing.T) {
	for _, selected := range []bool{false, true} {
		name := "whole-run"
		if selected {
			name = "one-ci"
		}
		t.Run(name, func(t *testing.T) {
			run := vmFixture(t)
			if selected {
				run.Test = "test/plugin/ddos-incident-confidence.ci"
				writeFile(t, run.Workspace, run.Test, "cmd=foreground:seq=1:exec=ze doctor\n")
			}
			failed, afterFailure, counted := false, 0, 0
			run.Run = func(argv, _ []string, tally io.Writer) int {
				if failed {
					afterFailure++
				}
				command := strings.Join(argv, " ")
				if strings.Contains(command, "nf_conntrack_acct") && strings.Contains(command, "ct state") {
					failed = true
					return 23
				}
				if tally != nil {
					counted++
					fmt.Fprintln(tally, "pass  1/1  100.0%  1s") //nolint:errcheck // in-memory tally
				}
				return 0
			}
			report, code := run.Execute()
			if code == 0 || !failed || afterFailure != 0 || counted != 0 || len(report.Phases) != 0 {
				t.Fatalf("missing preparation refusal: code=%d failed=%v later children=%d counted=%d phases=%d", code, failed, afterFailure, counted, len(report.Phases))
			}
		})
	}
}
