// The migration proof for `ze-rfc-selftest`: the Go action keeps the stable
// Python success text and code while returning structured fixture rows.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and step 12e part 5.
// PREVENTS: A shell-out selftest, an unclaimed action, or an unstructured success.
package main

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/rfc"
)

func TestRFCSelftestActionMatchesTheStablePythonContract(t *testing.T) {
	const pythonSuccess = "rfc_requirements selftest OK\n"

	legacy := devPyRunScript(t, "rfc_requirements.py", []string{"--selftest"}, devPyRoot(t))
	answer, code := rfc.Answer([]string{"selftest"})
	report, ok := answer.(leroot.SelftestReport)
	if !ok {
		t.Fatalf("selftest answer type %T, want leroot.SelftestReport", answer)
	}
	if code != legacy.Code {
		t.Fatalf("Go code %d, Python code %d", code, legacy.Code)
	}
	if len(report.Results) == 0 {
		t.Fatal("structured selftest report has no fixture rows")
	}
	if code == 0 {
		if legacy.Stdout != pythonSuccess {
			t.Fatalf("Python success output %q, want %q", legacy.Stdout, pythonSuccess)
		}
		if report.Text() != pythonSuccess {
			t.Fatalf("Go success output %q, want %q", report.Text(), pythonSuccess)
		}
		return
	}
	if !strings.Contains(legacy.Stderr, "TestRealTreeIsGreen.test_run_check_exits_zero_on_the_real_tree") {
		t.Fatalf("legacy red does not name TestRealTreeIsGreen: %s", legacy.Stderr)
	}
	for _, row := range report.Results {
		if row.Case == "real-tree/public-check" && !row.Passed {
			return
		}
	}
	t.Fatal("Go red has no failed real-tree/public-check row")
}

func TestRFCSelftestActionIsClaimedReadOnlyAndInProcess(t *testing.T) {
	for _, action := range rfc.Actions().Actions {
		if action.Gate != "ze-rfc-selftest" {
			continue
		}
		if action.Verb != "selftest" {
			t.Fatalf("verb %q, want selftest", action.Verb)
		}
		if action.Writes {
			t.Fatal("ze-rfc-selftest is marked as writing")
		}
		if len(action.Forks) != 0 {
			t.Fatalf("ze-rfc-selftest still forks %v", action.Forks)
		}
		return
	}
	t.Fatal("ze-rfc-selftest is not claimed by the rfc action table")
}
