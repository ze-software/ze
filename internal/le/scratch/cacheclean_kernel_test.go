package scratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance"
)

// VALIDATES: AC-14 of spec-appliance-ships-ze-kernel. No cache-clean or
// store-trim target is a built-kernel cache namespace, sits inside one, or
// contains one, so `./le scratch cache-clean` and disk-full recovery never
// delete a kernel that takes about thirty minutes to rebuild.
// Method: resolve the per-user target the way CleanCaches and trimStores do,
// with XDG_CACHE_HOME pointing both sides at one temporary root, then compare
// every target from cleanTargets (the list both commands walk) with
// appliance.KernelCacheNamespaces.
func TestCleanTargetsNeverReachTheKernelCache(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	root := t.TempDir()
	perUser, err := New(root, os.Environ()).cacheTarget()
	if err != nil {
		t.Fatal(err)
	}
	namespaces := appliance.KernelCacheNamespaces()
	if len(namespaces) == 0 {
		t.Fatal("appliance names no kernel cache namespace, so this test compares nothing")
	}
	for _, namespace := range namespaces {
		if !strings.HasPrefix(namespace, cacheHome) {
			t.Fatalf("kernel cache %s is not under XDG_CACHE_HOME %s, so this test compares two different roots", namespace, cacheHome)
		}
	}
	for _, target := range cleanTargets(root, filepath.Join(cacheHome, "go-build"), perUser) {
		for _, namespace := range namespaces {
			if pathWithin(target.path, namespace) {
				t.Errorf("%s target %s is inside kernel cache %s", target.name, target.path, namespace)
			}
			if pathWithin(namespace, target.path) {
				t.Errorf("%s target %s contains kernel cache %s", target.name, target.path, namespace)
			}
		}
	}
}

// pathWithin reports whether path is dir or below it.
func pathWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
