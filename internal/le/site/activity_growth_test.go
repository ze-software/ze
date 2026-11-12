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
