// Design: docs/architecture/testing/test-health.md -- old/new behavioral parity
package testweakened

import (
	"slices"
	"strings"
	"testing"
)

// The commit namespace every fixture in this package records its owner
// approvals under. Naming it once keeps the tests over the SAME derivation the
// live gate uses: the file is ApprovalPath(session) and nothing else names it.
const fixtureSession = "0f1e2d3c"

var fixtureApproval = ApprovalPath(fixtureSession)

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
	live := Check(Request{Root: root, ApprovalSession: fixtureSession})
	wantLive := "Weakened-test check: no path given, nothing compared.\n"
	if live.ExitCode() != 0 || live.Text() != wantLive {
		t.Fatalf("live shape = code %d, text %q", live.ExitCode(), live.Text())
	}
	path := "pkg/a_test.go"
	writeParityFile(t, root, path, "package a\nfunc TestA(t *testing.T) {\n"+
		"\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n")

	missing := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	wantProblem := "pkg/a_test.go weakens pkg.TestA, and no ze-test-judge approval names it:\n" +
		"    - adding t.Skip (0 -> 1); the test stops running\n" +
		"    Fix the code by default. Only an independent ze-test-judge agent approves a test weakening, BEFORE the edit.\n" +
		"    On approval the judge records its verdict, then retry:\n" +
		"    ./le test weakened approve unit pkg.TestA reason \"<the judge's verdict>\""
	if missing.ExitCode() != 1 || !slices.Equal(missing.Problems, []string{wantProblem}) {
		t.Fatalf("missing approval = code %d, problems %q", missing.ExitCode(), missing.Problems)
	}
	wantText := "Weakened-test check: 1 problem(s).\n\n" +
		"  " + wantProblem + "\n"
	if missing.Text() != wantText {
		t.Fatalf("missing.Text() = %q, want %q", missing.Text(), wantText)
	}

	if _, err := Approve(root, fixtureSession, "pkg.TestA", "the feature it drove is gone"); err != nil {
		t.Fatal(err)
	}
	accepted := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}})
	if accepted.ExitCode() != 0 || len(accepted.Problems) != 0 {
		t.Fatalf("accepted approval = code %d, problems %q", accepted.ExitCode(), accepted.Problems)
	}
	wantTrailers := []string{"Weakening-approved: pkg.TestA: the feature it drove is gone"}
	if !slices.Equal(accepted.Approvals.Trailers, wantTrailers) {
		t.Fatalf("accepted.Approvals.Trailers = %q, want %q", accepted.Approvals.Trailers, wantTrailers)
	}
	wantAccepted := "Weakened-test check: clean (1 of 1 path(s) are tests, " +
		"judged against HEAD).\n"
	if accepted.Text() != wantAccepted {
		t.Fatalf("accepted.Text() = %q, want %q", accepted.Text(), wantAccepted)
	}

	cannotRun := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{path}, Anchor: "MISSING"})
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
			problem: "tmp/commit-weakening-approved-0f1e2d3c.md has no `| Test | Reason |` table header, so no row in it can be read",
		},
		{
			name:    "three cells",
			content: fixtureLedgerHeader + "| TestA | reason | extra |\n",
			problem: "tmp/commit-weakening-approved-0f1e2d3c.md:3 has 3 cells; a row is `| Test | Reason |`",
		},
		{
			name:    "empty name",
			content: fixtureLedgerHeader + "| | reason |\n",
			problem: "tmp/commit-weakening-approved-0f1e2d3c.md:3 names no test",
		},
		{
			name:    "empty reason",
			content: fixtureLedgerHeader + "| TestA | |\n",
			problem: "tmp/commit-weakening-approved-0f1e2d3c.md:3 gives no reason for TestA; a row with no reason accepts nothing",
		},
		{
			name: "duplicate",
			content: fixtureLedgerHeader + "| TestA | first |\n" +
				"| TestA | second |\n",
			problem: "tmp/commit-weakening-approved-0f1e2d3c.md:4 names TestA again (already on line 3); one test, one reason",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := newParityRepository(t)
			writeParityFile(t, root, fixtureApproval, tc.content)
			writeParityFile(t, root, "pkg/a_test.go", parityWeakening)
			result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{"pkg/a_test.go"}})
			if result.ExitCode() != 1 || !slices.Contains(result.Problems, tc.problem) {
				t.Fatalf("Check() = code %d, problems %q; want %q", result.ExitCode(), result.Problems, tc.problem)
			}
		})
	}

	// A session holding no approval file holds no approval, and that is the
	// state a checkout is in between commits rather than a problem of its own.
	// The fail-closed contract sits where the absence decides something: a
	// weakening with no approval is refused, and the refusal carries the
	// command that records the owner's answer.
	root := newParityRepository(t)
	absent := Check(Request{Root: root, ApprovalSession: fixtureSession})
	if absent.ExitCode() != 0 || len(absent.Problems) != 0 {
		t.Fatalf("absent approval file = code %d, problems %q", absent.ExitCode(), absent.Problems)
	}
	writeParityFile(t, root, "pkg/a_test.go", parityWeakening)
	weakening := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{"pkg/a_test.go"}})
	if weakening.ExitCode() != 1 || len(weakening.Problems) != 1 {
		t.Fatalf("absent approval file with a weakening = code %d, problems %q",
			weakening.ExitCode(), weakening.Problems)
	}
	if !strings.Contains(weakening.Problems[0], "no ze-test-judge approval names it") ||
		!strings.Contains(weakening.Problems[0], ApproveCommand("pkg.TestA")) {
		t.Fatalf("absent-approval refusal = %q, want the unit refused and the approve command",
			weakening.Problems[0])
	}
}

// parityWeakening is pkg/a_test.go with a skip added: one weakening of pkg.TestA
// against the baseline newParityRepository commits.
const parityWeakening = "package a\nfunc TestA(t *testing.T) {\n\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n"

func TestCheckDoesNotReadLedgerWhenPopulationIsClean(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// The approval file is malformed, so a check that read it would fail
	// closed: a clean verdict proves a clean population never reads it.
	root := newParityRepository(t)
	writeParityFile(t, root, fixtureApproval, "# Tests\n\nno table\n")
	result := Check(Request{Root: root, ApprovalSession: fixtureSession, Paths: []string{"pkg/a_test.go", "docs/note.md"}})
	if result.ExitCode() != 0 {
		t.Fatalf("Check() = code %d, problems %q", result.ExitCode(), result.Problems)
	}
	want := "Weakened-test check: clean (1 of 2 path(s) are tests, " +
		"judged against HEAD).\n"
	if result.Text() != want {
		t.Fatalf("Check().Text() = %q, want %q", result.Text(), want)
	}
}

func TestWeakenedAreaPublishesNativeActions(t *testing.T) {
	t.Parallel()

	list := Actions()
	if list.Area != "test weakened" || len(list.Actions) != 3 {
		t.Fatalf("Actions() = %#v", list)
	}
	wantVerbs := []string{"selftest", "proposed", "approve"}
	wantWrites := []bool{false, false, true}
	for index, row := range list.Actions {
		if row.Verb != wantVerbs[index] || row.Why == "" || row.Writes != wantWrites[index] {
			t.Fatalf("Actions()[%d] = %#v", index, row)
		}
	}
}

func newParityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeParityFile(t, root, "pkg/a_test.go",
		"package a\nfunc TestA(t *testing.T) { require.Equal(t, 1, got) }\n")
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
