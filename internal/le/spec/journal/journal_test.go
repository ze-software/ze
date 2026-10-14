package specjournal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
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
			"| 2026-08-20 | z | config | later | fixed: fix |\n" +
			"| 2026-08-01 | z | config | earlier | fixed: fix |\n",
		"plan/journal/alpha.md": journalTableHead +
			"| 2026-07-15 | a | reactor | first | fixed: fix |\n" +
			"| 2026-07-16 | a | reactor | second | fixed: fix |\n",
		"plan/journal/singleton.md": journalTableHead +
			"| 2026-08-21 | one | cli | once | fixed: fix |\n",
	})

	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := "due at: 1 row since a check capture, 10 rows since a rule capture, 10 rows uncaptured\n" +
		"alpha: 2 rows, 0 open, 1d span (2026-07-15 .. 2026-07-16)\n" +
		"zebra: 2 rows, 0 open, 19d span (2026-08-01 .. 2026-08-20)\n"
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
			"| 2026-08-01 | a | cli | first | fixed: fix |\n" +
			"| 2026-08-03 | b | cli | second | fixed: fix |\n",
	})
	writeJournalFile(t, tree, "plan/journal/worktree-only.md", journalTableHead+
		"| 2026-08-04 | c | cli | first | fixed: fix |\n"+
		"| 2026-08-05 | d | cli | second | fixed: fix |\n")

	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got, want := report.Text(), thresholdText()+"\ncommitted: 2 rows, 0 open, 2d span (2026-08-01 .. 2026-08-03)\n"; got != want {
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
			"2026-08-01 | a | cli | missing leading pipe | fixed: fix |\n" +
			"| 2026-08-02 | b | cli | raw | pipe | fixed: fix |\n",
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
			"| - | a | cli | no date | fixed: fix |\n",
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
	if action.Why != journalWhy() {
		t.Fatalf("why = %q, want %q", action.Why, journalWhy())
	}
	validate := list.Actions[1]
	if validate.Verb != "validate" || validate.Writes ||
		validate.Why != "validate one edited plan/journal class file's header, rows, dates, Spec keys, "+
			"Fix markers, Captured: line, and the 600-character cap on a row added or rewritten against HEAD" {
		t.Fatalf("validate action = %#v", validate)
	}
}

// TestAnswerReturnsTheStructuredReport validates the command boundary. It
// prevents the native command from returning preformatted report text.
func TestAnswerReturnsTheStructuredReport(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/recurring.md": journalTableHead +
			"| 2026-08-01 | a | cli | first | fixed: fix |\n" +
			"| 2026-08-02 | b | cli | second | fixed: fix |\n",
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
			"| 2026-08-27 | spec-a | cli | symptom | fixed: fix |\n",
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
			"| 2026-08-27 | spec-a, spec-b (shared fix) | cli | symptom | fixed: fix |\n" +
			"| 2026-08-27 | none (outside a spec) | cli | symptom | fixed: fix |\n",
		"plan/journal/bad.md": journalTableHead +
			"| 2026-08-27 | future work | cli | symptom | fixed: fix |\n",
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
		body += fmt.Sprintf("| 2026-08-%02d | - | cli | symptom %d | fixed: fix |\n", day, day)
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
	if !strings.HasPrefix(text, "3 problem classes are due for a fix pass\n"+thresholdText()+"\nDUE gamma: 12 rows") {
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

// captureFiles is the Go check, its proof test and the rule point that the
// capture fixtures name, so a capture resolves at the fixture's HEAD.
func captureFiles(files map[string]string) map[string]string {
	merged := map[string]string{
		"internal/guard/guard.go":                  "package guard\n\n// Guard is the fixture check.\nfunc Guard() {}\n",
		"internal/guard/guard_test.go":             "package guard\n\nimport \"testing\"\n\nfunc TestGuard(t *testing.T) {}\n",
		"ai/rules/points/area/directives/point.md": "point\n",
	}
	maps.Copy(merged, files)
	return merged
}

const (
	checkCapture = "Captured: 2026-08-12 check internal/guard.Guard proof internal/guard.TestGuard\n"
	ruleCapture  = "Captured: 2026-08-12 rule area/directives/point\n"
)

// markedRows returns count rows dated from 2026-<month>-<firstDay>, one a day,
// each carrying fix as its Fix cell.
func markedRows(month string, firstDay, count int, fix string) string {
	body := ""
	for day := firstDay; day < firstDay+count; day++ {
		body += fmt.Sprintf("| 2026-%s-%02d | - | cli | symptom %d | %s |\n", month, day, day, fix)
	}
	return body
}

func classNamed(t *testing.T, report Report, name string) Class {
	t.Helper()
	for _, class := range report.Classes {
		if class.Name == name {
			return class
		}
	}
	t.Fatalf("class %s absent from %#v (problems %#v)", name, report.Classes, report.Problems)
	return Class{}
}

// TestCheckCountsOnlyRowsAfterTheCapture validates AC-1, AC-2 and AC-11: rows
// dated on or before a check capture are not counted, the first row dated after
// it makes the class due, and due classes sort by counted rows.
func TestCheckCountsOnlyRowsAfterTheCapture(t *testing.T) {
	before := "# Captured\n\n" + checkCapture + "\n" + journalTableHead + markedRows("08", 1, 12, "fixed: x")
	tree := journalFixture(t, captureFiles(map[string]string{"plan/journal/captured.md": before}))
	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	class := classNamed(t, report, "captured")
	wantCapture := Capture{Date: "2026-08-12", Kind: CaptureCheck, Check: "internal/guard.Guard", Proof: "internal/guard.TestGuard"}
	if class.CountedRows != 0 || class.Rows != 12 || class.Due || class.DueAt != DueRowsAfterCheck ||
		class.Captured == nil || *class.Captured != wantCapture {
		t.Fatalf("class = %#v, want 0 counted of 12, not due, due-at 1, capture %#v", class, wantCapture)
	}
	encoded, err := json.Marshal(class)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"counted-rows":0`, `"rows":12`, `"due-at":1`,
		`"captured":{"date":"2026-08-12","kind":"check","check":"internal/guard.Guard","proof":"internal/guard.TestGuard"}`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("json %s lacks %s", encoded, field)
		}
	}

	after := before + markedRows("08", 13, 1, "open: x")
	tree = journalFixture(t, captureFiles(map[string]string{
		"plan/journal/captured.md":   after,
		"plan/journal/uncaptured.md": journalTableHead + markedRows("08", 1, DueRowsMin, "open: x"),
	}))
	report, err = Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Classes) != 2 || report.Classes[0].Name != "uncaptured" || report.Classes[1].Name != "captured" {
		t.Fatalf("Classes = %#v, want uncaptured (10 counted) before captured (1 counted)", report.Classes)
	}
	if got := report.Classes[1]; got.CountedRows != 1 || !got.Due || got.Rows != 13 {
		t.Fatalf("captured class = %#v, want 1 counted, due, 13 rows", got)
	}
}

// TestRuleCaptureKeepsTheCountThreshold validates AC-2b: a rule capture keeps
// DueRowsMin over the rows dated after it, 9 not due and 10 due.
func TestRuleCaptureKeepsTheCountThreshold(t *testing.T) {
	for _, test := range []struct {
		after int
		due   bool
	}{{after: DueRowsMin - 1, due: false}, {after: DueRowsMin, due: true}} {
		body := ruleCapture + journalTableHead + markedRows("08", 1, 12, "fixed: x") +
			markedRows("09", 1, test.after, "open: x")
		tree := journalFixture(t, captureFiles(map[string]string{"plan/journal/ruled.md": body}))
		report, err := Check(tree)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		class := classNamed(t, report, "ruled")
		if class.CountedRows != test.after || class.Due != test.due || class.DueAt != DueRowsMin ||
			class.Captured == nil || class.Captured.Point != "area/directives/point" {
			t.Fatalf("after %d: class = %#v, want counted %d due %v due-at %d", test.after, class,
				test.after, test.due, DueRowsMin)
		}
	}
}

// TestCheckWithoutCaptureCountsEveryRow validates AC-3: an uncaptured class
// counts every row against DueRowsMin.
func TestCheckWithoutCaptureCountsEveryRow(t *testing.T) {
	tree := journalFixture(t, map[string]string{
		"plan/journal/plain.md": journalTableHead + markedRows("08", 1, DueRowsMin, "open: x"),
	})
	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	class := classNamed(t, report, "plain")
	if !class.Due || class.CountedRows != class.Rows || class.DueAt != DueRowsMin || class.Captured != nil {
		t.Fatalf("class = %#v, want due with every row counted", class)
	}
}

// TestThresholdsComeFromTheKindTable validates AC-3b: changing one kind's
// threshold in the table changes the text threshold line, the JSON thresholds
// and due-at, and the report action's help together.
func TestThresholdsComeFromTheKindTable(t *testing.T) {
	saved := captureKinds
	t.Cleanup(func() { captureKinds = saved })
	captureKinds[0].dueAt = 3

	body := checkCapture + journalTableHead + markedRows("08", 10, 6, "fixed: x")
	tree := journalFixture(t, captureFiles(map[string]string{"plan/journal/captured.md": body}))
	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	line := "due at: 3 rows since a check capture, 10 rows since a rule capture, 10 rows uncaptured"
	if !strings.Contains(report.Text(), line+"\n") {
		t.Fatalf("Text() = %q, want threshold line %q", report.Text(), line)
	}
	if !strings.Contains(journalWhy(), line) {
		t.Fatalf("journalWhy() = %q, want %q", journalWhy(), line)
	}
	class := classNamed(t, report, "captured")
	if class.DueAt != 3 || class.CountedRows != 3 || !class.Due {
		t.Fatalf("class = %#v, want due-at 3, 3 counted, due", class)
	}
	encoded, err := json.Marshal(report)
	if err != nil || !strings.Contains(string(encoded), `"thresholds":{"check":3,"rule":10,"uncaptured":10}`) {
		t.Fatalf("json = %s, %v; want the thresholds object from the table", encoded, err)
	}
}

// runProblems runs the report over files and returns its problems, requiring
// exit 1.
func runProblems(t *testing.T, files map[string]string) ([]Problem, string) {
	t.Helper()
	tree := journalFixture(t, captureFiles(files))
	var errOut bytes.Buffer
	report, code := Run(tree, &errOut)
	if code != 1 {
		t.Fatalf("Run code = %d, want 1: %s", code, errOut.String())
	}
	return report.Problems, errOut.String()
}

// TestRunRefusesAMalformedCaptureLine validates AC-4 and AC-7: every line that
// starts Captured: and breaks the grammar is a capture-malformed problem
// naming its line, never an uncaptured class.
func TestRunRefusesAMalformedCaptureLine(t *testing.T) {
	for _, line := range []string{
		"Captured: 2026-8-12 rule area/directives/point",
		"Captured: 2026-08-12 policy area/directives/point",
		"Captured: 2026-08-12 check internal/guard.Guard",
		"Captured: 2026-08-12 check internal/guard.Guard proof internal/guard.Guard",
		"Captured: 2026-08-12 rule area/directives/point | extra",
		"Captured: 2026-08-12 check internal/../guard.Guard proof internal/guard.TestGuard",
		"Captured: 2026-08-12 check /abs/guard.Guard proof internal/guard.TestGuard",
		"Captured: 2026-08-12 rule area/\x00point",
		"Captured: 2026-08-12 rule area/directives/point.md",
		"Captured: 2999-01-01 rule area/directives/point",
		"Captured:",
	} {
		body := "# Class\n" + line + "\n" + journalTableHead + markedRows("08", 1, 2, "fixed: x")
		problems, stderr := runProblems(t, map[string]string{"plan/journal/bad.md": body})
		if len(problems) != 1 || problems[0].Kind != ProblemCaptureMalformed || problems[0].Line != 2 {
			t.Fatalf("line %q: problems = %#v, want one capture-malformed at line 2", line, problems)
		}
		if !strings.HasPrefix(stderr, "CAPTURE MALFORMED: plan/journal/bad.md:2: ") {
			t.Fatalf("line %q: stderr = %q", line, stderr)
		}
	}
	problems, _ := runProblems(t, map[string]string{"plan/journal/late.md": "Captured: 2999-01-01 rule area/directives/point\n" +
		journalTableHead + markedRows("08", 1, 2, "fixed: x")})
	if !strings.Contains(problems[0].Detail, "dated after HEAD") {
		t.Fatalf("late capture detail = %q, want dated after HEAD", problems[0].Detail)
	}
}

// TestRunRefusesACaptureThatDoesNotResolve validates AC-5: a check, a proof or
// a point HEAD does not declare is capture-unresolved, and a function present
// only in the working tree does not resolve at HEAD.
func TestRunRefusesACaptureThatDoesNotResolve(t *testing.T) {
	rows := journalTableHead + markedRows("08", 1, 2, "fixed: x")
	for _, test := range []struct{ line, missing string }{
		{"Captured: 2026-08-12 check internal/guard.Absent proof internal/guard.TestGuard", "internal/guard.Absent"},
		{"Captured: 2026-08-12 check internal/guard.Guard proof internal/guard.TestAbsent", "internal/guard.TestAbsent"},
		{"Captured: 2026-08-12 rule area/directives/absent", "ai/rules/points/area/directives/absent.md"},
		{"Captured: 2026-08-12 check internal/nowhere.Guard proof internal/guard.TestGuard", "internal/nowhere.Guard"},
	} {
		problems, _ := runProblems(t, map[string]string{"plan/journal/bad.md": test.line + "\n" + rows})
		if len(problems) != 1 || problems[0].Kind != ProblemCaptureUnresolved ||
			!strings.Contains(problems[0].Detail, test.missing) || !strings.Contains(problems[0].Detail, "at HEAD") {
			t.Fatalf("%q: problems = %#v, want capture-unresolved naming %s", test.line, problems, test.missing)
		}
	}
	tree := journalFixture(t, captureFiles(map[string]string{"plan/journal/bad.md": "Captured: 2026-08-12 check " +
		"internal/guard.Later proof internal/guard.TestGuard\n" + rows}))
	writeJournalFile(t, tree, "internal/guard/later.go", "package guard\n\nfunc Later() {}\n")
	var errOut bytes.Buffer
	report, code := Run(tree, &errOut)
	if code != 1 || len(report.Problems) != 1 || report.Problems[0].Kind != ProblemCaptureUnresolved {
		t.Fatalf("working-tree-only check: code %d problems %#v", code, report.Problems)
	}
}

// TestRunRefusesTwoCaptureLines validates AC-6.
func TestRunRefusesTwoCaptureLines(t *testing.T) {
	problems, _ := runProblems(t, map[string]string{"plan/journal/twice.md": checkCapture + ruleCapture +
		journalTableHead + markedRows("08", 1, 2, "fixed: x")})
	if len(problems) != 1 || problems[0].Kind != ProblemCaptureDuplicated || problems[0].Line != 2 {
		t.Fatalf("problems = %#v, want one capture-duplicated at line 2", problems)
	}
}

// TestRunRefusesAnUnmarkedFixCell validates AC-8 in the report: a Fix cell that
// does not start with the exact marker refuses the class, with the count and
// the first three dates.
func TestRunRefusesAnUnmarkedFixCell(t *testing.T) {
	body := journalTableHead + markedRows("08", 1, 1, "Fixed: capital") + markedRows("08", 2, 1, "fixed:tight") +
		markedRows("08", 3, 1, "fixed: marked") + markedRows("08", 4, 2, "not fixed")
	problems, stderr := runProblems(t, map[string]string{"plan/journal/unmarked.md": body})
	want := []string{"2026-08-01", "2026-08-02", "2026-08-04"}
	if len(problems) != 1 || problems[0].Kind != ProblemUnmarkedFix || problems[0].Rows != 4 ||
		!reflect.DeepEqual(problems[0].Dates, want) {
		t.Fatalf("problems = %#v, want one unmarked-fix of 4 rows first dated %q", problems, want)
	}
	wantText := "UNMARKED FIX: plan/journal/unmarked.md: 4 row(s) whose Fix cell starts with neither " +
		"`fixed: ` nor `open: ` (first dated 2026-08-01, 2026-08-02, 2026-08-04)\n"
	if stderr != wantText {
		t.Fatalf("stderr = %q, want %q", stderr, wantText)
	}
}

// TestReportTextShowsCaptureAndOpenCounts validates AC-9 and AC-10: the exact
// text of a captured due class, an uncaptured class, the capture summary and
// the threshold line, and the JSON open and due-at fields.
func TestReportTextShowsCaptureAndOpenCounts(t *testing.T) {
	tree := journalFixture(t, captureFiles(map[string]string{
		"plan/journal/captured.md": checkCapture + journalTableHead + markedRows("08", 11, 2, "fixed: x") +
			markedRows("08", 13, 2, "open: x"),
		"plan/journal/ruled.md":  ruleCapture + journalTableHead + markedRows("08", 1, 3, "open: x"),
		"plan/journal/opened.md": journalTableHead + markedRows("08", 1, 5, "open: x"),
		"plan/journal/single.md": journalTableHead + markedRows("08", 1, 1, "open: x"),
	}))
	report, err := Check(tree)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := "1 problem classes are due for a fix pass\n" +
		"2 classes captured: 1 by check, 1 by rule\n" +
		"due at: 1 row since a check capture, 10 rows since a rule capture, 10 rows uncaptured\n" +
		"DUE captured: 2 rows since capture 2026-08-12 (check), 4 total, 2 open, 3d span (2026-08-11 .. 2026-08-14)\n" +
		"opened: 5 rows, 5 open, 4d span (2026-08-01 .. 2026-08-05)\n" +
		"ruled: 0 rows since capture 2026-08-12 (rule), 3 total, 3 open, 2d span (2026-08-01 .. 2026-08-03)\n"
	if got := report.Text(); got != want {
		t.Fatalf("Text() =\n%s\nwant\n%s", got, want)
	}
	encoded, err := json.Marshal(classNamed(t, report, "opened"))
	if err != nil || !strings.Contains(string(encoded), `"open":5`) || !strings.Contains(string(encoded), `"due-at":10`) {
		t.Fatalf("json = %s, %v; want open 5 and due-at 10", encoded, err)
	}
}

// TestHeadSpecEvidenceIgnoresTheCaptureLine validates AC-12.
func TestHeadSpecEvidenceIgnoresTheCaptureLine(t *testing.T) {
	rows := journalTableHead + "| 2026-08-27 | spec-a | cli | symptom | fixed: fix |\n"
	plain := journalFixture(t, map[string]string{"plan/journal/good.md": rows})
	captured := journalFixture(t, captureFiles(map[string]string{"plan/journal/good.md": "# Good\n\n" + checkCapture + rows}))
	for _, tree := range []string{plain, captured} {
		evidence, malformed, err := HeadSpecEvidence(tree)
		if err != nil || len(malformed) != 0 || !reflect.DeepEqual(evidence, map[string]string{"spec-a": "plan/journal/good.md"}) {
			t.Fatalf("HeadSpecEvidence = %#v, %q, %v", evidence, malformed, err)
		}
	}
}
