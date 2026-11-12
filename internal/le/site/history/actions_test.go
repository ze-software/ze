package sitehistory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// fixtureGit runs git in root with no user or system configuration, the
// extra environment given, and input on stdin, and answers its trimmed output.
func fixtureGit(t *testing.T, root, input string, extra []string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull), extra...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// fixtureRepository builds a two-day history and points le at it. Day one,
// 2026-01-05, holds main.go (3 lines: 2 code, 1 blank) and README.md (1 line).
// Day two, 2026-01-06, keeps main.go, adds main_test.go (2 lines: 1 code, 1
// comment) and grows README.md to 2 lines. It answers the root and the two
// commits.
func fixtureRepository(t *testing.T) (root, dayOne, dayTwo string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "", nil, "init", "-q", "--initial-branch=main")
	mainGo := fixtureGit(t, root, "package main\n\nfunc main() {}\n", nil, "hash-object", "-w", "--stdin")
	readmeOne := fixtureGit(t, root, "# Fixture\n", nil, "hash-object", "-w", "--stdin")
	testGo := fixtureGit(t, root, "package main\n// A test file.\n", nil, "hash-object", "-w", "--stdin")
	readmeTwo := fixtureGit(t, root, "# Fixture\nTwo.\n", nil, "hash-object", "-w", "--stdin")

	treeOne := fixtureGit(t, root, "100644 blob "+mainGo+"\tmain.go\n100644 blob "+readmeOne+"\tREADME.md\n", nil, "mktree")
	treeTwo := fixtureGit(t, root, "100644 blob "+mainGo+"\tmain.go\n100644 blob "+testGo+"\tmain_test.go\n100644 blob "+readmeTwo+"\tREADME.md\n", nil, "mktree")

	identity := []string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree"}
	dateOne := []string{"GIT_AUTHOR_DATE=Mon, 05 Jan 2026 10:00:00 +0000", "GIT_COMMITTER_DATE=Mon, 05 Jan 2026 10:00:00 +0000"}
	dateTwo := []string{"GIT_AUTHOR_DATE=Tue, 06 Jan 2026 09:00:00 +0000", "GIT_COMMITTER_DATE=Tue, 06 Jan 2026 09:00:00 +0000"}
	dayOne = fixtureGit(t, root, "day one\n", dateOne, append(identity, treeOne)...)
	dayTwo = fixtureGit(t, root, "day two\n", dateTwo, append(identity, treeTwo, "-p", dayOne)...)
	fixtureGit(t, root, "", nil, "update-ref", "refs/heads/main", dayTwo)

	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	return root, dayOne, dayTwo
}

// fixtureRepositoryDays is what the fixture repository measures, counted by
// hand. Every section not named held nothing.
func fixtureRepositoryDays(t *testing.T, dayOne, dayTwo string) []DayRecord {
	t.Helper()
	return []DayRecord{
		{Date: mustDate(t, "2026-01-05"), Commit: dayOne, Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 3, Gained: 3, Go: GoLines{Code: 2, Blank: 1}},
			"documentation": {Files: 1, Lines: 1, Gained: 1},
		}},
		{Date: mustDate(t, "2026-01-06"), Commit: dayTwo, Sections: map[string]SectionValues{
			"go-production": {Files: 1, Lines: 3, Go: GoLines{Code: 2, Blank: 1}},
			"go-tests":      {Files: 1, Lines: 2, Gained: 2, Go: GoLines{Code: 1, Comment: 1}},
			"documentation": {Files: 1, Lines: 2, Gained: 1},
		}},
	}
}

// TestSiteHistoryUpdateActionWritesTheFile asks whether `le site history
// update all`, through the registered action table, writes the measured
// history. Method: run it against a two-day fixture repository and compare
// every section of every day with the hand-counted values, a section the day
// does not name counting as zero.
func TestSiteHistoryUpdateActionWritesTheFile(t *testing.T) {
	root, dayOne, dayTwo := fixtureRepository(t)
	if _, code := Answer([]string{verbUpdate, keywordAll}); code != 0 {
		t.Fatalf("le site history update all answered %d, want 0", code)
	}
	history, err := ReadHistory(root)
	if err != nil {
		t.Fatalf("the update wrote no readable %s: %v", HistoryFile, err)
	}
	if history.Rules() != RulesIdentifier() {
		t.Errorf("the file records rules %s, the tool's are %s", history.Rules(), RulesIdentifier())
	}
	want := fixtureRepositoryDays(t, dayOne, dayTwo)
	got := history.Days()
	if len(got) != len(want) {
		t.Fatalf("the file records %d days, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].Date.Compare(want[index].Date) != 0 {
			t.Errorf("day %d is %s, want %s", index+1, got[index].Date, want[index].Date)
		}
		if got[index].Commit != want[index].Commit {
			t.Errorf("day %s records commit %s, want %s", want[index].Date, got[index].Commit, want[index].Commit)
		}
		if got[index].Modules != 0 {
			t.Errorf("day %s records %d modules; the fixture has no vendor/modules.txt", want[index].Date, got[index].Modules)
		}
		for _, section := range Sections() {
			if got[index].Sections[section.ID] != want[index].Sections[section.ID] {
				t.Errorf("day %s section %s holds %+v, want %+v", want[index].Date, section.ID,
					got[index].Sections[section.ID], want[index].Sections[section.ID])
			}
		}
	}
}

// TestSiteHistoryCheckActionJudgesTheFile asks whether `le site history
// check`, through the registered action table, passes a file that describes
// the history and refuses one whose newest day was altered. Method: write the
// hand-counted history of a fixture repository, check it, alter the newest
// day's documentation lines and gains together so the arithmetic still holds,
// and check again. Each row of AC-9 has its own test in check_test.go; this one
// proves the registered verb reaches the check and answers its exit codes.
func TestSiteHistoryCheckActionJudgesTheFile(t *testing.T) {
	root, dayOne, dayTwo := fixtureRepository(t)
	days := fixtureRepositoryDays(t, dayOne, dayTwo)
	history, err := NewHistory(RulesIdentifier(), Sections(), days)
	if err != nil {
		t.Fatalf("NewHistory refused the hand-counted fixture: %v", err)
	}
	if _, err := WriteHistory(root, history); err != nil {
		t.Fatal(err)
	}
	if _, code := Answer([]string{verbCheck}); code != 0 {
		t.Fatalf("le site history check answered %d on a file that describes the history, want 0", code)
	}

	days[1].Sections["documentation"] = SectionValues{Files: 1, Lines: 3, Gained: 2}
	altered, err := NewHistory(RulesIdentifier(), Sections(), days)
	if err != nil {
		t.Fatalf("NewHistory refused the altered fixture: %v", err)
	}
	if _, err := WriteHistory(root, altered); err != nil {
		t.Fatal(err)
	}
	if _, code := Answer([]string{verbCheck}); code != 1 {
		t.Errorf("le site history check answered %d on a newest day whose documentation lines were altered, want 1", code)
	}
}

// TestSiteHistoryUpdateRefusesAMalformedDay asks what a day not written
// YYYY-MM-DD answers. Method: the registered table answers the parse refusal's
// code 2, and the refusal names the day and ends on the form to type.
func TestSiteHistoryUpdateRefusesAMalformedDay(t *testing.T) {
	for _, raw := range []string{"2026-13-01", "2026-02-30", "2026-1-05", "2026-01-05T00:00:00Z", "yesterday"} {
		if _, code := Answer([]string{verbUpdate, keywordFrom, raw}); code != 2 {
			t.Errorf("le site history update from %s answered %d, want 2", raw, code)
		}
		_, err := parseUpdate(leaction.Arguments{keywordTo: {raw}})
		if err == nil {
			t.Fatalf("parseUpdate accepted to %s", raw)
		}
		if !strings.Contains(err.Error(), "the to day "+raw+" cannot be read") {
			t.Errorf("the refusal does not name to %s:\n%v", raw, err)
		}
		if !strings.HasSuffix(err.Error(), "fix: ./le site history update to YYYY-MM-DD") {
			t.Errorf("the refusal does not end on the form to type:\n%v", err)
		}
	}
}

// TestSiteHistoryUpdateRefusesFromAfterTo asks what an inverted range
// answers. Method: from one day after to answers 2 and names the range the
// other way round as its repair.
func TestSiteHistoryUpdateRefusesFromAfterTo(t *testing.T) {
	if _, code := Answer([]string{verbUpdate, keywordFrom, "2026-01-06", keywordTo, "2026-01-05"}); code != 2 {
		t.Errorf("le site history update from 2026-01-06 to 2026-01-05 answered %d, want 2", code)
	}
	_, err := parseUpdate(leaction.Arguments{keywordFrom: {"2026-01-06"}, keywordTo: {"2026-01-05"}})
	if err == nil {
		t.Fatal("parseUpdate accepted from 2026-01-06 to 2026-01-05")
	}
	if !strings.HasSuffix(err.Error(), "fix: ./le site history update from 2026-01-05 to 2026-01-06") {
		t.Errorf("the refusal does not end on the range the right way round:\n%v", err)
	}
}

// TestSiteHistoryUpdateRefusesAllWithABound asks what all beside a range
// bound answers. Method: all with from and all with to each answer 2, and the
// refusal names the whole-history run alone.
func TestSiteHistoryUpdateRefusesAllWithABound(t *testing.T) {
	for _, keyword := range []string{keywordFrom, keywordTo} {
		if _, code := Answer([]string{verbUpdate, keywordAll, keyword, "2026-01-05"}); code != 2 {
			t.Errorf("le site history update all %s 2026-01-05 answered %d, want 2", keyword, code)
		}
		_, err := parseUpdate(leaction.Arguments{keywordAll: {""}, keyword: {"2026-01-05"}})
		if err == nil {
			t.Fatalf("parseUpdate accepted all with %s", keyword)
		}
		if !strings.HasSuffix(err.Error(), "fix: ./le site history update all") {
			t.Errorf("the refusal does not end on the whole-history run:\n%v", err)
		}
	}
}

// TestSiteHistoryUpdateReadsItsRange asks what each accepted line asks for.
// Method: no keyword extends, all is the whole history, from equal to to is a
// one-day range, and a lone bound leaves the other end open.
func TestSiteHistoryUpdateReadsItsRange(t *testing.T) {
	if request, err := parseUpdate(leaction.Arguments{}); err != nil {
		t.Errorf("parseUpdate refused no keyword: %v", err)
	} else if _, extends := request.(extendRequest); !extends {
		t.Errorf("no keyword asks for %T, want an extension", request)
	}
	if request, err := parseUpdate(leaction.Arguments{keywordAll: {""}}); err != nil {
		t.Errorf("parseUpdate refused all: %v", err)
	} else if _, whole := request.(wholeRequest); !whole {
		t.Errorf("all asks for %T, want the whole history", request)
	}
	for _, row := range []struct {
		line leaction.Arguments
		span string
	}{
		{leaction.Arguments{keywordFrom: {"2026-01-05"}, keywordTo: {"2026-01-05"}}, "the days from 2026-01-05 to 2026-01-05"},
		{leaction.Arguments{keywordFrom: {"2026-01-05"}}, "the days from 2026-01-05 to the newest day of the history"},
		{leaction.Arguments{keywordTo: {"2026-01-05"}}, "the days from the first day of the history to 2026-01-05"},
	} {
		request, err := parseUpdate(row.line)
		if err != nil {
			t.Errorf("parseUpdate refused %v: %v", row.line, err)
			continue
		}
		if _, ranged := request.(rangeRequest); !ranged {
			t.Errorf("%v asks for %T, want a range", row.line, request)
		}
		if got := request.span(); got != row.span {
			t.Errorf("%v spans %q, want %q", row.line, got, row.span)
		}
	}
}
