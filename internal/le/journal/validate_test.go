package journal

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateFileAcceptsCanonicalJournalShard(t *testing.T) {
	root := t.TempDir()
	path := "plan/journal/nested/valid.md"
	writeJournalFile(t, root, path, journalTableHead+
		"| 2026-08-27 | spec-a, spec-b (shared fix) | cli | symptom | fix |\n"+
		"| 2026-08-27 | none (outside a spec) | api | symptom | fix |\n")
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
			root := t.TempDir()
			path := "plan/journal/invalid.md"
			writeJournalFile(t, root, path, test.body)
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
