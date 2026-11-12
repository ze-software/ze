package sitehistory

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// threeDayCommits is the three-day history the update tests share, counted by
// hand in threeDayRecords. Day one, 2026-03-02: main.go (1 code line) and
// README.md (1). Day two: README.md grows to 2 and plan/p.md (1) arrives. Day
// three: main.go grows to 3 lines (2 code, 1 blank) and plan/p.md leaves.
var threeDayCommits = []historyCommit{
	{when: "2026-03-02T10:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\n"}},
	{when: "2026-03-03T10:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\nb\n", "plan/p.md": "p\n"}},
	{when: "2026-03-04T10:00:00Z", files: map[string]string{"main.go": "package main\n\nfunc main() {}\n", "README.md": "a\nb\n"}},
}

// threeDayRecords is what the three-day history measures, counted by hand.
func threeDayRecords(t *testing.T, commits []string) []DayRecord {
	t.Helper()
	return []DayRecord{
		{Date: mustDate(t, "2026-03-02"), Commit: commits[0], Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 1, Gained: 1, Go: GoLines{Code: 1}},
			"documentation": {Files: 1, Lines: 1, Gained: 1},
		}},
		{Date: mustDate(t, "2026-03-03"), Commit: commits[1], Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 1, Go: GoLines{Code: 1}},
			"documentation": {Files: 1, Lines: 2, Gained: 1},
			"plans":         {Files: 1, Lines: 1, Gained: 1},
		}},
		{Date: mustDate(t, "2026-03-04"), Commit: commits[2], Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 3, Gained: 2, Go: GoLines{Code: 2, Blank: 1}},
			"documentation": {Files: 1, Lines: 2},
			"plans":         {Lost: 1},
		}},
	}
}

// historyBytes answers the history file's bytes under root, or nil when the
// file does not exist.
func historyBytes(t *testing.T, root string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(HistoryFile)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// mustUpdate runs one update and fails the test on a refusal.
func mustUpdate(t *testing.T, root string, request updateRequest) updated {
	t.Helper()
	answer, err := updateHistory(t.Context(), root, request)
	if err != nil {
		t.Fatalf("the update of %s refused:\n%v", request.span(), err)
	}
	return answer
}

// dateRef answers a pointer to the day raw names, for a range bound.
func dateRef(t *testing.T, raw string) *Date {
	t.Helper()
	day := mustDate(t, raw)
	return &day
}

// assertMeasured checks the days an answer says it measured.
func assertMeasured(t *testing.T, answer updated, days int, first, last string) {
	t.Helper()
	if answer.DaysMeasured != days {
		t.Errorf("the update measured %d days, want %d", answer.DaysMeasured, days)
	}
	if answer.FirstDay.String() != first {
		t.Errorf("the update measured from %s, want %s", answer.FirstDay, first)
	}
	if answer.LastDay.String() != last {
		t.Errorf("the update measured to %s, want %s", answer.LastDay, last)
	}
}

// TestUpdateWholeHistoryWritesTheSameBytesTwice asks whether a whole-history
// run writes the hand-counted days, one day per line, and the same bytes on a
// second run (AC-5, AC-6). Method: run it twice over the three-day history and
// compare the records, the answer and the bytes.
func TestUpdateWholeHistoryWritesTheSameBytesTwice(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)

	answer := mustUpdate(t, root, wholeRequest{})
	assertMeasured(t, answer, 3, "2026-03-02", "2026-03-04")
	// The newest day holds main.go's 3 lines and README.md's 2.
	if answer.TotalLines != 5 {
		t.Errorf("the answer totals %d lines on the newest day, want 5", answer.TotalLines)
	}
	history, err := ReadHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	assertDays(t, history.Days(), threeDayRecords(t, commits))

	first := historyBytes(t, root)
	if lines := strings.Count(string(first), "\n    {\"date\":"); lines != 3 {
		t.Errorf("the file holds %d day lines, want 3, one per day:\n%s", lines, first)
	}
	mustUpdate(t, root, wholeRequest{})
	if second := historyBytes(t, root); !bytes.Equal(first, second) {
		t.Errorf("the second run wrote other bytes:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestUpdateRangeReplacesOnlyNamedDays asks whether a range measures only its
// days, keeps the others byte for byte, and counts its first day against the
// recorded day before it (AC-6, R-1). Method: after a whole-history run, a
// one-day range, an open end on each side; each answer names its days, and
// the file stays the bytes of the whole run, which a first day counted against
// an empty tree would break (day two would gain main.go's line).
func TestUpdateRangeReplacesOnlyNamedDays(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)
	mustUpdate(t, root, wholeRequest{})
	whole := historyBytes(t, root)

	cases := []struct {
		name        string
		request     rangeRequest
		days        int
		first, last string
	}{
		{"from and to one day", rangeRequest{from: dateRef(t, "2026-03-03"), to: dateRef(t, "2026-03-03")}, 1, "2026-03-03", "2026-03-03"},
		{"from with an open end", rangeRequest{from: dateRef(t, "2026-03-03")}, 2, "2026-03-03", "2026-03-04"},
		{"to with an open start", rangeRequest{to: dateRef(t, "2026-03-03")}, 2, "2026-03-02", "2026-03-03"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertMeasured(t, mustUpdate(t, root, tc.request), tc.days, tc.first, tc.last)
			if after := historyBytes(t, root); !bytes.Equal(after, whole) {
				t.Errorf("the range changed the file:\nwant:\n%s\ngot:\n%s", whole, after)
			}
			history, err := ReadHistory(root)
			if err != nil {
				t.Fatal(err)
			}
			assertDays(t, history.Days(), threeDayRecords(t, commits))
		})
	}
}

// TestUpdateDefaultExtendsFromTheNewestRecordedDay asks whether the run with
// no keyword measures the whole history when there is no file, then
// re-measures the newest recorded day at its current last commit and appends
// the days after it (AC-6). Method: two days recorded; a later commit the
// same second day grows README.md to 3 lines, and a third day adds
// test/x.ci (1 line); the second run measures days two and three, and day
// one keeps its line.
func TestUpdateDefaultExtendsFromTheNewestRecordedDay(t *testing.T) {
	root := newFixtureRoot(t)
	early := buildHistory(t, root, "", []historyCommit{
		{when: "2026-03-02T10:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\n"}},
		{when: "2026-03-03T09:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\nb\n"}},
	})
	assertMeasured(t, mustUpdate(t, root, extendRequest{}), 2, "2026-03-02", "2026-03-03")
	dayOneLine := historyLine(t, historyBytes(t, root), "2026-03-02")

	late := buildHistory(t, root, early[1], []historyCommit{
		{when: "2026-03-03T18:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\nb\nc\n"}},
		{when: "2026-03-04T10:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\nb\nc\n", "test/x.ci": "t\n"}},
	})
	assertMeasured(t, mustUpdate(t, root, extendRequest{}), 2, "2026-03-03", "2026-03-04")

	history, err := ReadHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	mainGo := SectionValues{Files: 1, Lines: 1, Go: GoLines{Code: 1}}
	assertDays(t, history.Days(), []DayRecord{
		{Date: mustDate(t, "2026-03-02"), Commit: early[0], Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 1, Gained: 1, Go: GoLines{Code: 1}},
			"documentation": {Files: 1, Lines: 1, Gained: 1},
		}},
		{Date: mustDate(t, "2026-03-03"), Commit: late[0], Sections: map[string]SectionValues{
			"go-production": mainGo,
			"documentation": {Files: 1, Lines: 3, Gained: 2},
		}},
		{Date: mustDate(t, "2026-03-04"), Commit: late[1], Sections: map[string]SectionValues{
			"go-production":    mainGo,
			"documentation":    {Files: 1, Lines: 3},
			"functional-tests": {Files: 1, Lines: 1, Gained: 1},
		}},
	})
	if got := historyLine(t, historyBytes(t, root), "2026-03-02"); got != dayOneLine {
		t.Errorf("the kept day changed:\nbefore: %s\nafter:  %s", dayOneLine, got)
	}
}

// historyLine answers the one line of content that records day.
func historyLine(t *testing.T, content []byte, day string) string {
	t.Helper()
	for line := range strings.SplitSeq(string(content), "\n") {
		if strings.Contains(line, "\"date\":\""+day+"\"") {
			return line
		}
	}
	t.Fatalf("the file records no line for %s:\n%s", day, content)
	return ""
}

// TestUpdateRefusesWhatItCannotMerge asks what an update that cannot describe
// the history answers (AC-8, R-2, R-6, R-7, R-15). Method: one three-day
// history, each row preparing the file it needs in turn; every refusal names
// what is wrong, ends on the command that repairs it, and leaves the file's
// bytes, or its absence, as they were.
func TestUpdateRefusesWhatItCannotMerge(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)
	path := filepath.Join(root, filepath.FromSlash(HistoryFile))

	writeWhole := func(t *testing.T) {
		t.Helper()
		mustUpdate(t, root, wholeRequest{})
	}
	rows := []struct {
		name    string
		prepare func(t *testing.T)
		request updateRequest
		names   string
		fix     string
	}{
		{"a range with no file", func(t *testing.T) {
			t.Helper()
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
		}, rangeRequest{from: dateRef(t, "2026-03-03")}, "does not exist", repairWhole},
		{"a file under other rules", func(t *testing.T) {
			t.Helper()
			writeWhole(t)
			content := bytes.ReplaceAll(historyBytes(t, root), []byte(RulesIdentifier()), []byte(strings.Repeat("0", 64)))
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
		}, extendRequest{}, "measured under rules " + strings.Repeat("0", 64), repairWhole},
		{"a range leaving a gap", func(t *testing.T) {
			t.Helper()
			writeWhole(t)
			history, err := ReadHistory(root)
			if err != nil {
				t.Fatal(err)
			}
			first, err := NewHistory(history.Rules(), history.Sections(), history.Days()[:1])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := WriteHistory(root, first); err != nil {
				t.Fatal(err)
			}
		}, rangeRequest{from: dateRef(t, "2026-03-04")}, "would leave 2026-03-03 unrecorded", repairExtend},
		{"a range after the history", writeWhole, rangeRequest{from: dateRef(t, "2027-01-01")}, "hold no day", repairExtend},
		{"a range before the history", writeWhole, rangeRequest{to: dateRef(t, "2025-01-01")}, "hold no day", repairExtend},
		{"a rewritten history", func(t *testing.T) {
			t.Helper()
			writeWhole(t)
			rewritten := make([]historyCommit, len(threeDayCommits))
			for index, commit := range threeDayCommits {
				rewritten[index] = historyCommit{when: commit.when, files: map[string]string{"main.go": "package rewritten\n"}}
			}
			buildHistory(t, root, "", rewritten)
		}, extendRequest{}, commits[0] + ", which is not on HEAD's first-parent history", repairWhole},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			row.prepare(t)
			before := historyBytes(t, root)
			_, err := updateHistory(t.Context(), root, row.request)
			if err == nil {
				t.Fatal("the update accepted what it cannot merge")
			}
			if !strings.Contains(err.Error(), row.names) {
				t.Errorf("the refusal does not say %q:\n%v", row.names, err)
			}
			if !strings.HasSuffix(err.Error(), "fix: "+row.fix) {
				t.Errorf("the refusal does not end on %q:\n%v", row.fix, err)
			}
			if after := historyBytes(t, root); !bytes.Equal(after, before) {
				t.Errorf("the refused update changed %s:\nbefore:\n%s\nafter:\n%s", HistoryFile, before, after)
			}
		})
	}
}
