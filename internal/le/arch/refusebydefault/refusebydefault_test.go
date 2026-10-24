// VALIDATES: the refuse-by-default check refuses each of its four shapes on a
// changed line, accepts an annotated shape that names a visible guard and
// lists it, refuses an annotation that names none, judges only changed lines,
// and that report lists every shape under the source roots.
// PREVENTS: a gate that goes quiet because a rule's detection, its scope or
// its escape broke, and a fixture that only ever proved one rule.
package archrefusebydefault

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// every is a change set span covering every line of a fixture.
var every = []repochanged.LineSpan{{From: 1, To: 100000}}

// marked is one expected row: a line and the rule a fixture comment names.
type marked struct {
	Line int
	Rule Rule
}

// markers reads the `// want: <rule>` (or `// accepted: <rule>`) comments of
// a fixture, one per line, so the fixture is the only place an expectation is
// written.
func markers(t *testing.T, source, tag string) []marked {
	t.Helper()
	var found []marked
	for index, text := range strings.Split(source, "\n") {
		_, rule, ok := strings.Cut(text, "// "+tag+": ")
		if !ok {
			continue
		}
		found = append(found, marked{Line: index + 1, Rule: Rule(strings.TrimSpace(rule))})
	}
	return found
}

// rows answers a finding set as marked rows, for comparison.
func rows(findings Findings) []marked {
	answer := make([]marked, 0, len(findings))
	for _, finding := range findings {
		answer = append(answer, marked{Line: finding.Line, Rule: finding.Rule})
	}
	slices.SortFunc(answer, func(a, b marked) int { return a.Line - b.Line })
	return answer
}

// fixture reads one testdata fixture.
func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeSource writes a source file under tree, creating its directory.
func writeSource(t *testing.T, tree, rel, source string) {
	t.Helper()
	path := filepath.Join(tree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestRefuseByDefaultRefusesEachRuleFixture drives Check over one fixture per
// rule (AC-1 to AC-4), every line changed, and holds the findings to the
// fixture's own `want:` markers: each shape is refused on its line with its
// rule, and each correct neighbor in the fixture answers nothing. The rule
// set is checked whole, so a rule that stops firing reds its own fixture.
func TestRefuseByDefaultRefusesEachRuleFixture(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
	}{
		{name: "switch.go", rule: RuleSwitchNilError},
		{name: "miss.go", rule: RuleMissAnswered},
		{name: "pathid.go", rule: RulePathIDZero},
		{name: "state.go", rule: RuleLiteralState},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			source := fixture(t, current.name)
			want := markers(t, source, "want")
			if len(want) == 0 {
				t.Fatalf("fixture %s marks no finding, so it proves nothing", current.name)
			}
			for _, row := range want {
				if row.Rule != current.rule {
					t.Fatalf("fixture %s line %d marks %s, want only %s", current.name, row.Line, row.Rule, current.rule)
				}
			}
			tree := t.TempDir()
			rel := "internal/fixture/" + current.name
			writeSource(t, tree, rel, source)

			report, err := Check(tree, repochanged.ChangedLines{rel: every})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if got := rows(report.Findings); !slices.Equal(got, want) {
				t.Fatalf("findings = %v, want %v", got, want)
			}
			for _, finding := range report.Findings {
				if finding.Detail == "" {
					t.Errorf("line %d: a finding with no detail names no cause", finding.Line)
				}
				if finding.File != rel {
					t.Errorf("line %d: file = %q, want %q", finding.Line, finding.File, rel)
				}
			}
			if report.exitCode() != 1 {
				t.Errorf("exit = %d, want 1", report.exitCode())
			}
			if current.rule == RulePathIDZero {
				// AC-3: the finding names the accessor to use instead.
				for _, finding := range report.Findings {
					if !strings.Contains(finding.Detail, "nlri.HasPathID") {
						t.Errorf("line %d: detail names no presence accessor: %s", finding.Line, finding.Detail)
					}
				}
			}
			if !strings.Contains(report.Text(), string(current.rule)) {
				t.Errorf("text names no %s:\n%s", current.rule, report.Text())
			}
		})
	}
}

// TestRefuseByDefaultJudgesOnlyChangedLines proves the change-set scope: a
// fixture whose shapes sit on unchanged lines answers nothing and exits 0,
// and changing a default arm's return line alone makes that arm due.
func TestRefuseByDefaultJudgesOnlyChangedLines(t *testing.T) {
	source := fixture(t, "switch.go")
	tree := t.TempDir()
	rel := "internal/fixture/switch.go"
	writeSource(t, tree, rel, source)

	report, err := Check(tree, repochanged.ChangedLines{rel: {{From: 1, To: 3}}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Findings) != 0 || report.exitCode() != 0 {
		t.Fatalf("findings = %v exit %d, want none and 0", report.Findings, report.exitCode())
	}

	defaultLine := markers(t, source, "want")[0].Line
	report, err = Check(tree, repochanged.ChangedLines{rel: {{From: defaultLine + 1, To: defaultLine + 1}}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := []marked{{Line: defaultLine, Rule: RuleSwitchNilError}}
	if got := rows(report.Findings); !slices.Equal(got, want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
}

// TestRefuseByDefaultAnnotationNeedsGuard proves the escape (AC-5, AC-6): an
// annotation naming a guard the file holds is accepted, listed with that
// guard, and does not count toward the exit code; an annotation naming no
// guard, or a name the file cannot see, is refused on its own line and leaves
// the shape below it refused too.
func TestRefuseByDefaultAnnotationNeedsGuard(t *testing.T) {
	source := fixture(t, "annotated.go")
	tree := t.TempDir()
	rel := "internal/fixture/annotated.go"
	writeSource(t, tree, rel, source)

	report, err := Check(tree, repochanged.ChangedLines{rel: every})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got, want := rows(report.Findings), markers(t, source, "want"); !slices.Equal(got, want) {
		t.Fatalf("refused = %v, want %v", got, want)
	}
	if got, want := rows(report.Annotated), markers(t, source, "accepted"); !slices.Equal(got, want) {
		t.Fatalf("accepted = %v, want %v", got, want)
	}
	if report.Annotated[0].Guard != "Stop" {
		t.Errorf("guard = %q, want Stop", report.Annotated[0].Guard)
	}
	text := report.Text()
	if !strings.Contains(text, "guard: Stop") {
		t.Errorf("text does not list the accepted shape's guard:\n%s", text)
	}

	// The accepted shape alone, with the refused lines unchanged, exits 0 and
	// is still listed for the reviewer.
	accepted := report.Annotated[0].Line
	report, err = Check(tree, repochanged.ChangedLines{rel: {{From: accepted, To: accepted}}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.exitCode() != 0 || len(report.Annotated) != 1 {
		t.Fatalf("exit %d, annotated %v: want 0 and the one accepted shape", report.exitCode(), report.Annotated)
	}
}

// TestRefuseByDefaultReportListsTree proves report (AC-7) lists every shape
// in every judged file under the source roots whatever changed, and reads no
// test file, vendored module, testdata fixture or file outside the roots.
func TestRefuseByDefaultReportListsTree(t *testing.T) {
	tree := t.TempDir()
	switchSource := fixture(t, "switch.go")
	pathSource := fixture(t, "pathid.go")
	writeSource(t, tree, "internal/a/switch.go", switchSource)
	writeSource(t, tree, "pkg/b/pathid.go", pathSource)
	writeSource(t, tree, "internal/a/switch_test.go", switchSource)
	writeSource(t, tree, "internal/a/testdata/switch.go", switchSource)
	writeSource(t, tree, "vendor/x/switch.go", switchSource)
	writeSource(t, tree, "docs/switch.go", switchSource)

	report, err := Report(tree)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if report.Files != 2 {
		t.Errorf("files = %d, want 2", report.Files)
	}
	files := map[string][]marked{}
	for _, finding := range report.Findings {
		files[finding.File] = append(files[finding.File], marked{Line: finding.Line, Rule: finding.Rule})
	}
	if len(files) != 2 {
		t.Fatalf("report names files %v, want internal/a/switch.go and pkg/b/pathid.go", files)
	}
	if got, want := files["internal/a/switch.go"], markers(t, switchSource, "want"); !slices.Equal(got, want) {
		t.Errorf("internal/a/switch.go = %v, want %v", got, want)
	}
	if got, want := files["pkg/b/pathid.go"], markers(t, pathSource, "want"); !slices.Equal(got, want) {
		t.Errorf("pkg/b/pathid.go = %v, want %v", got, want)
	}
}

// TestRefuseByDefaultRefusesAFileItCannotParse proves a changed file that
// does not parse is an error, never a file with no finding.
func TestRefuseByDefaultRefusesAFileItCannotParse(t *testing.T) {
	tree := t.TempDir()
	rel := "internal/broken/broken.go"
	writeSource(t, tree, rel, "package broken\nfunc {\n")
	if _, err := Check(tree, repochanged.ChangedLines{rel: every}); err == nil {
		t.Fatal("Check over an unparsable changed file answered no error")
	}
}

// TestRefuseByDefaultReportRefusesATreeWithNoSourceRoot proves a report over
// a directory holding none of the source roots is an error, never an empty
// report that reads as a clean tree.
func TestRefuseByDefaultReportRefusesATreeWithNoSourceRoot(t *testing.T) {
	tree := t.TempDir()
	writeSource(t, tree, "docs/switch.go", fixture(t, "switch.go"))
	if _, err := Report(tree); !errors.Is(err, ErrNoSourceRoot) {
		t.Fatalf("Report over a tree with no source root: err = %v, want ErrNoSourceRoot", err)
	}
}
