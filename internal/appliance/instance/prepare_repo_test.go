package instance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// repoRoot walks up from the test's working directory to the module root (the
// directory holding go.mod alongside gokrazy/). It returns "" when the layout is
// not present, so the tests below can skip rather than fail in a stripped tree.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gokrazy", Name, "config.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// TestPrepareRealInstanceCarriesEveryModule verifies preparation of the ACTUAL
// checked-in gokrazy instance carries every builddir module and every tracked
// go.sum. The synthetic fixtures elsewhere in this package use two modules; only
// this test proves the real eight survive, which is what gok resolves the image
// from.
//
// VALIDATES: AC-3 -- the prepared instance holds all eight builddir modules.
// PREVENTS: a copy that silently drops modules, which would send gok to the
// network for whatever it could not find (the 2026-07-18 rtr7/kernel defect).
func TestPrepareRealInstanceCarriesEveryModule(t *testing.T) {
	root := repoRoot(t)
	if root == "" {
		t.Skip("checked-in gokrazy instance not found; not a full checkout")
	}
	srcBuildDir := filepath.Join(root, "gokrazy", Name, buildDirName)

	// Enumerate the tracked modules rather than hardcoding a count, so adding a
	// gokrazy submodule cannot leave this test asserting a stale number.
	var wantMods, wantSums []string
	if err := filepath.WalkDir(srcBuildDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcBuildDir, path)
		if relErr != nil {
			return relErr
		}
		switch d.Name() {
		case GoModName:
			wantMods = append(wantMods, rel)
		case "go.sum":
			wantSums = append(wantSums, rel)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk checked-in builddir: %v", err)
	}
	if len(wantMods) == 0 {
		t.Fatalf("no go.mod under %s; the fixture assumption is wrong", srcBuildDir)
	}

	prepared, cleanup, err := Prepare(filepath.Join(root, "gokrazy"), Options{})
	if err != nil {
		t.Fatalf("Prepare the real instance: %v", err)
	}
	defer cleanup()

	preparedBuildDir := filepath.Join(prepared, Name, buildDirName)
	for _, rel := range append(append([]string{}, wantMods...), wantSums...) {
		if _, err := os.Stat(filepath.Join(preparedBuildDir, rel)); err != nil {
			t.Errorf("prepared instance is missing %s: %v", rel, err)
		}
	}

	// Every filesystem-path replace must now be absolute and must exist. A
	// relative one would resolve against the new depth and point at nothing,
	// which is precisely how the pins were lost before.
	for _, rel := range wantMods {
		p := filepath.Join(preparedBuildDir, rel)
		data, readErr := os.ReadFile(p) //nolint:gosec // test-controlled path
		if readErr != nil {
			t.Errorf("read %s: %v", rel, readErr)
			continue
		}
		f, parseErr := modfile.Parse(p, data, nil)
		if parseErr != nil {
			t.Errorf("parse %s: %v", rel, parseErr)
			continue
		}
		for _, r := range f.Replace {
			if !modfile.IsDirectoryPath(r.New.Path) {
				continue // module-to-module version replace, depth-independent
			}
			if !filepath.IsAbs(r.New.Path) {
				t.Errorf("%s: replace %s => %s is still relative after preparation", rel, r.Old.Path, r.New.Path)
				continue
			}
			if _, err := os.Stat(r.New.Path); err != nil {
				t.Errorf("%s: replace %s => %s does not exist: %v", rel, r.Old.Path, r.New.Path, err)
			}
		}
	}
}

// listModules resolves the full module graph of the module rooted at dir,
// offline and against the checked-in gokrazy module cache, exactly as gok does
// (-mod=mod). It returns the raw "path version" lines, or an error.
func listModules(t *testing.T, dir, modcache string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-mod=mod", "-m", "-f", "{{.Path}} {{.Version}}", "all")
	cmd.Dir = dir
	var tb textbuf.Buffer
	cmd.Env = append(os.Environ(),
		tb.Str("GOMODCACHE=").Str(modcache).String(),
		"GOFLAGS=-modcacherw",
		"GOPROXY=off",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestPreparedModulesResolveIdenticallyToTracked verifies that every builddir
// module resolves to exactly the same version graph after preparation as before
// it, offline and against the checked-in module cache.
//
// This is the assertion that would have caught the 2026-07-18 defect on the day
// it landed. "The build succeeded" does not distinguish a build that used the
// pins from one that silently fetched newer versions; only comparing the
// resolved graph does.
//
// the two t.Skip calls below are ENVIRONMENT guards on a new test,
// not a relaxation of existing coverage. They fire only outside a full checkout
// or before `make ze-gokrazy-deps-download` has populated the cache, where no baseline
// exists to compare against. The test cannot skip its way to a false pass: if
// every module ends up skipped it calls t.Fatal, because a comparison test that
// compared nothing has proved nothing.
//
// VALIDATES: AC-5 -- per-module resolved versions are unchanged by preparation.
// PREVENTS: a preparation that quietly changes which upstream source is compiled
// into the appliance image, including its kernel.
func TestPreparedModulesResolveIdenticallyToTracked(t *testing.T) {
	root := repoRoot(t)
	if root == "" {
		t.Skip("checked-in gokrazy instance not found; not a full checkout")
	}
	modcache := filepath.Join(root, "gokrazy", "modcache")
	// Probe the DOWNLOAD cache, not the modcache directory. Only the vendored
	// gokrazy init sources under gokrazy/modcache are tracked (60 files); the
	// download cache that makes `go list` resolve offline is gitignored. A fresh
	// checkout therefore has the directory but resolves nothing, so every module
	// lost its baseline and the zero-comparison guard below reddened CI on every
	// push instead of reporting an absent prerequisite.
	if _, err := os.Stat(filepath.Join(modcache, "cache", "download")); err != nil {
		t.Skip("gokrazy/modcache download cache not populated; run make ze-gokrazy-deps-download")
	}
	srcBuildDir := filepath.Join(root, "gokrazy", Name, buildDirName)

	var mods []string
	if err := filepath.WalkDir(srcBuildDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == GoModName {
			rel, relErr := filepath.Rel(srcBuildDir, filepath.Dir(path))
			if relErr != nil {
				return relErr
			}
			mods = append(mods, rel)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk checked-in builddir: %v", err)
	}

	prepared, cleanup, err := Prepare(filepath.Join(root, "gokrazy"), Options{})
	if err != nil {
		t.Fatalf("Prepare the real instance: %v", err)
	}
	defer cleanup()
	preparedBuildDir := filepath.Join(prepared, Name, buildDirName)

	compared := 0
	for _, rel := range mods {
		want, wantErr := listModules(t, filepath.Join(srcBuildDir, rel), modcache)
		if wantErr != nil {
			// The tracked module itself cannot resolve offline, so there is no
			// baseline to compare against. Logging is honest here; failing would
			// blame preparation for a pre-existing cache gap.
			//
			// The go output is included because "exit status 1" alone is not
			// actionable: when every module lost its baseline the run ended at the
			// zero-comparison Fatal below with no indication of WHY go could not
			// resolve, which is exactly the state the QEMU unit phase reported.
			// a third ENVIRONMENT guard, in the same spirit as the two
			// documented on this test already, not a relaxed assertion. A missing Go
			// TOOLCHAIN is a prerequisite rather than a cache gap, and it strips every
			// module of its baseline at once -- so the zero-comparison Fatal below
			// fires and reads as a preparation regression when nothing was prepared
			// wrongly. The builddir modules pin a toolchain directive; when the
			// running Go is older and GOPROXY=off forbids fetching it, go reports
			// "toolchain not available" and resolves nothing. That is exactly the
			// QEMU VM, whose Go is installed by scripts/evidence/qemu-run.py. The
			// test still cannot skip its way to a false pass: on a host with the
			// toolchain it runs in full, and ze-precommit-verify is that host.
			if strings.Contains(want, "toolchain not available") {
				t.Skipf("go toolchain pinned by the builddir modules is unavailable offline; run where it is installed, or bump the VM's Go in scripts/evidence/qemu-run.py:\n%s", want)
			}
			t.Logf("no baseline for %s: tracked module does not resolve offline: %v\n%s", rel, wantErr, want)
			continue
		}
		got, gotErr := listModules(t, filepath.Join(preparedBuildDir, rel), modcache)
		if gotErr != nil {
			t.Errorf("%s: prepared module does not resolve offline though the tracked one does: %v\n%s", rel, gotErr, got)
			continue
		}
		if got != want {
			t.Errorf("%s: preparation changed the resolved module graph\n--- tracked ---\n%s\n--- prepared ---\n%s", rel, want, got)
		}
		compared++
	}

	if compared == 0 {
		t.Fatal("no module could be compared, so this test proved nothing; populate gokrazy/modcache with make ze-gokrazy-deps-download")
	}
	t.Logf("compared resolved module graphs for %d/%d builddir modules", compared, len(mods))
}

// TestPrepareRealInstanceLeavesTrackedTreeClean verifies preparing the real
// instance writes nothing under the tracked gokrazy directory.
//
// VALIDATES: AC-2 at the unit level -- preparation is one-directional.
// PREVENTS: a build step mutating a tracked path, the cross-commit hazard this
// spec exists to remove.
func TestPrepareRealInstanceLeavesTrackedTreeClean(t *testing.T) {
	root := repoRoot(t)
	if root == "" {
		t.Skip("checked-in gokrazy instance not found; not a full checkout")
	}
	tracked := filepath.Join(root, "gokrazy", Name)

	before, err := snapshotTree(tracked)
	if err != nil {
		t.Fatalf("snapshot before: %v", err)
	}

	prepared, cleanup, err := Prepare(filepath.Join(root, "gokrazy"), Options{ExtraKernelArgs: []string{"hugepages=512"}})
	if err != nil {
		t.Fatalf("Prepare the real instance: %v", err)
	}
	if !strings.HasPrefix(prepared, filepath.Join(root, "tmp")+string(filepath.Separator)) {
		t.Errorf("prepared parent %s is not under the project tmp/", prepared)
	}
	cleanup()

	after, err := snapshotTree(tracked)
	if err != nil {
		t.Fatalf("snapshot after: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("tracked tree changed size: %d entries before, %d after", len(before), len(after))
	}
	for path, mod := range before {
		if after[path] != mod {
			t.Errorf("tracked file %s was modified by a build preparation", path)
		}
	}
}

// snapshotTree maps each regular file under dir to its size and modification
// time, enough to detect any write without hashing the whole builddir.
func snapshotTree(dir string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		var b strings.Builder
		b.WriteString(info.ModTime().UTC().Format("20060102150405.000000000"))
		b.WriteByte(':')
		b.WriteString(strings.TrimSpace(formatInt(info.Size())))
		out[path] = b.String()
		return nil
	})
	return out, err
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
