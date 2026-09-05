// VALIDATES: the in-flight failure query. An agent asks whether a verification
// run that has not finished has reddened one of its own files, and gets an
// answer from the stage logs already on disk, qualified by how much of the run
// has judged nothing yet.
// PREVENTS: the query becoming a way to commit over a red. Every branch that
// cannot see the whole run answers undetermined, and no absence -- no run, no
// stage log, a red that named no file -- is ever rendered as a pass
// (ai/rules/principles.md).

package verify

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// redsRootedAt points lepath.Root at a fixture checkout for one test, and puts
// the previous value back after it.
//
// t.Setenv is not enough: internal/core/env builds its cache from os.Environ
// once per process, and this package reads the key long before this test runs,
// so a bare t.Setenv leaves the query reading the real checkout.
func redsRootedAt(t *testing.T, root string) {
	t.Helper()
	previous := env.Get(lepath.RootKey)
	if err := env.Set(lepath.RootKey, root); err != nil {
		t.Fatalf("point the checkout root at the fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set(lepath.RootKey, previous); err != nil {
			t.Fatalf("restore the checkout root: %v", err)
		}
	})
}

// redsFixtureWrite writes one file under root, creating its directory.
func redsFixtureWrite(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("create the directory for %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// redsStageLog renders one stage log the way the engine writes it: the header,
// whatever the stage printed, then the closing result line that proves the log
// is whole.
func redsStageLog(stage string, code int, body ...string) string {
	lines := append([]string{"### Stage: " + stage}, body...)
	lines = append(lines, "### Stage result: "+stage+" exit="+strconv.Itoa(code), "")

	return strings.Join(lines, "\n")
}

// redsGroupLines render a declared failure group and the count that closes it.
func redsGroupLines(id, kind, summary string, related ...string) []string {
	quoted := make([]string, 0, len(related))
	for _, path := range related {
		quoted = append(quoted, `"`+path+`"`)
	}

	return []string{
		`VERIFY FAILURE GROUP: {"group-id":"` + id + `","kind":"` + kind +
			`","related":[` + strings.Join(quoted, ",") + `],"summary":"` + summary +
			`","rerun":"le fixture"}`,
		"VERIFY FAILURE GROUPS COMPLETE: 1",
	}
}

// redsUnfinishedRun builds a checkout holding one full-mode run that has
// written four stage logs, one of them caught mid-write, and answers the root.
func redsUnfinishedRun(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	redsFixtureWrite(t, root, "internal/mine/a.go", "package mine\n")
	redsFixtureWrite(t, root, "internal/theirs/b.go", "package theirs\n")
	run := "tmp/verify/full-fixture"
	redsFixtureWrite(t, root, run+"/01-verify-lint-run.log", redsStageLog("verify lint/run", 1,
		redsGroupLines("lint:theirs", "lint", "golangci-lint reported findings",
			"internal/theirs/b.go")...))
	redsFixtureWrite(t, root, run+"/02-tier-check.log", redsStageLog("tier/check", 0, "OK"))
	redsFixtureWrite(t, root, run+"/03-rfc-check.log", redsStageLog("rfc/check", 2,
		"the stage broke before it could say which files it was about"))
	// A log the reader met between the engine's write and its last byte. It
	// carries a failure group and no result line, so it MUST count as a stage
	// that has not reported rather than as a red naming internal/mine/a.go.
	redsFixtureWrite(t, root, run+"/04-doc-wiring.log", strings.Join(append(
		[]string{"### Stage: doc wiring"},
		redsGroupLines("files:mine", "files", "a design reference does not resolve",
			"internal/mine/a.go")...), "\n"))

	return root
}

func TestRedsAnswersFromAnUnfinishedRun(t *testing.T) {
	root := redsUnfinishedRun(t)
	redsRootedAt(t, root)
	stages := len(verifyengine.StagesForMode(verifyengine.Mode))
	if stages < 10 {
		t.Fatalf("the full stage population is %d, so this fixture cannot be a partial run", stages)
	}

	answer, code := Answer([]string{"reds", "file", "internal/mine/a.go"})

	reds, ok := answer.(Reds)
	if !ok {
		t.Fatalf("the action answered %T, want Reds", answer)
	}
	if reds.Verdict != VerdictUndetermined {
		t.Errorf("verdict for an untouched path is %q, want %q", reds.Verdict, VerdictUndetermined)
	}
	if code == 0 {
		t.Errorf("an unfinished run answered 0, which reads as a pass")
	}
	if reds.Stages != stages || reds.Reported != 3 || reds.Pending != stages-3 {
		t.Errorf("counts are %d stages, %d reported, %d pending; want %d, 3, %d",
			reds.Stages, reds.Reported, reds.Pending, stages, stages-3)
	}
	if reds.Finished {
		t.Errorf("a run with no combined log was reported as finished")
	}
	if len(reds.Naming) != 0 {
		t.Errorf("reds naming internal/mine/a.go = %#v, want none: only the truncated log named it", reds.Naming)
	}
	if len(reds.Unattributed) != 1 || reds.Unattributed[0].Stage != "rfc/check" {
		t.Errorf("unattributed reds = %#v, want rfc/check alone", reds.Unattributed)
	}
	text := reds.Text()
	if !strings.Contains(text, strconv.Itoa(stages-3)+" not reported yet") {
		t.Errorf("the rendered answer does not state the unreported remainder:\n%s", text)
	}

	named, namedCode := Answer([]string{"reds", "file", "internal/theirs/b.go"})

	theirs, ok := named.(Reds)
	if !ok {
		t.Fatalf("the action answered %T, want Reds", named)
	}
	if theirs.Verdict != VerdictNamed || namedCode == 0 {
		t.Fatalf("verdict for the reddened path is %q at exit %d, want %q non-zero",
			theirs.Verdict, namedCode, VerdictNamed)
	}
	if len(theirs.Naming) != 1 || theirs.Naming[0].Stage != "verify lint/run" {
		t.Errorf("reds naming internal/theirs/b.go = %#v, want verify lint/run alone", theirs.Naming)
	}
}

func TestRedsRefusesToAnswerWithNoRun(t *testing.T) {
	root := t.TempDir()
	redsFixtureWrite(t, root, "internal/mine/a.go", "package mine\n")
	redsRootedAt(t, root)

	answer, code := Answer([]string{"reds", "file", "internal/mine/a.go"})

	reds, ok := answer.(Reds)
	if !ok {
		t.Fatalf("the action answered %T, want Reds", answer)
	}
	if reds.Verdict != VerdictNoRun {
		t.Fatalf("verdict with no run at all is %q, want %q", reds.Verdict, VerdictNoRun)
	}
	if code == 0 {
		t.Fatalf("no run answered 0, which reads as a pass over a tree nothing judged")
	}
	if reds.Reported != 0 || reds.Run != "" {
		t.Errorf("no run reported %d stage(s) from run %q", reds.Reported, reds.Run)
	}
	if !strings.Contains(reds.Text(), "no verification run") {
		t.Errorf("the rendered answer does not say no run exists:\n%s", reds.Text())
	}
}

func TestRedsCountsAFinishedRunAndClearsAPathNoRedNames(t *testing.T) {
	root := t.TempDir()
	redsFixtureWrite(t, root, "internal/mine/a.go", "package mine\n")
	redsFixtureWrite(t, root, "internal/theirs/b.go", "package theirs\n")
	run := "tmp/verify/changed-fixture"
	stages := verifyengine.StagesForMode(verifyengine.ChangedMode)
	if len(stages) == 0 {
		t.Fatal("the changed mode has no stage population")
	}
	for index, stage := range stages {
		body := []string{"OK"}
		code := 0
		if index == 0 {
			code = 1
			body = redsGroupLines("lint:theirs", "lint", "findings", "internal/theirs/b.go")
		}
		name := strings.NewReplacer("/", "-", " ", "-").Replace(stage.Identity.Name)
		redsFixtureWrite(t, root, run+"/"+strconv.Itoa(index+1)+"-"+name+".log",
			redsStageLog(stage.Identity.Name, code, body...))
	}
	redsFixtureWrite(t, root, run+"/ze-verify.log", "the combined log the run writes when it ends\n")
	redsRootedAt(t, root)

	answer, code := Answer([]string{"reds", "file", "internal/mine/a.go"})

	reds, ok := answer.(Reds)
	if !ok {
		t.Fatalf("the action answered %T, want Reds", answer)
	}
	if reds.Verdict != VerdictNotNamed || code != 0 {
		t.Fatalf("verdict for a path no red names in a whole run is %q at exit %d, want %q at 0",
			reds.Verdict, code, VerdictNotNamed)
	}
	if reds.Pending != 0 || !reds.Finished || reds.Mode != verifyengine.ChangedMode {
		t.Errorf("a whole changed-mode run reads as %d pending, finished=%v, mode %q",
			reds.Pending, reds.Finished, reds.Mode)
	}
}

func TestRedsGrammarPutsTheKeywordBeforeTheValue(t *testing.T) {
	if _, code := Answer([]string{"reds", "internal/mine/a.go"}); code != 2 {
		t.Errorf("a bare path after the action answered %d, want 2", code)
	}
	if _, code := Answer([]string{"reds", "file"}); code != 2 {
		t.Errorf("the file keyword with no value answered %d, want 2", code)
	}
	if _, code := Answer([]string{"reds", "file", "/etc/passwd"}); code != 2 {
		t.Errorf("an absolute path answered %d, want 2", code)
	}
	listing, code := Answer([]string{})
	if code != 0 {
		t.Fatalf("the area listing answered %d", code)
	}
	list, ok := listing.(leaction.List)
	if !ok {
		t.Fatalf("the area listing is %T", listing)
	}
	for _, row := range list.Actions {
		if row.Verb == "reds" {
			if row.Writes {
				t.Errorf("reds is listed as an action that writes")
			}

			return
		}
	}
	t.Errorf("reds is not in the verify area listing, so no reader discovers it")
}
