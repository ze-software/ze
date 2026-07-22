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
// Derived from the manifest, never hardcoded (ai/rules/feature-gate-registration.md).
func shippedTags(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "feature-gates.txt"))
	if err != nil {
		t.Fatalf("read feature-gates.txt: %v", err)
	}
	seen := map[string]bool{}
	tags := []string{"ze_core"}
	for _, line := range strings.Split(string(data), "\n") {
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
