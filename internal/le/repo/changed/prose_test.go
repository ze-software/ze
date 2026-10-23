package repochanged

import "testing"

// TestProsePathIsDeclaredOnTheRules proves the prose predicate answers from the
// nonGoPathRules marks alone: Markdown under the three prose trees is prose,
// and a root-level page, the RFC corpus, a non-Markdown file under a prose
// tree, a test body and Go are not. A push gate reads this answer to decide
// that a commit owes no full verification, so a false true here publishes code
// nobody verified.
// VALIDATES: AC-34 (spec-verification-debt-derived, D-9).
func TestProsePathIsDeclaredOnTheRules(t *testing.T) {
	cases := []struct {
		path  string
		prose bool
	}{
		{"docs/x.md", true},
		{"ai/rules/y.md", true},
		{"plan/z.md", true},
		{"README.md", false},
		{"AGENTS.md", false},
		{"CLAUDE.md", false},
		{"rfc/short/rfc4271.md", false},
		{"docs/x.png", false},
		{"test/runner/a.ci", false},
		{"internal/x/y.go", false},
		{"internal/x/README.md", false},
	}
	for _, c := range cases {
		if got := IsProse(c.path); got != c.prose {
			t.Errorf("IsProse(%q) = %v, want %v", c.path, got, c.prose)
		}
	}
}
