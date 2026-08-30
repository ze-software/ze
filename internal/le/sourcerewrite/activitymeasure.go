// Design: one measurement of a repository's commit calendar, drawn by two pages
// Related: activity.go renders the standalone dashboard from it; internal/le/site
// renders the published /project/activity/ page from the same value.
package sourcerewrite

import (
	"html"
	"strconv"
	"strings"
	"time"
)

// ActivitySeries is one measured series over a window: added source lines, or
// commits.
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
	// Weeks is how many columns the grid holds. A column is one week starting
	// on Sunday, so the grid runs from the Sunday at or before Start to the
	// Saturday at or after End and always divides into whole weeks.
	Weeks int
	// MonthLabels is one month-label span for each week column, most of them
	// empty: a month is named on the column its first day falls in, and only
	// once four columns have passed since the last name.
	MonthLabels string
	// Cells is one day-cell div for each day of the grid, the seven days of
	// one week before the next week's seven.
	Cells string
	// Lines counts added source lines and Commits counts commits, both over
	// the same days.
	Lines, Commits ActivitySeries
	// Go is the checkout's tracked Go source. It describes the tree as it
	// stands rather than the window, so it does not move with Start and End.
	Go ActivityGo
}

// MeasureActivity answers the year of history that ends on today, measured over
// the repository at root with the dashboard's own defaults: every source
// extension, every author, the checked-out ref.
//
// today decides the window, so a caller that must publish the same bytes twice
// MUST pass the same day twice. The measurement reads git and the working tree,
// and writes nothing.
func MeasureActivity(root string, today time.Time) (ActivityWindow, error) {
	options, err := resolveActivityOptions(defaultActivityOptions(root))
	if err != nil {
		return ActivityWindow{}, err
	}
	return measureActivity(options, today)
}

// measureActivity reads the history and the checkout that one dashboard draws.
func measureActivity(options ActivityOptions, generated time.Time) (ActivityWindow, error) {
	today := dateOnly(generated)
	start := today.AddDate(0, 0, -(options.Days - 1))
	totals, err := collectActivity(options, today)
	if err != nil {
		return ActivityWindow{}, err
	}
	repoStart, err := firstActivityCommit(options)
	if err != nil {
		return ActivityWindow{}, err
	}
	inventory, err := collectGoStats(options)
	if err != nil {
		return ActivityWindow{}, err
	}
	lines := measureSeries(totals.Additions, start, today)
	commits := measureSeries(totals.Commits, start, today)
	weeks := weeksBetween(sundayBefore(start), saturdayAfter(today))
	return ActivityWindow{
		Start: start, End: today, Weeks: len(weeks), MonthLabels: monthLabels(weeks),
		Cells: measureCells(weeks, start, today, repoStart, lines, commits),
		Lines: lines, Commits: commits, Go: inventory,
	}, nil
}

// measureSeries counts one series over the window it was collected for.
//
// The counts read only the days inside the window, and the thresholds read
// every day the collection answered. The two populations differ because the
// collection is bounded by --since alone, so a commit dated after today is
// counted by neither the total nor the peak while it still colors its own cell.
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

// measureCells renders the calendar grid: one div for each day of every week
// column, carrying both metrics so a reader can switch between them without
// the page being rebuilt.
//
// A day outside the measured window, and a day before the repository's first
// commit, are marked rather than dropped: the grid is a rectangle of whole
// weeks, and a missing cell would shift every later day by one square.
func measureCells(weeks [][]time.Time, start, today time.Time, repoStart *time.Time, lines, commits ActivitySeries) string {
	var cells strings.Builder
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

			// data-level repeats data-lines-level because the page opens on
			// the lines metric and the script overwrites data-level when the
			// reader switches to commits.
			label := html.EscapeString(day.Format("Mon 02 Jan 2006"))
			addedText, committedText := displayNumber(added), displayNumber(committed)
			level := strconv.Itoa(activityLevel(added, lines.Thresholds))
			cells.WriteString(`<div class="` + classes +
				`" data-date="` + day.Format("2006-01-02") +
				`" data-date-label="` + label +
				`" data-lines="` + strconv.Itoa(added) +
				`" data-lines-display="` + addedText +
				`" data-lines-level="` + level +
				`" data-commits="` + strconv.Itoa(committed) +
				`" data-commits-display="` + committedText +
				`" data-commits-level="` + strconv.Itoa(activityLevel(committed, commits.Thresholds)) +
				`" data-level="` + level +
				`" aria-label="` + label + `: ` + addedText + ` lines added, ` + committedText +
				` commits" tabindex="0"></div>` + "\n")
		}
	}
	return cells.String()
}
