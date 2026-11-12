package sitehistory

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/repo/objects"
)

// instant reads an RFC 3339 time for a fixture commit.
func instant(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("fixture time %q: %v", raw, err)
	}
	return parsed
}

// fixtureCommit builds one first-parent commit whose tree ID is its own ID
// with the first letter replaced, so a test can tell the two apart.
func fixtureCommit(t *testing.T, id, authored, committed string) objects.Commit {
	t.Helper()
	return objects.Commit{ID: id, Tree: "t" + id[1:], AuthorTime: instant(t, authored), CommitTime: instant(t, committed)}
}

// expectedDay is one recorded day as a test writes it by hand.
type expectedDay struct {
	date   string
	commit string
}

// TestDaySelectionFollowsEffectiveAuthorDay asks which commit each recorded
// day holds. Method: first-parent histories, oldest first, each with the days
// and commits written by hand: authors out of order, equal timestamps, the
// last second of a UTC day and the first of the next, an author in another
// timezone, committer dates all rewritten to one late day, and an idle day.
func TestDaySelectionFollowsEffectiveAuthorDay(t *testing.T) {
	const rewritten = "2026-03-27T12:00:00Z"
	for _, row := range []struct {
		name string
		line [][3]string
		want []expectedDay
	}{
		{
			name: "an author day earlier than its parent's is raised to the parent's",
			line: [][3]string{
				{"c1", "2026-01-03T10:00:00Z", rewritten},
				{"c2", "2026-01-02T12:00:00Z", rewritten},
				{"c3", "2026-01-04T08:00:00Z", rewritten},
			},
			want: []expectedDay{{"2026-01-03", "c2"}, {"2026-01-04", "c3"}},
		},
		{
			name: "of two commits with equal timestamps the later in first-parent order wins",
			line: [][3]string{
				{"c1", "2026-01-05T10:00:00Z", "2026-01-05T10:00:00Z"},
				{"c2", "2026-01-05T10:00:00Z", "2026-01-05T10:00:00Z"},
			},
			want: []expectedDay{{"2026-01-05", "c2"}},
		},
		{
			name: "the last commit of a day wins whatever its timestamp",
			line: [][3]string{
				{"c1", "2026-01-05T18:00:00Z", "2026-01-05T18:00:00Z"},
				{"c2", "2026-01-05T09:00:00Z", "2026-01-05T19:00:00Z"},
			},
			want: []expectedDay{{"2026-01-05", "c2"}},
		},
		{
			name: "23:59:59 UTC is that day and 00:00:00 UTC the next",
			line: [][3]string{
				{"c1", "2026-01-05T23:59:59Z", "2026-01-05T23:59:59Z"},
				{"c2", "2026-01-06T00:00:00Z", "2026-01-06T00:00:00Z"},
			},
			want: []expectedDay{{"2026-01-05", "c1"}, {"2026-01-06", "c2"}},
		},
		{
			name: "an author in another timezone lands on the UTC day",
			line: [][3]string{
				{"c1", "2026-01-06T01:30:00+02:00", "2026-01-06T01:30:00+02:00"},
				{"c2", "2026-01-05T22:30:00-03:00", "2026-01-05T22:30:00-03:00"},
			},
			want: []expectedDay{{"2026-01-05", "c1"}, {"2026-01-06", "c2"}},
		},
		{
			name: "committer dates rewritten to one late day decide nothing",
			line: [][3]string{
				{"c1", "2026-01-01T10:00:00Z", rewritten},
				{"c2", "2026-01-02T10:00:00Z", rewritten},
			},
			want: []expectedDay{{"2026-01-01", "c1"}, {"2026-01-02", "c2"}},
		},
		{
			name: "a day with no commit is absent",
			line: [][3]string{
				{"c1", "2026-01-01T10:00:00Z", "2026-01-01T10:00:00Z"},
				{"c2", "2026-01-03T10:00:00Z", "2026-01-03T10:00:00Z"},
			},
			want: []expectedDay{{"2026-01-01", "c1"}, {"2026-01-03", "c2"}},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			commits := make([]objects.Commit, 0, len(row.line))
			for _, fields := range row.line {
				commits = append(commits, fixtureCommit(t, fields[0], fields[1], fields[2]))
			}
			days, err := SelectDays(commits)
			if err != nil {
				t.Fatalf("SelectDays: %v", err)
			}
			if len(days) != len(row.want) {
				t.Fatalf("SelectDays answered %d days, want %d: %+v", len(days), len(row.want), days)
			}
			for index, want := range row.want {
				got := days[index]
				if got.Date.String() != want.date {
					t.Errorf("day %d is %s, want %s", index+1, got.Date, want.date)
				}
				if got.Commit != want.commit {
					t.Errorf("day %s records commit %s, want %s", want.date, got.Commit, want.commit)
				}
				if got.Tree != "t"+want.commit[1:] {
					t.Errorf("day %s records tree %s, want the tree of %s", want.date, got.Tree, want.commit)
				}
			}
		})
	}
}

// TestDaySelectionRefusesACommitAuthoredAfterItWasCommitted asks when a
// commit's dates are refused. Method: an author day one day after the
// committer day refuses and names the commit and both days; an author instant
// after the committer instant on the same UTC day passes.
func TestDaySelectionRefusesACommitAuthoredAfterItWasCommitted(t *testing.T) {
	future := []objects.Commit{
		fixtureCommit(t, "c1", "2026-01-04T10:00:00Z", "2026-01-04T10:00:00Z"),
		fixtureCommit(t, "c2", "2026-01-06T00:30:00Z", "2026-01-05T23:00:00Z"),
	}
	_, err := SelectDays(future)
	if err == nil {
		t.Fatal("SelectDays accepted a commit authored on 2026-01-06 and committed on 2026-01-05")
	}
	for _, named := range []string{"c2", "2026-01-06", "2026-01-05", "c1"} {
		if !strings.Contains(err.Error(), named) {
			t.Errorf("the refusal does not name %s: %v", named, err)
		}
	}

	sameDay := []objects.Commit{fixtureCommit(t, "c1", "2026-01-05T23:00:00Z", "2026-01-05T22:00:00Z")}
	days, err := SelectDays(sameDay)
	if err != nil {
		t.Fatalf("SelectDays refused an author and committer on the same UTC day: %v", err)
	}
	if len(days) != 1 || days[0].Date.String() != "2026-01-05" {
		t.Errorf("SelectDays answered %+v, want one day 2026-01-05", days)
	}
}

// TestDaySelectionRefusesAnEmptyHistory asks what an empty history answers.
// Method: no commit at all must be an error, never an empty list of days.
func TestDaySelectionRefusesAnEmptyHistory(t *testing.T) {
	days, err := SelectDays(nil)
	if err == nil {
		t.Fatalf("SelectDays(nil) answered %d days and no error", len(days))
	}
}
