// Design: website/AI.md -- the commit calendar counts what each commit added and deleted
package reporewrite

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// activityTestRepository builds a repository with two commits. The first, on
// 2026-08-10, adds a Go file of five lines, a Markdown file of four, a
// third-party Go file of three under vendor/, a first-party Go file of two
// under a directory merely named vendor, and a binary file. The second, on
// 2026-08-12, cuts the Go file to two lines and the Markdown file to one,
// deletes the third-party file, moves the first-party file out of its vendor
// directory unchanged, and rewrites the binary file.
func activityTestRepository(t *testing.T) string {
	t.Helper()
	// git rev-parse --show-toplevel answers the symlink-resolved path, so the
	// root is resolved here, as the measurement will report it.
	repository, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(arguments ...string) {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", repository}, arguments...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
		}
	}
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(repository, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(day string) {
		t.Helper()
		stamp := day + "T12:00:00Z"
		command := exec.CommandContext(t.Context(), "git", "-C", repository, "-c", "commit.gpgsign=false", "commit", "-q", "-m", day)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git commit %s: %v: %s", day, err, output)
		}
	}

	git("init", "-q", "-b", "main")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.test")
	write("main.go", "package main\n\nfunc a() {}\n\nfunc b() {}\n")
	write("notes.md", "one\ntwo\nthree\nfour\n")
	write("vendor/lib/lib.go", "package lib\n\nfunc L() {}\n")
	write("internal/tool/vendor/own.go", "package vendor\n// Own code.\n")
	write("logo.bin", "\x00\x01\x02binary\n")
	git("add", ".")
	commit("2026-08-10")

	write("main.go", "package main\n\n")
	write("notes.md", "one\n")
	git("rm", "-q", "vendor/lib/lib.go")
	git("mv", "internal/tool/vendor/own.go", "internal/tool/own.go")
	write("logo.bin", "\x00\x03\x04other\nlines\n")
	git("add", ".")
	commit("2026-08-12")
	return repository
}

// activityTestToday is the day both measurements end on, three days after the
// second commit.
var activityTestToday = time.Date(2026, time.August, 15, 9, 0, 0, 0, time.UTC)

// TestCommittedActivityCountsWhatEachCommitDeleted proves the published page's
// measurement counts deleted lines beside added ones, over every first-party
// file, under one set of path rules.
//
// The second day's square is drawn at heat level 1, because the heat scale
// reads added lines (2 of the first day's 11).
//
// Method: measure the fixture repository and compare each day's added and
// deleted lines with what the two commits did to each file. The Markdown file
// counts, the vendor/ file does not, the file under a first-party directory
// named vendor does, its move counts as two deleted and two added lines, and
// the binary file counts nothing. Over a window holding every commit, added
// minus deleted must then be the lines the first-party files hold at the end:
// two in main.go, one in notes.md, two in internal/tool/own.go. The vendor/
// file counts apart, as third-party lines: three added on the first day and
// three deleted on the second, so the all-files sum ends on the same five
// lines, and the cells carry those figures for the page's all-files curve
// while their label and heat level stay first-party.
//
// VALIDATES: deleted lines are measured in the same numstat pass as added
// lines, third-party lines in the same pass apart from them, and the committed
// population sums to the tree it ends on.
func TestCommittedActivityCountsWhatEachCommitDeleted(t *testing.T) {
	window, err := MeasureCommittedActivity(activityTestRepository(t), ActivityDaysDefault, activityTestToday)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	second := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	for _, check := range []struct {
		what      string
		got, want int
	}{
		{"lines added on the first day", window.Lines.daily[first], 5 + 4 + 2},
		{"lines deleted on the first day", window.Deleted.daily[first], 0},
		{"lines added on the second day", window.Lines.daily[second], 2},
		{"lines deleted on the second day", window.Deleted.daily[second], 3 + 3 + 2},
		{"lines added in the window", window.Lines.Total, 13},
		{"lines deleted in the window", window.Deleted.Total, 8},
		{"days with deleted lines", window.Deleted.ActiveDays, 1},
		{"commits in the window", window.Commits.Total, 2},
		{"lines the first-party files hold at the end", window.Lines.Total - window.Deleted.Total, 2 + 1 + 2},
		{"third-party lines added on the first day", window.ThirdPartyLines.daily[first], 3},
		{"third-party lines deleted on the first day", window.ThirdPartyDeleted.daily[first], 0},
		{"third-party lines added on the second day", window.ThirdPartyLines.daily[second], 0},
		{"third-party lines deleted on the second day", window.ThirdPartyDeleted.daily[second], 3},
		{"third-party lines added in the window", window.ThirdPartyLines.Total, 3},
		{"third-party lines deleted in the window", window.ThirdPartyDeleted.Total, 3},
		{"lines every text file holds at the end",
			window.Lines.Total + window.ThirdPartyLines.Total - window.Deleted.Total - window.ThirdPartyDeleted.Total, 2 + 1 + 2},
	} {
		if check.got != check.want {
			t.Errorf("%s: got %d, want %d", check.what, check.got, check.want)
		}
	}
	if !window.Deleted.PeakDay.Equal(second) {
		t.Errorf("the peak deletion day is %s, want %s", window.Deleted.PeakDay.Format(dayLayout), second.Format(dayLayout))
	}
	if !window.FromFirstCommit {
		t.Error("a window opening before the first commit does not say it holds every commit")
	}

	const deletedCell = `data-date="2026-08-12" data-date-label="Wed 12 Aug 2026" data-lines="2" data-lines-display="2"` +
		` data-lines-level="1" data-deleted="8" data-deleted-display="8"` +
		` data-third-party-added="0" data-third-party-deleted="3" data-commits="1"`
	if !strings.Contains(window.CellsWithDeleted, deletedCell) {
		t.Errorf("the page's grid carries no deleted lines for the second day; want\n%s", deletedCell)
	}
	const thirdPartyCell = `data-deleted="0" data-deleted-display="0"` +
		` data-third-party-added="3" data-third-party-deleted="0" data-commits="1"`
	if !strings.Contains(window.CellsWithDeleted, `data-date="2026-08-10"`) || !strings.Contains(window.CellsWithDeleted, thirdPartyCell) {
		t.Errorf("the page's grid carries no third-party lines for the first day; want\n%s", thirdPartyCell)
	}
	if !strings.Contains(window.CellsWithDeleted, `aria-label="Mon 10 Aug 2026: 11 lines added, 0 lines deleted, 1 commits"`) {
		t.Error("the first day's label to a screen reader counts the third-party lines, or names no first-party ones")
	}
	if strings.Contains(window.Cells, "third-party") {
		t.Error("the grid the slide and the dashboard draw carries third-party lines, so their output changed")
	}
	if !strings.Contains(window.CellsWithDeleted, `aria-label="Wed 12 Aug 2026: 2 lines added, 8 lines deleted, 1 commits"`) {
		t.Error("the page's grid does not name the second day's deleted lines to a screen reader")
	}
	if strings.Contains(window.Cells, "deleted") {
		t.Error("the grid the slide and the dashboard draw carries deleted lines, so their output changed")
	}
}

// TestSourceActivityKeepsItsPopulation proves the measurement the talk slide
// and the dashboard draw keeps its code files, its vendor globs, its rename
// pairing and its window, while it now also reads the deleted lines.
//
// Method: measure the same repository with MeasureActivity. Only main.go is a
// code file outside every vendor directory, so the window added five lines and
// deleted three, and the move is one rename of no changed lines. The window
// is the full year back from today, not a month start.
//
// VALIDATES: the slide's figures and window are unchanged by the committed
// population the page asks for.
func TestSourceActivityKeepsItsPopulation(t *testing.T) {
	window, err := MeasureActivity(activityTestRepository(t), ActivityDaysDefault, "", activityTestToday)
	if err != nil {
		t.Fatal(err)
	}
	if window.Lines.Total != 5 || window.Deleted.Total != 3 {
		t.Errorf("the source population added %d and deleted %d lines, want 5 and 3", window.Lines.Total, window.Deleted.Total)
	}
	if want := time.Date(2025, time.August, 16, 0, 0, 0, 0, time.UTC); !window.Start.Equal(want) {
		t.Errorf("the source window opens on %s, want %s", window.Start.Format(dayLayout), want.Format(dayLayout))
	}
}

// TestActivityWindowStartOpensOnAMonth proves the rule the published page's
// window opens by: the first month start on or after the day the window's
// length reaches back to, or the first day of the first commit's month when
// that is later. Without MonthStart the window keeps its length.
func TestActivityWindowStartOpensOnAMonth(t *testing.T) {
	day := func(year int, month time.Month, date int) time.Time {
		return time.Date(year, month, date, 0, 0, 0, 0, time.UTC)
	}
	firstCommit := day(2025, time.December, 19)
	oldCommit := day(2020, time.January, 5)
	for _, check := range []struct {
		what       string
		monthStart bool
		today      time.Time
		repoStart  *time.Time
		want       time.Time
	}{
		{"a year back, as the slide counts", false, day(2026, time.November, 12), &firstCommit, day(2025, time.November, 13)},
		{"the first commit's month is later", true, day(2026, time.November, 12), &firstCommit, day(2025, time.December, 1)},
		{"the history is older than the window", true, day(2026, time.November, 12), &oldCommit, day(2025, time.December, 1)},
		{"a year back lands on a month start", true, day(2026, time.October, 31), &oldCommit, day(2025, time.November, 1)},
		{"no commit at all", true, day(2026, time.November, 12), nil, day(2025, time.December, 1)},
	} {
		options := ActivityOptions{Days: ActivityDaysDefault, MonthStart: check.monthStart}
		if got := activityWindowStart(options, check.today, check.repoStart); !got.Equal(check.want) {
			t.Errorf("%s: the window opens on %s, want %s", check.what, got.Format(dayLayout), check.want.Format(dayLayout))
		}
	}
}

// TestMonthLabelsNameTheOpeningMonth proves the grid's month row for both
// openings. A window opening mid-month starts in a partial month whose next
// month begins three columns later, too close for both names, so the partial
// month gives way and the first full month is named. A window opening on the
// first of a month names that month over its first column.
func TestMonthLabelsNameTheOpeningMonth(t *testing.T) {
	today := time.Date(2026, time.November, 12, 0, 0, 0, 0, time.UTC)
	for _, check := range []struct {
		what  string
		start time.Time
		want  []string
	}{
		{"mid-month", time.Date(2025, time.November, 13, 0, 0, 0, 0, time.UTC),
			[]string{`style="grid-column:1"></span>`, `style="grid-column:4">Dec</span>`}},
		{"first of the month", time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC),
			[]string{`style="grid-column:1">Dec</span>`, `style="grid-column:5">Jan</span>`}},
	} {
		labels := monthLabels(weeksBetween(sundayBefore(check.start), saturdayAfter(today)))
		for _, want := range check.want {
			if !strings.Contains(labels, want) {
				t.Errorf("%s: the month row carries no %s\n%s", check.what, want, labels)
			}
		}
	}
}
