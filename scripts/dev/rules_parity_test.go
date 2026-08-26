// The migration's proof for the rules-corpus gates: the script and the command
// answer the same thing.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. Over this checkout and the
// fixture trees, rules_lint.py and rules_points.py each match the corresponding
// `le rules <verb>` command. Their exit codes and text match.
// PREVENTS: a port that agrees with its script on a clean corpus but differs on
// a violation. These five gates determine whether the rule corpus is coherent.
// A check that silently stops can pass every unit test while the repository
// incorrectly reports that all instructions remain enforced.
//
// It also pins the fail-open defects that the port FIXED but the scripts still
// have. Such a case asserts that the SCRIPT still passes. When somebody repairs
// the script, the case fails and must be deleted with the script it describes.
//
// A FIXTURE case copies both scripts into the fixture tree. Neither script
// reads ZE_REPO_ROOT. Each gets its checkout from its own __file__. A script
// left here would judge this checkout while the command judged the fixture.
// Copying the scripts makes both halves use one tree.
//
// Helpers carry a rulesPy prefix. Two other steps are porting into this same
// package, and two helpers of one name cannot both exist.

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/rules"
)

// The two scripts this file compares against.
const (
	rulesPyLint   = "rules_lint.py"
	rulesPyPoints = "rules_points.py"
)

// rulesPyTree writes a fixture checkout carrying both scripts, and points the
// command at it. The files map is repo-relative.
func rulesPyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := devPyTree(t, files)
	source := filepath.Join(devPyRoot(t), "scripts", "dev")
	dest := filepath.Join(root, "scripts", "dev")
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatalf("fixture scripts directory: %v", err)
	}
	for _, name := range []string{rulesPyLint, rulesPyPoints} {
		body, err := os.ReadFile(filepath.Join(source, name)) // #nosec G304 -- a tracked script path
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dest, name), body, 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	devPyPointAt(t, root)
	return root
}

// rulesPyRunScript runs the copy of a script that sits INSIDE tree.
//
// devPyRunScript runs the copy in this checkout. That is correct for tools that
// get their tree from ZE_REPO_ROOT, but not for these two scripts. Each gets its
// checkout from its own __file__, so this copy always judges this checkout.
func rulesPyRunScript(t *testing.T, tree, script string, args []string) devPyResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	argv := append([]string{filepath.Join(tree, "scripts", "dev", script)}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a fixture path and a test's own arguments
	cmd.Dir = tree
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v: %s", script, err, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// rulesPyRelative rewrites the absolute paths the scripts print into the
// tree-relative ones the commands print.
//
// This is the port's one deliberate difference here. It follows the decision
// that step 7 made for the codegen ports. A payload is data before it becomes a
// page. An absolute build-host path in `| json` describes the machine, not the
// corpus.
func rulesPyRelative(tree, text string) string {
	return strings.ReplaceAll(text, tree+string(filepath.Separator), "")
}

// rulesPyLintAnswer runs the lint command over the tree the environment names.
func rulesPyLintAnswer(t *testing.T, tree string) devPyResult {
	t.Helper()
	return devPyRunCommand(t, "rules", func(args []string) (any, int) {
		if len(args) != 1 || args[0] != "lint" {
			t.Fatalf("the case named %v, want exactly [lint]", args)
		}
		report, err := rules.Lint(tree)
		if err != nil {
			t.Fatalf("Lint: %v", err)
		}
		if report.Failed() {
			return report, 1
		}
		return report, 0
	}, []string{"lint"})
}

func TestRulesLintBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, rulesPyLint, nil, root)
	command := rulesPyLintAnswer(t, root)

	devPyAgree(t, "rules lint over the checkout", script, command, script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "state an RFC 2119 level") {
		t.Errorf("the command read no point over the real checkout: %q", command.Stdout)
	}
	if command.Stderr != "" {
		t.Errorf("the command wrote to stderr over a clean checkout: %q", command.Stderr)
	}
}

func TestRulesLintBothHalvesReportTheSameViolations(t *testing.T) {
	// One rule per violation the linter can find, so the comparison is over a
	// whole page rather than one message.
	tree := rulesPyTree(t, map[string]string{
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		"ai/rules/aaa-directive-trigger.md": "# A\n\n**When:** all commands MUST follow this\n" +
			"**Severity:** blocking\n\n## S\n\n- **MUST do it.**\n",
		"ai/rules/bbb-severity.md": "# B\n\n**When:** when it happens\n" +
			"**Severity:** advisory\n\n## S\n\nThis one is BLOCKING.\n",
		"ai/rules/ccc-order.md": "# C\n\n**Severity:** blocking\n**When:** when it happens\n" +
			"\n## S\n\n- **MUST do it.**\n",
		"ai/rules/ddd-related.md": "# D\n\n**When:** when it happens\n**Severity:** blocking\n" +
			"**Related:** ai/rules/x.md\n\n## S\n\n- **MUST do it.**\n",
		"ai/rules/points/eee/manifest.md": "---\ntitle: E\nwhen: when it happens\nseverity: blocking\n---\ns ## S\n  p\n",
		"ai/rules/points/eee/s/p.md":      "---\nkind: directive\nlevel:\n---\n- Do the thing.\n",
	})

	script := rulesPyRunScript(t, tree, rulesPyLint, nil)
	command := rulesPyLintAnswer(t, tree)

	devPyAgree(t, "rules lint over a violating tree", script, command,
		rulesPyRelative(tree, script.Stdout), command.Stdout)
	if script.Code != 1 {
		t.Errorf("the script exited %d over a violating tree, want 1", script.Code)
	}
}

func TestRulesLintBothHalvesReportTheSamePointViolations(t *testing.T) {
	// The rules in this tree are clean, but its points are not. Thus, RFC 2119
	// pass reports the problem. The script stops at the first failed pass, so
	// this page is reachable only when every rule conforms.
	tree := rulesPyTree(t, map[string]string{
		"go.mod":                                "module example.com/fixture\n",
		"feature-gates.txt":                     "ze_core\n",
		"ai/rules/aaa.md":                       "# A\n\n**When:** when it happens\n**Severity:** blocking\n\n## S\n\n- **MUST do it.**\n",
		"ai/rules/points/aaa/manifest.md":       "---\ntitle: A\nwhen: when it happens\nseverity: blocking\n---\ns ## S\n  p\n",
		"ai/rules/points/aaa/s/p.md":            "---\nkind: directive\nlevel: MUST\n---\n- **MUST do it.**\n",
		"ai/rules/points/aaa/s/no-keyword.md":   "---\nkind: directive\nlevel:\n---\n- Do the thing.\n",
		"ai/rules/points/aaa/s/lower-modal.md":  "---\nkind: directive\nlevel: MUST\n---\n- **MUST do it**, you should too.\n",
		"ai/rules/points/aaa/s/wrong-level.md":  "---\nkind: directive\nlevel: MAY\n---\n- **MUST do it.**\n",
		"ai/rules/points/aaa/s/no-matter.md":    "kind: directive\n",
		"ai/rules/points/aaa/s/unknown-rank.md": "---\nkind: directive\nlevel: SHALL\n---\n- **MUST do it.**\n",
	})

	script := rulesPyRunScript(t, tree, rulesPyLint, nil)
	command := rulesPyLintAnswer(t, tree)

	devPyAgree(t, "rules lint over violating points", script, command,
		rulesPyRelative(tree, script.Stdout), command.Stdout)
	if !strings.Contains(command.Stdout, "RFC 2119 language") {
		t.Errorf("the RFC 2119 pass did not report: %q", command.Stdout)
	}
}

func TestScriptRulesLintStillPassesAnEmptyCorpus(t *testing.T) {
	// The script reports success over a corpus it read nothing from: no rule
	// file, and no ai/rules/points/ tree at all. rules_points.py already
	// refuses the same shape in render_all and report_gate_map, so this is one
	// gate of the family disagreeing with the other two.
	//
	// The port refuses both. This case asserts that the SCRIPT still passes. When
	// somebody repairs the script, the case fails and must be deleted with the
	// script. The row in plan/journal/zero-value-as-valid-answer.md must also
	// close.
	tree := rulesPyTree(t, map[string]string{
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		"ai/rules/.keep":    "\n",
	})

	script := rulesPyRunScript(t, tree, rulesPyLint, nil)
	if script.Code != 0 || !strings.Contains(script.Stdout, "0 rule file(s) conform") {
		t.Fatalf("the script no longer passes an empty corpus (exit %d): %s%s\n"+
			"If it was repaired, delete this case with the script and close the row in\n"+
			"plan/journal/zero-value-as-valid-answer.md",
			script.Code, script.Stdout, script.Stderr)
	}

	command := rulesPyLintAnswer(t, tree)
	if command.Code != 1 {
		t.Errorf("the command passed an empty corpus (exit %d): %s", command.Code, command.Stdout)
	}
	for _, want := range []string{"no rule file under ai/rules/", "no rule point file under ai/rules/points/"} {
		if !strings.Contains(command.Stdout, want) {
			t.Errorf("the command does not name %q: %s", want, command.Stdout)
		}
	}
}

// rulesPyRenderAnswer runs the render command over the tree the case names.
func rulesPyRenderAnswer(t *testing.T, tree string, check bool) devPyResult {
	t.Helper()
	verb := "render-update"
	if check {
		verb = "render-check"
	}
	return devPyRunCommand(t, "rules", func(_ []string) (any, int) {
		report, err := rules.RenderAll(tree,
			filepath.Join(tree, "ai", "rules"),
			filepath.Join(tree, "ai", "rules", "points"), check)
		if err != nil {
			t.Fatalf("RenderAll: %v", err)
		}
		if report.Failed() {
			return report, 1
		}
		return report, 0
	}, []string{verb})
}

func TestRulesRenderCheckBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, rulesPyPoints, []string{"render", "--check"}, root)
	command := rulesPyRenderAnswer(t, root, true)

	devPyAgree(t, "rules render-check over the checkout", script, command, script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "rules are fresh") {
		t.Errorf("the command rendered nothing over the real checkout: %q", command.Stdout)
	}
}

func TestRulesRenderCheckBothHalvesPrintTheSameDrift(t *testing.T) {
	// The complete answer from this gate for a stale rule is a unified diff.
	// This comparison confirms that the ported difflib groups its hunks like the
	// Python implementation.
	tree := rulesPyTree(t, rulesPyFixtureCorpus("- **MUST do the OLD thing.**"))

	script := rulesPyRunScript(t, tree, rulesPyPoints, []string{"render", "--check"})
	command := rulesPyRenderAnswer(t, tree, true)

	devPyAgree(t, "rules render-check over a stale rule", script, command,
		rulesPyRelative(tree, script.Stderr), command.Stdout)
	if !strings.Contains(command.Stdout, "@@ -") {
		t.Errorf("the command printed no diff hunk: %q", command.Stdout)
	}
}

func TestRulesRenderUpdateBothHalvesWriteTheSameBytes(t *testing.T) {
	// A WRITING tool is compared by the bytes it leaves behind. Two halves that
	// print one line and write different rules have not been ported: the
	// rendered rule is what every agent reads.
	files := rulesPyFixtureCorpus("- **MUST do the OLD thing.**")
	byScript := rulesPyTree(t, files)
	script := rulesPyRunScript(t, byScript, rulesPyPoints, []string{"render"})

	byCommand := rulesPyTree(t, files)
	command := rulesPyRenderAnswer(t, byCommand, false)

	devPyAgree(t, "rules render-update", script, command, script.Stdout, command.Stdout)

	target := filepath.Join("ai", "rules", "aaa.md")
	fromScript, err := os.ReadFile(filepath.Join(byScript, target)) // #nosec G304 -- a fixture path
	if err != nil {
		t.Fatalf("reading the script's rule: %v", err)
	}
	fromCommand, err := os.ReadFile(filepath.Join(byCommand, target)) // #nosec G304 -- a fixture path
	if err != nil {
		t.Fatalf("reading the command's rule: %v", err)
	}
	if !bytes.Equal(fromScript, fromCommand) {
		t.Errorf("the two halves wrote different rules\nscript:\n%s\ncommand:\n%s", fromScript, fromCommand)
	}
	if !strings.Contains(string(fromCommand), "MUST do the NEW thing") {
		t.Errorf("the command did not rewrite the stale rule: %s", fromCommand)
	}
}

func TestRulesRoundTripBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, rulesPyPoints, []string{"roundtrip"}, root)
	command := devPyRunCommand(t, "rules", func(_ []string) (any, int) {
		out := t.TempDir()
		report, err := rules.RoundTrip(filepath.Join(root, "ai", "rules"), out)
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		if report.Failed() {
			return report, 1
		}
		return report, 0
	}, []string{"points-roundtrip-check"})

	devPyAgree(t, "rules points-roundtrip-check over the checkout", script, command,
		script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "round-trip byte-identical") {
		t.Errorf("the command split nothing over the real checkout: %q", command.Stdout)
	}
}

func TestScriptRulesRoundTripStillPassesAnEmptyCorpus(t *testing.T) {
	// The other script has the same fail-open behavior as the lint. With no rule
	// files, it reports "all 0 rules round-trip byte-identical" and exits 0.
	// render_all refuses the mirror-image shape when all three functions are
	// missing.
	tree := rulesPyTree(t, map[string]string{
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		"ai/rules/.keep":    "\n",
	})

	script := rulesPyRunScript(t, tree, rulesPyPoints, []string{"roundtrip"})
	if script.Code != 0 || !strings.Contains(script.Stdout, "all 0 rules round-trip") {
		t.Fatalf("the script no longer passes an empty corpus (exit %d): %s%s\n"+
			"If it was repaired, delete this case with the script and close the row in\n"+
			"plan/journal/zero-value-as-valid-answer.md",
			script.Code, script.Stdout, script.Stderr)
	}

	command := devPyRunCommand(t, "rules", func(_ []string) (any, int) {
		report, err := rules.RoundTrip(filepath.Join(tree, "ai", "rules"), t.TempDir())
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		if report.Failed() {
			return report, 1
		}
		return report, 0
	}, []string{"points-roundtrip-check"})
	if command.Code != 1 || !strings.Contains(command.Stdout, "read nothing and must not report success") {
		t.Errorf("the command passed an empty corpus (exit %d): %s", command.Code, command.Stdout)
	}
}

// rulesPyCoverageAnswer runs the gate-map command over the tree the case names.
func rulesPyCoverageAnswer(t *testing.T, tree string) devPyResult {
	t.Helper()
	return devPyRunCommand(t, "rules", func(_ []string) (any, int) {
		report, err := rules.Coverage(tree)
		if err != nil {
			t.Fatalf("Coverage: %v", err)
		}
		if report.Failed() {
			return report, 1
		}
		return report, 0
	}, []string{"gate-map-report"})
}

func TestRulesGateMapBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, rulesPyPoints, []string{"coverage"}, root)
	command := rulesPyCoverageAnswer(t, root)

	devPyAgree(t, "rules gate-map-report over the checkout", script, command,
		rulesPyRelative(root, script.Stdout), command.Stdout)
	if !strings.Contains(command.Stdout, "gate map: ") ||
		!strings.Contains(command.Stdout, "PUBLISHED: ") {
		t.Errorf("the command read nothing over the real checkout: %q", command.Stdout)
	}
	// The one deliberate difference is the path FORM, so the two halves must
	// still agree about how many disagreements the published table holds.
	if !strings.Contains(script.Stdout, "PUBLISHED: 0 ") || !strings.Contains(command.Stdout, "PUBLISHED: 0 ") {
		t.Errorf("the two halves disagree about the published table")
	}
}

func TestRulesGateMapBothHalvesReportTheSameFailures(t *testing.T) {
	// This fixture contains every failing dispatcher shape. It has a dangling
	// binding and a binding above no check. It also has a rationale and an
	// exception that name nothing.
	tree := rulesPyTree(t, rulesPyFixtureCoverage())

	script := rulesPyRunScript(t, tree, rulesPyPoints, []string{"coverage"})
	command := rulesPyCoverageAnswer(t, tree)

	devPyAgree(t, "rules gate-map-report over a failing tree", script, command,
		rulesPyRelative(tree, script.Stdout), command.Stdout)
	devPyAgree(t, "rules gate-map-report stderr", script, command,
		rulesPyRelative(tree, script.Stderr), rulesPyDiagnosis(t, tree))
	if script.Code != 1 {
		t.Errorf("the script exited %d over a failing tree, want 1", script.Code)
	}
	// A fixture with no commit has no HEAD, so both ratchets must say they did
	// not run rather than printing a zero.
	if !strings.Contains(command.Stdout, "REGRESSED: no HEAD baseline") {
		t.Errorf("the command printed a dispatcher ratchet with no baseline: %q", command.Stdout)
	}
}

func TestRulesGateMapBothHalvesRatchetAgainstHead(t *testing.T) {
	// The two ratchets need a baseline, and a baseline needs a commit. Without
	// one both halves print "not ratcheted", which is a different assertion
	// from the one this case makes.
	files := rulesPyFixtureCoverage()
	// A dispatcher whose bindings all resolve, so the commit is a clean
	// baseline and the edit below is the only regression.
	files[".claude/hooks/pretool-fixture.py"] = "#!/usr/bin/env python3\n\n\n" +
		"# ze point: alpha/section/first\ndef c_first(ctx):\n    return None\n\n\n" +
		"# ze point: alpha/section/general\ndef c_second(ctx):\n    return None\n\n\n" +
		"CHECKS = (\n    c_first,\n    c_second,\n)\n"
	files["ai/rules/points/alpha/section/first.md"] = "---\nkind: directive\nlevel: MUST\n---\n- **MUST do it.**\n"
	files["ai/rules/points/alpha/section/general.md"] = "---\nkind: directive\nlevel: MUST\n---\n- **MUST do it, in general.**\n"

	tree := rulesPyTree(t, files)
	rulesPyCommit(t, tree)

	// Take one binding away. The point is still on disk and no check names it
	// now, which is the cheapest route from gated to ungated.
	edited := strings.Replace(files[".claude/hooks/pretool-fixture.py"],
		"# ze point: alpha/section/general\n", "", 1)
	if err := os.WriteFile(filepath.Join(tree, ".claude", "hooks", "pretool-fixture.py"),
		[]byte(edited), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	script := rulesPyRunScript(t, tree, rulesPyPoints, []string{"coverage"})
	command := rulesPyCoverageAnswer(t, tree)

	devPyAgree(t, "rules gate-map-report against a HEAD baseline", script, command,
		rulesPyRelative(tree, script.Stdout), command.Stdout)
	if !strings.Contains(command.Stdout, "REGRESSED: 1 point(s)") ||
		!strings.Contains(command.Stdout, "alpha/section/general") {
		t.Errorf("the ratchet did not fire: %q", command.Stdout)
	}
	if script.Code != 1 {
		t.Errorf("the script exited %d over a regression, want 1", script.Code)
	}
}

// rulesPyCommit gives a fixture tree a HEAD, which is what the two ratchets
// compare against. The identity is the command's own: a test must not read the
// developer's git configuration, and a repository with none has no HEAD at all.
func rulesPyCommit(t *testing.T, tree string) {
	t.Helper()

	argv := [][]string{
		{"add", "--all"},
		{"-c", "user.email=fixture@example.com", "-c", "user.name=fixture",
			"commit", "--quiet", "--no-gpg-sign", "-m", "fixture"},
	}
	for _, args := range argv {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", tree}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", args[0], err, out)
		}
	}
}

// rulesPyDiagnosis answers the stderr lines the command would write, which the
// script writes as it goes. They are compared apart from stdout because the
// script interleaves the two streams and a merged capture cannot be ordered.
func rulesPyDiagnosis(t *testing.T, tree string) string {
	t.Helper()

	report, err := rules.Coverage(tree)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	var out strings.Builder
	for _, line := range report.Diagnosis {
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

// rulesPyFixtureCoverage answers a checkout whose dispatcher carries one good
// binding and one of every failing shape.
func rulesPyFixtureCoverage() map[string]string {
	dispatcher := `#!/usr/bin/env python3
"""A fixture dispatcher."""


# ze point: alpha/section/first
def c_first(ctx):
    return None


# ze point: alpha/section/renamed
def c_second(ctx):
    return None


# ze point: alpha/section/first
x = 1


CHECKS = (
    c_first,
    c_second,
)
`
	doc := "# Repo Maintenance\n\n**When:** when maintaining the repo\n**Severity:** blocking\n\n" +
		"## Hook-to-Rule Mapping\n\n### The `pretool-fixture.py` dispatcher\n\n" +
		"| Check | Enforces | Triggers on |\n|---|---|---|\n" +
		"| `c_first` | `alpha.md` | always |\n" +
		"| `c_second` | `alpha.md` | always |\n"

	return map[string]string{
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		".claude/settings.json": `{"hooks": {"PreToolUse": [{"matcher": "Write",` +
			` "hooks": [{"type": "command", "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/pretool-fixture.py"}]}]}}` + "\n",
		".claude/hooks/pretool-fixture.py": dispatcher,
		"ai/rules/repo-maintenance.md":     doc,
		"ai/rules/points/alpha/manifest.md": "---\ntitle: A\nwhen: when it happens\nseverity: blocking\n---" +
			"\nsection ## Section\n  first\n  general\n",
		"ai/rules/points/alpha/section/first.md": "---\nkind: directive\nlevel: MUST\n" +
			"rationale: ai/rationale/absent.md\n---\n- **MUST do it.**\n",
		"ai/rules/points/alpha/section/general.md": "---\nkind: directive\nlevel: MUST\n" +
			"excepted-by: alpha/section/gone\n---\n- **MUST do it, in general.**\n",
	}
}

// rulesPyFixtureCorpus answers a checkout holding one rule and its point tree,
// with the RENDERED rule carrying the body given. Passing a body the points do
// not produce makes the rule stale.
func rulesPyFixtureCorpus(renderedBody string) map[string]string {
	return map[string]string{
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		"ai/rules/aaa.md": "# A\n\n**When:** when it happens\n**Severity:** blocking\n\n" +
			"## S\n\n" + renderedBody + "\n",
		"ai/rules/points/aaa/manifest.md": "---\ntitle: A\nwhen: when it happens\nseverity: blocking\n---\ns ## S\n  p\n",
		"ai/rules/points/aaa/s/p.md":      "---\nkind: directive\nlevel: MUST\n---\n- **MUST do the NEW thing.**\n",
	}
}
