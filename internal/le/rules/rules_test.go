// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11. Functions expose the shared
// corpus spellings and match the Python answers.
// PREVENTS: messages that look equivalent but differ by byte. Python repr()
// selects quotes from content, and Python slices count runes. Go slices count
// bytes. Thus, a quoted rule line with an em dash can silently diverge.

package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPyReprPicksTheQuotePythonPicks(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"it's", `"it's"`},
		{`say "hi"`, `'say "hi"'`},
		{`both ' and "`, `'both \' and "'`},
		{"back\\slash", `'back\\slash'`},
		{"tab\there", `'tab\there'`},
		{"line\nbreak", `'line\nbreak'`},
		{"", "''"},
		{"em—dash", "'em—dash'"},
		{"\x00", `'\x00'`},
		{"\x7f", `'\x7f'`},
		// A zero-width space and a non-breaking space are both unprintable to
		// Python, and each escapes at its own width. Both are reachable: a rule
		// line pasted from a rendered document carries them.
		{"\u200b", `'\u200b'`},
		{"\u00a0", `'\xa0'`},
	}
	for _, tc := range cases {
		if got := pyRepr(tc.in); got != tc.want {
			t.Errorf("pyRepr(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestPyListReprRendersAPythonList(t *testing.T) {
	if got := pyListRepr(nil); got != "[]" {
		t.Errorf("pyListRepr(nil) = %s, want []", got)
	}
	if got := pyListRepr([]string{"advisory", "blocking"}); got != "['advisory', 'blocking']" {
		t.Errorf("pyListRepr = %s", got)
	}
}

func TestIsUpperStemNeedsACasedCharacter(t *testing.T) {
	cases := map[string]bool{
		"RETIRED":     true,
		"TRIGGERS":    true,
		"CORE":        true,
		"rule-format": false,
		"Planning":    false,
		"123":         false,
		"":            false,
		"ABC-1":       true,
	}
	for stem, want := range cases {
		if got := isUpperStem(stem); got != want {
			t.Errorf("isUpperStem(%q) = %v, want %v", stem, got, want)
		}
	}
}

func TestFirstRunesCountsRunesNotBytes(t *testing.T) {
	// The corpus is full of em dashes, and each one is three bytes. A byte
	// slice would cut one in half and produce invalid UTF-8 in a message the
	// script spells with Python's rune-counting slice.
	line := "a—b—c"
	if got := firstRunes(line, 3); got != "a—b" {
		t.Errorf("firstRunes = %q, want %q", got, "a—b")
	}
	if got := firstRunes(line, 99); got != line {
		t.Errorf("firstRunes past the end = %q, want the whole string", got)
	}
	if got := firstRunes(line, 0); got != "" {
		t.Errorf("firstRunes(_, 0) = %q, want the empty string", got)
	}
}

func TestLastRuneAnswersARuneNotAByte(t *testing.T) {
	if got := lastRune("ends—"); got != "—" {
		t.Errorf("lastRune = %q, want an em dash", got)
	}
	if got := lastRune("ends,"); got != "," {
		t.Errorf("lastRune = %q, want a comma", got)
	}
	if got := lastRune(""); got != "" {
		t.Errorf("lastRune of the empty string = %q", got)
	}
}

func TestRuleFilesDropsTheGeneratedArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"planning.md", "INDEX.md", "CONDENSED.md", "TRIGGERS.md", "CORE.md",
		"architecture.md", "notes.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "points"), 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	found, err := ruleFiles(dir)
	if err != nil {
		t.Fatalf("RuleFiles: %v", err)
	}
	want := []string{
		filepath.Join(dir, "architecture.md"),
		filepath.Join(dir, "planning.md"),
	}
	if len(found) != len(want) {
		t.Fatalf("RuleFiles found %v, want %v", found, want)
	}
	for i := range want {
		if found[i] != want[i] {
			t.Errorf("RuleFiles[%d] = %s, want %s", i, found[i], want[i])
		}
	}
}

func TestRelToNamesAPathAgainstTheCheckout(t *testing.T) {
	tree := string(filepath.Separator) + "checkout"
	if got := relTo(tree, filepath.Join(tree, "ai", "rules", "planning.md")); got != "ai/rules/planning.md" {
		t.Errorf("relTo = %s", got)
	}
}
