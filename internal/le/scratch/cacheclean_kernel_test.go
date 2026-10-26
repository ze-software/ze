package scratch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// VALIDATES: AC-14 of spec-appliance-ships-ze-kernel. No scratch clean or
// store-trim target is a built-kernel cache namespace, sits inside one, or
// contains one, so `./le scratch clean` and disk-full recovery never
// delete a kernel that takes about thirty minutes to rebuild.
// Method: lay the checkout out as every le run leaves it, cache/ a link to the
// per-user target with XDG_CACHE_HOME pointing both sides at one temporary
// root, plus one stale cache copy, then compare every target from cleanTargets
// (the list both commands walk), as spelled and where it lands through the
// link, with appliance.KernelCacheNamespaces.
func TestCleanTargetsNeverReachTheKernelCache(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	root := t.TempDir()
	perUser, err := New(root, os.Environ()).cacheTarget()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(perUser, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(perUser, filepath.Join(root, cacheName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, cacheName+staleCacheInfix+"20261024T000000Z-1"), 0o750); err != nil {
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
	targets := cleanTargets(root, filepath.Join(cacheHome, "go-build"))
	if !slices.ContainsFunc(targets, func(target cleanTarget) bool { return target.kind == staleCopy }) {
		t.Fatal("cleanTargets answered no stale copy row, so this test compares one row fewer than the trim walks")
	}
	for _, target := range targets {
		for _, path := range []string{target.path, gen.Landing(root, target.path)} {
			for _, namespace := range namespaces {
				if pathWithin(path, namespace) {
					t.Errorf("%s target %s is inside kernel cache %s", target.name, path, namespace)
				}
				if pathWithin(namespace, path) {
					t.Errorf("%s target %s contains kernel cache %s", target.name, path, namespace)
				}
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
	for _, target := range cleanTargets(t.TempDir(), ambient) {
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
// trim: cleanTargets is the trim's list as well as scratch clean's.
// PREVENTS: the hourly trim deleting a kernel cache that a checkout cache path
// happens to hold, which scratch clean would have skipped.
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
	targets := cleanTargets(root, "")
	old := seedEntry(t, checkout, "a0old-a", 5*time.Hour)

	_, rows := trimCaches(t.Context(), targets, budgetsOf(0, 1<<40, 1<<40), testPass(cacheEntryAgeFloor))
	if !exists(t, old) {
		t.Error("the trim removed an entry from a store that holds the kernel cache")
	}
	if row := rowNamed(t, rows, checkoutCache); !strings.Contains(row.Skipped, namespaces[0]) || row.EntriesRemoved != 0 {
		t.Errorf("checkout row = %+v, want a skip naming the kernel cache %s", row, namespaces[0])
	}
}
