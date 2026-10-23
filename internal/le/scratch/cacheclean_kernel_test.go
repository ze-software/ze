package scratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
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

// TestCleanTargetsSkipATargetHoldingTheKernelCache verifies a target that
// resolves into, or around, a kernel cache namespace is skipped, naming the
// kernel cache, whatever path the machine default or the environment gives it.
//
// VALIDATES: review round 1 NOTE 1 of spec-appliance-ships-ze-kernel: the
// namespace declaration is a guard cleanTargets consults, not a list only a
// test reads.
// PREVENTS: a GOCACHE pointed at the per-user cache root emptying every built
// kernel.
func TestCleanTargetsSkipATargetHoldingTheKernelCache(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	namespaces := appliance.KernelCacheNamespaces()
	if len(namespaces) == 0 {
		t.Fatal("appliance names no kernel cache namespace")
	}
	ambient := filepath.Dir(namespaces[0])
	for _, target := range cleanTargets(t.TempDir(), ambient, filepath.Join(t.TempDir(), "per-user")) {
		if target.path != ambient {
			continue
		}
		if !strings.Contains(target.skipped, namespaces[0]) {
			t.Errorf("%s target %s holds the kernel cache and is not skipped (skipped=%q)", target.name, target.path, target.skipped)
		}
		return
	}
	t.Fatalf("no target has the ambient path %s", ambient)
}

// TestTrimSkipsATargetHoldingTheKernelCache verifies the store trim honors the
// kernel-cache skip cleanTargets records, so a trimmed store that holds a built
// kernel is left whole, however far over budget it is.
//
// VALIDATES: review round 1 NOTE 1 of spec-appliance-ships-ze-kernel, for the
// trim: cleanTargets is the trim's list as well as cache-clean's.
// PREVENTS: the hourly trim deleting a kernel cache that a checkout cache path
// happens to hold, which cache-clean would have skipped.
// Method: point XDG_CACHE_HOME at the checkout Go cache, so the kernel cache
// namespaces sit inside a target the trim bounds, seed one entry older than
// the floor, and trim against a budget of zero.
func TestTrimSkipsATargetHoldingTheKernelCache(t *testing.T) {
	_ = seededUnit(t)
	root := t.TempDir()
	checkout := gotoolchain.GoCache(root)
	t.Setenv("XDG_CACHE_HOME", checkout)
	namespaces := appliance.KernelCacheNamespaces()
	if len(namespaces) == 0 || !pathWithin(namespaces[0], checkout) {
		t.Fatalf("kernel cache %v is not inside the checkout cache %s, so this test trims nothing that holds one", namespaces, checkout)
	}
	targets := cleanTargets(root, "", t.TempDir())
	old := seedEntry(t, checkout, "a0old-a", 5*time.Hour)

	_, rows := trimCaches(t.Context(), targets, nil, budgetsOf(0, 1<<40, 1<<40), testPass(cacheEntryAgeFloor))
	if !exists(t, old) {
		t.Error("the trim removed an entry from a store that holds the kernel cache")
	}
	if row := rowNamed(t, rows, checkoutCache); !strings.Contains(row.Skipped, namespaces[0]) || row.EntriesRemoved != 0 {
		t.Errorf("checkout row = %+v, want a skip naming the kernel cache %s", row, namespaces[0])
	}
}
