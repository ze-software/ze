// Design: docs/architecture/testing/test-health.md -- old/new behavioral parity
package weakened

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSelfTestCoversEveryDetectorVerdictThroughCheck(t *testing.T) {
	t.Parallel()

	report := SelfTest()
	if len(report.Failures) != 0 {
		t.Fatalf("SelfTest() failures = %q", report.Failures)
	}
	if report.Positive != 16 {
		t.Fatalf("SelfTest() positive = %d, want 16", report.Positive)
	}
	if report.Negative != 3 {
		t.Fatalf("SelfTest() negative = %d, want 3", report.Negative)
	}
	if report.Checks < 50 {
		t.Fatalf("SelfTest() checks = %d, want the complete fixture population", report.Checks)
	}
	if report.ExitCode() != 0 || report.Text() != "SELFTEST PASS\n" {
		t.Fatalf("SelfTest() verdict = (%d, %q)", report.ExitCode(), report.Text())
	}
}

func TestCheckMatchesProducerDiagnosticsAndExitCodes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	root := newParityRepository(t)
	live := Check(Request{Root: root})
	wantLive := "Weakened-test check: test/weakened.md parses (0 row(s)).\n"
	if live.ExitCode() != 0 || live.Text() != wantLive {
		t.Fatalf("live shape = code %d, text %q", live.ExitCode(), live.Text())
	}
	path := "pkg/a_test.go"
	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) {\n"+
		"\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")

	missing := Check(Request{Root: root, Paths: []string{path}})
	wantProblem := "pkg/a_test.go weakens TestA and test/weakened.md has no row for it:\n" +
		"    - adding t.Skip (0 -> 1); the test stops running\n" +
		"    Add the row, then commit the file with the change:\n" +
		"    | TestA | <what left the suite, and why the commit is correct without it> |"
	if missing.ExitCode() != 1 || !slices.Equal(missing.Problems, []string{wantProblem}) {
		t.Fatalf("missing row = code %d, problems %q", missing.ExitCode(), missing.Problems)
	}
	wantText := "Weakened-test check: 1 problem(s).\n\n" +
		"  " + wantProblem + "\n"
	if missing.Text() != wantText {
		t.Fatalf("missing.Text() = %q, want %q", missing.Text(), wantText)
	}

	writeParityFile(t, root, ContractPath,
		fixtureLedgerHeader+"| TestA | the feature it drove is gone |\n")
	accepted := Check(Request{Root: root, Paths: []string{path}})
	if accepted.ExitCode() != 0 || len(accepted.Problems) != 0 {
		t.Fatalf("accepted row = code %d, problems %q", accepted.ExitCode(), accepted.Problems)
	}
	wantAccepted := "Weakened-test check: clean (1 of 1 path(s) are tests, " +
		"judged against HEAD).\n"
	if accepted.Text() != wantAccepted {
		t.Fatalf("accepted.Text() = %q, want %q", accepted.Text(), wantAccepted)
	}

	cannotRun := Check(Request{Root: root, Paths: []string{path}, Anchor: "MISSING"})
	wantCannotRun := "check could not run: MISSING does not resolve to a commit, so nothing was compared"
	if cannotRun.ExitCode() != 2 || !slices.Equal(cannotRun.Problems, []string{wantCannotRun}) {
		t.Fatalf("invalid anchor = code %d, problems %q", cannotRun.ExitCode(), cannotRun.Problems)
	}
	wantCannotRunText := "Weakened-test check: CANNOT RUN.\n" +
		"  " + wantCannotRun + "\n"
	if cannotRun.Text() != wantCannotRunText {
		t.Fatalf("cannotRun.Text() = %q, want %q", cannotRun.Text(), wantCannotRunText)
	}
}

func TestCheckFailsClosedForEveryLedgerShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		problem string
	}{
		{
			name:    "no header",
			content: "# Tests\n\nno table\n",
			problem: "test/weakened.md has no `| Test | Reason |` table header, so no row in it can be read",
		},
		{
			name:    "three cells",
			content: fixtureLedgerHeader + "| TestA | reason | extra |\n",
			problem: "test/weakened.md:3 has 3 cells; a row is `| Test | Reason |`",
		},
		{
			name:    "empty name",
			content: fixtureLedgerHeader + "| | reason |\n",
			problem: "test/weakened.md:3 names no test",
		},
		{
			name:    "empty reason",
			content: fixtureLedgerHeader + "| TestA | |\n",
			problem: "test/weakened.md:3 gives no reason for TestA; a row with no reason accepts nothing",
		},
		{
			name: "duplicate",
			content: fixtureLedgerHeader + "| TestA | first |\n" +
				"| TestA | second |\n",
			problem: "test/weakened.md:4 names TestA again (already on line 3); one test, one reason",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeParityFile(t, root, ContractPath, tc.content)
			result := Check(Request{Root: root})
			if result.ExitCode() != 1 || !slices.Contains(result.Problems, tc.problem) {
				t.Fatalf("Check() = code %d, problems %q; want %q", result.ExitCode(), result.Problems, tc.problem)
			}
		})
	}

	root := t.TempDir()
	absent := Check(Request{Root: root})
	wantAbsent := "test/weakened.md is missing. The commit gate reads it, so a commit " +
		"that weakens a test has nowhere to record the reason."
	if absent.ExitCode() != 1 || !slices.Equal(absent.Problems, []string{wantAbsent}) {
		t.Fatalf("absent ledger = code %d, problems %q", absent.ExitCode(), absent.Problems)
	}
}

func TestCheckDoesNotReadLedgerWhenPopulationIsClean(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	root := newParityRepository(t)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(ContractPath))); err != nil {
		t.Fatal(err)
	}
	result := Check(Request{Root: root, Paths: []string{"pkg/a_test.go", "docs/note.md"}})
	if result.ExitCode() != 0 {
		t.Fatalf("Check() = code %d, problems %q", result.ExitCode(), result.Problems)
	}
	want := "Weakened-test check: clean (1 of 2 path(s) are tests, " +
		"judged against HEAD).\n"
	if result.Text() != want {
		t.Fatalf("Check().Text() = %q, want %q", result.Text(), want)
	}
}

func TestCheckQualifiesAmbiguousRemovedUnits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const baseline = "package a\nfunc TestSame(t *testing.T) { require.Equal(t, 1, got) }\n"
	paths := []string{"alpha/same_test.go", "beta/same_test.go"}
	for _, path := range paths {
		writeParityFile(t, root, path, baseline)
	}
	writeParityFile(t, root, ContractPath,
		fixtureLedgerHeader+"| TestSame | removed coverage |\n")
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root,
			"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline") {
		t.Fatal("initialize ambiguity repository")
	}
	for _, path := range paths {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}

	ambiguous := Check(Request{Root: root, Removed: paths})
	if ambiguous.ExitCode() != 1 || len(ambiguous.Problems) != 1 ||
		!strings.Contains(ambiguous.Problems[0], "weakens in 2 packages: alpha") ||
		!strings.Contains(ambiguous.Problems[0], "| beta.TestSame |") {
		t.Fatalf("ambiguous removed units = code %d, problems %q",
			ambiguous.ExitCode(), ambiguous.Problems)
	}

	writeParityFile(t, root, ContractPath, fixtureLedgerHeader+
		"| alpha.TestSame | removed alpha coverage |\n"+
		"| beta.TestSame | removed beta coverage |\n")
	accepted := Check(Request{Root: root, Removed: paths})
	if accepted.ExitCode() != 0 || len(accepted.Findings) != 2 {
		t.Fatalf("qualified removed units = code %d, findings %#v, problems %q",
			accepted.ExitCode(), accepted.Findings, accepted.Problems)
	}
}

func TestWeakenedAreaPublishesNativeActions(t *testing.T) {
	t.Parallel()

	list := Actions()
	if list.Area != "test-weakened" || len(list.Actions) != 3 {
		t.Fatalf("Actions() = %#v", list)
	}
	wantVerbs := []string{"check", "selftest", "proposed"}
	for index, row := range list.Actions {
		if row.Verb != wantVerbs[index] || row.Why == "" || row.Writes {
			t.Fatalf("Actions()[%d] = %#v", index, row)
		}
	}
}

func newParityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeParityFile(t, root, "pkg/a_test.go",
		"package a\nfunc TestA(t *testing.T) { require.Equal(t, 1, got) }\n")
	writeParityFile(t, root, ContractPath, fixtureLedgerHeader)
	if !runSelfTestGit(root, "init", "-q") || !runSelfTestGit(root, "add", "-A") ||
		!runSelfTestGit(root,
			"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline") {
		t.Fatal("initialize parity repository")
	}
	return root
}

func writeParityFile(t *testing.T, root, path, content string) {
	t.Helper()
	if err := writeSelfTestFile(root, path, content); err != nil {
		t.Fatal(err)
	}
}

func TestPythonFixtureStringsStayOutsideExecutableVerdicts(t *testing.T) {
	t.Parallel()

	text := "def test_fixture():\n" +
		"    fixture = \"pytest.skip('later')\\nassert True\"\n" +
		"    assert value\n"
	masked := executableTestText("test/fixtures/fixture_test.py", text)
	if strings.Contains(masked, "pytest.skip") || strings.Contains(masked, "assert True") {
		t.Fatalf("executableTestText() retained fixture source: %q", masked)
	}
	if !strings.Contains(masked, "assert value") {
		t.Fatalf("executableTestText() masked executable assertion: %q", masked)
	}
}
