// Related: quiet.go -- the run these cases drive from its entry point
//
// VALIDATES: a quiet job keeps the child's output out of the caller's stdout,
// keeps it in a log that survives the run, and answers where that log is and
// what failed in it.
// PREVENTS: the four-step shell pattern this replaces coming back because the
// command answered less than the pattern did. Each case fails if one of the
// four steps is dropped: the redirect, the durable path, the exit code, or the
// failure lines.

package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// quietAdmission answers an admission over a fixture checkout whose session
// identity is fixed, so the scratch path a quiet run writes to is predictable.
func quietAdmission(t *testing.T) *Admission {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "quiet-fixture")

	adm, err := NewIn(fixtureRepo(t))
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	adm.Poll = 100 * time.Millisecond
	adm.Banner = time.Minute
	adm.Color = false
	adm.Err = &strings.Builder{}
	return adm
}

// TestAQuietJobAnswersTheVerdictInsteadOfTheOutput is the whole of the pattern
// in one case: nothing reaches stdout, the log holds everything, the exit code
// is the child's, and the failure line is named with its line number.
func TestAQuietJobAnswersTheVerdictInsteadOfTheOutput(t *testing.T) {
	adm := quietAdmission(t)
	var terminal strings.Builder
	adm.Out = &terminal

	report, code := runQuiet(adm, "unit-pkg", []string{"sh", "-c",
		"echo 'ok  	first	0.1s'; echo 'FAIL	second	0.2s'; exit 1"})

	if code != 1 {
		t.Errorf("the quiet job answered %d, want the child's 1", code)
	}
	if terminal.String() != "" {
		t.Errorf("the child's output reached stdout:\n%s", terminal.String())
	}
	if report.Log == "" {
		t.Fatal("the report named no log, so the output cannot be read at all")
	}

	body := read(t, filepath.Join(adm.Root, report.Log))
	if !strings.Contains(body, "FAIL\tsecond\t0.2s") {
		t.Errorf("the log does not hold the child's output:\n%s", body)
	}
	if len(report.KeyLines) != 1 || report.KeyLines[0] != "2:FAIL\tsecond\t0.2s" {
		t.Errorf("the report's key lines are %v, want the FAIL line as line 2", report.KeyLines)
	}
}

// TestTheQuietLogSurvivesTheRun is the reason this log is not the registry's:
// ticket.Release removes that one, so a reader who arrives after the job has
// finished finds nothing.
func TestTheQuietLogSurvivesTheRun(t *testing.T) {
	adm := quietAdmission(t)
	adm.Out = &strings.Builder{}

	report, _ := runQuiet(adm, "unit-pkg", []string{"sh", "-c", "echo done"})

	if _, err := os.Stat(filepath.Join(adm.Root, report.Log)); err != nil {
		t.Fatalf("the quiet log is gone once the job ended: %v", err)
	}
	if !strings.HasPrefix(report.Log, "tmp/session/") {
		t.Errorf("the quiet log is at %q, want it in this session's scratch", report.Log)
	}
}

// TestAPassingQuietJobStillSaysWhereItsOutputWent covers the ordinary case: no
// failure line, and one line of answer that names the log anyway.
func TestAPassingQuietJobStillSaysWhereItsOutputWent(t *testing.T) {
	adm := quietAdmission(t)
	adm.Out = &strings.Builder{}

	report, code := runQuiet(adm, "unit-pkg", []string{"sh", "-c", "echo 'ok  	first	0.1s'"})

	if code != 0 {
		t.Fatalf("the quiet job answered %d, want 0", code)
	}
	if len(report.KeyLines) != 0 {
		t.Errorf("a passing job answered key lines %v", report.KeyLines)
	}
	text := report.Text()
	if !strings.HasPrefix(text, "job unit-pkg: exit 0, log tmp/session/") {
		t.Errorf("the answer is %q, want the verdict and the log path", text)
	}
}

// TestALabelThatIsNotAPathComponentIsRefusedBeforeAnythingIsWritten covers the
// order: the label becomes a filename here, so it is checked before it is used
// rather than by the admission that follows.
func TestALabelThatIsNotAPathComponentIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	adm := quietAdmission(t)
	adm.Out = &strings.Builder{}

	report, code := runQuiet(adm, "../escape", []string{"sh", "-c", "echo done"})

	if code != 2 {
		t.Errorf("an unusable label answered %d, want the refusal code 2", code)
	}
	if report.Log != "" {
		t.Errorf("a refused job named the log %q", report.Log)
	}
	if _, err := os.Stat(filepath.Join(adm.Root, "tmp", "escape.log")); err == nil {
		t.Error("a refused job wrote outside the session scratch directory")
	}
}

// TestQuietIsReadBeforeTheCommandAndNowhereAfterIt covers the grammar. The
// keyword is this command's, and the same word inside the child's argv belongs
// to the child.
func TestQuietIsReadBeforeTheCommandAndNowhereAfterIt(t *testing.T) {
	asked, ok := parseRun([]string{"label", "unit-pkg", "quiet", "command", "go", "test", "./..."})
	if !ok {
		t.Fatal("the quiet form was refused")
	}
	if !asked.Quiet {
		t.Error("the quiet keyword did not reach the run")
	}
	if strings.Join(asked.Argv, " ") != "go test ./..." {
		t.Errorf("the command is %v, want the argv after the command keyword", asked.Argv)
	}

	loud, ok := parseRun([]string{"label", "unit-pkg", "command", "git", "commit", "quiet"})
	if !ok {
		t.Fatal("the ordinary form was refused")
	}
	if loud.Quiet {
		t.Error("a quiet inside the child's argv was read as this command's keyword")
	}
	if strings.Join(loud.Argv, " ") != "git commit quiet" {
		t.Errorf("the command is %v, want the child's argv unchanged", loud.Argv)
	}
}
