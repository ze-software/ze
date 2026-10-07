// Related: runrecord.go -- the age bound a recorded run is judged against
// Related: due.go -- `record-run due`, which re-records a run before it ages out
//
// VALIDATES: owner decision 2026-10-07, run staleness option (c): a recorded
// green run, real-path or interop, counts only while its content id matches AND
// it is at most 30 days old on the day the check judges. Each case judges one
// fixture tree on day 30 (current) and day 31 (stale), so the bound has an
// observed green beside an observed red. `record-run due` re-records exactly
// the features holding a run older than its bound.
// PREVENTS: a run recorded once standing as proof forever while something its
// test reads, but its content id does not cover, changes underneath it.

package feature

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestCheckCountsARealPathRunForThirtyDays: a real-path run recorded on the
// fixture day is current on day 30 and stale on day 31, with the stale answer
// naming the age and the recorded date, distinct from a changed test.
func TestCheckCountsARealPathRunForThirtyDays(t *testing.T) {
	tree := fixtureTree(t, same)
	verdict := judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax))
	if len(verdict.Refusals) > 0 {
		t.Fatalf("a run 30 days old was refused: %v", verdict.Refusals)
	}
	verdict = judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax+1))
	requireRefused(t, &verdict, "S1: test/plugin/widget.ci stale: older than 30 days (recorded 2026-10-07)")
}

// TestCheckCountsAnInteropRunForThirtyDays is the same bound on an Interop
// entry's recorded scenario run.
func TestCheckCountsAnInteropRunForThirtyDays(t *testing.T) {
	tree := fixtureTree(t, protocolWith(fixtureScenario))
	verdict := judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax))
	if len(verdict.Refusals) > 0 {
		t.Fatalf("a scenario run 30 days old was refused: %v", verdict.Refusals)
	}
	verdict = judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax+1))
	requireRefused(t, &verdict, "S2: "+fixtureScenario+" stale: older than 30 days (recorded 2026-10-07)")
}

// TestCheckRefusesARunDatedAfterToday: a run dated in the future would never
// age out, so the record holding it is refused rather than counted.
func TestCheckRefusesARunDatedAfterToday(t *testing.T) {
	verdict := judgeOn(t, fixtureTree(t, same), fixtureToday.AddDate(0, 0, -1))
	requireRefused(t, &verdict, "is dated 2026-10-07, after today 2026-10-06")
}

// TestRecordDueReRecordsOnlyARunOlderThanTheBound: with a bound of 25 days, a
// run 25 days old is not due and one 26 days old is; a refused re-record is
// reported, not hidden; a bound above the age limit is refused, because it
// would skip runs that already stopped counting.
func TestRecordDueReRecordsOnlyARunOlderThanTheBound(t *testing.T) {
	tree := fixtureTree(t, same)
	var recorded []string
	record := func(id string) error {
		recorded = append(recorded, id)
		return nil
	}
	due, err := recordDue(tree, 25, fixtureToday.AddDate(0, 0, 25), record)
	if err != nil {
		t.Fatal(err)
	}
	if len(due)+len(recorded) > 0 {
		t.Fatalf("a run 25 days old was due under a 25-day bound: %v, recorded %v", due, recorded)
	}

	due, err = recordDue(tree, 25, fixtureToday.AddDate(0, 0, 26), record)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(recorded, []string{"widget"}) {
		t.Fatalf("recorded %v, want [widget]", recorded)
	}
	want := []DueRun{{Feature: "widget", OldestRun: "2026-10-07", Recorded: true}}
	if !slices.Equal(due, want) {
		t.Fatalf("due %v, want %v", due, want)
	}

	due, err = recordDue(tree, 25, fixtureToday.AddDate(0, 0, 26), func(string) error {
		return errors.New("the item failed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatalf("due %v, want one refused entry", due)
	}
	if due[0].Recorded {
		t.Fatalf("a refused re-record reported as recorded: %v", due[0])
	}
	if !strings.Contains(due[0].Refusal, "the item failed") {
		t.Fatalf("refusal %q does not carry the runner's error", due[0].Refusal)
	}

	if _, err := recordDue(tree, runAgeDaysMax+1, fixtureToday, record); err == nil {
		t.Fatal("a bound above the age limit was accepted")
	}
}
