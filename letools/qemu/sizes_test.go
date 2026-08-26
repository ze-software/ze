package qemu

// The byte-size parse the hugepage proof is configured through.
//
// Goal: prove that every size an operator can type is either read as a whole
// number of bytes or REFUSED. The goal is also to prove that a page count is
// always a whole number of whole pages. Method: use a table of sizes on either
// side of each boundary. Call the exported ParseSize and PageCount instead of
// their helpers. The run calls those two functions.

import (
	"testing"
)

// VALIDATES: each unit's multiplier, and that the "b" suffix does not swallow
// the tail of a longer unit.
// PREVENTS: a unit table reordered so that "b" matches "mb" first, which would
// read a 128 MB reservation as 128 bytes and reserve nothing.
func TestEachUnitAnswersItsOwnMultiplier(t *testing.T) {
	cases := []struct {
		size  string
		bytes uint64
	}{
		{"0b", 0},
		{"1b", 1},
		{"1kb", 1024},
		{"1mb", 1024 * 1024},
		{"128mb", 128 * 1024 * 1024},
		{"1gb", 1024 * 1024 * 1024},
		{"1tb", 1024 * 1024 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
	}
	for _, one := range cases {
		got, err := ParseSize(one.size)
		if err != nil {
			t.Errorf("ParseSize(%q): %v", one.size, err)
			continue
		}
		if got != one.bytes {
			t.Errorf("ParseSize(%q) = %d, want %d", one.size, got, one.bytes)
		}
	}
}

// VALIDATES: every shape that is not a whole number of bytes is refused.
// PREVENTS: the Python's int() reaching a kernel command line. "-1gb" answered
// a negative byte count there, and "1.5gb" ended the run in a traceback.
func TestASizeThatIsNotAWholeNumberIsRefused(t *testing.T) {
	for _, size := range []string{
		"", "b", "mb", "gb", "kb", "tb", " 1gb ",
		"-1gb", "+1gb", "1.5gb", "1 gb", "1gib", "gb1", "1", "128",
		"0x10mb", "99999999999999999mb",
	} {
		if got, err := ParseSize(size); err == nil {
			t.Errorf("ParseSize(%q) answered %d, want a refusal", size, got)
		}
	}
}

// VALIDATES: a page count is the number of WHOLE pages that fit, and a
// reservation smaller than one page is refused rather than answered as zero.
// PREVENTS: a zero page count reaching the kernel command line, where
// "hugepages=0" reserves nothing and the run then asserts that the kernel
// reserved something -- a red whose cause is the configuration rather than the
// appliance.
func TestPageCountRoundsDownAndRefusesLessThanOnePage(t *testing.T) {
	cases := []struct {
		total    string
		pageSize string
		pages    uint64
		refused  bool
	}{
		{"128mb", "2mb", 64, false},
		{"1gb", "2mb", 512, false},
		{"1gb", "1gb", 1, false},
		{"2mb", "2mb", 1, false},
		{"3mb", "2mb", 1, false},
		{"5mb", "2mb", 2, false},
		{"1mb", "2mb", 0, true},
		{"0b", "2mb", 0, true},
		{"2mb", "0b", 0, true},
		{"128mb", "notasize", 0, true},
		{"notasize", "2mb", 0, true},
	}
	for _, one := range cases {
		got, err := PageCount(one.total, one.pageSize)
		if one.refused {
			if err == nil {
				t.Errorf("PageCount(%q, %q) answered %d, want a refusal", one.total, one.pageSize, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("PageCount(%q, %q): %v", one.total, one.pageSize, err)
			continue
		}
		if got != one.pages {
			t.Errorf("PageCount(%q, %q) = %d, want %d", one.total, one.pageSize, got, one.pages)
		}
	}
}
