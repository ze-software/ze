// VALIDATES: `./le doc consistency` judges the `## Design decisions` section of
// each page a // Design: header names, and `./le commit create` refuses a
// commit that changes or removes a decision row present at its parent
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-8, AC-10).
// PREVENTS: an owner decision rewritten to fit a design, as the 2026-11-12
// pipe-chain spec rewrote docs/architecture/api/commands.md.
package docconsistency

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// The fixture tree: one source file whose header names one design page, and
// one whose header names a page that does not exist.
const (
	decisionsSource   = "internal/widget/widget.go"
	decisionsPage     = "docs/architecture/widget.md"
	danglingSource    = "internal/widget/dangling.go"
	danglingPage      = "docs/architecture/missing.md"
	decisionsSourceOK = "plan/records/widget.md"
)

// decisionRow is the one well-formed decision row of the fixture page.
const decisionRow = "| The widget answers data and the surface renders it | Rendering text in the widget | " +
	decisionsSourceOK + " | 2026-08-21 |\n"

// decisionsPageWith is the fixture page whose section holds rows.
func decisionsPageWith(rows string) string {
	return "# Widget\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n|---|---|---|---|\n" +
		rows + "\n## Wire format\n\nJSON.\n"
}

func writeDecisionsTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// findingFor answers whether report holds an error of check about file.
func findingFor(report Report, check, file string) bool {
	for _, finding := range report.Findings {
		if finding.Severity == SeverityError && finding.Check == check && finding.File == file {
			return true
		}
	}
	return false
}

// TestDesignDecisionsSectionJudged is the wiring row "`./le doc consistency`
// over a page with a malformed section". Method: Check a tree whose page holds
// a row with no Source, and a header naming a missing page; each is an error.
func TestDesignDecisionsSectionJudged(t *testing.T) {
	root := t.TempDir()
	malformed := "| The widget answers data | Rendering text in the widget |  | 2026-08-21 |\n"
	writeDecisionsTree(t, root, map[string]string{
		decisionsSource:   "// Design: " + decisionsPage + " -- the widget\n\npackage widget\n",
		danglingSource:    "// Design: " + danglingPage + " -- a page nobody wrote\n\npackage widget\n",
		decisionsPage:     decisionsPageWith(malformed),
		decisionsSourceOK: "# Widget record\n",
	})
	report := Check(root)
	if !findingFor(report, "design-decisions", decisionsPage) {
		t.Errorf("no design-decisions error for the row with no Source on %s: %#v", decisionsPage, report.Findings)
	}
	if !findingFor(report, "design-refs", danglingSource) {
		t.Errorf("no design-refs error for the header naming %s: %#v", danglingPage, report.Findings)
	}

	// Each AC-8 case in a tree of its own: the page the header names holds the
	// section as written, and a META-DECISIONS fixture and a commit exist for
	// the two Source forms that name them.
	for _, tc := range []struct {
		name  string
		page  string
		error bool
	}{
		{"a well-formed row", decisionsPageWith(decisionRow), false},
		{"no section at all", "# Widget\n\n## Wire format\n\nJSON.\n", false},
		{"the format shown in a fence", "# Widget\n\n```\n## Design decisions\n```\n", false},
		{"an empty Decision", decisionsPageWith("|  | Rendering text | " + decisionsSourceOK + " | 2026-08-21 |\n"), true},
		{"an empty It forbids", decisionsPageWith("| The widget answers data |  | " + decisionsSourceOK + " | 2026-08-21 |\n"), true},
		{"a Date not YYYY-MM-DD", decisionsPageWith("| The widget answers data | Rendering text | " + decisionsSourceOK + " | 2026/08/21 |\n"), true},
		{"a Source naming no file", decisionsPageWith("| The widget answers data | Rendering text | plan/records/absent.md | 2026-08-21 |\n"), true},
		{"a Source under tmp", decisionsPageWith("| The widget answers data | Rendering text | tmp/notes.md | 2026-08-21 |\n"), true},
		{"a Source leaving the checkout", decisionsPageWith("| The widget answers data | Rendering text | ../outside.md | 2026-08-21 |\n"), true},
		{"a META-DECISIONS section it holds", decisionsPageWith("| The widget answers data | Rendering text | `ai/META-DECISIONS.md` \"How agents work\" | 2026-08-21 |\n"), false},
		{"a META-DECISIONS section it lacks", decisionsPageWith("| The widget answers data | Rendering text | `ai/META-DECISIONS.md` \"No such rule\" | 2026-08-21 |\n"), true},
		{"META-DECISIONS with no section", decisionsPageWith("| The widget answers data | Rendering text | ai/META-DECISIONS.md | 2026-08-21 |\n"), true},
		{"a commit the repository holds", decisionsPageWith("| The widget answers data | Rendering text | COMMIT | 2026-08-21 |\n"), false},
		{"a commit the repository lacks", decisionsPageWith("| The widget answers data | Rendering text | 0123456789abcdef0123456789abcdef01234567 | 2026-08-21 |\n"), true},
		{"a row of three cells", decisionsPageWith("| The widget answers data | Rendering text | " + decisionsSourceOK + " |\n"), true},
		{"the heading twice", decisionsPageWith(decisionRow) + "\n## Design decisions\n\nMore.\n", true},
		{"a heading with no row", decisionsPageWith(""), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			commit := fixtureCommit(t, caseRoot)
			writeDecisionsTree(t, caseRoot, map[string]string{
				decisionsSource:        "// Design: " + decisionsPage + " -- the widget\n\npackage widget\n",
				decisionsPage:          strings.ReplaceAll(tc.page, "COMMIT", commit),
				decisionsSourceOK:      "# Widget record\n",
				"ai/META-DECISIONS.md": "# Meta\n\n## How agents work\n\n- A rule.\n",
			})
			report := Check(caseRoot)
			if got := findingFor(report, "design-decisions", decisionsPage); got != tc.error {
				t.Errorf("design-decisions error = %v, want %v: %#v", got, tc.error, report.Findings)
			}
		})
	}
}

// fixtureCommit makes root a repository holding one empty commit and answers
// its id, so a Source may name a commit the repository holds.
func fixtureCommit(t *testing.T, root string) string {
	t.Helper()
	git := func(args ...string) string {
		command := exec.CommandContext(t.Context(), "git", args...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "--allow-empty", "-m", "fixture")
	return git("rev-parse", "HEAD")
}

// TestCommitRefusesMalformedAddedDecisionRow is AC-9. Method: judge a commit
// that adds a row with no Source to a well-formed section; it is refused
// naming the page, the row and the cell. A commit that edits the page
// elsewhere and leaves an already malformed section alone is accepted: the
// check judges what the commit adds, never what the parent held.
func TestCommitRefusesMalformedAddedDecisionRow(t *testing.T) {
	root := t.TempDir()
	writeDecisionsTree(t, root, map[string]string{decisionsSourceOK: "# Widget record\n"})
	check := decisionRowsCheck(t)
	malformed := "| A second decision | Something else |  | 2026-11-19 |\n"

	refusal, err := check.Judge(pageChange(root, decisionsPageWith(decisionRow), decisionsPageWith(decisionRow+malformed)))
	if err != nil {
		t.Fatalf("the check could not judge: %v", err)
	}
	for _, want := range []string{decisionsPage, "row 2", "Source", "./le doc consistency"} {
		if !strings.Contains(refusal, want) {
			t.Errorf("the refusal of an added row with no Source does not name %q: %q", want, refusal)
		}
	}

	held := decisionsPageWith(malformed)
	refusal, err = check.Judge(pageChange(root, held, strings.Replace(held, "JSON.", "JSON, framed.", 1)))
	if err != nil {
		t.Fatalf("the check could not judge: %v", err)
	}
	if refusal != "" {
		t.Errorf("a commit that leaves a malformed section's rows alone was refused: %q", refusal)
	}
}

// decisionRowsCheck answers the registered change-set check this package
// owns, by its name, so the test runs what `./le commit create` runs.
func decisionRowsCheck(t *testing.T) changecheck.Check {
	t.Helper()
	for _, check := range changecheck.Registered() {
		if check.Name == name+" "+checkVerb {
			return check
		}
	}
	t.Fatalf("no change-set check named %q is registered", name+" "+checkVerb)
	return changecheck.Check{}
}

// pageChange is a commit carrying the fixture page, before and after.
func pageChange(root, before, after string) changecheck.Change {
	side := func(text string) changecheck.Side {
		return func(path string) (string, bool, error) {
			if path != decisionsPage {
				return "", false, nil
			}
			return text, true, nil
		}
	}
	return changecheck.Change{Root: root, Paths: []string{decisionsPage}, Before: side(before), After: side(after)}
}

// TestCommitRefusesChangedDecisionRow is the wiring row "`./le commit create`
// removing a decision row". Method: judge a commit that removes the row and
// one that rewrites it; each is refused naming the page and the decision's
// date. A commit that adds a row is accepted.
func TestCommitRefusesChangedDecisionRow(t *testing.T) {
	root := t.TempDir()
	writeDecisionsTree(t, root, map[string]string{decisionsSourceOK: "# Widget record\n"})
	check := decisionRowsCheck(t)
	before := decisionsPageWith(decisionRow)
	rewritten := strings.Replace(decisionRow, "the surface renders it", "the widget renders it too", 1)
	added := "| A second decision | Something else | " + decisionsSourceOK + " | 2026-11-19 |\n"

	for label, after := range map[string]string{
		"removed":   decisionsPageWith(""),
		"rewritten": decisionsPageWith(rewritten),
	} {
		refusal, err := check.Judge(pageChange(root, before, after))
		if err != nil {
			t.Fatalf("%s: the check could not judge: %v", label, err)
		}
		if !strings.Contains(refusal, decisionsPage) || !strings.Contains(refusal, "2026-08-21") {
			t.Errorf("%s: a commit that changes the decision of 2026-08-21 was not refused naming it: %q", label, refusal)
		}
	}

	refusal, err := check.Judge(pageChange(root, before, decisionsPageWith(decisionRow+added)))
	if err != nil {
		t.Fatalf("added: the check could not judge: %v", err)
	}
	if refusal != "" {
		t.Errorf("a commit that only adds a decision row was refused: %q", refusal)
	}
}
