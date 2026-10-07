// Design: docs/contributing/feature-maturity.md -- re-recording runs before they age out
// Related: runrecord.go -- the age bound a due run is measured against
// Related: recordrun.go -- the writer each due feature is run through
//
// A recorded run counts for runAgeDaysMax days. No workflow can write a record
// back to the repository (.github/workflows/evidence-nightly.yml holds
// `contents: read` and commits nothing), so a maintainer re-records the runs
// that are about to age out, before a release, with `record-run due <days>`.

package feature

import (
	"errors"
	"strconv"
	"time"
)

// DueRun is one feature `record-run due` re-recorded, or tried to.
type DueRun struct {
	Feature string `json:"feature"`
	// OldestRun is the date of the feature's oldest recorded run, the one that
	// made it due.
	OldestRun string `json:"oldest-run"`
	Recorded  bool   `json:"recorded"`
	// Refusal is why record-run wrote nothing; empty when Recorded.
	Refusal string `json:"refusal,omitempty"`
}

// RecordDue re-records, through RecordRun, every feature holding a recorded
// run older than days days today. The bool is true when every due feature was
// re-recorded.
func RecordDue(tree string, days int) ([]DueRun, bool, error) {
	due, err := recordDue(tree, days, calendarDay(time.Now()), func(id string) error {
		_, err := RecordRun(tree, id)
		return err
	})
	if err != nil {
		return nil, false, err
	}
	for _, run := range due {
		if !run.Recorded {
			return due, false, nil
		}
	}
	return due, true, nil
}

// recordDue is RecordDue judging on today with record as the writer, so a test
// drives it without a toolchain. A refused re-record is reported and the next
// due feature still runs: one failing test does not leave the others to age.
func recordDue(tree string, days int, today time.Time, record func(id string) error) ([]DueRun, error) {
	if days < 0 {
		return nil, errors.New("record-run due " + strconv.Itoa(days) + ": a day count is never negative")
	}
	if days > runAgeDaysMax {
		return nil, errors.New("record-run due " + strconv.Itoa(days) + ": a run older than " +
			strconv.Itoa(runAgeDaysMax) + " days already stopped counting, so a bound above " +
			strconv.Itoa(runAgeDaysMax) + " would skip it")
	}
	declarations, problems, err := Load(tree)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	due := []DueRun{}
	for i := range declarations {
		id := declarations[i].ID
		runs, err := loadRunRecord(tree, id, today)
		if err != nil {
			return nil, err
		}
		oldest, held := runs.oldestDate()
		if !held {
			continue // nothing recorded: a first record is the maintainer's decision.
		}
		if runAgeDays(oldest, today) <= days {
			continue
		}
		run := DueRun{Feature: id, OldestRun: oldest, Recorded: true}
		if err := record(id); err != nil {
			run.Recorded = false
			run.Refusal = err.Error()
		}
		due = append(due, run)
	}
	return due, nil
}

// oldestDate answers the date of the record's oldest run, and false for a
// record holding none. Dates are YYYY-MM-DD, so they order as strings.
func (r RunRecord) oldestDate() (string, bool) {
	oldest := ""
	for _, run := range r.Runs {
		if oldest == "" || run.Date < oldest {
			oldest = run.Date
		}
	}
	for _, run := range r.Interop {
		if oldest == "" || run.Date < oldest {
			oldest = run.Date
		}
	}
	return oldest, oldest != ""
}
