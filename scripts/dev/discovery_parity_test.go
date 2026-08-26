// This file proves the migration of the DISCOVERY cluster in this directory.
// The three generator and router scripts answer the same results as their
// commands. The two generators also write the same BYTES.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for `docs_to_code.py`,
// `package_map.py`, and `verify_wiring_docs.py`. Each script and its command
// write byte-identical index files from the committed and fixture trees. They
// also select the same gates and answer the same exit code.
// PREVENTS: a generator that agrees with its script on the WORD "current" but
// emits different bytes. The commit helper refuses a commit when the committed
// `ai/PACKAGE-MAP.md` disagrees with the tree. Different port bytes would make
// that gate invalid for every session on this machine. All output comparisons
// would still pass.
//
// The byte comparison uses a `git archive HEAD` export instead of the working
// tree because several sessions edit this checkout at once. A file written
// between the two runs would otherwise make the halves disagree about a tree
// that neither half got wrong. HEAD gives both halves the same 10,562 Go files.
//
// This file is deliberately HERE instead of beside the new packages. It is a
// migration artifact, so the commit that deletes the scripts also deletes their
// proof. Its name identifies the cluster because two other steps port code into
// this package.
//
// It also pins the fail-open defect that the port FIXED but the script still
// has. This case asserts that the SCRIPT still fails open. When somebody repairs
// the script, the case fails and must be deleted with the script.

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/discoveryindex"
	"github.com/ze-software/ze/letools/docstocode"
	"github.com/ze-software/ze/letools/docwiring"
)

const (
	packageMapScript  = "package_map.py"
	docsToCodeScript  = "docs_to_code.py"
	wiringDocsScript  = "verify_wiring_docs.py"
	packageMapCommand = "discovery-index"
	docsToCodeCommand = "docs-to-code"
	wiringDocsCommand = "doc-wiring"
)

// discoveryExport materializes the committed tree into a fresh directory.
//
// A `git archive HEAD` export gives both halves the required population. It
// contains the whole repository and is deterministic. It is also unaffected by
// other sessions that edit this checkout during the test.
func discoveryExport(t *testing.T) string {
	t.Helper()

	dest := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	archive := exec.CommandContext(ctx, "git", "archive", "HEAD")
	archive.Dir = devPyRoot(t)
	var archiveErr strings.Builder
	archive.Stderr = &archiveErr

	untar := exec.CommandContext(ctx, "tar", "-x", "-C", dest)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		t.Fatalf("git archive pipe: %v", err)
	}
	untar.Stdin = pipe
	var untarErr strings.Builder
	untar.Stderr = &untarErr

	if err := untar.Start(); err != nil {
		t.Fatalf("tar start: %v", err)
	}
	if err := archive.Run(); err != nil {
		t.Fatalf("git archive: %v: %s", err, archiveErr.String())
	}
	if err := untar.Wait(); err != nil {
		t.Fatalf("tar: %v: %s", err, untarErr.String())
	}
	return dest
}

// discoveryGenerate runs both halves of one generator over two copies of one
// tree and answers the two files they wrote.
//
// Each half gets its own copy because both write to the same path in their
// input tree. The script gets its tree from `--root`, and the command gets it
// from ZE_REPO_ROOT. The environment variable replaces the flag.
func discoveryGenerate(t *testing.T, script, command, output string, makeTree func(*testing.T) string) (string, string) {
	t.Helper()

	scriptTree := makeTree(t)
	commandTree := makeTree(t)

	scriptRun := devPyRunScript(t, script, []string{"--root", scriptTree}, scriptTree)
	if scriptRun.Code != 0 {
		t.Fatalf("the script exited %d: %s%s", scriptRun.Code, scriptRun.Stdout, scriptRun.Stderr)
	}

	devPyPointAt(t, commandTree)
	answer := discoveryAnswer(command)
	commandRun := devPyRunCommand(t, command, answer, []string{"update"})
	if commandRun.Code != 0 {
		t.Fatalf("the command exited %d: %s%s", commandRun.Code, commandRun.Stdout, commandRun.Stderr)
	}

	return devPyRead(t, filepath.Join(scriptTree, filepath.FromSlash(output))),
		devPyRead(t, filepath.Join(commandTree, filepath.FromSlash(output)))
}

// discoveryAnswer names the command under test.
func discoveryAnswer(command string) func([]string) (any, int) {
	if command == packageMapCommand {
		return discoveryindex.Answer
	}
	return docstocode.Answer
}

func TestPackageMapBothHalvesWriteTheSameBytesOverTheCommittedTree(t *testing.T) {
	fromScript, fromCommand := discoveryGenerate(t, packageMapScript, packageMapCommand,
		discoveryindex.OutputRel, discoveryExport)

	if fromScript != fromCommand {
		t.Errorf("the two halves wrote different package maps:\n%s", firstDifference(fromScript, fromCommand))
	}
	// A comparison of two empty files proves nothing, and an export that failed
	// to unpack would produce exactly that.
	if strings.Count(fromCommand, "\n| `") < 100 {
		t.Errorf("the map describes almost nothing, so the comparison is vacuous:\n%s", fromCommand)
	}
}

func TestDocsToCodeBothHalvesWriteTheSameBytesOverTheCommittedTree(t *testing.T) {
	fromScript, fromCommand := discoveryGenerate(t, docsToCodeScript, docsToCodeCommand,
		docstocode.OutputRel, discoveryExport)

	if fromScript != fromCommand {
		t.Errorf("the two halves wrote different design-doc indexes:\n%s", firstDifference(fromScript, fromCommand))
	}
	if strings.Count(fromCommand, "\n## `") < 50 {
		t.Errorf("the index names almost no document, so the comparison is vacuous:\n%s", fromCommand)
	}
}

// discoveryFixture answers a small tree that tests shapes absent from the real
// checkout. These shapes include a truncated summary and a bar in a
// responsibility. They also include a constant name, an embed-only directory,
// and a citation list that crosses from bullets into a table.
func discoveryFixture(t *testing.T) string {
	t.Helper()

	long := strings.Repeat("word ", 60)
	return devPyTree(t, map[string]string{
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		"ai/.keep":          "",

		// A doc comment continued across lines, with the sentence ending on the
		// second one.
		"internal/core/alpha/doc.go":   "// Package alpha is the first thing\n// in the tree. Second sentence.\npackage alpha\n",
		"internal/core/alpha/alpha.go": "// Design: docs/architecture/one.md -- alpha\npackage alpha\n",

		// A responsibility longer than the summary bound, so both halves must
		// cut it at the same character.
		"internal/core/beta/beta.go": "// Package beta " + long + "end\npackage beta\n" +
			"// Design: docs/architecture/one.md : colon separator\n",

		// A bar in the responsibility, which the table cell must escape.
		"internal/core/gamma/gamma.go": "// Package gamma reads a | b\npackage gamma\n",

		// No package doc, so the register.go Description is the responsibility,
		// and the registered name is held in a constant rather than a literal.
		"internal/plugins/delta/delta.go": "package delta\n",
		"internal/plugins/delta/register.go": "package delta\n\nconst pluginName = \"delta-plugin\"\n\n" +
			"var _ = Registration{Name: pluginName, Description: \"the delta plugin\"}\n",

		// A directory holding only an embed, which contributes no row.
		"internal/plugins/delta/yang/embed.go": "package yang\n",

		// Neither a doc nor a register.go: the responsibility is the worklist
		// marker.
		"cmd/tool/main.go": "package main\n",

		// Four citations of one document, which crosses from bullets to a table,
		// one of them carrying a bar the cell must escape.
		"pkg/one/a.go": "// Design: docs/architecture/two.md -- a | b\npackage one\n",
		"pkg/one/b.go": "// Design: docs/architecture/two.md -- b\npackage one\n",
		"pkg/one/c.go": "// Design: docs/architecture/two.md — em dash\npackage one\n",
		"pkg/one/d.go": "// Design: docs/architecture/two.md\npackage one\n",

		// Not a design document: no directory in the token.
		"pkg/two/two.go": "// Design: (none yet)\npackage two\n",

		// A test file, which neither index reads for a package doc.
		"pkg/two/two_test.go": "// Package two IS NOT the doc\npackage two\n",
	})
}

func TestPackageMapBothHalvesWriteTheSameBytesOverAFixture(t *testing.T) {
	fromScript, fromCommand := discoveryGenerate(t, packageMapScript, packageMapCommand,
		discoveryindex.OutputRel, discoveryFixture)

	if fromScript != fromCommand {
		t.Errorf("the two halves wrote different package maps:\n%s", firstDifference(fromScript, fromCommand))
	}
	for _, want := range []string{"reads a \\| b", "TODO", "delta-plugin", "..."} {
		if !strings.Contains(fromCommand, want) {
			t.Errorf("the fixture never exercised %q, so the comparison is weaker than it reads:\n%s", want, fromCommand)
		}
	}
	if strings.Contains(fromCommand, "delta/yang") {
		t.Error("an embed-only directory earned a row")
	}
}

func TestDocsToCodeBothHalvesWriteTheSameBytesOverAFixture(t *testing.T) {
	fromScript, fromCommand := discoveryGenerate(t, docsToCodeScript, docsToCodeCommand,
		docstocode.OutputRel, discoveryFixture)

	if fromScript != fromCommand {
		t.Errorf("the two halves wrote different design-doc indexes:\n%s", firstDifference(fromScript, fromCommand))
	}
	for _, want := range []string{"| File | Topic |", "a \\| b", "em dash", "colon separator"} {
		if !strings.Contains(fromCommand, want) {
			t.Errorf("the fixture never exercised %q, so the comparison is weaker than it reads:\n%s", want, fromCommand)
		}
	}
	if strings.Contains(fromCommand, "(none yet)") {
		t.Error("a placeholder that names no document earned an entry")
	}
}

func TestPackageMapBothHalvesReportTheSameStaleFile(t *testing.T) {
	tree := discoveryFixture(t)
	if err := os.WriteFile(filepath.Join(tree, "ai", "PACKAGE-MAP.md"), []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("writing a stale index: %v", err)
	}
	devPyPointAt(t, tree)

	script := devPyRunScript(t, packageMapScript, []string{"--root", tree, "--check"}, tree)
	command := devPyRunCommand(t, packageMapCommand, discoveryindex.Answer, []string{"check"})

	// The script writes its verdict to stderr, but the command writes it to
	// stdout. A command verdict is payload, so stdout lets `| json` carry it.
	// Only a real failure reaches stderr. This is the one deliberate difference,
	// and the test compares the text byte for byte.
	devPyAgree(t, "discovery-index over a stale tree", script, command, script.Stderr, command.Stdout)
	if script.Code != discoveryindex.StaleExit {
		t.Errorf("the script exited %d over a stale index, want %d: the commit gate reads that code apart from 1",
			script.Code, discoveryindex.StaleExit)
	}
}

func TestDocsToCodeBothHalvesReportTheSameStaleFile(t *testing.T) {
	tree := discoveryFixture(t)
	if err := os.WriteFile(filepath.Join(tree, "ai", "DOCS-TO-CODE.md"), []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("writing a stale index: %v", err)
	}
	devPyPointAt(t, tree)

	script := devPyRunScript(t, docsToCodeScript, []string{"--root", tree, "--check"}, tree)
	command := devPyRunCommand(t, docsToCodeCommand, docstocode.Answer, []string{"check"})

	devPyAgree(t, "docs-to-code over a stale tree", script, command, script.Stderr, command.Stdout)
	if script.Code != docstocode.StaleExit {
		t.Errorf("the script exited %d over a stale index, want %d", script.Code, docstocode.StaleExit)
	}
}

func TestDocsToCodeBothHalvesGenerateAnIndexThatWasNeverThere(t *testing.T) {
	// The index is gitignored, so a fresh clone and a verify worktree start
	// without it. A missing new index is not stale. Both halves write it and
	// report success, which keeps the doc gate green on a clean checkout.
	tree := discoveryFixture(t)
	devPyPointAt(t, tree)

	script := devPyRunScript(t, docsToCodeScript, []string{"--root", tree, "--check"}, tree)
	scriptText := devPyRead(t, filepath.Join(tree, filepath.FromSlash(docstocode.OutputRel)))

	if err := os.Remove(filepath.Join(tree, filepath.FromSlash(docstocode.OutputRel))); err != nil {
		t.Fatalf("removing the index between the halves: %v", err)
	}
	command := devPyRunCommand(t, docsToCodeCommand, docstocode.Answer, []string{"check"})
	commandText := devPyRead(t, filepath.Join(tree, filepath.FromSlash(docstocode.OutputRel)))

	devPyAgree(t, "docs-to-code over a tree with no index", script, command, script.Stdout, command.Stdout)
	if script.Code != 0 {
		t.Errorf("the script exited %d over a missing index, want 0", script.Code)
	}
	if scriptText != commandText {
		t.Errorf("the two halves generated different indexes:\n%s", firstDifference(scriptText, commandText))
	}
}

func TestDocWiringBothHalvesSelectTheSameTargetsOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, wiringDocsScript, []string{"--dry-run"}, root)
	command := devPyRunCommand(t, wiringDocsCommand, docwiring.Answer, []string{"dry-run"})

	devPyAgree(t, "doc-wiring dry run over the checkout", script, command, script.Stdout, command.Stdout)
	if command.Stderr != "" {
		t.Errorf("the command wrote to stderr on a dry run: %q", command.Stderr)
	}
}

// wiringFixture answers a tree whose changed files select several gates and
// exercise each check this gate runs itself.
func wiringFixture(t *testing.T) string {
	t.Helper()

	return devPyTree(t, map[string]string{
		"go.mod":                          "module example.com/fixture\n",
		"feature-gates.txt":               "ze_core\n",
		"test/.ci-sleep-baseline":         "# a comment\n3\n-1\n",
		"test/ui/justified.ci":            "# the daemon has no readiness signal here\ntime.sleep(1)\n",
		"test/ui/trailing.ci":             "time.sleep(1)  # trailing reason\n",
		"test/draft/hidden.ci":            "time.sleep(1)\ntime.sleep(1)\ntime.sleep(1)\n",
		"internal/component/cli/thing.go": "package cli\n\nfunc Exported() {}\n",
		"plan/known-failures/one.md":      "The red is real and its mechanism is unknown.\n",
	})
}

func TestDocWiringBothHalvesAgreeOverAFixture(t *testing.T) {
	tree := wiringFixture(t)
	devPyPointAt(t, tree)

	changed := []string{
		"test/ui/justified.ci",
		"test/ui/trailing.ci",
		"plan/known-failures/one.md",
	}
	scriptArgs := []string{"--root", tree}
	var commandArgs []string
	for _, path := range changed {
		scriptArgs = append(scriptArgs, "--changed-file", path)
		commandArgs = append(commandArgs, "changed-file", path)
	}

	script := devPyRunScript(t, wiringDocsScript, scriptArgs, tree)
	command := devPyRunCommand(t, wiringDocsCommand, docwiring.Answer, commandArgs)

	devPyAgree(t, "doc-wiring over a fixture", script, command, script.Stdout, command.Stdout)
	// The draft incubator holds three more sleeps than the tree outside it. A
	// half that counted them would answer a different number here, and the
	// ceiling is set so that count would fail.
	if !strings.Contains(command.Stdout, "ci-sleep ratchet") {
		t.Errorf("the ratchet never ran, so the draft exclusion is untested:\n%s", command.Stdout)
	}
}

func TestDocWiringBothHalvesDeclareTheSameFailureGroups(t *testing.T) {
	tree := wiringFixture(t)
	if err := os.WriteFile(filepath.Join(tree, "test", "ui", "blind.ci"),
		[]byte("time.sleep(1)\n"), 0o600); err != nil {
		t.Fatalf("writing an unjustified sleep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tree, "plan", "known-failures", "one.md"),
		[]byte("It only fails under load on a contended host.\n"), 0o600); err != nil {
		t.Fatalf("writing a load excuse: %v", err)
	}
	devPyPointAt(t, tree)

	changed := []string{"test/ui/blind.ci", "plan/known-failures/one.md"}
	scriptArgs := []string{"--root", tree}
	var commandArgs []string
	for _, path := range changed {
		scriptArgs = append(scriptArgs, "--changed-file", path)
		commandArgs = append(commandArgs, "changed-file", path)
	}

	script := devPyRunScript(t, wiringDocsScript, scriptArgs, tree)
	command := devPyRunCommand(t, wiringDocsCommand, docwiring.Answer, commandArgs)

	if script.Code != 1 || command.Code != 1 {
		t.Fatalf("the script exited %d and the command exited %d, want 1 from each\nscript:\n%s%s\ncommand:\n%s%s",
			script.Code, command.Code, script.Stdout, script.Stderr, command.Stdout, command.Stderr)
	}

	// The test compares the GROUPS as data instead of text. Each half writes JSON
	// with different encoder spacing, but the verify runner reads the parsed
	// object. A byte comparison would enforce an invisible difference and miss
	// a difference that every consumer sees.
	scriptGroups := parseGroups(t, script.Stdout)
	commandGroups := parseGroups(t, command.Stdout)
	if !slices.Equal(scriptGroups, commandGroups) {
		t.Errorf("the two halves declared different groups:\nscript:  %v\ncommand: %v", scriptGroups, commandGroups)
	}
	// Three checks fail on this tree: the unjustified sleep, the load excuse,
	// and the ratchet the extra sleep pushes over its ceiling. One group each.
	if len(commandGroups) != 3 {
		t.Fatalf("expected one group per failing check, got %v", commandGroups)
	}
	if got := completeCount(t, script.Stdout); got != completeCount(t, command.Stdout) {
		t.Errorf("the two halves published different group counts: %d and %d", got, completeCount(t, command.Stdout))
	}
}

// parseGroups reads every declared failure group out of one half's stdout and
// answers them in a comparable form.
func parseGroups(t *testing.T, text string) []string {
	t.Helper()

	const prefix = "VERIFY FAILURE GROUP: "
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		payload, found := strings.CutPrefix(line, prefix)
		if !found {
			continue
		}
		var group struct {
			GroupID string   `json:"group-id"`
			Kind    string   `json:"kind"`
			Related []string `json:"related"`
			Summary string   `json:"summary"`
			Rerun   string   `json:"rerun"`
		}
		if err := json.Unmarshal([]byte(payload), &group); err != nil {
			t.Fatalf("decoding a group line: %v\n%s", err, line)
		}
		canonical, err := json.Marshal(group)
		if err != nil {
			t.Fatalf("re-encoding a group: %v", err)
		}
		out = append(out, string(canonical))
	}
	return out
}

// completeCount reads the count a run publishes for its declared groups.
func completeCount(t *testing.T, text string) int {
	t.Helper()

	const prefix = "VERIFY FAILURE GROUPS COMPLETE: "
	for line := range strings.SplitSeq(text, "\n") {
		value, found := strings.CutPrefix(line, prefix)
		if !found {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			t.Fatalf("decoding the group count: %v\n%s", err, line)
		}
		return count
	}
	t.Fatalf("no group count in:\n%s", text)
	return 0
}

// TestSleepRatchetScriptStillPassesOverATreeItCannotRead pins the fail-open
// defect the PORT fixed and the script still carries.
//
// The ratchet counts sleeps in the files that it opened. An unreadable `.ci`
// contributes none, and a lower sleep count passes. The script reports that the
// count DROPPED and asks the developer to tighten the ceiling. That new ceiling
// comes from a tree that the script did not read.
//
// It reddens the day somebody repairs the script. The answer then is to delete
// this case with the script and close the row in
// plan/journal/gate-excludes-part-of-its-population.md.
func TestSleepRatchetScriptStillPassesOverATreeItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the case cannot be built")
	}

	tree := devPyTree(t, map[string]string{
		"go.mod":                  "module example.com/fixture\n",
		"feature-gates.txt":       "ze_core\n",
		"test/.ci-sleep-baseline": "2\n",
		"test/ui/a.ci":            "time.sleep(1)  # why\n",
		"test/ui/b.ci":            "time.sleep(1)  # why\n",
	})
	unreadable := filepath.Join(tree, "test", "ui", "b.ci")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	devPyPointAt(t, tree)

	script := devPyRunScript(t, wiringDocsScript,
		[]string{"--root", tree, "--changed-file", "test/ui/a.ci"}, tree)
	if script.Code != 0 || !strings.Contains(script.Stdout, "count dropped to 1") {
		t.Fatalf("the script no longer counts blind (exit %d): %s%s\n"+
			"If it was repaired, delete this case with the script and close the row in\n"+
			"plan/journal/gate-excludes-part-of-its-population.md",
			script.Code, script.Stdout, script.Stderr)
	}

	command := devPyRunCommand(t, wiringDocsCommand, docwiring.Answer,
		[]string{"changed-file", "test/ui/a.ci"})
	if command.Code == 0 {
		t.Errorf("the command passed over a tree it could not read: %s%s", command.Stdout, command.Stderr)
	}
	if !strings.Contains(command.Stdout, "b.ci") {
		t.Errorf("the command did not name the file it could not read:\n%s", command.Stdout)
	}
}

// firstDifference answers the first line where two renderings diverge, with a
// little context. A whole-file diff of a 671-package map is a wall rather than
// an answer.
func firstDifference(left, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	for i := range max(len(leftLines), len(rightLines)) {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		if l != r {
			var out strings.Builder
			out.WriteString("line ")
			out.WriteString(strconv.Itoa(i + 1))
			out.WriteString("\n  script:  ")
			out.WriteString(l)
			out.WriteString("\n  command: ")
			out.WriteString(r)
			return out.String()
		}
	}
	return "the texts are equal but their lengths differ"
}

// readDiscoveryConstants dumps the numbers and tables the three scripts declare,
// so they can be set against the packages' own.
//
// This is the only test here that READS a script instead of running it. None of
// these values is observable in either half's output. A header window one line
// shorter finds the same citations in this tree. A missing port allowlist entry
// silently fails an unobserved gate. A larger group chunk changes nothing until
// a check names 51 files. Output comparisons cannot detect these differences.
const readDiscoveryConstants = `
import json, sys
sys.path.insert(0, sys.argv[1])
import package_map, docs_to_code, discovery_sources, verify_wiring_docs
print(json.dumps({
    "package_map_header_lines": package_map.HEADER_LINES,
    "package_map_max_summary": package_map.MAX_SUMMARY,
    "package_map_roots": list(package_map.ROOTS),
    "package_map_skip_dirs": sorted(package_map.SKIP_DIRS),
    "docs_to_code_header_lines": docs_to_code.HEADER_LINES,
    "docs_to_code_skip_dirs": sorted(docs_to_code.SKIP_DIRS),
    "stale_exit": discovery_sources.STALE_EXIT,
    "outputs": list(discovery_sources.OUTPUTS),
    "generators": list(discovery_sources.GENERATORS),
    "related_per_group": verify_wiring_docs.RELATED_PER_GROUP,
    "target_order": list(verify_wiring_docs.TARGET_ORDER),
    "make_targets": sorted(verify_wiring_docs.MAKE_TARGETS),
    "allowlist": sorted([list(entry) for entry in verify_wiring_docs.WIRING_ALLOWLIST]),
    "functional_suites": [[k, v] for k, v in verify_wiring_docs.FUNCTIONAL_SUITE_BY_AREA.items()],
    "sleep_baseline": verify_wiring_docs.SLEEP_BASELINE,
    "draft_dir": verify_wiring_docs.DRAFT_DIR,
    "path_bearing_kind": verify_wiring_docs.PATH_BEARING_KIND,
    "unattributable_kind": verify_wiring_docs.UNATTRIBUTABLE_KIND,
}))
`

func TestDiscoveryScriptsAndCommandsShareTheSameNumbers(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	devDir := filepath.Join(devPyRoot(t), "scripts", "dev")
	out, err := exec.CommandContext(ctx, "python3", "-c", readDiscoveryConstants, devDir).Output()
	if err != nil {
		t.Fatalf("read the scripts' constants: %v", err)
	}

	var script struct {
		PackageMapHeaderLines int        `json:"package_map_header_lines"`
		PackageMapMaxSummary  int        `json:"package_map_max_summary"`
		PackageMapRoots       []string   `json:"package_map_roots"`
		PackageMapSkipDirs    []string   `json:"package_map_skip_dirs"`
		DocsToCodeHeaderLines int        `json:"docs_to_code_header_lines"`
		DocsToCodeSkipDirs    []string   `json:"docs_to_code_skip_dirs"`
		StaleExit             int        `json:"stale_exit"`
		Outputs               []string   `json:"outputs"`
		Generators            []string   `json:"generators"`
		RelatedPerGroup       int        `json:"related_per_group"`
		TargetOrder           []string   `json:"target_order"`
		MakeTargets           []string   `json:"make_targets"`
		Allowlist             [][]string `json:"allowlist"`
		FunctionalSuites      [][]string `json:"functional_suites"`
		SleepBaseline         string     `json:"sleep_baseline"`
		DraftDir              string     `json:"draft_dir"`
		PathBearingKind       string     `json:"path_bearing_kind"`
		UnattributableKind    string     `json:"unattributable_kind"`
	}
	if err := json.Unmarshal(out, &script); err != nil {
		t.Fatalf("decode the scripts' constants: %v\n%s", err, out)
	}

	for _, number := range []struct {
		what           string
		script, ported int
	}{
		{"the package map's header window", script.PackageMapHeaderLines, discoveryindex.HeaderLines},
		{"the responsibility bound", script.PackageMapMaxSummary, discoveryindex.MaxSummary},
		{"the design-doc index's header window", script.DocsToCodeHeaderLines, docstocode.HeaderLines},
		{"the stale exit code of the package map", script.StaleExit, discoveryindex.StaleExit},
		{"the stale exit code of the design-doc index", script.StaleExit, docstocode.StaleExit},
		{"the paths one failure group carries", script.RelatedPerGroup, docwiring.RelatedPerGroup},
	} {
		if number.script != number.ported {
			t.Errorf("%s: the script uses %d and the command uses %d",
				number.what, number.script, number.ported)
		}
	}

	for _, word := range []struct {
		what           string
		script, ported string
	}{
		{"the sleep baseline's path", script.SleepBaseline, docwiring.SleepBaseline},
		{"the draft incubator's name", script.DraftDir, docwiring.DraftDir},
		{"the package map's own path", script.Outputs[0], discoveryindex.OutputRel},
	} {
		if word.script != word.ported {
			t.Errorf("%s: the script uses %q and the command uses %q",
				word.what, word.script, word.ported)
		}
	}

	for _, list := range []struct {
		what           string
		script, ported []string
	}{
		{"the trees the package map covers", script.PackageMapRoots, discoveryindex.Roots()},
		{"the package map's skipped directories", script.PackageMapSkipDirs, discoveryindex.SkipDirs()},
		{"the design-doc index's skipped directories", script.DocsToCodeSkipDirs, docstocode.SkipDirs()},
		{"the order selected checks run in", script.TargetOrder, docwiring.TargetOrder()},
	} {
		if !slices.Equal(list.script, list.ported) {
			t.Errorf("%s: the script has %v and the command has %v",
				list.what, list.script, list.ported)
		}
	}

	// Output comparison cannot reach the allowlist table.
	// A dropped entry reddens the gate for a symbol that somebody reviewed years ago.
	// A spurious entry permanently lets an unwired symbol pass.
	var allowlist []string
	for _, entry := range docwiring.Allowlist() {
		allowlist = append(allowlist, entry.Path+"\x00"+entry.Name)
	}
	var scriptAllowlist []string
	for _, entry := range script.Allowlist {
		if len(entry) != 2 {
			t.Fatalf("an allowlist entry is not a path and a symbol: %v", entry)
		}
		scriptAllowlist = append(scriptAllowlist, entry[0]+"\x00"+entry[1])
	}
	slices.Sort(scriptAllowlist)
	slices.Sort(allowlist)
	if !slices.Equal(scriptAllowlist, allowlist) {
		t.Errorf("the wiring allowlists differ:\nscript has %d entries: %v\ncommand has %d entries: %v",
			len(scriptAllowlist), scriptAllowlist, len(allowlist), allowlist)
	}

	// The advisory table is ORDERED, and its first match wins.
	// A set comparison would miss a reordering that changes the suite named for a nested area.
	var suites []string
	for _, suite := range docwiring.FunctionalSuites() {
		suites = append(suites, suite.Prefix+"\x00"+suite.Suite)
	}
	var scriptSuites []string
	for _, entry := range script.FunctionalSuites {
		scriptSuites = append(scriptSuites, entry[0]+"\x00"+entry[1])
	}
	if !slices.Equal(scriptSuites, suites) {
		t.Errorf("the advisory tables differ:\nscript:  %v\ncommand: %v", scriptSuites, suites)
	}

	if !slices.Equal(script.MakeTargets, docwiringMakeTargets()) {
		t.Errorf("the delegated target sets differ:\nscript:  %v\ncommand: %v",
			script.MakeTargets, docwiringMakeTargets())
	}
	if len(script.Generators) != 1 || script.Generators[0] != "scripts/dev/package_map.py" {
		t.Errorf("the script's generator list moved: %v", script.Generators)
	}
}

// docwiringMakeTargets answers every delegated target that the command can run,
// in sorted order. It derives the targets from the ordered table.
func docwiringMakeTargets() []string {
	var out []string
	for _, target := range docwiring.TargetOrder() {
		if target != "wiring" {
			out = append(out, target)
		}
	}
	slices.Sort(out)
	return out
}

// TestDocWiringBothHalvesSelectTheSameTargetsForEveryPredicate drives the
// selection with one representative path per rule.
//
// A dry run over the working tree compares only the predicates that this session triggered.
// Thus, a rule untouched by the diff has no comparison.
// One path for each rule keeps the test independent of the dirty files.
// This method caught a missing digest rule that passed the working-tree comparison.
func TestDocWiringBothHalvesSelectTheSameTargetsForEveryPredicate(t *testing.T) {
	root := devPyRoot(t)

	// Each path targets ONE rule and is compared ALONE.
	// A batch CAN hide a rule that stopped firing when another path selects the same target.
	// A Go file under a digest base also selects the digest gate.
	// Thus, the batched comparison passed when the digest rule was absent.
	rules := []struct {
		path, target string
	}{
		{"internal/component/bgp/reactor/peer.go", "wiring"},
		{"internal/component/config/yang/command.go", "ze-command-contract-check"},
		{"cmd/ze/main.go", "ze-command-ownership-check"},
		{"docs/bgp-fsm.md", "ze-doc-verify"},
		{"ai/PACKAGE-MAP.md", "ze-discovery-index-check"},
		{"ai/digests/README.md", "ze-digest-check"},
		{"internal/component/plugin/all/all.go", "ze-inventory-json"},
		{"internal/component/web/nothing.templ", "ze-templ-output-check"},
		{"test/health/docker-exec-baseline.json", "ze-functional-docker-exec-check"},
		{"plan/.citation-baseline", "ze-spec-citation-check"},
	}

	for _, rule := range rules {
		script := devPyRunScript(t, wiringDocsScript,
			[]string{"--dry-run", "--changed-file", rule.path}, root)
		command := devPyRunCommand(t, wiringDocsCommand, docwiring.Answer,
			[]string{"dry-run", "changed-file", rule.path})

		devPyAgree(t, rule.path, script, command, script.Stdout, command.Stdout)

		// A comparison of two empty selections proves nothing, so each path
		// must actually reach the rule it was chosen for.
		if !slices.Contains(strings.Fields(command.Stdout), rule.target) {
			t.Errorf("%s no longer selects %s, so that rule is compared against silence:\n%s",
				rule.path, rule.target, command.Stdout)
		}
	}

	// No path selects three targets on its own.
	// Each target rides with a selected target, and this list keeps the set complete.
	rideAlong := map[string]bool{
		"ze-doc-index-check":      true,
		"ze-command-list-json":    true,
		"ze-plugin-imports-check": true,
	}
	for _, target := range docwiring.TargetOrder() {
		if rideAlong[target] {
			continue
		}
		chosen := false
		for _, rule := range rules {
			if rule.target == target {
				chosen = true
			}
		}
		if !chosen {
			t.Errorf("no path in this list is chosen for %s", target)
		}
	}
}
