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

// VALIDATES: scripts/docvalid/commands.go compiles and runs end-to-end.
// PREVENTS: a //go:build ignore script silently breaking when its handler
// or schema imports are renamed or refactored. The script may exit non-zero
// (orphans are an expected baseline) but it MUST produce its header line.
func TestValidateCommandsScriptRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	cmd := osexec.CommandContext(ctx, "go", "run", "scripts/docvalid/commands.go")
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
	cmd := osexec.CommandContext(ctx, "go", "run", "scripts/docvalid/doc_drift.go")
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
	cmd := osexec.CommandContext(ctx, "go", "run", "scripts/docvalid/doc_drift.go")
	cmd.Dir = repoRoot(t)
	out, _ := cmd.CombinedOutput() //nolint:errcheck // drift exit code is informational here
	s := string(out)
	if strings.Contains(s, "could not derive ze-functional-test suites") {
		t.Fatalf("doc_drift.go did not derive functional suites from split Makefiles:\n%s", s)
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
	if string(before) != string(after) {
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
