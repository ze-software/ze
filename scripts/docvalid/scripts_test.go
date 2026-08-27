// Smoke tests for the //go:build ignore scripts in this directory.
//
// commands.go and doc_drift.go both use //go:build ignore so they are
// excluded from normal compilation and from golangci-lint's type-checking
// pipeline (the linter reports "build constraints exclude all Go files").
// This test file does NOT have the ignore tag, so go test sees it as the
// only file in the package and gives the linter and verify-changed a real
// target. Each test then runs the script as a subprocess via "go run" from
// the package directory and verifies it produces the expected output header.
//
// Purpose: catch regressions where a script's transitive dependencies
// (handler imports, schema imports, plugin registry) break the script
// without anyone noticing until the next manual run.

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scriptTimeout bounds how long each smoke test will wait for a script to
// finish. The scripts load YANG modules and walk plugin registries, so a
// few seconds is normal. 60s is generous enough that even a cold run on
// virtualised CI will complete.
const scriptTimeout = 60 * time.Second

// shippedTags returns the build tags a shipped `ze` carries: ze_core plus every
// gate declared in feature-gates.txt. The scripts here ENUMERATE runtime
// registries (address families, commands, YANG modules) through
// internal/component/plugin/all, so an untagged `go run` sees only the
// always-on subset -- since spec-feature-gate-10-bgp that excludes the whole BGP
// subtree -- and reports drift against documentation that is in fact correct.
// Derived from the manifest, never hardcoded (ai/rules/plugins.md).
func shippedTags(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "feature-gates.txt"))
	if err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	seen := map[string]bool{}
	tags := []string{"ze_core"}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag := strings.Fields(line)[0]
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return strings.Join(tags, " ")
}

// goRunScript builds the argv for running one of this directory's
// //go:build ignore scripts with the shipped feature tags.
func goRunScript(t *testing.T, script string, extra ...string) []string {
	t.Helper()
	return append([]string{"run", "-tags", shippedTags(t), script}, extra...)
}

// VALIDATES: scripts/docvalid/commands.go compiles and runs end-to-end.
// PREVENTS: a //go:build ignore script silently breaking when its handler
// or schema imports are renamed or refactored. The script may exit non-zero
// (orphans are an expected baseline) but it MUST produce its header line.
func TestValidateCommandsScriptRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/commands.go")...)
	cmd.Dir = repoRoot(t)
	out, _ := cmd.CombinedOutput() //nolint:errcheck // exit code is informational; we assert on stdout
	if !strings.Contains(string(out), "Command Validation") {
		t.Fatalf("commands.go did not produce expected 'Command Validation' header:\n%s", out)
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go compiles and runs end-to-end.
// PREVENTS: silent break of doc_drift via plugin registry refactor.
// The script should normally print "No documentation drift detected"; if a
// future change introduces drift, it must still run to completion and report it.
func TestDocDriftScriptRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go")...)
	cmd.Dir = repoRoot(t)
	out, _ := cmd.CombinedOutput() //nolint:errcheck // exit code is informational; we assert on stdout/stderr
	s := string(out)
	if !strings.Contains(s, "documentation drift") &&
		!strings.Contains(s, "Documentation drift") &&
		!strings.Contains(s, "No documentation drift") {
		t.Fatalf("doc_drift.go did not produce expected output:\n%s", s)
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go follows Makefile include files when
// deriving ze-functional-test suites.
// PREVENTS: docs drift falling back to a tool failure after the Makefile is split.
func TestDocDriftDerivesFunctionalSuites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go")...)
	cmd.Dir = repoRoot(t)
	out, _ := cmd.CombinedOutput() //nolint:errcheck // drift exit code is informational here
	s := string(out)
	if strings.Contains(s, "could not derive ze-functional-test suites") {
		t.Fatalf("doc_drift.go did not derive functional suites from split Makefiles:\n%s", s)
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go rejects stale parser allocation claims.
// PREVENTS: reintroducing the old strings.Fields text-parser documentation after
// the parser moved to textparse.NewScanner.
// VALIDATES: scripts/docvalid/doc_drift.go treats "N+ interop scenarios" as a FLOOR.
// PREVENTS: the churn of editing docs/DESIGN.md and reddening the gate every time a
// scenario is added. That line was corrected twice in one day for no reader's benefit.
// The floor still catches the case that matters -- the page claiming more than exists.
func TestDocDriftInteropFloorClaim(t *testing.T) {
	scenarios := []string{"01-a", "02-b", "03-c"} // the real count for this fixture

	// A bare fixture DESIGN.md trips dozens of unrelated checks (missing plugin
	// tables and so on), so the exit code says nothing about the interop claim.
	// Assert on the interop MESSAGE instead: present means rejected, absent means
	// accepted. Keying on the exit code here would have passed for every case.
	const marker = "interop scenarios, actual is"

	for _, tc := range []struct {
		name   string
		claim  string
		reject bool
		want   string
	}{
		{name: "floor below actual is accepted", claim: "2+ interop scenarios run here."},
		{name: "floor equal to actual is accepted", claim: "3+ interop scenarios run here."},
		{name: "exact bare number is accepted", claim: "3 interop scenarios run here."},
		{name: "floor above actual is rejected", claim: "9+ interop scenarios run here.", reject: true, want: "claims at least 9 interop scenarios, actual is 3"},
		{name: "bare number still checked exactly", claim: "2 interop scenarios run here.", reject: true, want: "claims 2 interop scenarios, actual is 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, s := range scenarios {
				writeTempDoc(t, root, "test/interop/scenarios/"+s+"/check.py", "def check():\n    pass\n")
			}
			writeTempDoc(t, root, "docs/DESIGN.md", "# Design\n\n"+tc.claim+"\n")

			ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
			defer cancel()
			cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go", "--root", root)...)
			cmd.Dir = repoRoot(t)
			out, _ := cmd.CombinedOutput()

			got := strings.Contains(string(out), marker)
			if got != tc.reject {
				t.Fatalf("claim %q against %d scenarios: interop complaint present=%v, want %v:\n%s",
					tc.claim, len(scenarios), got, tc.reject, out)
			}
			if tc.want != "" && !strings.Contains(string(out), tc.want) {
				t.Fatalf("doc_drift.go did not report %q:\n%s", tc.want, out)
			}
		})
	}
}

func TestDocDriftRejectsStaleTextParserFieldsClaim(t *testing.T) {
	root := t.TempDir()
	writeTempDoc(t, root, "docs/architecture/api/text-parser.md", "# Text Parser Architecture\n\nAll functions allocate via `strings.Fields()`.\n")

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go", "--root", root)...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("doc_drift.go accepted stale strings.Fields parser claim:\n%s", out)
	}
	if !strings.Contains(string(out), "stale text parser claim references strings.Fields") {
		t.Fatalf("doc_drift.go did not report the stale parser claim:\n%s", out)
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go accepts scanner-based parser docs.
// PREVENTS: the narrow stale-claim check from rejecting the current source-linked wording.
func TestDocDriftAllowsScannerTextParserDoc(t *testing.T) {
	root := t.TempDir()
	writeTempDoc(t, root, "docs/architecture/api/text-parser.md", "# Text Parser Architecture\n\nThe parser uses `textparse.NewScanner` for token-by-token scanning.\n")

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go", "--root", root)...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doc_drift.go rejected scanner parser wording: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "No documentation drift detected") {
		t.Fatalf("doc_drift.go did not report a clean scanner fixture:\n%s", out)
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go's checkReadmeMD flags BARE exact
// README test-count claims in both directions (over-claim AND undercount), while
// still tolerating soft `N+` at-least claims whose floor is met
// (spec-fixit-doc-gate-and-refs AC-3). Before this, only `([\d,]+)\+`-style
// over-claims were caught, so a bare `57 fuzz targets` or a bare `10,000 unit
// tests` slipped through unseen.
// PREVENTS: a bare headline count in README drifting from the live tree without
// the doc gate noticing, and PREVENTS a regression that would start flagging the
// soft `N+` phrasing the project deliberately uses to avoid count re-drift (R-1).
func TestCheckReadmeMDFlagsBareAndUndercount(t *testing.T) {
	root := t.TempDir()

	// Mini source tree so doc_drift's live counts are deterministic:
	// countFuzzTargets walks the whole root (3 `func Fuzz`), countGoTestFunctions
	// walks internal/pkg/cmd (5 `func Test`). doc_drift only text-scans for the
	// `^func Fuzz`/`^func Test` prefixes, so the file need not compile.
	writeTempDoc(t, root, "internal/z/z_test.go",
		"package z\n\n"+
			"func FuzzA()\nfunc FuzzB()\nfunc FuzzC()\n"+
			"func TestA()\nfunc TestB()\nfunc TestC()\nfunc TestD()\nfunc TestE()\n")

	// One README exercises all four cases in a single run:
	//   bare over-claim (5 vs 3), bare exact-match (3 vs 3, tolerated),
	//   at-least over-claim (9+ vs 3, flagged), at-least met (1+ vs 3, tolerated),
	//   bare undercount for a second unit (2 unit tests vs 5, flagged).
	writeTempDoc(t, root, "README.md",
		"# Ze\n\n"+
			"bare over-claim: 5 fuzz targets in the tree\n"+
			"bare exact match: 3 fuzz targets in the tree\n"+
			"at-least over-claim: 9+ fuzz targets in the tree\n"+
			"at-least tolerated: 1+ fuzz targets in the tree\n"+
			"bare undercount: 2 unit tests in the tree\n")

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go", "--root", root)...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("doc_drift.go accepted bare mismatched README counts:\n%s", out)
	}
	s := string(out)

	mustFlag := []string{
		"claims 5 fuzz targets (bare exact count), actual is 3", // bare over-claim
		"claims 9+ fuzz targets, actual is 3",                   // at-least over-claim
		"claims 2 unit tests (bare exact count), actual is 5",   // bare undercount
	}
	for _, want := range mustFlag {
		if !strings.Contains(s, want) {
			t.Errorf("doc_drift.go did not flag expected drift %q:\n%s", want, s)
		}
	}
	mustNotFlag := []string{
		"claims 3 fuzz targets (bare exact count)", // bare exact match: tolerated
		"claims 1+ fuzz targets",                   // at-least floor met: tolerated
	}
	for _, unwanted := range mustNotFlag {
		if strings.Contains(s, unwanted) {
			t.Errorf("doc_drift.go wrongly flagged tolerated claim %q:\n%s", unwanted, s)
		}
	}
}

// VALIDATES: scripts/dev/code_to_docs.py accepts the source-anchor separators
// already used by docs.
// PREVENTS: false stale references when anchors use a single hyphen separator.
func TestCodeToDocsParsesSourceAnchorSeparators(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	script := "import sys\n" +
		"sys.path.insert(0, 'scripts/dev')\n" +
		"import code_to_docs\n" +
		"paths = code_to_docs.extract_paths('internal/core/metrics/metrics.go - Registry interface; internal/core/metrics/prometheus.go -- PrometheusRegistry; internal/core/metrics/nop.go \\u2014 NopRegistry')\n" +
		"print('\\n'.join(paths))\n"
	cmd := osexec.CommandContext(ctx, "python3", "-c", script)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("code_to_docs.py import failed: %v\n%s", err, out)
	}
	s := string(out)
	for _, want := range []string{
		"internal/core/metrics/metrics.go",
		"internal/core/metrics/prometheus.go",
		"internal/core/metrics/nop.go",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing parsed path %q in:\n%s", want, s)
		}
	}
}

// VALIDATES: scripts/dev/code_to_docs.py --check reports stale anchors without
// regenerating ai/CODE-TO-DOCS.md.
// PREVENTS: audit/check mode mutating generated documentation indexes.
func TestCodeToDocsCheckModeIsReadOnly(t *testing.T) {
	root := repoRoot(t)
	indexPath := filepath.Join(root, "ai", "CODE-TO-DOCS.md")

	// The index is generated and gitignored (.gitignore), so a clean checkout
	// does not carry it: a worktree, a fresh clone and a from-scratch CI run all
	// start without it. Reading it directly made this test pass on a machine
	// that had run the generator and fail everywhere else, which is a verdict
	// about the checkout rather than about check mode. Generate it when it is
	// absent, so the assertion below measures the same thing wherever it runs.
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		genCtx, genCancel := context.WithTimeout(context.Background(), scriptTimeout)
		gen := osexec.CommandContext(genCtx, "python3", "scripts/dev/code_to_docs.py")
		gen.Dir = root
		genOut, genErr := gen.CombinedOutput()
		genCancel()
		if genErr != nil {
			t.Fatalf("generate code-to-docs index: %v\n%s", genErr, genOut)
		}
	}

	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read code-to-docs index before check: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "python3", "scripts/dev/code_to_docs.py", "--check")
	cmd.Dir = root
	out, _ := cmd.CombinedOutput() //nolint:errcheck // stale refs may make check exit non-zero
	if strings.Contains(string(out), "wrote ") {
		t.Fatalf("check mode reported a write:\n%s", out)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read code-to-docs index after check: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("code-to-docs index changed in check mode")
	}
}

// VALIDATES: scripts/docvalid/doc_drift.go reports a source file it could not
// read in full, instead of counting the lines it managed to reach.
// PREVENTS: a low test or fuzz count agreeing with a document that understates
// the tree, because the scan stopped on a line above bufio.MaxScanTokenSize and
// nobody was told.
func TestDocDriftReportsUnreadableSource(t *testing.T) {
	root := t.TempDir()
	readToolSource(t, "func countMatchingLines(")

	// One line above bufio.MaxScanTokenSize (64 KiB) stops the scan, so the
	// `func Test` below it is never counted.
	writeTempDoc(t, root, "internal/z/z_test.go",
		"package z\n\n// "+strings.Repeat("x", 70*1024)+"\nfunc TestA()\n")

	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/doc_drift.go", "--root", root)...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("doc_drift.go counted a file it could not read in full:\n%s", out)
	}
	if !strings.Contains(string(out), "read stopped early") {
		t.Fatalf("doc_drift.go did not report the unreadable file:\n%s", out)
	}
	if !strings.Contains(string(out), "z_test.go") {
		t.Fatalf("doc_drift.go did not name the unreadable file:\n%s", out)
	}
}

// readToolSource reads a build-ignored tool in this directory and checks it
// still holds want.
//
// The read is what binds the test cache to the tool. A build-ignored file is
// not an input to this test package's build, and a subprocess read is not an
// input to the cache either, so without this an edit to the tool comes back as
// a cached pass.
func readToolSource(t *testing.T, want string) {
	t.Helper()
	src, err := os.ReadFile("doc_drift.go")
	if err != nil {
		t.Fatalf("read the tool under test: %v", err)
	}
	if !strings.Contains(string(src), want) {
		t.Fatalf("doc_drift.go no longer holds %q; this test drives the wrong tool", want)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func writeTempDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create doc directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp doc: %v", err)
	}
}

// TestAllYangCommandsHaveRegisteredRPC asserts that every YANG ze:command
// leaf (every command WireMethod) has a registered handler. This is the
// command-surface-ownership contract (AC-5/AC-6): relocating a handler into
// its owner package must not drop its registration. Unlike
// TestValidateCommandsScriptRuns, which only asserts the script runs, this
// test fails if any YANG command is left without a handler -- exactly the
// breakage an owner move can silently cause when one of the four command
// import islands (plugin/all, config/yang/cli/tree.go, cli/client/main.go,
// and this validator) is not kept in sync.
func TestAllYangCommandsHaveRegisteredRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/commands.go")...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput() //nolint:errcheck // asserted on stdout below
	s := string(out)
	if !strings.Contains(s, "Command Validation") {
		t.Fatalf("commands.go did not run:\n%s", s)
	}
	if strings.Contains(s, "YANG commands with no handler") {
		t.Fatalf("a YANG command has no registered handler (an owner move dropped "+
			"a registration across the command import islands):\n%s", s)
	}
	if !strings.Contains(s, "All commands validated.") {
		t.Fatalf("command contract not satisfied (expected \"All commands validated.\"):\n%s\nerr=%v", s, err)
	}
}

// TestLocalDataRegistrationsAreCounted proves the validator sees a command that
// answers with DATA rather than by printing.
//
// commands.go finds local handlers by parsing source for calls named
// MustRegisterLocal, MustRegisterLocalMeta, RegisterLocal and RegisterLocalMeta.
// RegisterLocalData was added without being added there, so on the day twelve
// commands GAINED a handler the validator reported that they had none, and
// counted 25 local handlers where there were 37.
//
// TestAllYangCommandsHaveRegisteredRPC catches the same regression, and this
// names the cause: it fails on the one wire method rather than on whichever
// twelve happen to be converted, so the next reader is pointed at the
// registration spelling instead of at an owner move.
//
// VALIDATES: the validator's registration-name list covers every way a command
// registers a local handler.
// PREVENTS: a new registration API blinding this checker silently.
func TestLocalDataRegistrationsAreCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t, "scripts/docvalid/commands.go")...)
	cmd.Dir = repoRoot(t)
	out, _ := cmd.CombinedOutput() //nolint:errcheck // asserted on stdout below
	s := string(out)

	if !strings.Contains(s, "Command Validation") {
		t.Fatalf("commands.go did not run:\n%s", s)
	}
	// Scoped to the SECTION, not to the whole output. The validator also prints
	// a table of every wire method, so a bare Contains for the name matches a
	// line that is not a failure -- the first draft of this test did exactly
	// that and failed against a healthy tree.
	const heading = "## YANG commands with no handler"
	start := strings.Index(s, heading)
	if start < 0 {
		return // no command is unhandled, which is the passing state
	}
	section := s[start:]
	if end := strings.Index(section[len(heading):], "\n## "); end >= 0 {
		section = section[:len(heading)+end]
	}
	// `show env list` registers through RegisterLocalData.
	if strings.Contains(section, "ze-show:env-list") {
		t.Fatalf("a command registered with RegisterLocalData reads as unhandled; "+
			"the validator's registration-name list is missing a spelling:\n%s", section)
	}
}

// TestDocDriftOperatorTableIgnoresFixtureRoots pins BOTH directions of the
// pipe operator-table check's scope.
//
// doc_drift.go is run two ways: over the repository, and with --root pointing
// at a temporary tree holding one or two documents to check a single claim.
// The check demanded docs/features/pipe-operators.generated.md under whatever
// root it was given, so every fixture invocation reported it missing, which is
// a finding about the fixture rather than about the documentation.
//
// The second half matters more than the first: a scope narrowed to fix a false
// positive is one keystroke from a check that never fires at all.
//
// VALIDATES: the check judges a tree that carries docs/features, and only that.
// PREVENTS: the narrowing silently disabling the check.
func TestDocDriftOperatorTableIgnoresFixtureRoots(t *testing.T) {
	runDrift := func(t *testing.T, root string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
		defer cancel()
		cmd := osexec.CommandContext(ctx, "go",
			goRunScript(t, "scripts/docvalid/doc_drift.go", "--root", root)...)
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// A tree with no docs/features owes no generated table.
	bare := t.TempDir()
	writeTempDoc(t, bare, "docs/architecture/api/text-parser.md",
		"# Text Parser Architecture\n\nThe parser uses `textparse.NewScanner` for token-by-token scanning.\n")
	out, err := runDrift(t, bare)
	if err != nil {
		t.Fatalf("a fixture root with no docs/features was judged:\n%s", out)
	}

	// A tree that HAS docs/features and lacks the table is still reported, so
	// the narrowing did not turn the check off.
	populated := t.TempDir()
	writeTempDoc(t, populated, "docs/features/formatting.md", "# Output Formatting\n")
	out, err = runDrift(t, populated)
	if err == nil {
		t.Fatalf("a tree carrying docs/features and no operator table was accepted:\n%s", out)
	}
	if !strings.Contains(out, "the generated pipe operator reference is missing") {
		t.Fatalf("the operator-table check did not fire on a tree that owes one:\n%s", out)
	}
}

// VALIDATES: The per-command website catalog is compared structurally with the
// live `ze help command --json` contract.
// PREVENTS: AC-15 passing after one command loses an operator, an availability
// qualifier, or an alias while the global operator table remains unchanged.
func TestDocDriftRejectsPerCommandCatalogMutations(t *testing.T) {
	readToolSource(t, "checkPublishedCommandSurfaces")
	const liveCatalog = `[{
  "path": "show test",
  "description": "Show test rows",
  "mode": "read-only",
  "wire-method": "ze-show:test",
  "args": [{"name": "family", "type": "enum", "values": ["ipv4"], "mandatory": true}],
  "pipes": [{"name": "family", "description": "Filter by family", "takes-arg": true}],
  "operators": [
    {"name": "json", "class": "global", "available": "always", "description": "JSON output"},
    {"name": "match", "class": "data", "available": "with-rows", "description": "Keep matching rows"},
    {"name": "log", "class": "stream", "available": "when-streaming", "description": "Append updates"}
  ],
  "answer-shape": "tab",
  "address-fields": ["address"],
  "pipe-aliases": [{"name": "summary", "description": "Show a summary", "expansion": "display address"}]
}]`

	for _, tc := range []struct {
		name      string
		published string
	}{
		{
			name: "one operator",
			published: strings.Replace(
				liveCatalog, `"name": "match"`, `"name": "count"`, 1),
		},
		{
			name: "one qualifier",
			published: strings.Replace(
				liveCatalog, `"available": "when-streaming"`, `"available": "always"`, 1),
		},
		{
			name: "one alias",
			published: strings.Replace(
				liveCatalog, `"name": "summary"`, `"name": "brief"`, 1),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := filepath.Join(root, "live.json")
			writeTempDoc(t, root, "live.json", liveCatalog)
			writeTempDoc(t, root, "website/data/cli-commands.json", tc.published)

			ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
			defer cancel()
			cmd := osexec.CommandContext(ctx, "go", goRunScript(t,
				"scripts/docvalid/doc_drift.go",
				"--root", root,
				"--command-catalog", livePath,
			)...)
			cmd.Dir = repoRoot(t)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("doc drift accepted a per-command %s mutation:\n%s", tc.name, out)
			}
			if !strings.Contains(string(out),
				"the published website command catalog and the live command catalog disagree") {
				t.Fatalf("doc drift did not report the per-command %s mutation:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: Per-command catalog read and parse failures are findings, never a
// skipped comparison.
// PREVENTS: AC-15 passing vacuously when command generation or a published
// catalog cannot be consumed.
func TestDocDriftCommandCatalogErrorsFailClosed(t *testing.T) {
	readToolSource(t, "loadLiveCommandCatalog")

	t.Run("malformed published catalog", func(t *testing.T) {
		root := t.TempDir()
		livePath := filepath.Join(root, "live.json")
		writeTempDoc(t, root, "live.json", renderedCommandCatalogFixture)
		writeTempDoc(t, root, "website/data/cli-commands.json", `{`)

		ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
		defer cancel()
		cmd := osexec.CommandContext(ctx, "go", goRunScript(t,
			"scripts/docvalid/doc_drift.go",
			"--root", root,
			"--command-catalog", livePath,
		)...)
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("doc drift accepted malformed published JSON:\n%s", out)
		}
		if !strings.Contains(string(out), "could not parse the published website command catalog") {
			t.Fatalf("doc drift did not report the published parse error:\n%s", out)
		}
	})

	t.Run("unreadable live catalog", func(t *testing.T) {
		root := t.TempDir()
		missing := filepath.Join(root, "missing.json")
		writeTempDoc(t, root, "website/data/cli-commands.json",
			`[{"path":"show test","mode":"read-only"}]`)

		ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
		defer cancel()
		cmd := osexec.CommandContext(ctx, "go", goRunScript(t,
			"scripts/docvalid/doc_drift.go",
			"--root", root,
			"--command-catalog", missing,
		)...)
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("doc drift accepted an unreadable live catalog:\n%s", out)
		}
		if !strings.Contains(string(out), "could not generate or parse the live per-command catalog") {
			t.Fatalf("doc drift did not report the live read error:\n%s", out)
		}
	})
}

const renderedCommandCatalogFixture = `[{
  "path": "show test",
  "description": "Show test rows",
  "mode": "read-only",
  "wire-method": "ze-show:test",
  "args": [{"name": "family", "type": "enum", "values": ["ipv4"], "mandatory": true}],
  "pipes": [{"name": "family", "description": "Filter by family", "takes-arg": true}],
  "operators": [
    {"name": "json", "class": "global", "available": "always", "description": "JSON output"},
    {"name": "save", "class": "global", "available": "always", "local-only": true, "description": "Save output"},
    {"name": "match", "class": "data", "available": "with-rows", "description": "Keep matching rows"},
    {"name": "log", "class": "stream", "available": "when-streaming", "description": "Append updates"}
  ],
  "answer-shape": "tab",
  "address-fields": ["address"],
  "pipe-aliases": [{"name": "summary", "description": "Show a summary", "expansion": "display address"}]
}]`

func runRenderedCommandDriftFixture(t *testing.T, root, livePath string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", goRunScript(t,
		"scripts/docvalid/doc_drift.go",
		"--root", root,
		"--command-catalog", livePath,
	)...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeRenderedCommandCatalogFixture(t *testing.T, root string) string {
	t.Helper()
	writeTempDoc(t, root, "live.json", renderedCommandCatalogFixture)
	return filepath.Join(root, "live.json")
}

func writePublishedCommandSurfaceFixture(t *testing.T, root string, dropAddress bool) {
	t.Helper()
	writeTempDoc(t, root, "website/data/cli-commands.json",
		renderedCommandCatalogFixture)
	primaryHTML := `<html data-site-postprocessed="true"><body>
<header>Injected site header <span>Always</span><code>catalog-absent</code></header>
<section class="cli-pipe-guide">
<table><tbody>
<tr><td><code>json</code></td><td>Output and control</td><td>Always</td><td>JSON output</td></tr>
<tr><td><code>save</code></td><td>Output and control</td><td>Always, Local process only</td><td>Save output</td></tr>
<tr><td><code>match</code></td><td>Row data</td><td>With rows</td><td>Keep matching rows</td></tr>
<tr><td><code>log</code></td><td>Streaming</td><td>While streaming</td><td>Append updates</td></tr>
</tbody></table>
</section>
<tr id="cmd-show-test"><td><code>show test</code></td><td>
<p><span>Answer shape</span><code>tab</code></p>
<p><span>Address fields</span><code>address</code></p>
<strong>Command pipes</strong><div class="cli-pipe-chips"><code title="Filter by family">family &lt;value&gt;</code></div>
<details class="cli-pipe-descriptions"><summary>Command pipe descriptions</summary><dl><dt><code>family &lt;value&gt;</code></dt><dd>Filter by family</dd></dl></details>
<strong>Aliases</strong><dl><dt><code>summary</code></dt><dd>Show a summary <code>display address</code></dd></dl>
<p><span>Always</span><code>json · save</code></p>
<p><span>With rows</span><code>match</code></p>
<p><span>While streaming</span><code>log</code></p>
<p><span>Local process only</span><code>save</code></p>
</td></tr>
<footer>Injected publication stamp</footer>
</body></html>
`
	if dropAddress {
		primaryHTML = strings.Replace(primaryHTML,
			"<p><span>Address fields</span><code>address</code></p>\n", "", 1)
	}
	writeTempDoc(t, root, "website/reference/cli/index.html", primaryHTML)
	writeTempDoc(t, root, "website/reference/cli/index.md",
		strings.Join([]string{
			"# CLI Reference",
			"Always: `catalog-absent`",
			"",
			"",
			"| Command | Mode | Description | Pipes |",
			"| --- | --- | --- | --- |",
			"| `show test` | Read-only | Show test rows | Answer shape: `tab`<br>Address fields: `address`<br>Command: `family <value>`<br>Aliases: `summary -> display address`<br>Always: `json`, `save`<br>With rows: `match`<br>While streaming: `log`<br>Local process only: `save` |",
			"",
		}, "\n"))
	writeTempDoc(t, root, "website/reference/command-equivalents/index.html",
		`<html data-site-postprocessed="true"><body><tr id="cmd-eq-show-test"><td><code>show test</code></td></tr></body></html>
`)
	writeTempDoc(t, root, "website/reference/command-equivalents/index.md",
		"# Command Equivalents\n\n| `show test` | Read-only | [details](show-test/) |\n")
	writeTempDoc(t, root,
		"website/reference/command-equivalents/show-test/index.html",
		`<html data-site-postprocessed="true"><body>
<aside><dt>Pipes, always</dt><dd>catalog-absent</dd></aside>
<article class="cmd-detail-card cmd-detail-ze">
<div><dt>Pipes, always</dt><dd>json, save</dd></div>
<div><dt>Pipes, on its rows</dt><dd>match</dd></div>
<div><dt>Pipes, while streaming</dt><dd>log</dd></div>
<div><dt>Pipes, local process only</dt><dd>save</dd></div>
<div><dt>Command pipes</dt><dd><code>family &lt;value&gt;</code>: Filter by family</dd></div>
<div><dt>Pipe aliases</dt><dd><code>summary</code>: Show a summary (<code>display address</code>)</dd></div>
<div><dt>Answer shape</dt><dd>tab</dd></div>
<div><dt>Address fields</dt><dd>address</dd></div>
</article>
</body></html>
`)
	writeTempDoc(t, root,
		"website/reference/command-equivalents/show-test/index.md",
		strings.Join([]string{
			"# `show test`",
			"",
			"## Ze command",
			"",
			"- Registry path: `show test`",
			"- Answer shape: tab",
			"- Address fields: address",
			"- Pipes, always: json, save",
			"- Pipes, on rows: match",
			"- Pipes, while streaming: log",
			"- Pipes, local process only: save",
			"- Command pipes: `family <value>`: Filter by family",
			"- Pipe aliases: `summary`: Show a summary (`display address`)",
			"## Mapping intents",
			"",
			"- Pipes, always: catalog-absent",
			"",
			"",
		}, "\n"))
	writeTempDoc(t, root, "website/llms.txt",
		strings.Join([]string{
			"# Ze",
			"",
			"## CLI command surface",
			"Site note: pipes always: catalog-absent",
			"",
			"",
			"- `show test` (read-only; wire ze-show:test; pipes always: json save, with-rows: match, when-streaming: log, local-only: save; shape tab; address-fields address; filters family; aliases summary=display address; args family:enum): Show test rows",
			"",
		}, "\n"))
}

// VALIDATES: published HTML may carry normal site-pipeline wrappers while every
// per-command contract dimension remains structurally identical to live JSON.
// PREVENTS: raw-renderer byte comparison flagging headers, stamps, canonical
// rewrites, or asset versions as CLI drift.
func TestDocDriftAcceptsPublishedHTMLPostprocessing(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err != nil {
		t.Fatalf("doc drift rejected benign published HTML postprocessing:\n%s", out)
	}
}

// VALIDATES: structural published-HTML comparison still requires each command
// dimension after byte comparison is removed.
// PREVENTS: accepting a postprocessed primary page that dropped address fields.
func TestDocDriftRejectsPublishedHTMLContractLoss(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, true)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted published HTML without address fields:\n%s", out)
	}
	if !strings.Contains(out, "website/reference/cli/index.html") {
		t.Fatalf("doc drift did not identify the published primary HTML:\n%s", out)
	}
	if !strings.Contains(out, "missing address fields") {
		t.Fatalf("doc drift did not identify the dropped address dimension:\n%s", out)
	}
}

// VALIDATES: ze-doc-verify generates and checks canonical per-command surfaces
// when neither published sibling checkout exists.
// PREVENTS: a normal single-repository checkout returning clean without
// exercising any rendered command contract.
func TestDocDriftNoSiblingsStillValidatesRenderedCommands(t *testing.T) {
	readToolSource(t, "renderExpectedCommandSurfaces")
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err != nil {
		t.Fatalf("a complete no-sibling renderer fixture failed:\n%s", out)
	}
}

// VALIDATES: the no-sibling path checks independent command dimensions in the
// canonical renderer output rather than treating successful process exit as proof.
// PREVENTS: a renderer silently dropping one command's address contract while
// ze-doc-verify has no published sibling to compare.
func TestDocDriftNoSiblingsRejectsMutatedRendererContract(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	sourcePath := filepath.Join(repoRoot(t), "website", "tools", "render-llms-txt.py")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(source),
		"if address_fields:",
		"if False and address_fields:",
		1,
	)
	if mutated == string(source) {
		t.Fatal("the llms renderer mutation did not apply")
	}
	writeTempDoc(t, root, "website/tools/render-llms-txt.py", mutated)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a renderer that dropped address fields:\n%s", out)
	}
	if !strings.Contains(out, "generated per-command surface dropped part of the live command contract") {
		t.Fatalf("doc drift did not report the mutated renderer contract:\n%s", out)
	}
	if !strings.Contains(out, `address field "address"`) {
		t.Fatalf("doc drift did not identify the dropped address dimension:\n%s", out)
	}
}

// VALIDATES: the primary Markdown parser keeps the final local-only group
// bounded to the Pipes table cell and requires save under that exact qualifier.
// PREVENTS: a row-level name search passing when save remains under always but
// disappears from the independent local-process-only contract.
func TestDocDriftNoSiblingsRejectsPrimaryMarkdownQualifierMutation(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	sourcePath := filepath.Join(repoRoot(t), "website", "tools", "render-cli-catalog.py")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	original := `    for availability, names in operators_by_availability(command).items():
        parts.append(`
	replacement := `    for availability, names in operators_by_availability(command).items():
        if availability == "local-only":
            continue
        parts.append(`
	mutationAt := bytes.LastIndex(source, []byte(original))
	if mutationAt == -1 {
		t.Fatal("the primary Markdown qualifier mutation did not apply")
	}
	mutated := make([]byte, 0, len(source)+len(replacement)-len(original))
	mutated = append(mutated, source[:mutationAt]...)
	mutated = append(mutated, replacement...)
	mutated = append(mutated, source[mutationAt+len(original):]...)
	writeTempDoc(t, root, "website/tools/render-cli-catalog.py", string(mutated))

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted primary Markdown without local-only save:\n%s", out)
	}
	if !strings.Contains(out, "reference/cli/index.md") {
		t.Fatalf("doc drift did not identify the primary Markdown row:\n%s", out)
	}
	if !strings.Contains(out, `local-only surface qualifier for operator "save"`) {
		t.Fatalf("doc drift did not identify the exact missing qualifier:\n%s", out)
	}
}

// VALIDATES: canonical renderer execution errors are documentation findings.
// PREVENTS: a broken in-repo renderer degrading into a skipped surface check.
func TestDocDriftRendererErrorsFailClosed(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writeTempDoc(t, root, "website/tools/render-cli-catalog.py",
		`raise SystemExit("fixture renderer failure")`+"\n")

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a failing canonical renderer:\n%s", out)
	}
	if !strings.Contains(out, "could not generate the expected per-command surfaces") {
		t.Fatalf("doc drift did not report expected-surface generation failure:\n%s", out)
	}
	if !strings.Contains(out, "fixture renderer failure") {
		t.Fatalf("doc drift hid the renderer's corrective detail:\n%s", out)
	}
}

// VALIDATES: every published HTML, Markdown, and llms command surface is
// structurally checked against live JSON, and every generated page path exists.
// PREVENTS: current cli-commands.json masking stale human or agent-facing pages.
func TestDocDriftRejectsStaleRenderedCommandSurfaces(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writeTempDoc(t, root, "website/data/cli-commands.json",
		renderedCommandCatalogFixture)
	surfaces := []struct {
		name string
		path string
	}{
		{name: "primary CLI HTML", path: "reference/cli/index.html"},
		{name: "primary CLI Markdown", path: "reference/cli/index.md"},
		{name: "command equivalents HTML", path: "reference/command-equivalents/index.html"},
		{name: "command equivalents Markdown", path: "reference/command-equivalents/index.md"},
		{name: "llms", path: "llms.txt"},
	}
	for _, surface := range surfaces {
		writeTempDoc(t, root, filepath.Join("website", surface.path),
			"stale rendered command surface\n")
	}

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted stale rendered surfaces with current JSON:\n%s", out)
	}
	for _, surface := range surfaces {
		if !strings.Contains(out, filepath.ToSlash(surface.path)) {
			t.Fatalf("doc drift did not identify stale %s:\n%s", surface.name, out)
		}
	}
	if !strings.Contains(out,
		"the generated per-command surface dropped part of the live command contract") {
		t.Fatalf("doc drift did not structurally reject stale surfaces:\n%s", out)
	}
	if !strings.Contains(out, "reference/command-equivalents/show-test/index.html") {
		t.Fatalf("doc drift did not identify a missing generated detail page:\n%s", out)
	}
	if !strings.Contains(out, "the published per-command surface is missing or unreadable") {
		t.Fatalf("doc drift did not fail closed on missing published surfaces:\n%s", out)
	}
}

// VALIDATES: the wiki generator runs and its Markdown is checked structurally
// even when no sibling wiki checkout exists.
// PREVENTS: a missing sibling turning generator failure or a hard-coded operator
// into a clean documentation gate.
func TestDocDriftNoSiblingWikiFailsClosed(t *testing.T) {
	t.Run("generator failure", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writeTempDoc(t, root, "scripts/dev/gen_wiki_commands.py",
			"raise SystemExit(\"wiki fixture failure\")\n")

		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil {
			t.Fatalf("doc drift accepted a failing wiki generator without a sibling:\n%s", out)
		}
		if !strings.Contains(out, "could not generate the expected wiki command catalog") ||
			!strings.Contains(out, "wiki fixture failure") {
			t.Fatalf("doc drift hid the no-sibling wiki generator failure:\n%s", out)
		}
	})

	t.Run("generator syntax error", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writeTempDoc(t, root, "scripts/dev/gen_wiki_commands.py", "def broken(:\n")

		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil {
			t.Fatalf("doc drift accepted invalid wiki generator syntax:\n%s", out)
		}
		if !strings.Contains(out, "could not generate the expected wiki command catalog") ||
			!strings.Contains(out, "SyntaxError") {
			t.Fatalf("doc drift hid the wiki generator syntax error:\n%s", out)
		}
	})

	t.Run("empty output", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writeTempDoc(t, root, "scripts/dev/gen_wiki_commands.py", "pass\n")

		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil {
			t.Fatalf("doc drift accepted empty wiki generator output:\n%s", out)
		}
		if !strings.Contains(out, "could not generate the expected wiki command catalog") ||
			!strings.Contains(out, "empty output") {
			t.Fatalf("doc drift hid the empty wiki generator output:\n%s", out)
		}
	})

	t.Run("catalog-absent operator", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		source, err := os.ReadFile(filepath.Join(
			repoRoot(t), "scripts", "dev", "gen_wiki_commands.py",
		))
		if err != nil {
			t.Fatal(err)
		}
		old := `lines.append(f"Always: {names}")`
		replacement := "lines.append(f\"Always: {names}, `catalog-absent`\")"
		mutated := strings.Replace(string(source), old, replacement, 1)
		if mutated == string(source) {
			t.Fatal("the wiki operator mutation did not apply")
		}
		writeTempDoc(t, root, "scripts/dev/gen_wiki_commands.py", mutated)

		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil {
			t.Fatalf("doc drift accepted a catalog-absent wiki operator:\n%s", out)
		}
		if !strings.Contains(out, "catalog-absent operator") ||
			!strings.Contains(out, "scripts/dev/gen_wiki_commands.py") {
			t.Fatalf("doc drift did not identify the extra wiki operator:\n%s", out)
		}
	})
}

// VALIDATES: primary HTML and Markdown publish the exact command-owned filter
// and alias sets from the live catalog.
// PREVENTS: the global operator checks masking a dropped command-specific pipe.
func TestDocDriftRejectsDroppedPrimaryFiltersAndAliases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		old         string
		replacement string
	}{
		{
			name: "HTML filter",
			path: "reference/cli/index.html",
			old:  `<code title="Filter by family">family &lt;value&gt;</code>`,
		},
		{
			name: "HTML alias",
			path: "reference/cli/index.html",
			old:  `<strong>Aliases</strong><dl><dt><code>summary</code></dt><dd>Show a summary <code>display address</code></dd></dl>`,
		},
		{
			name: "Markdown filter",
			path: "reference/cli/index.md",
			old:  "Command: `family <value>`<br>",
		},
		{
			name: "Markdown alias",
			path: "reference/cli/index.md",
			old:  "Aliases: `summary -> display address`<br>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.replacement)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a dropped primary %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify the mutated primary surface:\n%s", out)
			}
		})
	}
}

// VALIDATES: every rendered command operator group is an exact set projection,
// not a one-way expected-name search.
// PREVENTS: a stale hard-coded operator surviving after catalog removal.
func TestDocDriftRejectsExtraOperatorsOnEveryRenderedSurface(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary HTML",
			path: "reference/cli/index.html",
			old:  "<span>Always</span><code>json · save</code>",
			new:  "<span>Always</span><code>json · save · catalog-absent</code>",
		},
		{
			name: "primary Markdown",
			path: "reference/cli/index.md",
			old:  "Always: `json`, `save`",
			new:  "Always: `json`, `save`, `catalog-absent`",
		},
		{
			name: "command-equivalent HTML",
			path: "reference/command-equivalents/show-test/index.html",
			old:  "<div><dt>Pipes, always</dt><dd>json, save</dd></div>",
			new:  "<div><dt>Pipes, always</dt><dd>json, save, catalog-absent</dd></div>",
		},
		{
			name: "command-equivalent Markdown",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "- Pipes, always: json, save",
			new:  "- Pipes, always: json, save, catalog-absent",
		},
		{
			name: "llms",
			path: "llms.txt",
			old:  "pipes always: json save,",
			new:  "pipes always: json save catalog-absent,",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted an extra operator on %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "catalog-absent operator") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify the extra %s operator:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: each command owns at most one correctly labeled operator group
// for an availability on every rendered surface.
// PREVENTS: a renderer hiding a catalog-absent operator in a second group after
// the first group has already satisfied the live contract.
func TestDocDriftRejectsDuplicateOperatorGroupsOnEveryRenderedSurface(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary HTML",
			path: "reference/cli/index.html",
			old:  "<p><span>Always</span><code>json · save</code></p>",
			new:  "<p><span>Always</span><code>json · save</code></p><p><span>Always</span><code>catalog-absent</code></p>",
		},
		{
			name: "primary Markdown",
			path: "reference/cli/index.md",
			old:  "Always: `json`, `save`",
			new:  "Always: `json`, `save`<br>Always: `catalog-absent`",
		},
		{
			name: "command-equivalent HTML",
			path: "reference/command-equivalents/show-test/index.html",
			old:  "<div><dt>Pipes, always</dt><dd>json, save</dd></div>",
			new:  "<div><dt>Pipes, always</dt><dd>json, save</dd></div><div><dt>Pipes, always</dt><dd>catalog-absent</dd></div>",
		},
		{
			name: "command-equivalent Markdown",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "- Pipes, always: json, save",
			new:  "- Pipes, always: json, save\n- Pipes, always: catalog-absent",
		},
		{
			name: "llms",
			path: "llms.txt",
			old:  "pipes always: json save,",
			new:  "pipes always: json save, always: catalog-absent,",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a duplicate operator group on %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "duplicate operator availability group") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify the duplicate %s group:\n%s", tc.name, out)
			}
		})
	}

	t.Run("wiki", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		source, err := os.ReadFile(filepath.Join(
			repoRoot(t), "scripts", "dev", "gen_wiki_commands.py",
		))
		if err != nil {
			t.Fatal(err)
		}
		old := `lines.append(f"Always: {names}")`
		replacement := old + "\n            lines.append(\"Always: `catalog-absent`\")"
		mutated := strings.Replace(string(source), old, replacement, 1)
		if mutated == string(source) {
			t.Fatal("the duplicate wiki operator mutation did not apply")
		}
		writeTempDoc(t, root, "scripts/dev/gen_wiki_commands.py", mutated)

		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil {
			t.Fatalf("doc drift accepted a duplicate wiki operator group:\n%s", out)
		}
		if !strings.Contains(out, "duplicate operator availability group") ||
			!strings.Contains(out, "scripts/dev/gen_wiki_commands.py") {
			t.Fatalf("doc drift did not identify the duplicate wiki group:\n%s", out)
		}
	})
}

// VALIDATES: a unique operator group preserves the catalog's ordered names.
// PREVENTS: set-only comparison accepting reordered generated documentation.
func TestDocDriftRejectsReorderedOperatorGroup(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t,
		root,
		"reference/cli/index.html",
		"<p><span>Always</span><code>json · save</code></p>",
		"<p><span>Always</span><code>save · json</code></p>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a reordered operator group:\n%s", out)
	}
	if !strings.Contains(out, "always operator order") ||
		!strings.Contains(out, "reference/cli/index.html") {
		t.Fatalf("doc drift did not identify the reordered operator group:\n%s", out)
	}
}

// VALIDATES: the primary HTML operator reference preserves catalog-owned class
// and description, with unrelated site postprocessing still allowed.
// PREVENTS: operator names remaining current while their explanatory metadata drifts.
func TestDocDriftRejectsStaleRenderedOperatorMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		old  string
		new  string
	}{
		{name: "class", old: "Output and control", new: "Stale class"},
		{name: "description", old: "JSON output", new: "Stale description"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, "reference/cli/index.html", tc.old, tc.new,
			)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a stale operator %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "generated operator metadata disagrees") ||
				!strings.Contains(out, tc.name) {
				t.Fatalf("doc drift did not identify stale operator %s:\n%s", tc.name, out)
			}
		})
	}
}

func mutatePublishedCommandSurface(
	t *testing.T,
	root, relative, old, replacement string,
) {
	t.Helper()
	path := filepath.Join(root, "website", filepath.FromSlash(relative))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(content), old, replacement, 1)
	if mutated == string(content) {
		t.Fatalf("surface mutation %q did not apply to %s", old, relative)
	}
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
}
