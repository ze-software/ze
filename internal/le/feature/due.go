// Design: docs/contributing/feature-maturity.md -- re-recording runs before they age out
// Related: runrecord.go -- the age bound a due run is measured against
// Related: recordrun.go -- the writer each due feature is run through
//
// A recorded run is current for runAgeDaysMax days. A stale run never lowers a
// Supported level HEAD holds (held.go), but it is named: `./le feature report`
// lists it as a warning, and `./le site build` names every supported feature
// carrying one before it publishes, re-recording them first when given
// `refresh` (StaleSupported, RefreshStale). No workflow can write a record back
// to the repository (.github/workflows/evidence-nightly.yml holds
// `contents: read` and commits nothing), so the refresh runs on a maintainer's
// machine, through the site build or `record-run due <days>`.

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
			strconv.Itoa(runAgeDaysMax) + " days is already stale, so a bound above " +
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

// StaleFeature is one feature HEAD holds at Supported that carries a stale
// recorded run. Stale holds the check's warnings, one per stale run, each
// naming why it is stale.
type StaleFeature struct {
	Feature string   `json:"feature"`
	Stale   []string `json:"stale"`
	// Refreshed is true when RefreshStale re-recorded every run of the
	// feature; Refusal is why it did not. Both are empty before a refresh.
	Refreshed bool   `json:"refreshed,omitempty"`
	Refusal   string `json:"refusal,omitempty"`
}

// StaleSupported answers every feature HEAD holds at Supported whose recorded
// runs include a stale one, judged today. A declaration the parser refuses is
// an error: the answer would otherwise omit it in silence.
func StaleSupported(tree string) ([]StaleFeature, error) {
	return staleSupportedOn(tree, calendarDay(time.Now()))
}

func staleSupportedOn(tree string, today time.Time) ([]StaleFeature, error) {
	verdicts, problems, err := checkOn(tree, today)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	stale := []StaleFeature{}
	for i := range verdicts {
		if len(verdicts[i].Warnings) == 0 {
			continue
		}
		stale = append(stale, StaleFeature{Feature: verdicts[i].Declaration.ID, Stale: verdicts[i].Warnings})
	}
	return stale, nil
}

// RefreshStale re-records, through RecordRun, every feature stale names, and
// answers each with whether it recorded. One refused feature does not stop the
// others. A refused refresh leaves the run stale and the level standing.
func RefreshStale(tree string, stale []StaleFeature) []StaleFeature {
	return refreshStale(stale, func(id string) error {
		_, err := RecordRun(tree, id)
		return err
	})
}

// refreshStale is RefreshStale with record as the writer, so a test drives it
// without a toolchain.
func refreshStale(stale []StaleFeature, record func(id string) error) []StaleFeature {
	refreshed := make([]StaleFeature, 0, len(stale))
	for _, feature := range stale {
		feature.Refreshed = true
		if err := record(feature.Feature); err != nil {
			feature.Refreshed = false
			feature.Refusal = err.Error()
		}
		refreshed = append(refreshed, feature)
	}
	return refreshed
}
