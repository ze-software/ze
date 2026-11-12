// Design: docs/architecture/site-facts.md -- the committed-facts pattern, its second instance
// Related: sections.go -- the section of every path a recorded day's tree holds
// Related: file.go -- the file records the days chosen here
//
// Day selection, step 2 of the Transformation Path of
// plan/post-release/engineering/spec-activity-codebase-growth.md: which commit
// of the first-parent history measures each UTC day.

package sitehistory

import (
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/repo/objects"
)

// dateLayout is how a day is written: in a keyword, in the file and in a
// message.
const dateLayout = "2006-01-02"

// Date is one UTC calendar day.
//
// The zero Date names no day. DateOf and ParseDate never answer it, and
// MarshalText refuses it, so it cannot reach the file.
type Date struct {
	// midnight is the day's first instant, in UTC.
	midnight time.Time
}

// DateOf answers the UTC day an instant falls on, whatever its location.
func DateOf(instant time.Time) Date {
	year, month, day := instant.UTC().Date()
	return Date{midnight: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate reads a day written YYYY-MM-DD, two digits for the month and the
// day, and refuses any other text, a day the month does not hold included.
func ParseDate(raw string) (Date, error) {
	parsed, err := time.Parse(dateLayout, raw)
	if err != nil {
		return Date{}, fmt.Errorf("%q is not a day written YYYY-MM-DD: %w", raw, err)
	}
	return Date{midnight: parsed}, nil
}

// String writes the day as YYYY-MM-DD, for a reader. Compare days with
// Compare, never through this text.
func (d Date) String() string {
	return d.midnight.Format(dateLayout)
}

// Compare answers -1 when d is before other, 0 when they are the same day and
// +1 when d is after other.
func (d Date) Compare(other Date) int {
	return d.midnight.Compare(other.midnight)
}

// IsZero reports whether d names no day.
func (d Date) IsZero() bool {
	return d.midnight.IsZero()
}

// MarshalText writes the day as YYYY-MM-DD, and refuses the zero Date, which
// names no day.
func (d Date) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return nil, errors.New("a day of the size history holds no date")
	}
	return []byte(d.String()), nil
}

// UnmarshalText reads a day written YYYY-MM-DD, as ParseDate does.
func (d *Date) UnmarshalText(raw []byte) error {
	parsed, err := ParseDate(string(raw))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Day is one recorded day of the history: the day and the commit whose tree
// measures it.
type Day struct {
	// Date is the UTC day.
	Date Date
	// Commit is the full ID of the day's last commit in first-parent order.
	Commit string
	// Tree is the full ID of that commit's tree.
	Tree string
}

// SelectDays answers the recorded days of a first-parent history, oldest
// first. commits MUST be the first-parent ancestry oldest first, as
// objects.FirstParents answers it.
//
// A commit's day is its author day in UTC, raised to its parent's day when
// earlier, so days never run backward along the history: a March 2026 rewrite
// stamped 1,727 commits with one committer date, and the author date is the
// only clock that still places them. A day's commit is the last commit of
// that day in first-parent order, whatever its timestamp. A day with no commit
// is absent.
//
// A commit whose author day is later than its own committer day is refused,
// because the raise would carry every later commit onto that future day. An
// empty history is refused, because no day can be recorded from it.
func SelectDays(commits []objects.Commit) ([]Day, error) {
	if len(commits) == 0 {
		return nil, opererr.New("the history to measure holds no commit",
			"the size history records the last commit of each day, so a history with no commit has no day to record",
			"run ./le site history update in a checkout whose HEAD has at least one commit")
	}

	days := make([]Day, 0, len(commits))
	for index, commit := range commits {
		authored := DateOf(commit.AuthorTime)
		committed := DateOf(commit.CommitTime)
		if authored.Compare(committed) > 0 {
			return nil, authoredAfterCommitted(commits, index, authored, committed)
		}

		day := authored
		last := len(days) - 1
		if last >= 0 && day.Compare(days[last].Date) < 0 {
			day = days[last].Date
		}
		recorded := Day{Date: day, Commit: commit.ID, Tree: commit.Tree}
		if last >= 0 && day.Compare(days[last].Date) == 0 {
			days[last] = recorded
			continue
		}
		days = append(days, recorded)
	}
	return days, nil
}

// authoredAfterCommitted is the refusal of a commit whose author day is later
// than its committer day: it names the commit, both days, and the commit before
// it, up to which the history can still be measured.
func authoredAfterCommitted(commits []objects.Commit, index int, authored, committed Date) error {
	commit := commits[index].ID
	problem := "commit " + commit + " was authored on " + authored.String() +
		", after it was committed on " + committed.String()
	why := "its author date is in the future relative to when it was committed, and the size history places each commit on its author day, so every later commit would be carried onto that future day"
	fix := "correct the author date of commit " + commit + " so it is not later than " + committed.String() +
		", then run ./le site history update"
	if index > 0 {
		fix = "correct the author date of commit " + commit + " so it is not later than " + committed.String() +
			", or measure the history only up to its parent " + commits[index-1].ID
	}
	return opererr.New(problem, why, fix)
}
