// Design: docs/architecture/testing/qemu-integration.md -- what the VM proves
// Related: alltests.go -- the run whose suite children this reads
// Related: alltests_report.go -- where the count is carried
//
// alltests_tally.go reads the number of tests a functional suite EXECUTED out
// of the suite's own output.
//
// An exit code alone cannot tell a suite that ran two thousand tests from one
// that ran none. The .ci runner answers 0 when its selection is empty
// (Runner.Run, internal/test/runner/runner.go: "no tests selected"), so a suite
// whose directory moved, whose verb changed, or whose wrapper died before the
// first test reports success, and the run's summary renders ALL PHASES PASSED
// over a population that never started. That is the defect
// plan/journal/gate-excludes-part-of-its-population.md is named for.
//
// The count is the runner's own summary line, which every functional suite
// prints through one producer (Display.Summary, internal/test/runner/display.go):
//
//	pass  59/59  100.0%  22.7s
//	fail  40/42  95.2%  3.2s  failed 2 [a, b]
//
// That producer prints NOTHING when it has neither an executed test nor a
// skipped one, so an absent line is itself the answer: the suite ran nothing.

package qemu

import (
	"strconv"
	"strings"
)

// tallyLineMax bounds the line being assembled. A .ci test can print a single
// line of any length, and the summary is under 100 columns, so a longer line
// is discarded rather than accumulated.
const tallyLineMax = 4096

// The two words the summary line can start with.
const (
	tallyPass = "pass"
	tallyFail = "fail"
)

// suiteTally counts the tests one functional suite executed, by reading the
// runner's summary line out of a copy of its stdout.
//
// It is an io.Writer so it can sit beside the terminal in a MultiWriter
// (gaterun.StreamTee) and never delay a byte a person is waiting for. It is
// written by one goroutine, the one copying the child's output, and read after
// the child exits. It is NOT safe for concurrent use.
type suiteTally struct {
	line []byte
	// over says the line being assembled passed tallyLineMax and is discarded
	// to its newline.
	over bool
	// Seen says a summary line was read. Without it the suite executed no test
	// and skipped none, which is not a pass.
	Seen bool
	// Tests is how many tests the suite executed: passed plus failed plus timed
	// out, which is the denominator the runner prints. Skipped tests are not in
	// it, so a suite whose whole population skipped answers 0 with Seen true.
	Tests int
}

// Write assembles lines and reads the summary out of each completed one.
func (t *suiteTally) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			t.read(string(t.line))
			t.line = t.line[:0]
			t.over = false
			continue
		}
		if t.over {
			continue
		}
		if len(t.line) == tallyLineMax {
			t.over = true
			t.line = t.line[:0]
			continue
		}
		t.line = append(t.line, b)
	}
	return len(p), nil
}

// read takes the count from one line when that line is the summary.
//
// The shape is checked on three fields rather than one: the verdict word, a
// `passed/total` pair, and a percentage. A test that prints "pass" or a
// fraction of its own therefore does not become a count.
func (t *suiteTally) read(line string) {
	fields := strings.Fields(withoutANSI(line))
	if len(fields) < 3 {
		return
	}
	if fields[0] != tallyPass && fields[0] != tallyFail {
		return
	}
	if !strings.HasSuffix(fields[2], "%") {
		return
	}
	_, total, ok := strings.Cut(fields[1], "/")
	if !ok {
		return
	}
	executed, err := strconv.Atoi(total)
	if err != nil {
		return
	}
	t.Seen = true
	t.Tests = executed
}

// withoutANSI removes the color escapes the runner writes when its stdout is a
// terminal. The guest run is a pipe, where the runner writes none, so this
// covers the operator who runs the same action by hand inside the VM.
//
// An escape is ESC '[' then parameter bytes then one final letter. The loop is
// bounded by the line, which Write has already bounded.
func withoutANSI(line string) string {
	if !strings.ContainsRune(line, 0x1b) {
		return line
	}
	var out strings.Builder
	out.Grow(len(line))
	for i := 0; i < len(line); i++ {
		if line[i] != 0x1b {
			out.WriteByte(line[i])
			continue
		}
		i++
		if i >= len(line) || line[i] != '[' {
			continue
		}
		for i++; i < len(line); i++ {
			if line[i] >= '@' && line[i] <= '~' {
				break
			}
		}
	}
	return out.String()
}
