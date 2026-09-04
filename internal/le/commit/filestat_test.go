package commit

import (
	"strings"
	"testing"
)

// VALIDATES: StatText renders one line per staged path, and the counts are
// legible enough that an unexpectedly large file stands out.
//
// PREVENTS: the silence that let two of another session's uncommitted changes
// ride into commits in one night. `le commit create` stages the working-tree
// version of every path it is given, and printed nothing about what that
// version contained. The author believed one file was a one-line change; it
// carried twenty insertions of someone else's documentation, and the only
// reason it was caught was reading the git output afterwards.
func TestStatTextRendersEveryPathWithItsCounts(t *testing.T) {
	text := StatText([]FileStat{
		{Path: "internal/component/iface/rate.go", Added: 12, Deleted: 3},
		{Path: "docs/plugin-development/metrics.md", Added: 20, Deleted: 0},
		{Path: "internal/component/iface/new_test.go", New: true},
		{Path: "some/binary.bin", Added: statUnavailable, Deleted: statUnavailable},
	})

	for _, want := range []string{
		"internal/component/iface/rate.go",
		"docs/plugin-development/metrics.md",
		"internal/component/iface/new_test.go",
		"some/binary.bin",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("StatText dropped %s:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "+12 -3") {
		t.Errorf("StatText lost the counts:\n%s", text)
	}
	if !strings.Contains(text, "+20 -0") {
		t.Errorf("StatText lost the counts for the file this exists to make visible:\n%s", text)
	}
	if !strings.Contains(text, "new") {
		t.Errorf("a path HEAD does not have must say so rather than read as +0 -0:\n%s", text)
	}
	if !strings.Contains(text, "?") {
		t.Errorf("a path git could not measure must say so rather than be dropped:\n%s", text)
	}
	if lines := strings.Count(strings.TrimSpace(text), "\n"); lines != 4 {
		t.Errorf("want a header and four rows, got %d newlines:\n%s", lines, text)
	}
}

// VALIDATES: no paths renders nothing, so a caller can append it without a
// conditional and a commit that stages only removals prints no empty header.
func TestStatTextIsEmptyWithoutPaths(t *testing.T) {
	if got := StatText(nil); got != "" {
		t.Errorf("StatText(nil) = %q, want empty", got)
	}
	if got := StatText([]FileStat{}); got != "" {
		t.Errorf("StatText(empty) = %q, want empty", got)
	}
}

// VALIDATES: a binary file and an unmeasurable one are distinguished from a
// file that genuinely changed nothing. numstat writes `-` for a binary, which
// is the absence of a count and not a zero, and reporting it as +0 -0 would
// tell the author the file is inert when nothing is known about it.
func TestUnavailableCountsAreNotReportedAsZero(t *testing.T) {
	unavailable := statCounts(FileStat{Added: statUnavailable, Deleted: statUnavailable})
	zero := statCounts(FileStat{Added: 0, Deleted: 0})
	if unavailable == zero {
		t.Errorf("an unmeasurable file renders as %q, the same as an unchanged one", unavailable)
	}
	if strings.Contains(unavailable, "0") {
		t.Errorf("an unmeasurable file renders a count: %q", unavailable)
	}
}

// VALIDATES: countOrUnavailable reads a numstat column, and answers the
// unavailable sentinel for the `-` numstat writes for a binary file.
func TestCountOrUnavailableReadsNumstatColumns(t *testing.T) {
	if got := countOrUnavailable("12"); got != 12 {
		t.Errorf("countOrUnavailable(\"12\") = %d, want 12", got)
	}
	if got := countOrUnavailable("0"); got != 0 {
		t.Errorf("countOrUnavailable(\"0\") = %d, want 0", got)
	}
	if got := countOrUnavailable("-"); got != statUnavailable {
		t.Errorf("countOrUnavailable(\"-\") = %d, want the unavailable sentinel %d", got, statUnavailable)
	}
}
