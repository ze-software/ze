// VALIDATES: every vendored module a patch under Dir changes is its upstream
// release plus those patches, file for file, and nothing else, and every
// patch applies forward to that release, the direction the re-vendor uses.
// PREVENTS: a hand edit in a patched vendored module that no patch records,
// a patch that `go mod vendor` dropped or a re-vendor left half applied, and
// a patch that reverts out of the vendored tree and cannot be applied again.

package vendorpatch

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestPatchedVendorModulesMatchUpstream reverts every patch of a module in a
// copy of its vendored tree and requires the result to equal the module as
// the Go module proxy serves it at the vendored version. It then applies the
// patches, in set order, to a second tree written from the upstream files,
// and requires each one to apply: `./le setup install` applies them in that
// direction after `go mod vendor`, so a patch that only reverts cannot
// rebuild the tree.
//
// The set is the patches present in Dir, grouped by the module their review
// stamp names, so a new patch is checked with no edit here. A module carrying
// no patch is not judged: the go command already keeps it equal to upstream.
//
// Every vendored file is read with os.ReadFile while it is copied, which
// registers it with testlog, so an edit to any of them invalidates the cached
// result (see assertVendorPatchApplied for why that matters).
func TestPatchedVendorModulesMatchUpstream(t *testing.T) {
	root := repoRoot(t)
	paths, err := Patches(root)
	if err != nil {
		t.Fatal(err)
	}
	vendored, err := Vendored(root)
	if err != nil {
		t.Fatal(err)
	}
	byModule := map[string][]string{}
	for _, relative := range paths {
		if err := CheckStamp(root, relative, vendored); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		module, _, err := Stamp(body)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range patchTargets(string(body)) {
			if !strings.HasPrefix(target, "vendor/"+module+"/") {
				t.Fatalf("%s is stamped for %s but changes %s, outside vendor/%s; one patch changes one module", relative, module, target, module)
			}
		}
		byModule[module] = append(byModule[module], relative)
	}
	if len(byModule) == 0 {
		t.Fatalf("no patch under %s; the check would pass over nothing", Dir)
	}
	for module, patches := range byModule {
		t.Run(module, func(t *testing.T) {
			compareWithUpstream(t, root, module, vendored[module], patches, vendored)
		})
	}
}

// compareWithUpstream copies vendor/<module> (without the nested vendored
// modules under it), reverts patches in reverse order, and compares every
// remaining file with the upstream module directory. It then applies patches
// forward, in set order, to a second tree that holds those upstream files.
//
// The forward apply is not redundant with the revert. A patch that adds a
// file under `--- /dev/null` with no `new file mode` line reverts cleanly,
// and git refuses it forward with "dev/null: No such file or directory".
//
// THE SECOND TREE IS LOAD-BEARING. Git reads such a section as a change to a
// file named dev/null, so the revert leaves an empty dev/null in the tree it
// ran in, and a forward apply in that same tree finds it and passes. Measured
// 2026-11-13: applied to the reverted copy the malformed patch passed, and
// applied to a tree the revert never touched it was refused. That tree holds
// the files a fresh `go mod vendor` writes, so a refusal here is the refusal
// the re-vendor step would meet.
func compareWithUpstream(t *testing.T, root, module, version string, patches []string, vendored map[string]string) {
	t.Helper()
	upstream := upstreamDir(t, root, module, version)
	work := t.TempDir()
	source := filepath.Join(root, "vendor", filepath.FromSlash(module))
	copied := copyModuleTree(t, source, filepath.Join(work, "vendor", filepath.FromSlash(module)), module, vendored)

	for _, relative := range slices.Backward(patches) {
		gitApplyReverse(t, work, filepath.Join(root, filepath.FromSlash(relative)), relative)
	}

	pristine := t.TempDir()
	for _, rel := range copied {
		got, err := os.ReadFile(filepath.Join(work, "vendor", filepath.FromSlash(module), rel))
		if os.IsNotExist(err) {
			continue // a file the patch adds; reverting removed it
		}
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join(upstream, rel))
		if err != nil {
			t.Errorf("vendor/%s/%s is not in %s %s and no patch under %s adds it.\nRecovery: run `./le setup install` to re-vendor and re-apply the patches, or record the change as a patch.", module, rel, module, version, Dir)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("vendor/%s/%s differs from %s %s plus the patches under %s.\nRecovery: run `./le setup install` to re-vendor and re-apply the patches, or record the change as a patch.", module, rel, module, version, Dir)
		}
		writeUpstreamFile(t, filepath.Join(pristine, "vendor", filepath.FromSlash(module), rel), want)
	}
	if t.Failed() {
		return // the vendored tree is not upstream plus the patches, and that is the finding to fix first
	}

	for _, relative := range patches {
		gitApplyForward(t, pristine, filepath.Join(root, filepath.FromSlash(relative)), relative, module, version)
	}
}

// writeUpstreamFile writes one upstream file into the tree the forward apply
// runs in, creating its directory.
func writeUpstreamFile(t *testing.T, target string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("prepare the tree the patches are applied to: %v", err)
	}
	if err := os.WriteFile(target, content, 0o600); err != nil {
		t.Fatalf("prepare the tree the patches are applied to: %v", err)
	}
}

// copyModuleTree copies every regular file under source to target, skipping a
// directory that is itself another vendored module, and answers the copied
// paths relative to source.
func copyModuleTree(t *testing.T, source, target, module string, vendored map[string]string) []string {
	t.Helper()
	var copied []string
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if _, nested := vendored[module+"/"+filepath.ToSlash(rel)]; nested && rel != "." {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(target, rel), 0o750)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		copied = append(copied, filepath.ToSlash(rel))
		return os.WriteFile(filepath.Join(target, rel), body, 0o600)
	})
	if err != nil {
		t.Fatalf("copy vendor/%s: %v", module, err)
	}
	return copied
}

// upstreamDir answers the module cache directory of module@version, through
// `go mod download`, which reads the shared module cache and reaches the proxy
// only when the cache lacks it.
func upstreamDir(t *testing.T, root, module, version string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", module+"@"+version)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.Output()
	var answer struct {
		Dir   string
		Error string
	}
	if jsonErr := json.Unmarshal(out, &answer); jsonErr != nil {
		t.Fatalf("go mod download %s@%s: %v (%v)\n%s\nRecovery: run `go mod download %s@%s` with network access, then re-run this test.", module, version, err, jsonErr, out, module, version)
	}
	if answer.Error != "" {
		t.Fatalf("go mod download %s@%s: %s\nRecovery: run `go mod download %s@%s` with network access, then re-run this test.", module, version, answer.Error, module, version)
	}
	if err != nil {
		t.Fatalf("go mod download %s@%s: %v", module, version, err)
	}
	return answer.Dir
}

// gitApplyReverse reverts patch inside work. GIT_CEILING_DIRECTORIES stops git
// from finding a repository above work (TMPDIR can sit inside this checkout),
// so the patch paths resolve against work and not against a repository root.
func gitApplyReverse(t *testing.T, work, patch, relative string) {
	t.Helper()
	if out, err := gitApply(t, work, "--reverse", patch); err != nil {
		t.Fatalf("the vendored change is missing or differs from %s: %v\n%s\nRecovery: run `./le setup install`, which re-vendors and applies every patch under %s.", relative, err, out, Dir)
	}
}

// gitApplyForward applies patch inside work, which MUST hold the upstream
// files of module with every earlier patch of the set applied, and MUST NOT
// be a tree a revert ran in (see compareWithUpstream). It is the `git apply`
// the re-vendor step runs (internal/le/setup applyVendorPatches).
func gitApplyForward(t *testing.T, work, patch, relative, module, version string) {
	t.Helper()
	if out, err := gitApply(t, work, patch); err != nil {
		t.Fatalf("%s reverts out of vendor/%s and does not apply to %s %s: %v\n%s\n"+
			"`./le setup install` applies every patch forward after `go mod vendor`, so it cannot rebuild vendor/%s from this patch.\n"+
			"A file the patch adds needs a `new file mode 100644` line between its `diff --git` line and its `--- /dev/null` line.\n"+
			"Recovery: write the patch again with `git diff --no-index`, as docs/contributing/patching-the-standard-library.md, section \"Patching a vendored module\", describes, then run:\n./le test unit pkg path ./%s/",
			relative, module, module, version, err, out, module, filepath.ToSlash(filepath.Dir(Dir)))
	}
}

// gitApply runs `git apply` with arguments inside work and answers its
// output. GIT_CEILING_DIRECTORIES is set as gitApplyReverse describes.
func gitApply(t *testing.T, work string, arguments ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"apply"}, arguments...)...)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(work))
	return cmd.CombinedOutput()
}
