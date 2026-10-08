// Related: runrecord.go -- the age bound a recorded run is judged against
// Related: held.go -- the Supported level HEAD already holds, which a stale run never lowers
// Related: due.go -- `record-run due`, which re-records a run before it ages out
//
// VALIDATES: the owner decisions on run staleness. A recorded green run,
// real-path or interop, is current while its content id matches AND it is at
// most 30 days old on the day the check judges (2026-10-07, option (c)). A run
// that goes stale never lowers a Supported level HEAD already holds: the check
// accepts it and names the stale run as a warning, distinct by cause; a
// promotion to Supported still needs every counted run current (2026-10-08).
// Each case judges one fixture tree on day 30 (current) and day 31 (stale), so
// the bound has an observed green beside an observed red. `record-run due`
// re-records exactly the features holding a run older than its bound.
// PREVENTS: a supported feature dropping off the page because nobody re-ran
// its tests within a month, and a promotion standing on a run that no longer
// proves the test as it is now.

package feature

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// requireWarned asserts one warning of the widget verdict contains want, and
// that the verdict holds Supported with no refusal.
func requireWarned(t *testing.T, verdict *Verdict, want string) {
	t.Helper()
	if len(verdict.Refusals) > 0 {
		t.Fatalf("a held Supported level was refused on a stale run: %v", verdict.Refusals)
	}
	if verdict.Ceiling != LevelSupported {
		t.Fatalf("ceiling %v, want supported; unmet %v", verdict.Ceiling, verdict.Unmet)
	}
	for _, warning := range verdict.Warnings {
		if strings.Contains(warning, want) {
			return
		}
	}
	t.Fatalf("no warning containing %q; warnings: %v", want, verdict.Warnings)
}

// promotedFixture is fixtureTree with HEAD holding the declaration at
// experimental and the working tree raising it to supported, which is what a
// promotion looks like to the check before it is committed. The Doc review is
// dated today, so the uncommitted declaration does not make it stale.
func promotedFixture(t *testing.T, edit func(string) string, today time.Time) string {
	t.Helper()
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(edit(text), "| Level | supported |", "| Level | experimental |", 1)
	})
	promoted := strings.Replace(edit(passingDeclaration), "| Doc review | 2026-10-07:",
		"| Doc review | "+today.Format(attestationLayout)+":", 1)
	writeFile(t, tree, "features/widget.md", promoted)
	return tree
}

// TestCheckCountsARealPathRunForThirtyDays: a real-path run recorded on the
// fixture day is current on day 30 and stale on day 31. On a Supported level
// HEAD holds, the stale run is a warning naming the age and the recorded date,
// and the level stands (owner decision 2026-10-08).
func TestCheckCountsARealPathRunForThirtyDays(t *testing.T) {
	tree := fixtureTree(t, same)
	verdict := judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax))
	if len(verdict.Refusals)+len(verdict.Warnings) > 0 {
		t.Fatalf("a run 30 days old was refused or warned: %v %v", verdict.Refusals, verdict.Warnings)
	}
	verdict = judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax+1))
	requireWarned(t, &verdict, "S1: test/plugin/widget.ci stale: older than 30 days (recorded 2026-10-07)")
}

// TestCheckCountsAnInteropRunForThirtyDays is the same bound on an Interop
// entry's recorded scenario run.
func TestCheckCountsAnInteropRunForThirtyDays(t *testing.T) {
	tree := fixtureTree(t, protocolWith(fixtureScenario))
	verdict := judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax))
	if len(verdict.Refusals)+len(verdict.Warnings) > 0 {
		t.Fatalf("a scenario run 30 days old was refused or warned: %v %v", verdict.Refusals, verdict.Warnings)
	}
	verdict = judgeOn(t, tree, fixtureToday.AddDate(0, 0, runAgeDaysMax+1))
	requireWarned(t, &verdict, "S2: "+fixtureScenario+" stale: older than 30 days (recorded 2026-10-07)")
}

// TestCheckKeepsAHeldSupportedLevelOverAChangedRun: a test or scenario edited
// after its recorded run leaves a held Supported level standing, with a
// warning that names the change rather than the age.
func TestCheckKeepsAHeldSupportedLevelOverAChangedRun(t *testing.T) {
	tree := fixtureTree(t, protocolWith(fixtureScenario))
	writeFile(t, tree, "test/plugin/widget.ci", "cmd=foreground:seq=1:exec=ze changed\n")
	writeFile(t, tree, fixtureScenarioDir+"/README", "an edited scenario directory\n")
	verdict := judgeOne(t, tree)
	requireWarned(t, &verdict, "S1: test/plugin/widget.ci stale: test changed since its recorded green run")
	requireWarned(t, &verdict, "S2: "+fixtureScenario+" stale: scenario changed since its recorded green run")

	tree = fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Docs |", "| Extra criteria | supported: lab = "+fixtureScenario+" |\n| Docs |", 1)
	})
	writeFile(t, tree, fixtureScenarioDir+"/README", "an edited scenario directory\n")
	verdict = judgeOne(t, tree)
	requireWarned(t, &verdict, "extra criterion 'lab': "+fixtureScenario+" stale: scenario changed")
}

// TestCheckRefusesAPromotionOnAStaleRun: raising a level to Supported needs
// every counted run current, so an aged or changed run refuses the promotion
// that the same run only warns about once HEAD holds the level.
func TestCheckRefusesAPromotionOnAStaleRun(t *testing.T) {
	day31 := fixtureToday.AddDate(0, 0, runAgeDaysMax+1)
	verdict := judgeOn(t, promotedFixture(t, same, day31), day31)
	requireRefused(t, &verdict, "S1: test/plugin/widget.ci stale: older than 30 days (recorded 2026-10-07)")

	verdict = judgeOn(t, promotedFixture(t, protocolWith(fixtureScenario), day31), day31)
	requireRefused(t, &verdict, "S2: "+fixtureScenario+" stale: older than 30 days (recorded 2026-10-07)")

	tree := promotedFixture(t, same, fixtureToday)
	writeFile(t, tree, "test/plugin/widget.ci", "cmd=foreground:seq=1:exec=ze changed\n")
	verdict = judgeOne(t, tree)
	requireRefused(t, &verdict, "S1: test/plugin/widget.ci stale: test changed since its recorded green run")

	verdict = judgeOne(t, promotedFixture(t, same, fixtureToday))
	if len(verdict.Refusals)+len(verdict.Warnings) > 0 {
		t.Fatalf("a promotion on current runs was refused or warned: %v %v", verdict.Refusals, verdict.Warnings)
	}
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

// TestStaleSupportedNamesAHeldLevelWithAStaleRun: the list the site build
// warns from holds a feature HEAD keeps at Supported on a stale run, with the
// check's own warning, and nothing for a feature whose runs are current.
func TestStaleSupportedNamesAHeldLevelWithAStaleRun(t *testing.T) {
	tree := fixtureTree(t, same)
	stale, err := staleSupportedOn(tree, fixtureToday.AddDate(0, 0, runAgeDaysMax))
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Fatalf("a feature on current runs was listed stale: %v", stale)
	}
	stale, err = staleSupportedOn(tree, fixtureToday.AddDate(0, 0, runAgeDaysMax+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("stale %v, want widget alone", stale)
	}
	if stale[0].Feature != "widget" {
		t.Fatalf("stale %v, want widget", stale)
	}
	if !strings.Contains(strings.Join(stale[0].Stale, "\n"), "stale: older than 30 days (recorded 2026-10-07)") {
		t.Fatalf("stale runs %v do not name the age", stale[0].Stale)
	}
}

// TestRefreshStaleRecordsEachAndReportsARefusal: a refresh re-records every
// listed feature, and one that fails is reported with the runner's error
// without stopping the next.
func TestRefreshStaleRecordsEachAndReportsARefusal(t *testing.T) {
	var recorded []string
	refreshed := refreshStale([]StaleFeature{{Feature: "a"}, {Feature: "b"}}, func(id string) error {
		recorded = append(recorded, id)
		if id == "a" {
			return errors.New("the item failed")
		}
		return nil
	})
	if !slices.Equal(recorded, []string{"a", "b"}) {
		t.Fatalf("recorded %v, want [a b]", recorded)
	}
	if refreshed[0].Refreshed {
		t.Fatalf("a refused refresh reported as recorded: %v", refreshed[0])
	}
	if !strings.Contains(refreshed[0].Refusal, "the item failed") {
		t.Fatalf("refusal %q does not carry the runner's error", refreshed[0].Refusal)
	}
	if !refreshed[1].Refreshed {
		t.Fatalf("a recorded refresh reported as refused: %v", refreshed[1])
	}
}
