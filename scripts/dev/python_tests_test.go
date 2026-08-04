package main

// Brings the repo's Python unit tests into `go test`, so `make ze-unit-test`
// actually runs them.
//
// Before this, eight *_test.py files sat in scripts/dev and NOTHING executed them:
// the repo has no pytest and no `unittest discover`, no make target references
// *_test.py, and the Go tests in this package cover the Go tools only. They all
// passed when run by hand, so this wires up working tests rather than repairing
// broken ones. A test nothing runs is the test-side twin of unwired code: it reads
// as coverage and provides none (ai/rules/completion.md).
//
// Discovery is by glob rather than a hardcoded list, deliberately: a new
// <tool>_test.py is picked up with no further wiring, so this cannot rot the way
// the originals did. The empty-glob guard below is what makes that safe.
//
// It globs SEVERAL roots because the repo has more than one Python source root,
// and the single-directory version reproduced its own bug one directory over:
// test/scripts/ze_api.py is the observer helper library every .ci test imports,
// and its test file was invisible here purely because of where it lives. Its
// fail-closed sentinel guard -- the one that un-swallowed a whole class of silent
// .ci false-passes -- could be deleted with nothing going red. Each root carries
// its own non-empty assertion so a root that stops contributing tests fails
// loudly instead of quietly covering less.
//
// Two conventions coexist here on purpose. A Python tool with an in-script
// --selftest (dep_audit.py, migrate_module.py, qemu-run.py) gets a small Go test
// that shells out to it. A tool with a standalone <tool>_test.py is covered by
// this file. Both end up inside `go test`; neither needs a make target.

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// pythonTestRoots are the repo-relative directories searched for *_test.py.
// Every entry MUST contain at least one test file; see the per-root guard in
// TestPythonUnitTests for why that is an assertion rather than a convenience.
var pythonTestRoots = []string{
	"scripts/dev",
	"test/scripts",
	// The perf harness is a third Python source root, and it earned its place
	// the same way test/scripts did: test/perf/run.py writes a COMMITTED
	// document (docs/performance.md), and a truncate-before-generate bug there
	// emptied it to zero bytes during an ordinary benchmark run without failing
	// anything. Its guards are only guards if something runs their tests.
	"test/perf",
	// The IPsec interop lab parses the output of external commands, and nothing
	// ran its tests before 2026-07-31. A `bytes\s+(\d+)` pattern that matches no
	// iproute2 release lived in four copies of the scenarios for that reason.
	// Every copy read zero, so every "traffic flowed" assertion built on it
	// passed whatever the tunnel did. test/ipsec-interop/lab_test.py pins the
	// parser against captured `ip -s xfrm state` output.
	"test/ipsec-interop",
	// The interop speaker engine is the independent BGP peer several scenarios judge
	// ze with, so a bug in ITS decode makes a scenario pass or fail for the wrong
	// reason. Its tests sat unrun until 2026-08-04 because the file is named
	// test_engine.py (pytest style) and handed itself to pytest, which is not
	// installed: `python3 test_engine.py` exited with ImportError, and no root
	// covered the directory anyway.
	"test/interop/speaker",
}

// pythonTestGlobs are the file-name shapes that count as a Python test.
//
// Both conventions are live in this repository and neither is worth a mass rename:
// scripts/dev and test/scripts use <tool>_test.py, mirroring Go, while the interop
// speaker uses pytest's test_<tool>.py. A discovery rule that knew only one of them
// would silently cover half the corpus, which is the failure this whole file exists
// to prevent.
var pythonTestGlobs = []string{"*_test.py", "test_*.py"}

// test-relax: removes a DUPLICATE repoRoot helper I had added here, which did not
// compile -- the package already defines repoRoot(t) in verify_wiring_docs_test.go
// and Go rejects the redeclaration. No test coverage is lost: the existing helper
// carries the equivalent t.Fatalf, and every assertion in TestPythonUnitTests
// below is untouched. The hook counted the dead helper's error paths as removed
// assertions.
//
// The repo root therefore comes from the package's existing repoRoot helper, so
// the roots above stay repo-relative and there is one notion of "the root" here.
//
// TestPythonUnitTests runs every *_test.py under pythonTestRoots via `go test`.
//
// VALIDATES: the Python tooling's own unit tests execute in CI-equivalent runs
// (`make ze-unit-test` covers ./scripts/dev via `go list ./...`), across every
// Python source root rather than only the one this file happens to sit in.
// PREVENTS: a *_test.py silently never running, which is how all eight original
// scripts/dev ones ended up as dead weight that looked like coverage -- and how
// test/scripts/ze_api_test.py would have been born dead had it been dropped in
// beside its subject without widening the search.
func TestPythonUnitTests(t *testing.T) {
	root := repoRoot(t)

	total := 0
	for _, rel := range pythonTestRoots {
		dir := filepath.Join(root, rel)
		var matches []string
		for _, pattern := range pythonTestGlobs {
			found, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				t.Fatalf("globbing %s in %s: %v", pattern, rel, err)
			}
			matches = append(matches, found...)
		}
		// One file matching both shapes (test_x_test.py) would run twice; sorting and
		// de-duplicating keeps the count honest and the run order stable.
		slices.Sort(matches)
		matches = slices.Compact(matches)
		// An empty root means the layout moved and this wiring stopped covering
		// it. Fail loudly instead of passing vacuously, which is the exact
		// failure mode this test exists to prevent.
		if len(matches) == 0 {
			t.Fatalf("no %v found in %s: did the layout change? "+
				"This test must not pass vacuously -- fix the path in "+
				"pythonTestRoots or drop the root deliberately.", pythonTestGlobs, rel)
		}
		total += len(matches)

		for _, script := range matches {
			name, err := filepath.Rel(root, script)
			if err != nil {
				name = script
			}
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "python3", filepath.Base(script))
				// Run from the script's own directory so a test can import the
				// module it covers as a sibling, the way an observer does.
				cmd.Dir = filepath.Dir(script)
				out, err := cmd.CombinedOutput()
				code := 0
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					code = ee.ExitCode()
				} else if err != nil {
					t.Fatalf("running %s: %v", name, err)
				}
				if code != 0 {
					t.Fatalf("%s failed (exit %d):\n%s", name, code, string(out))
				}
			})
		}
	}

	if total == 0 {
		t.Fatal("no Python unit tests collected at all")
	}
}
