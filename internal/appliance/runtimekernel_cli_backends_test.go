// Design: docs/architecture/appliance/build-artifacts.md -- the runtime kernel cache serves either backend's build of the same inputs
// Related: cmd_kernel.go -- runKernel, servedKernelCachePath, servableKernelCachePaths
// Related: cache.go -- evictKeepN

package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runtimeKernelCLIEntries prepares a runtime kernel source root as the cwd and
// an empty cache, and answers the cache entries a runtime lookup for arch with
// no --builder tries, the preferred backend's first.
func runtimeKernelCLIEntries(t *testing.T, arch string) []string {
	t.Helper()
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	td, err := kernelTargetFor(kernelTargetRuntime)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := runtimeKernelCacheEntries(root, defaultKernelVersion, arch, runtimeKernelProfile, "", td)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0] == entries[1] {
		t.Fatalf("a lookup with no --builder tries %v, want two distinct entries", entries)
	}
	return entries
}

// runKernelCaptured runs `ze appliance kernel` with args and answers its exit
// code and what it wrote to stdout and stderr. It points os.Stdout at a temp
// file around the call and reads stderr through captureStderr.
func runKernelCaptured(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout.txt")
	out, err := os.Create(path) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = out
	stderr = captureStderr(t, func() { code = runKernel(args) })
	os.Stdout = orig
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	return code, string(data), stderr
}

// TestKernelPrintCacheDirNamesTheServedEntry proves `ze appliance kernel
// --target runtime --print-cache-dir` with no --builder prints the entry a
// resolution is served from, so a caller that reads or removes it acts on the
// kernel a run boots. Method: the artifact in both entries, only the preferred
// backend's, only the other backend's (a Mac where only the Docker build or only
// the QEMU build exists), and in neither, where a build would publish the
// preferred entry.
func TestKernelPrintCacheDirNamesTheServedEntry(t *testing.T) {
	cases := []struct {
		name      string
		built     [2]bool
		wantIndex int
	}{
		{"both backends built", [2]bool{true, true}, 0},
		{"only the preferred backend built", [2]bool{true, false}, 0},
		{"only the other backend built", [2]bool{false, true}, 1},
		{"neither built", [2]bool{false, false}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries := runtimeKernelCLIEntries(t, archARM64)
			for index, built := range tc.built {
				if !built {
					continue
				}
				if err := os.MkdirAll(entries[index], 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(entries[index], runtimeKernelArtifact), []byte("kernel"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			code, stdout, stderr := runKernelCaptured(t, "--target", "runtime", "--arch", archARM64, "--print-cache-dir")
			if code != exitOK {
				t.Fatalf("exit %d, stderr:\n%s", code, stderr)
			}
			if got := strings.TrimSpace(stdout); got != entries[tc.wantIndex] {
				t.Fatalf("printed %s, want %s", got, entries[tc.wantIndex])
			}
		})
	}
}

// TestKernelEvictCacheKeepsEveryServableEntry proves `ze appliance kernel
// --target runtime --evict-cache` with no --builder keeps both backends' entries
// of the current inputs, either of which a resolution may be served, however
// old, while it still evicts an old entry of other inputs. Method: both entries
// the oldest of their arch, two newer entries filling the keep count, one older
// unrelated entry, all past the grace window, then one eviction.
func TestKernelEvictCacheKeepsEveryServableEntry(t *testing.T) {
	entries := runtimeKernelCLIEntries(t, archARM64)
	ns := filepath.Join(resolveCacheDir(), runtimeKernelCacheDir)
	for _, entry := range entries {
		if filepath.Dir(entry) != ns {
			t.Fatalf("entry %s is not in the runtime kernel cache namespace %s", entry, ns)
		}
	}
	now := time.Now()
	mk := func(dir string, ageMin int) {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		at := now.Add(-time.Duration(ageMin) * time.Minute)
		if err := os.Chtimes(dir, at, at); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(ns, defaultKernelVersion+"-runtime-"+archARM64+"-runtime-old0-bbbb")
	newer := []string{
		filepath.Join(ns, defaultKernelVersion+"-runtime-"+archARM64+"-runtime-new1-bbbb"),
		filepath.Join(ns, defaultKernelVersion+"-runtime-"+archARM64+"-runtime-new2-bbbb"),
	}
	mk(unrelated, 500)
	mk(entries[0], 400)
	mk(entries[1], 300)
	mk(newer[0], 200)
	mk(newer[1], 100)

	origNow, origGrace := evictNow, evictGrace
	t.Cleanup(func() { evictNow, evictGrace = origNow, origGrace })
	evictNow = func() time.Time { return now }
	evictGrace = 10 * time.Minute

	code, _, stderr := runKernelCaptured(t, "--target", "runtime", "--arch", archARM64, "--evict-cache")
	if code != exitOK {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	want := map[string]bool{
		unrelated:  false, // old, of other inputs: evicted
		entries[0]: true,  // the preferred backend's entry
		entries[1]: true,  // the other backend's entry, which a resolution also serves
		newer[0]:   true,
		newer[1]:   true,
	}
	for dir, kept := range want {
		_, err := os.Stat(dir)
		if kept != (err == nil) {
			t.Errorf("%s: kept=%v, want %v", filepath.Base(dir), err == nil, kept)
		}
	}
}
