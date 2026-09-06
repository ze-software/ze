// Design: docs/architecture/testing/ci-format.md -- the stream assertion vocabulary
// Overview: runner_exec.go -- the run that produces rec.ClientOutput
// Related: record_parse.go -- the directives these fields come from

package runner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/test/trace"
)

// recordStep appends one assertion result to the record's step trace. The step
// number is derived from the trace's own length, so a caller cannot hand out a
// number that disagrees with the position.
func (r *Record) recordStep(assert string, passed bool, detail string) {
	r.StepTrace = append(r.StepTrace, trace.StepResult{
		Step: len(r.StepTrace) + 1, Kind: stepKindExpect, Assert: assert,
		Passed: passed, Detail: detail,
	})
}

// checkOutputAssertions evaluates every stdout and stderr assertion the .ci
// declared, and reports whether they all held. It runs whenever the test
// declares one, regardless of whether an exit code was asserted: these checks
// used to be nested inside the `if rec.ExpectExitCode != nil` block, so a
// cmd=foreground test that only checked stdout/stderr/files had every assertion
// silently skipped and then fell through to a default "unknown" failure
// (handover 3).
//
// Every assertion reads rec.ClientOutput, which is the accumulated stdout AND
// stderr of every command in the file (runner_exec.go). So these are FILE-level
// checks, the stream named in the directive selects nothing, and a file that
// asserts a needle present for one command and absent for another asserts two
// contradictory things about one string. Split such a file.
func checkOutputAssertions(rec *Record) bool {
	for _, expected := range rec.ExpectStderrMatch {
		if !strings.Contains(rec.ClientOutput, expected) {
			rec.Error = fmt.Errorf("stderr does not contain %q", expected)
			rec.FailureType = "stderr_mismatch"
			rec.recordStep("stderr-contains", false, rec.Error.Error())
			return false
		}
		rec.recordStep("stderr-contains", true, "")
	}
	for _, forbidden := range rec.RejectStderrMatch {
		if strings.Contains(rec.ClientOutput, forbidden) {
			rec.Error = fmt.Errorf("stderr unexpectedly contains %q", forbidden)
			rec.FailureType = "stderr_mismatch"
			rec.recordStep("stderr-reject-contains", false, rec.Error.Error())
			return false
		}
		rec.recordStep("stderr-reject-contains", true, "")
	}
	for _, expected := range rec.ExpectStdoutMatch {
		if !strings.Contains(rec.ClientOutput, expected) {
			rec.Error = fmt.Errorf("stdout does not contain %q", expected)
			rec.FailureType = FailTypeStdoutMismatch
			rec.recordStep("stdout-contains", false, rec.Error.Error())
			return false
		}
		rec.recordStep("stdout-contains", true, "")
	}
	for _, forbidden := range rec.ExpectStdoutNotMatch {
		if strings.Contains(rec.ClientOutput, forbidden) {
			rec.Error = fmt.Errorf("stdout unexpectedly contains %q", forbidden)
			rec.FailureType = FailTypeStdoutMismatch
			rec.recordStep("stdout-not-contains", false, rec.Error.Error())
			return false
		}
		rec.recordStep("stdout-not-contains", true, "")
	}
	for _, pattern := range rec.ExpectStdoutRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			rec.Error = fmt.Errorf("invalid stdout regex %q: %w", pattern, err)
			rec.FailureType = FailTypeStdoutMismatch
			rec.recordStep("stdout-regex", false, rec.Error.Error())
			return false
		}
		if !re.MatchString(rec.ClientOutput) {
			rec.Error = fmt.Errorf("stdout does not match regex %q", pattern)
			rec.FailureType = FailTypeStdoutMismatch
			rec.recordStep("stdout-regex", false, rec.Error.Error())
			return false
		}
		rec.recordStep("stdout-regex", true, "")
	}
	for _, pattern := range rec.RejectStdoutRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			rec.Error = fmt.Errorf("invalid reject stdout regex %q: %w", pattern, err)
			rec.FailureType = FailTypeStdoutMismatch
			rec.recordStep("stdout-reject-regex", false, rec.Error.Error())
			return false
		}
		if re.MatchString(rec.ClientOutput) {
			rec.Error = fmt.Errorf("stdout matches forbidden regex %q", pattern)
			rec.FailureType = FailTypeStdoutMismatch
			rec.recordStep("stdout-reject-regex", false, rec.Error.Error())
			return false
		}
		rec.recordStep("stdout-reject-regex", true, "")
	}
	return true
}
