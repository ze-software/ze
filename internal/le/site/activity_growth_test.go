// Design: website/AI.md -- the page opens on the size view; the slide keeps its calendar.
package site

import (
	"strings"
	"testing"
	"time"

	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

// TestActivityPageOffersGrowthRepresentation proves the page offers its two
// views through tabs: repository size, selected first, and the commit calendar,
// each a tab panel the tabs control, with the size view's charts, date control
// and readout, and the cumulative curve under the calendar with its own date
// control and readout.
func TestActivityPageOffersGrowthRepresentation(t *testing.T) {
	window := reporewrite.ActivityWindow{
		Start: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
	}
	body, err := activityPageBody(&window, activityHistoryFigures(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`id="activity-tab-size" role="tab" aria-controls="activity-size" aria-selected="true"`,
		`id="activity-tab-calendar" role="tab" aria-controls="activity-calendar" aria-selected="false"`,
		`id="activity-size" role="tabpanel" aria-labelledby="activity-tab-size"`,
		`id="activity-calendar" role="tabpanel" aria-labelledby="activity-tab-calendar"`,
		`id="activity-size-chart"`,
		`id="activity-change-chart"`,
		`id="activity-size-day"`,
		`id="activity-size-readout"`,
		`id="activity-cumulative-chart"`,
		`id="activity-cumulative-day"`,
		`id="activity-cumulative-readout"`,
		`<span data-ze-history="go-production:lines"></span>`,
		`<span data-ze-history="go-tests:lines"></span>`,
		`<span data-ze-history="third-party-go:lines"></span>`,
		`<span data-ze-history="total:lines"></span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("activity page does not expose %s", want)
		}
	}
}

// TestTheCumulativeCurveStatesNetLines proves the curve's Added Lines wording
// says it draws lines added minus lines deleted, in its sentence, its readout
// label, its chart name and its slider unit, that the readout carries the
// detail the script fills with the added and deleted figures, and that the
// calendar's note states the window's added, deleted and net lines with the
// population they count.
//
// VALIDATES: the cumulative curve is balanced and says so; the calendar view
// shows deletions beside additions.
func TestTheCumulativeCurveStatesNetLines(t *testing.T) {
	window := reporewrite.ActivityWindow{
		Start:           time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		End:             time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		FromFirstCommit: true,
		Lines:           reporewrite.ActivitySeries{Total: 17},
		Deleted:         reporewrite.ActivitySeries{Total: 19},
	}
	body, err := activityPageBody(&window, activityHistoryFigures(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-text-lines="Lines added minus lines deleted by commits, accumulated since the calendar&#39;s first day. ` +
			`The curve falls on a day whose commits deleted more lines than they added. ` +
			`A second line draws the same figure over every committed file, the third-party trees included."`,
		`data-text-lines="Net lines"`,
		`<div class="cumulative-legend" hidden><span><span class="cumulative-swatch cumulative-swatch-first"></span>Without third-party</span>` +
			`<span><span class="cumulative-swatch"></span>All files</span></div>`,
		`Under Added Lines, the cumulative curve also draws every committed file, the third-party trees included, as a second line.`,
		`data-aria-lines="Cumulative lines added minus lines deleted by commits.`,
		`data-unit-lines="net lines"`,
		`<span id="activity-cumulative-detail" hidden></span>`,
		`Commits in this window added 17 lines and deleted 19 lines, a net change of -2 lines.`,
		`The lines count every file committed outside the third-party trees (vendor/, third_party/, website/assets/vendor/)`,
		`The window reaches back to the first commit, so the net change is the number of lines these files hold`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the calendar view does not carry %s", want)
		}
	}
	if strings.Contains(body, "not subtracted") {
		t.Error("the calendar view still says deleted lines are not subtracted")
	}

	window.FromFirstCommit = false
	body, err = activityPageBody(&window, activityHistoryFigures(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "reaches back to the first commit") {
		t.Error("a window that opens after the first commit claims it holds every commit")
	}
}

// TestTheSlideKeepsItsAddedLineCalendar proves the talk slide draws the grid
// without deleted lines and ships the tooltip wording it always shipped, while
// the page draws the grid that carries them. Method: give the two grids
// different markers and read which one each surface draws.
//
// VALIDATES: the slide's output does not change with the page's deletions.
func TestTheSlideKeepsItsAddedLineCalendar(t *testing.T) {
	window := reporewrite.ActivityWindow{Cells: "<!-- added-only grid -->", CellsWithDeleted: "<!-- grid with deleted lines -->"}
	slide := activityBody(&window, activitySurfaceSlide)
	if !strings.Contains(slide, "<!-- added-only grid -->") || strings.Contains(slide, "deleted") {
		t.Error("the slide draws deleted lines, so its output changed")
	}
	if strings.Contains(slide, "third-party") || strings.Contains(slide, "thirdParty") {
		t.Error("the slide carries third-party lines, so its output changed")
	}
	if !strings.Contains(slide, `return cell.dataset.linesDisplay + " " + plural(value, "line", "lines") + " added";`+"\n}\n") {
		t.Error("the slide's tooltip no longer ships its added-line wording")
	}
	page := activityChartHTML(&window, activitySurfacePage)
	if !strings.Contains(page, "<!-- grid with deleted lines -->") {
		t.Error("the page does not draw the grid that carries deleted lines")
	}
}

func TestActivitySlideKeepsCalendarRepresentation(t *testing.T) {
	window := reporewrite.ActivityWindow{}
	body := activityBody(&window, activitySurfaceSlide)
	if strings.Contains(body, `role="tab"`) {
		t.Error("slide embed must retain its existing calendar without page tabs")
	}
	if !strings.Contains(body, `class="activity-grid"`) {
		t.Error("slide embed lost its calendar grid")
	}
}
