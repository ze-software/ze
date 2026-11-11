// Design: website/AI.md -- page tabs reuse the calendar and current inventory.
package site

import (
	"strings"
	"testing"
	"time"

	reporewrite "github.com/ze-software/ze/internal/le/repo/rewrite"
)

func TestActivityPageOffersGrowthRepresentation(t *testing.T) {
	window := reporewrite.ActivityWindow{
		Start: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		Go: reporewrite.ActivityGo{
			Total:  reporewrite.ActivityGoBucket{TotalLines: 30},
			Code:   reporewrite.ActivityGoBucket{TotalLines: 10},
			Tests:  reporewrite.ActivityGoBucket{TotalLines: 20},
			Vendor: reporewrite.ActivityGoBucket{TotalLines: 40},
		},
	}
	body := activityBody(&window, activitySurfacePage)
	for _, want := range []string{
		`id="activity-tab-calendar" role="tab" aria-controls="activity-calendar" aria-selected="true"`,
		`id="activity-tab-growth" role="tab" aria-controls="activity-growth" aria-selected="false"`,
		`id="activity-calendar" role="tabpanel" aria-labelledby="activity-tab-calendar"`,
		`id="activity-growth" role="tabpanel" aria-labelledby="activity-tab-growth" hidden`,
		`id="activity-growth-lines-chart"`,
		`id="activity-growth-commits-chart"`,
		`id="activity-growth-code">10</strong>`,
		`id="activity-growth-tests">20</strong>`,
		`id="activity-growth-vendor">40</strong>`,
		`id="activity-growth-overall">70</strong>`,
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
