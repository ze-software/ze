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
		"| 2026-08-27 | spec-a, spec-b (shared fix) | cli | symptom | fix |\n" +
		"| 2026-08-27 | none (outside a spec) | api | symptom | fix |\n"})
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
			body: journalTableHead + "| 2026-08-27 | spec-a | cli | raw | pipe | fix |\n",
			kind: "malformed-row",
		},
		{
			name: "date",
			body: journalTableHead + "| 27 August | spec-a | cli | symptom | fix |\n",
			kind: "invalid-date",
		},
		{
			name: "spec",
			body: journalTableHead + "| 2026-08-27 | future work | cli | symptom | fix |\n",
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
	suffix := " | fix |"
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
		{name: "rewritten long row", worktree: strings.Replace(long, "| fix |", "| better fix |", 1) + "\n", wantLine: 3},
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
