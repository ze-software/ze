// VALIDATES: every vendored module a patch under Dir changes is its upstream
// release plus those patches, file for file, and nothing else.
// PREVENTS: a hand edit in a patched vendored module that no patch records,
// and a patch that `go mod vendor` dropped or a re-vendor left half applied.

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
// the Go module proxy serves it at the vendored version.
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
// remaining file with the upstream module directory.
func compareWithUpstream(t *testing.T, root, module, version string, patches []string, vendored map[string]string) {
	t.Helper()
	upstream := upstreamDir(t, root, module, version)
	work := t.TempDir()
	source := filepath.Join(root, "vendor", filepath.FromSlash(module))
	copied := copyModuleTree(t, source, filepath.Join(work, "vendor", filepath.FromSlash(module)), module, vendored)

	for _, relative := range slices.Backward(patches) {
		gitApplyReverse(t, work, filepath.Join(root, filepath.FromSlash(relative)), relative)
	}

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
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "apply", "--reverse", patch)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(work))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the vendored change is missing or differs from %s: %v\n%s\nRecovery: run `./le setup install`, which re-vendors and applies every patch under %s.", relative, err, out, Dir)
	}
}
