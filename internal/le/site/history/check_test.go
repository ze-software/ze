package sitehistory

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHistoryBytes replaces the history file under root with content.
func writeHistoryBytes(t *testing.T, root string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(HistoryFile)), content, 0o600); err != nil {
		t.Fatal(err)
	}
}

// replaceInHistory rewrites the history file under root with the one
// occurrence of old replaced by replacement, and fails when old is not there
// exactly once, so a fixture that stopped matching cannot pass in silence.
func replaceInHistory(t *testing.T, root, old, replacement string) {
	t.Helper()
	content := historyBytes(t, root)
	if count := bytes.Count(content, []byte(old)); count != 1 {
		t.Fatalf("the history file holds %q %d times, want once:\n%s", old, count, content)
	}
	writeHistoryBytes(t, root, bytes.Replace(content, []byte(old), []byte(replacement), 1))
}

// writeHistoryDays replaces the history file under root with days, under the
// tool's rules and section table.
func writeHistoryDays(t *testing.T, root string, days []DayRecord) {
	t.Helper()
	history, err := NewHistory(RulesIdentifier(), Sections(), days)
	if err != nil {
		t.Fatalf("NewHistory refused the fixture: %v", err)
	}
	if _, err := WriteHistory(root, history); err != nil {
		t.Fatal(err)
	}
}

// assertRefusal runs the check over root and asks whether it refused as a file
// that does not describe the history (code 1), said every text of names, and
// ended on the whole-history run, the repair every such refusal names: the
// check never fails for lag, the only state the default run repairs.
func assertRefusal(t *testing.T, root string, names []string) {
	t.Helper()
	_, err := checkHistory(t.Context(), root)
	if err == nil {
		t.Fatal("the check passed a file that does not describe the history")
	}
	if code := refusalCode(err); code != 1 {
		t.Errorf("the refusal answers code %d, want 1:\n%v", code, err)
	}
	for _, name := range names {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal does not say %q:\n%v", name, err)
		}
	}
	if !strings.HasSuffix(err.Error(), "fix: "+repairWhole) {
		t.Errorf("the refusal does not end on %q:\n%v", repairWhole, err)
	}
}

// assertChecked runs the check over root through check, the verb's body, and
// asks whether it answered 0 with the newest day, commit, day count and lag
// given.
func assertChecked(t *testing.T, root string, want checked) {
	t.Helper()
	answer, code := check(root)
	if code != 0 {
		t.Fatalf("the check answered %d on a file that describes the history, want 0", code)
	}
	got, isChecked := answer.(checked)
	if !isChecked {
		t.Fatalf("the check answered %T, want checked", answer)
	}
	if got.NewestDay.Compare(want.NewestDay) != 0 {
		t.Errorf("the check answered newest day %s, want %s", got.NewestDay, want.NewestDay)
	}
	// The days are compared above; Compare is their equality.
	got.NewestDay, want.NewestDay = Date{}, Date{}
	if got != want {
		t.Errorf("the check answered %+v, want %+v", got, want)
	}
}

// TestCheckPassesAFileThatDescribesTheHistory asks whether a file the update
// wrote, with no commit after it, passes with no lag. Method: the three-day
// history, a whole-history update, then the check.
func TestCheckPassesAFileThatDescribesTheHistory(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)
	mustUpdate(t, root, wholeRequest{})
	assertChecked(t, root, checked{File: HistoryFile, NewestDay: mustDate(t, "2026-03-04"), Commit: commits[2], Days: 3})
}

// TestCheckAcceptsLaterCommitsOnTheNewestDay asks whether the commit that
// lands the file, and every commit after it, leave the check green and count
// as lag (R-13, AC-9). Method: update a two-day history, then commit the
// written file itself later the same UTC day, which changes that day's tree,
// and one more commit on the next day. The check measures the newest day at
// the commit the file records, so it answers 0 with two commits and one day
// of lag; measuring the day's current last commit would find the file in the
// website section and refuse.
func TestCheckAcceptsLaterCommitsOnTheNewestDay(t *testing.T) {
	root := newFixtureRoot(t)
	early := []historyCommit{
		{when: "2026-03-02T10:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\n"}},
		{when: "2026-03-03T09:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\nb\n"}},
	}
	commits := buildHistory(t, root, "", early)
	mustUpdate(t, root, wholeRequest{})

	landed := map[string]string{"main.go": "package main\n", "README.md": "a\nb\n", HistoryFile: string(historyBytes(t, root))}
	later := map[string]string{"main.go": "package main\n", "README.md": "a\nb\n", HistoryFile: landed[HistoryFile], "test/x.ci": "t\n"}
	buildHistory(t, root, commits[1], []historyCommit{
		{when: "2026-03-03T18:00:00Z", files: landed},
		{when: "2026-03-04T10:00:00Z", files: later},
	})
	assertChecked(t, root, checked{File: HistoryFile, NewestDay: mustDate(t, "2026-03-03"), Commit: commits[1], Days: 2, LagCommits: 2, LagDays: 1})
}

// TestCheckRefusesAnAlteredNewestDay asks whether a figure of the newest day
// that no tree had is found, with the day, the section and both values
// (AC-9). Method: the three-day history, then the newest day's documentation
// lines raised from 2 to 3 with its gained raised from 0 to 1, so the line
// arithmetic still holds and only the re-measurement can see it.
func TestCheckRefusesAnAlteredNewestDay(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)
	days := threeDayRecords(t, commits)
	days[2].Sections["documentation"] = SectionValues{Files: 1, Lines: 3, Gained: 1}
	writeHistoryDays(t, root, days)
	assertRefusal(t, root, []string{"recorded day 2026-03-04 section documentation records lines 3", commits[2] + " measures lines 2"})
}

// TestCheckRefusesBrokenArithmeticOnAnOlderDay asks whether a size altered on
// an older day is refused, though the check measures only the newest day
// (AC-9). Method: the three-day history, then the first day's documentation
// lines raised from 1 to 4 in the file's bytes; the reader refuses the second
// day, whose 2 lines no longer equal 4 plus 1 gained.
func TestCheckRefusesBrokenArithmeticOnAnOlderDay(t *testing.T) {
	root := newFixtureRoot(t)
	buildHistory(t, root, "", threeDayCommits)
	mustUpdate(t, root, wholeRequest{})
	replaceInHistory(t, root, `"documentation":{"files":1,"lines":1,`, `"documentation":{"files":1,"lines":4,`)
	assertRefusal(t, root, []string{"day 2026-03-03 section documentation holds 2 lines", "its 4 lines on 2026-03-02"})
}

// TestCheckRefusesStaleRules asks whether a file measured under other rules is
// refused with both identifiers (AC-9, R-7). Method: the three-day history,
// then the file's rules identifier replaced by 64 zeros.
func TestCheckRefusesStaleRules(t *testing.T) {
	root := newFixtureRoot(t)
	buildHistory(t, root, "", threeDayCommits)
	mustUpdate(t, root, wholeRequest{})
	stale := strings.Repeat("0", 64)
	replaceInHistory(t, root, RulesIdentifier(), stale)
	assertRefusal(t, root, []string{"measured under rules " + stale, "counts under rules " + RulesIdentifier()})
}

// TestCheckRefusesAMissingMiddleDay asks whether a file that skips a history
// day between its first and newest days is refused, naming that day (AC-9,
// R-15). Method: the three-day history recorded without its second day, the
// third day counted by hand against the first so the arithmetic holds: main.go
// grows from 1 line to 3 (2 code, 1 blank), README.md from 1 to 2.
func TestCheckRefusesAMissingMiddleDay(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)
	writeHistoryDays(t, root, []DayRecord{
		threeDayRecords(t, commits)[0],
		{Date: mustDate(t, "2026-03-04"), Commit: commits[2], Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 3, Gained: 2, Go: GoLines{Code: 2, Blank: 1}},
			"documentation": {Files: 1, Lines: 2, Gained: 1},
		}},
	})
	assertRefusal(t, root, []string{"does not record 2026-03-03", "before its newest recorded day 2026-03-04"})
}

// TestCheckRefusesAnOlderDayBeforeItsLastCommit asks whether a recorded day
// older than the newest that names an earlier commit of its day is refused,
// with both commits (Check Semantics). Method: a first day of two commits
// with the same tree and a second day; the update records the first day's
// last commit, which the file's bytes then replace with the earlier one. The
// trees are equal, so only the commit rule can refuse it.
func TestCheckRefusesAnOlderDayBeforeItsLastCommit(t *testing.T) {
	root := newFixtureRoot(t)
	files := map[string]string{"main.go": "package main\n"}
	commits := buildHistory(t, root, "", []historyCommit{
		{when: "2026-03-02T09:00:00Z", files: files},
		{when: "2026-03-02T18:00:00Z", files: files},
		{when: "2026-03-03T10:00:00Z", files: map[string]string{"main.go": "package main\n", "README.md": "a\n"}},
	})
	mustUpdate(t, root, wholeRequest{})
	replaceInHistory(t, root, commits[1], commits[0])
	assertRefusal(t, root, []string{"recorded day 2026-03-02 names commit " + commits[0], "the last commit of that day is " + commits[1]})
}

// TestCheckRefusesACommitNotOnTheHistory asks whether a recorded commit that
// HEAD's first-parent history does not hold is refused by name (AC-9, R-6).
// Method: the three-day history, then the first day's commit in the file
// replaced by a well-formed ID no object has, as a rewritten history leaves it.
func TestCheckRefusesACommitNotOnTheHistory(t *testing.T) {
	root := newFixtureRoot(t)
	commits := buildHistory(t, root, "", threeDayCommits)
	mustUpdate(t, root, wholeRequest{})
	gone := "0123456789abcdef0123456789abcdef01234567"
	replaceInHistory(t, root, commits[0], gone)
	assertRefusal(t, root, []string{"recorded day 2026-03-02 names commit " + gone + ", which is not on HEAD's first-parent history"})
}
