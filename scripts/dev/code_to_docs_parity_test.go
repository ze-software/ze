// The migration's proof for the source-anchor REVERSE index: scripts/dev/
// code_to_docs.py and `le docs-to-code ze-doc-index-*` write the same BYTES and
// reach the same verdict.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for `code_to_docs.py`. Both halves
// write a byte-identical ai/CODE-TO-DOCS.md from the committed tree. Over the
// fixture trees, they agree about a stale anchor and an unproven symbol claim.
// They also agree about the two counts.
// PREVENTS: a generator that agrees with its script on the WORD "valid" but
// emits different bytes. Readers search the index when they edit code. Thus, a
// port that writes different groups is wrong even when the verdicts match.
//
// The byte comparison uses a `git archive HEAD` export instead of the working
// tree because several sessions edit this checkout at once. A file written
// between the two runs would otherwise make the halves disagree about a tree
// that neither half got wrong.
//
// This file is deliberately HERE instead of beside letools/docstocode. It is a
// migration artifact, so the commit that deletes the script also deletes its
// proof.

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

	"github.com/ze-software/ze/letools/docstocode"
)

const (
	codeToDocsScript  = "code_to_docs.py"
	codeToDocsCommand = "docs-to-code"
	codeCheckVerb     = "ze-doc-index-check"
	codeUpdateVerb    = "ze-doc-index-update"
)

// codeToDocsRun runs one half over one tree and answers what it printed.
//
// The script gets its tree from its own location by resolving the parents of
// its file. Thus, a fixture run copies the script into the fixture. The command
// gets the tree from ZE_REPO_ROOT.
func codeToDocsScriptRun(t *testing.T, tree string, args []string) devPyResult {
	t.Helper()

	source := filepath.Join(devPyRoot(t), "scripts", "dev", codeToDocsScript)
	body, err := os.ReadFile(source) //nolint:gosec // a tracked script of this checkout
	if err != nil {
		t.Fatalf("reading the script: %v", err)
	}
	dest := filepath.Join(tree, "scripts", "dev")
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatalf("fixture script directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, codeToDocsScript), body, 0o700); err != nil { //nolint:gosec // a fixture copy of a tracked script
		t.Fatalf("fixture script: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	argv := append([]string{filepath.Join(dest, codeToDocsScript)}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a fixture copy of a tracked script
	cmd.Dir = tree
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()

	// An ExitError is the tool's own verdict and is what this file compares.
	// Anything else means the run never happened.
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		t.Fatalf("running %s: %v: %s", codeToDocsScript, runErr, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// TestCodeToDocsBothHalvesWriteTheSameBytesOverTheCommittedTree is the byte
// comparison, over the population both halves must agree about.
func TestCodeToDocsBothHalvesWriteTheSameBytesOverTheCommittedTree(t *testing.T) {
	scriptTree := discoveryExport(t)
	commandTree := discoveryExport(t)

	script := codeToDocsScriptRun(t, scriptTree, nil)
	if script.Code != 0 {
		t.Fatalf("the script exited %d: %s%s", script.Code, script.Stdout, script.Stderr)
	}

	devPyPointAt(t, commandTree)
	command := devPyRunCommand(t, codeToDocsCommand, docstocode.Answer, []string{codeUpdateVerb})
	if command.Code != 0 {
		t.Fatalf("the command exited %d: %s%s", command.Code, command.Stdout, command.Stderr)
	}

	fromScript := devPyRead(t, filepath.Join(scriptTree, filepath.FromSlash(docstocode.CodeOutputRel)))
	fromCommand := devPyRead(t, filepath.Join(commandTree, filepath.FromSlash(docstocode.CodeOutputRel)))
	if fromScript != fromCommand {
		t.Errorf("the two halves wrote different reverse indexes:\n%s", firstDifference(fromScript, fromCommand))
	}
	// A comparison of two empty files proves nothing, and an export that failed
	// to unpack would produce exactly that.
	if strings.Count(fromCommand, "\n## `") < 100 {
		t.Errorf("the index describes almost nothing, so the comparison is vacuous:\n%s", head(fromCommand))
	}
	// The verdicts are also compared. A matching index with different totals
	// means that the port agrees about the file but misstates what it read. The
	// script uses an ABSOLUTE path, and the command uses a relative path. Thus,
	// the comparison uses the counts that each path carries.
	if countsOf(script.Stdout) != countsOf(command.Stdout) {
		t.Errorf("the two halves count differently:\nscript:\n%s\ncommand:\n%s", script.Stdout, command.Stdout)
	}
	if !strings.Contains(command.Stdout, " code paths, ") {
		t.Errorf("the command's verdict carries no counts:\n%s", command.Stdout)
	}
}

// TestCodeToDocsBothHalvesJudgeTheCommittedTreeTheSame is the check half over
// the same population: the same exit code and the same findings.
func TestCodeToDocsBothHalvesJudgeTheCommittedTreeTheSame(t *testing.T) {
	tree := discoveryExport(t)

	script := codeToDocsScriptRun(t, tree, []string{"--check"})

	devPyPointAt(t, tree)
	command := devPyRunCommand(t, codeToDocsCommand, docstocode.Answer, []string{codeCheckVerb})

	devPyAgree(t, "the committed tree", script, command, script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "code paths") {
		t.Errorf("the check said nothing about what it read:\n%s", command.Stdout)
	}
}

// TestCodeToDocsBothHalvesReportTheSameStaleAnchor drives the finding, because
// the committed tree has none and a comparison of two clean verdicts proves
// nothing.
func TestCodeToDocsBothHalvesReportTheSameStaleAnchor(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		return devPyTree(t, map[string]string{
			"go.mod":            "module example.com/m\n\ngo 1.21\n",
			"feature-gates.txt": "ze_x pkg/x\n",
			"internal/a/a.go":   "package a\n\nfunc Run() {}\n",
			"docs/one.md":       "<!-- source: internal/a/a.go -- Run -->\n",
			"docs/two.md":       "<!-- source: internal/gone/gone.go -- Missing -->\n",
			"docs/three.md":     "<!-- source: internal/gone/gone.go -- Other -->\n",
		})
	}

	scriptTree, commandTree := build(t), build(t)
	script := codeToDocsScriptRun(t, scriptTree, []string{"--check"})

	devPyPointAt(t, commandTree)
	command := devPyRunCommand(t, codeToDocsCommand, docstocode.Answer, []string{codeCheckVerb})

	devPyAgree(t, "a stale anchor", script, command, script.Stdout, command.Stdout)
	if script.Code != 1 {
		t.Errorf("a stale anchor exited %d, want 1", script.Code)
	}
	if !strings.Contains(command.Stdout, "MISSING: internal/gone/gone.go") {
		t.Errorf("the stale path went unnamed:\n%s", command.Stdout)
	}
	if !strings.Contains(command.Stdout, "docs/three.md:1") {
		t.Errorf("the documents naming the stale path went unnamed:\n%s", command.Stdout)
	}
}

// TestCodeToDocsBothHalvesReportTheSameUnprovenClaim drives the second finding.
// It also drives the two severity rules. A claim that the file MENTIONS is
// demoted, and a single lowercase word is prose.
func TestCodeToDocsBothHalvesReportTheSameUnprovenClaim(t *testing.T) {
	const source = "package a\n\nfunc Run() { helper() }\n\ntype Peer struct {\n\tName string\n}\n"
	build := func(t *testing.T) string {
		t.Helper()
		return devPyTree(t, map[string]string{
			"go.mod":            "module example.com/m\n\ngo 1.21\n",
			"feature-gates.txt": "ze_x pkg/x\n",
			"internal/a/a.go":   source,
			// Declared, mentioned-but-not-declared, prose, and absent.
			"docs/one.md":   "<!-- source: internal/a/a.go -- Run, Peer.Name -->\n",
			"docs/two.md":   "<!-- source: internal/a/a.go -- helper -->\n",
			"docs/three.md": "<!-- source: internal/a/a.go -- routing -->\n",
			"docs/four.md":  "<!-- source: internal/a/a.go -- Nowhere -->\n",
		})
	}

	scriptTree, commandTree := build(t), build(t)
	script := codeToDocsScriptRun(t, scriptTree, []string{"--check"})

	devPyPointAt(t, commandTree)
	command := devPyRunCommand(t, codeToDocsCommand, docstocode.Answer, []string{codeCheckVerb})

	devPyAgree(t, "an unproven claim", script, command, script.Stdout, command.Stdout)
	if script.Code != 1 {
		t.Errorf("an unproven claim exited %d, want 1", script.Code)
	}
	if !strings.Contains(command.Stdout, "'Nowhere'") {
		t.Errorf("the claim nothing declares went unreported:\n%s", command.Stdout)
	}
	for _, demoted := range []string{"'helper'", "'routing'", "'Run'", "'Peer.Name'"} {
		if strings.Contains(command.Stdout, demoted) {
			t.Errorf("%s was reported, and the severity rules demote it:\n%s", demoted, command.Stdout)
		}
	}
	// Exactly one finding: a comparison that passed at four would be agreeing
	// about a check that reports everything.
	if got := strings.Count(command.Stdout, "  CLAIM: "); got != 1 {
		t.Errorf("the check reported %d claims, want exactly one", got)
	}
}

// TestCodeToDocsBothHalvesReadTheSameAnchorGrammar pins the parsing the index
// rests on: semicolon segments, comma-separated relative names, a line suffix,
// and a token under no known root.
func TestCodeToDocsBothHalvesReadTheSameAnchorGrammar(t *testing.T) {
	const anchor = "<!-- source: internal/a/a.go, b.go -- Run; cmd/x/main.go:12 -- Main; nowhere.go -- Ghost -->\n"
	build := func(t *testing.T) string {
		t.Helper()
		return devPyTree(t, map[string]string{
			"go.mod":            "module example.com/m\n\ngo 1.21\n",
			"feature-gates.txt": "ze_x pkg/x\n",
			"internal/a/a.go":   "package a\n\nfunc Run() {}\n",
			"internal/a/b.go":   "package a\n",
			"cmd/x/main.go":     "package main\n\nfunc Main() {}\n",
			// The index is written into ai/, but neither half creates that
			// directory. A tree with no ai/ gives the generator no destination.
			// Both halves report that problem instead of creating a directory.
			"ai/keep.md":  "the directory the index is written into\n",
			"docs/one.md": anchor,
		})
	}

	scriptTree, commandTree := build(t), build(t)
	script := codeToDocsScriptRun(t, scriptTree, nil)

	devPyPointAt(t, commandTree)
	command := devPyRunCommand(t, codeToDocsCommand, docstocode.Answer, []string{codeUpdateVerb})

	fromScript := devPyRead(t, filepath.Join(scriptTree, filepath.FromSlash(docstocode.CodeOutputRel)))
	fromCommand := devPyRead(t, filepath.Join(commandTree, filepath.FromSlash(docstocode.CodeOutputRel)))
	if fromScript != fromCommand {
		t.Errorf("the two halves read the anchor differently:\n%s", firstDifference(fromScript, fromCommand))
	}
	for _, path := range []string{"internal/a/a.go", "internal/a/b.go", "cmd/x/main.go:12"} {
		if !strings.Contains(fromCommand, path) {
			t.Errorf("%s was dropped by the anchor grammar:\n%s", path, fromCommand)
		}
	}
	if strings.Contains(fromCommand, "nowhere.go") {
		t.Errorf("a token under no known root was indexed:\n%s", fromCommand)
	}
	if script.Code != 0 || command.Code != 0 {
		t.Errorf("generating the index exited %d and %d, want 0 and 0", script.Code, command.Code)
	}
}

// TestCodeToDocsBothHalvesIgnoreWhatGitIgnores is why the walk asks git at all.
//
// docs/ contains gitignored research output only on machines with the local
// research files. If the generator indexes this output, its result is not
// reproducible: 1439 code paths on that host and 1438 on a clean checkout. This
// checkout has no such output now. Thus, a port that stops asking git agrees
// with the script on the committed tree but not on a developer's machine. The
// fixture restores that case.
func TestCodeToDocsBothHalvesIgnoreWhatGitIgnores(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		return devPyTree(t, map[string]string{
			"go.mod":                 "module example.com/m\n\ngo 1.21\n",
			"feature-gates.txt":      "ze_x pkg/x\n",
			".gitignore":             "docs/research/\n",
			"ai/keep.md":             "the directory the index is written into\n",
			"internal/a/a.go":        "package a\n",
			"internal/b/b.go":        "package b\n",
			"docs/kept.md":           "<!-- source: internal/a/a.go -- indexed -->\n",
			"docs/research/local.md": "<!-- source: internal/b/b.go -- never indexed -->\n",
		})
	}

	scriptTree, commandTree := build(t), build(t)
	script := codeToDocsScriptRun(t, scriptTree, nil)

	devPyPointAt(t, commandTree)
	command := devPyRunCommand(t, codeToDocsCommand, docstocode.Answer, []string{codeUpdateVerb})

	fromScript := devPyRead(t, filepath.Join(scriptTree, filepath.FromSlash(docstocode.CodeOutputRel)))
	fromCommand := devPyRead(t, filepath.Join(commandTree, filepath.FromSlash(docstocode.CodeOutputRel)))
	if fromScript != fromCommand {
		t.Errorf("the two halves disagree about the ignored document:\n%s", firstDifference(fromScript, fromCommand))
	}
	if !strings.Contains(fromCommand, "internal/a/a.go") {
		t.Errorf("the tracked document was not indexed:\n%s", fromCommand)
	}
	if strings.Contains(fromCommand, "internal/b/b.go") {
		t.Errorf("a gitignored document was indexed, so the index is not reproducible:\n%s", fromCommand)
	}
	if script.Code != 0 || command.Code != 0 {
		t.Errorf("generating the index exited %d and %d, want 0 and 0", script.Code, command.Code)
	}
}

// countsOf answers the parenthesized counts in a generate-mode verdict. The
// script includes an absolute path, and the command includes a relative path.
// This function drops the path and keeps the numbers.
func countsOf(verdict string) string {
	open := strings.Index(verdict, "(")
	if open < 0 {
		return verdict
	}
	return verdict[open:]
}

// head answers the first lines of a rendering, for a failure message.
func head(body string) string {
	lines := strings.SplitN(body, "\n", 20)
	return strings.Join(lines, "\n")
}
