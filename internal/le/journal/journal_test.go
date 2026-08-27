package journal

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

const journalTableHead = "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"

// TestCheckReportsRecurringClassesInStableOrder validates that one-row classes
// stay silent and recurring classes include their full date span in name order.
// It prevents map iteration from changing the report between runs.
func TestCheckReportsRecurringClassesInStableOrder(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/zebra.md": journalTableHead +
			"| 2026-08-20 | z | config | later | fix |\n" +
			"| 2026-08-01 | z | config | earlier | fix |\n",
		"plan/journal/alpha.md": journalTableHead +
			"| 2026-07-15 | a | reactor | first | fix |\n" +
			"| 2026-07-16 | a | reactor | second | fix |\n",
		"plan/journal/singleton.md": journalTableHead +
			"| 2026-08-21 | one | cli | once | fix |\n",
	})

	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := "alpha: 2 rows, 1d span (2026-07-15 .. 2026-07-16)\n" +
		"zebra: 2 rows, 19d span (2026-08-01 .. 2026-08-20)\n"
	if got := report.Text(); got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
	if len(report.Classes) != 2 {
		t.Fatalf("Classes = %d, want 2: %#v", len(report.Classes), report.Classes)
	}
	if report.Classes[0].Name != "alpha" || report.Classes[1].Name != "zebra" {
		t.Fatalf("class order = %#v, want alpha then zebra", report.Classes)
	}
}

// TestCheckReadsHeadAndNamesAWorktreeOnlyClass validates the committed-tree
// boundary. It prevents another session's uncommitted row from changing the
// recurrence count without the report naming that omission.
func TestCheckReadsHeadAndNamesAWorktreeOnlyClass(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/committed.md": journalTableHead +
			"| 2026-08-01 | a | cli | first | fix |\n" +
			"| 2026-08-03 | b | cli | second | fix |\n",
	})
	writeJournalFile(t, tree, "plan/journal/worktree-only.md", journalTableHead+
		"| 2026-08-04 | c | cli | first | fix |\n"+
		"| 2026-08-05 | d | cli | second | fix |\n")

	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got, want := report.Text(), "committed: 2 rows, 2d span (2026-08-01 .. 2026-08-03)\n"; got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
	wantUnread := []string{"plan/journal/worktree-only.md"}
	if !reflect.DeepEqual(report.Unread, wantUnread) {
		t.Fatalf("Unread = %#v, want %#v", report.Unread, wantUnread)
	}

	var errOut bytes.Buffer
	_, code := Run(tree, &errOut)
	if code != 0 {
		t.Fatalf("Run code = %d, want 0: %s", code, errOut.String())
	}
	wantWarning := "NOT AT HEAD: plan/journal/worktree-only.md is on disk and not committed, so its rows are not in the counts above\n"
	if got := errOut.String(); got != wantWarning {
		t.Fatalf("stderr = %q, want %q", got, wantWarning)
	}
}

// TestRunRefusesMalformedPipeRows validates both malformed pipe shapes. It
// prevents a missing leading pipe or an extra raw pipe from reducing recurrence.
func TestRunRefusesMalformedPipeRows(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/bad-pipes.md": journalTableHead +
			"2026-08-01 | a | cli | missing leading pipe | fix |\n" +
			"| 2026-08-02 | b | cli | raw | pipe | fix |\n",
	})

	var errOut bytes.Buffer
	report, code := Run(tree, &errOut)
	if code != 1 {
		t.Fatalf("Run code = %d, want 1", code)
	}
	if len(report.Problems) != 1 || report.Problems[0].Rows != 2 {
		t.Fatalf("Problems = %#v, want one two-row malformed problem", report.Problems)
	}
	want := "MALFORMED: plan/journal/bad-pipes.md: 2 row(s) do not hold the five cells | Date | Spec | Surface | Symptom | Fix |\n"
	if got := errOut.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if got := report.Text(); got != "" {
		t.Fatalf("malformed class reached stdout: %q", got)
	}
}

// TestRunRefusesAnUnreadableHead validates that a failed git read is not an
// empty journal. It prevents an absent verdict from using exit code zero.
func TestRunRefusesAnUnreadableHead(t *testing.T) {
	tree := t.TempDir()
	var errOut bytes.Buffer
	report, code := Run(tree, &errOut)
	if code != 2 {
		t.Fatalf("Run code = %d, want 2", code)
	}
	if len(report.Classes) != 0 {
		t.Fatalf("unreadable repository returned classes: %#v", report.Classes)
	}
	if !strings.HasPrefix(errOut.String(), "journal: git ls-tree HEAD -- plan/journal failed:") {
		t.Fatalf("stderr does not name the failed HEAD read: %q", errOut.String())
	}
}

// TestRunRefusesAnUnparseableDate validates every row before the recurrence
// threshold. It prevents a malformed singleton from hiding below that threshold.
func TestRunRefusesAnUnparseableDate(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/undated.md": journalTableHead +
			"| - | a | cli | no date | fix |\n",
	})

	var errOut bytes.Buffer
	report, code := Run(tree, &errOut)
	if code != 1 {
		t.Fatalf("Run code = %d, want 1", code)
	}
	want := "UNPARSEABLE DATE: plan/journal/undated.md: '-' -- every row needs a YYYY-MM-DD Date, which is what the span is computed from\n"
	if got := errOut.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if len(report.Problems) != 1 || report.Problems[0].Kind != ProblemUnparseableDate {
		t.Fatalf("Problems = %#v, want one unparseable-date problem", report.Problems)
	}
}

// TestRunKeepsAnEmptyJournalEmpty validates the producer's empty-output
// contract. It prevents a summary header or an empty table from appearing.
func TestRunKeepsAnEmptyJournalEmpty(t *testing.T) {
	tree := journalFixture(t, map[string]string{"README.md": "fixture\n"})
	var errOut bytes.Buffer
	report, code := Run(tree, &errOut)
	if code != 0 {
		t.Fatalf("Run code = %d, want 0: %s", code, errOut.String())
	}
	if got := report.Text(); got != "" {
		t.Fatalf("Text() = %q, want empty", got)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

// TestActionsPreserveTheProducerRegistryRow validates the gate identity,
// purpose, and writes flag from one table.
func TestActionsPreserveTheProducerRegistryRow(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 1 {
		t.Fatalf("Actions = %d, want 1: %#v", len(list.Actions), list.Actions)
	}
	action := list.Actions[0]
	if action.Gate != "ze-journal-report" {
		t.Fatalf("gate = %q, want ze-journal-report", action.Gate)
	}
	if action.Writes {
		t.Fatal("ze-journal-report is marked as writing")
	}
	if action.Why != journalWhy {
		t.Fatalf("why = %q, want %q", action.Why, journalWhy)
	}
}

// TestAnswerReturnsTheStructuredReport validates the command boundary. It
// prevents the native command from returning preformatted report text.
func TestAnswerReturnsTheStructuredReport(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/recurring.md": journalTableHead +
			"| 2026-08-01 | a | cli | first | fix |\n" +
			"| 2026-08-02 | b | cli | second | fix |\n",
	})
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	payload, code := Answer([]string{"report"})
	if code != 0 {
		t.Fatalf("Answer code = %d, want 0", code)
	}
	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("Answer payload = %T, want Report", payload)
	}
	if len(report.Classes) != 1 || report.Classes[0].Name != "recurring" {
		t.Fatalf("Answer report = %#v, want recurring class", report)
	}
}

func journalFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	tree := t.TempDir()
	for rel, body := range files {
		writeJournalFile(t, tree, rel, body)
	}
	journalGit(t, tree, "init", "--quiet", "--initial-branch=main")
	journalGit(t, tree, "config", "user.email", "test@example.com")
	journalGit(t, tree, "config", "user.name", "Ze Test")
	journalGit(t, tree, "config", "commit.gpgsign", "false")
	journalGit(t, tree, "add", "--all")
	journalGit(t, tree, "commit", "--quiet", "--message=seed journal")
	return tree
}

func writeJournalFile(t *testing.T, tree, rel, body string) {
	t.Helper()
	path := filepath.Join(tree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixture file: %v", err)
	}
}

func journalGit(t *testing.T, tree string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", tree}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", args[0], err, output)
	}
}
