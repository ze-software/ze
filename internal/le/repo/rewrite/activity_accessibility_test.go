// Design: website/AI.md -- activity counts retain their meaning in accessible labels
package reporewrite

import (
	"strings"
	"testing"
	"time"
)

// TestActivityCalendarAccessibleCounts uses raw counts in spoken labels while
// keeping grouped display values for tooltips. A grouped historical count can
// equal an unrelated site statistic, so it must not look like that statistic's
// display string when the site checks accessible text.
func TestActivityCalendarAccessibleCounts(t *testing.T) {
	day := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
	lines := ActivitySeries{daily: map[time.Time]int{day: 2202}}
	commits := ActivitySeries{daily: map[time.Time]int{day: 1234}}
	totals := activityCellTotals{deleted: ActivitySeries{daily: map[time.Time]int{day: 5678}}}
	for _, published := range []bool{false, true} {
		t.Run(map[bool]string{false: "slide", true: "page"}[published], func(t *testing.T) {
			var extra *activityCellTotals
			if published {
				extra = &totals
			}
			cells := measureCells([][]time.Time{{day}}, day, day, nil, lines, commits, extra)
			label := `aria-label="Sat 14 Mar 2026: 2202 lines added, `
			if published {
				label += "5678 lines deleted, "
			}
			label += `1234 commits"`
			if !strings.Contains(cells, label) {
				t.Errorf("calendar label must state its raw counts: want %s in %s", label, cells)
			}
			for _, display := range []string{`data-lines-display="2,202"`, `data-commits-display="1,234"`} {
				if !strings.Contains(cells, display) {
					t.Errorf("calendar lost grouped display value %s", display)
				}
			}
		})
	}
}
