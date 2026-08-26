// The migration's proof for the six rule-digest gates: the script and the
// command answer the same thing.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. Over this checkout, two
// `git archive HEAD` exports, and fixture trees, each script matches its
// `le rules <verb>` command. This covers rules_index.py, rules_condensed.py, and
// rules_router.py. The exit codes, text, and written bytes match.
// PREVENTS: an incorrect port of the two GENERATORS that every session loads.
// TRIGGERS.md and CORE.md are the instruction payload. A generator can match a
// verdict but write different bytes for every agent. The generator itself is
// also the check that notices this difference.
//
// It also pins the fail-open defects that the port FIXED but the scripts still
// have. Such a case asserts that the SCRIPT still passes. When somebody repairs
// the script, the case fails and must be deleted with the script it describes.
//
// A FIXTURE case copies all three scripts into the fixture tree. These scripts
// do not read ZE_REPO_ROOT. Each gets its checkout from its own __file__. A
// script left here would therefore judge this checkout while the command judged
// the fixture. rules_condensed and rules_router import each other, so all three
// scripts must travel together.
//
// Helpers carry a digestPy prefix. Three other steps are porting into this same
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

// The three scripts this file compares against. They move as one: each imports
// at least one of the others.
var digestPyScripts = [...]string{"rules_index.py", "rules_condensed.py", "rules_router.py"}

// digestPyTree writes a fixture checkout carrying all three scripts, and points
// the command at it. The files map is repo-relative.
func digestPyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := devPyTree(t, files)
	digestPyCopyScripts(t, root)
	devPyPointAt(t, root)
	return root
}

// digestPyCopyScripts puts this checkout's scripts inside the tree they are to
// judge.
func digestPyCopyScripts(t *testing.T, tree string) {
	t.Helper()

	source := filepath.Join(devPyRoot(t), "scripts", "dev")
	dest := filepath.Join(tree, "scripts", "dev")
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatalf("fixture scripts directory: %v", err)
	}
	for _, name := range digestPyScripts {
		body, err := os.ReadFile(filepath.Join(source, name)) // #nosec G304 -- a tracked script path
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dest, name), body, 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}

// digestPyRunScript runs the copy of a script that sits INSIDE tree.
func digestPyRunScript(t *testing.T, tree, script string, args ...string) devPyResult {
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

// digestPyCommand runs one `le rules <verb>` over the tree the environment
// names, through the area's own dispatch.
func digestPyCommand(t *testing.T, verb string) devPyResult {
	t.Helper()
	return devPyRunCommand(t, "rules", rules.Answer, []string{verb})
}

// digestPyRelative rewrites the absolute path the two generators print for the
// file they wrote into the tree-relative one the commands print.
//
// This is the port's one deliberate difference here. It follows the decision
// that step 7 made for the codegen ports. A payload is data before it becomes a
// page. An absolute build-host path in `| json` describes the machine, not the
// corpus.
func digestPyRelative(tree, text string) string {
	return strings.ReplaceAll(text, tree+string(filepath.Separator), "")
}

// digestPyRefusal answers the stderr line the command writes for a tree it
// refuses.
//
// The line goes to the process stderr instead of an injectable writer. Thus,
// this test rebuilds it from the library error with the spelling from
// leaction.ReportError. The coverage diagnosis uses the same technique: compare
// the DATUM instead of a capture of a global.
func digestPyRefusal(t *testing.T, tree, verb string) string {
	t.Helper()

	var err error
	switch verb {
	case "condensed-check":
		_, err = rules.Digest(tree, true)
	case "router-report":
		_, err = rules.Router(tree)
	default:
		t.Fatalf("no refusal is defined for %q", verb)
	}
	if err == nil {
		t.Fatalf("%s answered no error over a tree the script refused", verb)
	}

	var line strings.Builder
	line.WriteString("error: ")
	line.WriteString(err.Error())
	line.WriteString("\n")
	return line.String()
}

// digestPyLadderCorpus is the smallest fixture whose precedence ladder
// resolves: the ladder, the two rules its rungs name, and one routed rule.
func digestPyLadderCorpus() map[string]string {
	return map[string]string{
		"ai/rules/rule-precedence.md": "# Rule Precedence\n\n" +
			"**When:** when two rules disagree\n**Severity:** blocking\n\n" +
			"## The Ladder\n\n" +
			"| Rung | Governs | Rules | What it does |\n" +
			"|------|---------|-------|--------------|\n" +
			"| 1 | Destruction | `never-destroy-work` | STOP and ask |\n" +
			"| 2 | Outside correctness | `rfc-compliance` | Implement it |\n" +
			"| 3 | Scope | `completion` | Never reduce scope |\n",
		"ai/rules/never-destroy-work.md": "# Never Destroy Work\n\n" +
			"**When:** before deleting a file holding uncommitted work\n" +
			"**Severity:** blocking\n\n## Directives\n\n- MUST ask first.\n",
		"ai/rules/rfc-compliance.md": "# RFC Compliance\n\n" +
			"**When:** writing protocol code for any RFC ze implements\n" +
			"**Severity:** blocking\n\n## Directives\n\n- MUST conform.\n",
		"ai/rules/completion.md": "# Completion\n\n" +
			"**When:** before claiming any gokrazy qdisc work done\n" +
			"**Severity:** blocking\n\n## Directives\n\n" +
			"The first sentence is the statement. The second is elaboration.\n\n" +
			"## Why It Matters\n\n- This bullet must not survive.\n",
		"plan/spec-thing.md": "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n",
	}
}

// ─── The four read-only gates, over this checkout ───────────────────────────

func TestRulesDigestReadOnlyGatesBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)
	devPyPointAt(t, root)

	cases := []struct {
		verb   string
		script string
		args   []string
		says   string
	}{
		{"index-check", "rules_index.py", []string{"--check"}, "up to date"},
		{"condensed-check", "rules_condensed.py", []string{"--check"}, "up to date"},
		{"payload-report", "rules_condensed.py", []string{"--payload"}, "budget:"},
		{"router-report", "rules_router.py", nil, "past task descriptions"},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			script := devPyRunScript(t, tc.script, tc.args, root)
			command := digestPyCommand(t, tc.verb)

			devPyAgree(t, tc.verb, script, command, script.Stdout, command.Stdout)
			if !strings.Contains(command.Stdout, tc.says) {
				t.Errorf("the command read nothing over the real checkout: %q", command.Stdout)
			}
		})
	}
}

// The read-only gates must have READ the corpus, not merely agreed about it. A
// gate that read nothing would agree with a script that read nothing too.
func TestRulesDigestGatesReadTheWholeCheckout(t *testing.T) {
	root := devPyRoot(t)
	devPyPointAt(t, root)

	index := digestPyCommand(t, "index-check")
	if !strings.Contains(index.Stdout, "rules, ai/rules/INDEX.md") {
		t.Errorf("the index check names no count: %q", index.Stdout)
	}
	condensed := digestPyCommand(t, "condensed-check")
	for _, want := range []string{"ai/rules/TRIGGERS.md", "ai/rules/CORE.md"} {
		if !strings.Contains(condensed.Stdout, want) {
			t.Errorf("the digest check never reached %s: %q", want, condensed.Stdout)
		}
	}
	router := digestPyCommand(t, "router-report")
	if strings.Contains(router.Stdout, "corpus: 0 past task descriptions") {
		t.Errorf("the router read no spec: %q", router.Stdout)
	}
	payload := digestPyCommand(t, "payload-report")
	if strings.Contains(payload.Stdout, "ai/rules/CORE.md: 0 chars") {
		t.Errorf("the payload measured an empty core: %q", payload.Stdout)
	}
}

// ─── The two WRITING gates, over two exports of the committed tree ──────────

// A generator parity test that compares only the VERDICT tests one word.
// `git archive HEAD` gives both halves the same input, but the working tree
// cannot. Several sessions edit it at once. A rule file changed between the two
// runs would make the halves disagree even when neither half was wrong.
func TestRulesDigestBothHalvesWriteTheSameBytes(t *testing.T) {
	byScript := discoveryExport(t)
	byCommand := discoveryExport(t)
	devPyPointAt(t, byCommand)

	cases := []struct {
		verb    string
		script  string
		args    []string
		written []string
	}{
		{"index-update", "rules_index.py", nil, []string{"ai/rules/INDEX.md"}},
		{"condensed-update", "rules_condensed.py", nil,
			[]string{"ai/rules/TRIGGERS.md", "ai/rules/CORE.md"}},
	}
	for _, tc := range cases {
		script := digestPyRunScript(t, byScript, tc.script, tc.args...)
		command := digestPyCommand(t, tc.verb)

		devPyAgree(t, tc.verb, script, command,
			digestPyRelative(byScript, script.Stdout), command.Stdout)

		for _, rel := range tc.written {
			want := devPyRead(t, filepath.Join(byScript, filepath.FromSlash(rel)))
			got := devPyRead(t, filepath.Join(byCommand, filepath.FromSlash(rel)))
			if want == got {
				continue
			}
			t.Errorf("%s: the two halves wrote different %s (%d bytes against %d)",
				tc.verb, rel, len(want), len(got))
		}
	}

	// The generators must have WRITTEN the corpus, not agreed on an empty file.
	index := devPyRead(t, filepath.Join(byCommand, "ai", "rules", "INDEX.md"))
	if strings.Count(index, "\n| ") < 20 {
		t.Errorf("the index holds %d rows", strings.Count(index, "\n| "))
	}
	core := devPyRead(t, filepath.Join(byCommand, "ai", "rules", "CORE.md"))
	if !strings.Contains(core, "<!-- always-on: precedence rung 1/2 -->") {
		t.Errorf("the core names no ladder member")
	}
}

// ─── The fixtures: the branches the checkout never reaches ──────────────────

func TestRulesIndexBothHalvesNameTheSameRuleWithNoSummary(t *testing.T) {
	files := digestPyLadderCorpus()
	files["ai/rules/quiet.md"] = "# Quiet\n\n- only a bullet\n\n| a | b |\n"
	tree := digestPyTree(t, files)

	script := digestPyRunScript(t, tree, "rules_index.py", "--check")
	command := digestPyCommand(t, "index-check")
	devPyAgree(t, "index-check over a rule with no trigger", script, command,
		script.Stdout, command.Stdout)
	if script.Code != 1 {
		t.Fatalf("a rule with no derivable summary answered %d", script.Code)
	}
	if !strings.Contains(command.Stdout, "quiet.md") {
		t.Errorf("the page does not name the rule: %q", command.Stdout)
	}
}

func TestRulesDigestBothHalvesWriteTheSameFixtureTree(t *testing.T) {
	byScript := digestPyTree(t, digestPyLadderCorpus())
	byCommand := digestPyTree(t, digestPyLadderCorpus())
	devPyPointAt(t, byCommand)

	for _, tc := range []struct {
		verb    string
		script  string
		written []string
	}{
		{"index-update", "rules_index.py", []string{"ai/rules/INDEX.md"}},
		{"condensed-update", "rules_condensed.py",
			[]string{"ai/rules/TRIGGERS.md", "ai/rules/CORE.md"}},
	} {
		script := digestPyRunScript(t, byScript, tc.script)
		command := digestPyCommand(t, tc.verb)
		devPyAgree(t, tc.verb, script, command,
			digestPyRelative(byScript, script.Stdout), command.Stdout)
		for _, rel := range tc.written {
			want := devPyRead(t, filepath.Join(byScript, filepath.FromSlash(rel)))
			got := devPyRead(t, filepath.Join(byCommand, filepath.FromSlash(rel)))
			if want != got {
				t.Errorf("%s: the halves wrote different %s\nscript:\n%s\ncommand:\n%s",
					tc.verb, rel, want, got)
			}
		}
	}

	// The condenser drops the denylisted section but preserves the rule
	// statement. Thus, this fixture tests the condenser instead of a header.
	core := devPyRead(t, filepath.Join(byCommand, "ai", "rules", "CORE.md"))
	if strings.Contains(core, "must not survive") {
		t.Errorf("a denylisted section reached the core: %q", core)
	}
}

func TestBothHalvesRefuseTheSameUnreadableLadder(t *testing.T) {
	base := digestPyLadderCorpus()
	ladder := base["ai/rules/rule-precedence.md"]

	cases := []struct {
		name   string
		ladder string
	}{
		{"the Rung column is renamed", strings.Replace(ladder, "| Rung |", "| Level |", 1)},
		{"no rung 1 or 2 row survives", strings.NewReplacer(
			"| 1 | Destruction", "| 4 | Destruction",
			"| 2 | Outside correctness", "| 5 | Outside correctness").Replace(ladder)},
		{"the rungs name no rule under ai/rules", strings.NewReplacer(
			"`never-destroy-work`", "`CLAUDE.md`",
			"`rfc-compliance`", "`AGENTS.md`").Replace(ladder)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := digestPyLadderCorpus()
			files["ai/rules/rule-precedence.md"] = tc.ladder
			tree := digestPyTree(t, files)

			for _, pair := range []struct{ verb, script, arg string }{
				{"condensed-check", "rules_condensed.py", "--check"},
				{"router-report", "rules_router.py", ""},
			} {
				args := []string{}
				if pair.arg != "" {
					args = append(args, pair.arg)
				}
				script := digestPyRunScript(t, tree, pair.script, args...)
				command := digestPyCommand(t, pair.verb)
				if script.Code != 1 || command.Code != 1 {
					t.Errorf("%s: an unreadable ladder answered %d and %d",
						pair.verb, script.Code, command.Code)
				}
				if got := digestPyRefusal(t, tree, pair.verb); got != script.Stderr {
					t.Errorf("%s: the refusals differ.\nscript:\n%s\ncommand:\n%s",
						pair.verb, script.Stderr, got)
				}
			}
		})
	}
}

func TestRulesRouterBothHalvesAgreeOverAFixtureCorpus(t *testing.T) {
	files := digestPyLadderCorpus()
	// A second spec has only ONE distinctive word in common with the routed
	// rule's trigger. Thus, the first task selects the rule, but this task does
	// not. The two halves must agree on that threshold.
	files["plan/spec-other.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy loader.\n"
	files["plan/TEMPLATE.md"] = "# T\n\n## Task\n\nthe template is never corpus\n"
	tree := digestPyTree(t, files)

	script := digestPyRunScript(t, tree, "rules_router.py")
	command := digestPyCommand(t, "router-report")
	devPyAgree(t, "router-report over a fixture corpus", script, command,
		script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "corpus: 2 past task descriptions") {
		t.Errorf("the template reached the corpus, or a spec did not: %q", command.Stdout)
	}
	if !strings.Contains(command.Stdout, "min 0, max 1") {
		t.Errorf("both tasks surfaced the same rules, so the threshold proves nothing: %q",
			command.Stdout)
	}
}

// An EMPTY task corpus is the one case where the halves must differ. The script
// prints its warning TWICE, once in each artifact builder. The command prints it
// once and carries the fact in the payload so `| json` can reach it. The exit
// code, page, and written bytes must still match.
func TestBothHalvesReportAnEmptyTaskCorpusAndStillWriteTheSameBytes(t *testing.T) {
	files := digestPyLadderCorpus()
	delete(files, "plan/spec-thing.md")
	byScript := digestPyTree(t, files)
	byCommand := digestPyTree(t, files)

	script := digestPyRunScript(t, byScript, "rules_condensed.py")
	command := digestPyCommand(t, "condensed-update")
	devPyAgree(t, "condensed-update over an empty corpus", script, command,
		digestPyRelative(byScript, script.Stdout), command.Stdout)

	warnings := 0
	for _, line := range strings.Split(script.Stderr, "\n") {
		if strings.HasPrefix(line, "warning: the task corpus is empty") {
			warnings++
		}
	}
	if warnings != 2 {
		t.Errorf("the script printed the warning %d times, want 2 (one per builder): %q",
			warnings, script.Stderr)
	}

	report, err := rules.Digest(byCommand, true)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !report.EmptyCorpus {
		t.Errorf("the payload does not carry the empty corpus: %+v", report)
	}
	for _, rel := range []string{"ai/rules/TRIGGERS.md", "ai/rules/CORE.md"} {
		want := devPyRead(t, filepath.Join(byScript, filepath.FromSlash(rel)))
		got := devPyRead(t, filepath.Join(byCommand, filepath.FromSlash(rel)))
		if want != got {
			t.Errorf("the halves wrote different %s over an empty corpus", rel)
		}
	}
}

// ─── The fail-open defects the port fixed and the scripts still carry ───────

// TestScriptRulesIndexStillOverwritesAnIndexItReadNothingFor reddens the day
// somebody repairs rules_index.py. The answer then is to delete this case with
// the script and close the row in plan/journal/zero-value-as-valid-answer.md.
func TestScriptRulesIndexStillOverwritesAnIndexItReadNothingFor(t *testing.T) {
	const keep = "# Ze Rules Index\n\ncontent that must survive\n"
	// One tree per half, because the first half to run would otherwise leave
	// the second reading what it wrote.
	byScript := digestPyTree(t, map[string]string{"ai/rules/INDEX.md": keep})
	byCommand := digestPyTree(t, map[string]string{"ai/rules/INDEX.md": keep})

	script := digestPyRunScript(t, byScript, "rules_index.py")
	if script.Code != 0 {
		t.Fatalf("the script now refuses an empty corpus (code %d): fix is landed, delete this case", script.Code)
	}
	if strings.Contains(devPyRead(t, filepath.Join(byScript, "ai", "rules", "INDEX.md")), "must survive") {
		t.Fatalf("the script no longer overwrites the index: fix is landed, delete this case")
	}

	command := digestPyCommand(t, "index-update")
	if command.Code != 1 {
		t.Errorf("the command answered %d for a corpus it read nothing from", command.Code)
	}
	if _, err := rules.Index(byCommand, false); err == nil ||
		!strings.Contains(err.Error(), "must not report success") {
		t.Errorf("the refusal does not say what went wrong: %v", err)
	}
	if !strings.Contains(devPyRead(t, filepath.Join(byCommand, "ai", "rules", "INDEX.md")), "must survive") {
		t.Errorf("the command overwrote the index it refused to build")
	}
}

// TestScriptRulesPayloadStillCountsAnAbsentFileAsZero reddens the day somebody
// repairs rules_condensed.py. The answer then is to delete this case with the
// script and close the row in plan/journal/zero-value-as-valid-answer.md.
func TestScriptRulesPayloadStillCountsAnAbsentFileAsZero(t *testing.T) {
	tree := digestPyTree(t, map[string]string{
		"ai/INSTRUCTIONS.md":   "instructions\n",
		"ai/rules/TRIGGERS.md": "triggers\n",
	})

	script := digestPyRunScript(t, tree, "rules_condensed.py", "--payload")
	if script.Code != 0 {
		t.Fatalf("the script now refuses an absent payload file (code %d): fix is landed, delete this case",
			script.Code)
	}
	if !strings.Contains(script.Stdout, "ai/rules/CORE.md: 0 chars") ||
		!strings.Contains(script.Stdout, "-- MET") {
		t.Fatalf("the script no longer reads an absent file as zero: fix is landed, delete this case\n%s",
			script.Stdout)
	}

	command := digestPyCommand(t, "payload-report")
	if command.Code != 1 {
		t.Errorf("the command answered %d for a payload that is not there", command.Code)
	}
	if !strings.Contains(command.Stdout, "the payload is incomplete") {
		t.Errorf("the page does not qualify the verdict: %q", command.Stdout)
	}
	// The measured lines are unchanged: the port adds a verdict, it does not
	// rewrite the numbers the script prints.
	if !strings.Contains(command.Stdout, "ai/rules/CORE.md: 0 chars") {
		t.Errorf("the port changed the measurement rather than qualifying it: %q", command.Stdout)
	}
}
