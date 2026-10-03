package citation

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: symbol, test node-id, digest line-run, and brace references reduce to files.
// PREVENTS: live source citations being reported because their location suffix stayed attached.
func TestCitationGrammarPreservesFileTargets(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"internal/x/owner_test.go",
		"internal/x/owner.go",
		"internal/x/first.go",
		"internal/x/second.go",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "internal/x/owner_test.go::TestCase", want: "internal/x/owner_test.go"},
		{raw: "internal/x/owner.go:Owner", want: "internal/x/owner.go"},
		{raw: "internal/x/owner.go,47,64,82-90", want: "internal/x/owner.go"},
		{raw: "internal/x/{first,second}.go", want: "internal/x/first.go,internal/x/second.go"},
	}
	for _, test := range tests {
		got := strings.Join(candidates(root, test.raw), ",")
		if got != test.want {
			t.Errorf("candidates(%q)=%q, want %q", test.raw, got, test.want)
		}
	}
}

// VALIDATES: Paths answers a backtick path, a backtick path with a symbol
// suffix and a markdown link target, and answers nothing for an anchor-only
// link, an external URL and a path written outside backticks.
// METHOD: one line per shape, each answer compared with the exact expected
// list, so a grammar that widened or narrowed goes red.
func TestPathsAnswersEachCitationShape(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		line string
		want []string
	}{
		{"see `internal/le/rfc/names_test.go` for the rule", []string{"internal/le/rfc/names_test.go"}},
		{"`internal/le/rfc/names.go::stemPrefix` mints it", []string{"internal/le/rfc/names.go"}},
		{"[the gates](docs/contributing/rfc-conformance-gates.md)", []string{"docs/contributing/rfc-conformance-gates.md"}},
		{"[here](#anchor) and [there](https://example.com/x)", []string{}},
		{"a plain internal/le/rfc/names_test.go mention", []string{}},
	}
	for _, test := range tests {
		if got := Paths(root, test.line); !slices.Equal(got, test.want) {
			t.Errorf("Paths(%q) = %v, want %v", test.line, got, test.want)
		}
	}
}
