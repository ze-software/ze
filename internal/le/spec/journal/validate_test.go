package specjournal

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateFileAcceptsCanonicalJournalShard(t *testing.T) {
	path := "plan/journal/nested/valid.md"
	root := journalFixture(t, map[string]string{path: journalTableHead +
		"| 2026-08-27 | spec-a, spec-b (shared fix) | cli | symptom | fixed: fix |\n" +
		"| 2026-08-27 | none (outside a spec) | api | symptom | fixed: fix |\n"})
	report, err := ValidateFile(root, path)
	if err != nil || report.ExitCode() != 0 || report.Rows != 2 || len(report.Problems) != 0 {
		t.Fatalf("ValidateFile(valid) = %#v, %v", report, err)
	}
	if !strings.Contains(report.Text(), "is valid (2 row(s))") {
		t.Fatalf("valid Text = %q", report.Text())
	}
}

func TestValidateFileNamesEveryMalformedContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		kind string
	}{
		{
			name: "header",
			body: "| When | Spec | Surface | Symptom | Fix |\n|---|---|---|---|---|\n",
			kind: "missing-header",
		},
		{
			name: "cell count",
			body: journalTableHead + "| 2026-08-27 | spec-a | cli | raw | pipe | fixed: fix |\n",
			kind: "malformed-row",
		},
		{
			name: "date",
			body: journalTableHead + "| 27 August | spec-a | cli | symptom | fixed: fix |\n",
			kind: "invalid-date",
		},
		{
			name: "spec",
			body: journalTableHead + "| 2026-08-27 | future work | cli | symptom | fixed: fix |\n",
			kind: "unreadable-spec",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := "plan/journal/invalid.md"
			root := journalFixture(t, map[string]string{path: test.body})
			report, err := ValidateFile(root, path)
			if err != nil || report.ExitCode() != 1 {
				t.Fatalf("ValidateFile = %#v, %v", report, err)
			}
			kinds := make([]string, len(report.Problems))
			for index, problem := range report.Problems {
				kinds[index] = problem.Kind
			}
			if !slices.Contains(kinds, test.kind) {
				t.Fatalf("problem kinds = %q, want %q; text %q", kinds, test.kind, report.Text())
			}
		})
	}
}

func TestValidateFileRefusesUnsafeAndNonJournalPaths(t *testing.T) {
	root := t.TempDir()
	writeJournalFile(t, root, "docs/not-journal.md", journalTableHead)
	writeJournalFile(t, root, "plan/journal/README.md", journalTableHead)
	writeJournalFile(t, root, "plan/journal/valid.md", journalTableHead)
	outside := filepath.Join(filepath.Dir(root), "outside-journal.md")
	if err := os.WriteFile(outside, []byte(journalTableHead), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "plan", "journal", "linked.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"", "docs/not-journal.md", "plan/journal/README.md", outside,
		"../outside-journal.md", "plan/journal/linked.md",
	} {
		if report, err := ValidateFile(root, path); err == nil {
			t.Errorf("ValidateFile(%q) = %#v, want path refusal", path, report)
		}
	}
}

// journalRowOf returns one valid journal row exactly chars characters long.
func journalRowOf(t *testing.T, date string, chars int) string {
	t.Helper()
	prefix := "| " + date + " | - | cli | "
	suffix := " | fixed: fix |"
	pad := chars - len(prefix) - len(suffix)
	if pad < 1 {
		t.Fatalf("row of %d characters is shorter than its cells", chars)
	}
	return prefix + strings.Repeat("s", pad) + suffix
}

// TestValidateFileCapsAddedRowLength validates the row cap against HEAD. It
// proves a new or rewritten row over RowCharsMax is refused with its line and
// length, that a row of exactly RowCharsMax passes, and that a long row HEAD
// already carries is never flagged, even when it is only re-spaced.
func TestValidateFileCapsAddedRowLength(t *testing.T) {
	t.Parallel()
	long := journalRowOf(t, "2026-08-01", 1200)
	tests := []struct {
		name     string
		worktree string
		wantLine int
	}{
		{name: "new long row", worktree: long + "\n" + journalRowOf(t, "2026-08-02", RowCharsMax+1) + "\n", wantLine: 4},
		{name: "new row at the cap", worktree: long + "\n" + journalRowOf(t, "2026-08-02", RowCharsMax) + "\n"},
		{name: "unchanged long head row", worktree: long + "\n"},
		{name: "re-spaced long head row", worktree: strings.Replace(long, "| - |", "|   -   |", 1) + "\n"},
		{name: "rewritten long row", worktree: strings.Replace(long, "| fixed: fix |", "| fixed: better fix |", 1) + "\n", wantLine: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := "plan/journal/capped.md"
			root := journalFixture(t, map[string]string{path: journalTableHead + long + "\n"})
			writeJournalFile(t, root, path, journalTableHead+test.worktree)
			report, err := ValidateFile(root, path)
			if err != nil {
				t.Fatalf("ValidateFile: %v", err)
			}
			var flagged []ValidationProblem
			for _, problem := range report.Problems {
				if problem.Kind == "row-too-long" {
					flagged = append(flagged, problem)
				}
			}
			if test.wantLine == 0 {
				if len(flagged) != 0 || report.ExitCode() != 0 {
					t.Fatalf("problems = %#v, want none", report.Problems)
				}
				return
			}
			if len(flagged) != 1 || flagged[0].Line != test.wantLine || report.ExitCode() != 1 {
				t.Fatalf("problems = %#v, want one row-too-long at line %d", report.Problems, test.wantLine)
			}
			if !strings.Contains(flagged[0].Message, "ai/rationale/") {
				t.Fatalf("message %q does not say where the detail goes", flagged[0].Message)
			}
		})
	}
}

// TestValidateFileCapsEveryRowOfANewShard validates that a class file HEAD
// does not carry has every row measured, so a new class cannot open long.
func TestValidateFileCapsEveryRowOfANewShard(t *testing.T) {
	root := journalFixture(t, map[string]string{"plan/journal/seed.md": journalTableHead})
	path := "plan/journal/new.md"
	writeJournalFile(t, root, path, journalTableHead+journalRowOf(t, "2026-08-01", 700)+"\n")
	report, err := ValidateFile(root, path)
	if err != nil || len(report.Problems) != 1 || report.Problems[0].Kind != "row-too-long" {
		t.Fatalf("ValidateFile(new shard) = %#v, %v", report, err)
	}
	if !strings.Contains(report.Problems[0].Message, "700 characters") {
		t.Fatalf("message %q does not name the length", report.Problems[0].Message)
	}
}

// validationKinds returns the problem kinds of a report, in order.
func validationKinds(report ValidationReport) []string {
	kinds := make([]string, len(report.Problems))
	for index, problem := range report.Problems {
		kinds[index] = problem.Kind
	}
	return kinds
}

// TestValidateFileRefusesUnmarkedAndBadCaptures validates AC-4 to AC-8 through
// the working-tree reader: an unmarked row, new or held at HEAD, and every
// capture refusal, each resolved against the working tree.
func TestValidateFileRefusesUnmarkedAndBadCaptures(t *testing.T) {
	path := "plan/journal/edited.md"
	held := "| 2026-08-01 | - | cli | held | not fixed |\n"
	tests := []struct {
		name     string
		head     string
		worktree string
		kind     string
	}{
		{name: "new unmarked row", worktree: journalTableHead + "| 2026-08-02 | - | cli | new | fix |\n", kind: ProblemUnmarkedFix},
		{name: "held unmarked row", head: journalTableHead + held, worktree: journalTableHead + held, kind: ProblemUnmarkedFix},
		{name: "malformed capture", worktree: "Captured: 2026-08-12 policy x\n" + journalTableHead, kind: ProblemCaptureMalformed},
		{name: "future capture", worktree: "Captured: 2999-01-01 rule area/directives/point\n" + journalTableHead, kind: ProblemCaptureMalformed},
		{name: "unresolved check", worktree: "Captured: 2026-08-12 check internal/guard.Absent proof internal/guard.TestGuard\n" + journalTableHead, kind: ProblemCaptureUnresolved},
		{name: "unresolved point", worktree: "Captured: 2026-08-12 rule area/directives/absent\n" + journalTableHead, kind: ProblemCaptureUnresolved},
		{name: "duplicated capture", worktree: checkCapture + ruleCapture + journalTableHead, kind: ProblemCaptureDuplicated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			if test.head != "" {
				files[path] = test.head
			}
			root := journalFixture(t, captureFiles(files))
			writeJournalFile(t, root, path, test.worktree)
			report, err := ValidateFile(root, path)
			if err != nil {
				t.Fatalf("ValidateFile: %v", err)
			}
			if kinds := validationKinds(report); !slices.Contains(kinds, test.kind) || report.ExitCode() != 1 {
				t.Fatalf("problem kinds = %q, want %q; text %q", kinds, test.kind, report.Text())
			}
		})
	}
	root := journalFixture(t, captureFiles(nil))
	writeJournalFile(t, root, "internal/guard/later.go", "package guard\n\nfunc Later() {}\n")
	writeJournalFile(t, root, path, "Captured: 2026-08-12 check internal/guard.Later proof internal/guard.TestGuard\n"+
		journalTableHead+"| 2026-08-02 | - | cli | new | fixed: fix |\n")
	report, err := ValidateFile(root, path)
	if err != nil || report.ExitCode() != 0 {
		t.Fatalf("a check in the working tree resolves for validate: %#v, %v", report, err)
	}
}

// TestValidateFileHoldsARowWhoseOnlyChangeIsTheMarker validates AC-13 and the
// row-length boundary: an over-cap HEAD row whose only change is an exact added
// marker is held, any other change or a misspelled marker is measured, and a
// flipped or dropped marker still pairs, with the drop raising unmarked-fix.
func TestValidateFileHoldsARowWhoseOnlyChangeIsTheMarker(t *testing.T) {
	t.Parallel()
	marked := journalRowOf(t, "2026-08-01", 1200)
	unmarked := strings.Replace(marked, "| fixed: fix |", "| fix |", 1)
	tests := []struct {
		name     string
		head     string
		worktree string
		tooLong  bool
		unmarked bool
	}{
		{name: "fixed added", head: unmarked, worktree: marked},
		{name: "open added", head: unmarked, worktree: strings.Replace(unmarked, "| fix |", "| open: fix |", 1)},
		{name: "marker and prose", head: unmarked, worktree: strings.Replace(unmarked, "| fix |", "| fixed: fix more |", 1), tooLong: true},
		{name: "capital marker", head: unmarked, worktree: strings.Replace(unmarked, "| fix |", "| Fixed: fix |", 1), tooLong: true, unmarked: true},
		{name: "marker without space", head: unmarked, worktree: strings.Replace(unmarked, "| fix |", "| fixed:fix |", 1), tooLong: true, unmarked: true},
		{name: "flipped marker", head: marked, worktree: strings.Replace(marked, "| fixed: fix |", "| open: fix |", 1)},
		{name: "dropped marker", head: marked, worktree: unmarked, unmarked: true},
		{name: "new over-cap row", head: marked, worktree: marked + "\n" + journalRowOf(t, "2026-08-02", RowCharsMax+1), tooLong: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := "plan/journal/held.md"
			root := journalFixture(t, map[string]string{path: journalTableHead + test.head + "\n"})
			writeJournalFile(t, root, path, journalTableHead+test.worktree+"\n")
			report, err := ValidateFile(root, path)
			if err != nil {
				t.Fatalf("ValidateFile: %v", err)
			}
			kinds := validationKinds(report)
			if slices.Contains(kinds, "row-too-long") != test.tooLong ||
				slices.Contains(kinds, ProblemUnmarkedFix) != test.unmarked {
				t.Fatalf("problem kinds = %q, want row-too-long %v, unmarked-fix %v", kinds, test.tooLong, test.unmarked)
			}
		})
	}
}

// TestValidateFileHoldsARowMovedFromAnotherClass validates AC-15 and AC-16: a
// merge or a split moves an over-cap HEAD row unchanged into another class
// file, and that destination's validation pairs it with the source's HEAD row
// instead of measuring it as added. A moved row whose prose changed on the way
// is measured, and so is an over-cap row no HEAD class holds.
func TestValidateFileHoldsARowMovedFromAnotherClass(t *testing.T) {
	t.Parallel()
	moved := journalRowOf(t, "2026-08-01", 1200)
	tests := []struct {
		name    string
		target  string
		tooLong bool
	}{
		{name: "moved unchanged", target: moved},
		{name: "moved with a changed marker", target: strings.Replace(moved, "| fixed: fix |", "| open: fix |", 1)},
		{name: "moved and rewritten", target: strings.Replace(moved, "| fixed: fix |", "| fixed: fix more |", 1), tooLong: true},
		{name: "held nowhere", target: journalRowOf(t, "2026-08-02", 1200), tooLong: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source, target := "plan/journal/source.md", "plan/journal/target.md"
			root := journalFixture(t, map[string]string{
				source: journalTableHead + moved + "\n",
				target: journalTableHead + journalRowOf(t, "2026-07-01", 100) + "\n",
			})
			writeJournalFile(t, root, source, journalTableHead)
			writeJournalFile(t, root, target, journalTableHead+journalRowOf(t, "2026-07-01", 100)+"\n"+test.target+"\n")
			report, err := ValidateFile(root, target)
			if err != nil {
				t.Fatalf("ValidateFile: %v", err)
			}
			if kinds := validationKinds(report); slices.Contains(kinds, "row-too-long") != test.tooLong {
				t.Fatalf("problem kinds = %q, want row-too-long %v", kinds, test.tooLong)
			}
		})
	}
}

// TestValidateFileJudgesAFileInARepositoryWithNoCommit validates that an
// unborn HEAD, which holds no class file, is judged as an empty HEAD and never
// as a failed read: ValidateFile answers the file's own verdict, accepting a
// marked row and refusing an unmarked one. The post-write hook's probe tree is
// such a repository. PREVENTS: the moved-row pairing running git ls-tree over
// a HEAD that does not resolve and refusing every file it validates.
func TestValidateFileJudgesAFileInARepositoryWithNoCommit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		fix  string
		exit int
	}{
		{name: "marked", fix: "fixed: fix", exit: 0},
		{name: "unmarked", fix: "fix", exit: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := "plan/journal/unborn.md"
			root := t.TempDir()
			writeJournalFile(t, root, path, journalTableHead+"| 2026-08-22 | spec-x | hooks | symptom | "+test.fix+" |\n")
			journalGit(t, root, "init", "--quiet", "--initial-branch=main")
			report, err := ValidateFile(root, path)
			if err != nil {
				t.Fatalf("ValidateFile over an unborn HEAD: %v", err)
			}
			if report.ExitCode() != test.exit {
				t.Fatalf("ValidateFile exit = %d, want %d: %#v", report.ExitCode(), test.exit, report)
			}
		})
	}
}
