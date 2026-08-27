package main

// Brings the Python RFC requirement coverage gate (scripts/dev/rfc_requirements.py) into
// `go test`, so `make ze-unit-test` exercises it. The gate logic and its fixture-based
// unit tests live in rfc_requirements.py / rfc_requirements_test.py (--check /
// --selftest); these tests run the real script and assert its exit code.
//
// Spec: plan/spec-rfc-requirement-coverage.md. Skill: ai/skills/ze-rfc.md.

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// rfcGateTimeout bounds one run of the script. The slowest call measured on this
// tree is --check at about 72s and --selftest at about 41s, so this is roughly
// four times the worst case and still well inside the package's own -timeout.
const rfcGateTimeout = 300 * time.Second

// runRFCGate runs `python3 rfc_requirements.py <args...>` from the package directory
// (scripts/dev); the script resolves the repo root from its own location.
//
// The context has two parents on purpose. t.Context() ends the child when the test
// ends, which is what stops an orphaned python outliving a canceled run. It cannot
// bound the call itself: it is canceled just before the Cleanup functions run, and
// by then CombinedOutput has already returned. The timeout is what kills a hung
// script, so both are needed and neither replaces the other.
func runRFCGate(t *testing.T, args ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), rfcGateTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", append([]string{"rfc_requirements.py"}, args...)...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running rfc_requirements.py %v: %v", args, err)
	}
	return code, string(out)
}

// TestRFCRequirementsGate asserts every MUST-level requirement of every enrolled RFC is
// bound to a positive AND a negative test, or carries a reasoned annotation.
//
// A failure here means an RFC obligation Ze claims to meet has no test proving it, or a
// test that proved one was deleted or weakened.
func TestRFCRequirementsGate(t *testing.T) {
	code, out := runRFCGate(t, "--check")
	if code != 0 {
		t.Fatalf("rfc_requirements.py --check failed (exit %d) -- an enrolled RFC has a "+
			"MUST-level requirement with no test, only one polarity, a stale annotation, "+
			"or a tag naming an unknown requirement. See ai/skills/ze-rfc.md.\n%s", code, out)
	}
}

// TestRFCLedgerFresh asserts the committed ai/RFC-REQUIREMENTS.md and every shard under
// rfc/requirements/ match what their sources render to right now. A failure means a test
// was re-tagged, moved, or deleted without `make ze-rfc-index-update`, so a generated page lies
// about which tests enforce which requirement (AC-20). This is the same staleness that
// once slipped through: two commits re-tagged RFC 7606 tests and the ledger was not
// regenerated. The captured output names the file, which is the index, a shard that
// differs or is absent, or a shard the render no longer produces.
func TestRFCLedgerFresh(t *testing.T) {
	code, out := runRFCGate(t, "--check-fresh")
	if code != 0 {
		t.Fatalf("a generated RFC requirement page is stale (exit %d) -- run: make ze-rfc-index-update\n%s", code, out)
	}
}

// TestRFCRequirementsSelftest runs the gate's own fixture-based unit tests before it is
// trusted to judge the tree: line parsing, id allocation, tag scanning (including .ci
// terminator blocks), polarity pairing, annotation staleness, and the enrolment ratchet.
func TestRFCRequirementsSelftest(t *testing.T) {
	code, out := runRFCGate(t, "--selftest")
	if code != 0 {
		t.Fatalf("rfc_requirements.py --selftest failed (exit %d):\n%s", code, out)
	}
}

// TestRFCRequirementsFailsClosed asserts the gate refuses to report clean when it has
// nothing to compare. "Clean" must mean "I compared things and found nothing", never
// "I compared nothing" (ai/rules/evidence.md).
//
// Guards the specific regression where rfc/enrolled.txt is emptied (or the file is lost)
// and the gate cheerfully passes while enforcing zero requirements.
func TestRFCRequirementsFailsClosed(t *testing.T) {
	code, out := runRFCGate(t, "--check")
	if code != 0 {
		t.Skipf("gate already failing for another reason; nothing to assert here:\n%s", out)
	}
	if !strings.Contains(out, "gated MUST-level requirement") {
		t.Fatalf("a clean gate run must report HOW MANY requirements it actually "+
			"enforced, so a vacuous pass is visible. Got:\n%s", out)
	}
	if strings.Contains(out, ": 0 gated MUST-level requirement") {
		t.Fatal("gate reported clean while enforcing 0 requirements -- that is a vacuous " +
			"pass, not a passing gate (ai/rules/evidence.md)")
	}
}
