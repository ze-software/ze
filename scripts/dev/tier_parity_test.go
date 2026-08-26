// This file proves the migration of the three PLACEMENT-and-ANCHOR gate tools.
// `dep_audit.py`, `validate.py`, and `digest_check.py` match their commands on
// the result, stream, and exit code.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for the three scripts and their five
// gates: `ze-tier-check`, `ze-tier-selftest`, `ze-repository-check`,
// `ze-repository-tree-check`, and `ze-digest-check`. Over the committed and
// fixture trees, each script matches its command. They print the same page to
// the same stream and answer the same exit code.
// PREVENTS: a comparison that passes because both halves found nothing. Each
// case uses either the large committed tree or a fixture that makes the gate
// RED. Every case also asserts how much it compared.
//
// The tree-wide comparisons use a `git archive HEAD` export instead of the
// working tree because several sessions edit this checkout at once. A file
// written between the two runs would otherwise make the halves disagree about a
// tree that neither half got wrong.
//
// This file is deliberately HERE instead of beside the new packages. It is a
// migration artifact, so the commit that deletes the scripts also deletes their
// proof. Its name identifies the cluster because several other steps port code
// into this package. Its helpers use the `tierPy` prefix for the same reason.
//
// One difference is DELIBERATE and is asserted rather than compared: the COLOR
// from the repository gate. The script writes raw ANSI, but a compiled Ze
// package can omit ANSI. Thus, the command writes the semantic palette from
// docs/architecture/cli/color-system.md. The test strips color before it
// compares the pages. letools/repository test
// TestTheTwoSeveritiesAreColoredApart checks the command shades. Step 2 made the
// same choice in scripts/lint/parity_test.go.
//
// It also pins the four fail-open defects that the ports FIXED but the scripts
// still have. These cases assert that the SCRIPT still fails open. When
// somebody repairs a script, its case fails and must be deleted with that
// script.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/digest"
	"github.com/ze-software/ze/letools/repository"
	"github.com/ze-software/ze/letools/tier"
)

const (
	tierScript       = "dep_audit.py"
	repositoryScript = "validate.py"
	digestScript     = "digest_check.py"
)

// tierPyRunCopied runs a COPY of one script that the fixture tree holds.
//
// `digest_check.py` derives its checkout from its own __file__, so running this
// checkout's copy judges this checkout whatever the working directory says. A
// fixture comparison therefore needs the script inside the fixture.
func tierPyRunCopied(t *testing.T, tree, script string, args []string) devPyResult {
	t.Helper()

	source := filepath.Join(devPyRoot(t), "scripts", "dev", script)
	raw, err := os.ReadFile(source) //nolint:gosec // a tracked script of this checkout
	if err != nil {
		t.Fatalf("reading %s: %v", script, err)
	}
	dest := filepath.Join(tree, "scripts", "dev", script)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		t.Fatalf("fixture scripts directory: %v", err)
	}
	if err := os.WriteFile(dest, raw, 0o600); err != nil {
		t.Fatalf("copying %s into the fixture: %v", script, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()
	argv := append([]string{dest}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a copy of a tracked script and a test's own arguments
	cmd.Dir = tree
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	// An ExitError is the tool's own verdict and is what this file compares.
	// Anything else means the run never happened.
	var exit *exec.ExitError
	if runErr := cmd.Run(); runErr != nil && !errors.As(runErr, &exit) {
		t.Fatalf("running the fixture's %s: %v: %s", script, runErr, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// tierPyGitTree exports the committed tree and makes a separate repository.
// `dep_audit.py` needs this because it asks git for the checkout root instead of
// getting a root from a flag.
func tierPyGitTree(t *testing.T) string {
	t.Helper()

	tree := discoveryExport(t)
	cmd := exec.CommandContext(t.Context(), "git", "-C", tree, "init", "--quiet")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init in the export: %v: %s", err, out)
	}
	return tree
}

// ─── ze-digest-check ────────────────────────────────────────────────────────

func TestDigestBothHalvesAgreeOverTheCheckout(t *testing.T) {
	script := devPyRunScript(t, digestScript, nil, devPyRoot(t))
	command := devPyRunCommand(t, "digest", digest.Answer, nil)

	devPyAgree(t, "digest over the checkout", script, command, script.Stdout, command.Stdout)
	if script.Stderr != command.Stderr {
		t.Errorf("the two halves wrote different stderr\nscript:\n%s\ncommand:\n%s", script.Stderr, command.Stderr)
	}

	// A comparison of two "checked 0 anchors" lines proves nothing.
	if !strings.Contains(command.Stdout, " anchors across ") {
		t.Fatalf("the command did not report a count: %q", command.Stdout)
	}
	report, err := digest.Check(devPyRoot(t))
	if err != nil {
		t.Fatalf("checking the checkout: %v", err)
	}
	if len(report.Resolved) < 500 || report.Digests < 10 {
		t.Fatalf("the comparison covered %d anchors across %d digests, which is too few to mean anything",
			len(report.Resolved), report.Digests)
	}
}

func TestDigestBothHalvesAgreeOverAFixtureFullOfBadAnchors(t *testing.T) {
	files := map[string]string{
		"ai/digests/one.md": strings.Join([]string{
			"<!-- digest-base: internal/a internal/b -->",
			"`x.go:9` `x.go:1-99` `x.go:3-2` `gone.go:1` `peer.go:1` `internal/nowhere.go`",
			"`x.go:0` `internal/a/x.go:2`",
			"",
		}, "\n"),
		"ai/digests/two.md":    "`y.go:1`\n",
		"ai/digests/three.md":  "<!-- digest-base: internal/absent -->\n`z.go:1`\n",
		"ai/digests/README.md": "not a digest\n",
		"internal/a/x.go":      "1\n2\n",
		"internal/a/peer.go":   "1\n",
		"internal/b/peer.go":   "1\n",
	}
	tree := devPyTree(t, files)
	script := tierPyRunCopied(t, tree, digestScript, nil)

	// The test compares the command failure page through the library instead of
	// captured stderr. The action writes it to global os.Stderr, which no
	// injectable writer reaches. letools/digest test
	// TestTheVerdictAndTheFailurePageAreDifferentStreams confirms that the action
	// uses this stream.
	report, err := digest.Check(tree)
	if err != nil {
		t.Fatalf("checking the fixture: %v", err)
	}

	if got := 1; script.Code != got {
		t.Fatalf("the script exited %d and the command answers %d", script.Code, got)
	}
	if script.Stdout != report.Text() {
		t.Errorf("the two halves print different verdicts\nscript:\n%s\ncommand:\n%s", script.Stdout, report.Text())
	}
	if script.Stderr != report.Diagnosis() {
		t.Errorf("the two halves print different failures\nscript:\n%s\ncommand:\n%s", script.Stderr, report.Diagnosis())
	}

	// The comparison is only worth having if the fixture actually broke every
	// branch it was built for.
	page := report.Diagnosis()
	for _, fragment := range []string{
		"line 9 out of range", "reversed line range 3-2", "ambiguous -- matches",
		"file not found under ['internal/a', 'internal/b']", "linked file does not exist",
		"no `<!-- digest-base:", "declared base subtree `internal/absent` does not exist",
		"line 0 out of range",
	} {
		if !strings.Contains(page, fragment) {
			t.Errorf("the fixture never produced %q, so that branch is uncompared:\n%s", fragment, page)
		}
	}
	if strings.Count(page, "\n  ") < 8 {
		t.Errorf("the fixture produced %d findings, and it was built for at least eight:\n%s",
			strings.Count(page, "\n  "), page)
	}
}

func TestScriptDigestStillPassesATreeWithNoDigests(t *testing.T) {
	tree := devPyTree(t, map[string]string{"go.mod": "module example.com/m\n"})
	script := tierPyRunCopied(t, tree, digestScript, nil)

	if script.Code != 0 || !strings.Contains(script.Stdout, "across 0 digests, all resolve") {
		t.Fatalf("the script no longer passes a tree with no digest -- delete this case with the script.\ncode %d: %s",
			script.Code, script.Stdout)
	}

	if _, err := digest.Check(tree); err == nil {
		t.Fatal("the command accepted a tree with no digest")
	}
}

// ─── ze-repository-check and ze-repository-tree-check ───────────────────────

func TestRepositoryBothHalvesAgreeOverTheCommittedTree(t *testing.T) {
	tree := discoveryExport(t)
	script := devPyRunScript(t, repositoryScript, []string{"--root", tree, "--changed-file", ""}, tree)

	devPyPointAt(t, tree)
	command := devPyRunCommand(t, "repository", repository.Answer, []string{"tree-check"})

	devPyAgree(t, "repository tree-check over the committed tree", script, command,
		tierPyPlain(script.Stdout), tierPyPlain(command.Stdout))
	if script.Stderr != command.Stderr {
		t.Errorf("the two halves wrote different stderr\nscript:\n%s\ncommand:\n%s", script.Stderr, command.Stderr)
	}

	// The three tree-wide checks read every document and every spec. A
	// comparison over a tree holding neither would agree about nothing.
	docs, err := filepath.Glob(filepath.Join(tree, "docs", "*", "*.md"))
	if err != nil || len(docs) < 50 {
		t.Fatalf("the export holds %d documents, which is too few for this comparison to mean anything", len(docs))
	}
}

func TestRepositoryBothHalvesAgreeOverAFixtureFullOfFindings(t *testing.T) {
	spec := strings.Join([]string{
		"| Status | in-progress |",
		"### Acceptance Criteria",
		"| AC-1 | done | TestOne | a note |",
		"| AC-2 | done |  | a note |",
		"",
	}, "\n")
	tree := devPyTree(t, map[string]string{
		"go.mod":    "module example.com/m\n",
		"docs/a.md": "<!-- source: internal/a/x.go:9 -- a line number -->\n<!-- source: internal/gone.go -- a stale path -->\n",
		"docs/b/c.md": "<!-- source: https://example.com/x.go -- external -->\n" +
			"<!-- source: ../sibling/x.go -- a sibling checkout -->\n",
		"plan/spec-open.md":         spec,
		"internal/a/x.go":           "package a\n\nfunc Orphan() {}\n\ntype Dead struct{}\n",
		"internal/plugins/x/cmd.go": "package x\n\nfunc init() { MustRegisterRootHandler(\"show untested\", nil) }\n",
		"test/ui/a.ci":              "nothing here\n",
	})
	changed := []string{"internal/a/x.go", "internal/plugins/x/cmd.go"}

	args := []string{"--root", tree}
	for _, file := range changed {
		args = append(args, "--changed-file", file)
	}
	script := devPyRunScript(t, repositoryScript, args, tree)

	report, err := repository.Run(t.Context(), tree, changed)
	if err != nil {
		t.Fatalf("running the checks: %v", err)
	}

	if script.Code != report.Code() {
		t.Fatalf("the script exited %d and the command answers %d\nscript:\n%s", script.Code, report.Code(), script.Stdout)
	}
	if tierPyPlain(script.Stdout) != tierPyPlain(report.Text()) {
		t.Errorf("the two halves print different pages\nscript:\n%s\ncommand:\n%s", script.Stdout, report.Text())
	}

	// Every one of the five checks must have contributed, or the comparison is
	// about whichever ones happened to fire.
	page := tierPyPlain(report.Text())
	for _, fragment := range []string{
		"contains line number", "points to non-existent file: internal/gone.go",
		"exported symbol Orphan has no cross-package non-test caller",
		"exported symbol Dead has no cross-package non-test caller",
		"AC-2 has empty 'Demonstrated By' column",
		"CLI command 'show untested' has no .ci test mentioning it",
	} {
		if !strings.Contains(page, fragment) {
			t.Errorf("the fixture never produced %q, so that check is uncompared:\n%s", fragment, page)
		}
	}
	if report.Issues != 6 {
		t.Errorf("the fixture produced %d issues, and it was built for six:\n%s", report.Issues, page)
	}
}

func TestScriptRepositoryStillPassesOverAnUnreadableDocument(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so this case cannot be staged")
	}
	tree := devPyTree(t, map[string]string{
		"go.mod":    "module example.com/m\n",
		"docs/a.md": "<!-- source: internal/gone.go -- a stale path -->\n",
	})
	path := filepath.Join(tree, "docs", "a.md")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skipf("cannot make a file unreadable here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	script := devPyRunScript(t, repositoryScript, []string{"--root", tree, "--changed-file", ""}, tree)
	if script.Code != 0 || !strings.Contains(tierPyPlain(script.Stdout), "all checks passed") {
		t.Fatalf("the script no longer passes over a document it cannot read -- delete this case with the script.\ncode %d: %s",
			script.Code, script.Stdout)
	}

	if _, err := repository.Run(t.Context(), tree, nil); err == nil {
		t.Fatal("the command passed over a document it could not read")
	}
}

func TestScriptRepositoryStillReadsAFailedGitCommandAsAnEmptyChangedSet(t *testing.T) {
	// A directory that is NOT a git checkout: both git commands fail, and the
	// script's changed set comes back empty rather than absent.
	tree := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":          "module example.com/m\n",
		"internal/a/x.go": "package a\n\nfunc Orphan() {}\n",
	} {
		path := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}

	script := devPyRunScript(t, repositoryScript, []string{"--root", tree}, tree)
	if script.Code != 0 || !strings.Contains(tierPyPlain(script.Stdout), "all checks passed") {
		t.Fatalf("the script no longer passes when git cannot answer -- delete this case with the script.\ncode %d: %s",
			script.Code, script.Stdout)
	}

	if _, err := repository.ChangedFiles(t.Context(), tree); err == nil {
		t.Fatal("the command read a failed git command as an empty changed set")
	}
}

// ─── ze-tier-check and ze-tier-selftest ─────────────────────────────────────

func TestTierBothHalvesAgreeOverTheCommittedTree(t *testing.T) {
	tree := tierPyGitTree(t)
	script := devPyRunScript(t, tierScript, []string{"--check"}, tree)

	devPyPointAt(t, tree)
	command := devPyRunCommand(t, "tier", tier.Answer, []string{"check"})

	devPyAgree(t, "tier check over the committed tree", script, command, script.Stdout, command.Stdout)
	if script.Stderr != command.Stderr {
		t.Errorf("the two halves wrote different stderr\nscript:\n%s\ncommand:\n%s", script.Stderr, command.Stderr)
	}

	// The five checks read every import edge in the repository. A comparison
	// over a tree with no subsystems would agree about nothing.
	if !strings.Contains(command.Stdout, "OK: non-engine placement categories clean") {
		t.Fatalf("the manifest check did not run:\n%s%s", command.Stdout, command.Stderr)
	}
	if !strings.Contains(command.Stdout, "OK: core import direction clean") {
		t.Fatalf("the core direction check did not run:\n%s%s", command.Stdout, command.Stderr)
	}
}

func TestTierBothHalvesPrintTheSameAuditOverTheCommittedTree(t *testing.T) {
	tree := tierPyGitTree(t)
	script := devPyRunScript(t, tierScript, nil, tree)

	devPyPointAt(t, tree)
	command := devPyRunCommand(t, "tier", tier.Answer, []string{"report"})

	devPyAgree(t, "tier report over the committed tree", script, command, script.Stdout, command.Stdout)

	// The audit is the richest comparison in this file: one block per registry,
	// every subsystem classified, every shared library's importers listed.
	if strings.Count(command.Stdout, "\nAREA: ") != 3 {
		t.Fatalf("the audit covered %d areas, want the three registries:\n%s",
			strings.Count(command.Stdout, "\nAREA: "), command.Stdout)
	}
	if strings.Count(command.Stdout, "\n  ") < 100 {
		t.Fatalf("the audit lists %d subsystems, which is too few to mean anything",
			strings.Count(command.Stdout, "\n  "))
	}
}

func TestTierBothHalvesRunTheSameSelftest(t *testing.T) {
	script := devPyRunScript(t, tierScript, []string{"--selftest"}, devPyRoot(t))
	command := devPyRunCommand(t, "tier", tier.Answer, []string{"selftest"})

	devPyAgree(t, "tier selftest", script, command, script.Stdout, command.Stdout)
	if script.Stderr != "" {
		t.Errorf("the script's selftest wrote to stderr: %s", script.Stderr)
	}

	// Equal pages show that both printed "dep_audit selftest OK". The number of
	// properties behind that page makes the result meaningful. The port has a
	// separate fixture set because the script's plugin-roots fixture cannot
	// survive replacing the parser with a function call.
	report, err := tier.Selftest()
	if err != nil {
		t.Fatalf("running the selftest: %v", err)
	}
	if len(report.Results) < 20 {
		t.Fatalf("the port's selftest carries %d cases, and the script's fixtures assert more", len(report.Results))
	}
}

func TestTierBothHalvesAgreeOverAFixtureWithAMisplacedEngine(t *testing.T) {
	files := tierFixtureFiles(t)
	tree := devPyTree(t, files)
	script := devPyRunScript(t, tierScript, []string{"--check"}, tree)

	// As above: the failure page is compared through the library, because the
	// action writes it to os.Stderr and letools/tier's own
	// TestTheVerdictAndTheFailureAreDifferentStreams pins that it does.
	report, err := tier.Check(tree)
	if err != nil {
		t.Fatalf("checking the fixture: %v", err)
	}

	if script.Code != report.Failed {
		t.Fatalf("the script exited %d and the command answers %d\nscript:\n%s%s",
			script.Code, report.Failed, script.Stdout, script.Stderr)
	}
	if script.Stdout != report.Text() {
		t.Errorf("the two halves print different verdicts\nscript:\n%s\ncommand:\n%s", script.Stdout, report.Text())
	}
	if script.Stderr != report.Diagnosis() {
		t.Errorf("the two halves print different failures\nscript:\n%s\ncommand:\n%s", script.Stderr, report.Diagnosis())
	}
	if report.Failed != 2 {
		t.Fatalf("the fixture was built to fail and the gate answered %d:\n%s%s",
			report.Failed, report.Text(), report.Diagnosis())
	}
	if !strings.Contains(report.Diagnosis(), "FAIL: new misplaced engine(s)") {
		t.Errorf("the engine-placement failure is missing:\n%s", report.Diagnosis())
	}
}

func TestScriptTierStillPassesOverAnUnreadableGoFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so this case cannot be staged")
	}
	files := tierFixtureFiles(t)
	files["internal/core/bad/uses.go"] = "package bad\nimport _ \"example.com/m/internal/component/thing\"\n"
	tree := devPyTree(t, files)

	path := filepath.Join(tree, "internal", "core", "bad", "uses.go")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skipf("cannot make a file unreadable here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	// The script treats an unreadable file as having no import edge. Thus, its
	// upward import never reaches the direction gate. The gate reports only the
	// existing engine-placement problem.
	script := devPyRunScript(t, tierScript, []string{"--check"}, tree)
	if strings.Contains(script.Stderr, "new upward import(s)") {
		t.Fatalf("the script no longer passes over a Go file it cannot read -- delete this case with the script.\n%s",
			script.Stderr)
	}

	module, err := tier.ModulePath(tree)
	if err != nil {
		t.Fatalf("reading the module path: %v", err)
	}
	if _, err := tier.CollectEdges(tree, module); err == nil {
		t.Fatal("the command passed over a Go file it could not read")
	}
}

// tierFixtureFiles answers a fixture checkout that fails the engine-placement
// check. Its generator declares the plugin roots that the Go half gets by
// calling letools/pluginimports.
//
// This generator lets the test compare the same populations. The script parses
// its source text for the search roots, and the command calls the compiled list.
// Any other fixture declaration would point the two gates at different
// populations.
func tierFixtureFiles(t *testing.T) map[string]string {
	t.Helper()

	var roots strings.Builder
	roots.WriteString("package main\n\nvar pluginDirs = []string{\n")
	for _, root := range tier.PluginDirs() {
		roots.WriteString("\t\"" + root + "\",\n")
	}
	roots.WriteString("}\n\nvar nestedPluginDomains = []string{}\n")

	return map[string]string{
		"go.mod":                            "module example.com/m\n",
		"scripts/codegen/plugin_imports.go": roots.String(),
		// A misplaced engine: in component, and nothing depends on it.
		"internal/component/edgeproto/r.go": "package edgeproto\nfunc R(){ sdk.NewWithConn() }\n",
		// A correctly placed one, so the check is not reporting everything.
		"internal/plugins/edge1/r.go": "package edge1\nfunc R(){ sdk.NewWithConn() }\n",
		// The two manifests the other checks read.
		"feature-gates.txt": "# no gates in this fixture\n",
		".golangci.yml":     "run:\n  build-tags:\n    - ze_core\nlinters: {}\n",
		"scripts/dev/tier_non_engine_categories.txt": "# path category rationale\n",
	}
}

// ─── the shared constants ───────────────────────────────────────────────────

// tierPyConstants is the program that reads the three modules' own tables and
// bounds and prints them as one document.
//
// An output comparison cannot observe a shared constant. A table with a missing
// entry changes the verdict only for a new input. Thus, the test compares each
// constant BY VALUE with the Go value.
const tierPyConstants = `
import importlib, json, sys
sys.path.insert(0, sys.argv[1])
d = importlib.import_module("digest_check")
v = importlib.import_module("validate")
a = importlib.import_module("dep_audit")
print(json.dumps({
  "digest": {
    "digest_dir": list(d.DIGEST_DIR),
    "skip_files": sorted(d.SKIP_FILES),
    "skip_walk": sorted(d.SKIP_WALK),
    "top_dirs": sorted(d.TOP_DIRS),
    "base_re": d.BASE_RE.pattern,
    "backtick_re": d.BACKTICK_RE.pattern,
    "anchor_re": d.ANCHOR_RE.pattern,
  },
  "repository": {
    "source_anchor_re": v.SOURCE_ANCHOR_RE.pattern,
    "source_anchor_line_re": v.SOURCE_ANCHOR_LINE_RE.pattern,
    "ac_row_re": v.AC_ROW_RE.pattern,
    "spec_status_re": v.SPEC_STATUS_RE.pattern,
    "register_re": v.REGISTER_RE.pattern,
    "exported_func_re": v.EXPORTED_FUNC_RE.pattern,
    "exported_type_re": v.EXPORTED_TYPE_RE.pattern,
    "func_recv_re": v.FUNC_RECV_RE.pattern,
    "exported_iface_re": v.EXPORTED_IFACE_RE.pattern,
    "exported_iface_named_re": v.EXPORTED_IFACE_NAMED_RE.pattern,
    "iface_method_re": v.IFACE_METHOD_RE.pattern,
    "registered_server_re": v.REGISTERED_SERVER_RE.pattern,
    "const_spec_re": v.CONST_SPEC_RE.pattern,
    "cli_paths": list(v.CLI_PATHS),
    "interface_dispatch": {k[0] + " " + k[1]: sorted(m) for k, m in v.INTERFACE_DISPATCH_METHODS.items()},
  },
  "tier": {
    "default_areas": list(a.DEFAULT_AREAS),
    "legal_categories": sorted(a.LEGAL_NON_ENGINE_CATEGORIES),
    "domain_library_prefixes": list(a.DOMAIN_LIBRARY_PREFIXES),
    "non_feature_prefixes": list(a.NON_FEATURE_PREFIXES),
    "disableable_nonprod_prefixes": list(a.DISABLEABLE_NONPROD_PREFIXES),
    "golangci_base_tags": sorted(a.GOLANGCI_BASE_TAGS),
    "baseline": a.BASELINE,
    "non_engine_categories": a.NON_ENGINE_CATEGORIES,
    "core_import_baseline": a.CORE_IMPORT_BASELINE,
    "feature_gates_manifest": a.FEATURE_GATES_MANIFEST,
    "golangci": a.GOLANGCI,
    "core_area_prefix": a.CORE_AREA_PREFIX,
    "core_forbidden": list(a.CORE_FORBIDDEN),
    "core_fix_routes": list(a.CORE_FIX_ROUTES),
  },
}))
`

func TestTheThreeToolsShareTheirScriptsConstants(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-c", tierPyConstants,
		filepath.Join(devPyRoot(t), "scripts", "dev"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reading the modules' constants: %v", err)
	}

	var document struct {
		Digest struct {
			DigestDir  []string `json:"digest_dir"`
			SkipFiles  []string `json:"skip_files"`
			SkipWalk   []string `json:"skip_walk"`
			TopDirs    []string `json:"top_dirs"`
			BaseRe     string   `json:"base_re"`
			BacktickRe string   `json:"backtick_re"`
			AnchorRe   string   `json:"anchor_re"`
		} `json:"digest"`
		Repository struct {
			SourceAnchorRe       string              `json:"source_anchor_re"`
			SourceAnchorLineRe   string              `json:"source_anchor_line_re"`
			ACRowRe              string              `json:"ac_row_re"`
			SpecStatusRe         string              `json:"spec_status_re"`
			RegisterRe           string              `json:"register_re"`
			ExportedFuncRe       string              `json:"exported_func_re"`
			ExportedTypeRe       string              `json:"exported_type_re"`
			FuncRecvRe           string              `json:"func_recv_re"`
			ExportedIfaceRe      string              `json:"exported_iface_re"`
			ExportedIfaceNamedRe string              `json:"exported_iface_named_re"`
			IfaceMethodRe        string              `json:"iface_method_re"`
			RegisteredServerRe   string              `json:"registered_server_re"`
			ConstSpecRe          string              `json:"const_spec_re"`
			CLIPaths             []string            `json:"cli_paths"`
			InterfaceDispatch    map[string][]string `json:"interface_dispatch"`
		} `json:"repository"`
		Tier struct {
			DefaultAreas               []string `json:"default_areas"`
			LegalCategories            []string `json:"legal_categories"`
			DomainLibraryPrefixes      []string `json:"domain_library_prefixes"`
			NonFeaturePrefixes         []string `json:"non_feature_prefixes"`
			DisableableNonProdPrefixes []string `json:"disableable_nonprod_prefixes"`
			GolangciBaseTags           []string `json:"golangci_base_tags"`
			BaselineFile               string   `json:"baseline"`
			NonEngineCategories        string   `json:"non_engine_categories"`
			CoreImportBaseline         string   `json:"core_import_baseline"`
			FeatureGatesManifest       string   `json:"feature_gates_manifest"`
			Golangci                   string   `json:"golangci"`
			CoreAreaPrefix             string   `json:"core_area_prefix"`
			CoreForbidden              []string `json:"core_forbidden"`
			CoreFixRoutes              []string `json:"core_fix_routes"`
		} `json:"tier"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		t.Fatalf("decoding the modules' constants: %v", err)
	}

	texts := []struct{ name, script, command string }{
		{"digest base pattern", document.Digest.BaseRe, digest.BasePattern},
		{"digest backtick pattern", document.Digest.BacktickRe, digest.BacktickPattern},
		{"digest anchor pattern", document.Digest.AnchorRe, digest.AnchorPattern},
		{"source anchor pattern", document.Repository.SourceAnchorRe, repository.SourceAnchorPattern},
		{"source anchor line pattern", document.Repository.SourceAnchorLineRe, repository.SourceAnchorLinePattern},
		{"AC row pattern", document.Repository.ACRowRe, repository.ACRowPattern},
		{"spec status pattern", document.Repository.SpecStatusRe, tierPyTrimMultiline(repository.SpecStatusPattern)},
		{"register pattern", document.Repository.RegisterRe, repository.RegisterPattern},
		{"exported func pattern", document.Repository.ExportedFuncRe, repository.ExportedFuncPattern},
		{"exported type pattern", document.Repository.ExportedTypeRe, repository.ExportedTypePattern},
		{"func receiver pattern", document.Repository.FuncRecvRe, repository.FuncRecvPattern},
		{"exported interface pattern", document.Repository.ExportedIfaceRe, repository.ExportedIfacePattern},
		{"named interface pattern", document.Repository.ExportedIfaceNamedRe, repository.ExportedIfaceNamedPattern},
		{"interface method pattern", document.Repository.IfaceMethodRe, repository.IfaceMethodPattern},
		{"registered server pattern", document.Repository.RegisteredServerRe, repository.RegisteredServerPattern},
		{"const spec pattern", document.Repository.ConstSpecRe, repository.ConstSpecPattern},
		{"tier baseline", document.Tier.BaselineFile, tier.Baseline},
		{"tier non-engine categories", document.Tier.NonEngineCategories, tier.NonEngineCategories},
		{"tier core import baseline", document.Tier.CoreImportBaseline, tier.CoreImportBaseline},
		{"tier feature-gate manifest", document.Tier.FeatureGatesManifest, tier.FeatureGatesManifest},
		{"tier lint configuration", document.Tier.Golangci, tier.Golangci},
		{"tier core area prefix", document.Tier.CoreAreaPrefix, tier.CoreAreaPrefix},
	}
	for _, pair := range texts {
		if pair.script != pair.command {
			t.Errorf("%s: the script says %q and the command says %q", pair.name, pair.script, pair.command)
		}
	}

	lists := []struct {
		name            string
		script, command []string
	}{
		{"digest directory", document.Digest.DigestDir, digest.DigestDir[:]},
		{"digest skip files", document.Digest.SkipFiles, tierPySortedKeys(digest.SkipFiles)},
		{"digest skip walk", document.Digest.SkipWalk, tierPySortedKeys(digest.SkipWalk)},
		{"digest top dirs", document.Digest.TopDirs, tierPySortedKeys(digest.TopDirs)},
		{"CLI paths", document.Repository.CLIPaths, repository.CLIPaths[:]},
		{"tier default areas", document.Tier.DefaultAreas, tier.DefaultAreas[:]},
		{"tier legal categories", document.Tier.LegalCategories, tierPySortedCopy(tier.LegalNonEngineCategories[:])},
		{"tier domain library prefixes", document.Tier.DomainLibraryPrefixes, tier.DomainLibraryPrefixes[:]},
		{"tier non-feature prefixes", document.Tier.NonFeaturePrefixes, tier.NonFeaturePrefixes[:]},
		{"tier non-production prefixes", document.Tier.DisableableNonProdPrefixes, tier.DisableableNonProdPrefixes[:]},
		{"tier lint base tags", document.Tier.GolangciBaseTags, tierPySortedCopy(tier.GolangciBaseTags[:])},
		{"tier core forbidden", document.Tier.CoreForbidden, tier.CoreForbidden[:]},
		{"tier core fix routes", document.Tier.CoreFixRoutes, tier.CoreFixRoutes[:]},
	}
	for _, pair := range lists {
		if !slices.Equal(pair.script, pair.command) {
			t.Errorf("%s: the script says %q and the command says %q", pair.name, pair.script, pair.command)
		}
	}

	// The dispatch allowlist is a map of sets. An output comparison cannot reach
	// its entries because only grpc-go calls the exempt methods. This repository
	// has no such calls.
	if len(document.Repository.InterfaceDispatch) != len(repository.InterfaceDispatchMethods) {
		t.Fatalf("the dispatch allowlist holds %d sites in the script and %d in the command",
			len(document.Repository.InterfaceDispatch), len(repository.InterfaceDispatchMethods))
	}
	for key, methods := range document.Repository.InterfaceDispatch {
		pkg, receiver, _ := strings.Cut(key, " ")
		site := repository.DispatchSite{Package: pkg, Receiver: receiver}
		got, ok := repository.InterfaceDispatchMethods[site]
		if !ok {
			t.Errorf("the command holds no dispatch site for %q", key)
			continue
		}
		if !slices.Equal(methods, tierPySortedKeys(got)) {
			t.Errorf("%s: the script exempts %q and the command exempts %q", key, methods, tierPySortedKeys(got))
		}
	}
}

// tierPyTrimMultiline drops the Go inline flag from a pattern, because Python
// passes re.MULTILINE as an argument and the flag never reaches its own pattern
// string.
func tierPyTrimMultiline(pattern string) string {
	return strings.TrimPrefix(pattern, "(?m)")
}

// tierPySortedKeys answers a set's members in order, which is how the Python
// side renders its own sets.
func tierPySortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// tierPySortedCopy answers a sorted copy, so comparing against a Python
// sorted() never reorders a package-level list.
func tierPySortedCopy(items []string) []string {
	out := slices.Clone(items)
	slices.Sort(out)
	return out
}

// tierPySGR matches a color escape, so a comparison can be about the page
// rather than about the shade.
var tierPySGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// tierPyPlain strips the color from a page. Only the repository gate colors
// itself, and only its two halves disagree about the shade.
func tierPyPlain(page string) string { return tierPySGR.ReplaceAllString(page, "") }
