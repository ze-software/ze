// Design: docs/architecture/testing/ci-format.md -- the stream assertion vocabulary
// Overview: runner_exec.go -- the run that produces rec.ClientOutput
// Related: record_parse.go -- the directives these fields come from

package runner

import (
	"fmt"
	"regexp"
	"strconv"
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
// Every expect= is checked against the stream it names in the output of the
// command it was written under, and against nothing else. It used to read
// rec.ClientOutput, the accumulated stdout AND stderr of every command in the
// file, so an assertion could be satisfied by a command its author never
// named. Two files did exactly that:
// test/plugin/kernel-capability-unknown-starts.ci and
// kernel-capability-doctor-reports.ci each asserted that `ze doctor` printed a
// diagnostic code and then ran `ze show diagnostic <that code>`, whose output
// satisfied the assertion on a host where the doctor check does not run at all.
//
// An expect= reads only the stream it names. It used to read the two streams
// joined, so a stdout assertion passed on a line the command wrote to stderr.
//
// A reject= never reads less than it did before the scoping, because a
// narrower reject is a weaker test. rejectScope decides what each kind reads.
func checkOutputAssertions(rec *Record) bool {
	for i := range rec.RunCommands {
		if !checkCommandAssertions(rec, &rec.RunCommands[i]) {
			return false
		}
	}
	return true
}

// commandStdout and commandStderr answer the output one command wrote to
// each stream, as the half-open slice of that accumulator it wrote between
// starting and finishing.
//
// A span that was never closed answers empty rather than the whole buffer. A
// command the run never reached produced nothing, and reading the accumulator
// from its start offset to the end would hand it every later command's output,
// which is the defect this scoping exists to remove.
func commandStdout(rec *Record, cmd *RunCommand) string {
	return spanOf(rec.ClientStdout, cmd.stdoutFrom, cmd.stdoutTo)
}

func commandStderr(rec *Record, cmd *RunCommand) string {
	return spanOf(rec.ClientStderr, cmd.stderrFrom, cmd.stderrTo)
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

// commandHolding answers the command whose stream satisfies holds, skipping
// the one that asserted it. It is the diagnosis for the authoring slip this
// scoping exposes: an assertion written under a command that does not produce
// what it names. nil when no command wrote it, which is the ordinary failure.
//
// The search runs from the last command back. A daemon's span stays open until
// the run ends, so it also covers every byte a later command wrote; the latest
// command whose span holds the needle is the narrowest, and the likeliest
// writer.
func commandHolding(rec *Record, asserting *RunCommand, stream func(*Record, *RunCommand) string, holds func(string) bool) *RunCommand {
	for i := len(rec.RunCommands) - 1; i >= 0; i-- {
		other := &rec.RunCommands[i]
		if other == asserting {
			continue
		}
		if holds(stream(rec, other)) {
			return other
		}
	}
	return nil
}

// commandWithin answers the latest command, other than asserting, whose span
// on st lies inside asserting's span and holds the text. It narrows where a
// forbidden line that a reject under a daemon saw sits: the daemon's span
// covers every later command's bytes, and a span inside it is narrower. It
// cannot say which process wrote the line, only whose span it was found
// inside. A span that merely overlaps is not inside, so a reject under a
// helper never points at the daemon whose span is wider than its own. nil
// when no narrower span holds it.
func commandWithin(rec *Record, asserting *RunCommand, st streamOf, holds func(string) bool) *RunCommand {
	from, to := st.bounds(asserting)
	for i := len(rec.RunCommands) - 1; i >= 0; i-- {
		other := &rec.RunCommands[i]
		if other == asserting {
			continue
		}
		otherFrom, otherTo := st.bounds(other)
		if otherFrom < from {
			continue
		}
		if otherTo > to {
			continue
		}
		if holds(st.read(rec, other)) {
			return other
		}
	}
	return nil
}

// streamOf pairs the stream an assertion reads with the one it does not, so a
// miss can say the command wrote the needle to the other stream.
type streamOf struct {
	name, otherName string
	read, other     func(*Record, *RunCommand) string
	bounds          func(*RunCommand) (from, to int)
}

var (
	streamStdout = streamOf{name: "stdout", otherName: "stderr", read: commandStdout, other: commandStderr,
		bounds: func(c *RunCommand) (int, int) { return c.stdoutFrom, c.stdoutTo }}
	streamStderr = streamOf{name: "stderr", otherName: "stdout", read: commandStderr, other: commandStdout,
		bounds: func(c *RunCommand) (int, int) { return c.stderrFrom, c.stderrTo }}
)

// rejectSpan is what every reject= except reject=stderr:pattern= reads: both
// streams of its own command's span, joined, so the stream name on such a
// reject selects nothing. That is what the command-scoped contains= rejects
// and reject=stdout:pattern= read before expect= learned to select a stream;
// a reject that read one stream would check less than it did.
//
// reject=stderr:pattern= is the other kind: a log check over the WHOLE run's
// stderr wherever it sits in the file, read by validateLogging exactly as it
// was when the pattern lived on Record. These two are the only places a
// reject's scope is decided.
//
// The last foreground helper's rejects read from its start to the end of the
// run, past the end its expects stop at (RunCommand.rejectsToRunEnd).
func rejectSpan(rec *Record, cmd *RunCommand) string {
	if cmd.rejectsToRunEnd {
		return spanOf(rec.ClientStdout, cmd.stdoutFrom, len(rec.ClientStdout)) +
			spanOf(rec.ClientStderr, cmd.stderrFrom, len(rec.ClientStderr))
	}
	return commandStdout(rec, cmd) + commandStderr(rec, cmd)
}

// checkCommandAssertions evaluates one command's own assertions: each expect=
// against the stream it names within this command's span, and each reject=
// other than reject=stderr:pattern= against both streams of that span
// (rejectSpan). Every failure names the command, so a multi-command file says
// WHICH step disagreed.
func checkCommandAssertions(rec *Record, cmd *RunCommand) bool {
	if !cmd.assertsAStream() {
		return true
	}
	fail := func(assert string, err error) bool {
		rec.Error = fmt.Errorf("cmd seq=%d (%s): %w", cmd.Seq, cmd.Exec, err)
		rec.FailureType = FailTypeStdoutMismatch
		rec.recordStep(assert, false, rec.Error.Error())
		return false
	}

	// An assertion that names the wrong command or the wrong stream is the
	// common authoring slip, and the runner can answer it rather than leaving
	// the author to bisect: it holds both streams of every command. So a miss
	// says WHERE the needle was written, when it was.
	diagnose := func(assert string, st streamOf, what string, holds func(string) bool) bool {
		if holds(st.other(rec, cmd)) {
			return fail(assert, fmt.Errorf("output %s on %s, but this command wrote it to %s: "+
				"assert on %s, or write it to %s", what, st.name, st.otherName, st.otherName, st.name))
		}
		if other := commandHolding(rec, cmd, st.read, holds); other != nil {
			return fail(assert, fmt.Errorf("output %s, but cmd seq=%d (%s) wrote it: "+
				"move the assertion under that command", what, other.Seq, other.Exec))
		}
		return fail(assert, fmt.Errorf("output %s", what))
	}
	missing := func(assert string, st streamOf, needle string) bool {
		return diagnose(assert, st, "does not contain "+strconv.Quote(needle),
			func(span string) bool { return strings.Contains(span, needle) })
	}
	unmatched := func(assert string, st streamOf, re *regexp.Regexp) bool {
		return diagnose(assert, st, "does not match regex "+strconv.Quote(re.String()), re.MatchString)
	}
	// A reject reads both streams, and a daemon's span covers every byte a
	// later command wrote. So the failure names the stream the text is on and
	// the narrowest command span inside this one that holds it.
	forbid := func(assert, what string, holds func(string) bool) bool {
		for _, st := range [...]streamOf{streamStdout, streamStderr} {
			if !holds(st.read(rec, cmd)) {
				continue
			}
			if inner := commandWithin(rec, cmd, st, holds); inner != nil {
				return fail(assert, fmt.Errorf("output %s on %s, found inside the output span of cmd seq=%d (%s)",
					what, st.name, inner.Seq, inner.Exec))
			}
			return fail(assert, fmt.Errorf("output %s on %s", what, st.name))
		}
		return fail(assert, fmt.Errorf("output %s across stdout and stderr joined", what))
	}
	forbidText := func(assert, needle string) bool {
		return forbid(assert, "unexpectedly contains "+strconv.Quote(needle),
			func(span string) bool { return strings.Contains(span, needle) })
	}
	forbidMatch := func(assert string, re *regexp.Regexp) bool {
		return forbid(assert, "matches forbidden regex "+strconv.Quote(re.String()), re.MatchString)
	}

	stdout, stderr := commandStdout(rec, cmd), commandStderr(rec, cmd)
	both := rejectSpan(rec, cmd)
	for _, expected := range cmd.ExpectStderrHas {
		if !strings.Contains(stderr, expected) {
			return missing("stderr-contains", streamStderr, expected)
		}
		rec.recordStep("stderr-contains", true, "")
	}
	for _, forbidden := range cmd.RejectStderrHas {
		if strings.Contains(both, forbidden) {
			return forbidText("stderr-reject-contains", forbidden)
		}
		rec.recordStep("stderr-reject-contains", true, "")
	}
	for _, pattern := range cmd.ExpectStderrRe {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fail("stderr-regex", fmt.Errorf("invalid stderr regex %q: %w", pattern, err))
		}
		if !re.MatchString(stderr) {
			return unmatched("stderr-regex", streamStderr, re)
		}
		rec.recordStep("stderr-regex", true, "")
	}
	// cmd.RejectStderrRe is not read here: it is a whole-run log check
	// (validateLogging), see rejectSpan.
	for _, expected := range cmd.ExpectStdout {
		if !strings.Contains(stdout, expected) {
			return missing("stdout-contains", streamStdout, expected)
		}
		rec.recordStep("stdout-contains", true, "")
	}
	for _, forbidden := range cmd.RejectStdout {
		if strings.Contains(both, forbidden) {
			return forbidText("stdout-not-contains", forbidden)
		}
		rec.recordStep("stdout-not-contains", true, "")
	}
	for _, pattern := range cmd.ExpectStdoutRe {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fail("stdout-regex", fmt.Errorf("invalid stdout regex %q: %w", pattern, err))
		}
		if !re.MatchString(stdout) {
			return unmatched("stdout-regex", streamStdout, re)
		}
		rec.recordStep("stdout-regex", true, "")
	}
	for _, pattern := range cmd.RejectStdoutRe {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fail("stdout-reject-regex", fmt.Errorf("invalid stdout regex %q: %w", pattern, err))
		}
		if re.MatchString(both) {
			return forbidMatch("stdout-reject-regex", re)
		}
		rec.recordStep("stdout-reject-regex", true, "")
	}
	return true
}
