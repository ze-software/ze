// Related: runlog.go -- the two answers these cases drive from their entry point
//
// VALIDATES: the lines a reader of a failed run needs first are the lines these
// functions return, and the limit is honored.
// PREVENTS: a summary that renders a whole log, and a numbering that points at
// the wrong line of it.

package runlog

import (
	"strings"
	"testing"
)

// testLog is one go test run: two packages passing, one failing, and a panic.
const testLog = `ok  	github.com/ze-software/ze/internal/core/env	0.012s
--- FAIL: TestEAPIdentity (0.00s)
    eap_test.go:41: identity was empty
FAIL	github.com/ze-software/ze/internal/core/eap	0.310s
ok  	github.com/ze-software/ze/internal/core/textbuf	0.004s
panic: send on closed channel
`

func TestTheFailureLinesCarryTheLineTheyCameFrom(t *testing.T) {
	lines, err := Key(strings.NewReader(testLog), 10)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}

	want := []string{
		"2:--- FAIL: TestEAPIdentity (0.00s)",
		"4:FAIL\tgithub.com/ze-software/ze/internal/core/eap\t0.310s",
		"6:panic: send on closed channel",
	}
	if len(lines) != len(want) {
		t.Fatalf("the log answered %d key lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
	}
	for index := range want {
		if lines[index] != want[index] {
			t.Errorf("key line %d is %q, want %q", index, lines[index], want[index])
		}
	}
}

// TestAPassingRunHasNoKeyLines states what an empty answer means. A caller that
// read it as "the run passed" would be right here and wrong for a run that
// failed without writing a line of that shape, so the verdict stays the exit
// code's to give.
func TestAPassingRunHasNoKeyLines(t *testing.T) {
	lines, err := Key(strings.NewReader("ok  	github.com/ze-software/ze/internal/core/env	0.012s\n"), 10)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("a passing run answered %d key lines, want none:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}

func TestTheLimitStopsTheScan(t *testing.T) {
	lines, err := Key(strings.NewReader(testLog), 2)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("a limit of 2 answered %d lines:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}

// TestTheHeadIsTheFirstLinesUnnumbered covers the lint stage, whose findings
// start at the first line of its log.
func TestTheHeadIsTheFirstLinesUnnumbered(t *testing.T) {
	lines, err := Head(strings.NewReader(testLog), 2)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("the head answered %d lines, want 2:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if lines[1] != "--- FAIL: TestEAPIdentity (0.00s)" {
		t.Errorf("the second head line is %q, want it unnumbered", lines[1])
	}
}

// TestALastLineWithNoNewlineIsALine covers a log whose writer was killed
// mid-line, which is what a broken slot leaves behind.
func TestALastLineWithNoNewlineIsALine(t *testing.T) {
	lines, err := Key(strings.NewReader("ok  	one	0.1s\npanic: killed"), 10)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(lines) != 1 || lines[0] != "2:panic: killed" {
		t.Errorf("the truncated log answered %v, want the panic as line 2", lines)
	}
}

// TestAnEmptyLogAnswersNothing states that no line is invented for a job that
// wrote nothing, which is what a job refused before it started leaves.
func TestAnEmptyLogAnswersNothing(t *testing.T) {
	lines, err := Key(strings.NewReader(""), 10)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("an empty log answered %v", lines)
	}
	head, err := Head(strings.NewReader(""), 10)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(head) != 0 {
		t.Errorf("an empty log answered the head %v", head)
	}
}
