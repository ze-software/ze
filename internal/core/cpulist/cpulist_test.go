package cpulist

import (
	"strings"
	"testing"
)

// TestParse covers the CPU-list grammar Ze reads from sysfs and accepts
// from the worker-cores leaf. The two share one parser, so one table proves
// both.
//
// VALIDATES: the boundary rows of the spec's Boundary Tests table -- core id 0,
// core id 255, and core id 256 which no uint8 holds.
// PREVENTS: a malformed list parsing to a plausible-looking core set, which
// would pin VPP workers to CPUs the operator never named.
func TestParse(t *testing.T) {
	valid := []struct {
		in   string
		want []uint8
	}{
		{"", nil},
		{"   \n", nil},
		{"0", []uint8{0}},
		{"255", []uint8{255}},
		{"0-3", []uint8{0, 1, 2, 3}},
		{"0-31\n", []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
			16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}},
		{"7,2-3", []uint8{2, 3, 7}},
		{"4-4", []uint8{4}},
	}
	for _, tt := range valid {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}

	invalid := []struct {
		in     string
		reason string
	}{
		{"256", "above the 255"},
		{"1-256", "above the 255"},
		{"3-1", "counts down"},
		{"2,2", "listed more than once"},
		{"0-3,2", "listed more than once"},
		{"a", "is not a number"},
		{"1,,2", "is not a number"},
		{"-1", "is not a number"},
	}
	for _, tt := range invalid {
		got, err := Parse(tt.in)
		if err == nil {
			t.Errorf("Parse(%q) = %v, want an error", tt.in, got)
			continue
		}
		if !strings.Contains(err.Error(), tt.reason) {
			t.Errorf("Parse(%q) error = %q, want it to name %q", tt.in, err, tt.reason)
		}
	}
}

// TestFormat proves the corelist-workers rendering collapses runs, which
// is what keeps the emitted line the "1-3" VPP operators read in every
// deployment guide.
func TestFormat(t *testing.T) {
	tests := []struct {
		in   []uint8
		want string
	}{
		{nil, ""},
		{[]uint8{3}, "3"},
		{[]uint8{1, 2, 3}, "1-3"},
		{[]uint8{1, 3}, "1,3"},
		{[]uint8{0, 1, 4, 5, 6, 9}, "0-1,4-6,9"},
		{[]uint8{254, 255}, "254-255"},
	}
	for _, tt := range tests {
		if got := Format(tt.in); got != tt.want {
			t.Errorf("Format(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
