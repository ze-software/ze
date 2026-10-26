package citation

import (
	"path"
	"testing"
)

// VALIDATES: a live corpus file under a record tree is still policed, and a
// record beside it is not, so a citation rewrite skips exactly the records.
// PREVENTS: `./le rfc move` rewriting a journal row or a weakened shard, or
// leaving a learned index pointing at the old name.
func TestPolicedKeepsCorpusFilesUnderRecordTrees(t *testing.T) {
	cases := []struct {
		path    string
		policed bool
	}{
		{"plan/learned/RECURRING-PATTERNS.md", true},
		{"plan/README.md", true},
		{"plan/templates/TEMPLATE.md", true},
		{"plan/templates/TEMPLATE-CLOSURE.md", true},
		{"ai/rules/testing.md", true},
		{"docs/contributing/rfc-conformance-gates.md", true},
		{"internal/component/gtsm/gtsm_test.go", true},
		{"plan/learned/123-some-spec.md", false},
		{"plan/journal/unwired-feature.md", false},
		{"plan/records/knowledge-routing-plan.md", false},
		{"plan/release/fixes/spec-open.md", false},
		{"plan/proposals/spec-idea.md", false},
		{"vendor/example.com/x/README.md", false},
	}
	for _, tc := range cases {
		if got := Policed(tc.path); got != tc.policed {
			t.Errorf("Policed(%q) = %v, want %v", tc.path, got, tc.policed)
		}
	}
}

// VALIDATES: every corpus glob is well formed, so InCorpus never meets the
// path.Match error it reads as "no match".
func TestCorpusGlobsAreWellFormed(t *testing.T) {
	for _, pattern := range CorpusGlobs {
		if _, err := path.Match(pattern, ""); err != nil {
			t.Errorf("corpus glob %q: %v", pattern, err)
		}
	}
}
