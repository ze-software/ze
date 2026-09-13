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

// setClientOutput stores the run's two accumulators and their concatenation
// together.
//
// One writer for all three, because a command's span indexes into ClientStdout
// and ClientStderr while every report reads ClientOutput: setting one without
// the others leaves an assertion reading a buffer that does not hold what the
// reader was shown.
func setClientOutput(rec *Record, stdout, stderr string) {
	rec.ClientStdout = stdout
	rec.ClientStderr = stderr
	rec.ClientOutput = stdout + stderr
	// Closing the spans HERE rather than at the end of the run is what makes
	// the early-return paths safe: a test that gives up mid-file still reaches
	// this setter, and a command whose span was left open then ends at the
	// output that exists rather than at an offset past it.
	closeOpenSpans(rec, len(stdout), len(stderr))
}

// closeOpenSpans ends every span still open when the run finished.
//
// A background process is still writing when the test ends, and a foreground
// daemon's bytes arrive throughout its life rather than in one block, so both
// close at the final accumulator lengths. A command the run never REACHED never
// opened a span at all: its marks stay spanOpen with stdoutFrom zero, and it is
// closed empty, because charging it every later command's output is the defect
// this scoping removes rather than a smaller version of it.
func closeOpenSpans(rec *Record, stdoutLen, stderrLen int) {
	for i := range rec.RunCommands {
		cmd := &rec.RunCommands[i]
		// stdoutFrom still spanOpen means the command never launched, so it
		// wrote nothing and its span stays empty.
		if cmd.stdoutFrom != spanOpen && cmd.stdoutTo == spanOpen {
			cmd.stdoutTo = stdoutLen
		}
		if cmd.stderrFrom != spanOpen && cmd.stderrTo == spanOpen {
			cmd.stderrTo = stderrLen
		}
	}
}

// checkOutputAssertions evaluates every scoped stream assertion the .ci
// declared, and reports whether they all held. It runs whenever the test
// declares one, regardless of whether an exit code was asserted: these checks
// used to be nested inside the `if rec.ExpectExitCode != nil` block, so a
// cmd=foreground test that only checked stdout/stderr/files had every assertion
// silently skipped and then fell through to a default "unknown" failure
// (handover 3).
//
// Every assertion is checked against the output of the command it was written
// under, and against nothing else. It used to read rec.ClientOutput, the
// accumulated stdout AND stderr of every command in the file, so an assertion
// could be satisfied by a command its author never named and a reject could
// trip on one. Two files did exactly that:
// test/plugin/kernel-capability-unknown-starts.ci and
// kernel-capability-doctor-reports.ci each asserted that `ze doctor` printed a
// diagnostic code and then ran `ze explain <that code>`, whose output satisfied
// the assertion on a host where the doctor check does not run at all.
//
// The stream name still selects nothing for a `contains=`: a command's span
// covers both its streams, because a program is free to write a message to
// either and a test that pins which one is asserting the wrong thing.
// expect=stderr:pattern= is a different mechanism over the daemon's relayed
// stderr and stays with validateLogging.
func checkOutputAssertions(rec *Record) bool {
	for i := range rec.RunCommands {
		if !checkCommandAssertions(rec, &rec.RunCommands[i]) {
			return false
		}
	}
	return true
}

// commandSpan answers the output one command produced, as the half-open slice
// of each accumulator it wrote between starting and finishing.
//
// A span that was never closed answers empty rather than the whole buffer. A
// command the run never reached produced nothing, and reading the accumulator
// from its start offset to the end would hand it every later command's output,
// which is the defect this scoping exists to remove.
func commandSpan(rec *Record, cmd *RunCommand) string {
	return spanOf(rec.ClientStdout, cmd.stdoutFrom, cmd.stdoutTo) +
		spanOf(rec.ClientStderr, cmd.stderrFrom, cmd.stderrTo)
}

func spanOf(buf string, from, to int) string {
	if from < 0 || to <= from {
		return ""
	}
	if to > len(buf) {
		to = len(buf)
	}
	if from > len(buf) {
		return ""
	}
	return buf[from:to]
}

// commandHolding answers the command whose span satisfies holds, skipping the
// one that asserted it. It is the diagnosis for the authoring slip this scoping
// exposes: an assertion written under a command that does not produce what it
// names. nil when no command wrote it, which is the ordinary failure.
func commandHolding(rec *Record, asserting *RunCommand, holds func(string) bool) *RunCommand {
	for i := range rec.RunCommands {
		other := &rec.RunCommands[i]
		if other == asserting {
			continue
		}
		if holds(commandSpan(rec, other)) {
			return other
		}
	}
	return nil
}

// checkCommandAssertions evaluates one command's own assertions against its own
// span, naming the command in every failure so a multi-command file says WHICH
// step disagreed.
func checkCommandAssertions(rec *Record, cmd *RunCommand) bool {
	if !cmd.assertsAStream() {
		return true
	}
	out := commandSpan(rec, cmd)

	fail := func(assert string, err error) bool {
		rec.Error = fmt.Errorf("cmd seq=%d (%s): %w", cmd.Seq, cmd.Exec, err)
		rec.FailureType = FailTypeStdoutMismatch
		rec.recordStep(assert, false, rec.Error.Error())
		return false
	}

	// An assertion that names the wrong command is the common authoring slip,
	// and the runner can answer it rather than leaving the author to bisect: it
	// holds every command's span. So a miss says WHICH command wrote the needle,
	// when one did.
	missing := func(assert, needle string) bool {
		if other := commandHolding(rec, cmd, func(span string) bool { return strings.Contains(span, needle) }); other != nil {
			return fail(assert, fmt.Errorf("output does not contain %q, but cmd seq=%d (%s) wrote it: "+
				"move the assertion under that command", needle, other.Seq, other.Exec))
		}
		return fail(assert, fmt.Errorf("output does not contain %q", needle))
	}

	unmatched := func(assert string, re *regexp.Regexp) bool {
		if other := commandHolding(rec, cmd, re.MatchString); other != nil {
			return fail(assert, fmt.Errorf("output does not match regex %q, but cmd seq=%d (%s) matches it: "+
				"move the assertion under that command", re.String(), other.Seq, other.Exec))
		}
		return fail(assert, fmt.Errorf("output does not match regex %q", re.String()))
	}

	for _, expected := range cmd.ExpectStderrHas {
		if !strings.Contains(out, expected) {
			return missing("stderr-contains", expected)
		}
		rec.recordStep("stderr-contains", true, "")
	}
	for _, forbidden := range cmd.RejectStderrHas {
		if strings.Contains(out, forbidden) {
			return fail("stderr-reject-contains", fmt.Errorf("output unexpectedly contains %q", forbidden))
		}
		rec.recordStep("stderr-reject-contains", true, "")
	}
	for _, expected := range cmd.ExpectStdout {
		if !strings.Contains(out, expected) {
			return missing("stdout-contains", expected)
		}
		rec.recordStep("stdout-contains", true, "")
	}
	for _, forbidden := range cmd.RejectStdout {
		if strings.Contains(out, forbidden) {
			return fail("stdout-not-contains", fmt.Errorf("output unexpectedly contains %q", forbidden))
		}
		rec.recordStep("stdout-not-contains", true, "")
	}
	for _, pattern := range cmd.ExpectStdoutRe {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fail("stdout-regex", fmt.Errorf("invalid stdout regex %q: %w", pattern, err))
		}
		if !re.MatchString(out) {
			return unmatched("stdout-regex", re)
		}
		rec.recordStep("stdout-regex", true, "")
	}
	for _, pattern := range cmd.RejectStdoutRe {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fail("stdout-reject-regex", fmt.Errorf("invalid stdout regex %q: %w", pattern, err))
		}
		if re.MatchString(out) {
			return fail("stdout-reject-regex", fmt.Errorf("output matches forbidden regex %q", pattern))
		}
		rec.recordStep("stdout-reject-regex", true, "")
	}
	return true
}
