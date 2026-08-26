// VALIDATES: spec-le-is-a-ze-binary AC-11. The two gates print bytes that match
// Python difflib, including hunk grouping and autojunk.
// PREVENTS: a valid diff with different bytes. A different grouping weakens
// parity to a verdict comparison. A wrong hunk header also sends the author to
// the wrong line.
//
// python3 difflib on this machine produced every expectation below. Use the same
// method to derive them again after a difflib change.

package rules

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func TestTheUnifiedDiffIsPythonDifflibsBytes(t *testing.T) {
	numbered := func(prefix string, n int) []string {
		out := make([]string, 0, n)
		var tb textbuf.Buffer
		for i := range n {
			tb.Reset()
			out = append(out, tb.Str(prefix).Int(int64(i)).String())
		}
		return out
	}

	thirty := numbered("l", 30)
	thirtyEdited := append([]string(nil), thirty...)
	thirtyEdited[2] = "X"
	thirtyEdited[25] = "X"

	// 250 lines with a blank between each pair. The blank appears 125 times,
	// far past the one-per-cent bound, so autojunk purges it and the block
	// search never starts a run on it.
	var popular []string
	for _, line := range numbered("line ", 125) {
		popular = append(popular, line, "")
	}
	popularEdited := append([]string(nil), popular...)
	popularEdited[100] = "CHANGED"

	cases := []struct {
		name string
		a, b []string
		want []string
	}{
		{name: "one line changed",
			a: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
			b: []string{"a", "b", "c", "D", "e", "f", "g", "h"},
			want: []string{
				"--- a/f.md", "+++ b/f.md", "@@ -1,7 +1,7 @@",
				" a", " b", " c", "-d", "+D", " e", " f", " g",
			}},
		{name: "insert at head",
			a:    []string{"a", "b", "c"},
			b:    []string{"z", "a", "b", "c"},
			want: []string{"--- a/f.md", "+++ b/f.md", "@@ -1,3 +1,4 @@", "+z", " a", " b", " c"}},
		{name: "delete at tail",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b"},
			want: []string{"--- a/f.md", "+++ b/f.md", "@@ -1,3 +1,2 @@", " a", " b", "-c"}},
		{name: "two hunks far apart",
			a: thirty, b: thirtyEdited,
			want: []string{
				"--- a/f.md", "+++ b/f.md",
				"@@ -1,6 +1,6 @@", " l0", " l1", "-l2", "+X", " l3", " l4", " l5",
				"@@ -23,7 +23,7 @@", " l22", " l23", " l24", "-l25", "+X", " l26", " l27", " l28",
			}},
		{name: "empty a", a: nil, b: []string{"a", "b"},
			want: []string{"--- a/f.md", "+++ b/f.md", "@@ -0,0 +1,2 @@", "+a", "+b"}},
		{name: "empty b", a: []string{"a", "b"}, b: nil,
			want: []string{"--- a/f.md", "+++ b/f.md", "@@ -1,2 +0,0 @@", "-a", "-b"}},
		{name: "identical", a: []string{"a", "b"}, b: []string{"a", "b"}, want: nil},
		{name: "all different",
			a: []string{"a", "b", "c"}, b: []string{"x", "y", "z"},
			want: []string{"--- a/f.md", "+++ b/f.md", "@@ -1,3 +1,3 @@",
				"-a", "-b", "-c", "+x", "+y", "+z"}},
		{name: "autojunk 250 lines", a: popular, b: popularEdited,
			want: []string{
				"--- a/f.md", "+++ b/f.md", "@@ -98,7 +98,7 @@",
				" ", " line 49", " ", "-line 50", "+CHANGED", " ", " line 51", " ",
			}},
	}

	for _, tc := range cases {
		got := unifiedDiff(tc.a, tc.b, "a/f.md", "b/f.md")
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s:\ngot\n%s\nwant\n%s", tc.name,
				strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
		}
	}
}

func TestAutojunkChangesTheHunksAndThisPairProvesIt(t *testing.T) {
	// The 250-line case above passes with or without popular-element removal, so
	// it does not test autojunk. This pair does. It has 220 elements, a
	// 14-symbol alphabet, and three edits. With removal, difflib returns ONE
	// 158-line hunk and 316 diff lines. Without removal, it returns THREE hunks
	// and 29 lines. python3 difflib produced both results on this machine with
	// autojunk set to True and False.
	const (
		before = "flglkffglhbakgfihkayfcaygkjixhefijhiahxlgyxxbbhxkfdfhlbgbicfxiyfice" +
			"glyjcdkbgcxyhkyeylceyxlixbbxfejeeyhibkicdycdxekyabjyxxfkfaigbfgbjka" +
			"eieyeebxclkhcxbaelhihyxabfcxkhdlceyyybhibxhddhfahflhaleaiacblhbjbaji" +
			"gbiefhyexyyxgcbjje"
		after = "flglkffglhbakgfihkayfcaygkjixhefijhiahxlgyxxbbhxkfdfhlbgbicfxiyfiee" +
			"glyjcdkbgcxyhkyeylceyxlixbbxfejeeyhibkicdycdxekyabjyxxfkfacgbfgbjka" +
			"eieyeebxclkhcxbaelhihyxabfcxkhdlceyyybhibxhddhfahflhaleaiacblhbjbaji" +
			"gbiefhyehyyxgcbjje"
	)
	split := func(s string) []string {
		out := make([]string, 0, len(s))
		for _, r := range s {
			out = append(out, string(r))
		}
		return out
	}

	a, b := split(before), split(after)
	if len(a) != 220 || len(b) != 220 {
		t.Fatalf("the fixture is %d and %d elements, want 220 each", len(a), len(b))
	}

	got := unifiedDiff(a, b, "a/f.md", "b/f.md")
	var hunks []string
	for _, line := range got {
		if strings.HasPrefix(line, "@@") {
			hunks = append(hunks, line)
		}
	}
	if len(hunks) != 1 || hunks[0] != "@@ -63,158 +63,158 @@" {
		t.Errorf("the hunks are %v, want one at @@ -63,158 +63,158 @@ -- the popular-element purge did not run", hunks)
	}
	if len(got) != 316 {
		t.Errorf("the diff is %d lines, want 316", len(got))
	}
}

func TestAnEmptyRangePrintsTheLineItAttachesTo(t *testing.T) {
	cases := []struct {
		start, stop int
		want        string
	}{
		{0, 1, "1"},
		{0, 0, "0,0"},
		{4, 4, "4,0"},
		{0, 7, "1,7"},
		{22, 29, "23,7"},
	}
	for _, tc := range cases {
		if got := unifiedRange(tc.start, tc.stop); got != tc.want {
			t.Errorf("unifiedRange(%d, %d) = %s, want %s", tc.start, tc.stop, got, tc.want)
		}
	}
}

func TestTheDiffPageIsCutToItsBudget(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = "x"
	}
	if got := strings.Count(diffHead(lines), "\n"); got != 23 {
		t.Errorf("diffHead kept %d newlines, want 23", got)
	}
	if got := diffHead([]string{"only"}); got != "only" {
		t.Errorf("diffHead of a short page = %q", got)
	}
	if got := diffHead(nil); got != "" {
		t.Errorf("diffHead of nothing = %q", got)
	}
}
