// VALIDATES: AC-3 the severity verdict moved out of the neutral trafficstat layer
// and is now computed display-only in the CLI from Snapshot.History, reproducing
// the historical ">2x recent-average caution / >5x danger" thresholds (need >=5
// samples; a zero average is normal).
// PREVENTS: a monitor-TUI severity regression when the verdict left window.go, and
// divide-by-zero on a cold/zero history.

package cmd

import "testing"

func TestDisplaySeverityFromHistory(t *testing.T) {
	cases := []struct {
		name    string
		history []float64
		want    string
	}{
		{"too-few-samples", []float64{1, 100, 100, 100}, "normal"},
		{"zero-average", []float64{0, 0, 0, 0, 0, 0}, "normal"},
		{"steady", []float64{5, 5, 5, 5, 5, 5}, "normal"},
		{"caution-over-2x", []float64{1, 1, 1, 1, 1, 10}, "caution"},
		{"danger-over-5x", []float64{1, 1, 1, 1, 1, 30}, "danger"},
		{"nil-history", nil, "normal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := displaySeverity(tc.history); got != tc.want {
				t.Errorf("displaySeverity(%v) = %q, want %q", tc.history, got, tc.want)
			}
		})
	}
}
