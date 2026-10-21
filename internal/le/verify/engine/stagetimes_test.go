// VALIDATES: the stage-time history every run appends to and the next run
// reads its cost order from.
// PREVENTS: a torn or hostile line in a file every session shares deciding
// an order, or a malformed line disappearing without a count.
package verifyengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAMalformedHistoryLineIsSkippedAndCounted proves the reader takes only
// lines the engine could have written: six columns, finite non-negative
// seconds (zero is valid), an integer exit and an RFC 3339 start. Every other
// line is skipped and counted, and the run's combined log says how many.
func TestAMalformedHistoryLineIsSkippedAndCounted(t *testing.T) {
	checkout := t.TempDir()
	writeHistory(t, checkout,
		historyLine("good", "0"),
		historyLine("negative", "-1"),
		historyLine("nan", "NaN"),
		historyLine("inf", "+Inf"),
		historyLine("word", "fast"),
		"torn\t1.0\t0",
		"badexit\t1.0\tx\tfull\tabc\t2026-10-21T03:00:00Z",
		"badstart\t1.0\t0\tfull\tabc\tyesterday",
		"\t1.0\t0\tfull\tabc\t2026-10-21T03:00:00Z",
	)
	costs, err := ReadStageCosts(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if costs.Malformed != 8 {
		t.Fatalf("malformed = %d, want 8", costs.Malformed)
	}
	if seconds, ok := costs.Seconds["good"]; !ok || seconds != 0 || len(costs.Seconds) != 1 {
		t.Fatalf("costs = %#v, want only good at 0s", costs.Seconds)
	}
	report := RunMode(context.Background(), t.TempDir(), "abc", Mode,
		Plan{History: checkout}, recordingRunner("", 0, new([]string)), Slot{})
	if !strings.Contains(report.Console, "Stage-time history: 8 malformed lines skipped") {
		t.Fatalf("combined log does not count the skipped lines:\n%.400s", report.Console)
	}
}

// TestEveryReachedStageIsAppendedToTheHistory proves AC-7 at the engine: a
// run appends one line per reached stage (stage, seconds, exit, mode, commit,
// start) to the history it names, green or red, after any line already there.
func TestEveryReachedStageIsAppendedToTheHistory(t *testing.T) {
	checkout := t.TempDir()
	writeHistory(t, checkout, historyLine("earlier", "2.000"))
	population := StagesForMode(Mode)
	report := RunMode(context.Background(), t.TempDir(), "abc", Mode,
		Plan{History: checkout, FirstRed: true}, recordingRunner(population[1].Identity.Name, 1, new([]string)), Slot{})
	if report.Code != 1 {
		t.Fatalf("exit %d, want 1", report.Code)
	}
	data, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(StageTimesPath)))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("history holds %d lines, want the earlier one and the two reached stages:\n%s", len(lines), data)
	}
	fields := strings.Split(lines[2], "\t")
	if len(fields) != 6 || fields[0] != population[1].Identity.Name || fields[2] != "1" ||
		fields[3] != Mode || fields[4] != "abc" {
		t.Fatalf("red stage line %q", lines[2])
	}
	if !strings.Contains(report.Console, "Stage times recorded in "+StageTimesPath) {
		t.Fatal("the combined log does not say the times were recorded")
	}
}

// TestAnOverLongHistoryLineIsSkippedAndCounted proves one line longer than
// stageTimeLineMax costs the reader that line and nothing else. The history is
// shared by every session on the machine, so a torn write or a hostile writer
// can leave such a line; a reader that failed wholesale on it would put every
// later run back in declared order, silently, for as long as the line stayed.
//
// Method: a valid line, an over-long one, a second valid line, and an
// over-long final line with no newline. Both valid stages must be read, both
// long lines counted, and no error answered.
func TestAnOverLongHistoryLineIsSkippedAndCounted(t *testing.T) {
	checkout := t.TempDir()
	long := strings.Repeat("x", 2*stageTimeLineMax)
	path := filepath.Join(checkout, filepath.FromSlash(StageTimesPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	content := historyLine("before", "2") + "\n" + long + "\n" + historyLine("after", "3") + "\n" + long
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	costs, err := ReadStageCosts(checkout)
	if err != nil {
		t.Fatalf("ReadStageCosts over an over-long line = %v, want the line skipped", err)
	}
	if costs.Seconds["before"] != 2 || costs.Seconds["after"] != 3 || len(costs.Seconds) != 2 {
		t.Fatalf("costs = %v, want before=2 and after=3 only", costs.Seconds)
	}
	if costs.Malformed != 2 {
		t.Fatalf("malformed = %d, want the 2 over-long lines counted", costs.Malformed)
	}
}
