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

	report, code := adm.RunQuiet("unit-pkg", []string{"sh", "-c",
		"echo 'ok  	first	0.1s'; echo 'FAIL	second	0.2s'; exit 1"}, "", nil)

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

	report, _ := adm.RunQuiet("unit-pkg", []string{"sh", "-c", "echo done"}, "", nil)

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

	report, code := adm.RunQuiet("unit-pkg", []string{"sh", "-c", "echo 'ok  	first	0.1s'"}, "", nil)

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

	report, code := adm.RunQuiet("../escape", []string{"sh", "-c", "echo done"}, "", nil)

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

// TestTwoCommandsUnderOneLabelDoNotShareALog drives two different commands
// under one label from the same session, which is what sibling subagents do.
//
// VALIDATES: the log path is keyed on the command as well as the label, so each
// run reads its own output back.
// PREVENTS: the shape this file's own comment assumed away until 2026-09-03,
// that a session runs one thing at a time. A session's subagents share its
// CLAUDE_CODE_SESSION_ID and so its scratch directory, and delegation is the
// default here, so two of them under one label opened one path O_TRUNC and each
// took the other's KeyLines for its own verdict.
func TestTwoCommandsUnderOneLabelDoNotShareALog(t *testing.T) {
	adm := quietAdmission(t)
	adm.Out = &strings.Builder{}

	first, _ := adm.RunQuiet("unit-pkg", []string{"sh", "-c", "echo FAIL\tsibling-one"}, "", nil)
	second, _ := adm.RunQuiet("unit-pkg", []string{"sh", "-c", "echo FAIL\tsibling-two"}, "", nil)

	if first.Log == second.Log {
		t.Fatalf("both commands wrote %s, so the second truncated the first and "+
			"each read the other's verdict", first.Log)
	}
	if body := read(t, filepath.Join(adm.Root, first.Log)); !strings.Contains(body, "sibling-one") {
		t.Errorf("the first log holds %q, want its own output", body)
	}
	if body := read(t, filepath.Join(adm.Root, second.Log)); !strings.Contains(body, "sibling-two") {
		t.Errorf("the second log holds %q, want its own output", body)
	}

	// The same command twice still shares one log, which is the behavior the
	// original comment described and the only thing it was right about.
	repeat, _ := adm.RunQuiet("unit-pkg", []string{"sh", "-c", "echo FAIL\tsibling-one"}, "", nil)
	if repeat.Log != first.Log {
		t.Errorf("the same work wrote %s then %s, want one log", first.Log, repeat.Log)
	}
}

// TestQuietRunTakesADirectoryAndAnEnvironment drives the exported quiet run the
// way another le package calls it: with the directory the child runs in and the
// environment it runs under.
//
// VALIDATES: the child runs in the named directory, sees a variable only the
// named environment carries, and its output still lands in the quiet log.
// PREVENTS: the quiet run dropping either argument and running the child in the
// checkout root under this process's environment, which for `le test unit pkg`
// would run `go test` without the toolchain's cgo and cache settings.
func TestQuietRunTakesADirectoryAndAnEnvironment(t *testing.T) {
	adm := quietAdmission(t)
	adm.Out = &strings.Builder{}

	dir := filepath.Join(adm.Root, "child-dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("create the child's directory: %v", err)
	}
	environ := append(os.Environ(), "QUIET_FIXTURE_MARK=named-environment")

	report, code := adm.RunQuiet("unit-pkg", []string{"sh", "-c", "pwd -P; echo \"mark=$QUIET_FIXTURE_MARK\""}, dir, environ)
	if code != 0 {
		t.Fatalf("the quiet run answered %d, want 0", code)
	}
	if report.Log == "" {
		t.Fatal("the report named no log")
	}

	body := read(t, filepath.Join(adm.Root, report.Log))
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve the child's directory: %v", err)
	}
	if !strings.Contains(body, resolved+"\n") {
		t.Errorf("the child did not run in %s; the log holds:\n%s", resolved, body)
	}
	if !strings.Contains(body, "mark=named-environment\n") {
		t.Errorf("the child did not see the named environment; the log holds:\n%s", body)
	}
}
