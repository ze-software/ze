package doccheck

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/doc/citation"
)

// VALIDATES: the link sweep reports, for every line, exactly the paths
// citation.Paths answers for that line, so `./le rfc rename`, which rewrites
// what citation.Paths answers, and the sweep agree about every citation.
// METHOD: one tracked page whose lines hold a backtick path, a backtick path
// with a symbol suffix, a markdown link, an anchor-only and an external link,
// and a plain mention, every target absent. The dead references the sweep
// reports for each line must equal citation.Paths over that line, and the plain
// mention and the two non-path links must report nothing.
func TestCitedPathsMatchesTheLinkSweep(t *testing.T) {
	lines := []string{
		"see `internal/absent/names_test.go` for the rule",
		"`internal/absent/names.go::stemPrefix` mints it",
		"[the gates](docs/absent/gates.md)",
		"[here](#anchor) and [there](https://example.com/x)",
		"a plain internal/absent/plain_test.go mention",
	}
	const page = "docs/architecture/cites.md"
	root := fixtureRepository(t, map[string]string{page: strings.Join(lines, "\n") + "\n"})
	report, err := checkLinks(root)
	if err != nil {
		t.Fatal(err)
	}
	reported := make(map[int][]string)
	for _, finding := range report.Errors {
		rest, ok := strings.CutPrefix(finding, page+":")
		if !ok {
			t.Fatalf("finding outside the fixture page: %q", finding)
		}
		number, target, ok := strings.Cut(rest, ": dead path reference: ")
		if !ok {
			t.Fatalf("finding is not a dead path reference: %q", finding)
		}
		target, _, _ = strings.Cut(target, " -- ")
		line, err := strconv.Atoi(number)
		if err != nil {
			t.Fatalf("finding has no line number: %q", finding)
		}
		reported[line] = append(reported[line], target)
	}
	total := 0
	for index, line := range lines {
		want := citation.Paths(root, line)
		total += len(want)
		got := reported[index+1]
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("line %d %q: the sweep reported %v, citation.Paths answers %v", index+1, line, got, want)
		}
	}
	if total != 3 {
		t.Errorf("citation.Paths answered %d paths over the fixture, want 3", total)
	}
}
