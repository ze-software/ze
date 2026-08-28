// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the working-tree report
// is called as a function, answers structured data, and answers 1 only when a
// ceiling was asked for and exceeded.
// PREVENTS: a report whose first-match prefix order files ai/rules/ under
// ai-docs or native development tools under the generic internal area.

package workingtree

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTheFirstMatchingPrefixWins(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"ai/rules/writing.md", "rules"},
		{"ai/INDEX.md", "ai-docs"},
		{"plan/journal/x.md", "journal"},
		{"plan/audits/x.md", "audits"},
		{"plan/spec-x.md", "specs"},
		{"docs/x.md", "docs"},
		{"test/ui/x.ci", "tests"},
		{"internal/le/evidence/docker_run.go", "evidence-tools"},
		{"internal/le/commit/actions.go", "tooling"},
		{".golangci.yml", "build"},
		{"pkg/plugin/x.go", "plugin-sdk"},
		{"internal/component/bgp/x.go", "bgp"},
		{"internal/component/plugin/x.go", "plugin-engine"},
		{"internal/component/command/x.go", "cli-command"},
		{"internal/core/x.go", "internal"},
		{"cmd/ze/x.go", "cmd"},
		{"README.md", "other"},
	}
	for _, tc := range cases {
		if got := areaOf(tc.path); got != tc.want {
			t.Errorf("AreaOf(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestPorcelainCarriesTheNewNameOfARename(t *testing.T) {
	paths := parsePorcelain("R  docs/old.md -> docs/new.md\n")
	if len(paths) != 1 || paths[0] != "docs/new.md" {
		t.Fatalf("ParsePorcelain answered %q, want the new name", paths)
	}
}

func TestPorcelainSkipsBlankLines(t *testing.T) {
	paths := parsePorcelain(" M a.go\n\n?? b.go\n")
	if len(paths) != 2 {
		t.Fatalf("ParsePorcelain answered %q, want two paths", paths)
	}
}

func TestPorcelainStripsTheQuotesGitAddsToAnUnusualName(t *testing.T) {
	paths := parsePorcelain("?? \"docs/a b.md\"\n")
	if len(paths) != 1 || paths[0] != "docs/a b.md" {
		t.Fatalf("ParsePorcelain answered %q", paths)
	}
}

func TestGroupsAreOrderedByCountThenName(t *testing.T) {
	report := Group([]string{
		"docs/a.md", "docs/b.md",
		"cmd/ze/x.go",
		"ai/rules/y.md", "ai/rules/z.md", "ai/rules/w.md",
	}, 0)

	if report.Paths != 6 {
		t.Errorf("Paths is %d, want 6", report.Paths)
	}
	want := []string{"rules", "docs", "cmd"}
	for i, area := range want {
		if report.Areas[i].Area != area {
			t.Fatalf("area %d is %q, want %q: %+v", i, report.Areas[i].Area, area, report.Areas)
		}
	}
}

func TestTwoAreasOfEqualSizeAreOrderedByName(t *testing.T) {
	report := Group([]string{"docs/a.md", "cmd/x.go"}, 0)

	if report.Areas[0].Area != "cmd" || report.Areas[1].Area != "docs" {
		t.Fatalf("equal areas are not in name order: %+v", report.Areas)
	}
}

func TestFilesWithinAnAreaAreSorted(t *testing.T) {
	report := Group([]string{"docs/b.md", "docs/a.md"}, 0)

	if report.Areas[0].Files[0] != "docs/a.md" {
		t.Fatalf("the files of an area are not sorted: %+v", report.Areas[0].Files)
	}
}

// The page names four files before it summarizes the rest, so four and five are
// the two sides of that boundary.
func TestFourFilesAreAllNamedAndFiveAreNot(t *testing.T) {
	four := Group([]string{"docs/a.md", "docs/b.md", "docs/c.md", "docs/d.md"}, 0).Text()
	if strings.Contains(four, "more") {
		t.Errorf("four files were summarized: %q", four)
	}

	five := Group([]string{"docs/a.md", "docs/b.md", "docs/c.md", "docs/d.md", "docs/e.md"}, 0).Text()
	if !strings.Contains(five, "+1 more") {
		t.Errorf("the fifth file is not accounted for: %q", five)
	}
}

func TestACleanTreeSaysSo(t *testing.T) {
	report := Group(nil, 0)

	if report.Paths != 0 || len(report.Areas) != 0 {
		t.Fatalf("a clean tree reported %+v", report)
	}
	if report.Text() != "working tree: clean\n" {
		t.Errorf("a clean tree renders %q", report.Text())
	}
}

func TestOneAreaGetsNoAdvice(t *testing.T) {
	text := Group([]string{"docs/a.md"}, 0).Text()

	if strings.Contains(text, "More than one area") {
		t.Errorf("a single-area tree was advised to land a chunk: %q", text)
	}
}

func TestTwoAreasGetTheAdvice(t *testing.T) {
	text := Group([]string{"docs/a.md", "cmd/x.go"}, 0).Text()

	if !strings.Contains(text, "More than one area is in flight") {
		t.Errorf("a two-area tree got no advice: %q", text)
	}
}

// The ceiling is the numeric input of this tool. 0 means advisory, so the
// boundary runs 0, the exact ceiling, and one past it.
func TestTheCeilingIsAdvisoryAtZero(t *testing.T) {
	report := Group([]string{"docs/a.md", "cmd/x.go", ".golangci.yml"}, 0)

	if report.Exceeded() {
		t.Error("a zero ceiling made three areas a failure")
	}
	if strings.Contains(report.Text(), "exceeds") {
		t.Errorf("a zero ceiling rendered a verdict: %q", report.Text())
	}
}

func TestAreasEqualToTheCeilingPass(t *testing.T) {
	report := Group([]string{"docs/a.md", "cmd/x.go"}, 2)

	if report.Exceeded() {
		t.Error("two areas exceeded a ceiling of two")
	}
}

func TestOneAreaPastTheCeilingFails(t *testing.T) {
	report := Group([]string{"docs/a.md", "cmd/x.go", ".golangci.yml"}, 2)

	if !report.Exceeded() {
		t.Fatal("three areas did not exceed a ceiling of two")
	}
	if !strings.Contains(report.Text(), "3 areas exceeds max-areas 2") {
		t.Errorf("the verdict is not on the page: %q", report.Text())
	}
}

func TestACleanTreeCannotExceedACeiling(t *testing.T) {
	if Group(nil, 1).Exceeded() {
		t.Error("a clean tree exceeded a ceiling")
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Group([]string{"docs/a.md"}, 3))
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"paths"`, `"areas"`, `"area"`, `"files"`, `"max-areas"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestAnswerRefusesAValueWithNoKeyword(t *testing.T) {
	payload, code := Answer([]string{"3"})

	if payload != nil || code != 1 {
		t.Fatalf("a bare value was accepted: payload=%v code=%d", payload, code)
	}
}

func TestAnswerRefusesAKeywordWithNoValue(t *testing.T) {
	payload, code := Answer([]string{"max-areas"})

	if payload != nil || code != 1 {
		t.Fatalf("a keyword with no value was accepted: payload=%v code=%d", payload, code)
	}
}

func TestAnswerRefusesACeilingThatIsNotANumber(t *testing.T) {
	payload, code := Answer([]string{"max-areas", "many"})

	if payload != nil || code != 1 {
		t.Fatalf("a non-numeric ceiling was accepted: payload=%v code=%d", payload, code)
	}
}

func TestAnswerRefusesANegativeCeiling(t *testing.T) {
	payload, code := Answer([]string{"max-areas", "-1"})

	if payload != nil || code != 1 {
		t.Fatalf("a negative ceiling was accepted: payload=%v code=%d", payload, code)
	}
}

func TestAnswerReadsTheTreeItIsRunIn(t *testing.T) {
	payload, code := Answer(nil)

	if code != 0 {
		t.Fatalf("the advisory answered %d over this checkout", code)
	}
	if _, ok := payload.(Report); !ok {
		t.Fatalf("Answer did not answer a Report: %T", payload)
	}
}
