// Design: docs/architecture/site-facts.md -- recorded Go lines share the activity inventory.
package sitefacts

import (
	"strconv"
	"testing"
)

// TestRepositoryFactsIncludeGoLineInventory proves that recorded line totals
// include comments and blanks while excluding dependencies and untracked work.
func TestRepositoryFactsIncludeGoLineInventory(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"source/source.go":             "// Source comment.\npackage source\n\nvar Value = 1\n",
		"source/source_test.go":        "// Test comment.\npackage source\n\n",
		"vendor/example/dependency.go": "package dependency\nvar Value = 2\n",
	})
	writeFixtureFile(t, root, "untracked.go", "package untracked\n")
	updated(t, root)
	written := readFacts(t, root)
	for key, want := range map[string]int{
		"repo.go_source_lines": 4,
		"repo.go_test_lines":   3,
	} {
		held, found := written.Facts[key]
		if !found {
			t.Errorf("recorded facts lack %s", key)
			continue
		}
		if held.Value != want || held.Category != categoryCommitted || held.Source == "" {
			t.Errorf("%s = %+v; want %d committed lines with provenance", key, held, want)
		}
	}
}

// TestGoLineDisplayPrecisionMatchesStaleness checks the boundary a visitor can
// see, which is finer than the older repository-count formatter's precision.
func TestGoLineDisplayPrecisionMatchesStaleness(t *testing.T) {
	for _, key := range []string{"repo.go_source_lines", "repo.go_test_lines"} {
		t.Run(key, func(t *testing.T) {
			before := facts{Facts: map[string]fact{key: {
				Category: categoryCommitted, Value: 1_564_999, Source: "fixture",
			}}}
			sameDisplay := facts{Facts: map[string]fact{key: {
				Category: categoryCommitted, Value: 1_560_001, Source: "fixture",
			}}}
			if differences := compare(before, sameDisplay); len(differences) != 0 {
				t.Errorf("equivalent 1.56 million figures disagree: %+v", differences)
			}
			after := facts{Facts: map[string]fact{key: {
				Category: categoryCommitted, Value: 1_565_000, Source: "fixture",
			}}}
			differences := compare(before, after)
			if len(differences) != 1 {
				t.Fatalf("1.56 million → 1.57 million has %d differences, want one: %+v", len(differences), differences)
			}
			if differences[0].Committed != "1.56 million" || differences[0].Derived != "1.57 million" {
				t.Errorf("visible difference = %+v, want 1.56 million → 1.57 million", differences[0])
			}
		})
	}
}

// TestGoLineFormattingBoundaries holds the compact units at their rounding
// transitions and the largest inventory the machine's integer can represent.
func TestGoLineFormattingBoundaries(t *testing.T) {
	for count, want := range map[int]string{
		0: "0", 999: "999", 1_000: "1.00 thousand",
		999_994: "999.99 thousand", 999_995: "1.00 million", 1_000_000: "1.00 million",
		1_564_999: "1.56 million", 1_565_000: "1.57 million",
	} {
		if got := FormatGoLines(count); got != want {
			t.Errorf("FormatGoLines(%d) = %q, want %q", count, got, want)
		}
	}
	maximum := int(^uint(0) >> 1)
	want := "9223372036854.78 million"
	if strconv.IntSize == 32 {
		want = "2147.48 million"
	}
	if got := FormatGoLines(maximum); got != want {
		t.Errorf("FormatGoLines(maxInt) = %q, want %q", got, want)
	}
}
