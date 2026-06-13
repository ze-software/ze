// VALIDATES: Duration parsing with explicit units for CLI input.
// PREVENTS: Ambiguous bare numbers accepted as durations.

package duration

import "testing"

func TestParseMinutes(t *testing.T) {
	tests := []struct {
		input   string
		minutes int
		ok      bool
	}{
		{"0", 0, true},
		{"30m", 30, true},
		{"1h", 60, true},
		{"24h", 1440, true},
		{"2h", 120, true},
		{"90s", 2, true},
		{"60s", 1, true},
		{"1s", 1, true},
		{"59s", 1, true},
		{"30", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"m", 0, false},
		{"10x", 0, false},
		{"0m", 0, true},
		{"0h", 0, true},
		{"0s", 0, true},
		{"99999999999999999999m", 0, false},
		{"35791395h", 0, false},
	}

	for _, tt := range tests {
		minutes, ok := ParseMinutes(tt.input)
		if ok != tt.ok || (ok && minutes != tt.minutes) {
			t.Errorf("ParseMinutes(%q) = (%d, %v), want (%d, %v)", tt.input, minutes, ok, tt.minutes, tt.ok)
		}
	}
}
