// Design: website/AI.md -- one commit calendar, measured once and drawn by two pages
// Related: activity.go renders the standalone dashboard from it; internal/le/site
// renders the published /project/activity/ page from the same value.
package reporewrite

import (
	"html"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ActivitySeries is one measured series over a window: added lines, deleted
// lines, or commits.
type ActivitySeries struct {
	// Total sums the window, ActiveDays counts the days that carried anything,
	// and Peak is the largest single day, which PeakDay names.
	Total      int
	ActiveDays int
	Peak       int
	PeakDay    time.Time
	// Thresholds are the four boundaries between the five non-zero heat
	// levels, ascending. A series whose largest day is one or zero answers
	// four zeros, so every non-zero day draws at the top level.
	Thresholds []int
	// daily holds the value of each day, which the counts above summarize. It
	// stays unexported because a reader outside this package wants the counts:
	// the standalone dashboard's top-days table is the one reader that needs
	// the days themselves, and it lives here.
	daily map[time.Time]int
}

// ActivityWindow is one repository's commit history over a span of days, laid
// out as a calendar heatmap, with the Go source inventory of the checkout it
// was measured in.
//
// Two pages draw it: the standalone dashboard writeActivity writes, and the
// published /project/activity/ page internal/le/site renders through the shared
// site shell. The two pages differ in their chrome and agree on their grid, so
// the grid markup is produced here once rather than spelled in both.
type ActivityWindow struct {
	// Start and End bound the measured days, both inclusive, as UTC dates.
	Start, End time.Time
	// FromFirstCommit says Start is on or before the day of the repository's
	// first commit, so the window holds every commit of the history.
	FromFirstCommit bool
	// Weeks is how many columns the grid holds. A column is one week starting
	// on Sunday, so the grid runs from the Sunday at or before Start to the
	// Saturday at or after End and always divides into whole weeks.
	Weeks int
	// MonthLabels is one month-label span for each week column, most of them
	// empty: a month is named on the column its first day falls in.
	MonthLabels string
	// Cells is one day-cell div for each day of the grid, the seven days of
	// one week before the next week's seven, carrying added lines and commits.
	// The standalone dashboard and the talk slide draw it.
	Cells string
	// CellsWithDeleted is the same grid with each cell also carrying the
	// day's deleted lines, and the lines the third-party trees added and
	// deleted that day, which the published page draws.
	CellsWithDeleted string
	// Lines counts added lines, Deleted the lines deleted, under the same path
	// rules over the same numstat lines, and Commits counts commits, all over
	// the same days.
	Lines, Deleted, Commits ActivitySeries
	// ThirdPartyLines and ThirdPartyDeleted count the lines added and deleted
	// in the third-party trees ThirdPartyPath names, over the same numstat
	// lines and days. Only the committed measurement (MeasureCommittedActivity)
	// reads them, and Lines and Deleted never hold them, so the two pairs sum
	// to every committed text file; any other measurement answers them empty.
	ThirdPartyLines, ThirdPartyDeleted ActivitySeries
	// Go is the checkout's tracked Go source. It describes the tree as it
	// stands rather than the window, so it does not move with Start and End.
	Go ActivityGo
}

// MeasureActivity answers the days of history that end on today, measured over
// the repository at root with the dashboard's own file defaults: every source
// extension and every author. An empty ref takes the checked-out one.
//
// today decides the window, so a caller that must publish the same bytes twice
// MUST pass the same day twice. The measurement reads git and the working tree,
// and writes nothing.
func MeasureActivity(root string, days int, ref string, today time.Time) (ActivityWindow, error) {
	options := defaultActivityOptions(root)
	options.Days = days
	if ref != "" {
		options.Ref = ref
	}
	resolved, err := resolveActivityOptions(options)
	if err != nil {
		return ActivityWindow{}, err
	}
	return measureActivity(resolved, today)
}

// MeasureCommittedActivity answers the days of history that end on today, over
// the checked-out ref, for the published page: every committed file outside
// the third-party trees counts (ActivityOptions.Committed), the third-party
// trees' own lines are counted apart (ThirdPartyLines, ThirdPartyDeleted), and
// the window
// opens on the first day of a month (ActivityOptions.MonthStart), so it holds
// at most days days.
//
// today decides the window, as it does for MeasureActivity.
func MeasureCommittedActivity(root string, days int, today time.Time) (ActivityWindow, error) {
	options := defaultActivityOptions(root)
	options.Days = days
	options.Committed = true
	options.MonthStart = true
	resolved, err := resolveActivityOptions(options)
	if err != nil {
		return ActivityWindow{}, err
	}
	return measureActivity(resolved, today)
}

// measureActivity reads the history and the checkout that one dashboard draws.
func measureActivity(options ActivityOptions, generated time.Time) (ActivityWindow, error) {
	today := dateOnly(generated)
	repoStart, err := firstActivityCommit(options)
	if err != nil {
		return ActivityWindow{}, err
	}
	start := activityWindowStart(options, today, repoStart)
	totals, err := collectActivity(options, start, today)
	if err != nil {
		return ActivityWindow{}, err
	}
	inventory, err := collectGoStats(options)
	if err != nil {
		return ActivityWindow{}, err
	}
	lines := measureSeries(totals.Additions, start, today)
	deleted := measureSeries(totals.Deletions, start, today)
	commits := measureSeries(totals.Commits, start, today)
	published := activityCellTotals{
		deleted:           deleted,
		thirdPartyAdded:   measureSeries(totals.ThirdPartyAdditions, start, today),
		thirdPartyDeleted: measureSeries(totals.ThirdPartyDeletions, start, today),
	}
	weeks := weeksBetween(sundayBefore(start), saturdayAfter(today))
	return ActivityWindow{
		Start: start, End: today, FromFirstCommit: repoStart != nil && !repoStart.Before(start),
		Weeks: len(weeks), MonthLabels: monthLabels(weeks),
		Cells:            measureCells(weeks, start, today, repoStart, lines, commits, nil),
		CellsWithDeleted: measureCells(weeks, start, today, repoStart, lines, commits, &published),
		Lines:            lines, Deleted: deleted, Commits: commits,
		ThirdPartyLines: published.thirdPartyAdded, ThirdPartyDeleted: published.thirdPartyDeleted,
		Go: inventory,
	}, nil
}

// activityWindowStart answers the first day the window measures: Days days
// back to today, both counted, or under MonthStart the first day of a month.
// That is the first month start on or after the Days-back day, or the first
// day of the month of the repository's first commit (repoStart) when that is
// later, so the grid neither opens part-way through a month nor spends weeks
// before the history began.
func activityWindowStart(options ActivityOptions, today time.Time, repoStart *time.Time) time.Time {
	start := today.AddDate(0, 0, -(options.Days - 1))
	if !options.MonthStart {
		return start
	}
	if start.Day() != 1 {
		start = time.Date(start.Year(), start.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	}
	if repoStart == nil {
		return start
	}
	first := time.Date(repoStart.Year(), repoStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	if first.After(start) {
		return first
	}
	return start
}

// measureSeries counts one series over the window it was collected for.
//
// The counts read only the days inside the window, and the thresholds read
// every day the collection answered. Both bounds of the collection are today
// and the day the window opened, so the two populations agree: no day outside
// the window reaches either the counts or the heat scale.
func measureSeries(values map[time.Time]int, start, today time.Time) ActivitySeries {
	series := ActivitySeries{PeakDay: today, Thresholds: activityThresholds(values), daily: values}
	for day, value := range values {
		if day.Before(start) || day.After(today) {
			continue
		}
		series.Total += value
		if value > 0 {
			series.ActiveDays++
		}
		if value > series.Peak {
			series.Peak, series.PeakDay = value, day
		}
	}
	return series
}

// activityCellTotals is what the published grid's cells carry beyond the
// slide's: each day's deleted lines, and the lines the third-party trees added
// and deleted, which the cumulative curve's all-files line reads.
type activityCellTotals struct {
	deleted, thirdPartyAdded, thirdPartyDeleted ActivitySeries
}

// measureCells renders the calendar grid: one div for each day of every week
// column, carrying both metrics so a reader can switch between them without
// the page being rebuilt. When published is given, each cell also carries the
// lines that day deleted and the third-party trees' added and deleted lines;
// the heat level stays the added lines', because the grid shows how much work
// a day carried, and a day that rewrote as much as it removed would read as
// idle on a net scale. The third-party figures reach neither the level nor
// the cell's label: the grid counts first-party work.
//
// A day outside the measured window, and a day before the repository's first
// commit, are marked rather than dropped: the grid is a rectangle of whole
// weeks, and a missing cell would shift every later day by one square.
func measureCells(weeks [][]time.Time, start, today time.Time, repoStart *time.Time, lines, commits ActivitySeries, published *activityCellTotals) string {
	var cells textbuf.Buffer
	cells.Reset()
	for _, week := range weeks {
		for _, day := range week {
			added, committed := lines.daily[day], commits.daily[day]
			classes := "day-cell"
			if day.Before(start) || day.After(today) {
				classes += " outside"
			}
			if repoStart != nil && day.Before(*repoStart) {
				classes += " pre-repo"
			}

			// data-level repeats data-lines-level so the grid carries a heat
			// level before any script runs; the page script rewrites it for
			// the metric the switch shows.
			label := html.EscapeString(day.Format("Mon 02 Jan 2006"))
			addedText, committedText := displayNumber(added), displayNumber(committed)
			level := strconv.Itoa(activityLevel(added, lines.Thresholds))
			cells.Str(`<div class="`).Str(classes).
				Str(`" data-date="`).Str(day.Format("2006-01-02")).
				Str(`" data-date-label="`).Str(label).
				Str(`" data-lines="`).Int(int64(added)).
				Str(`" data-lines-display="`).Str(addedText).
				Str(`" data-lines-level="`).Str(level)
			deletedLabel := ""
			if published != nil {
				removed := published.deleted.daily[day]
				removedText := displayNumber(removed)
				cells.Str(`" data-deleted="`).Int(int64(removed)).
					Str(`" data-deleted-display="`).Str(removedText).
					Str(`" data-third-party-added="`).Int(int64(published.thirdPartyAdded.daily[day])).
					Str(`" data-third-party-deleted="`).Int(int64(published.thirdPartyDeleted.daily[day]))
				deletedLabel = removedText + " lines deleted, "
			}
			cells.Str(`" data-commits="`).Int(int64(committed)).
				Str(`" data-commits-display="`).Str(committedText).
				Str(`" data-commits-level="`).Int(int64(activityLevel(committed, commits.Thresholds))).
				Str(`" data-level="`).Str(level).
				Str(`" aria-label="`).Str(label).Str(`: `).Str(addedText).Str(` lines added, `).Str(deletedLabel).Str(committedText).
				Str(` commits" tabindex="0"></div>` + "\n")
		}
	}
	return cells.String()
}
