package vendorpatch

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// vendorPatchRecovery is the recovery a patch under Dir carries when it is not
// applied. It matches the upstream check in upstream_test.go: the re-vendor
// applies every patch, so no single `git apply` line is owed.
const vendorPatchRecovery = "run `./le setup install` from the repository root, which re-vendors and applies every patch under " + Dir + "."

// TestEveryVendorPatchIsApplied requires every patch under Dir to be applied
// in the vendored tree, in one subtest per patch file, named after the file.
//
// The set is the files Patches answers, so a patch added to the directory is
// guarded with no test edited, and a patch that is not applied fails its own
// subtest while the others still report. Patches lists the directory with
// os.ReadDir, which registers it with testlog, so adding or removing a patch
// invalidates a cached result. An empty set fails: the guard would pass over
// nothing. A patch file that is gone is simply not listed, so this guard
// cannot notice it; the named guards (TestNetlinkXFRMPatchApplied and the
// other *_patch_test.go files) pin that their patches are present.
func TestEveryVendorPatchIsApplied(t *testing.T) {
	root := repoRoot(t)
	paths, err := Patches(root)
	if err != nil {
		t.Fatalf("%v\nRecovery: run the test in a full checkout, whose %s holds the vendor patches.", err, Dir)
	}
	if len(paths) == 0 {
		t.Fatalf("no patch under %s, so the guard would pass over nothing.\nRecovery: run `git status -- %s` to find the removed patches, restore them, then %s",
			Dir, Dir, vendorPatchRecovery)
	}
	for _, relative := range paths {
		t.Run(path.Base(relative), func(t *testing.T) {
			assertVendorPatchApplied(t, relative, vendorPatchRecovery)
		})
	}
}

// assertVendorPatchApplied fails unless every hunk of patchPath is present in
// the working tree. recovery is the sentence, printed after "Recovery: ", that
// tells a reader how to put the patch back.
//
// It is the one producer for every vendor-patch guard, the derived
// TestEveryVendorPatchIsApplied and the named guards, so no two of them can
// disagree about what "applied" means.
//
// A patch under Dir is judged at its place in its module's stack: the patches
// under Dir stamped for the same module, in Patches order, the order the
// re-vendor applies them. A later patch of the stack can change or sit beside
// lines an earlier one added, so the earlier patch does not reverse out of the
// final vendored tree. It is applied when it reverses once every later patch of
// its stack has been reversed. That runs in a copy of the files the stack
// targets, in a t.TempDir(), so the vendored tree is never written. A patch
// with no later patch, a patch outside Dir and a patch with no stamp are
// checked with `git apply --reverse --check` on the checkout itself, which is
// the same judgement with nothing to reverse first.
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
// cache. The stacked check reads every later patch of the stack and every file
// it targets the same way, so editing a later patch invalidates the entry too.
func assertVendorPatchApplied(t *testing.T, patchPath, recovery string) {
	t.Helper()
	root := repoRoot(t)

	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(patchPath)))
	if err != nil {
		t.Fatalf("cannot read the patch at %s: %v\nRecovery: %s", patchPath, err, recovery)
	}
	targets := patchTargets(string(body))
	if len(targets) == 0 {
		t.Fatalf("%s names no target file, so the guard would pass over an empty patch.\nRecovery: write the patch again with its `+++ b/<path>` lines, as docs/contributing/patching-the-standard-library.md, section \"Patching a vendored module\", describes.", patchPath)
	}
	if !strings.HasPrefix(patchPath, Dir+"/") {
		assertPatchAppliedStandalone(t, root, patchPath, targets, recovery)
		return
	}
	module, _, err := Stamp(body)
	if err != nil {
		// TestPatchedVendorModulesMatchUpstream refuses a patch under Dir with
		// no stamp; with no module there is no stack to place it in.
		assertPatchAppliedStandalone(t, root, patchPath, targets, recovery)
		return
	}
	later := laterInStack(t, root, patchPath, module, recovery)
	if len(later) == 0 {
		assertPatchAppliedStandalone(t, root, patchPath, targets, recovery)
		return
	}
	assertPatchAppliedInStack(t, root, patchPath, module, targets, later, recovery)
}

// stackedPatch is a patch that sorts after the one being judged, in the same
// module's stack, with the files it writes.
type stackedPatch struct {
	path    string
	targets []string
}

// laterInStack answers the patches under Dir stamped for module that sort
// after patchPath, in Patches order. Each one's body is read with os.ReadFile,
// which registers it with testlog.
func laterInStack(t *testing.T, root, patchPath, module, recovery string) []stackedPatch {
	t.Helper()
	paths, err := Patches(root)
	if err != nil {
		t.Fatalf("%v\nRecovery: %s", err, recovery)
	}
	var later []stackedPatch
	for _, relative := range paths {
		if relative <= patchPath {
			continue // Patches is sorted: this one is applied before patchPath
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("%s is not judged: %s, listed after it, cannot be read: %v\nRecovery: %s", patchPath, relative, err, recovery)
		}
		other, _, err := Stamp(body)
		if err != nil {
			continue // unstamped, so in no stack; TestPatchedVendorModulesMatchUpstream refuses it
		}
		if other != module {
			continue
		}
		later = append(later, stackedPatch{path: relative, targets: patchTargets(string(body))})
	}
	return later
}

// assertPatchAppliedStandalone requires patchPath to reverse out of the
// checkout as it stands.
func assertPatchAppliedStandalone(t *testing.T, root, patchPath string, targets []string, recovery string) {
	t.Helper()
	for _, rel := range targets {
		// The content is not inspected. The READ is the point: it registers
		// the file with testlog so the cache tracks it. A file the patch adds
		// exists only once the patch is applied, so its absence fails here.
		if _, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s patches %s, which cannot be read: %v. The vendored tree lacks the change the patch carries.\nRecovery: %s",
				patchPath, rel, err, recovery)
		}
	}
	if out, err := gitApply(t, root, "--reverse", "--check", filepath.Join(root, filepath.FromSlash(patchPath))); err != nil {
		t.Fatalf("the vendored change is missing or differs from %s: %v\n%s\nThe build compiles the vendored code without the change the patch carries.\nRecovery: %s",
			patchPath, err, out, recovery)
	}
}

// assertPatchAppliedInStack copies the files patchPath and the later patches
// of its stack target into a t.TempDir(), reverses the later patches there in
// reverse order, and then requires patchPath to reverse. A file a patch adds is
// in the copy as the final tree holds it, so reversing the later patches first
// restores the content the earlier patch wrote.
func assertPatchAppliedInStack(t *testing.T, root, patchPath, module string, targets []string, later []stackedPatch, recovery string) {
	t.Helper()
	work := t.TempDir()
	if rel, err := copyPatchTargets(t, root, work, targets); err != nil {
		t.Fatalf("%s patches %s, which cannot be read: %v. The vendored tree lacks the change the patch carries.\nRecovery: %s",
			patchPath, rel, err, recovery)
	}
	for _, next := range later {
		if rel, err := copyPatchTargets(t, root, work, next.targets); err != nil {
			t.Fatalf("%s is not judged: %s, stacked after it on %s, patches %s, which cannot be read: %v. Look at %s first.\nRecovery: %s",
				patchPath, next.path, module, rel, err, next.path, recovery)
		}
	}

	var reversed []string
	for _, next := range slices.Backward(later) {
		if out, err := gitApply(t, work, "--reverse", filepath.Join(root, filepath.FromSlash(next.path))); err != nil {
			t.Fatalf("%s is not judged: %s, stacked after it on %s, does not reverse out of the vendored tree%s: %v\n%s\nLook at %s first: every later patch of the stack must reverse before %s can be checked.\nRecovery: %s",
				patchPath, next.path, module, afterReversing(reversed), err, out, next.path, patchPath, recovery)
		}
		reversed = append(reversed, path.Base(next.path))
	}
	if out, err := gitApply(t, work, "--reverse", "--check", filepath.Join(root, filepath.FromSlash(patchPath))); err != nil {
		t.Fatalf("the vendored change is missing or differs from %s at its place in the %s stack%s: %v\n%s\nThe build compiles the vendored code without the change the patch carries.\nRecovery: %s",
			patchPath, module, afterReversing(reversed), err, out, recovery)
	}
}

// copyPatchTargets copies every file in targets from the checkout into work at
// the same relative path. The os.ReadFile is the testlog registration
// assertVendorPatchApplied describes. It answers the target it could not read.
func copyPatchTargets(t *testing.T, root, work string, targets []string) (string, error) {
	t.Helper()
	for _, rel := range targets {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return rel, err
		}
		writeUpstreamFile(t, filepath.Join(work, filepath.FromSlash(rel)), content)
	}
	return "", nil
}

// afterReversing names, for a failure message, the later patches already
// reversed in the copy, in the order they were reversed.
func afterReversing(reversed []string) string {
	if len(reversed) == 0 {
		return ""
	}
	return ", after reversing " + strings.Join(reversed, ", ")
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
