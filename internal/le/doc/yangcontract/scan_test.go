// Design: docs/contributing/documentation-testing.md -- source counts exclude runtime artifacts

package docyangcontract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSourceCountsIncludeUncommittedFiles counts a tree with no Git metadata,
// including a source directory whose name contains vendor rather than being it.
func TestSourceCountsIncludeUncommittedFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vendor-checkout")
	for _, path := range []string{
		"root_test.go",
		"internal/vendorcheck/check_test.go",
		"pkg/sdk/sdk_test.go",
		"cmd/probe/main_test.go",
		"test/probe/probe_test.go",
		"tools/probe/probe_test.go",
	} {
		writeDoc(t, root, path, "package probe\nfunc FuzzProbe() {}\nfunc TestProbe() {}\n")
	}
	c := checker{}
	if got := c.countFuzzTargets(root); got != 6 {
		t.Errorf("fuzz targets = %d, want 6", got)
	}
	if got := c.countGoTestFunctions(root); got != 3 {
		t.Errorf("product test functions = %d, want 3", got)
	}
	if len(c.unreadable) != 0 {
		t.Fatalf("readable source produced findings: %+v", c.unreadable)
	}
}

// TestSourceCountsExcludeArtifacts places countable and unopenable files in
// excluded trees. Dangling links fail deterministically even when run as root.
func TestSourceCountsExcludeArtifacts(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/probe/real_test.go", "package probe\nfunc FuzzReal() {}\nfunc TestReal() {}\n")
	for _, dir := range []string{
		"tmp/evidence/run/ze/crash",
		"vendor/dependency",
		"gokrazy/modcache/module",
		".cache/build",
		"node_modules/dependency",
		"internal/probe/testdata",
		"pkg/tmp/generated",
		"cmd/.cache/build",
		"internal/vendor/dependency",
	} {
		writeDoc(t, root, filepath.Join(dir, "copy_test.go"), "package probe\nfunc FuzzCopy() {}\nfunc TestCopy() {}\n")
		link := filepath.Join(root, dir, "unreadable_test.go")
		if err := os.Symlink("absent", link); err != nil {
			t.Fatalf("create artifact link: %v", err)
		}
	}
	c := checker{}
	if got := c.countFuzzTargets(root); got != 1 {
		t.Errorf("fuzz targets = %d, want 1", got)
	}
	if got := c.countGoTestFunctions(root); got != 1 {
		t.Errorf("product test functions = %d, want 1", got)
	}
	if len(c.unreadable) != 0 {
		t.Fatalf("artifact trees were read: %+v", c.unreadable)
	}
}

// TestSourceCountsRetainReadFailures puts the same unopenable source in each
// product area and checks that both counters name it while counting its sibling.
func TestSourceCountsRetainReadFailures(t *testing.T) {
	for _, area := range []string{"internal", "pkg", "cmd"} {
		t.Run(area, func(t *testing.T) {
			root := t.TempDir()
			writeDoc(t, root, filepath.Join(area, "real_test.go"), "package probe\nfunc FuzzReal() {}\nfunc TestReal() {}\n")
			link := filepath.Join(root, area, "unreadable_test.go")
			if err := os.Symlink("absent", link); err != nil {
				t.Fatalf("create source link: %v", err)
			}
			for _, name := range []string{"fuzz", "unit"} {
				t.Run(name, func(t *testing.T) {
					c := checker{}
					count := c.countFuzzTargets
					if name == "unit" {
						count = c.countGoTestFunctions
					}
					if got := count(root); got != 1 {
						t.Errorf("source count = %d, want 1", got)
					}
					if len(c.unreadable) != 1 {
						t.Fatalf("unreadable source findings = %+v, want one", c.unreadable)
					}
					if c.unreadable[0].File != link {
						t.Fatalf("unreadable path = %q, want %q", c.unreadable[0].File, link)
					}
				})
			}
		})
	}
}

// TestSourceCountsReportUnopenableRoots uses a regular file as an ancestor of
// each scan root, so the filesystem refuses the walk even when run as root.
func TestSourceCountsReportUnopenableRoots(t *testing.T) {
	base := t.TempDir()
	writeDoc(t, base, "not-a-directory", "not a directory\n")
	root := filepath.Join(base, "not-a-directory", "source")
	for _, name := range []string{"fuzz", "unit"} {
		t.Run(name, func(t *testing.T) {
			c := checker{}
			count := c.countFuzzTargets
			wantPaths := []string{root}
			if name == "unit" {
				count = c.countGoTestFunctions
				wantPaths = []string{
					filepath.Join(root, "internal"),
					filepath.Join(root, "pkg"),
					filepath.Join(root, "cmd"),
				}
			}
			if got := count(root); got != 0 {
				t.Errorf("source count = %d, want 0", got)
			}
			if len(c.unreadable) != len(wantPaths) {
				t.Fatalf("root findings = %+v, want paths %v", c.unreadable, wantPaths)
			}
			for i, want := range wantPaths {
				if c.unreadable[i].File != want {
					t.Errorf("root finding path = %q, want %q", c.unreadable[i].File, want)
				}
				if c.unreadable[i].Detail == "" {
					t.Errorf("root finding for %q has no filesystem error", want)
				}
			}
		})
	}
}

// TestSourceCountsAllowAbsentRoots distinguishes absent optional source trees
// from roots the filesystem cannot inspect, using both counters on each case.
func TestSourceCountsAllowAbsentRoots(t *testing.T) {
	base := t.TempDir()
	for _, root := range []string{base, filepath.Join(base, "absent")} {
		c := checker{}
		if got := c.countFuzzTargets(root); got != 0 {
			t.Errorf("fuzz targets under %q = %d, want 0", root, got)
		}
		if got := c.countGoTestFunctions(root); got != 0 {
			t.Errorf("product test functions under %q = %d, want 0", root, got)
		}
		if len(c.unreadable) != 0 {
			t.Fatalf("absent source roots produced findings: %+v", c.unreadable)
		}
	}
}
