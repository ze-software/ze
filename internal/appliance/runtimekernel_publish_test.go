// Design: docs/architecture/appliance/build-artifacts.md -- one build per key, published by one rename
// Related: runtimekernel_publish.go -- publishRuntimeKernel, the lock and the staging reclaim
// Related: runtimekernel_cached.go -- CachedRuntimeKernelTree, the no-build reader

package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// runtimeKernelTestBuild is the command a test's refusal must end with.
func runtimeKernelTestBuild(arch string) string { return "./le setup docker host build arch " + arch }

// writeValidRuntimeKernel writes a tree that passes validateRuntimeKernelTree
// under runtimeKernelOwnershipFixture's requirements, provenance included.
func writeValidRuntimeKernel(t *testing.T, dir string) {
	t.Helper()
	writeRuntimeKernelTree(t, dir, runtime.GOARCH)
	config := fakeRuntimeConfig() + "CONFIG_RUNTIME_OWNERSHIP_FIXTURE=y\n"
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

// assertNoRuntimeKernelEntry fails when anything exists at the entry path.
func assertNoRuntimeKernelEntry(t *testing.T, cached, when string) {
	t.Helper()
	if _, err := os.Lstat(cached); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s: the cache entry %s exists (%v), want no entry", when, cached, err)
	}
}

// stagingDirs answers the staging directories beside the entry.
func stagingDirs(t *testing.T, cached string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(cached), "."+filepath.Base(cached)+runtimeKernelStagingInfix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

// TestCachedRuntimeKernelTreeRefusesColdCache checks that the lookup a test, a
// record run or a lab uses never builds: on a cold cache it refuses, naming the
// entry's path and the build command on the last line, and creates nothing;
// once the entry exists it answers that entry under the key RuntimeKernelTree
// uses. Method: the builder seam fails the test if it is ever reached.
func TestCachedRuntimeKernelTreeRefusesColdCache(t *testing.T) {
	root, cached, _ := runtimeKernelOwnershipFixture(t)
	setTestKernelBuild(t, func(kernelBuildSpec) error {
		t.Fatal("the no-build lookup reached the kernel builder")
		return nil
	})

	_, err := CachedRuntimeKernelTree(root, runtime.GOARCH, runtimeKernelTestBuild)
	if err == nil {
		t.Fatal("a cold cache was answered, want a refusal")
	}
	message := err.Error()
	if !strings.Contains(message, cached) {
		t.Errorf("refusal does not name the entry path %s:\n%s", cached, message)
	}
	lines := strings.Split(message, "\n")
	if want := "fix: " + runtimeKernelTestBuild(runtime.GOARCH); lines[len(lines)-1] != want {
		t.Errorf("refusal's last line = %q, want %q", lines[len(lines)-1], want)
	}
	assertNoRuntimeKernelEntry(t, cached, "after the refusal")
	if dirs := stagingDirs(t, cached); len(dirs) != 0 {
		t.Errorf("the no-build lookup created staging directories %q", dirs)
	}

	writeValidRuntimeKernel(t, cached)
	tree, err := CachedRuntimeKernelTree(root, runtime.GOARCH, runtimeKernelTestBuild)
	if err != nil {
		t.Fatalf("a warm cache was refused: %v", err)
	}
	if tree != cached {
		t.Errorf("warm lookup = %q, want the keyed entry %q", tree, cached)
	}
}

// TestRuntimeKernelPublishIsAtomic checks that no entry exists while a build
// runs, that a failed build leaves no entry, and that the next build of the key
// reclaims the staging directory the failure left and publishes a whole tree.
// Method: the fake builder looks at the entry path from inside the build.
func TestRuntimeKernelPublishIsAtomic(t *testing.T) {
	root, cached, _ := runtimeKernelOwnershipFixture(t)
	var failed string
	setTestKernelBuild(t, func(spec kernelBuildSpec) error {
		writeValidRuntimeKernel(t, spec.outDir)
		// A build that dies before the builder's last step has no provenance.
		if err := os.Remove(filepath.Join(spec.outDir, kernelbuilder.ProvenanceName)); err != nil {
			t.Fatal(err)
		}
		assertNoRuntimeKernelEntry(t, cached, "during the build")
		failed = spec.outDir
		return errors.New("fixture build killed before it finished")
	})
	if _, err := RuntimeKernelTree(root, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), failed) {
		t.Fatalf("a failed build was not refused naming its staging directory %q: %v", failed, err)
	}
	assertNoRuntimeKernelEntry(t, cached, "after a failed build")

	calls := 0
	setTestKernelBuild(t, func(spec kernelBuildSpec) error {
		calls++
		if _, err := os.Stat(failed); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the failed build's staging %s was not reclaimed before the next build: %v", failed, err)
		}
		writeValidRuntimeKernel(t, spec.outDir)
		assertNoRuntimeKernelEntry(t, cached, "during the second build")
		return nil
	})
	tree, err := RuntimeKernelTree(root, runtime.GOARCH)
	if err != nil {
		t.Fatalf("the second build was refused: %v", err)
	}
	if calls != 1 || tree != cached {
		t.Fatalf("second build: calls=%d tree=%q, want one build published at %q", calls, tree, cached)
	}
	for _, name := range []string{"vmlinuz", "config", kernelbuilder.ProvenanceName} {
		if _, err := os.Stat(filepath.Join(cached, name)); err != nil {
			t.Errorf("the published entry lacks %s: %v", name, err)
		}
	}
	if dirs := stagingDirs(t, cached); len(dirs) != 0 {
		t.Errorf("staging directories remain after publication: %q", dirs)
	}
}

// TestRuntimeKernelPublishAdoptsAFinishedBuild checks that a build which
// finished, but whose run was killed before the rename, is published by the next
// build of the key instead of being built again. Method: a staging directory
// holding a whole tree with its provenance, and a builder that fails the test.
func TestRuntimeKernelPublishAdoptsAFinishedBuild(t *testing.T) {
	root, cached, _ := runtimeKernelOwnershipFixture(t)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	finished := filepath.Join(filepath.Dir(cached), "."+filepath.Base(cached)+runtimeKernelStagingInfix+"killed")
	writeValidRuntimeKernel(t, finished)
	setTestKernelBuild(t, func(kernelBuildSpec) error {
		t.Error("a finished build in staging was built again")
		return nil
	})
	tree, err := RuntimeKernelTree(root, runtime.GOARCH)
	if err != nil || tree != cached {
		t.Fatalf("finished staging was not published: tree=%q err=%v", tree, err)
	}
	if _, err := os.Stat(filepath.Join(cached, "vmlinuz")); err != nil {
		t.Errorf("the adopted entry lacks its kernel: %v", err)
	}
}

// TestRuntimeKernelConcurrentResolvesBuildOnce checks that two runs resolving
// one cold key build it once: the second waits on the key's lock and then
// reuses the first's entry. Method: the first build blocks until the second
// reports that it waits, so the overlap is certain rather than timed.
func TestRuntimeKernelConcurrentResolvesBuildOnce(t *testing.T) {
	root, cached, _ := runtimeKernelOwnershipFixture(t)
	started := make(chan struct{})
	release := make(chan struct{})
	waiting := make(chan struct{})
	var calls atomic.Int32
	setTestKernelBuild(t, func(spec kernelBuildSpec) error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		writeValidRuntimeKernel(t, spec.outDir)
		return nil
	})
	oldNotice := runtimeKernelWaitNotice
	runtimeKernelWaitNotice = func(string, string) { close(waiting) }
	t.Cleanup(func() { runtimeKernelWaitNotice = oldNotice })

	trees := make([]string, 2)
	errs := make([]error, 2)
	var group sync.WaitGroup
	resolve := func(index int) {
		defer group.Done()
		trees[index], errs[index] = RuntimeKernelTree(root, runtime.GOARCH)
	}
	group.Add(2)
	go resolve(0)
	<-started
	go resolve(1)
	<-waiting
	close(release)
	group.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("two concurrent resolves built the key %d times, want once", got)
	}
	for index := range trees {
		if errs[index] != nil || trees[index] != cached {
			t.Errorf("resolve %d: tree=%q err=%v, want %q", index, trees[index], errs[index], cached)
		}
	}
}
