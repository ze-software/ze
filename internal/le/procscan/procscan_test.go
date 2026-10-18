package procscan

import "testing"

// VALIDATES: the portable ps elapsed-time field parses in every shape ps emits,
// and a shape it never emits is refused rather than read as an age.
// PREVENTS: the reaper going blind again. `etimes` is Linux-only, so on macOS
// the whole process scan exited non-zero, no session looked either dead or
// running, and the command reported nothing to remove over eleven live
// directories -- a clean-tree answer produced by having examined nothing.
func TestElapsedSecondsReadsEveryShapePSEmits(t *testing.T) {
	for _, row := range []struct {
		value string
		want  int64
	}{
		{"00:00", 0},
		{"00:01", 1},
		{"01:00", 60},
		{"59:59", 3599},
		{"01:00:00", 3600},
		{"23:59:59", 86399},
		{"1-00:00:00", 86400},
		{"02-04:31:48", 189108},
		{"  02:30  ", 150},
	} {
		got, ok := elapsedSeconds(row.value)
		if !ok || got != row.want {
			t.Errorf("elapsedSeconds(%q) = %d, %v; want %d, true", row.value, got, ok, row.want)
		}
	}
}

func TestElapsedSecondsRefusesWhatPSNeverEmits(t *testing.T) {
	for _, value := range []string{"", "   ", "12", "1:2:3:4", "aa:bb", "-1:00", "x-01:00:00", "01:-1"} {
		if got, ok := elapsedSeconds(value); ok {
			t.Errorf("elapsedSeconds(%q) = %d, true; want a refusal", value, got)
		}
	}
}
