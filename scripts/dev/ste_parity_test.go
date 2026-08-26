// The migration's proof for the STE prose gates: the script and the command
// answer the same thing.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. Over fixture repositories
// with all three writing surfaces, ste_check.py matches each `le ste <verb>`
// command. Their exit codes and text match.
// PREVENTS: a ratchet that agrees with its script on a clean tree but differs on
// a grown tree. commit_helper.py runs the ratchet for every commit. A silently
// stopped check can pass every unit test while the repository incorrectly
// reports that it still measures the prose.
//
// The WHOLE-CHECKOUT comparison is absent by a cost decision, not by mistake.
// Python reviews 8120 documents in 38 seconds, and Go takes 44 seconds. Race
// instrumentation then makes the Go run exceed the target timeout.
// `test/ui/le-ste-answers.ci` instead drives the BUILT binary over a
// `git archive HEAD` export. It compares the page and payload byte for byte.
//
// It also pins the three fail-open defects that the port FIXED but the script
// still has. Each case asserts that the SCRIPT still passes. When somebody
// repairs the script, the case fails and must be deleted with the script that
// it describes.
//
// A FIXTURE case copies the script into the fixture tree. The script does not
// read ZE_REPO_ROOT. It gets its checkout from its own `__file__`. Thus, the
// copy in this checkout would judge this checkout while the command judged the
// fixture. Copying the script makes both halves use one tree.
//
// Helpers carry a stePy prefix. Several steps are porting into this same
// package, and two helpers of one name cannot both exist.

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/letools/ste"
)

// steScript is the tool this file compares against.
const steScript = "ste_check.py"

// stePyTree writes a fixture checkout carrying the script, commits it, and
// points the command at it.
//
// The ratchet compares each file with its own HEAD version. A fixture without a
// commit has no baseline, so both halves would measure nothing.
func stePyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := devPyTree(t, files)
	dest := filepath.Join(root, "scripts", "dev")
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatalf("fixture scripts directory: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(devPyRoot(t), "scripts", "dev", steScript)) // #nosec G304 -- a tracked script path
	if err != nil {
		t.Fatalf("reading %s: %v", steScript, err)
	}
	if err := os.WriteFile(filepath.Join(dest, steScript), body, 0o600); err != nil {
		t.Fatalf("writing %s: %v", steScript, err)
	}

	stePyGit(t, root, "add", "--all")
	stePyGit(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=test",
		"commit", "--quiet", "-m", "fixture")
	devPyPointAt(t, root)
	return root
}

// stePyWrite changes one file of a fixture after its commit, which is what
// makes it a candidate for the ratchet.
func stePyWrite(t *testing.T, root, rel, body string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("fixture file: %v", err)
	}
}

// stePyGit runs one git command in the fixture.
func stePyGit(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// stePyRunScript runs the copy of the script that sits INSIDE tree.
//
// devPyRunScript runs the copy in this checkout. That is correct for tools that
// get their tree from ZE_REPO_ROOT, but not for this script. It gets the
// checkout from its own `__file__`, so this copy always judges this checkout.
func stePyRunScript(t *testing.T, tree string, args ...string) devPyResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	argv := append([]string{filepath.Join(tree, "scripts", "dev", steScript)}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a fixture path and a test's own arguments
	cmd.Dir = tree
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v: %s", steScript, err, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// stePyCommand runs one `le ste` action through the binary path. leroot.Run
// splits the pipe chain, calls the tool, and renders the payload.
func stePyCommand(t *testing.T, args ...string) devPyResult {
	t.Helper()
	return devPyRunCommand(t, "ste", ste.Answer, args)
}

// stePyBoth answers what the script printed and what the command printed for
// one action, over the tree the fixture points at.
//
// A failed script verdict goes to stderr, and a passing verdict goes to stdout.
// The command always puts its verdict on stdout because a port moves a verdict
// to the payload. Only a real failure reaches stderr. Thus, the test joins both
// script streams and compares them with command stdout.
func stePyBoth(t *testing.T, tree string, scriptArgs, commandArgs []string) (script, command devPyResult) {
	t.Helper()

	script = stePyRunScript(t, tree, scriptArgs...)
	command = stePyCommand(t, commandArgs...)
	return script, command
}

// stePyAgree fails unless the two halves answered the same code and the same
// text.
func stePyAgree(t *testing.T, what string, script, command devPyResult) {
	t.Helper()
	devPyAgree(t, what, script, command, script.Stdout+script.Stderr, command.Stdout)
}

// ─── The ratchet, over fixtures ─────────────────────────────────────────────

// steFixtureFiles is the smallest fixture that carries all three surfaces.
func steFixtureFiles() map[string]string {
	return map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/guide.md":     "# Guide\n\nThe daemon starts the session.\n",
		"internal/a/a.go":   "package a\n\n// The daemon starts the session.\nfunc A() {}\n",
		"internal/a/a.yang": "module a {\n  leaf b {\n    description \"The daemon starts the session.\";\n  }\n}\n",
	}
}

func TestSteCheckBothHalvesReportTheSameGrowth(t *testing.T) {
	tree := stePyTree(t, steFixtureFiles())
	stePyWrite(t, tree, "docs/guide.md",
		"# Guide\n\nThe daemon may start the session. It should typically work.\n")

	script, command := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a grown file", script, command)

	if command.Code != 3 {
		t.Errorf("a grown habit answers 3, got %d", command.Code)
	}
}

func TestSteCheckBothHalvesSeeAGrownHabitOnEverySurface(t *testing.T) {
	tree := stePyTree(t, steFixtureFiles())
	stePyWrite(t, tree, "internal/a/a.go",
		"package a\n\n// The daemon may start the session.\nfunc A() {}\n")
	stePyWrite(t, tree, "internal/a/a.yang",
		"module a {\n  leaf b {\n    description \"The daemon may start the session.\";\n  }\n}\n")

	script, command := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a Go comment and a YANG description", script, command)

	// The test asserts the absolute verdict, not only agreement. Both halves can
	// scan NEITHER surface, answer 0, and agree. That result would prove the
	// opposite of the case name.
	if command.Code != 3 {
		t.Errorf("a grown habit in a Go comment and a YANG description answers 3, got %d", command.Code)
	}
}

func TestSteCheckBothHalvesPassWhenTheHabitShrank(t *testing.T) {
	tree := stePyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/guide.md":     "# Guide\n\nThe daemon may start. It should stop.\n",
	})
	stePyWrite(t, tree, "docs/guide.md", "# Guide\n\nThe daemon starts. It stops.\n")

	script, command := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over prose that improved", script, command)

	if command.Code != 0 {
		t.Errorf("prose that improved answers 0, got %d", command.Code)
	}
}

func TestSteCheckBothHalvesFollowARename(t *testing.T) {
	// The baseline follows the rename, so moving a legacy document does not
	// report its whole inherited content as new.
	tree := stePyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/old.md":       "# Guide\n\nThe daemon may start. It should stop.\n",
	})
	stePyGit(t, tree, "mv", "docs/old.md", "docs/new.md")

	script, command := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a renamed document", script, command)

	if command.Code != 0 {
		t.Errorf("a rename carries its baseline, so it answers 0, got %d", command.Code)
	}

	// The commit-time form has its OWN rename lookup, and the commit helper uses
	// this lookup. Without it, the baseline is `HEAD:docs/new.md`, which does not
	// exist. Every finding in the moved document then appears new, and a pure
	// rename fails the gate.
	namedScript, namedCommand := stePyBoth(t, tree,
		[]string{"--check", "docs/new.md"}, []string{"check", "file", "docs/new.md"})
	stePyAgree(t, "ste check naming the renamed document", namedScript, namedCommand)
	if namedCommand.Code != 0 {
		t.Errorf("the named form lost the rename's baseline, got %d: %s",
			namedCommand.Code, namedCommand.Stdout)
	}
}

func TestSteCheckBothHalvesGateOnlyTheNamedFiles(t *testing.T) {
	// The commit-time form attributes files to one commit because several
	// sessions share a checkout. The working tree is the wrong attribution
	// unit. commit_helper.py passes the paths after `--check`. The command puts
	// each path after a `file` keyword because the CLI rule bans paths in
	// untyped positional slots.
	tree := stePyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/mine.md":      "# Mine\n\nThe daemon starts.\n",
		"docs/theirs.md":    "# Theirs\n\nThe daemon starts.\n",
	})
	stePyWrite(t, tree, "docs/mine.md", "# Mine\n\nThe daemon starts cleanly.\n")
	stePyWrite(t, tree, "docs/theirs.md", "# Theirs\n\nThe daemon may start. It should stop.\n")

	script, command := stePyBoth(t, tree,
		[]string{"--check", "docs/mine.md"}, []string{"check", "file", "docs/mine.md"})
	stePyAgree(t, "ste check scoped to one commit's files", script, command)

	if command.Code != 0 {
		t.Errorf("another session's file must not fail this commit, got %d", command.Code)
	}

	// The same tree gated on the other file fails, which is what says the scope
	// was read rather than the whole tree ignored.
	bothScript, bothCommand := stePyBoth(t, tree,
		[]string{"--check", "docs/theirs.md"}, []string{"check", "file", "docs/theirs.md"})
	stePyAgree(t, "ste check scoped to the grown file", bothScript, bothCommand)
	if bothCommand.Code != 3 {
		t.Errorf("the named file grew a habit and must fail, got %d", bothCommand.Code)
	}
}

func TestSteReviewChangedBothHalvesAgreeOverAFixture(t *testing.T) {
	tree := stePyTree(t, steFixtureFiles())
	stePyWrite(t, tree, "docs/guide.md",
		"# Guide\n\nThe daemon may start the session. It should typically work.\n")

	script, command := stePyBoth(t, tree, []string{"--changed"}, []string{"review-changed"})
	stePyAgree(t, "ste review-changed over a fixture", script, command)
}

func TestSteBothHalvesSkipAGeneratedDocument(t *testing.T) {
	tree := stePyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/gen.md":       "GENERATED by tool\n\nThe daemon starts.\n",
	})
	stePyWrite(t, tree, "docs/gen.md", "GENERATED by tool\n\nThe daemon may start. It should stop.\n")

	script, command := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a generated document", script, command)

	if command.Code != 0 {
		t.Errorf("a generated document belongs to its producer, got %d", command.Code)
	}
}

func TestSteBothHalvesLeaveASpecOutOfThePopulation(t *testing.T) {
	// A document that is DELETED when the work closes is not worth an STE edit
	// (owner directive, 2026-08-10).
	tree := stePyTree(t, map[string]string{
		"go.mod":             "module fixture\n",
		"feature-gates.txt":  "ze_core\n",
		"plan/spec-thing.md": "# Spec\n\nThe daemon starts.\n",
		"plan/journal/x.md":  "# Journal\n\nThe daemon starts.\n",
	})
	stePyWrite(t, tree, "plan/spec-thing.md", "# Spec\n\nThe daemon may start. It should stop.\n")

	script, command := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a spec", script, command)
	if command.Code != 0 {
		t.Errorf("a spec is outside the population, got %d", command.Code)
	}

	// The durable record beside it IS in the population, which is what says the
	// exclusion is the glob rather than the whole directory.
	stePyWrite(t, tree, "plan/journal/x.md", "# Journal\n\nThe daemon may start. It should stop.\n")
	journalScript, journalCommand := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a journal page", journalScript, journalCommand)
	if journalCommand.Code != 3 {
		t.Errorf("plan/journal outlives every spec and is gated, got %d", journalCommand.Code)
	}
}

// ─── The three fail-open defects the port FIXED ─────────────────────────────

func TestScriptSteStillPassesAGitItCannotAsk(t *testing.T) {
	// `git_lines` answers None when git cannot run, and `candidates` converts
	// that value to an empty list. The gate then reports "no habit grew in 0
	// changed document(s)" and exits 0 for an unread tree. An unborn HEAD reaches
	// this path with the least setup. commit_helper.py calls the same entry point.
	//
	// See plan/journal/zero-value-as-valid-answer.md. Delete this case with the
	// script at step 14.
	tree := devPyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/guide.md":     "# Guide\n\nThe daemon may start. It should typically stop.\n",
	})
	dest := filepath.Join(tree, "scripts", "dev")
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatalf("fixture scripts directory: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(devPyRoot(t), "scripts", "dev", steScript)) // #nosec G304 -- a tracked script path
	if err != nil {
		t.Fatalf("reading %s: %v", steScript, err)
	}
	if err := os.WriteFile(filepath.Join(dest, steScript), body, 0o600); err != nil {
		t.Fatalf("writing %s: %v", steScript, err)
	}
	devPyPointAt(t, tree)

	script := stePyRunScript(t, tree, "--check")
	if script.Code != 0 {
		t.Fatalf("the script no longer fails open on an unanswerable git (exit %d): "+
			"delete this case and close the journal row\n%s%s",
			script.Code, script.Stdout, script.Stderr)
	}

	command := stePyCommand(t, "check")
	if command.Code == 0 {
		t.Errorf("the port must refuse a population git could not name, got %d: %s",
			command.Code, command.Stdout)
	}
}

func TestScriptSteStillPassesAFileItCannotRead(t *testing.T) {
	// `ratchet` uses `except OSError: continue` when it cannot read a changed
	// document. That document contributes no habit count or growth. The gate
	// reports success because it cannot examine the document.
	//
	// See plan/journal/gate-excludes-part-of-its-population.md. Delete this case
	// with the script at step 14.
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}

	tree := stePyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/guide.md":     "# Guide\n\nThe daemon starts.\n",
	})
	stePyWrite(t, tree, "docs/guide.md", "# Guide\n\nThe daemon may start. It should typically stop.\n")

	unreadable := filepath.Join(tree, "docs", "guide.md")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("making the document unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

	script := stePyRunScript(t, tree, "--check")
	if script.Code != 0 {
		t.Fatalf("the script no longer fails open on an unreadable document (exit %d): "+
			"delete this case and close the journal row\n%s%s",
			script.Code, script.Stdout, script.Stderr)
	}

	command := stePyCommand(t, "check")
	if command.Code == 0 {
		t.Errorf("the port must refuse a document it cannot measure, got %d: %s",
			command.Code, command.Stdout)
	}
}

func TestScriptSteStillHonorsAnOptOutWithNoReason(t *testing.T) {
	// `IGNORE_FILE` ends `(?P<reason>.+?)\s*(?:-->|$)`, so for `<!-- ste:
	// ignore-file -->` the non-greedy group swallows the comment's own
	// terminator and the `$` branch closes the match. The reason reads `-->`
	// and the whole document leaves the population of a gate that blocks
	// commits, on an exemption the guide calls invalid.
	//
	// See plan/journal/escape-hatch-scoped-wider-than-its-justification.md.
	// Delete this case with the script at step 14.
	tree := stePyTree(t, map[string]string{
		"go.mod":            "module fixture\n",
		"feature-gates.txt": "ze_core\n",
		"docs/guide.md":     "# Guide\n\nThe daemon starts.\n",
	})
	stePyWrite(t, tree, "docs/guide.md",
		"<!-- ste: ignore-file -->\n\n# Guide\n\nThe daemon may start. It should typically stop.\n")

	script := stePyRunScript(t, tree, "--check")
	if script.Code != 0 {
		t.Fatalf("the script no longer honors an opt-out with no reason (exit %d): "+
			"delete this case and close the journal row\n%s%s",
			script.Code, script.Stdout, script.Stderr)
	}

	command := stePyCommand(t, "check")
	if command.Code != 3 {
		t.Errorf("the port must review a document whose opt-out states no reason, got %d: %s",
			command.Code, command.Stdout)
	}

	// A reason that IS stated is honored by both halves, which is what says the
	// hatch still works rather than being closed.
	stePyWrite(t, tree, "docs/guide.md",
		"<!-- ste: ignore-file quotes RFC text at length -->\n\n# Guide\n\nThe daemon may start. It should stop.\n")
	reasoned, reasonedCommand := stePyBoth(t, tree, []string{"--check"}, []string{"check"})
	stePyAgree(t, "ste check over a reasoned opt-out", reasoned, reasonedCommand)
	if reasonedCommand.Code != 0 {
		t.Errorf("a stated reason must still exempt the document, got %d", reasonedCommand.Code)
	}
}
