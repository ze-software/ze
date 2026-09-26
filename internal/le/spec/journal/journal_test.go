package specjournal

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// TestActionsPublishTheNativeRegistryRows validates both actions from one table.
func TestActionsPublishTheNativeRegistryRows(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 2 {
		t.Fatalf("Actions = %d, want 2: %#v", len(list.Actions), list.Actions)
	}
	action := list.Actions[0]
	if action.Verb != "report" {
		t.Fatalf("verb = %q, want report", action.Verb)
	}
	if action.Writes {
		t.Fatal("journal report is marked as writing")
	}
	if action.Why != journalWhy {
		t.Fatalf("why = %q, want %q", action.Why, journalWhy)
	}
	validate := list.Actions[1]
	if validate.Verb != "validate" || validate.Writes ||
		validate.Why != "validate one edited plan/journal class file's header, rows, dates, Spec keys, "+
			"and the 600-character cap on a row added or rewritten against HEAD" {
		t.Fatalf("validate action = %#v", validate)
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

func TestValidateActionUsesClosedFileKeywordGrammar(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/valid.md": journalTableHead +
			"| 2026-08-27 | spec-a | cli | symptom | fix |\n",
	})
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	payload, code := Answer([]string{"validate", "file", "plan/journal/valid.md"})
	report, ok := payload.(ValidationReport)
	if code != 0 || !ok || report.Rows != 1 {
		t.Fatalf("validate action = %T %#v, code %d", payload, payload, code)
	}
	if _, code := Answer([]string{"validate", "plan/journal/valid.md"}); code != 2 {
		t.Fatalf("identifier-first grammar code = %d, want 2", code)
	}
	if _, code := Answer([]string{"validate", "file"}); code != 2 {
		t.Fatalf("missing file value code = %d, want 2", code)
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

func TestSpecStemsReadsEveryCanonicalCellShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cell  string
		want  []string
		valid bool
	}{
		{cell: "-", want: []string{}, valid: true},
		{cell: "none (walked into during closure)", want: []string{}, valid: true},
		{cell: "n/a", want: []string{}, valid: true},
		{cell: "spec-a", want: []string{"spec-a"}, valid: true},
		{cell: "spec-a (measurement only)", want: []string{"spec-a"}, valid: true},
		{cell: "spec-a, spec-b, spec-a", want: []string{"spec-a", "spec-b"}, valid: true},
		{cell: "123-start", want: []string{"123-start"}, valid: true},
		{cell: "future work", valid: false},
		{cell: "../escape", valid: false},
		{cell: "spec-a (nested (note))", valid: false},
	}
	for _, test := range tests {
		got, valid := specStems(test.cell)
		if valid != test.valid || !reflect.DeepEqual(got, test.want) {
			t.Errorf("SpecStems(%q) = %q, %v; want %q, %v",
				test.cell, got, valid, test.want, test.valid)
		}
	}
}

func TestHeadSpecEvidenceNormalizesCellsAndRejectsUnreadableShard(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/good.md": journalTableHead +
			"| 2026-08-27 | spec-a, spec-b (shared fix) | cli | symptom | fix |\n" +
			"| 2026-08-27 | none (outside a spec) | cli | symptom | fix |\n",
		"plan/journal/bad.md": journalTableHead +
			"| 2026-08-27 | future work | cli | symptom | fix |\n",
	})
	evidence, malformed, err := HeadSpecEvidence(tree)
	if err != nil {
		t.Fatal(err)
	}
	wantEvidence := map[string]string{
		"spec-a": "plan/journal/good.md",
		"spec-b": "plan/journal/good.md",
	}
	if !reflect.DeepEqual(evidence, wantEvidence) {
		t.Fatalf("HeadSpecEvidence evidence = %#v, want %#v", evidence, wantEvidence)
	}
	if !reflect.DeepEqual(malformed, []string{"plan/journal/bad.md"}) {
		t.Fatalf("HeadSpecEvidence malformed = %q", malformed)
	}
}

// journalOfRows returns a journal class file holding rows dated rows.
func journalOfRows(rows int) string {
	body := journalTableHead
	for day := 1; day <= rows; day++ {
		body += fmt.Sprintf("| 2026-08-%02d | - | cli | symptom %d | fix |\n", day, day)
	}
	return body
}

// TestCheckMarksDueClassesAndOrdersThemFirst validates the fix-pass threshold
// at 9 and 10 rows, the order (due classes first by row count, the rest by
// name), the DUE marker and count line in Text, and the json field.
func TestCheckMarksDueClassesAndOrdersThemFirst(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/alpha.md":   journalOfRows(DueRowsMin - 1),
		"plan/journal/beta.md":    journalOfRows(DueRowsMin),
		"plan/journal/gamma.md":   journalOfRows(DueRowsMin + 2),
		"plan/journal/delta.md":   journalOfRows(2),
		"plan/journal/epsilon.md": journalOfRows(DueRowsMin),
	})
	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := []struct {
		name string
		rows int
		due  bool
	}{
		{"gamma", DueRowsMin + 2, true},
		{"beta", DueRowsMin, true},
		{"epsilon", DueRowsMin, true},
		{"alpha", DueRowsMin - 1, false},
		{"delta", 2, false},
	}
	if len(report.Classes) != len(want) {
		t.Fatalf("Classes = %#v, want %d", report.Classes, len(want))
	}
	for index, class := range report.Classes {
		if class.Name != want[index].name || class.Rows != want[index].rows || class.Due != want[index].due {
			t.Fatalf("Classes[%d] = %#v, want %+v", index, class, want[index])
		}
	}
	if report.DueCount() != 3 {
		t.Fatalf("DueCount = %d, want 3", report.DueCount())
	}
	text := report.Text()
	if !strings.HasPrefix(text, "3 problem classes have 10+ rows and are due for a fix pass\nDUE gamma: 12 rows") {
		t.Fatalf("Text() = %q, want the count line then DUE gamma", text)
	}
	if !strings.Contains(text, "\nalpha: 9 rows") || strings.Contains(text, "DUE alpha") {
		t.Fatalf("Text() = %q, want alpha unmarked", text)
	}
	encoded, err := json.Marshal(report.Classes[0])
	if err != nil || !strings.Contains(string(encoded), `"due":true`) {
		t.Fatalf("json = %s, %v; want \"due\":true", encoded, err)
	}
	encoded, err = json.Marshal(report.Classes[3])
	if err != nil || !strings.Contains(string(encoded), `"due":false`) {
		t.Fatalf("json = %s, %v; want \"due\":false", encoded, err)
	}
}

// TestHeadFilesReadsAMissingPathWithASpaceAsAbsent validates the cat-file
// protocol reader. git echoes the object name back, so a missing path holding a
// space answers a header with more than two fields. It proves that answer is
// read as absent, and that the answers after it still pair with their paths.
func TestHeadFilesReadsAMissingPathWithASpaceAsAbsent(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/seed.md": journalOfRows(1),
		"plan/journal/tail.md": journalOfRows(2),
	})
	paths := []string{"plan/journal/seed.md", "plan/journal/a b.md", "plan/journal/tail.md"}
	contents, err := headFiles(tree, paths)
	if err != nil {
		t.Fatalf("headFiles: %v", err)
	}
	if _, present := contents["plan/journal/a b.md"]; present {
		t.Fatalf("a path HEAD does not carry was read: %#v", contents)
	}
	if contents["plan/journal/seed.md"] != journalOfRows(1) || contents["plan/journal/tail.md"] != journalOfRows(2) {
		t.Fatalf("contents = %#v, want seed and tail at HEAD", contents)
	}
}
