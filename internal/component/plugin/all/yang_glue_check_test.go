package all

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
)

// TestYANGGlueCurrent runs the YANG glue generator in --check mode.
//
// This is the feeder that makes `ze-yang-glue-check` reachable. The Makefile
// target existed but had no caller: not in `generate`'s check twin, not in
// stagesForMode (scripts/status/verify_run.go), not in .github/workflows. A `.yang`
// file added or edited without `make generate` therefore left a stale
// yang/*/register.go, and the module was silently never wired -- the failure
// mode is invisible, because nothing errors, the schema simply is not there.
//
// It mirrors TestGeneratedPluginImportsCurrent in all_test.go deliberately:
// same package (already covered by the unit stage), same shell-out-to-the-real
// -generator shape, so the check is exercised by the generator that writes the
// file rather than by a reimplementation that can drift.
//
// CAVEAT -- not a sufficient guard on its own. A .yang file is not an input of
// THIS package, so `go test` may serve a cached PASS after one is edited (the
// full-verify stage is ze-unit-test-cached). Measured: adding a stray .yang and
// re-running without -count=1 returned a cached ok. The uncached backstop is
// the `ze-generated-files-check` make stage, which runs the same
// `yang_glue.go --check` from a recipe and is wired into both stagesForMode
// branches. This feeder's value is the fast local signal; do not remove the
// make stage on the strength of it.
//
// VALIDATES: every yang/*/register.go is current with respect to its .yang
// sources, checked by the generator itself (scripts/codegen/yang_glue.go).
// PREVENTS: a YANG module silently never being wired because register.go was
// not regenerated -- config for that module would parse as unknown, with no
// build or test failure to point at the cause.
func TestYANGGlueCurrent(t *testing.T) {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "go", "run", "../../../../scripts/codegen/yang_glue.go", "--check")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("yang glue generated files are stale: %v\n%s\nRun `make generate` and commit the result.", err, out)
	}

	// Non-vacuity. yang_glue.go exits 0 with "no yang/ directories with .yang
	// files found" when discoverYangDirs matches nothing, so a layout change or
	// a broken walk turns this test (and the ze-generated-files-check line) green
	// while guarding zero files. Assert it actually looked at a plausible number
	// of directories, the same way TestPythonUnitTests fails on an empty glob.
	const minYangDirs = 100 // 149 at the time of writing; a floor, not a count
	m := yangDirCountRE.FindSubmatch(out)
	if m == nil {
		// Name the likely cause rather than a generic "shape changed": the
		// early return in yang_glue.go's main() is the failure this assertion
		// exists to catch, and it has its own distinctive message.
		if bytes.Contains(out, []byte("no yang/ directories with .yang files found")) {
			t.Fatalf("yang_glue --check found NO yang/ directories and exited 0, so it guarded nothing: discoverYangDirs no longer matches the tree layout.\n%s", out)
		}
		t.Fatalf("yang_glue --check did not report a directory count; its output shape changed, so this test can no longer prove it checked anything:\n%s", out)
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("unparsable yang directory count %q: %v", m[1], err)
	}
	if n < minYangDirs {
		t.Fatalf("yang_glue --check reported only %d yang/ directories (floor %d): discovery is broken and this check is guarding almost nothing\n%s", n, minYangDirs, out)
	}
}

// yangDirCountRE extracts N from "yang_glue: N yang/ directories are current".
var yangDirCountRE = regexp.MustCompile(`yang_glue: (\d+) yang/ directories are current`)
