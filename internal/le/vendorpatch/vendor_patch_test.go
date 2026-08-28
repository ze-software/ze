package vendorpatch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// assertVendorPatchApplied fails unless every hunk of patchPath is present in
// the working tree. recovery is the command a reader runs to put it back.
//
// It is the one producer for every vendor-patch guard, so no two of them can
// disagree about what "applied" means.
//
// THE READS BELOW ARE LOAD-BEARING, and they are not for parsing.
//
// `go test` caches a passing result and reuses it until one of the test's
// INPUTS changes. It learns those inputs from testlog, which the test binary
// writes when IT opens a file: os.ReadFile, os.Open, os.Stat. A file read by a
// CHILD PROCESS is invisible to that mechanism. The check that matters here
// runs `git apply`, so git opens the vendored file and the test binary never
// does -- and the cache concluded nothing had changed when the patch was
// reverted underneath it. The guard reported PASS on an unpatched tree, which
// is the exact failure it exists to prevent, and it is worse than no guard
// because it is green.
//
// Reading the patch and every file it touches, through os.ReadFile, puts them
// in testlog. Reverting any of them now invalidates the entry and the check
// re-runs. Measured 2026-08-25: with the reads, a reverted patch fails on the
// second run as well as the first; without them, the second run passes from
// cache.
func assertVendorPatchApplied(t *testing.T, patchPath, recovery string) {
	t.Helper()
	root := repoRoot(t)
	patch := filepath.Join(root, filepath.FromSlash(patchPath))

	body, err := os.ReadFile(patch)
	if err != nil {
		t.Fatalf("cannot read the patch at %s: %v", patchPath, err)
	}
	targets := patchTargets(string(body))
	if len(targets) == 0 {
		t.Fatalf("%s names no target file; the guard would pass over an empty patch", patchPath)
	}
	for _, rel := range targets {
		// The content is not inspected. The READ is the point: it registers
		// the file with testlog so the cache tracks it.
		if _, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s patches %s, which cannot be read: %v", patchPath, rel, err)
		}
	}

	// t.Context() ends the child with the test; it cannot bound this call, since
	// it is canceled only just before the Cleanup functions run. The timeout is
	// what kills a git that never returns.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "apply", "--reverse", "--check", patch)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the vendored change is missing or differs from %s: %v\n%s\nRecovery: run `%s` from the repository root after `go mod vendor`.",
			patchPath, err, out, recovery)
	}
}

// patchTargets returns the repository-relative paths a unified diff writes to,
// read from its `+++ b/<path>` lines. /dev/null is skipped: a patch that only
// deletes a file has no target to read.
func patchTargets(body string) []string {
	var out []string
	for line := range strings.SplitSeq(body, "\n") {
		rest, ok := strings.CutPrefix(line, "+++ ")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if i := strings.IndexAny(rest, "\t"); i >= 0 {
			rest = rest[:i]
		}
		if rest == "/dev/null" {
			continue
		}
		out = append(out, strings.TrimPrefix(rest, "b/"))
	}
	return out
}
